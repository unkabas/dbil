package discover

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/unkabas/dbil/internal/store"
)

// Mode controls which scanners are wired by Manager.
type Mode string

const (
	ModeOff    Mode = ""
	ModeEnv    Mode = "env"
	ModeDocker Mode = "docker"
	ModeBoth   Mode = "both"
)

// Config captures the env-var-derived settings used at boot time.
type Config struct {
	Mode            Mode
	AutoConnectJSON string // raw DBIL_AUTO_CONNECT value
	Network         string // DBIL_NETWORK
}

// AuditAppender is the slice of *store.AuditRepo discover needs. Kept as an
// interface so tests can supply a no-op recorder.
type AuditAppender interface {
	Append(ctx context.Context, userID, action, resource string, details map[string]any) (store.AppendResult, error)
}

// Scanner is one source of candidates (env or docker).
type Scanner interface {
	Source() Source
	Scan(ctx context.Context) ([]Entry, error)
}

// envScanner adapts ParseEnvJSON to the Scanner interface so the manager
// treats env and docker uniformly. The JSON is re-parsed each tick so
// changing DBIL_AUTO_CONNECT requires a restart (env vars are read once at
// boot anyway, but rescanning is cheap).
type envScanner struct {
	raw string
}

func (s *envScanner) Source() Source { return SourceEnv }
func (s *envScanner) Scan(_ context.Context) ([]Entry, error) {
	return ParseEnvJSON(s.raw)
}

// dockerScannerAdapter exposes DockerScanner under the Scanner interface.
type dockerScannerAdapter struct{ *DockerScanner }

func (d *dockerScannerAdapter) Source() Source { return SourceDocker }

// Manager runs each scanner on a 30 s tick, upserts results into the repo,
// transitions disappearing entries to unreachable, and writes audit lines
// for newly detected items.
type Manager struct {
	scanners []Scanner
	repo     *store.DiscoveredRepo
	audit    AuditAppender
	log      *slog.Logger
	interval time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewManager wires a Manager with the env scanner. Callers add the docker
// scanner separately (it needs an external client built by serve_cmd).
func NewManager(cfg Config, repo *store.DiscoveredRepo, audit AuditAppender, log *slog.Logger) (*Manager, error) {
	if repo == nil {
		return nil, errors.New("discover.NewManager: repo is required")
	}
	if log == nil {
		log = slog.Default()
	}
	m := &Manager{repo: repo, audit: audit, log: log, interval: 30 * time.Second}
	switch cfg.Mode {
	case ModeOff:
	case ModeEnv, ModeBoth:
		m.scanners = append(m.scanners, &envScanner{raw: cfg.AutoConnectJSON})
	case ModeDocker:
		// Docker-only; the docker scanner is added via AddDockerScanner.
	default:
		return nil, fmt.Errorf("discover: unknown mode %q", cfg.Mode)
	}
	return m, nil
}

// AddDockerScanner appends a docker scanner backed by client. No-op when
// client is nil (covers the missing-socket degraded path).
func (m *Manager) AddDockerScanner(client dockerLister, network string) {
	if client == nil {
		return
	}
	m.scanners = append(m.scanners, &dockerScannerAdapter{
		DockerScanner: &DockerScanner{Client: client, Network: network, Log: m.log},
	})
}

// SetScanners replaces the scanner set (used by tests that need full control).
func (m *Manager) SetScanners(scanners ...Scanner) {
	m.scanners = scanners
}

// SetInterval overrides the default 30 s tick (used by tests).
func (m *Manager) SetInterval(d time.Duration) {
	if d > 0 {
		m.interval = d
	}
}

// RunOnce executes every scanner exactly once and persists the results.
// Returns the total entries persisted (new + refreshed).
func (m *Manager) RunOnce(ctx context.Context) (int, error) {
	persisted := 0
	for _, s := range m.scanners {
		entries, err := s.Scan(ctx)
		if err != nil {
			m.log.Warn("discover.scan_failed", "source", string(s.Source()), "err", err.Error())
			continue
		}
		keys := make([]string, 0, len(entries))
		for _, e := range entries {
			keys = append(keys, e.Key)
			d, isNew, err := m.repo.Upsert(ctx, store.DiscoveredUpsert{
				Source:    string(e.Source),
				SourceKey: e.Key,
				Alias:     e.Alias,
				Host:      e.Host,
				Port:      e.Port,
				Database:  e.Database,
				Username:  e.Username,
				Password:  e.Password,
				Tag:       e.Tag,
			})
			if err != nil {
				m.log.Warn("discover.upsert_failed",
					"source", string(e.Source), "key", e.Key, "err", err.Error())
				continue
			}
			persisted++
			if isNew && m.audit != nil {
				_, _ = m.audit.Append(ctx, "dbil:discover", "discover.detected",
					fmt.Sprintf("discovered:%d", d.ID), map[string]any{
						"alias":  e.Alias,
						"source": string(e.Source),
						"tag":    e.Tag,
					})
			}
		}
		if err := m.repo.MarkUnreachable(ctx, string(s.Source()), keys); err != nil {
			m.log.Warn("discover.mark_unreachable_failed", "source", string(s.Source()), "err", err.Error())
		}
	}
	return persisted, nil
}

// Start runs RunOnce immediately, then on every interval tick. Calling Start
// twice is a no-op.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	m.cancel = cancel
	m.done = done
	m.mu.Unlock()

	go m.loop(ctx, done)
}

func (m *Manager) loop(ctx context.Context, done chan struct{}) {
	defer close(done)
	m.tickOnce(ctx)
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.tickOnce(ctx)
		}
	}
}

func (m *Manager) tickOnce(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			m.log.Error("discover.tick_panic", "panic", fmt.Sprint(r))
		}
	}()
	tickCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if _, err := m.RunOnce(tickCtx); err != nil {
		m.log.Warn("discover.tick_err", "err", err.Error())
	}
}

// Shutdown stops the tick loop and waits for the in-flight tick to finish.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	cancel := m.cancel
	done := m.done
	m.cancel = nil
	m.done = nil
	m.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}

