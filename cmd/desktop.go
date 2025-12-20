package cmd

import (
	"log"

	"github.com/gen2brain/beeep"
	"github.com/nlewo/comin/internal/client"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var title string

var desktopCmd = &cobra.Command{
	Use:   "desktop",
	Short: "Send desktop notifications ",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		unixSocketPath, _ := cmd.Flags().GetString("unix-socket-path")
		title, _ := cmd.Flags().GetString("title")
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		opts := client.ClientOpts{
			UnixSocketPath: unixSocketPath,
		}
		err := beeep.Notify(title, "comin desktop notifications started", []byte{})
		if err != nil {
			panic(err)
		}

		client, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		err = client.Events(func(event *protobuf.Event) error {
			var message string
			logrus.Debugf("received event: %s", event)
			switch v := event.Type.(type) {
			default:
				logrus.Errorf("unexpected type %T", v)
			case *protobuf.Event_Suspend_:
				message = "agent is suspended"
			case *protobuf.Event_Resume_:
				message = "agent is resumed"
			case *protobuf.Event_EvalStartedType:
				message = "evaluation started"
			case *protobuf.Event_EvalFinishedType:
				dpl := event.Type.(*protobuf.Event_EvalFinishedType).EvalFinishedType.Generation
				switch dpl.EvalStatus {
				case "failed":
					message = "evaluation failed"
				case "evaluated":
				default:
					logrus.Errorf("unexpected evaluation status: %s", dpl.EvalStatus)
				}
			case *protobuf.Event_BuildStartedType:
				message = "build started"
			case *protobuf.Event_BuildFinishedType:
				dpl := event.Type.(*protobuf.Event_BuildFinishedType).BuildFinishedType.Generation
				switch dpl.BuildStatus {
				case "failed":
					message = "build failed"
				case "built":
				default:
					logrus.Errorf("unexpected deployment status: %s", dpl.BuildStatus)
				}
			case *protobuf.Event_DeploymentStartedType:
				message = "deployment started"
			case *protobuf.Event_DeploymentFinishedType:
				dpl := event.Type.(*protobuf.Event_DeploymentFinishedType).DeploymentFinishedType.Deployment
				switch dpl.Status {
				case "done":
					message = "deployment finished"
				case "failed":
					message = "deployment finished"
				default:
					logrus.Errorf("unexpected deployment status: %s", dpl.Status)
				}
			}
			if message != "" {
				err := beeep.Notify(title, message, []byte{})
				if err != nil {
					logrus.Errorf("failed to send the notification: %s", err)
				}
			}
			return nil
		})
		log.Fatalf("failed to consume to the event stream: %s", err)
	},
}

func init() {
	desktopCmd.Flags().StringVarP(&title, "title", "", "comin", "the notification title")
	desktopCmd.Flags().StringP("unix-socket-path", "", "/var/lib/comin/grpc.sock", "the GRPC Unix socket path")
	rootCmd.AddCommand(desktopCmd)
}
