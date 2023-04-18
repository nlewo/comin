package cmd

import (
	"github.com/nlewo/comin/nix"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Eval a machine from a local repository",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		_, _, err := nix.ShowDerivation(".", hostname)
		if err != nil {
			logrus.Errorf("Failed to eval the configuration '%s': '%s'", hostname, err)
		}
	},
}

func init() {
	hostnameDefault, err := os.Hostname()
	if err != nil {
		logrus.Error(err)
	}
	evalCmd.Flags().StringVarP(&hostname, "hostname", "", hostnameDefault, "the name of the configuration to eval")
	rootCmd.AddCommand(evalCmd)
}
