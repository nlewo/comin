package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/nlewo/comin/internal/manager"
	store "github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var statusOneline bool

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
		defer res.Body.Close() // nolint
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

func longStatus(status manager.State) {
	fmt.Printf("Status of the machine %s\n", status.Builder.Hostname)
	if status.NeedToReboot {
		fmt.Printf("  Need to reboot: yes\n")
	}
	fmt.Printf("  Fetcher\n")
	if status.Fetcher.RepositoryStatus.SelectedCommitShouldBeSigned {
		if status.Fetcher.RepositoryStatus.SelectedCommitSigned {
			fmt.Printf("    Commit %s signed by %s\n", status.Fetcher.RepositoryStatus.SelectedCommitId, status.Fetcher.RepositoryStatus.SelectedCommitSignedBy)
		} else {
			fmt.Printf("    Commit %s is not signed while it should be\n", status.Fetcher.RepositoryStatus.SelectedCommitId)
		}
	}
	for _, r := range status.Fetcher.RepositoryStatus.Remotes {
		fmt.Printf("    Remote %s %s fetched %s\n",
			r.Name, r.Url, humanize.Time(r.FetchedAt),
		)
	}
	fmt.Printf("  Builder\n")
	if status.Builder.Generation != nil {
		store.GenerationShow(*status.Builder.Generation)
	} else {
		fmt.Printf("    No build available\n")
	}
	status.Deployer.Show("    ")

}

func onelineStatus(status manager.State) {
	if status.Builder.Generation != nil && status.Builder.IsEvaluating {
		fmt.Printf(" eval   %s/%s (%s)", status.Builder.Generation.SelectedRemoteName, status.Builder.Generation.SelectedBranchName,
			humanize.Time(status.Builder.Generation.EvalStartedAt))
	} else if status.Builder.Generation != nil && status.Builder.IsBuilding {
		fmt.Printf(" build  %s/%s (%s)", status.Builder.Generation.SelectedRemoteName, status.Builder.Generation.SelectedBranchName,
			humanize.Time(status.Builder.Generation.BuildStartedAt))
	} else if status.Builder.Generation != nil && status.Builder.Generation.EvalStatus == store.EvalFailed {
		fmt.Printf(" %s/%s (%s)", status.Builder.Generation.SelectedRemoteName, status.Builder.Generation.SelectedBranchName,
			humanize.Time(status.Builder.Generation.EvalEndedAt))
	} else if status.Builder.Generation != nil && status.Builder.Generation.BuildStatus == store.BuildFailed {
		fmt.Printf(" %s/%s (%s)", status.Builder.Generation.SelectedRemoteName, status.Builder.Generation.SelectedBranchName,
			humanize.Time(status.Builder.Generation.BuildEndedAt))
	} else if status.Deployer.Deployment != nil {
		switch status.Deployer.Deployment.Status {
		case store.Running:
			fmt.Printf(" deploy %s/%s (%s)", status.Deployer.Deployment.Generation.SelectedRemoteName, status.Deployer.Deployment.Generation.SelectedBranchName,
				humanize.Time(status.Deployer.Deployment.EndedAt))
		case store.Failed:
			fmt.Printf(" %s/%s (%s)", status.Deployer.Deployment.Generation.SelectedRemoteName, status.Deployer.Deployment.Generation.SelectedBranchName,
				humanize.Time(status.Deployer.Deployment.EndedAt))
		case store.Done:
			fmt.Printf(" %s/%s (%s)", status.Deployer.Deployment.Generation.SelectedRemoteName, status.Deployer.Deployment.Generation.SelectedBranchName,
				humanize.Time(status.Deployer.Deployment.EndedAt))
		}
	}
	if status.NeedToReboot {
		fmt.Printf(" ")
	}
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
		if statusOneline {
			onelineStatus(status)
		} else {
			longStatus(status)
		}
	},
}

func init() {
	statusCmd.PersistentFlags().BoolVarP(&statusOneline, "oneline", "", false, "oneline")
	rootCmd.AddCommand(statusCmd)
}
