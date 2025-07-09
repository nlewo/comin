package cmd

import (
	"context"
	"runtime"

	"github.com/nlewo/comin/internal/executor"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build a machine configuration",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.TODO()
		hosts := make([]string, 1)
		var configurationAttr string
		if runtime.GOOS == "darwin" {
			configurationAttr = "darwinConfigurations"
		} else {
			configurationAttr = "nixosConfigurations"
		}
		executor, _ := executor.NewNixExecutor(configurationAttr)
		if hostname != "" {
			hosts[0] = hostname
		} else {
			hosts, _ = executor.List(flakeUrl)
		}
		for _, host := range hosts {
			logrus.Infof("Building the NixOS configuration of machine '%s'", host)

			drvPath, _, err := executor.ShowDerivation(ctx, flakeUrl, host)
			if err != nil {
				logrus.Errorf("Failed to evaluate the configuration '%s': '%s'", host, err)
			}
			err = executor.Build(ctx, drvPath)
			if err != nil {
				logrus.Errorf("Failed to build the configuration '%s': '%s'", host, err)
			}
		}
	},
}

func init() {
	buildCmd.Flags().StringVarP(&hostname, "hostname", "", "", "the name of the configuration to build")
	buildCmd.Flags().StringVarP(&flakeUrl, "flake-url", "", ".", "the URL of the flake")
	rootCmd.AddCommand(buildCmd)
}
