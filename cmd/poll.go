package cmd

import (
	"github.com/nlewo/comin/config"
	"github.com/nlewo/comin/poll"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

var configFilepath string
var dryRun bool

var pollCmd = &cobra.Command{
	Use:   "poll",
	Short: "Poll a repository and deploy the configuration",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := config.Read(configFilepath)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
		err = poll.Poller(dryRun, config)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
	},
}

func init() {
	pollCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "dry-run mode")
	pollCmd.PersistentFlags().StringVarP(&configFilepath, "config", "", "", "the configuration file path")
	pollCmd.MarkPersistentFlagRequired("config")
	rootCmd.AddCommand(pollCmd)
}
