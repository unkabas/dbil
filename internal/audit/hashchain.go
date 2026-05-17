// Package audit provides the pure helpers for DBil's tamper-evident audit log:
// canonical JSON encoding, the per-entry hash function, and chain verification.
//
// Storage (writing entries to SQLite, holding the audit DEK, etc.) lives in
// internal/store. This package is intentionally side-effect-free so it can be
// fuzzed and reasoned about in isolation.
package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// GenesisSeed is the fixed seed string for the audit chain's genesis prev_hash.
// Changing this value would re-genesis every existing chain.
const GenesisSeed = "dbil-audit-genesis-v1"

// GenesisHash is sha256(GenesisSeed). It is the prev_hash supplied for the
// very first entry in any audit log.
var GenesisHash = sha256.Sum256([]byte(GenesisSeed))

// ErrChainBroken is returned by Verify when an entry's stored prev_hash or
// entry_hash does not match its recomputed value.
var ErrChainBroken = errors.New("audit chain integrity broken")

// Entry is a single audit log record. The Details field carries arbitrary
// structured data and is canonicalized before being hashed so the chain is
// stable across JSON key-order rewrites.
type Entry struct {
	ID       uint64
	TS       int64 // nanoseconds since unix epoch
	UserID   string
	Action   string
	Resource string
	Details  json.RawMessage
	PrevHash [32]byte
}

// Canonicalize re-encodes a JSON value into a deterministic byte form by
// recursively sorting every object's keys. Arrays preserve their order.
// Empty / nil input is treated as JSON null so callers do not have to
// special-case "no details".
func Canonicalize(raw json.RawMessage) ([]byte, error) {
	if len(raw) == 0 {
		return []byte("null"), nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("audit: canonicalize: %w", err)
	}
	var buf bytes.Buffer
	if err := writeCanonical(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonical(buf *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
		return nil
	case bool, float64, string:
		b, err := json.Marshal(x)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	case []any:
		buf.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(kb)
			buf.WriteByte(':')
			if err := writeCanonical(buf, x[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
		return nil
	default:
		return fmt.Errorf("audit: unsupported JSON value of type %T", x)
	}
}

// Hash computes the SHA-256 of the canonical entry encoding per the spec:
//
//	sha256( BE(id) || BE(ts) || user_id || action || resource ||
//	        canonical(details) || prev_hash )
func Hash(e Entry) ([32]byte, error) {
	details, err := Canonicalize(e.Details)
	if err != nil {
		return [32]byte{}, err
	}
	var buf bytes.Buffer
	var u [8]byte
	binary.BigEndian.PutUint64(u[:], e.ID)
	buf.Write(u[:])
	binary.BigEndian.PutUint64(u[:], uint64(e.TS))
	buf.Write(u[:])
	buf.WriteString(e.UserID)
	buf.WriteString(e.Action)
	buf.WriteString(e.Resource)
	buf.Write(details)
	buf.Write(e.PrevHash[:])
	return sha256.Sum256(buf.Bytes()), nil
}

// Verify reports a chain-integrity error if current.PrevHash differs from
// prevHash or if currentHash differs from Hash(current).
func Verify(prevHash [32]byte, current Entry, currentHash [32]byte) error {
	if current.PrevHash != prevHash {
		return fmt.Errorf("%w: prev_hash mismatch at entry %d", ErrChainBroken, current.ID)
	}
	got, err := Hash(current)
	if err != nil {
		return err
	}
	if got != currentHash {
		return fmt.Errorf("%w: entry_hash mismatch at entry %d", ErrChainBroken, current.ID)
	}
	return nil
}
