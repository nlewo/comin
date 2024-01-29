package cmd

import (
	"os"

	"github.com/nlewo/comin/config"
	"github.com/nlewo/comin/http"
	"github.com/nlewo/comin/manager"
	"github.com/nlewo/comin/poller"
	"github.com/nlewo/comin/repository"
	"github.com/nlewo/comin/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var configFilepath string

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run comin to deploy your published configurations",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Read(configFilepath)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
		gitConfig := config.MkGitConfig(cfg)

		repositoryStatus := repository.RepositoryStatus{}
		repository, err := repository.New(gitConfig, repositoryStatus)
		if err != nil {
			logrus.Errorf("Failed to initialize the repository: %s", err)
			os.Exit(1)
		}

		machineId, err := utils.ReadMachineId()
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

		manager := manager.New(repository, gitConfig.Path, cfg.Hostname, machineId)
		go poller.Poller(manager, cfg.Remotes)
		go http.Serve(manager, cfg.HttpServer.Address, cfg.HttpServer.Port)
		manager.Run()
	},
}

func init() {
	runCmd.PersistentFlags().StringVarP(&configFilepath, "config", "", "", "the configuration file path")
	runCmd.MarkPersistentFlagRequired("config")
	rootCmd.AddCommand(runCmd)
}
