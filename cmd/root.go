/*
Copyright © 2022 lewo <lewo@abesis.fr>

*/
package cmd

import (
	"os"
	"github.com/spf13/cobra"
	"github.com/sirupsen/logrus"
)

var rootCmd = &cobra.Command{
	Use:   "comin",
	Short: "Deployment tool",
}

var Verbose bool

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "verbose output")
	if Verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

}
