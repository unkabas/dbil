package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/unkabas/dbil/internal/audit"
	"github.com/unkabas/dbil/internal/crypto"
)

// auditDEKInfo is the HKDF info string used to derive the audit-table DEK
// from the master key. A fixed deterministic DEK avoids the need to store a
// wrapped DEK row for the audit table.
const auditDEKInfo = "dbil:audit-dek-v1"

// AppendResult is returned by Append.
type AppendResult struct {
	Entry     audit.Entry
	EntryHash [32]byte
}

// AuditRepo writes and verifies audit log entries.
type AuditRepo struct {
	DB *sql.DB
	MK crypto.MasterKey
}

// NewAuditRepo binds an AuditRepo to db and mk.
func NewAuditRepo(db *sql.DB, mk crypto.MasterKey) *AuditRepo {
	return &AuditRepo{DB: db, MK: mk}
}

func (r *AuditRepo) auditDEK() ([]byte, error) {
	return crypto.HKDF(r.MK, auditDEKInfo, crypto.KeySize)
}

// Append serializes details (canonical JSON via marshal), encrypts them
// with the audit DEK, computes the chain hash, and inserts the row inside
// a single transaction so concurrent appends remain serializable.
func (r *AuditRepo) Append(ctx context.Context, userID, action, resource string, details map[string]any) (AppendResult, error) {
	raw, err := json.Marshal(details)
	if err != nil {
		return AppendResult{}, fmt.Errorf("audit append: marshal details: %w", err)
	}

	dek, err := r.auditDEK()
	if err != nil {
		return AppendResult{}, err
	}
	defer crypto.Zero(dek)

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return AppendResult{}, fmt.Errorf("audit append: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		prevHash  [32]byte
		prevBytes []byte
	)
	err = tx.QueryRowContext(ctx, `SELECT entry_hash FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&prevBytes)
	switch {
	case err == nil:
		if len(prevBytes) != 32 {
			return AppendResult{}, fmt.Errorf("audit append: prev entry_hash has %d bytes", len(prevBytes))
		}
		copy(prevHash[:], prevBytes)
	case errors.Is(err, sql.ErrNoRows):
		prevHash = audit.GenesisHash
	default:
		return AppendResult{}, fmt.Errorf("audit append: load prev hash: %w", err)
	}

	var prevID int64
	if err := tx.QueryRowContext(ctx, `SELECT IFNULL(MAX(id), 0) FROM audit_log`).Scan(&prevID); err != nil {
		return AppendResult{}, fmt.Errorf("audit append: load max id: %w", err)
	}
	nextID := uint64(prevID + 1)

	aadConnID := fmt.Sprintf("audit:%d", nextID)
	ef, err := crypto.EncryptField(dek, aadConnID, raw)
	if err != nil {
		return AppendResult{}, fmt.Errorf("audit append: encrypt details: %w", err)
	}

	now := time.Now().UnixNano()
	entry := audit.Entry{
		ID:       nextID,
		TS:       now,
		UserID:   userID,
		Action:   action,
		Resource: resource,
		Details:  json.RawMessage(raw),
		PrevHash: prevHash,
	}
	h, err := audit.Hash(entry)
	if err != nil {
		return AppendResult{}, fmt.Errorf("audit append: hash: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_log
		    (id, ts_ns, user_id, action, resource, details_enc, details_nonce, prev_hash, entry_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		int64(nextID), now, userID, action, resource,
		ef.Ciphertext, ef.Nonce, prevHash[:], h[:],
	)
	if err != nil {
		return AppendResult{}, fmt.Errorf("audit append: insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return AppendResult{}, fmt.Errorf("audit append: commit: %w", err)
	}
	return AppendResult{Entry: entry, EntryHash: h}, nil
}

// Count returns the number of rows in audit_log.
func (r *AuditRepo) Count(ctx context.Context) (int, error) {
	var n int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log`).Scan(&n); err != nil {
		return 0, fmt.Errorf("audit count: %w", err)
	}
	return n, nil
}

// VerifyChain walks audit_log in id order, decrypts each row's details,
// and recomputes both prev_hash linkage and entry_hash. Returns the first
// violation wrapping audit.ErrChainBroken.
func (r *AuditRepo) VerifyChain(ctx context.Context) error {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, ts_ns, user_id, action, resource,
		       details_enc, details_nonce, prev_hash, entry_hash
		FROM audit_log
		ORDER BY id ASC`)
	if err != nil {
		return fmt.Errorf("audit verify: query: %w", err)
	}
	defer rows.Close()

	dek, err := r.auditDEK()
	if err != nil {
		return err
	}
	defer crypto.Zero(dek)

	expectedPrev := audit.GenesisHash
	for rows.Next() {
		var (
			id           int64
			tsNS         int64
			userID       string
			action       string
			resource     string
			detailsEnc   []byte
			detailsNonce []byte
			prevHashB    []byte
			entryHashB   []byte
		)
		if err := rows.Scan(&id, &tsNS, &userID, &action, &resource,
			&detailsEnc, &detailsNonce, &prevHashB, &entryHashB); err != nil {
			return fmt.Errorf("audit verify: scan: %w", err)
		}
		if len(prevHashB) != 32 || len(entryHashB) != 32 {
			return fmt.Errorf("audit verify: malformed hash columns at id %d", id)
		}
		var prev, got [32]byte
		copy(prev[:], prevHashB)
		copy(got[:], entryHashB)

		if prev != expectedPrev {
			return fmt.Errorf("%w: prev_hash mismatch at id %d", audit.ErrChainBroken, id)
		}

		aadConnID := fmt.Sprintf("audit:%d", id)
		ef := crypto.EncryptedField{
			Nonce:      detailsNonce,
			Ciphertext: detailsEnc,
			Version:    crypto.EnvelopeVersion,
		}
		raw, err := crypto.DecryptField(dek, aadConnID, ef)
		if err != nil {
			return fmt.Errorf("%w: decrypt details at id %d: %v", audit.ErrChainBroken, id, err)
		}

		entry := audit.Entry{
			ID:       uint64(id),
			TS:       tsNS,
			UserID:   userID,
			Action:   action,
			Resource: resource,
			Details:  json.RawMessage(raw),
			PrevHash: prev,
		}
		if err := audit.Verify(expectedPrev, entry, got); err != nil {
			return err
		}
		expectedPrev = got
	}
	return rows.Err()
}
