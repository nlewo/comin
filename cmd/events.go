package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/nlewo/comin/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var jsonFlag bool

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
			if jsonFlag {
				jsonData, err := json.Marshal(streamer.Event)
				if err != nil {
					log.Fatalf("failed to marshal event to json: %v", err)
				}
				fmt.Println(string(jsonData))
			} else {
				fmt.Println(streamer.Event.Short())
			}
		}
	},
}

func init() {
	eventsCmd.Flags().BoolVar(&jsonFlag, "json", false, "Print events as JSON")
	rootCmd.AddCommand(eventsCmd)
}
