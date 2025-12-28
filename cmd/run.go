package cmd

import (
	"os"
	"path"
	"runtime"
	"time"

	brokerPkg "github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/builder"
	"github.com/nlewo/comin/internal/config"
	"github.com/nlewo/comin/internal/deployer"
	executorPkg "github.com/nlewo/comin/internal/executor"
	"github.com/nlewo/comin/internal/fetcher"
	"github.com/nlewo/comin/internal/http"
	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/prometheus"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/repository"
	"github.com/nlewo/comin/internal/scheduler"
	"github.com/nlewo/comin/internal/server"
	storePkg "github.com/nlewo/comin/internal/store"
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

		if err := os.MkdirAll(cfg.StateDir, os.ModePerm); err != nil {
			logrus.Errorf("Failed to create the state dir %s: %s", cfg.StateDir, err)
			return
		}
		// TODO: this could be removed from release > 0.9.0.
		//
		// Previous comin versions didn't correctly set the
		// permissions of the /var/lib/comin folder. This
		// means we need to explicitly chown them to fix this
		// for existing comin deployments.
		if err := os.Chmod(cfg.StateDir, 0755); err != nil {
			logrus.Errorf("Failed to chmod the state dir %s: %s", cfg.StateDir, err)
			return
		}

		var executor executorPkg.Executor
		switch cfg.RepositoryType {
		case "flake":
			executor, err = executorPkg.NewNixOSFlake()
			if runtime.GOOS == "darwin" {
				executor, err = executorPkg.NewNixDarwinFlake()
			}
		case "nix":
			executor, err = executorPkg.NewNixOSNix()
		}
		if err != nil {
			logrus.Errorf("Failed to create the executor: %s", err)
			return
		}

		machineId, err := executor.ReadMachineId()
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

		metrics := prometheus.New()
		storeFilename := path.Join(cfg.StateDir, "store.json")
		gcRootsDir := path.Join(cfg.StateDir, "gcroots")
		broker := brokerPkg.New()
		broker.Start()
		store, err := storePkg.New(broker, storeFilename, gcRootsDir, 10, 10)
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
		var lastDeployment *protobuf.Deployment
		if ok, ld := store.LastDeployment(); ok {
			mainCommitId = ld.Generation.MainCommitId
			lastDeployment = ld
			metrics.SetDeploymentInfo(ld.Generation.SelectedCommitId, ld.Status)
		}
		repository, err := repository.New(gitConfig, mainCommitId, metrics)
		if err != nil {
			logrus.Errorf("Failed to initialize the repository: %s", err)
			os.Exit(1)
		}

		fetcher := fetcher.NewFetcher(repository)
		fetcher.Start(cmd.Context())
		sched := scheduler.New()
		sched.FetchRemotes(fetcher, cfg.Remotes)

		builder := builder.New(store, executor, gitConfig.Path, gitConfig.Dir, cfg.SystemAttr, cfg.Hostname, 30*time.Minute, 30*time.Minute)
		deployer := deployer.New(store, executor.Deploy, lastDeployment, cfg.PostDeploymentCommand)

		mode, err := manager.ParseMode(cfg.BuildConfirmer.Mode)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
		buildConfirmer := manager.NewConfirmer(broker, mode, time.Duration(cfg.BuildConfirmer.AutoDuration)*time.Second, "build")
		buildConfirmer.Start()
		mode, err = manager.ParseMode(cfg.DeployConfirmer.Mode)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
		deployConfirmer := manager.NewConfirmer(broker, mode, time.Duration(cfg.DeployConfirmer.AutoDuration)*time.Second, "deploy")
		deployConfirmer.Start()
		manager := manager.New(store, metrics, sched, fetcher, builder, deployer, machineId, executor, buildConfirmer, deployConfirmer, broker)

		http.Serve(manager,
			metrics,
			cfg.ApiServer.ListenAddress, cfg.ApiServer.Port,
			cfg.Exporter.ListenAddress, cfg.Exporter.Port)
		srv := server.New(broker, manager, cfg.Grpc.UnixSocketPath)
		srv.Start()
		manager.Run(cmd.Context())
	},
}

func init() {
	runCmd.PersistentFlags().StringVarP(&configFilepath, "config", "", "", "the configuration file path")
	_ = runCmd.MarkPersistentFlagRequired("config")
	rootCmd.AddCommand(runCmd)
}
