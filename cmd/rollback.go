package cmd

import (
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/nlewo/comin/internal/config"
	"github.com/nlewo/comin/internal/deployer"
	executorPkg "github.com/nlewo/comin/internal/executor"
	"github.com/nlewo/comin/internal/protobuf"
	storePkg "github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var deploymentUUID, generationUUID, commitID string

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback to a previous deployment",
	Long: `Rollback to a previous deployment.
If no flags are provided, it rolls back to the last successful deployment.
You can specify a deployment to roll back to using one of the following flags:
--deployment-uuid, --generation-uuid, or --commit-id.`,
	Run: func(cmd *cobra.Command, args []string) {
		var stateDir string
		if configFilepath != "" {
			cfg, err := config.Read(configFilepath)
			if err != nil {
				logrus.Error(err)
				os.Exit(1)
			}
			stateDir = cfg.StateDir
		} else {
			stateDir = "/var/lib/comin"
		}

		storeFilename := path.Join(stateDir, "store.json")
		gcRootsDir := path.Join(stateDir, "gcroots")
		store, err := storePkg.New(storeFilename, gcRootsDir, 10, 10)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
		if err := store.Load(); err != nil {
			logrus.Errorf("Ignoring the state file %s because of the loading error: %s", storeFilename, err)
		}

		var deploymentToRollback *protobuf.Deployment
		// Ensure that only one flag is provided
		if deploymentUUID != "" && generationUUID != "" || deploymentUUID != "" && commitID != "" || generationUUID != "" && commitID != "" {
			fmt.Println("Error: only one of --deployment-uuid, --generation-uuid, or --commit-id can be provided")
			os.Exit(1)
		}

		if deploymentUUID != "" {
			deploymentToRollback, err = store.GetDeploymentByUUID(deploymentUUID)
			if err != nil {
				logrus.Error(err)
				os.Exit(1)
			}
		} else if generationUUID != "" {
			deploymentToRollback, err = store.GetDeploymentByGenerationUUID(generationUUID)
			if err != nil {
				logrus.Error(err)
				os.Exit(1)
			}
		} else if commitID != "" {
			deploymentToRollback, err = store.GetDeploymentByCommitId(commitID)
			if err != nil {
				logrus.Error(err)
				os.Exit(1)
			}
		} else {
			deploymentToRollback, err = store.GetLastSuccessfulDeployment()
			if err != nil {
				logrus.Error(err)
				os.Exit(1)
			}
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
		if err := deployer.Rollback(deploymentToRollback); err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

		fmt.Println("Rollback successful")
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
	rollbackCmd.PersistentFlags().StringVarP(&configFilepath, "config", "", "", "the configuration file path")
	rollbackCmd.Flags().StringVar(&deploymentUUID, "deployment-uuid", "", "The UUID of the deployment to roll back to")
	rollbackCmd.Flags().StringVar(&generationUUID, "generation-uuid", "", "The UUID of the generation to roll back to")
	rollbackCmd.Flags().StringVar(&commitID, "commit-id", "", "The commit ID of the deployment to roll back to")
}
