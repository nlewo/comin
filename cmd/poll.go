package cmd

import (
	"github.com/spf13/cobra"
	"github.com/nlewo/comin/poll"
)

var pollCmd = &cobra.Command{
	Use:   "poll REPOSITORY-URL REPOSITORY-URL-FALLBACK-1 ...",
	Short: "Poll a repository and deploy the configuration",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		poll.Poller([]string{"bla"})
	},
}

func init() {
	rootCmd.AddCommand(pollCmd)
}
