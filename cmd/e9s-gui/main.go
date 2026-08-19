//go:build gui

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	e9saws "github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/gui"
	"github.com/dostrow/e9s/internal/service"
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
		Use:     "e9s-gui",
		Short:   "Experimental GTK 4 frontend for e9s",
		Version: version,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.Load()
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
			if refresh <= 0 {
				refresh = 5
			}

			client, err := e9saws.NewClient(context.Background(), region, profile)
			if err != nil {
				return fmt.Errorf("initialize AWS client: %w", err)
			}

			status := profile
			if status == "" {
				status = "default"
			}
			return gui.Run(gui.Options{
				ECS:             service.NewECS(client),
				Logs:            service.NewLogs(client),
				Alarms:          service.NewAlarms(client),
				SSM:             service.NewSSM(client),
				Secrets:         service.NewSecrets(client),
				Lambda:          service.NewLambda(client),
				CodeBuild:       service.NewCodeBuild(client),
				EC2:             service.NewEC2(client),
				EC2Network:      service.NewEC2Network(client),
				Config:          &cfg,
				ReloadConfig:    config.Reload,
				DefaultCluster:  cluster,
				Profile:         status,
				Region:          client.Region(),
				RefreshInterval: refresh,
			})
		},
	}

	rootCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "ECS cluster to open")
	rootCmd.Flags().StringVarP(&region, "region", "r", "", "AWS region")
	rootCmd.Flags().StringVarP(&profile, "profile", "p", "", "AWS profile name")
	rootCmd.Flags().IntVar(&refresh, "refresh", 5, "Auto-refresh interval in seconds")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
