package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/unkabas/dbil/internal/auth"
	"github.com/unkabas/dbil/internal/postgres"
	"github.com/unkabas/dbil/internal/store"
)

type queryReq struct {
	SQL string `json:"sql"`
}

// QueryHandler handles POST /api/connections/{id}/query. RequireAuth-gated.
// Body: {"sql": "..."}; optional X-Connection-Passphrase header for
// passphrase-protected connections; optional "X-Confirm: yes" header for
// statements that require confirmation under the connection's tag policy.
func QueryHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var req queryReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if strings.TrimSpace(req.SQL) == "" {
			jsonError(w, http.StatusBadRequest, "sql is required")
			return
		}

		user, _ := auth.UserFromContext(r.Context())
		confirm := strings.EqualFold(r.Header.Get("X-Confirm"), "yes")

		res, err := d.Manager.Execute(r.Context(), postgres.ExecuteParams{
			ConnID:     id,
			Passphrase: r.Header.Get("X-Connection-Passphrase"),
			SQL:        req.SQL,
			Confirm:    confirm,
			UserEmail:  user.Email,
			ReadOnly:   !auth.CanWrite(user.Role),
		})
		if err != nil {
			respondExecuteError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"columns":       res.Columns,
			"rows":          res.Rows,
			"rows_affected": res.RowsAffected,
			"command_tag":   res.CommandTag,
			"duration_ms":   res.Duration.Milliseconds(),
			"truncated":     res.Truncated,
		})
	}
}

func respondExecuteError(w http.ResponseWriter, err error) {
	var be *postgres.BlockedError
	var ce *postgres.ConfirmationRequiredError
	switch {
	case errors.Is(err, store.ErrConnectionNotFound):
		jsonError(w, http.StatusNotFound, "connection not found")
	case errors.Is(err, store.ErrPassphraseRequired):
		jsonErrorWithReason(w, http.StatusPreconditionRequired, "passphrase required", "")
	case errors.Is(err, store.ErrInvalidPassphrase):
		jsonError(w, http.StatusUnauthorized, "invalid passphrase")
	case errors.As(err, &be):
		jsonErrorWithReason(w, http.StatusForbidden, "blocked by policy", be.Reason)
	case errors.As(err, &ce):
		jsonErrorWithReason(w, http.StatusPreconditionRequired, "confirmation required", ce.Reason)
	case errors.Is(err, context.DeadlineExceeded):
		jsonError(w, http.StatusGatewayTimeout, "statement timeout")
	default:
		jsonError(w, http.StatusBadGateway, "execute failed: "+err.Error())
	}
}

func jsonErrorWithReason(w http.ResponseWriter, status int, msg, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]string{"error": msg}
	if reason != "" {
		resp["reason"] = reason
	}
	_ = json.NewEncoder(w).Encode(resp)
}
