// Package config loads DBilConfig from environment variables with sensible
// defaults for both container and local-dev environments.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// DBilConfig is the runtime configuration the binary uses everywhere. It is
// loaded once at startup; per-command flags should be added as cobra flags
// rather than additional env reads.
type DBilConfig struct {
	// DataDir holds the SQLite state file, the auto-generated master key (if
	// applicable), and the initial-credentials.txt artifact from `dbil init`.
	DataDir string

	// Port is the HTTP UI port. Default 4242 (uncommon enough that it rarely
	// collides with project backends, unlike 8080/3000/5000).
	Port int

	// MasterKeyFile is the path passed to the file-based MK loader. Empty
	// makes the loader skip itself, falling through to the next loader.
	MasterKeyFile string

	// MasterKeyEnvVar is the name of the env var the env loader reads.
	// Default DBIL_MASTER_KEY.
	MasterKeyEnvVar string

	// AuditSyslogAddr, when non-empty, enables periodic export of audit-chain
	// checkpoints to a syslog endpoint.
	AuditSyslogAddr string
}

// Load reads DBilConfig from the process environment.
func Load() (DBilConfig, error) {
	cfg := DBilConfig{
		DataDir:         defaultDataDir(),
		Port:            4242,
		MasterKeyFile:   "",
		MasterKeyEnvVar: "DBIL_MASTER_KEY",
		AuditSyslogAddr: "",
	}
	if v := os.Getenv("DBIL_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("DBIL_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return DBilConfig{}, fmt.Errorf("config: DBIL_PORT not a number: %w", err)
		}
		if n < 1 || n > 65535 {
			return DBilConfig{}, fmt.Errorf("config: DBIL_PORT out of range: %d", n)
		}
		cfg.Port = n
	}
	if v := os.Getenv("DBIL_MASTER_KEY_FILE"); v != "" {
		cfg.MasterKeyFile = v
	}
	if v := os.Getenv("DBIL_MASTER_KEY_ENV"); v != "" {
		cfg.MasterKeyEnvVar = v
	}
	if v := os.Getenv("DBIL_AUDIT_SYSLOG"); v != "" {
		cfg.AuditSyslogAddr = v
	}
	return cfg, nil
}

// defaultDataDir returns "/data" when running inside a Docker container
// (detected via /.dockerenv) and "./dbil-data" otherwise.
func defaultDataDir() string {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "/data"
	}
	return "./dbil-data"
}
