package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/unkabas/dbil/internal/auth"
	"github.com/unkabas/dbil/internal/pg"
	"github.com/unkabas/dbil/internal/postgres"
)

type mutateReq struct {
	Changes []pg.RowChange `json:"changes"`
}

// MutateTableHandler — POST /api/connections/{id}/table/{schema}/{name}/mutations
//
// Applies a batch of typed cell edits (update / delete / insert) to a table
// that has a primary key. The backend introspects the PK, builds PK-scoped SQL
// (never trusting client-supplied SQL), and runs the batch atomically through
// the connection's tag policy and audit chain. Mounted under RequireRole
// (admin, member); viewer never reaches here. Honors X-Confirm and
// X-Connection-Passphrase like the SQL editor.
func MutateTableHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, schema, name, ok := parseTableRoute(w, r)
		if !ok {
			return
		}
		var req mutateReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(req.Changes) == 0 {
			jsonError(w, http.StatusBadRequest, "at least one change is required")
			return
		}

		passphrase := r.Header.Get("X-Connection-Passphrase")
		pool, err := d.Manager.OpenByID(r.Context(), id, passphrase)
		if err != nil {
			respondPoolOpenError(w, err)
			return
		}

		shape, err := pg.IntrospectTable(r.Context(), pool, schema, name)
		if err != nil {
			switch {
			case errors.Is(err, pg.ErrNoPrimaryKey):
				jsonError(w, http.StatusUnprocessableEntity, "table has no primary key, editing disabled")
			case errors.Is(err, pg.ErrInvalidIdentifier):
				jsonError(w, http.StatusBadRequest, "invalid identifier")
			default:
				jsonError(w, http.StatusBadGateway, "introspect table failed: "+err.Error())
			}
			return
		}

		stmts, err := pg.BuildMutations(schema, name, shape, req.Changes)
		if err != nil {
			switch {
			case errors.Is(err, pg.ErrUnknownColumn),
				errors.Is(err, pg.ErrInvalidChange),
				errors.Is(err, pg.ErrInvalidIdentifier):
				jsonError(w, http.StatusBadRequest, err.Error())
			default:
				jsonError(w, http.StatusBadRequest, "invalid changes: "+err.Error())
			}
			return
		}

		user, _ := auth.UserFromContext(r.Context())
		confirm := strings.EqualFold(r.Header.Get("X-Confirm"), "yes")

		res, err := d.Manager.ExecuteBatch(r.Context(), postgres.BatchParams{
			ConnID:     id,
			Passphrase: passphrase,
			Stmts:      stmts,
			Confirm:    confirm,
			UserEmail:  user.Email,
			ReadOnly:   !auth.CanWrite(user.Role),
		})
		if err != nil {
			respondExecuteError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"rows_affected": res.RowsAffected,
			"statements":    res.Statements,
		})
	}
}
