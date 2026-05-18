package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/unkabas/dbil/internal/store"
)

type discoverEntryView struct {
	ID             int64  `json:"id"`
	Source         string `json:"source"`
	SourceKey      string `json:"source_key"`
	Alias          string `json:"alias"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Database       string `json:"database"`
	Username       string `json:"username"`
	HasPassword    bool   `json:"has_password"`
	Tag            string `json:"tag"`
	Status         string `json:"status"`
	LastSeenMs     int64  `json:"last_seen_ms"`
	CreatedAtMs    int64  `json:"created_at_ms"`
	ApprovedConnID *int64 `json:"approved_conn_id,omitempty"`
}

func toDiscoverView(d store.Discovered) discoverEntryView {
	v := discoverEntryView{
		ID:          d.ID,
		Source:      d.Source,
		SourceKey:   d.SourceKey,
		Alias:       d.Alias,
		Host:        d.Host,
		Port:        d.Port,
		Database:    d.Database,
		Username:    d.Username,
		HasPassword: d.HasPassword,
		Tag:         d.Tag,
		Status:      d.Status,
		LastSeenMs:  d.LastSeen.UnixMilli(),
		CreatedAtMs: d.CreatedAt.UnixMilli(),
	}
	if d.ApprovedConnID.Valid {
		id := d.ApprovedConnID.Int64
		v.ApprovedConnID = &id
	}
	return v
}

// ListDiscoverHandler — GET /api/discover
//   Returns every discovered_connections row, newest first.
func ListDiscoverHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Discovered == nil {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"entries":[]}`))
			return
		}
		entries, err := d.Discovered.List(r.Context())
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		out := make([]discoverEntryView, len(entries))
		for i, e := range entries {
			out[i] = toDiscoverView(e)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"entries": out})
	}
}

type approveRequest struct {
	Passphrase string `json:"passphrase"`
}

type approveResponse struct {
	ConnectionID int64 `json:"connection_id"`
}

// ApproveDiscoverHandler — POST /api/discover/{id}/approve
//   Body: {"passphrase": "..."} — required when entry.tag == "production".
//   Creates the real connection via ConnectionsRepo.Create, marks the
//   discovered row approved, and writes a discover.approved audit entry.
func ApproveDiscoverHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Discovered == nil {
			jsonError(w, http.StatusServiceUnavailable, "discovery is disabled")
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}

		var req approveRequest
		if r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, http.StatusBadRequest, "invalid request body")
				return
			}
		}

		entry, err := d.Discovered.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrDiscoveredNotFound) {
				jsonError(w, http.StatusNotFound, "discovered entry not found")
				return
			}
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if entry.Status == store.DiscoverStatusApproved {
			jsonError(w, http.StatusConflict, "entry already approved")
			return
		}
		if entry.Status == store.DiscoverStatusRejected {
			jsonError(w, http.StatusConflict, "entry was rejected")
			return
		}
		if entry.Tag == store.TagProduction && req.Passphrase == "" {
			jsonErrorWithReason(w, http.StatusPreconditionRequired,
				"passphrase required for production-tagged connections", "")
			return
		}

		password, err := d.Discovered.RevealPassword(r.Context(), id)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to reveal password")
			return
		}

		conn, err := d.Conns.Create(r.Context(), store.CreateConnectionParams{
			Alias:      entry.Alias,
			Host:       entry.Host,
			Port:       entry.Port,
			Tag:        entry.Tag,
			TLSMode:    store.TLSDisable,
			Username:   entry.Username,
			Password:   password,
			Database:   entry.Database,
			Passphrase: req.Passphrase,
		})
		if err != nil {
			if errors.Is(err, store.ErrConnectionExists) {
				jsonError(w, http.StatusConflict, "connection alias already exists")
				return
			}
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := d.Discovered.MarkApproved(r.Context(), id, conn.ID); err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to mark approved")
			return
		}
		auditAppend(r, d, "discover.approved", fmt.Sprintf("discovered:%d", id), map[string]any{
			"alias":         entry.Alias,
			"source":        entry.Source,
			"tag":           entry.Tag,
			"connection_id": conn.ID,
		})

		// Auto-start observability collectors for the new connection when it
		// isn't passphrase-protected (matches the connections.Create flow).
		if d.ObservMgr != nil && !conn.RequiresPassphrase {
			d.ObservMgr.Start(conn.ID, pollIntervalFor(conn.Tag))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(approveResponse{ConnectionID: conn.ID})
	}
}

// RejectDiscoverHandler — POST /api/discover/{id}/reject
//   Marks the row rejected. Subsequent scans don't resurrect it.
func RejectDiscoverHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Discovered == nil {
			jsonError(w, http.StatusServiceUnavailable, "discovery is disabled")
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		entry, err := d.Discovered.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrDiscoveredNotFound) {
				jsonError(w, http.StatusNotFound, "discovered entry not found")
				return
			}
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if err := d.Discovered.Reject(r.Context(), id); err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		auditAppend(r, d, "discover.rejected", fmt.Sprintf("discovered:%d", id), map[string]any{
			"alias":  entry.Alias,
			"source": entry.Source,
			"tag":    entry.Tag,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
