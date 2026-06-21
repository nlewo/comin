package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nlewo/comin/pkg/client"
	"github.com/nlewo/comin/pkg/tui"
	"github.com/spf13/cobra"
)

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type keyMap struct {
	Fetch   key.Binding
	Suspend key.Binding
	Quit    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Fetch, k.Suspend, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Fetch, k.Suspend, k.Quit}}
}

var keys = keyMap{
	Fetch: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "fetch"),
	),
	Suspend: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "suspend/resume"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

type Model struct {
	manager tui.ManagerModel
	stream  chan client.Streamer
	client  *client.Client
	help    help.Model
	keys    keyMap
}

func consumeStream(m Model) func() tea.Msg {
	return func() tea.Msg {
		return <-m.stream
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(consumeStream(m), tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case client.Streamer:
		if msg.Event != nil {
			m.manager.ConnectionMsg = ""
			tui.UpdateManager(&m.manager, msg.Event)
		} else {
			m.manager.ConnectionMsg = msg.FailureMsg
		}
		return m, consumeStream(m)
	case tickMsg:
		return m, tick()
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Fetch):
			if m.client != nil {
				m.client.Fetch()
			}
		case key.Matches(msg, m.keys.Suspend):
			if m.client != nil {
				if m.manager.IsSuspended {
					m.client.Resume() // nolint: errcheck
				} else {
					m.client.Suspend() // nolint: errcheck
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.manager.View())
	b.WriteString("\nHelp: " + m.help.View(m.keys) + "\n")
	return b.String()
}

var watchTest bool

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch the comin state",
	Run: func(cmd *cobra.Command, args []string) {
		var ch chan client.Streamer
		var c *client.Client
		if watchTest {
			ch = watchScenario()
		} else {
			if unixSocketPath == "" {
				unixSocketPath = "/var/lib/comin/grpc.sock"
			}
			opts := client.ClientOpts{
				UnixSocketPath: unixSocketPath,
			}
			cVal, err := client.New(opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			c = &cVal
			ch = c.Stream(context.Background())
		}
		p := tea.NewProgram(Model{
			stream: ch,
			client: c,
			help:   help.New(),
			keys:   keys,
		})
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	watchCmd.Flags().BoolVar(&watchTest, "test", false, "use a test scenario instead of a live gRPC connection")
	rootCmd.AddCommand(watchCmd)
}
