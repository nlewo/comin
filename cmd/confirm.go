package cmd

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var confirmCmd = &cobra.Command{
	Use:   "confirm",
	Short: "Confirm a build can be proceeded",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		url := "http://localhost:4242/api/confirm"
		client := http.Client{
			Timeout: time.Second * 2,
		}
		req, err := http.NewRequest(http.MethodPost, url, nil)
		if err != nil {
			return
		}
		resp, err := client.Do(req)
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("%s", body)
		}
	},
}

func init() {
	rootCmd.AddCommand(confirmCmd)
}
