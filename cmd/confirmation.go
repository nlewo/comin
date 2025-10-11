package cmd

import (
	"fmt"

	"github.com/nlewo/comin/internal/client"
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
		build_uuid := state.Controller.Build.Needed
		deploy_uuid := state.Controller.Deploy.Needed
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
		confirmationShow(status.Controller)
	},
}

func confirmerShow(c *protobuf.Confirmer, for_ string) {
	if !c.Enabled.GetValue() {
		fmt.Printf("Confirmation for %s is disabled\n", for_)
		return
	}
	empty := false
	if c.Allowed != "" {
		empty = false
		fmt.Printf("Allowed for %s: %s\n", for_, c.Allowed)
	}
	if c.Needed != "" {
		empty = false
		fmt.Printf("Confirmation needed for %s: %s\n", for_, c.Needed)
	}
	if empty {
		fmt.Printf("No confirmation for %s\n", for_)
	}
}

func confirmationShow(controller *protobuf.Controller) {
	confirmerShow(controller.Build, "building")
	confirmerShow(controller.Deploy, "deploying")
}

func init() {
	rootCmd.AddCommand(confirmationCmd)
	confirmationCmd.AddCommand(confirmationShowCmd)
	confirmationCmd.AddCommand(confirmationAcceptCmd)
}
