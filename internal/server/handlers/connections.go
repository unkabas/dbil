package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/unkabas/dbil/internal/auth"
	"github.com/unkabas/dbil/internal/store"
)

type createConnReq struct {
	Alias      string `json:"alias"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Tag        string `json:"tag"`
	TLSMode    string `json:"tls_mode"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Database   string `json:"database"`
	Passphrase string `json:"passphrase,omitempty"`
	SSHHostID  *int64 `json:"ssh_host_id,omitempty"`
}

type connView struct {
	ID                 int64  `json:"id"`
	Alias              string `json:"alias"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Tag                string `json:"tag"`
	TLSMode            string `json:"tls_mode"`
	RequiresPassphrase bool   `json:"requires_passphrase"`
	SSHHostID          *int64 `json:"ssh_host_id,omitempty"`
	CreatedAt          int64  `json:"created_at"`
	UpdatedAt          int64  `json:"updated_at"`
}

func toView(c store.Connection) connView {
	return connView{
		ID: c.ID, Alias: c.Alias, Host: c.Host, Port: c.Port,
		Tag: c.Tag, TLSMode: c.TLSMode,
		RequiresPassphrase: c.RequiresPassphrase,
		SSHHostID:          c.SSHHostID,
		CreatedAt:          c.CreatedAt.Unix(),
		UpdatedAt:          c.UpdatedAt.Unix(),
	}
}

// ListConnections handles GET /api/connections. RequireAuth-gated.
func ListConnections(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conns, err := d.Conns.List(r.Context())
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		out := make([]connView, len(conns))
		for i, c := range conns {
			out[i] = toView(c)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

// CreateConnection handles POST /api/connections. RequireAuth-gated.
func CreateConnection(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createConnReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		c, err := d.Conns.Create(r.Context(), store.CreateConnectionParams{
			Alias:      req.Alias,
			Host:       req.Host,
			Port:       req.Port,
			Tag:        req.Tag,
			TLSMode:    req.TLSMode,
			Username:   req.Username,
			Password:   req.Password,
			Database:   req.Database,
			Passphrase: req.Passphrase,
			SSHHostID:  req.SSHHostID,
		})
		if err != nil {
			if errors.Is(err, store.ErrConnectionExists) {
				jsonError(w, http.StatusConflict, "connection alias already exists")
				return
			}
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		auditAppend(r, d, "connection.create", fmt.Sprintf("conn:%d", c.ID),
			map[string]any{"alias": c.Alias, "tag": c.Tag, "tls_mode": c.TLSMode})
		// Start observability collectors for connections that don't need a
		// per-session passphrase. Passphrase-protected ones wait until a user
		// explicitly opens them (Plan 6.1 will add a /start-collectors hook).
		if d.ObservMgr != nil && !c.RequiresPassphrase {
			d.ObservMgr.Start(c.ID, pollIntervalFor(c.Tag))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(toView(c))
	}
}

// GetConnection handles GET /api/connections/{id}. RequireAuth-gated.
func GetConnection(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		c, err := d.Conns.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrConnectionNotFound) {
				jsonError(w, http.StatusNotFound, "connection not found")
				return
			}
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(toView(c))
	}
}

// DeleteConnection handles DELETE /api/connections/{id}. RequireAuth-gated.
// Also drops the cached pool if any.
func DeleteConnection(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		if err := d.Conns.Delete(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrConnectionNotFound) {
				jsonError(w, http.StatusNotFound, "connection not found")
				return
			}
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if d.ObservMgr != nil {
			d.ObservMgr.Stop(id)
		}
		d.Manager.CloseConn(id)
		auditAppend(r, d, "connection.delete", fmt.Sprintf("conn:%d", id), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

// pollIntervalFor maps a connection tag to its collector tick cadence. Kept
// here (not imported from internal/policy) to avoid pulling the policy
// package into every handler test set; the values match policy.PolicyFor.
func pollIntervalFor(tag string) time.Duration {
	switch tag {
	case store.TagProduction:
		return 5 * time.Second
	case store.TagStaging:
		return 10 * time.Second
	case store.TagDev:
		return 30 * time.Second
	default:
		return 60 * time.Second
	}
}

// TestConnection handles POST /api/connections/{id}/test. RequireAuth-gated.
// Caller supplies an optional X-Connection-Passphrase header for
// passphrase-protected connections.
func TestConnection(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		passphrase := r.Header.Get("X-Connection-Passphrase")
		probe, err := d.Manager.Probe(r.Context(), id, passphrase)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrConnectionNotFound):
				jsonError(w, http.StatusNotFound, "connection not found")
			case errors.Is(err, store.ErrPassphraseRequired):
				jsonError(w, http.StatusPreconditionRequired, "passphrase required")
			case errors.Is(err, store.ErrInvalidPassphrase):
				jsonError(w, http.StatusUnauthorized, "invalid passphrase")
			default:
				jsonError(w, http.StatusBadGateway, "probe failed: "+err.Error())
			}
			return
		}
		auditAppend(r, d, "connection.test", fmt.Sprintf("conn:%d", id),
			map[string]any{"version": probe.Version, "has_pg_stat_statements": probe.HasPgStatStatements})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":                probe.Version,
			"superuser_ok":           probe.SuperuserOK,
			"has_pg_stat_statements": probe.HasPgStatStatements,
		})
	}
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

// auditAppend records an audit entry on the authed user's behalf. Audit
// failures are swallowed at the handler boundary — the spec section 12
// "Crypto failures" rule applies to the chain integrity check, not to a
// transient append failure that we don't want to expose as an API error.
func auditAppend(r *http.Request, d Deps, action, resource string, details map[string]any) {
	if d.Auth.Audit == nil {
		return
	}
	user, _ := auth.UserFromContext(r.Context())
	if details == nil {
		details = map[string]any{}
	}
	_, _ = d.Auth.Audit.Append(r.Context(), user.Email, action, resource, details)
}
