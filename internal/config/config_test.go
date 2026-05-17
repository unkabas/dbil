package config

import (
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"DBIL_DATA_DIR", "DBIL_PORT", "DBIL_MASTER_KEY_FILE",
		"DBIL_MASTER_KEY_ENV", "DBIL_AUDIT_SYSLOG",
	} {
		t.Setenv(k, "")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 4242 {
		t.Fatalf("default port want 4242, got %d", cfg.Port)
	}
	if cfg.MasterKeyEnvVar != "DBIL_MASTER_KEY" {
		t.Fatalf("default env var name unexpected: %q", cfg.MasterKeyEnvVar)
	}
	if cfg.DataDir == "" {
		t.Fatal("default DataDir empty")
	}
	if cfg.MasterKeyFile != "" {
		t.Fatalf("default MasterKeyFile must be empty, got %q", cfg.MasterKeyFile)
	}
	if cfg.AuditSyslogAddr != "" {
		t.Fatalf("default AuditSyslogAddr must be empty, got %q", cfg.AuditSyslogAddr)
	}
}

func TestLoad_OverridesAll(t *testing.T) {
	clearEnv(t)
	t.Setenv("DBIL_DATA_DIR", "/tmp/dbil-x")
	t.Setenv("DBIL_PORT", "8080")
	t.Setenv("DBIL_MASTER_KEY_FILE", "/secrets/mk")
	t.Setenv("DBIL_MASTER_KEY_ENV", "MY_KEY")
	t.Setenv("DBIL_AUDIT_SYSLOG", "syslog.local:514")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "/tmp/dbil-x" {
		t.Errorf("DataDir not overridden, got %q", cfg.DataDir)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port not overridden, got %d", cfg.Port)
	}
	if cfg.MasterKeyFile != "/secrets/mk" {
		t.Errorf("MasterKeyFile not overridden, got %q", cfg.MasterKeyFile)
	}
	if cfg.MasterKeyEnvVar != "MY_KEY" {
		t.Errorf("MasterKeyEnvVar not overridden, got %q", cfg.MasterKeyEnvVar)
	}
	if cfg.AuditSyslogAddr != "syslog.local:514" {
		t.Errorf("AuditSyslogAddr not overridden, got %q", cfg.AuditSyslogAddr)
	}
}

func TestLoad_BadPort(t *testing.T) {
	clearEnv(t)
	t.Setenv("DBIL_PORT", "notanumber")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-numeric port")
	}
}

func TestLoad_PortOutOfRange(t *testing.T) {
	for _, v := range []string{"0", "65536", "-1"} {
		t.Run(v, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("DBIL_PORT", v)
			if _, err := Load(); err == nil {
				t.Fatalf("expected error for port %s", v)
			}
		})
	}
}
