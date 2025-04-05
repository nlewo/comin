package cmd

import (
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Trigger a fetch of all Git remotes",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		url := "http://localhost:4242/api/fetcher/fetch"
		client := http.Client{
			Timeout: time.Second * 2,
		}
		req, err := http.NewRequest(http.MethodPost, url, nil)
		if err != nil {
			return
		}
		_, _ = client.Do(req)
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)
}
