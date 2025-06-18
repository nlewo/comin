package cmd

import (
	"os"
	"path"
	"time"

	"github.com/nlewo/comin/internal/builder"
	"github.com/nlewo/comin/internal/config"
	"github.com/nlewo/comin/internal/deployer"
	"github.com/nlewo/comin/internal/executor"
	"github.com/nlewo/comin/internal/fetcher"
	"github.com/nlewo/comin/internal/http"
	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/prometheus"
	"github.com/nlewo/comin/internal/repository"
	"github.com/nlewo/comin/internal/scheduler"
	storePkg "github.com/nlewo/comin/internal/store"
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

		machineId, err := utils.ReadMachineId()
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

		metrics := prometheus.New()
		storeFilename := path.Join(cfg.StateDir, "store.json")
		gcRootsDir := path.Join(cfg.StateDir, "gcroots")
		store, err := storePkg.New(storeFilename, gcRootsDir, 10, 10)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
		if err := store.Load(); err != nil {
			logrus.Errorf("Ignoring the state file %s because of the loading error: %s", storeFilename, err)
		}
		metrics.SetBuildInfo(cmd.Version)

		// We get the last mainCommitId to avoid useless
		// redeployment as well as non fast forward checkouts
		var mainCommitId string
		var lastDeployment *storePkg.Deployment
		if ok, ld := store.LastDeployment(); ok {
			mainCommitId = ld.Generation.MainCommitId
			lastDeployment = &ld
		}
		repository, err := repository.New(gitConfig, mainCommitId, metrics)
		if err != nil {
			logrus.Errorf("Failed to initialize the repository: %s", err)
			os.Exit(1)
		}

		fetcher := fetcher.NewFetcher(repository)
		fetcher.Start()
		sched := scheduler.New()
		sched.FetchRemotes(fetcher, cfg.Remotes)

		executor, err := executor.New()
		if err != nil {
			logrus.Error("Failed to create executor")
			return
		}

		builder := builder.New(store, gitConfig.Path, gitConfig.Dir, cfg.Hostname, 30*time.Minute, executor.Eval, 30*time.Minute, executor.Build)
		deployer := deployer.New(executor.Deploy, lastDeployment, cfg.PostDeploymentCommand)

		manager := manager.New(store, metrics, sched, fetcher, builder, deployer, machineId)

		http.Serve(manager,
			metrics,
			cfg.ApiServer.ListenAddress, cfg.ApiServer.Port,
			cfg.Exporter.ListenAddress, cfg.Exporter.Port)
		manager.Run()
	},
}

func init() {
	runCmd.PersistentFlags().StringVarP(&configFilepath, "config", "", "", "the configuration file path")
	_ = runCmd.MarkPersistentFlagRequired("config")
	rootCmd.AddCommand(runCmd)
}
