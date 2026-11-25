package cmd

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/nlewo/comin/pkg/client"
	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var confirmationCmd = &cobra.Command{
	Use: "confirmation",
}

var confirmationFor string

// acceptTarget picks the generation to confirm and the confirmer(s) to
// confirm it for. Without forWhat, a generation waiting for building is
// accepted for deploying too.
func acceptTarget(buildUuid, deployUuid, forWhat string) (uuid, scope string) {
	switch forWhat {
	case "build":
		return buildUuid, "build"
	case "deploy":
		return deployUuid, "deploy"
	}
	if buildUuid != "" {
		return buildUuid, "all"
	}
	return deployUuid, "deploy"
}

var confirmationAcceptCmd = &cobra.Command{
	Use:   "accept",
	Short: "Accept a generation for building and/or a deploying",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		if confirmationFor != "" && confirmationFor != "build" && confirmationFor != "deploy" {
			logrus.Fatalf("--for must be build or deploy, not %q", confirmationFor)
		}
		opts := client.ClientOpts{
			UnixSocketPath: "/run/comin-grpc.sock",
		}
		c, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		state, err := c.GetManagerState()
		if err != nil {
			logrus.Fatal(err)
		}
		uuid, scope := acceptTarget(state.BuildConfirmer.Submitted, state.DeployConfirmer.Submitted, confirmationFor)
		switch {
		case uuid == "":
			fmt.Printf("No confirmation is required\n")
		case scope == "all":
			fmt.Printf("Generation %s accepted for building and deploying\n", uuid)
			c.Confirm(uuid, scope) // nolint
		default:
			fmt.Printf("Generation %s accepted for %sing\n", uuid, scope)
			c.Confirm(uuid, scope) // nolint
		}
	},
}

var confirmationShowCmd = &cobra.Command{
	Use:  "show",
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		opts := client.ClientOpts{
			UnixSocketPath: "/run/comin-grpc.sock",
		}
		c, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		status, err := c.GetManagerState()
		if err != nil {
			logrus.Fatal(err)
		}
		confirmerShow(status.BuildConfirmer, "building")
		confirmerShow(status.DeployConfirmer, "deploying")
	},
}

func confirmerShow(c *protobuf.Confirmer, for_ string) {
	empty := false
	if c.Confirmed != "" {
		empty = false
		fmt.Printf("Confirmation submitted for %s: %s\n", for_, c.Confirmed)
	}
	if c.Submitted != "" {
		empty = false
		fmt.Printf("Confirmation needed for %s: %s\n", for_, c.Submitted)
		if c.Mode == int64(manager.Auto) {
			fmt.Printf("  Auto confirmation in %s\n", humanize.Time(c.AutoconfirmStartedAt.AsTime().Add(time.Duration(c.AutoconfirmDuration*int64(time.Second)))))

		}
	}
	if empty {
		fmt.Printf("No confirmation for %s\n", for_)
	}
}

func init() {
	rootCmd.AddCommand(confirmationCmd)
	confirmationCmd.AddCommand(confirmationShowCmd)
	confirmationCmd.AddCommand(confirmationAcceptCmd)
	confirmationAcceptCmd.Flags().StringVar(&confirmationFor, "for", "",
		"accept only the generation waiting for building or for deploying (build or deploy); "+
			"without it, a generation waiting for building is accepted for both")
}
