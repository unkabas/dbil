package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/unkabas/dbil/internal/crypto"
)

// Connection tag values stored in the connections.tag column. These drive
// later policy (TLS required, statement timeouts, etc.) starting in Plan 4.
const (
	TagLocal      = "local"
	TagDev        = "dev"
	TagStaging    = "staging"
	TagProduction = "production"
)

// TLS mode values matching libpq.
const (
	TLSDisable    = "disable"
	TLSRequire    = "require"
	TLSVerifyCA   = "verify-ca"
	TLSVerifyFull = "verify-full"
)

var (
	// ErrConnectionExists is returned by Create when the alias is taken.
	ErrConnectionExists = errors.New("connection already exists")
	// ErrConnectionNotFound is returned by Get/Delete/Reveal when no row matches.
	ErrConnectionNotFound = errors.New("connection not found")
	// ErrPassphraseRequired is returned by Reveal when the connection was
	// stored with a passphrase but the caller did not supply one.
	ErrPassphraseRequired = errors.New("connection passphrase required")
	// ErrInvalidPassphrase is returned by Reveal when the supplied passphrase
	// fails to decrypt the password ciphertext.
	ErrInvalidPassphrase = errors.New("connection passphrase invalid")
)

// Connection is the metadata view of a row — no plaintext credentials.
type Connection struct {
	ID                 int64
	Alias              string
	Host               string
	Port               int
	Tag                string
	TLSMode            string
	RequiresPassphrase bool
	SSHHostID          *int64 // nil = direct connection; set = tunnel through this SSH host
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Revealed bundles a Connection with its decrypted credentials. Returned only
// from Reveal — never persisted, never serialised in API responses verbatim.
type Revealed struct {
	Connection
	Username string
	Password string
	Database string
}

// CreateConnectionParams is the input to ConnectionsRepo.Create.
type CreateConnectionParams struct {
	Alias      string
	Host       string
	Port       int
	Tag        string
	TLSMode    string
	Username   string
	Password   string
	Database   string
	Passphrase string // when non-empty, the password column is passphrase-wrapped
	SSHHostID  *int64 // optional: tunnel this connection through an ssh_hosts row
}

// nullableID converts a sql.NullInt64 to *int64.
func nullableID(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

// idValue converts a *int64 to an any suitable for a nullable SQL parameter.
func idValue(id *int64) any {
	if id == nil {
		return nil
	}
	return *id
}

// ConnectionsRepo wraps the connections table with envelope-encrypted access.
type ConnectionsRepo struct {
	DB *sql.DB
	MK crypto.MasterKey
}

// NewConnectionsRepo binds a repo to db and the master key.
func NewConnectionsRepo(db *sql.DB, mk crypto.MasterKey) *ConnectionsRepo {
	return &ConnectionsRepo{DB: db, MK: mk}
}

// Create inserts a new connection. A fresh DEK is generated and wrapped
// under the master key with AAD bound to the alias. Every sensitive field
// is encrypted with the DEK. If params.Passphrase is non-empty, the password
// column is additionally wrapped with the passphrase-derived key.
func (r *ConnectionsRepo) Create(ctx context.Context, params CreateConnectionParams) (Connection, error) {
	if params.Alias == "" {
		return Connection{}, errors.New("connections: alias required")
	}
	if err := validateTag(params.Tag); err != nil {
		return Connection{}, err
	}
	if err := validateTLS(params.TLSMode); err != nil {
		return Connection{}, err
	}
	if params.Port < 1 || params.Port > 65535 {
		return Connection{}, fmt.Errorf("connections: invalid port %d", params.Port)
	}

	dek, err := crypto.GenerateDEK()
	if err != nil {
		return Connection{}, fmt.Errorf("connections: generate dek: %w", err)
	}
	defer crypto.Zero(dek)

	wrapped, err := crypto.WrapDEK(r.MK, params.Alias, dek)
	if err != nil {
		return Connection{}, fmt.Errorf("connections: wrap dek: %w", err)
	}

	userEF, err := crypto.EncryptField(dek, params.Alias, []byte(params.Username))
	if err != nil {
		return Connection{}, err
	}
	dbEF, err := crypto.EncryptField(dek, params.Alias, []byte(params.Database))
	if err != nil {
		return Connection{}, err
	}

	var (
		salt               []byte
		passwordEF         crypto.EncryptedField
		requiresPassphrase = 0
	)
	if params.Passphrase != "" {
		salt, err = crypto.NewSalt()
		if err != nil {
			return Connection{}, fmt.Errorf("connections: salt: %w", err)
		}
		passwordEF, err = crypto.WrapWithPassphrase(params.Passphrase, salt, params.Alias, []byte(params.Password))
		if err != nil {
			return Connection{}, fmt.Errorf("connections: wrap password: %w", err)
		}
		requiresPassphrase = 1
	} else {
		passwordEF, err = crypto.EncryptField(dek, params.Alias, []byte(params.Password))
		if err != nil {
			return Connection{}, fmt.Errorf("connections: encrypt password: %w", err)
		}
	}

	now := time.Now().UnixNano()
	res, err := r.DB.ExecContext(ctx, `
		INSERT INTO connections
			(alias, host, port, tag, tls_mode, requires_passphrase, salt,
			 dek_nonce, dek_ciphertext,
			 username_nonce, username_ct,
			 password_nonce, password_ct,
			 database_nonce, database_ct,
			 ssh_host_id,
			 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.Alias, params.Host, params.Port, params.Tag, params.TLSMode, requiresPassphrase, salt,
		wrapped.Nonce, wrapped.Ciphertext,
		userEF.Nonce, userEF.Ciphertext,
		passwordEF.Nonce, passwordEF.Ciphertext,
		dbEF.Nonce, dbEF.Ciphertext,
		idValue(params.SSHHostID),
		now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return Connection{}, ErrConnectionExists
		}
		return Connection{}, fmt.Errorf("connections: insert: %w", err)
	}
	id, _ := res.LastInsertId()
	return Connection{
		ID:                 id,
		Alias:              params.Alias,
		Host:               params.Host,
		Port:               params.Port,
		Tag:                params.Tag,
		TLSMode:            params.TLSMode,
		RequiresPassphrase: requiresPassphrase != 0,
		SSHHostID:          params.SSHHostID,
		CreatedAt:          time.Unix(0, now),
		UpdatedAt:          time.Unix(0, now),
	}, nil
}

// List returns all connections in id-ascending order, metadata only.
func (r *ConnectionsRepo) List(ctx context.Context) ([]Connection, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, alias, host, port, tag, tls_mode, requires_passphrase, ssh_host_id, created_at, updated_at
		FROM connections ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("connections: list: %w", err)
	}
	defer rows.Close()
	var out []Connection
	for rows.Next() {
		var (
			c            Connection
			requiresPass int
			sshHostID    sql.NullInt64
			createdNS    int64
			updatedNS    int64
		)
		if err := rows.Scan(&c.ID, &c.Alias, &c.Host, &c.Port, &c.Tag, &c.TLSMode, &requiresPass, &sshHostID, &createdNS, &updatedNS); err != nil {
			return nil, fmt.Errorf("connections: list scan: %w", err)
		}
		c.RequiresPassphrase = requiresPass != 0
		c.SSHHostID = nullableID(sshHostID)
		c.CreatedAt = time.Unix(0, createdNS)
		c.UpdatedAt = time.Unix(0, updatedNS)
		out = append(out, c)
	}
	return out, rows.Err()
}

// Get returns metadata for one connection by id.
func (r *ConnectionsRepo) Get(ctx context.Context, id int64) (Connection, error) {
	var (
		c            Connection
		requiresPass int
		sshHostID    sql.NullInt64
		createdNS    int64
		updatedNS    int64
	)
	err := r.DB.QueryRowContext(ctx, `
		SELECT id, alias, host, port, tag, tls_mode, requires_passphrase, ssh_host_id, created_at, updated_at
		FROM connections WHERE id = ?`, id,
	).Scan(&c.ID, &c.Alias, &c.Host, &c.Port, &c.Tag, &c.TLSMode, &requiresPass, &sshHostID, &createdNS, &updatedNS)
	if errors.Is(err, sql.ErrNoRows) {
		return Connection{}, ErrConnectionNotFound
	}
	if err != nil {
		return Connection{}, fmt.Errorf("connections: get: %w", err)
	}
	c.RequiresPassphrase = requiresPass != 0
	c.SSHHostID = nullableID(sshHostID)
	c.CreatedAt = time.Unix(0, createdNS)
	c.UpdatedAt = time.Unix(0, updatedNS)
	return c, nil
}

// Delete removes a connection by id. Returns ErrConnectionNotFound for misses.
func (r *ConnectionsRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM connections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("connections: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrConnectionNotFound
	}
	return nil
}

// Reveal returns the decrypted credentials for a connection. When the row
// has requires_passphrase=true, an empty passphrase returns ErrPassphraseRequired
// and a wrong passphrase returns ErrInvalidPassphrase. For all other failures
// (corrupt ciphertext, wrong MK), the error is wrapped from crypto.
func (r *ConnectionsRepo) Reveal(ctx context.Context, id int64, passphrase string) (Revealed, error) {
	var (
		c             Connection
		requiresPass  int
		sshHostID     sql.NullInt64
		createdNS     int64
		updatedNS     int64
		salt          []byte
		dekNonce      []byte
		dekCT         []byte
		usernameNonce []byte
		usernameCT    []byte
		passwordNonce []byte
		passwordCT    []byte
		databaseNonce []byte
		databaseCT    []byte
	)
	err := r.DB.QueryRowContext(ctx, `
		SELECT id, alias, host, port, tag, tls_mode, requires_passphrase, ssh_host_id, salt,
		       dek_nonce, dek_ciphertext,
		       username_nonce, username_ct,
		       password_nonce, password_ct,
		       database_nonce, database_ct,
		       created_at, updated_at
		FROM connections WHERE id = ?`, id,
	).Scan(
		&c.ID, &c.Alias, &c.Host, &c.Port, &c.Tag, &c.TLSMode, &requiresPass, &sshHostID, &salt,
		&dekNonce, &dekCT,
		&usernameNonce, &usernameCT,
		&passwordNonce, &passwordCT,
		&databaseNonce, &databaseCT,
		&createdNS, &updatedNS,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Revealed{}, ErrConnectionNotFound
	}
	if err != nil {
		return Revealed{}, fmt.Errorf("connections: reveal load: %w", err)
	}
	c.RequiresPassphrase = requiresPass != 0
	c.SSHHostID = nullableID(sshHostID)
	c.CreatedAt = time.Unix(0, createdNS)
	c.UpdatedAt = time.Unix(0, updatedNS)

	if c.RequiresPassphrase && passphrase == "" {
		return Revealed{}, ErrPassphraseRequired
	}

	dek, err := crypto.UnwrapDEK(r.MK, c.Alias, crypto.WrappedDEK{Nonce: dekNonce, Ciphertext: dekCT})
	if err != nil {
		return Revealed{}, fmt.Errorf("connections: unwrap dek: %w", err)
	}
	defer crypto.Zero(dek)

	username, err := crypto.DecryptField(dek, c.Alias, crypto.EncryptedField{
		Nonce: usernameNonce, Ciphertext: usernameCT, Version: crypto.EnvelopeVersion,
	})
	if err != nil {
		return Revealed{}, fmt.Errorf("connections: decrypt username: %w", err)
	}
	database, err := crypto.DecryptField(dek, c.Alias, crypto.EncryptedField{
		Nonce: databaseNonce, Ciphertext: databaseCT, Version: crypto.EnvelopeVersion,
	})
	if err != nil {
		return Revealed{}, fmt.Errorf("connections: decrypt database: %w", err)
	}

	var password []byte
	if c.RequiresPassphrase {
		password, err = crypto.UnwrapWithPassphrase(passphrase, salt, c.Alias, crypto.EncryptedField{
			Nonce: passwordNonce, Ciphertext: passwordCT, Version: crypto.EnvelopeVersion,
		})
		if err != nil {
			return Revealed{}, ErrInvalidPassphrase
		}
	} else {
		password, err = crypto.DecryptField(dek, c.Alias, crypto.EncryptedField{
			Nonce: passwordNonce, Ciphertext: passwordCT, Version: crypto.EnvelopeVersion,
		})
		if err != nil {
			return Revealed{}, fmt.Errorf("connections: decrypt password: %w", err)
		}
	}

	return Revealed{
		Connection: c,
		Username:   string(username),
		Password:   string(password),
		Database:   string(database),
	}, nil
}

func validateTag(t string) error {
	switch t {
	case TagLocal, TagDev, TagStaging, TagProduction:
		return nil
	}
	return fmt.Errorf("connections: invalid tag %q", t)
}

func validateTLS(m string) error {
	switch m {
	case TLSDisable, TLSRequire, TLSVerifyCA, TLSVerifyFull:
		return nil
	}
	return fmt.Errorf("connections: invalid tls_mode %q", m)
}
