package cmd

import (
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var DeployerRetryCmd = &cobra.Command{
	Use:   "deployer-retry",
	Short: "Retry the last deployment (only if it failed)",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		url := "http://localhost:4242/api/deployer/retry"
		client := http.Client{
			Timeout: time.Second * 2,
		}
		req, err := http.NewRequest(http.MethodPost, url, nil)
		if err != nil {
			return
		}
		client.Do(req)
	},
}

func init() {
	rootCmd.AddCommand(DeployerRetryCmd)
}
