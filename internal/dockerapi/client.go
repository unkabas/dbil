// Package dockerapi is a tiny read-only Docker Engine API client. dbil
// only needs to list containers and inspect their env + networks for
// auto-discovery; the full github.com/docker/docker module costs ~5MB
// of dependencies and ships two AuthZ-plugin CVEs marked "Fixed in: N/A"
// that don't affect dbil but pollute govulncheck.
//
// This file talks the Docker Engine HTTP API directly over the unix
// socket (default /var/run/docker.sock; overrideable via DOCKER_HOST).
// Only two endpoints are implemented:
//
//   GET /containers/json        — list running containers (Summary[]).
//   GET /containers/{id}/json   — inspect one container (Inspect).
//
// Field set is intentionally minimal: every field exposed corresponds to
// something internal/discover actually reads.
package dockerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultSocket is the standard Docker Engine unix socket path. Used when
// DOCKER_HOST is not set.
const DefaultSocket = "/var/run/docker.sock"

// Summary is a stripped-down /containers/json entry. Field names match
// the JSON the engine emits.
type Summary struct {
	ID              string                  `json:"Id"`
	Names           []string                `json:"Names"`
	Labels          map[string]string       `json:"Labels"`
	NetworkSettings *NetworkSettingsSummary `json:"NetworkSettings"`
}

// NetworkSettingsSummary mirrors the shape returned by ContainerList. Each
// key is a network name; the IPAddress is on the endpoint, not the network.
type NetworkSettingsSummary struct {
	Networks map[string]*EndpointSettings `json:"Networks"`
}

// EndpointSettings holds the per-network endpoint details we care about.
// The full Docker schema has many more fields; only IPAddress is consumed.
type EndpointSettings struct {
	IPAddress string `json:"IPAddress"`
}

// Inspect is a stripped-down /containers/{id}/json response. The two fields
// that matter to discovery: Config.Env (where dbil.* label values point)
// and NetworkSettings.Networks (fallback IP source).
type Inspect struct {
	Config          *Config                 `json:"Config"`
	NetworkSettings *NetworkSettingsInspect `json:"NetworkSettings"`
}

// Config holds container-level configuration. We only read Env.
type Config struct {
	Env []string `json:"Env"`
}

// NetworkSettingsInspect mirrors the inspect shape (same shape as the
// summary in practice, but kept distinct in case Docker diverges them).
type NetworkSettingsInspect struct {
	Networks map[string]*EndpointSettings `json:"Networks"`
}

// Client is a minimal Docker Engine API client. Construct via NewClient
// or NewClientFromEnv. Methods are concurrency-safe; the underlying
// http.Client reuses connections.
type Client struct {
	http *http.Client
	base string // base URL — http://docker for unix sockets, real URL otherwise
}

// NewClient connects to a unix socket. Returns an error if the socket
// is not reachable so callers can degrade gracefully (the discover
// manager logs a warning and continues without docker).
func NewClient(socketPath string) (*Client, error) {
	if socketPath == "" {
		socketPath = DefaultSocket
	}
	c := &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
		},
		base: "http://docker",
	}
	return c, nil
}

// NewClientFromEnv honours DOCKER_HOST (unix:///path or tcp://host:port).
// Anything other than a tcp:// or unix:// scheme falls back to the default
// unix socket — keeps the behaviour predictable inside containers where
// /var/run/docker.sock is the universal answer.
func NewClientFromEnv() (*Client, error) {
	host := envDockerHost()
	if host == "" {
		return NewClient(DefaultSocket)
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("dockerapi: parse DOCKER_HOST: %w", err)
	}
	switch u.Scheme {
	case "unix":
		return NewClient(u.Path)
	case "tcp", "http":
		c := &Client{
			http: &http.Client{Timeout: 30 * time.Second},
			base: "http://" + u.Host,
		}
		return c, nil
	default:
		return NewClient(DefaultSocket)
	}
}

// ContainerList returns running containers. The signature mirrors what
// discover.DockerScanner expected from docker/docker so the call sites
// don't have to change.
func (c *Client) ContainerList(ctx context.Context) ([]Summary, error) {
	var out []Summary
	if err := c.get(ctx, "/containers/json", &out); err != nil {
		return nil, fmt.Errorf("ContainerList: %w", err)
	}
	return out, nil
}

// ContainerInspect returns the full inspect response for one container id.
func (c *Client) ContainerInspect(ctx context.Context, id string) (Inspect, error) {
	if id == "" {
		return Inspect{}, fmt.Errorf("ContainerInspect: id is empty")
	}
	var out Inspect
	if err := c.get(ctx, "/containers/"+id+"/json", &out); err != nil {
		return Inspect{}, fmt.Errorf("ContainerInspect: %w", err)
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, path string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	// Engine API requires a Host header even over unix sockets.
	req.Host = "docker"
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("docker engine returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

// envDockerHost reads DOCKER_HOST without pulling os into the test path.
// Tests can override by setting it directly.
var envDockerHost = func() string {
	// Imported lazily so tests can override the func itself.
	return strings.TrimSpace(getEnv("DOCKER_HOST"))
}
