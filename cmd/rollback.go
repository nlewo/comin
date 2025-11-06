package cmd

import (
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/nlewo/comin/internal/config"
	"github.com/nlewo/comin/internal/deployer"
	executorPkg "github.com/nlewo/comin/internal/executor"
	storePkg "github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback to the last successful deployment",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Read(configFilepath)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

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

		lastSuccessfulDeployment, err := store.GetLastSuccessfulDeployment()
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

		executor, err := executorPkg.NewNixOS()
		if runtime.GOOS == "darwin" {
			executor, err = executorPkg.NewNixDarwin()
		}
		if err != nil {
			logrus.Errorf("Failed to create the executor: %s", err)
			return
		}

		deployer := deployer.New(store, executor.Deploy, nil, "", "")
		if err := deployer.Rollback(lastSuccessfulDeployment); err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

		fmt.Println("Rollback successful")
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
	rollbackCmd.PersistentFlags().StringVarP(&configFilepath, "config", "", "", "the configuration file path")
	_ = rollbackCmd.MarkPersistentFlagRequired("config")
}
