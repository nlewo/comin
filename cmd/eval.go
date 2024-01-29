package cmd

import (
	"context"

	"github.com/nlewo/comin/nix"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Eval a machine from a local repository",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		hosts := make([]string, 1)
		ctx := context.TODO()
		if hostname != "" {
			hosts[0] = hostname
		} else {
			hosts, _ = nix.List(flakeUrl)
		}
		for _, host := range hosts {
			logrus.Infof("Evaluating the NixOS configuration of machine '%s'", host)
			_, _, err := nix.ShowDerivation(ctx, flakeUrl, host)
			if err != nil {
				logrus.Errorf("Failed to eval the configuration '%s': '%s'", host, err)
			}
		}
	},
}

func init() {
	evalCmd.Flags().StringVarP(&hostname, "hostname", "", "", "the name of the configuration to eval")
	evalCmd.Flags().StringVarP(&flakeUrl, "flake-url", "", ".", "the URL of the flake")
	rootCmd.AddCommand(evalCmd)
}
