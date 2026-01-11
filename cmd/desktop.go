package cmd

import (
	"fmt"
	"log"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/nlewo/comin/internal/builder"
	"github.com/nlewo/comin/internal/client"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/store"
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
		err := beeep.Notify(title, "Agent desktop notifications started.", []byte{})
		if err != nil {
			panic(err)
		}

		test, _ := cmd.Flags().GetBool("test")
		if test {
			scenario()
		} else {
			opts := client.ClientOpts{
				UnixSocketPath: unixSocketPath,
			}
			client, err := client.New(opts)
			if err != nil {
				logrus.Fatal(err)
			}
			err = client.Events(handler)
			log.Fatalf("failed to consume to the event stream: %s", err)
		}
	},
}

func scenario() {
	g := protobuf.Generation{
		BuildReason:        builder.BuildReasonNeedBuild,
		SelectedRemoteName: "origin",
		SelectedBranchName: "main",
	}
	e := protobuf.Event{Type: &protobuf.Event_BuildStartedType{BuildStartedType: &protobuf.Event_BuildStarted{Generation: &g}}}
	handler(&e) // nolint: errcheck
	time.Sleep(time.Second)

	d := protobuf.Deployment{
		Status: store.StatusToString(store.Init),
	}
	e = protobuf.Event{Type: &protobuf.Event_DeploymentStartedType{DeploymentStartedType: &protobuf.Event_DeploymentStarted{Deployment: &d}}}
	handler(&e) // nolint: errcheck
	time.Sleep(time.Second)

	d = protobuf.Deployment{
		Status: store.StatusToString(store.Done),
	}
	e = protobuf.Event{Type: &protobuf.Event_DeploymentFinishedType{DeploymentFinishedType: &protobuf.Event_DeploymentFinished{Deployment: &d}}}
	handler(&e) // nolint: errcheck

	time.Sleep(time.Second)
	e = protobuf.Event{Type: &protobuf.Event_RebootRequired_{RebootRequired: &protobuf.Event_RebootRequired{Deployment: &d}}}
	handler(&e) // nolint: errcheck
}

func handler(event *protobuf.Event) error {
	var message string
	logrus.Debugf("received event: %s", event)
	switch v := event.Type.(type) {
	default:
		logrus.Errorf("unexpected type %T", v)
	case *protobuf.Event_Suspend_:
		message = "The agent is suspended."
	case *protobuf.Event_Resume_:
		message = "The agent is resumed."
	case *protobuf.Event_EvalStartedType:
	case *protobuf.Event_EvalFinishedType:
		dpl := event.Type.(*protobuf.Event_EvalFinishedType).EvalFinishedType.Generation
		switch dpl.EvalStatus {
		case "failed":
			message = "The evaluation has failed."
		case "evaluated":
		default:
			logrus.Errorf("unexpected evaluation status: %s", dpl.EvalStatus)
		}
	case *protobuf.Event_BuildStartedType:
		g := event.Type.(*protobuf.Event_BuildStartedType).BuildStartedType.Generation
		if g.BuildReason == builder.BuildReasonNeedBuild {
			message = fmt.Sprintf("A new commit from %s/%s is available.\nIt and will be built.", g.SelectedRemoteName, g.SelectedBranchName)
		}
	case *protobuf.Event_BuildFinishedType:
		dpl := event.Type.(*protobuf.Event_BuildFinishedType).BuildFinishedType.Generation
		switch dpl.BuildStatus {
		case "failed":
			message = "The build has failed."
		case "built":
		default:
			logrus.Errorf("unexpected deployment status: %s", dpl.BuildStatus)
		}
	case *protobuf.Event_DeploymentStartedType:
		message = "A deployment started."
	case *protobuf.Event_DeploymentFinishedType:
		dpl := event.Type.(*protobuf.Event_DeploymentFinishedType).DeploymentFinishedType.Deployment
		switch dpl.Status {
		case "done":
			message = "The deployment is finished."
		case "failed":
			message = "The deployment has failed."
		default:
			logrus.Errorf("unexpected deployment status: %s", dpl.Status)
		}
	case *protobuf.Event_RebootRequired_:
		message = "The machine needs to be rebooted to take the deployment into account."
	}
	if message != "" {
		err := beeep.Notify(title, message, []byte{})
		if err != nil {
			logrus.Errorf("failed to send the notification: %s", err)
		}
	}
	return nil
}

func init() {
	desktopCmd.Flags().StringVarP(&title, "title", "", "comin", "the notification title")
	desktopCmd.Flags().StringP("unix-socket-path", "", "/var/lib/comin/grpc.sock", "the GRPC Unix socket path")
	desktopCmd.Flags().BoolP("test", "", false, "do not get events from the agent but from predefined scenari")
	rootCmd.AddCommand(desktopCmd)
}
