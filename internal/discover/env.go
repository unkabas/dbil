package discover

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// envEntry matches the shape DBIL_AUTO_CONNECT users provide. Tolerant
// of both `user` and `username` to match common compose habits.
type envEntry struct {
	Alias    string `json:"alias"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
	Tag      string `json:"tag"`
}

var validTags = map[string]struct{}{
	"local":      {},
	"dev":        {},
	"staging":    {},
	"production": {},
}

// ParseEnvJSON parses the DBIL_AUTO_CONNECT env var content into Entries.
// Empty input returns (nil, nil). Each entry is validated; the first
// failure aborts parsing so misconfigured drop-ins never silently skip
// rows.
func ParseEnvJSON(raw string) ([]Entry, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var raws []envEntry
	if err := json.Unmarshal([]byte(raw), &raws); err != nil {
		return nil, fmt.Errorf("DBIL_AUTO_CONNECT: %w", err)
	}
	out := make([]Entry, 0, len(raws))
	for i, r := range raws {
		e, err := toEntry(r)
		if err != nil {
			return nil, fmt.Errorf("DBIL_AUTO_CONNECT[%d]: %w", i, err)
		}
		out = append(out, e)
	}
	return out, nil
}

func toEntry(r envEntry) (Entry, error) {
	if r.Alias == "" {
		return Entry{}, fmt.Errorf("alias is required")
	}
	if r.Host == "" {
		return Entry{}, fmt.Errorf("host is required")
	}
	if r.Port <= 0 || r.Port > 65535 {
		return Entry{}, fmt.Errorf("port out of range (got %d)", r.Port)
	}
	user := r.User
	if user == "" {
		user = r.Username
	}
	if user == "" {
		return Entry{}, fmt.Errorf("user is required")
	}
	if r.Database == "" {
		return Entry{}, fmt.Errorf("database is required")
	}
	tag := r.Tag
	if tag == "" {
		tag = "dev"
	}
	if _, ok := validTags[tag]; !ok {
		return Entry{}, fmt.Errorf("invalid tag %q", tag)
	}
	return Entry{
		Source:   SourceEnv,
		Key:      envKey(r.Alias, r.Host, r.Port, r.Database, user),
		Alias:    r.Alias,
		Host:     r.Host,
		Port:     r.Port,
		Database: r.Database,
		Username: user,
		Password: r.Password,
		Tag:      tag,
	}, nil
}

// envKey is a stable 16-hex-char digest of the identifying tuple. We
// intentionally do not include the password — rotating credentials in
// the env shouldn't create a duplicate "pending" row.
func envKey(alias, host string, port int, database, user string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s|%s", alias, host, port, database, user)))
	return hex.EncodeToString(h[:8])
}
