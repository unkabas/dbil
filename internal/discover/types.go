// Package discover scans for Postgres services to surface in the UI for
// user approval. Two scanners are wired in v0.8.0:
//
//   - env: parses DBIL_AUTO_CONNECT JSON for declarative drop-ins.
//   - docker: lists containers via the Docker engine API, filters by
//     dbil.* labels, and resolves credentials from target container env.
//
// The manager hands all candidates to store.DiscoveredRepo where each
// entry waits for explicit user approval before becoming a real
// connection. That approval gate is the security boundary that prevents
// a rogue container from joining the connection list silently.
package discover

// Source enumerates where a candidate came from. Persisted as the
// `source` column of discovered_connections.
type Source string

const (
	SourceEnv    Source = "env"
	SourceDocker Source = "docker"
)

// Entry is the engine-neutral candidate. The manager translates it into
// a store.DiscoveredUpsert before persisting.
type Entry struct {
	Source   Source
	Key      string // stable per scanner: env keys hash inputs; docker uses container id
	Alias    string
	Host     string
	Port     int
	Database string
	Username string
	Password string
	Tag      string
}
