package cmd

import (
	"fmt"
	"slices"
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
		deploymentList(status.Store.Deployments, status.Store.RetentionReasons)
	},
}

var deploymentLatestSubmitCmd = &cobra.Command{
	Use:  "submit-latest",
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		operation, _ := cmd.Flags().GetString("operation")
		if !slices.Contains([]string{"test", "switch", "boot"}, operation) {
			logrus.Fatalf("The operation is '%s' while it must be one of [test, switch, boot]", operation)
		}
		opts := client.ClientOpts{
			UnixSocketPath: "/var/lib/comin/grpc.sock",
		}
		c, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		err = c.DeploymentLatestSubmit(operation)
		if err != nil {
			logrus.Fatal(err)
		}
	},
}

func deploymentList(dpls []*protobuf.Deployment, retentions map[string]string) {
	endedAtCmp := func(a, b *protobuf.Deployment) int {
		return a.EndedAt.AsTime().Compare(b.EndedAt.AsTime())
	}
	slices.SortFunc(dpls, endedAtCmp)

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
		reason, ok := retentions[dpl.Uuid]
		if ok {
			fmt.Printf("  retention         %s\n", reason)
		}
		fmt.Print("\n")
	}
}

func init() {
	rootCmd.AddCommand(deploymentCmd)
	deploymentCmd.AddCommand(deploymentListCmd)
	deploymentLatestSubmitCmd.Flags().StringP("operation", "", "", "The deployment operation: [boot, test, switch]")
	deploymentCmd.AddCommand(deploymentLatestSubmitCmd)
}
