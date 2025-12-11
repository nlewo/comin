package cmd

import (
	"github.com/nlewo/comin/internal/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Trigger a fetch of all Git remotes",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		if unixSocketPath == "" {
			unixSocketPath = "/var/lib/comin/grpc.sock"
		}
		opts := client.ClientOpts{
			UnixSocketPath: unixSocketPath,
		}
		c, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		c.Fetch()
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)
	fetchCmd.PersistentFlags().StringVarP(&unixSocketPath, "unix-socket-path", "", "", "the GRPC Unix path (default to /var/lib/comin/grpc.sock)")
}
