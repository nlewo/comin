package cmd

import (
	"github.com/spf13/cobra"
	"github.com/nlewo/comin/poll"
	"github.com/sirupsen/logrus"
	"os"
	"fmt"
)

var stateDir, authsFilepath string
var dryRun bool

var pollCmd = &cobra.Command{
	Use:   "poll REPOSITORY-URL",
	Short: "Poll a repository and deploy the configuration",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := poll.Poller(hostname, stateDir, authsFilepath, dryRun, args[0:])
		if err != nil {
			fmt.Printf("Error: %s", err)
			os.Exit(1)
		}
	},
}

func init() {
	hostnameDefault, err := os.Hostname()
	if err != nil {
		logrus.Error(err)
	}
	pollCmd.Flags().StringVarP(&hostname, "hostname", "", hostnameDefault, "the name of the configuration to deploy")
	pollCmd.Flags().StringVarP(&stateDir, "state-dir", "", "/var/lib/comin", "the path of the state directory")
	pollCmd.Flags().StringVarP(&authsFilepath, "auths-file", "", "", "the path of the JSON auths file")
	pollCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "dry-run mode")
	rootCmd.AddCommand(pollCmd)
}
