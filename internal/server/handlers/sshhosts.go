package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/unkabas/dbil/internal/store"
)

type sshHostView struct {
	ID                 int64  `json:"id"`
	Alias              string `json:"alias"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	AuthMethod         string `json:"auth_method"`
	HostKeyFingerprint string `json:"host_key_fingerprint,omitempty"`
	RequiresPassphrase bool   `json:"requires_passphrase"`
	CreatedAt          int64  `json:"created_at"`
	UpdatedAt          int64  `json:"updated_at"`
}

func toSSHView(h store.SSHHost) sshHostView {
	return sshHostView{
		ID: h.ID, Alias: h.Alias, Host: h.Host, Port: h.Port, Username: h.Username,
		AuthMethod: h.AuthMethod, HostKeyFingerprint: h.HostKeyFingerprint,
		RequiresPassphrase: h.RequiresPassphrase,
		CreatedAt:          h.CreatedAt.Unix(),
		UpdatedAt:          h.UpdatedAt.Unix(),
	}
}

// ListSSHHosts — GET /api/ssh-hosts. Writers only.
func ListSSHHosts(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hosts, err := d.SSHHosts.List(r.Context())
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "internal error")
			return
		}
		out := make([]sshHostView, len(hosts))
		for i, h := range hosts {
			out[i] = toSSHView(h)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type createSSHHostReq struct {
	Alias         string `json:"alias"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	AuthMethod    string `json:"auth_method"`
	Secret        string `json:"secret"`         // private key PEM or password
	KeyPassphrase string `json:"key_passphrase"` // optional, for an encrypted private key
	Passphrase    string `json:"passphrase"`     // optional, wraps the secret at rest
}

// CreateSSHHost — POST /api/ssh-hosts. Writers only.
func CreateSSHHost(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createSSHHostReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		h, err := d.SSHHosts.Create(r.Context(), store.CreateSSHHostParams{
			Alias:         strings.TrimSpace(req.Alias),
			Host:          strings.TrimSpace(req.Host),
			Port:          req.Port,
			Username:      strings.TrimSpace(req.Username),
			AuthMethod:    req.AuthMethod,
			Secret:        req.Secret,
			KeyPassphrase: req.KeyPassphrase,
			Passphrase:    req.Passphrase,
		})
		if err != nil {
			if errors.Is(err, store.ErrSSHHostExists) {
				jsonError(w, http.StatusConflict, "ssh host alias already exists")
				return
			}
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		auditAppend(r, d, "sshhost.create", fmt.Sprintf("sshhost:%d", h.ID), map[string]any{
			"alias": h.Alias, "auth_method": h.AuthMethod,
		})
		writeJSON(w, http.StatusCreated, toSSHView(h))
	}
}

// DeleteSSHHost — DELETE /api/ssh-hosts/{id}. Writers only.
func DeleteSSHHost(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		if err := d.SSHHosts.Delete(r.Context(), id); err != nil {
			switch {
			case errors.Is(err, store.ErrSSHHostNotFound):
				jsonError(w, http.StatusNotFound, "ssh host not found")
			case errors.Is(err, store.ErrSSHHostReferenced):
				jsonError(w, http.StatusConflict, "ssh host is in use by a connection")
			default:
				jsonError(w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		auditAppend(r, d, "sshhost.delete", fmt.Sprintf("sshhost:%d", id), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

// TestSSHHost — POST /api/ssh-hosts/{id}/test. Writers only. Opens the tunnel
// (only) to verify reachability + auth and pins the host key on first connect.
func TestSSHHost(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		passphrase := r.Header.Get("X-Connection-Passphrase")
		fp, err := d.Manager.TestSSHHost(r.Context(), id, passphrase)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrSSHHostNotFound):
				jsonError(w, http.StatusNotFound, "ssh host not found")
			case errors.Is(err, store.ErrSSHPassphraseRequired):
				jsonError(w, http.StatusPreconditionRequired, "passphrase required")
			case errors.Is(err, store.ErrSSHInvalidPassphrase):
				jsonError(w, http.StatusUnauthorized, "invalid passphrase")
			default:
				jsonError(w, http.StatusBadGateway, "ssh tunnel failed: "+err.Error())
			}
			return
		}
		auditAppend(r, d, "sshhost.test", fmt.Sprintf("sshhost:%d", id),
			map[string]any{"fingerprint": fp})
		writeJSON(w, http.StatusOK, map[string]any{
			"reachable":            true,
			"host_key_fingerprint": fp,
		})
	}
}
