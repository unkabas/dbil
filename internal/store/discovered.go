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

// discoverDEKInfo is the HKDF info string used to derive the deterministic
// DEK that encrypts discovered_connections.password_ct. Same trick as the
// audit DEK — no wrapped-key row needed; rotation flows from MK rotation.
const discoverDEKInfo = "dbil:discover-dek-v1"

// Discovery source values.
const (
	DiscoverSourceEnv    = "env"
	DiscoverSourceDocker = "docker"
)

// Discovery status values.
const (
	DiscoverStatusPending     = "pending"
	DiscoverStatusApproved    = "approved"
	DiscoverStatusRejected    = "rejected"
	DiscoverStatusUnreachable = "unreachable"
)

// ErrDiscoveredNotFound is returned by Get/Approve/Reject when no row matches.
var ErrDiscoveredNotFound = errors.New("discovered entry not found")

// Discovered is the metadata view of a discovered_connections row — no
// plaintext password. Approve returns the password separately.
type Discovered struct {
	ID             int64
	Source         string
	SourceKey      string
	Alias          string
	Host           string
	Port           int
	Database       string
	Username       string
	HasPassword    bool
	Tag            string
	Status         string
	LastSeen       time.Time
	CreatedAt      time.Time
	ApprovedConnID sql.NullInt64
}

// DiscoveredUpsert is the input shape for upserting a discovered entry. The
// discoverer builds one of these per scanned candidate; password is encrypted
// at the repo boundary.
type DiscoveredUpsert struct {
	Source    string
	SourceKey string
	Alias     string
	Host      string
	Port      int
	Database  string
	Username  string
	Password  string // optional; encrypted before INSERT
	Tag       string
}

// DiscoveredRepo persists discovered_connections rows.
type DiscoveredRepo struct {
	DB *sql.DB
	MK crypto.MasterKey
}

// NewDiscoveredRepo binds the repo to db + master key.
func NewDiscoveredRepo(db *sql.DB, mk crypto.MasterKey) *DiscoveredRepo {
	return &DiscoveredRepo{DB: db, MK: mk}
}

func (r *DiscoveredRepo) dek() ([]byte, error) {
	return crypto.HKDF(r.MK, discoverDEKInfo, crypto.KeySize)
}

// Upsert inserts a new entry or refreshes an existing one matching
// (source, source_key). Returns (entry, isNew). Rejected entries are not
// resurrected — they remain rejected. Approved entries only get last_seen
// refreshed.
func (r *DiscoveredRepo) Upsert(ctx context.Context, in DiscoveredUpsert) (Discovered, bool, error) {
	if err := validateTag(in.Tag); err != nil {
		return Discovered{}, false, err
	}
	if in.Source != DiscoverSourceEnv && in.Source != DiscoverSourceDocker {
		return Discovered{}, false, fmt.Errorf("invalid source %q", in.Source)
	}
	if in.Alias == "" || in.Host == "" || in.SourceKey == "" {
		return Discovered{}, false, fmt.Errorf("alias, host, source_key required")
	}
	if in.Port <= 0 || in.Port > 65535 {
		return Discovered{}, false, fmt.Errorf("port out of range")
	}

	now := time.Now().UTC()
	nowNS := now.UnixNano()

	dek, err := r.dek()
	if err != nil {
		return Discovered{}, false, err
	}
	defer crypto.Zero(dek)

	var nonce, ct []byte
	if in.Password != "" {
		ef, encErr := crypto.EncryptField(dek, in.Source+":"+in.SourceKey, []byte(in.Password))
		if encErr != nil {
			return Discovered{}, false, fmt.Errorf("encrypt password: %w", encErr)
		}
		nonce, ct = ef.Nonce, ef.Ciphertext
	}

	// Existing row?
	var existing Discovered
	row := r.DB.QueryRowContext(ctx,
		`SELECT id, source, source_key, alias, host, port, database, username,
		        (password_ct IS NOT NULL) AS has_password, tag, status,
		        last_seen_ns, created_at_ns, approved_conn_id
		   FROM discovered_connections WHERE source = ? AND source_key = ?`,
		in.Source, in.SourceKey,
	)
	if err := scanDiscovered(row, &existing); err == nil {
		// Refresh based on status.
		switch existing.Status {
		case DiscoverStatusRejected:
			// Do nothing.
			return existing, false, nil
		case DiscoverStatusApproved:
			_, _ = r.DB.ExecContext(ctx,
				`UPDATE discovered_connections SET last_seen_ns = ? WHERE id = ?`,
				nowNS, existing.ID,
			)
			existing.LastSeen = now
			return existing, false, nil
		default:
			// pending or unreachable → refresh fields and revive to pending.
			_, err := r.DB.ExecContext(ctx,
				`UPDATE discovered_connections
				   SET alias = ?, host = ?, port = ?, database = ?, username = ?,
				       password_nonce = ?, password_ct = ?, tag = ?, status = 'pending',
				       last_seen_ns = ?
				 WHERE id = ?`,
				in.Alias, in.Host, in.Port, in.Database, in.Username,
				nonce, ct, in.Tag, nowNS, existing.ID,
			)
			if err != nil {
				return Discovered{}, false, err
			}
			updated := existing
			updated.Alias = in.Alias
			updated.Host = in.Host
			updated.Port = in.Port
			updated.Database = in.Database
			updated.Username = in.Username
			updated.HasPassword = ct != nil
			updated.Tag = in.Tag
			updated.Status = DiscoverStatusPending
			updated.LastSeen = now
			return updated, false, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Discovered{}, false, fmt.Errorf("upsert lookup: %w", err)
	}

	res, err := r.DB.ExecContext(ctx,
		`INSERT INTO discovered_connections
		   (source, source_key, alias, host, port, database, username,
		    password_nonce, password_ct, tag, status, last_seen_ns, created_at_ns)
		   VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?)`,
		in.Source, in.SourceKey, in.Alias, in.Host, in.Port, in.Database,
		in.Username, nonce, ct, in.Tag, nowNS, nowNS,
	)
	if err != nil {
		return Discovered{}, false, fmt.Errorf("upsert insert: %w", err)
	}
	id, _ := res.LastInsertId()
	return Discovered{
		ID:          id,
		Source:      in.Source,
		SourceKey:   in.SourceKey,
		Alias:       in.Alias,
		Host:        in.Host,
		Port:        in.Port,
		Database:    in.Database,
		Username:    in.Username,
		HasPassword: ct != nil,
		Tag:         in.Tag,
		Status:      DiscoverStatusPending,
		LastSeen:    now,
		CreatedAt:   now,
	}, true, nil
}

// List returns all entries, optionally filtered by status. With no filter,
// every status is returned in created_at_ns desc order.
func (r *DiscoveredRepo) List(ctx context.Context, statuses ...string) ([]Discovered, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if len(statuses) == 0 {
		rows, err = r.DB.QueryContext(ctx,
			`SELECT id, source, source_key, alias, host, port, database, username,
			        (password_ct IS NOT NULL), tag, status, last_seen_ns, created_at_ns,
			        approved_conn_id
			   FROM discovered_connections
			  ORDER BY created_at_ns DESC`,
		)
	} else {
		placeholders := strings.Repeat("?,", len(statuses))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, len(statuses))
		for i, s := range statuses {
			args[i] = s
		}
		rows, err = r.DB.QueryContext(ctx,
			`SELECT id, source, source_key, alias, host, port, database, username,
			        (password_ct IS NOT NULL), tag, status, last_seen_ns, created_at_ns,
			        approved_conn_id
			   FROM discovered_connections
			  WHERE status IN (`+placeholders+`)
			  ORDER BY created_at_ns DESC`,
			args...,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Discovered
	for rows.Next() {
		var d Discovered
		if err := scanDiscovered(rows, &d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Get loads a single discovered row by id.
func (r *DiscoveredRepo) Get(ctx context.Context, id int64) (Discovered, error) {
	row := r.DB.QueryRowContext(ctx,
		`SELECT id, source, source_key, alias, host, port, database, username,
		        (password_ct IS NOT NULL), tag, status, last_seen_ns, created_at_ns,
		        approved_conn_id
		   FROM discovered_connections WHERE id = ?`, id,
	)
	var d Discovered
	if err := scanDiscovered(row, &d); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Discovered{}, ErrDiscoveredNotFound
		}
		return Discovered{}, err
	}
	return d, nil
}

// RevealPassword decrypts the stored password for id. Returns an empty string
// (no error) when no password was ever stored.
func (r *DiscoveredRepo) RevealPassword(ctx context.Context, id int64) (string, error) {
	var (
		source, sourceKey   string
		nonce, ct           []byte
	)
	err := r.DB.QueryRowContext(ctx,
		`SELECT source, source_key, password_nonce, password_ct
		   FROM discovered_connections WHERE id = ?`, id,
	).Scan(&source, &sourceKey, &nonce, &ct)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrDiscoveredNotFound
	}
	if err != nil {
		return "", err
	}
	if len(ct) == 0 {
		return "", nil
	}
	dek, err := r.dek()
	if err != nil {
		return "", err
	}
	defer crypto.Zero(dek)
	pt, err := crypto.DecryptField(dek, source+":"+sourceKey, crypto.EncryptedField{
		Nonce: nonce, Ciphertext: ct, Version: crypto.EnvelopeVersion,
	})
	if err != nil {
		return "", err
	}
	defer crypto.Zero(pt)
	return string(pt), nil
}

// MarkApproved records the connections.id of an approved entry.
func (r *DiscoveredRepo) MarkApproved(ctx context.Context, id, connID int64) error {
	res, err := r.DB.ExecContext(ctx,
		`UPDATE discovered_connections
		    SET status = 'approved', approved_conn_id = ?, last_seen_ns = ?
		  WHERE id = ?`,
		connID, time.Now().UnixNano(), id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrDiscoveredNotFound
	}
	return nil
}

// Reject sets status='rejected' so the entry never resurfaces on rescan.
func (r *DiscoveredRepo) Reject(ctx context.Context, id int64) error {
	res, err := r.DB.ExecContext(ctx,
		`UPDATE discovered_connections SET status = 'rejected' WHERE id = ?`, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrDiscoveredNotFound
	}
	return nil
}

// MarkUnreachable transitions any pending entry from source whose source_key
// is not in keepKeys into the unreachable state. Approved/rejected rows are
// untouched.
func (r *DiscoveredRepo) MarkUnreachable(ctx context.Context, source string, keepKeys []string) error {
	if len(keepKeys) == 0 {
		_, err := r.DB.ExecContext(ctx,
			`UPDATE discovered_connections SET status = 'unreachable'
			   WHERE source = ? AND status = 'pending'`, source,
		)
		return err
	}
	placeholders := strings.Repeat("?,", len(keepKeys))
	placeholders = placeholders[:len(placeholders)-1]
	args := []any{source}
	for _, k := range keepKeys {
		args = append(args, k)
	}
	_, err := r.DB.ExecContext(ctx,
		`UPDATE discovered_connections
		    SET status = 'unreachable'
		  WHERE source = ? AND status = 'pending'
		    AND source_key NOT IN (`+placeholders+`)`,
		args...,
	)
	return err
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows so we can reuse
// scanDiscovered between QueryRow and the iteration in List.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanDiscovered(rs rowScanner, d *Discovered) error {
	var lastNS, createdNS int64
	return scanIntoTime(rs.Scan(
		&d.ID, &d.Source, &d.SourceKey, &d.Alias, &d.Host, &d.Port,
		&d.Database, &d.Username, &d.HasPassword, &d.Tag, &d.Status,
		&lastNS, &createdNS, &d.ApprovedConnID,
	), &lastNS, &createdNS, d)
}

func scanIntoTime(err error, lastNS, createdNS *int64, d *Discovered) error {
	if err != nil {
		return err
	}
	d.LastSeen = time.Unix(0, *lastNS).UTC()
	d.CreatedAt = time.Unix(0, *createdNS).UTC()
	return nil
}
