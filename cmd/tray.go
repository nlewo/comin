package cmd

import (
	"fmt"
	"time"

	"github.com/nlewo/comin/cmd/icons"
	"github.com/nlewo/comin/internal/client"
	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/protobuf"

	"fyne.io/systray"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var trayCmd = &cobra.Command{
	Use:   "tray",
	Short: "Open Desktop Tray Application",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Println("Loading tray.")
		unixSocketPath, _ := cmd.Flags().GetString("unix-socket-path")
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		opts := client.ClientOpts{
			UnixSocketPath: unixSocketPath,
		}
		client, err := client.New(opts)
		if err != nil {
			logrus.Fatal(err)
		}
		systray.Run(func() { onReady(client) }, onExit)
	},
}

func onReady(client client.Client) {
	systray.SetIcon(icons.GetIcon(client))
	systray.SetTitle("comin")
	systray.SetTooltip("comin")
	go refreshAvailableConfirmationsLoop(client)
	go client.Events(trayEventHandler(client)) // nolint: errcheck
}

func onExit() {}

func trayEventHandler(client client.Client) func(*protobuf.Event) error {
	return func(event *protobuf.Event) error {
		// Reuse the handler from ./desktop.go for notifications.
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
	buildItem, deployItem := initialzeTrayMenu(client)
	// Reload statuses when the tray is opened.
	for range systray.TrayOpenedCh {
		status, err := client.GetManagerState()
		if err != nil {
			logrus.Errorln("Failed to retrieve manager state.")
		}
		logrus.Debugln(fmt.Sprintf("status is %s", status))
		if status == nil {
			updatePendingItem(nil, "build", buildItem)
			updatePendingItem(nil, "deploy", deployItem)
		} else {
			updatePendingItem(status.BuildConfirmer, "build", buildItem)
			updatePendingItem(status.DeployConfirmer, "deploy", deployItem)
		}
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

func initialzeTrayMenu(client client.Client) (*systray.MenuItem, *systray.MenuItem) {
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
	buildItem = makePendingItem(buildConfirmer, "build", func() {
		status, err := client.GetManagerState()
		if err != nil {
			displayNotification("Failed to confirm build.")
			logrus.Errorln("Failed to retrieve manager state.")
		} else {
			if status.BuildConfirmer.Submitted != "" {
				client.Confirm(status.BuildConfirmer.Submitted, "build") // nolint: errcheck
			}
		}
	})
	deployItem = makePendingItem(deployConfirmer, "deploy", func() {
		status, err := client.GetManagerState()
		if err != nil {
			displayNotification("Failed to confirm deployment.")
			logrus.Errorln("Failed to retrieve manager state.")
		} else {
			if status.DeployConfirmer.Submitted != "" {
				client.Confirm(status.DeployConfirmer.Submitted, "deploy") // nolint: errcheck
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

func addMenuItem(title string, tooltip string, watcher func()) *systray.MenuItem {
	item := systray.AddMenuItem(title, tooltip)
	go func() {
		for range item.ClickedCh {
			watcher()
		}
	}()
	return item
}

func init() {
	trayCmd.Flags().StringP("unix-socket-path", "", "/var/lib/comin/grpc.sock", "the GRPC Unix socket path")
	trayCmd.Flags().StringVarP(&title, "title", "", "comin", "the notification title")
	rootCmd.AddCommand(trayCmd)
}
