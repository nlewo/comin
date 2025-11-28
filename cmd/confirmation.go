package cmd

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/nlewo/comin/internal/client"
	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var confirmationCmd = &cobra.Command{
	Use: "confirmation",
}

var confirmationAcceptCmd = &cobra.Command{
	Use:   "accept",
	Short: "Accept a generation for building and/or a deploying",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		opts := client.ClientOpts{
			UnixSocketPath: "/var/lib/comin/grpc.sock",
		}
		c, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		state, err := c.GetManagerState()
		if err != nil {
			logrus.Fatal(err)
		}
		build_uuid := state.BuildConfirmer.Submitted
		deploy_uuid := state.DeployConfirmer.Submitted
		if build_uuid != "" {
			fmt.Printf("Generation %s accepted for building and deploying\n", build_uuid)
			c.Confirm(build_uuid, "all") // nolint
		} else if deploy_uuid != "" {
			fmt.Printf("Generation %s accepted for deploying\n", deploy_uuid)
			c.Confirm(deploy_uuid, "deploy") // nolint
		} else {
			fmt.Printf("No confirmation is required\n")
		}
	},
}

var confirmationShowCmd = &cobra.Command{
	Use:  "show",
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
}
