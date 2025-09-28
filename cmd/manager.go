package cmd

import (
	"github.com/nlewo/comin/internal/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var suspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend build and deploy operations",
	Long:  "This command suspends the build and deploy operations. If a build is running, it is stopped. If a deployment is running, it is not interupted but future deployment will be suspended.",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		opts := client.ClientOpts{
			UnixSocketPath: "/var/lib/comin/grpc.sock",
		}
		c, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		err = c.Suspend()
		if err != nil {
			logrus.Fatal(err)
		}
	},
}
var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume build and deploy operations",
	Long:  "This command resumes the build and deploy operations. If a build has been suspended, it will be restarted.",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		opts := client.ClientOpts{
			UnixSocketPath: "/var/lib/comin/grpc.sock",
		}
		c, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		err = c.Resume()
		if err != nil {
			logrus.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(suspendCmd)
	rootCmd.AddCommand(resumeCmd)
}
