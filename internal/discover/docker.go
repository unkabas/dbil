package discover

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// dockerLister is the minimal slice of the Docker client used by DockerScanner.
// Keeping this small lets tests provide a fake without spinning up dockerd.
type dockerLister interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerInspect(ctx context.Context, id string) (container.InspectResponse, error)
}

// Labels recognised on Postgres-bearing containers. All under the `dbil.`
// namespace per spec section 7.3.
const (
	LabelEnable      = "dbil.enable"
	LabelAlias       = "dbil.alias"
	LabelTag         = "dbil.tag"
	LabelPort        = "dbil.port"
	LabelUsernameEnv = "dbil.creds.username_env"
	LabelPasswordEnv = "dbil.creds.password_env"
	LabelDatabaseEnv = "dbil.creds.database_env"
)

// DockerScanner reads the running-container set and produces discover Entries
// for those that opt in via labels.
type DockerScanner struct {
	Client  dockerLister
	Network string // when set, only containers attached to this network are considered
	Log     *slog.Logger
}

// Scan returns one Entry per opted-in container that resolved successfully.
// Skipped containers are logged at debug level with a reason; never errored
// on so a single misconfigured container can't poison the whole scan.
func (s *DockerScanner) Scan(ctx context.Context) ([]Entry, error) {
	list, err := s.Client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("docker list: %w", err)
	}
	out := make([]Entry, 0, len(list))
	for _, c := range list {
		if c.Labels[LabelEnable] != "true" {
			continue
		}
		if s.Network != "" && !attachedTo(c, s.Network) {
			s.skip(c, "not attached to network "+s.Network)
			continue
		}
		insp, err := s.Client.ContainerInspect(ctx, c.ID)
		if err != nil {
			s.skip(c, "inspect failed: "+err.Error())
			continue
		}
		entry, err := s.buildEntry(c, insp)
		if err != nil {
			s.skip(c, err.Error())
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func (s *DockerScanner) skip(c container.Summary, reason string) {
	if s.Log == nil {
		return
	}
	s.Log.Debug("discover.skip", "container_id", c.ID, "labels_alias", c.Labels[LabelAlias], "reason", reason)
}

func attachedTo(c container.Summary, network string) bool {
	if c.NetworkSettings == nil {
		return false
	}
	_, ok := c.NetworkSettings.Networks[network]
	return ok
}

func (s *DockerScanner) buildEntry(c container.Summary, insp container.InspectResponse) (Entry, error) {
	if insp.Config == nil {
		return Entry{}, fmt.Errorf("inspect returned no config")
	}
	env := parseEnv(insp.Config.Env)

	userVar := c.Labels[LabelUsernameEnv]
	if userVar == "" {
		return Entry{}, fmt.Errorf("missing label %s", LabelUsernameEnv)
	}
	user, ok := env[userVar]
	if !ok || user == "" {
		return Entry{}, fmt.Errorf("env var %s not set on target container", userVar)
	}

	dbVar := c.Labels[LabelDatabaseEnv]
	if dbVar == "" {
		return Entry{}, fmt.Errorf("missing label %s", LabelDatabaseEnv)
	}
	db, ok := env[dbVar]
	if !ok || db == "" {
		return Entry{}, fmt.Errorf("env var %s not set on target container", dbVar)
	}

	// Password may legitimately be empty (passwordless local dev).
	var password string
	if pwVar := c.Labels[LabelPasswordEnv]; pwVar != "" {
		password = env[pwVar]
	}

	alias := c.Labels[LabelAlias]
	if alias == "" {
		alias = primaryName(c)
	}

	tag := c.Labels[LabelTag]
	if tag == "" {
		tag = "dev"
	}
	if _, ok := validTags[tag]; !ok {
		return Entry{}, fmt.Errorf("invalid tag %q", tag)
	}

	port := 5432
	if raw := c.Labels[LabelPort]; raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 || n > 65535 {
			return Entry{}, fmt.Errorf("invalid %s=%q", LabelPort, raw)
		}
		port = n
	}

	host := resolveHost(c, insp, s.Network)
	if host == "" {
		return Entry{}, fmt.Errorf("could not resolve host for container")
	}

	return Entry{
		Source:   SourceDocker,
		Key:      dockerKey(c.ID),
		Alias:    alias,
		Host:     host,
		Port:     port,
		Database: db,
		Username: user,
		Password: password,
		Tag:      tag,
	}, nil
}

// resolveHost prefers DNS-style names (works on user-defined networks);
// falls back to the IP address on the chosen network.
func resolveHost(c container.Summary, insp container.InspectResponse, network string) string {
	name := primaryName(c)
	if network != "" && c.NetworkSettings != nil {
		if ep, ok := c.NetworkSettings.Networks[network]; ok && ep != nil {
			if ep.IPAddress != "" && name == "" {
				return ep.IPAddress
			}
		}
	}
	if name != "" {
		return name
	}
	if insp.NetworkSettings != nil {
		for _, ep := range insp.NetworkSettings.Networks {
			if ep != nil && ep.IPAddress != "" {
				return ep.IPAddress
			}
		}
	}
	return ""
}

// primaryName returns the container's first registered name, with the
// leading "/" stripped that Docker prepends.
func primaryName(c container.Summary) string {
	if len(c.Names) == 0 {
		return ""
	}
	return strings.TrimPrefix(c.Names[0], "/")
}

// dockerKey is "docker:" + the first 16 chars of the container id — stable
// across restarts of dbil but changes when the user recreates the container.
func dockerKey(id string) string {
	if len(id) > 16 {
		return id[:16]
	}
	return id
}

func parseEnv(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, e := range env {
		i := strings.IndexByte(e, '=')
		if i <= 0 {
			continue
		}
		out[e[:i]] = e[i+1:]
	}
	return out
}
