package cmd

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/nlewo/comin/internal/client"
	"github.com/nlewo/comin/internal/deployer"
	pb "github.com/nlewo/comin/internal/protobuf"
	store "github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var statusOneline bool
var statusJson bool

func longStatus(status *pb.State) {
	fmt.Printf("Status of the machine %s\n", status.Builder.Hostname)
	if status.NeedToReboot.GetValue() {
		fmt.Printf("  Need to reboot: yes\n")
	}
	if status.IsSuspended.GetValue() {
		fmt.Printf("  Is suspended: yes\n")
	}
	fmt.Printf("  Fetcher\n")
	if status.Fetcher.RepositoryStatus != nil && status.Fetcher.RepositoryStatus.SelectedCommitShouldBeSigned.GetValue() {
		if status.Fetcher.RepositoryStatus.SelectedCommitSigned.GetValue() {
			fmt.Printf("    Commit %s signed by %s\n", status.Fetcher.RepositoryStatus.SelectedCommitId, status.Fetcher.RepositoryStatus.SelectedCommitSignedBy)
		} else {
			fmt.Printf("    Commit %s is not signed while it should be\n", status.Fetcher.RepositoryStatus.SelectedCommitId)
		}
	}
	for _, r := range status.Fetcher.RepositoryStatus.Remotes {
		fmt.Printf("    Remote %s %s fetched %s\n",
			r.Name, r.Url, humanize.Time(r.FetchedAt.AsTime()),
		)
	}
	fmt.Printf("  Builder\n")
	if status.Builder.Generation != nil {
		store.GenerationShow(status.Builder.Generation)
	} else {
		fmt.Printf("    No build available\n")
	}
	deployer.Show(status.Deployer, "    ")
}

func jsonStatus(status *pb.State) {
	// Create a protojson marshaler with options
	marshaler := protojson.MarshalOptions{
		// Use proto field names (not camelCase)
		UseProtoNames: true,
		// Include fields with zero values
		EmitUnpopulated: true,
		// Pretty-print with indentation
		Indent: "  ",
		// Allow multiple marshaling of the same message
		AllowPartial: true,
	}

	buf, err := marshaler.Marshal(status)
	if err != nil {
		logrus.Fatalf("Error while marshaling the state to JSON: %s", err)
	}
	fmt.Printf("%s", buf)
}

func onelineStatus(status *pb.State) {
	if status.IsSuspended.GetValue() {
		fmt.Printf(" ⏸️ ")
	}
	if status.Builder.Generation != nil && status.Builder.IsEvaluating.GetValue() {
		fmt.Printf(" eval   %s/%s (%s)", status.Builder.Generation.SelectedRemoteName, status.Builder.Generation.SelectedBranchName,
			humanize.Time(status.Builder.Generation.EvalStartedAt.AsTime()))
	} else if status.Builder.Generation != nil && status.Builder.IsBuilding.GetValue() {
		fmt.Printf(" build  %s/%s (%s)", status.Builder.Generation.SelectedRemoteName, status.Builder.Generation.SelectedBranchName,
			humanize.Time(status.Builder.Generation.BuildStartedAt.AsTime()))
	} else if status.Builder.Generation != nil && status.Builder.Generation.EvalStatus == store.EvalFailed.String() {
		fmt.Printf(" %s/%s (%s)", status.Builder.Generation.SelectedRemoteName, status.Builder.Generation.SelectedBranchName,
			humanize.Time(status.Builder.Generation.EvalEndedAt.AsTime()))
	} else if status.Builder.Generation != nil && status.Builder.Generation.BuildStatus == store.BuildFailed.String() {
		fmt.Printf(" %s/%s (%s)", status.Builder.Generation.SelectedRemoteName, status.Builder.Generation.SelectedBranchName,
			humanize.Time(status.Builder.Generation.BuildEndedAt.AsTime()))
	} else if status.Deployer.Deployment != nil {
		switch status.Deployer.Deployment.Status {
		case store.StatusToString(store.Running):
			fmt.Printf(" deploy %s/%s (%s)", status.Deployer.Deployment.Generation.SelectedRemoteName, status.Deployer.Deployment.Generation.SelectedBranchName,
				humanize.Time(status.Deployer.Deployment.EndedAt.AsTime()))
		case store.StatusToString(store.Failed):
			fmt.Printf(" %s/%s (%s)", status.Deployer.Deployment.Generation.SelectedRemoteName, status.Deployer.Deployment.Generation.SelectedBranchName,
				humanize.Time(status.Deployer.Deployment.EndedAt.AsTime()))
		case store.StatusToString(store.Done):
			fmt.Printf(" %s/%s (%s)", status.Deployer.Deployment.Generation.SelectedRemoteName, status.Deployer.Deployment.Generation.SelectedBranchName,
				humanize.Time(status.Deployer.Deployment.EndedAt.AsTime()))
		}
	}
	if status.NeedToReboot.GetValue() {
		fmt.Printf(" ")
	}
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the status of the local machine",
	Args:  cobra.MinimumNArgs(0),
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
		if statusJson {
			jsonStatus(status)
		} else if statusOneline {
			onelineStatus(status)
		} else {
			longStatus(status)
		}
	},
}

func init() {
	statusCmd.PersistentFlags().BoolVarP(&statusOneline, "oneline", "", false, "oneline")
	statusCmd.PersistentFlags().BoolVarP(&statusJson, "json", "", false, "json")
	rootCmd.AddCommand(statusCmd)
}
