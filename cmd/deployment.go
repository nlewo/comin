package cmd

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nlewo/comin/internal/client"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var deploymentCmd = &cobra.Command{
	Use: "deployment",
}

var bootOnly bool

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
		dpls := status.Store.Deployments
		if bootOnly {
			dpls = filterBootEntryDeployments(dpls, status.Store)
		}
		deploymentList(dpls, status.Store, bootOnly)
	},
}

var deploymentSwitchLatestCmd = &cobra.Command{
	Use:  "switch-latest",
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		opts := client.ClientOpts{
			UnixSocketPath: "/var/lib/comin/grpc.sock",
		}
		c, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		err = c.SwitchDeploymentLatest()
		if err != nil {
			logrus.Fatal(err)
		}
	},
}

func retentionListsForDeployment(uuid string, store *protobuf.Store) (result []string) {
	if store.DeploymentSwitched == uuid {
		result = append(result, "switched")
	}
	if store.DeploymentBooted == uuid {
		result = append(result, "booted")
	}
	if slices.Contains(store.DeploymentsBootEntry, uuid) {
		result = append(result, "boot")
	}
	if slices.Contains(store.DeploymentsSuccessful, uuid) {
		result = append(result, "successful")
	}
	if slices.Contains(store.DeploymentsAny, uuid) {
		result = append(result, "any")
	}
	return
}

func filterBootEntryDeployments(dpls []*protobuf.Deployment, store *protobuf.Store) []*protobuf.Deployment {
	bootUUIDs := make(map[string]struct{})
	if store.DeploymentSwitched != "" {
		bootUUIDs[store.DeploymentSwitched] = struct{}{}
	}
	if store.DeploymentBooted != "" {
		bootUUIDs[store.DeploymentBooted] = struct{}{}
	}
	for _, uuid := range store.DeploymentsBootEntry {
		bootUUIDs[uuid] = struct{}{}
	}
	var filtered []*protobuf.Deployment
	for _, dpl := range dpls {
		if _, ok := bootUUIDs[dpl.Uuid]; ok {
			filtered = append(filtered, dpl)
		}
	}
	return filtered
}

func deploymentList(dpls []*protobuf.Deployment, store *protobuf.Store, newestFirst bool) {
	endedAtCmp := func(a, b *protobuf.Deployment) int {
		return a.EndedAt.AsTime().Compare(b.EndedAt.AsTime())
	}
	slices.SortFunc(dpls, endedAtCmp)
	if newestFirst {
		slices.Reverse(dpls)
	}

	for _, dpl := range dpls {
		fmt.Printf("%s\n", dpl.Uuid)
		fmt.Printf("  status             %s\n", dpl.Status)
		fmt.Printf("  ended at           %s\n", dpl.EndedAt.AsTime().Format(time.DateTime))
		fmt.Printf("  operation          %s\n", dpl.Operation)
		if dpl.ProfilePath != "" {
			fmt.Printf("  profile path       %s\n", dpl.ProfilePath)
		}
		fmt.Printf("  out path           %s\n", dpl.Generation.OutPath)
		fmt.Printf("  generation uuid    %s\n", dpl.Generation.Uuid)
		fmt.Printf("    commit id        %s\n", dpl.Generation.SelectedCommitId)
		if lists := retentionListsForDeployment(dpl.Uuid, store); len(lists) > 0 {
			fmt.Printf("  part of retention  %s\n", strings.Join(lists, ", "))
		}
		fmt.Print("\n")
	}
}

func init() {
	rootCmd.AddCommand(deploymentCmd)
	deploymentCmd.AddCommand(deploymentListCmd)
	deploymentListCmd.Flags().BoolVar(&bootOnly, "boot", false, "only show boot entry deployments, newest first")
	deploymentCmd.AddCommand(deploymentSwitchLatestCmd)
}
