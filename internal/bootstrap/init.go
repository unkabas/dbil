// Package bootstrap orchestrates `dbil init`: master key load, state store
// open and migrate, admin user creation, and the genesis audit entry.
//
// RunInit is idempotent: a second invocation against the same DataDir does
// not create another admin or another genesis entry, but it does verify the
// existing audit chain.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/unkabas/dbil/internal/auth"
	"github.com/unkabas/dbil/internal/config"
	"github.com/unkabas/dbil/internal/crypto"
	"github.com/unkabas/dbil/internal/store"
)

// Version is the build version string; main wires it from ldflags.
var Version = "dev"

// AdminEmail is the email assigned to the auto-created admin user.
const AdminEmail = "admin@local"

// InitResult is what RunInit returns.
type InitResult struct {
	AdminEmail      string
	MasterKeySource crypto.Source
	AuditGenesisID  uint64
	CreatedAdmin    bool
}

// RunInit performs the one-time bootstrap of a DBil data directory:
//
//  1. Ensure DataDir exists (0700).
//  2. Run the master-key loader chain.
//  3. Open SQLite at DataDir/dbil.db and apply migrations.
//  4. If no admin user exists, generate one, write initial-credentials.txt
//     (0600), and append a genesis audit entry.
//  5. On re-runs, verify the audit chain instead of re-creating anything.
func RunInit(ctx context.Context, cfg config.DBilConfig) (InitResult, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return InitResult{}, fmt.Errorf("bootstrap: mkdir data dir: %w", err)
	}

	mk, src, err := LoadMasterKey(ctx, cfg)
	if err != nil {
		return InitResult{}, fmt.Errorf("bootstrap: load master key: %w", err)
	}
	defer mk.Wipe()

	slog.Info("master key loaded", "source", string(src))
	if src == crypto.SourceAuto || src == crypto.SourceEnv {
		slog.Warn("master key source is suitable for dev only; configure DBIL_MASTER_KEY_FILE (Docker secret) or KMS for production",
			"source", string(src))
	}

	db, err := store.Open(filepath.Join(cfg.DataDir, "dbil.db"))
	if err != nil {
		return InitResult{}, fmt.Errorf("bootstrap: open db: %w", err)
	}
	defer store.Close(db)

	if err := store.Apply(db); err != nil {
		return InitResult{}, fmt.Errorf("bootstrap: apply migrations: %w", err)
	}

	users := store.NewUsersRepo(db)
	auditRepo := store.NewAuditRepo(db, mk)

	res := InitResult{AdminEmail: AdminEmail, MasterKeySource: src}

	hasAdmin, err := users.AdminExists(ctx)
	if err != nil {
		return InitResult{}, fmt.Errorf("bootstrap: check admin: %w", err)
	}
	if !hasAdmin {
		password, err := auth.GeneratePassword()
		if err != nil {
			return InitResult{}, fmt.Errorf("bootstrap: generate password: %w", err)
		}
		if _, err := users.Create(ctx, AdminEmail, password, store.RoleAdmin, true); err != nil {
			return InitResult{}, fmt.Errorf("bootstrap: create admin: %w", err)
		}
		credPath := filepath.Join(cfg.DataDir, "initial-credentials.txt")
		if err := writeInitialCreds(credPath, AdminEmail, password); err != nil {
			return InitResult{}, fmt.Errorf("bootstrap: write initial creds: %w", err)
		}
		slog.Warn("initial admin password generated; rotate immediately after first login",
			"email", AdminEmail, "password_file", credPath)
		fmt.Println("=== DBil first-run admin credentials ===")
		fmt.Printf("email:    %s\n", AdminEmail)
		fmt.Printf("password: %s\n", password)
		fmt.Printf("stored:   %s (mode 0600)\n", credPath)
		fmt.Println("=> Rotate this password at first login.")
		fmt.Println("=========================================")
		res.CreatedAdmin = true
	}

	n, err := auditRepo.Count(ctx)
	if err != nil {
		return InitResult{}, fmt.Errorf("bootstrap: count audit: %w", err)
	}
	if n == 0 {
		ar, err := auditRepo.Append(ctx, "system", "bootstrap.init", "dbil", map[string]any{
			"version":           Version,
			"master_key_source": string(src),
		})
		if err != nil {
			return InitResult{}, fmt.Errorf("bootstrap: append genesis: %w", err)
		}
		res.AuditGenesisID = ar.Entry.ID
	} else {
		if err := auditRepo.VerifyChain(ctx); err != nil {
			return InitResult{}, fmt.Errorf("bootstrap: audit chain verify: %w", err)
		}
		// The genesis entry is always row 1 by construction.
		res.AuditGenesisID = 1
	}
	return res, nil
}

// LoadMasterKey builds and runs the full loader chain in the documented
// order: KMS, Keychain, File, Env, TTY, Auto. Exposed so both `dbil init`
// and `dbil serve` (and future commands) share the exact same source order.
func LoadMasterKey(ctx context.Context, cfg config.DBilConfig) (crypto.MasterKey, crypto.Source, error) {
	chain := crypto.NewChain(
		crypto.NewKMSLoader(),
		crypto.NewKeychainLoader(),
		crypto.NewFileLoader(cfg.MasterKeyFile),
		crypto.NewEnvLoader(cfg.MasterKeyEnvVar),
		crypto.NewTTYLoader(),
		crypto.NewAutoLoader(filepath.Join(cfg.DataDir, "master.key")),
	)
	return chain.Load(ctx)
}

// writeInitialCreds writes the first-run credentials file at mode 0600.
func writeInitialCreds(path, email, password string) error {
	body := fmt.Sprintf(
		"# DBil initial credentials — rotate after first login, then delete this file.\nemail=%s\npassword=%s\n",
		email, password,
	)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return err
	}
	// os.WriteFile only honors mode on file creation, so force it for re-init.
	return os.Chmod(path, 0o600)
}
