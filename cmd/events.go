package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/nlewo/comin/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Watch for comin agent events",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		opts := client.ClientOpts{
			UnixSocketPath: "/var/lib/comin/grpc.sock",
		}
		c, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		ch := c.Stream(context.Background())
		for streamer := range ch {
			if streamer.FailureMsg != "" {
				log.Fatalf("failed to consume to the event stream: %s", streamer.FailureMsg)
			}
			fmt.Println(streamer.Event)
		}
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)
}
