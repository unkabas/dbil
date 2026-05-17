// Package policy maps a connection's tag to its safety policy: statement
// timeout, DML/DDL gating, dangerous-statement gating, and confirmation
// requirements. The numbers come from spec section 7.4.
package policy

import (
	"time"

	"github.com/unkabas/dbil/internal/store"
)

// Policy is the per-tag safety configuration applied by Manager.Execute.
type Policy struct {
	// Timeout caps a single statement's execution time via context deadline.
	Timeout time.Duration

	// DMLAllowed false blocks INSERT/UPDATE/DELETE/MERGE/TRUNCATE/COPY/CALL.
	DMLAllowed bool
	// DMLRequiresConfirm requires X-Confirm: yes for any DML statement.
	DMLRequiresConfirm bool

	// DDLAllowed false blocks CREATE/ALTER/DROP/GRANT/REVOKE/etc outright.
	DDLAllowed bool
	// DDLRequiresConfirm requires X-Confirm: yes for any DDL statement.
	DDLRequiresConfirm bool

	// DangerousAllowed false blocks DELETE/UPDATE-without-WHERE outright.
	DangerousAllowed bool
	// DangerousRequiresConfirm requires X-Confirm: yes for those statements.
	DangerousRequiresConfirm bool
}

// PolicyFor returns the default policy for tag. Unknown tags get a
// conservative policy (10 s timeout, nothing allowed) so a misconfigured
// connection cannot silently bypass guardrails.
func PolicyFor(tag string) Policy {
	switch tag {
	case store.TagLocal:
		return Policy{
			Timeout:          5 * time.Minute,
			DMLAllowed:       true,
			DDLAllowed:       true,
			DangerousAllowed: true,
		}
	case store.TagDev:
		return Policy{
			Timeout:          30 * time.Second,
			DMLAllowed:       true,
			DDLAllowed:       true,
			DangerousAllowed: true,
		}
	case store.TagStaging:
		return Policy{
			Timeout:                  30 * time.Second,
			DMLAllowed:               true,
			DMLRequiresConfirm:       true,
			DDLAllowed:               true,
			DDLRequiresConfirm:       true,
			DangerousAllowed:         true,
			DangerousRequiresConfirm: true,
		}
	case store.TagProduction:
		return Policy{
			Timeout:            10 * time.Second,
			DMLAllowed:         true,
			DMLRequiresConfirm: true,
			// DDL blocked outright until "second approval" workflows ship.
			DDLAllowed: false,
			// Dangerous statements blocked outright.
			DangerousAllowed: false,
		}
	}
	return Policy{Timeout: 10 * time.Second}
}
