package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/nlewo/comin/state"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

func getStatus() (state state.State, err error) {
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
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(body, &state)
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
		state, err := getStatus()
		if err != nil {
			logrus.Fatal(err)
		}
		fmt.Printf("- The commit ID is %s\n", state.RepositoryStatus.SelectedCommitId)
		fmt.Printf("- The commit msg is\n    %s\n",
			strings.Replace(
				state.RepositoryStatus.SelectedCommitMsg,
				"\n",
				"\n    ",
				-1,
			),
		)
		fmt.Printf("- Deployed from '%s/%s'\n",
			state.RepositoryStatus.SelectedBranchName,
			state.RepositoryStatus.SelectedRemoteName,
		)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
