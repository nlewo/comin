package cmd

import (
	"github.com/nlewo/comin/config"
	"github.com/nlewo/comin/deploy"
	"github.com/nlewo/comin/http"
	"github.com/nlewo/comin/inotify"
	"github.com/nlewo/comin/worker"
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

		deployer, err := deploy.NewDeployer(dryRun, config)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

		wk := worker.NewWorker(deployer.Deploy)
		go worker.Scheduler(wk, config.Pollers)
		// FIXME: the state should be available from somewhere else...
		go http.Run(wk, config.Webhook, config.StateFilepath)
		go inotify.Run(wk, config.Inotify)
		wk.Run()
	},
}

func init() {
	pollCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "dry-run mode")
	pollCmd.PersistentFlags().StringVarP(&configFilepath, "config", "", "", "the configuration file path")
	pollCmd.MarkPersistentFlagRequired("config")
	rootCmd.AddCommand(pollCmd)
}
