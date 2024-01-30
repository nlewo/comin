package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func getStatus() (status manager.State, err error) {
	url := "http://localhost:4242/status"
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
		fmt.Printf("Status of the machine '%s':\n", status.Hostname)
		fmt.Printf("  Deployment status is '%s'\n", status.Deployment.Status)
		fmt.Printf("  Deployed from '%s/%s'\n",
			status.Deployment.Generation.RepositoryStatus.SelectedRemoteName,
			status.Deployment.Generation.RepositoryStatus.SelectedBranchName,
		)
		fmt.Printf("  Deployed commit ID is '%s'\n", status.Deployment.Generation.RepositoryStatus.SelectedCommitId)
		fmt.Printf("  Deployed commit msg is\n    %s\n",
			utils.FormatCommitMsg(status.Deployment.Generation.RepositoryStatus.SelectedCommitMsg),
		)
		fmt.Printf("  Deployed %s\n", humanize.Time(status.Deployment.EndAt))
		for _, r := range status.RepositoryStatus.Remotes {
			fmt.Printf("  Remote %s fetched %s\n",
				r.Url, humanize.Time(r.FetchedAt),
			)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
