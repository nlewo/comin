package cmd

import (
	"os"

	"github.com/nlewo/comin/internal/config"
	"github.com/nlewo/comin/internal/http"
	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/prometheus"
	"github.com/nlewo/comin/internal/poller"
	"github.com/nlewo/comin/internal/repository"
	"github.com/nlewo/comin/internal/utils"
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

		metrics := prometheus.New()
		manager := manager.New(repository, metrics, gitConfig.Path, cfg.Hostname, machineId)
		go poller.Poller(manager, cfg.Remotes)
		http.Serve(manager,
			metrics,
			cfg.ApiServer.ListenAddress, cfg.ApiServer.Port,
			cfg.Exporter.ListenAddress, cfg.Exporter.Port)
		manager.Run()
	},
}

func init() {
	runCmd.PersistentFlags().StringVarP(&configFilepath, "config", "", "", "the configuration file path")
	runCmd.MarkPersistentFlagRequired("config")
	rootCmd.AddCommand(runCmd)
}
