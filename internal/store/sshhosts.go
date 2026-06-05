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

// SSH authentication methods stored in ssh_hosts.auth_method.
const (
	SSHAuthKey      = "key"
	SSHAuthPassword = "password"
)

var (
	// ErrSSHHostExists is returned by CreateSSHHost when the alias is taken.
	ErrSSHHostExists = errors.New("ssh host already exists")
	// ErrSSHHostNotFound is returned when no ssh_hosts row matches.
	ErrSSHHostNotFound = errors.New("ssh host not found")
	// ErrSSHHostReferenced is returned by DeleteSSHHost when a connection still
	// references the host (ON DELETE RESTRICT).
	ErrSSHHostReferenced = errors.New("ssh host is referenced by a connection")
	// ErrSSHPassphraseRequired is returned by RevealSSHHost when the host's
	// secret is passphrase-wrapped but no passphrase was supplied.
	ErrSSHPassphraseRequired = errors.New("ssh host passphrase required")
	// ErrSSHInvalidPassphrase is returned when the supplied passphrase fails to
	// decrypt the host secret.
	ErrSSHInvalidPassphrase = errors.New("ssh host passphrase invalid")
)

// SSHHost is the metadata view of an ssh_hosts row — no secret material.
type SSHHost struct {
	ID                 int64
	Alias              string
	Host               string
	Port               int
	Username           string
	AuthMethod         string
	HostKeyFingerprint string
	RequiresPassphrase bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// RevealedSSHHost bundles an SSHHost with its decrypted secret. Returned only
// from RevealSSHHost; never persisted, never serialised in API responses.
type RevealedSSHHost struct {
	SSHHost
	Secret        string // private key PEM (auth_method=key) or password (auth_method=password)
	KeyPassphrase string // passphrase for an encrypted private key; empty otherwise
}

// CreateSSHHostParams is the input to CreateSSHHost.
type CreateSSHHostParams struct {
	Alias         string
	Host          string
	Port          int
	Username      string
	AuthMethod    string
	Secret        string // private key PEM or password
	KeyPassphrase string // optional: passphrase protecting an encrypted private key
	Passphrase    string // optional: when set, the secret is passphrase-wrapped (Argon2id)
}

// SSHHostsRepo wraps the ssh_hosts table with envelope-encrypted access.
type SSHHostsRepo struct {
	DB *sql.DB
	MK crypto.MasterKey
}

// NewSSHHostsRepo binds a repo to db and the master key.
func NewSSHHostsRepo(db *sql.DB, mk crypto.MasterKey) *SSHHostsRepo {
	return &SSHHostsRepo{DB: db, MK: mk}
}

func validateSSHAuth(m string) error {
	switch m {
	case SSHAuthKey, SSHAuthPassword:
		return nil
	}
	return fmt.Errorf("ssh hosts: invalid auth_method %q", m)
}

// Create inserts a new SSH host. A fresh DEK is wrapped under the master key
// (AAD bound to alias). The secret is DEK-encrypted, or passphrase-wrapped
// when params.Passphrase is set. An optional private-key passphrase is always
// DEK-encrypted.
func (r *SSHHostsRepo) Create(ctx context.Context, params CreateSSHHostParams) (SSHHost, error) {
	if params.Alias == "" {
		return SSHHost{}, errors.New("ssh hosts: alias required")
	}
	if err := validateSSHAuth(params.AuthMethod); err != nil {
		return SSHHost{}, err
	}
	if params.Port < 1 || params.Port > 65535 {
		return SSHHost{}, fmt.Errorf("ssh hosts: invalid port %d", params.Port)
	}
	if params.Secret == "" {
		return SSHHost{}, errors.New("ssh hosts: secret (key or password) required")
	}

	dek, err := crypto.GenerateDEK()
	if err != nil {
		return SSHHost{}, fmt.Errorf("ssh hosts: generate dek: %w", err)
	}
	defer crypto.Zero(dek)

	wrapped, err := crypto.WrapDEK(r.MK, params.Alias, dek)
	if err != nil {
		return SSHHost{}, fmt.Errorf("ssh hosts: wrap dek: %w", err)
	}

	var (
		salt               []byte
		secretEF           crypto.EncryptedField
		requiresPassphrase = 0
	)
	if params.Passphrase != "" {
		salt, err = crypto.NewSalt()
		if err != nil {
			return SSHHost{}, fmt.Errorf("ssh hosts: salt: %w", err)
		}
		secretEF, err = crypto.WrapWithPassphrase(params.Passphrase, salt, params.Alias, []byte(params.Secret))
		if err != nil {
			return SSHHost{}, fmt.Errorf("ssh hosts: wrap secret: %w", err)
		}
		requiresPassphrase = 1
	} else {
		secretEF, err = crypto.EncryptField(dek, params.Alias, []byte(params.Secret))
		if err != nil {
			return SSHHost{}, fmt.Errorf("ssh hosts: encrypt secret: %w", err)
		}
	}

	var keyPassNonce, keyPassCT []byte
	if params.KeyPassphrase != "" {
		kpEF, err := crypto.EncryptField(dek, params.Alias, []byte(params.KeyPassphrase))
		if err != nil {
			return SSHHost{}, fmt.Errorf("ssh hosts: encrypt key passphrase: %w", err)
		}
		keyPassNonce, keyPassCT = kpEF.Nonce, kpEF.Ciphertext
	}

	now := time.Now().UnixNano()
	res, err := r.DB.ExecContext(ctx, `
		INSERT INTO ssh_hosts
			(alias, host, port, username, auth_method, host_key_fingerprint,
			 requires_passphrase, salt,
			 dek_nonce, dek_ciphertext,
			 secret_nonce, secret_ct,
			 key_passphrase_nonce, key_passphrase_ct,
			 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.Alias, params.Host, params.Port, params.Username, params.AuthMethod, "",
		requiresPassphrase, salt,
		wrapped.Nonce, wrapped.Ciphertext,
		secretEF.Nonce, secretEF.Ciphertext,
		keyPassNonce, keyPassCT,
		now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return SSHHost{}, ErrSSHHostExists
		}
		return SSHHost{}, fmt.Errorf("ssh hosts: insert: %w", err)
	}
	id, _ := res.LastInsertId()
	return SSHHost{
		ID:                 id,
		Alias:              params.Alias,
		Host:               params.Host,
		Port:               params.Port,
		Username:           params.Username,
		AuthMethod:         params.AuthMethod,
		RequiresPassphrase: requiresPassphrase != 0,
		CreatedAt:          time.Unix(0, now),
		UpdatedAt:          time.Unix(0, now),
	}, nil
}

func scanSSHHost(s interface{ Scan(...any) error }) (SSHHost, error) {
	var (
		h            SSHHost
		requiresPass int
		createdNS    int64
		updatedNS    int64
	)
	if err := s.Scan(&h.ID, &h.Alias, &h.Host, &h.Port, &h.Username, &h.AuthMethod,
		&h.HostKeyFingerprint, &requiresPass, &createdNS, &updatedNS); err != nil {
		return SSHHost{}, err
	}
	h.RequiresPassphrase = requiresPass != 0
	h.CreatedAt = time.Unix(0, createdNS)
	h.UpdatedAt = time.Unix(0, updatedNS)
	return h, nil
}

const sshHostMetaCols = `id, alias, host, port, username, auth_method,
	host_key_fingerprint, requires_passphrase, created_at, updated_at`

// List returns all SSH hosts in id-ascending order, metadata only.
func (r *SSHHostsRepo) List(ctx context.Context) ([]SSHHost, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT `+sshHostMetaCols+` FROM ssh_hosts ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("ssh hosts: list: %w", err)
	}
	defer rows.Close()
	var out []SSHHost
	for rows.Next() {
		h, err := scanSSHHost(rows)
		if err != nil {
			return nil, fmt.Errorf("ssh hosts: list scan: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// Get returns metadata for one SSH host by id.
func (r *SSHHostsRepo) Get(ctx context.Context, id int64) (SSHHost, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT `+sshHostMetaCols+` FROM ssh_hosts WHERE id = ?`, id)
	h, err := scanSSHHost(row)
	if errors.Is(err, sql.ErrNoRows) {
		return SSHHost{}, ErrSSHHostNotFound
	}
	if err != nil {
		return SSHHost{}, fmt.Errorf("ssh hosts: get: %w", err)
	}
	return h, nil
}

// Delete removes an SSH host. Returns ErrSSHHostReferenced when a connection
// still points at it (FK ON DELETE RESTRICT), ErrSSHHostNotFound on a miss.
func (r *SSHHostsRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM ssh_hosts WHERE id = ?`, id)
	if err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY constraint") {
			return ErrSSHHostReferenced
		}
		return fmt.Errorf("ssh hosts: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSSHHostNotFound
	}
	return nil
}

// SetFingerprint pins (or updates) the host key fingerprint, called after a
// successful trust-on-first-use test connection.
func (r *SSHHostsRepo) SetFingerprint(ctx context.Context, id int64, fp string) error {
	now := time.Now().UnixNano()
	res, err := r.DB.ExecContext(ctx,
		`UPDATE ssh_hosts SET host_key_fingerprint = ?, updated_at = ? WHERE id = ?`, fp, now, id)
	if err != nil {
		return fmt.Errorf("ssh hosts: set fingerprint: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrSSHHostNotFound
	}
	return nil
}

// Reveal returns the decrypted secret (and optional key passphrase) for an SSH
// host. Passphrase-wrapped hosts require a non-empty passphrase.
func (r *SSHHostsRepo) Reveal(ctx context.Context, id int64, passphrase string) (RevealedSSHHost, error) {
	var (
		h            SSHHost
		requiresPass int
		createdNS    int64
		updatedNS    int64
		salt         []byte
		dekNonce     []byte
		dekCT        []byte
		secretNonce  []byte
		secretCT     []byte
		kpNonce      []byte
		kpCT         []byte
	)
	err := r.DB.QueryRowContext(ctx, `
		SELECT id, alias, host, port, username, auth_method, host_key_fingerprint,
		       requires_passphrase, salt,
		       dek_nonce, dek_ciphertext,
		       secret_nonce, secret_ct,
		       key_passphrase_nonce, key_passphrase_ct,
		       created_at, updated_at
		FROM ssh_hosts WHERE id = ?`, id,
	).Scan(
		&h.ID, &h.Alias, &h.Host, &h.Port, &h.Username, &h.AuthMethod, &h.HostKeyFingerprint,
		&requiresPass, &salt,
		&dekNonce, &dekCT,
		&secretNonce, &secretCT,
		&kpNonce, &kpCT,
		&createdNS, &updatedNS,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RevealedSSHHost{}, ErrSSHHostNotFound
	}
	if err != nil {
		return RevealedSSHHost{}, fmt.Errorf("ssh hosts: reveal load: %w", err)
	}
	h.RequiresPassphrase = requiresPass != 0
	h.CreatedAt = time.Unix(0, createdNS)
	h.UpdatedAt = time.Unix(0, updatedNS)

	if h.RequiresPassphrase && passphrase == "" {
		return RevealedSSHHost{}, ErrSSHPassphraseRequired
	}

	dek, err := crypto.UnwrapDEK(r.MK, h.Alias, crypto.WrappedDEK{Nonce: dekNonce, Ciphertext: dekCT})
	if err != nil {
		return RevealedSSHHost{}, fmt.Errorf("ssh hosts: unwrap dek: %w", err)
	}
	defer crypto.Zero(dek)

	var secret []byte
	if h.RequiresPassphrase {
		secret, err = crypto.UnwrapWithPassphrase(passphrase, salt, h.Alias, crypto.EncryptedField{
			Nonce: secretNonce, Ciphertext: secretCT, Version: crypto.EnvelopeVersion,
		})
		if err != nil {
			return RevealedSSHHost{}, ErrSSHInvalidPassphrase
		}
	} else {
		secret, err = crypto.DecryptField(dek, h.Alias, crypto.EncryptedField{
			Nonce: secretNonce, Ciphertext: secretCT, Version: crypto.EnvelopeVersion,
		})
		if err != nil {
			return RevealedSSHHost{}, fmt.Errorf("ssh hosts: decrypt secret: %w", err)
		}
	}

	keyPass := ""
	if len(kpCT) > 0 {
		kp, err := crypto.DecryptField(dek, h.Alias, crypto.EncryptedField{
			Nonce: kpNonce, Ciphertext: kpCT, Version: crypto.EnvelopeVersion,
		})
		if err != nil {
			return RevealedSSHHost{}, fmt.Errorf("ssh hosts: decrypt key passphrase: %w", err)
		}
		keyPass = string(kp)
	}

	return RevealedSSHHost{SSHHost: h, Secret: string(secret), KeyPassphrase: keyPass}, nil
}
