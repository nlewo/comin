package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/nlewo/comin/nix"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List hosts of the local repository",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		hosts, _ := nix.List()
		for _, host := range hosts {
			fmt.Println(host)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
