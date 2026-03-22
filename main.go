// e9s is an interactive terminal UI for managing AWS infrastructure.
package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	e9saws "github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/ui"
)

var version = "dev"

func main() {
	var (
		cluster string
		region  string
		profile string
		refresh int
	)

	rootCmd := &cobra.Command{
		Use:     "e9s",
		Short:   "Interactive terminal UI for AWS ECS",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			// CLI flags override config file
			if cluster == "" {
				cluster = cfg.Defaults.Cluster
			}
			if region == "" {
				region = cfg.Defaults.Region
			}
			if profile == "" {
				profile = cfg.Defaults.Profile
			}
			if !cmd.Flags().Changed("refresh") {
				refresh = cfg.Defaults.RefreshInterval
			}

			ctx := context.Background()

			client, err := e9saws.NewClient(ctx, region, profile)
			if err != nil {
				return fmt.Errorf("failed to initialize AWS client: %w", err)
			}

			app := ui.NewApp(client, &cfg, cluster, refresh)
			p := tea.NewProgram(app, tea.WithAltScreen())

			if _, err := p.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	rootCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "Default cluster name (skips cluster selection)")
	rootCmd.Flags().StringVarP(&region, "region", "r", "", "AWS region (default: from AWS config)")
	rootCmd.Flags().StringVarP(&profile, "profile", "p", "", "AWS profile name")
	rootCmd.Flags().IntVar(&refresh, "refresh", 5, "Refresh interval in seconds")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
