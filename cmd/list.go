package cmd

import (
	"fmt"
	"github.com/nlewo/comin/internal/nix"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List hosts of the local repository",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		hosts, _ := nix.List(flakeUrl)
		for _, host := range hosts {
			fmt.Println(host)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&flakeUrl, "flake-url", "", ".", "the URL of the flake")
}
