package main

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/unkabas/dbil/internal/auth"
	"github.com/unkabas/dbil/internal/bootstrap"
	"github.com/unkabas/dbil/internal/config"
	"github.com/unkabas/dbil/internal/postgres"
	"github.com/unkabas/dbil/internal/server"
	"github.com/unkabas/dbil/internal/server/handlers"
	"github.com/unkabas/dbil/internal/store"
)

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the DBil HTTP server (requires `dbil init` to have been run first)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return err
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
			mgr := postgres.NewManager(postgres.NewPGX(), conns, auditRepo)

			handler := handlers.Mount(handlers.Deps{
				Auth:    authDeps,
				Conns:   conns,
				Manager: mgr,
				Version: version,
			})
			addr := fmt.Sprintf(":%d", cfg.Port)
			srv := server.New(addr, handler)

			// Graceful shutdown when ctx is cancelled (root command listens
			// for SIGINT/SIGTERM and cancels its context).
			go func() {
				<-ctx.Done()
				slog.Info("shutdown requested; draining in-flight requests")
				mgr.Shutdown()
				_ = srv.Shutdown(context.Background())
			}()

			fmt.Fprintf(cmd.OutOrStdout(), "dbil listening on %s\n", addr)
			return srv.Start()
		},
	}
}
