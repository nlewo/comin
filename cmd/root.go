/*
Copyright © 2022 lewo <lewo@abesis.fr>
*/
package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

var debug bool
var hostname string
var flakeUrl string

// Set at build time
var version = "0.0.0"

var rootCmd = &cobra.Command{
	Use:     "comin",
	Short:   "GitOps For NixOS Machines",
	Version: version,
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
