package cmd

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var suspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend build and deploy operations",
	Long:  "This command suspends the build and deploy operations. If a build is running, it is stopped. If a deployment is running, it is not interupted but future deployment will be suspended.",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		url := "http://localhost:4242/api/manager/suspend"
		client := http.Client{
			Timeout: time.Second * 2,
		}
		req, err := http.NewRequest(http.MethodPost, url, nil)
		if err != nil {
			return
		}
		resp, _ := client.Do(req)
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("error: %s", string(body))
		}
	},
}
var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume build and deploy operations",
	Long:  "This command resumes the build and deploy operations. If a build has been suspended, it will be restarted.",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		url := "http://localhost:4242/api/manager/resume"
		client := http.Client{
			Timeout: time.Second * 2,
		}
		req, err := http.NewRequest(http.MethodPost, url, nil)
		if err != nil {
			return
		}
		resp, _ := client.Do(req)
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("error: %s", string(body))
		}
	},
}

func init() {
	rootCmd.AddCommand(suspendCmd)
	rootCmd.AddCommand(resumeCmd)
}
