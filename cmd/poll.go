package cmd

import (
	"github.com/spf13/cobra"
	"github.com/nlewo/comin/poll"
	"github.com/sirupsen/logrus"
	"os"
)

var stateDir string
var dryRun bool

var pollCmd = &cobra.Command{
	Use:   "poll REPOSITORY-URL",
	Short: "Poll a repository and deploy the configuration",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		poll.Poller(hostname, stateDir, dryRun, args[0:])
	},
}

func init() {
	hostnameDefault, err := os.Hostname()
	if err != nil {
		logrus.Error(err)
	}
	pollCmd.Flags().StringVarP(&hostname, "hostname", "", hostnameDefault, "the name of the configuration to deploy")
	pollCmd.Flags().StringVarP(&stateDir, "state-dir", "", "/var/lib/comin", "the path of the state directory")
	pollCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "dry-run mode")
	rootCmd.AddCommand(pollCmd)
}
