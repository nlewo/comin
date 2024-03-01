package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/nlewo/comin/internal/deployment"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func printDeployment(d deployment.Deployment) {
	fmt.Printf("Deployment UUID  %s\n", d.UUID)
	fmt.Printf("Generation UUID  %s\n", d.Generation.UUID)
	fmt.Printf("Commit ID        %s\n", d.Generation.SelectedCommitId)
	fmt.Printf("Commit Message   %s\n", utils.FormatCommitMsg(d.Generation.SelectedCommitMsg))
	fmt.Printf("Operation        %s\n", d.Operation)
	fmt.Printf("Status           %s\n", deployment.StatusToString(d.Status))
	fmt.Printf("Ended At         %s (%s)\n", d.EndAt, humanize.Time(d.EndAt))
	fmt.Printf("Duration         %s\n", d.EndAt.Sub(d.StartAt).String())
}

func getDeploymentList() (d []deployment.Deployment, err error) {
	url := "http://localhost:4242/deployment"
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
	err = json.Unmarshal(body, &d)
	if err != nil {
		return
	}
	return
}

var deploymentListCmd = &cobra.Command{
	Use:   "deployment",
	Short: "Get the list of deployments",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		d, err := getDeploymentList()
		if err != nil {
			logrus.Fatal(err)
		}
		for _, v := range d {
			printDeployment(v)
			fmt.Println()
		}
	},
}

func init() {
	rootCmd.AddCommand(deploymentListCmd)
}
