package cmd

import (
	"os"

	"github.com/nlewo/comin/internal/executor"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var darwinDeployCmd = &cobra.Command{
	Use:    executor.DarwinDeployCommand + " DIRECTORY OUT-PATH OPERATION [PROFILE-PATH...]",
	Short:  "Deploy a nix-darwin system and record the output and the result in a directory",
	Args:   cobra.MinimumNArgs(3),
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		if err := executor.RunDarwinDeploy(args[0], args[1], args[2], args[3:]); err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(darwinDeployCmd)
}
