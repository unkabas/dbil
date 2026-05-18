package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/unkabas/dbil/internal/observ"
	"github.com/unkabas/dbil/internal/policy"
	"github.com/unkabas/dbil/internal/store"
)

// TerminateBackendHandler — POST /api/connections/{id}/locks/{pid}/terminate
//
// Sends a pg_terminate_backend(pid) to the connection's pool. Honours
// per-tag policy: connections tagged staging or production require an
// X-Confirm: yes header. Writes a query.terminate audit entry with the
// pid + result (or query.terminate.failed on driver error).
func TerminateBackendHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		pid, err := strconv.Atoi(chi.URLParam(r, "pid"))
		if err != nil || pid <= 0 {
			jsonError(w, http.StatusBadRequest, "invalid pid")
			return
		}

		conn, err := d.Conns.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrConnectionNotFound) {
				jsonError(w, http.StatusNotFound, "connection not found")
				return
			}
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Match the query handler's confirmation pattern: tags that require
		// confirm for dangerous statements also require it to kill a backend.
		pol := policy.PolicyFor(conn.Tag)
		confirm := strings.EqualFold(r.Header.Get("X-Confirm"), "yes")
		if pol.DangerousRequiresConfirm && !confirm {
			jsonErrorWithReason(w, http.StatusPreconditionRequired,
				"X-Confirm: yes required to terminate backends on tag "+conn.Tag, "")
			return
		}

		passphrase := r.Header.Get("X-Connection-Passphrase")
		pool, err := d.Manager.OpenByID(r.Context(), id, passphrase)
		if err != nil {
			respondPoolOpenError(w, err)
			return
		}

		signalled, err := observ.TerminateBackend(r.Context(), pool, pid)
		if err != nil {
			auditAppend(r, d, "query.terminate.failed", fmt.Sprintf("conn:%d", id), map[string]any{
				"alias": conn.Alias,
				"tag":   conn.Tag,
				"pid":   pid,
				"error": err.Error(),
			})
			jsonError(w, http.StatusBadGateway, "terminate failed: "+err.Error())
			return
		}
		auditAppend(r, d, "query.terminate", fmt.Sprintf("conn:%d", id), map[string]any{
			"alias":     conn.Alias,
			"tag":       conn.Tag,
			"pid":       pid,
			"signalled": signalled,
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"signalled": signalled, "pid": pid})
	}
}
