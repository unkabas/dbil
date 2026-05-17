// Command dbil is the entrypoint binary. Subcommands `version` and `init`
// are wired here; later plans add `serve`.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/unkabas/dbil/internal/bootstrap"
	"github.com/unkabas/dbil/internal/config"
	dlog "github.com/unkabas/dbil/internal/log"
)

// Build-time variables. Wired via -ldflags by the Makefile.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	bootstrap.Version = version

	dlog.Setup(slog.LevelInfo, dlog.InContainer(), os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	root := &cobra.Command{
		Use:           "dbil",
		Short:         "DBil — security-first PostgreSQL tool",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(versionCmd())
	root.AddCommand(initCmd())
	root.AddCommand(serveCmd())

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the build version, commit, and date",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "dbil %s (%s) built %s\n", version, commit, date)
			return nil
		},
	}
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the DBil data directory (master key, state store, admin user)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			res, err := bootstrap.RunInit(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			if res.CreatedAdmin {
				fmt.Fprintf(cmd.OutOrStdout(),
					"init complete: admin %s created; audit genesis id=%d; master key source=%s\n",
					res.AdminEmail, res.AuditGenesisID, res.MasterKeySource)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(),
					"init complete: already initialized; audit chain verified; master key source=%s\n",
					res.MasterKeySource)
			}
			return nil
		},
	}
}
