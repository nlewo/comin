package cmd

import (
	"fmt"
	"log"

	"github.com/nlewo/comin/internal/client"
	"github.com/nlewo/comin/internal/protobuf"
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
		client, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		err = client.Events(func(event *protobuf.Event) error {
			fmt.Println(event)
			return nil
		})
		if err != nil {
			log.Fatalf("failed to consume to the event stream: %s", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)
}
