package policy

import (
	"testing"
	"time"

	"github.com/unkabas/dbil/internal/store"
)

func TestPolicyFor_Local(t *testing.T) {
	p := PolicyFor(store.TagLocal)
	if p.Timeout != 5*time.Minute {
		t.Fatalf("local timeout: %v", p.Timeout)
	}
	if !p.DMLAllowed || !p.DDLAllowed || !p.DangerousAllowed {
		t.Fatal("local must allow everything")
	}
	if p.DMLRequiresConfirm || p.DDLRequiresConfirm || p.DangerousRequiresConfirm {
		t.Fatal("local must not require confirmation")
	}
}

func TestPolicyFor_Dev(t *testing.T) {
	p := PolicyFor(store.TagDev)
	if p.Timeout != 30*time.Second {
		t.Fatalf("dev timeout: %v", p.Timeout)
	}
	if !p.DMLAllowed || !p.DDLAllowed || !p.DangerousAllowed {
		t.Fatal("dev must allow everything")
	}
}

func TestPolicyFor_Staging(t *testing.T) {
	p := PolicyFor(store.TagStaging)
	if !p.DMLAllowed || !p.DDLAllowed || !p.DangerousAllowed {
		t.Fatal("staging must allow with confirmation, not block")
	}
	if !p.DMLRequiresConfirm || !p.DDLRequiresConfirm || !p.DangerousRequiresConfirm {
		t.Fatal("staging must require confirmation for DML/DDL/Dangerous")
	}
}

func TestPolicyFor_Production(t *testing.T) {
	p := PolicyFor(store.TagProduction)
	if p.Timeout != 10*time.Second {
		t.Fatalf("production timeout: %v", p.Timeout)
	}
	if !p.DMLAllowed {
		t.Fatal("production should allow DML with confirmation")
	}
	if !p.DMLRequiresConfirm {
		t.Fatal("production DML must require confirmation")
	}
	if p.DDLAllowed {
		t.Fatal("production must block DDL outright in Plan 4")
	}
	if p.DangerousAllowed {
		t.Fatal("production must block dangerous statements outright")
	}
}

func TestPolicyFor_Unknown(t *testing.T) {
	p := PolicyFor("unknown-tag")
	if p.DMLAllowed || p.DDLAllowed || p.DangerousAllowed {
		t.Fatal("unknown tag must default to deny-all")
	}
	if p.Timeout != 10*time.Second {
		t.Fatalf("unknown tag must use 10s timeout, got %v", p.Timeout)
	}
}
