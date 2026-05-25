package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/unkabas/dbil/internal/auth"
	"github.com/unkabas/dbil/internal/bootstrap"
	"github.com/unkabas/dbil/internal/config"
	"github.com/unkabas/dbil/internal/discover"
	"github.com/unkabas/dbil/internal/dockerapi"
	"github.com/unkabas/dbil/internal/observ"
	"github.com/unkabas/dbil/internal/policy"
	"github.com/unkabas/dbil/internal/postgres"
	"github.com/unkabas/dbil/internal/server"
	"github.com/unkabas/dbil/internal/server/handlers"
	"github.com/unkabas/dbil/internal/store"
)

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the DBil HTTP server, bootstrapping empty data dirs automatically",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := prepareContainerRuntime(cfg); err != nil {
				return err
			}

			if _, err := bootstrap.RunInit(ctx, cfg); err != nil {
				return fmt.Errorf("serve: bootstrap: %w", err)
			}

			mk, src, err := bootstrap.LoadMasterKey(ctx, cfg)
			if err != nil {
				return fmt.Errorf("serve: load master key: %w", err)
			}
			defer mk.Wipe()
			slog.Info("master key loaded", "source", string(src))

			db, err := store.Open(filepath.Join(cfg.DataDir, "dbil.db"))
			if err != nil {
				return fmt.Errorf("serve: open db: %w", err)
			}
			defer store.Close(db)

			if err := store.Apply(db); err != nil {
				return fmt.Errorf("serve: apply migrations: %w", err)
			}

			auditRepo := store.NewAuditRepo(db, mk)
			authDeps := auth.Deps{
				Users:    store.NewUsersRepo(db),
				Sessions: store.NewSessionsRepo(db),
				Audit:    auditRepo,
			}
			conns := store.NewConnectionsRepo(db, mk)
			pgxMgr := postgres.NewManager(postgres.NewPGX(), conns, auditRepo)
			observRepo := store.NewObservabilityRepo(db)
			discoveredRepo := store.NewDiscoveredRepo(db, mk)

			observMgr := observ.NewManager(
				observRepo,
				func(ctx context.Context, id int64) (postgres.Pool, error) {
					return pgxMgr.OpenByID(ctx, id, "")
				},
				observ.DefaultFactory,
			)

			existing, err := conns.List(ctx)
			if err != nil {
				slog.Warn("serve: listing existing connections failed", "err", err)
			}
			for _, c := range existing {
				if c.RequiresPassphrase {
					slog.Info("observ: skipping collectors for passphrase-protected connection",
						"alias", c.Alias)
					continue
				}
				observMgr.Start(c.ID, policy.PolicyFor(c.Tag).PollInterval)
			}

			dscMgr := buildDiscoverManager(discoveredRepo, auditRepo)
			if dscMgr != nil {
				dscMgr.Start()
			}

			handler := handlers.Mount(handlers.Deps{
				Auth:       authDeps,
				Conns:      conns,
				Manager:    pgxMgr,
				Observ:     observRepo,
				ObservMgr:  observMgr,
				Discovered: discoveredRepo,
				Version:    version,
			})
			addr := fmt.Sprintf(":%d", cfg.Port)
			srv := server.New(addr, handler)

			go func() {
				<-ctx.Done()
				slog.Info("shutdown requested; draining in-flight requests")
				if dscMgr != nil {
					dscMgr.Shutdown()
				}
				observMgr.Shutdown()
				pgxMgr.Shutdown()
				_ = srv.Shutdown(context.Background())
			}()

			fmt.Fprintf(cmd.OutOrStdout(), "dbil listening on %s\n", addr)
			return srv.Start()
		},
	}
}

// buildDiscoverManager wires the discover.Manager according to env vars.
// Returns nil when DBIL_DISCOVER is empty or unset (the no-discovery default).
// Docker socket failures degrade gracefully — the manager still runs with
// any env-mode scanners wired in.
func buildDiscoverManager(repo *store.DiscoveredRepo, audit *store.AuditRepo) *discover.Manager {
	mode := discover.Mode(os.Getenv("DBIL_DISCOVER"))
	if mode == discover.ModeOff {
		return nil
	}
	autoJSON := os.Getenv("DBIL_AUTO_CONNECT")
	network := os.Getenv("DBIL_NETWORK")

	m, err := discover.NewManager(discover.Config{
		Mode:            mode,
		AutoConnectJSON: autoJSON,
		Network:         network,
	}, repo, audit, slog.Default())
	if err != nil {
		slog.Warn("discover: manager init failed", "err", err)
		return nil
	}

	if mode == discover.ModeDocker || mode == discover.ModeBoth {
		cli, err := dockerapi.NewClientFromEnv()
		if err != nil {
			slog.Warn("discover: docker client unavailable, running env-only",
				"err", err.Error(),
				"hint", "mount /var/run/docker.sock or unset DBIL_DISCOVER=docker")
		} else {
			m.AddDockerScanner(cli, network)
			slog.Info("discover: docker scanner enabled", "network", network)
		}
	}
	slog.Info("discover: started", "mode", string(mode), "network", network)
	return m
}
