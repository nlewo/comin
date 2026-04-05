package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/nlewo/comin/internal/client"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/store"
	"github.com/spf13/cobra"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	activeStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func formatTime(t time.Time) string {
	if time.Since(t) < 10*time.Second {
		return "less than 10 seconds ago"
	}
	return humanize.Time(t)
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func commitMsgSummary(msg string) string {
	parts := strings.SplitN(strings.TrimSpace(msg), "\n", 2)
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		return parts[0] + "..."
	}
	return parts[0]
}

func boolToString(v bool) string {
	if v {
		return warnStyle.Render("yes")
	}
	return dimStyle.Render("no")
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

// FetcherModel holds the current fetcher state and renders it.
type FetcherModel struct {
	isFetching       bool
	repositoryStatus *protobuf.RepositoryStatus
}

func (fm FetcherModel) View() string {
	var b strings.Builder

	var status string
	if fm.isFetching {
		status = activeStyle.Render("fetching...")
	} else {
		status = dimStyle.Render("idle")
	}
	b.WriteString(sectionStyle.Render("Fetcher") + "  " + status + "\n")

	if fm.repositoryStatus == nil {
		return b.String()
	}
	for _, r := range fm.repositoryStatus.Remotes {
		fetchedAt := ""
		if r.FetchedAt != nil {
			fetchedAt = "  " + dimStyle.Render(formatTime(r.FetchedAt.AsTime()))
		}
		b.WriteString("  " + labelStyle.Render("Remote: ") + r.Name + fetchedAt + "\n")
		b.WriteString("    " + labelStyle.Render("URL:     ") + r.Url + "\n")
		if r.FetchErrorMsg != "" {
			b.WriteString("    " + errorStyle.Render("Error: "+r.FetchErrorMsg) + "\n")
		}
		if r.Main != nil {
			commitID := r.Main.CommitId
			if len(commitID) > 8 {
				commitID = commitID[:8]
			}
			b.WriteString("    " + labelStyle.Render("main:    ") +
				commitID + "  " + commitMsgSummary(r.Main.CommitMsg) + "\n")
			if r.Main.ErrorMsg != "" {
				b.WriteString("      " + errorStyle.Render(r.Main.ErrorMsg) + "\n")
			}
		}
		if r.Testing != nil {
			commitID := r.Testing.CommitId
			if len(commitID) > 8 {
				commitID = commitID[:8]
			}
			b.WriteString("    " + labelStyle.Render("testing: ") +
				commitID + "  " + commitMsgSummary(r.Testing.CommitMsg) + "\n")
			if r.Testing.ErrorMsg != "" {
				b.WriteString("      " + errorStyle.Render(r.Testing.ErrorMsg) + "\n")
			}
		}
	}
	return b.String()
}

type BuilderModel struct {
	isEvaluating bool
	isBuilding   bool
	isSuspended  bool
	generation   *protobuf.Generation
}

func (bm BuilderModel) View() string {
	var b strings.Builder
	var status string
	if bm.isSuspended {
		status = warnStyle.Render("⏸ suspended")
	} else if bm.isEvaluating {
		status = activeStyle.Render("evaluating...")
	} else if bm.isBuilding {
		status = activeStyle.Render("building...")
	} else {
		status = dimStyle.Render("idle")
	}
	b.WriteString(sectionStyle.Render("Builder") + "  " + status + "\n")

	if bm.generation != nil {
		g := bm.generation
		commitID := g.SelectedCommitId
		if len(commitID) > 8 {
			commitID = commitID[:8]
		}
		b.WriteString("  " + labelStyle.Render("Commit:  ") +
			fmt.Sprintf("%s from %s/%s\n", commitID, g.SelectedRemoteName, g.SelectedBranchName))
		if msg := commitMsgSummary(g.SelectedCommitMsg); msg != "" {
			b.WriteString("  " + labelStyle.Render("Message: ") + msg + "\n")
		}

		switch g.EvalStatus {
		case store.Evaluating.String():
			b.WriteString("  " + labelStyle.Render("Eval:    ") +
				activeStyle.Render("running") +
				fmt.Sprintf(" since %s\n", formatTime(g.EvalStartedAt.AsTime())))
		case store.Evaluated.String():
			b.WriteString("  " + labelStyle.Render("Eval:    ") +
				successStyle.Render("succeeded") +
				fmt.Sprintf(" %s\n", formatTime(g.EvalEndedAt.AsTime())))
		case store.EvalFailed.String():
			b.WriteString("  " + labelStyle.Render("Eval:    ") +
				errorStyle.Render("failed") +
				fmt.Sprintf(" %s\n", formatTime(g.EvalEndedAt.AsTime())))
		}

		switch g.BuildStatus {
		case store.Building.String():
			b.WriteString("  " + labelStyle.Render("Build:   ") +
				activeStyle.Render("running") +
				fmt.Sprintf(" since %s\n", formatTime(g.BuildStartedAt.AsTime())))
		case store.Built.String():
			b.WriteString("  " + labelStyle.Render("Build:   ") +
				successStyle.Render("succeeded") +
				fmt.Sprintf(" %s\n", formatTime(g.BuildEndedAt.AsTime())))
		case store.BuildFailed.String():
			b.WriteString("  " + labelStyle.Render("Build:   ") +
				errorStyle.Render("failed") +
				fmt.Sprintf(" %s\n", formatTime(g.BuildEndedAt.AsTime())))
		}
	}
	return b.String()
}

type DeployerModel struct {
	isDeploying bool
	isSuspended bool
	deployment  *protobuf.Deployment
}

func (dm DeployerModel) View() string {
	var b strings.Builder
	var status string
	if dm.isSuspended {
		status = warnStyle.Render("⏸ suspended")
	} else if dm.isDeploying {
		status = activeStyle.Render("deploying...")
	} else {
		status = dimStyle.Render("idle")
	}
	b.WriteString(sectionStyle.Render("Deployer") + "  " + status + "\n")

	if dm.deployment != nil {
		d := dm.deployment
		if d.Generation != nil {
			g := d.Generation
			commitID := g.SelectedCommitId
			if len(commitID) > 8 {
				commitID = commitID[:8]
			}
			b.WriteString("  " + labelStyle.Render("Commit:    ") +
				fmt.Sprintf("%s from %s/%s\n", commitID, g.SelectedRemoteName, g.SelectedBranchName))
			if msg := commitMsgSummary(g.SelectedCommitMsg); msg != "" {
				b.WriteString("  " + labelStyle.Render("Message:   ") + msg + "\n")
			}
		}
		b.WriteString("  " + labelStyle.Render("Operation: ") + d.Operation + "\n")

		switch d.Status {
		case store.StatusToString(store.Running):
			b.WriteString("  " + labelStyle.Render("Status:    ") +
				activeStyle.Render("running") +
				fmt.Sprintf(" since %s\n", formatTime(d.StartedAt.AsTime())))
		case store.StatusToString(store.Done):
			b.WriteString("  " + labelStyle.Render("Status:    ") +
				successStyle.Render("done") +
				fmt.Sprintf(" %s\n", formatTime(d.EndedAt.AsTime())))
		case store.StatusToString(store.Failed):
			b.WriteString("  " + labelStyle.Render("Status:    ") +
				errorStyle.Render("failed") +
				fmt.Sprintf(" %s\n", formatTime(d.EndedAt.AsTime())))
		}
	}
	return b.String()
}

type ManagerModel struct {
	needToReboot  bool
	isSuspended   bool
	hostname      string
	connectionMsg string
	fetcher       FetcherModel
	builder       BuilderModel
	deployer      DeployerModel
}

func (mm ManagerModel) View() string {
	var b strings.Builder

	header := titleStyle.Render("comin")
	if mm.hostname != "" {
		header += "  " + mm.hostname
	}
	b.WriteString(header + "\n")
	if mm.connectionMsg != "" {
		b.WriteString("  " + warnStyle.Render("Disconnected: "+mm.connectionMsg) + "\n")
		return b.String()
	}
	b.WriteString("  " + dimStyle.Render("Connected") + "\n")
	b.WriteString("  " + labelStyle.Render("Reboot required: ") + boolToString(mm.needToReboot) + "\n")
	b.WriteString("  " + labelStyle.Render("Suspended:       ") + boolToString(mm.isSuspended) + "\n")
	b.WriteString("\n")
	b.WriteString(mm.fetcher.View())
	b.WriteString("\n")
	b.WriteString(mm.builder.View())
	b.WriteString("\n")
	b.WriteString(mm.deployer.View())
	return b.String()
}

type Model struct {
	manager ManagerModel
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

func updateModel(m Model, event *protobuf.Event) Model {
	switch e := event.Type.(type) {
	case *protobuf.Event_ManagerState_:
		state := e.ManagerState.State
		m.manager.needToReboot = state.NeedToReboot.GetValue()
		m.manager.isSuspended = state.IsSuspended.GetValue()
		if state.Builder != nil {
			m.manager.hostname = state.Builder.Hostname
			m.manager.builder.isEvaluating = state.Builder.IsEvaluating.GetValue()
			m.manager.builder.isBuilding = state.Builder.IsBuilding.GetValue()
			m.manager.builder.isSuspended = state.Builder.IsSuspended.GetValue()
			m.manager.builder.generation = state.Builder.Generation
		}
		if state.Deployer != nil {
			m.manager.deployer.isDeploying = state.Deployer.IsDeploying.GetValue()
			m.manager.deployer.isSuspended = state.Deployer.IsSuspended.GetValue()
			m.manager.deployer.deployment = state.Deployer.Deployment
		}
		if state.Fetcher != nil {
			m.manager.fetcher.isFetching = state.Fetcher.IsFetching.GetValue()
			m.manager.fetcher.repositoryStatus = state.Fetcher.RepositoryStatus
		}
	case *protobuf.Event_EvalStartedType:
		m.manager.builder.isEvaluating = true
		m.manager.builder.generation = e.EvalStartedType.Generation
	case *protobuf.Event_EvalFinishedType:
		m.manager.builder.isEvaluating = false
		m.manager.builder.generation = e.EvalFinishedType.Generation
	case *protobuf.Event_BuildStartedType:
		m.manager.builder.isBuilding = true
		m.manager.builder.generation = e.BuildStartedType.Generation
	case *protobuf.Event_BuildFinishedType:
		m.manager.builder.isBuilding = false
		m.manager.builder.generation = e.BuildFinishedType.Generation
	case *protobuf.Event_DeploymentStartedType:
		m.manager.deployer.isDeploying = true
		m.manager.deployer.deployment = e.DeploymentStartedType.Deployment
	case *protobuf.Event_DeploymentFinishedType:
		m.manager.deployer.isDeploying = false
		m.manager.deployer.deployment = e.DeploymentFinishedType.Deployment
	case *protobuf.Event_Fetched_:
		m.manager.fetcher.isFetching = false
		m.manager.fetcher.repositoryStatus = e.Fetched.RepositoryStatus
	case *protobuf.Event_Suspend_:
		m.manager.isSuspended = true
	case *protobuf.Event_Resume_:
		m.manager.isSuspended = false
	case *protobuf.Event_RebootRequired_:
		m.manager.needToReboot = true
	}
	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case client.Streamer:
		if msg.Event != nil {
			m.manager.connectionMsg = ""
			m = updateModel(m, msg.Event)
		} else {
			m.manager.connectionMsg = msg.FailureMsg
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
				if m.manager.isSuspended {
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
