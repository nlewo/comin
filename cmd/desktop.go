package cmd

import (
	"fmt"
	"time"

	"fyne.io/systray"
	"github.com/dustin/go-humanize"
	"github.com/gen2brain/beeep"
	"github.com/nlewo/comin/cmd/icons"
	"github.com/nlewo/comin/internal/builder"
	"github.com/nlewo/comin/internal/client"
	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/store"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var title string

const buildTitle = "build"
const deployTitle = "deploy"

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
			tray, _ := (cmd.Flags().GetBool("tray"))
			if tray {
				logrus.Println("Loading tray.")
				systray.Run(func() { onTrayReady(client) }, onTrayExit)
			} else {
				err = client.Events(desktopEventHandler)
				logrus.Fatalf("failed to consume to the event stream: %s", err)
			}
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
	desktopEventHandler(&e) // nolint: errcheck
	time.Sleep(time.Second)

	d := protobuf.Deployment{
		Status: store.StatusToString(store.Init),
	}
	e = protobuf.Event{Type: &protobuf.Event_DeploymentStartedType{DeploymentStartedType: &protobuf.Event_DeploymentStarted{Deployment: &d}}}
	desktopEventHandler(&e) // nolint: errcheck
	time.Sleep(time.Second)

	d = protobuf.Deployment{
		Status: store.StatusToString(store.Done),
	}
	e = protobuf.Event{Type: &protobuf.Event_DeploymentFinishedType{DeploymentFinishedType: &protobuf.Event_DeploymentFinished{Deployment: &d}}}
	desktopEventHandler(&e) // nolint: errcheck

	time.Sleep(time.Second)
	e = protobuf.Event{Type: &protobuf.Event_RebootRequired_{RebootRequired: &protobuf.Event_RebootRequired{Deployment: &d}}}
	desktopEventHandler(&e) // nolint: errcheck
}

func generateEventMessage(event *protobuf.Event) string {
	var message string
	logrus.Debugf("received event: %s", event)
	switch v := event.Type.(type) {
	default:
		logrus.Errorf("unexpected type %T", v)
	case *protobuf.Event_Fetched_:
	case *protobuf.Event_ManagerState_:
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
			message = fmt.Sprintf("A new commit from %s/%s is building.", g.SelectedRemoteName, g.SelectedBranchName)
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
	case *protobuf.Event_ConfirmationSubmittedType:
		uuid := event.Type.(*protobuf.Event_ConfirmationSubmittedType).ConfirmationSubmittedType.GetUuid()
		message = fmt.Sprintf("A confirmation request was submitted. %s", uuid)
	}
	return message
}

func desktopEventHandler(event *protobuf.Event) error {
	message := generateEventMessage(event)
	displayNotification(message)
	return nil
}

func displayNotification(message string) {
	if message != "" {
		logrus.Println(fmt.Sprintf("Displaying notification: %s", message))
		err := beeep.Notify(title, message, []byte{})
		if err != nil {
			logrus.Errorf("failed to send the notification: %s", err)
		}
	}
}

func onTrayReady(client client.Client) {
	systray.SetIcon(icons.GetIcon(client))
	systray.SetTitle("comin")
	systray.SetTooltip("comin")
	go refreshAvailableConfirmationsLoop(client)
	go client.Events(trayEventHandler(client)) // nolint: errcheck
}

func onTrayExit() {}

func trayEventHandler(client client.Client) func(*protobuf.Event) error {
	return func(event *protobuf.Event) error {
		// Reuse the notification handler
		go desktopEventHandler(event) // nolint: errcheck
		// Watch the events for specifically the confirmation events so we can update the tray icon.
		switch event.Type.(type) {
		case *protobuf.Event_ConfirmationSubmittedType:
			systray.SetIcon(icons.GetIcon(client))
		case *protobuf.Event_ConfirmationConfirmedType:
			systray.SetIcon(icons.GetIcon(client))
		case *protobuf.Event_ConfirmationCancelledType:
			systray.SetIcon(icons.GetIcon(client))
		}
		return nil
	}
}

func refreshAvailableConfirmationsLoop(client client.Client) {
	buildItem, deployItem := initializeTrayMenu(client)
	// Reload statuses when the tray is opened.
	for range systray.TrayOpenedCh {
		status, err := client.GetManagerState()
		if err != nil {
			logrus.Errorln("Failed to retrieve manager state.")
		}
		logrus.Debugln(fmt.Sprintf("status is %s", status))
		if status == nil {
			updatePendingItem(nil, buildTitle, buildItem)
			updatePendingItem(nil, deployTitle, deployItem)
		} else {
			updatePendingItem(status.BuildConfirmer, buildTitle, buildItem)
			updatePendingItem(status.DeployConfirmer, deployTitle, deployItem)
		}
		// Also refresh the icon in case it's desynced for any reason.
		systray.SetIcon(icons.GetIcon(client))
	}
}

func generateTrayMessage(c *protobuf.Confirmer, for_ string) (string, bool) {
	if c.Confirmed != "" {
		return fmt.Sprintf("%s: Confirmed: %s.", for_, c.Confirmed), false
	} else if c.Submitted != "" {
		base := fmt.Sprintf("%s: Confirmation needed: %s.", for_, c.Submitted)
		if c.Mode == int64(manager.Auto) {
			return fmt.Sprintf("%s\nAuto confirmation in %s\n",
					base,
					humanize.Time(c.AutoconfirmStartedAt.AsTime().Add(time.Duration(c.AutoconfirmDuration*int64(time.Second))))),
				true
		} else {
			return base, true
		}
	} else {
		return fmt.Sprintf("%s: No confirmations needed.", for_), false
	}
}

// Initialize the items in the tray menu.
func initializeTrayMenu(client client.Client) (*systray.MenuItem, *systray.MenuItem) {
	// Header
	addMenuItem("comin", "", func() {}).Disable()
	systray.AddSeparator()

	// Status items
	var buildItem *systray.MenuItem
	var deployItem *systray.MenuItem
	status, err := client.GetManagerState()
	if err != nil {
		logrus.Errorln("Failed to retrieve manager state.")
	}
	var buildConfirmer *protobuf.Confirmer
	var deployConfirmer *protobuf.Confirmer
	// If the status can't be loaded, we still want to create the tray items.
	if status != nil {
		buildConfirmer = status.BuildConfirmer
		deployConfirmer = status.DeployConfirmer
	}
	buildItem = makePendingItem(buildConfirmer, buildTitle, func() {
		status, err := client.GetManagerState()
		if err != nil {
			displayNotification("Failed to confirm build.")
			logrus.Errorln("Failed to retrieve manager state.")
		} else {
			if status.BuildConfirmer.Submitted != "" {
				client.Confirm(status.BuildConfirmer.Submitted, buildTitle) // nolint: errcheck
			}
		}
	})
	deployItem = makePendingItem(deployConfirmer, deployTitle, func() {
		status, err := client.GetManagerState()
		if err != nil {
			displayNotification("Failed to confirm deployment.")
			logrus.Errorln("Failed to retrieve manager state.")
		} else {
			if status.DeployConfirmer.Submitted != "" {
				client.Confirm(status.DeployConfirmer.Submitted, deployTitle) // nolint: errcheck
			}
		}
	})
	// Quit item
	systray.AddSeparator()
	addMenuItem("Quit", "Quit the tray application", func() {
		logrus.Println("Quit was pressed. Closing.")
		systray.Quit()
	})
	return buildItem, deployItem
}

func updatePendingItem(c *protobuf.Confirmer, for_ string, item *systray.MenuItem) {
	if c == nil {
		item.SetTitle(fmt.Sprintf("%s: Failed to connect to comin service.", for_))
		item.Disable()
		return
	}
	title, shouldEnable := generateTrayMessage(c, for_)
	item.SetTitle(title)
	if shouldEnable {
		item.Enable()
	} else {
		item.Disable()
	}
}

func makePendingItem(c *protobuf.Confirmer, for_ string, callback func()) *systray.MenuItem {
	if c == nil {
		item := addMenuItem(fmt.Sprintf("%s: Failed to connect to comin service.", for_), "", callback)
		item.Disable()
		return item
	}
	title, shouldEnable := generateTrayMessage(c, for_)
	item := addMenuItem(title, "", callback)
	if shouldEnable {
		item.Enable()
	} else {
		item.Disable()
	}
	return item
}

// Helper function to create a tray item with a callback.
// The goroutine is never stopped, so only use for items that will not be removed from the tray.
func addMenuItem(title string, tooltip string, callback func()) *systray.MenuItem {
	item := systray.AddMenuItem(title, tooltip)
	go func() {
		for range item.ClickedCh {
			callback()
		}
	}()
	return item
}

func init() {
	desktopCmd.Flags().StringVarP(&title, "title", "", "comin", "the notification title")
	desktopCmd.Flags().StringP("unix-socket-path", "", "/var/lib/comin/grpc.sock", "the GRPC Unix socket path")
	desktopCmd.Flags().BoolP("test", "", false, "do not get events from the agent but from predefined scenari")
	desktopCmd.Flags().BoolP("tray", "", false, "Show in the system tray")
	rootCmd.AddCommand(desktopCmd)
}
