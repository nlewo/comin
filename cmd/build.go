package cmd

import (
	"github.com/nlewo/comin/nix"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build a machine from a local repository",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		nix.Build(".", hostname)
	},
}

func init() {
	hostnameDefault, err := os.Hostname()
	if err != nil {
		logrus.Error(err)
	}
	buildCmd.Flags().StringVarP(&hostname, "hostname", "", hostnameDefault, "the name of the configuration to deploy")
	rootCmd.AddCommand(buildCmd)
}
