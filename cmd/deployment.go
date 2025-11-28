package cmd

import (
	"fmt"
	"time"

	"github.com/nlewo/comin/internal/client"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var deploymentCmd = &cobra.Command{
	Use: "deployment",
}

var deploymentListCmd = &cobra.Command{
	Use:  "list",
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		opts := client.ClientOpts{
			UnixSocketPath: "/var/lib/comin/grpc.sock",
		}
		c, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		status, err := c.GetManagerState()
		if err != nil {
			logrus.Fatal(err)
		}
		deploymentList(status.Store.Deployments)
	},
}

func deploymentList(dpls []*protobuf.Deployment) {
	for _, dpl := range dpls {
		fmt.Printf("%s\n", dpl.Uuid)
		fmt.Printf("  ended at          %s\n", dpl.EndedAt.AsTime().Format(time.DateTime))
		fmt.Printf("  operation         %s\n", dpl.Operation)
		if dpl.ProfilePath != "" {
			fmt.Printf("  profile path      %s\n", dpl.ProfilePath)
		}
		fmt.Printf("  out path          %s\n", dpl.Generation.OutPath)
		fmt.Printf("  generation uuid   %s\n", dpl.Generation.Uuid)
		fmt.Printf("    commit id       %s\n", dpl.Generation.SelectedCommitId)
		fmt.Print("\n")
	}
}

func init() {
	rootCmd.AddCommand(deploymentCmd)
	deploymentCmd.AddCommand(deploymentListCmd)
}
