package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/nlewo/comin/internal/builder"
	"github.com/nlewo/comin/internal/manager"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func getStatus() (status manager.State, err error) {
	url := "http://localhost:4242/api/status"
	client := http.Client{
		Timeout: time.Second * 2,
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return
	}
	res, err := client.Do(req)
	if err != nil {
		return
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &status)
	if err != nil {
		return
	}
	return
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the status of the local machine",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		status, err := getStatus()
		if err != nil {
			logrus.Fatal(err)
		}
		fmt.Printf("Status of the machine %s\n", status.Builder.Hostname)
		needToReboot := "no"
		if status.NeedToReboot {
			needToReboot = "yes"
		}
		fmt.Printf("  Need to reboot: %s\n", needToReboot)
		fmt.Printf("  Fetcher\n")
		for _, r := range status.Fetcher.RepositoryStatus.Remotes {
			fmt.Printf("    Remote %s %s fetched %s\n",
				r.Name, r.Url, humanize.Time(r.FetchedAt),
			)
		}
		fmt.Printf("  Builder\n")
		builder.GenerationShow(*status.Builder.Generation)
		status.Deployer.Show("    ")
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
