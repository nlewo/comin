/*
Copyright © 2022 lewo <lewo@abesis.fr>

*/
package cmd

import (
	"os"
	"github.com/spf13/cobra"
	"github.com/sirupsen/logrus"
)

var debug bool
var hostname string

var rootCmd = &cobra.Command{
	Use:   "comin",
	Short: "Deployment tool",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if debug {
			logrus.Info("Debug logs enabled")
			logrus.SetLevel(logrus.DebugLevel)
		}
	}
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "verbose logging")
}
