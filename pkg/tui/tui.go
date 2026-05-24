package tui

import (
	"fmt"
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/dustin/go-humanize"
	"github.com/nlewo/comin/internal/store"
	"github.com/nlewo/comin/pkg/protobuf"
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

// FetcherModel holds the current fetcher state and renders it.
type FetcherModel struct {
	IsFetching       bool
	RepositoryStatus *protobuf.RepositoryStatus
}

func (fm FetcherModel) View() string {
	var b strings.Builder

	var status string
	if fm.IsFetching {
		status = activeStyle.Render("fetching...")
	} else {
		status = dimStyle.Render("idle")
	}
	b.WriteString(sectionStyle.Render("Fetcher") + "  " + status + "\n")

	if fm.RepositoryStatus == nil {
		return b.String()
	}
	for _, r := range fm.RepositoryStatus.Remotes {
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

// BuilderModel holds the current builder state and renders it.
type BuilderModel struct {
	IsEvaluating bool
	IsBuilding   bool
	IsSuspended  bool
	Generation   *protobuf.Generation
}

func (bm BuilderModel) View() string {
	var b strings.Builder
	var status string
	if bm.IsSuspended {
		status = warnStyle.Render("⏸ suspended")
	} else if bm.IsEvaluating {
		status = activeStyle.Render("evaluating...")
	} else if bm.IsBuilding {
		status = activeStyle.Render("building...")
	} else {
		status = dimStyle.Render("idle")
	}
	b.WriteString(sectionStyle.Render("Builder") + "  " + status + "\n")

	if bm.Generation != nil {
		g := bm.Generation
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

// DeployerModel holds the current deployer state and renders it.
type DeployerModel struct {
	IsDeploying bool
	IsSuspended bool
	Deployment  *protobuf.Deployment
}

func (dm DeployerModel) View() string {
	var b strings.Builder
	var status string
	if dm.IsSuspended {
		status = warnStyle.Render("⏸ suspended")
	} else if dm.IsDeploying {
		status = activeStyle.Render("deploying...")
	} else {
		status = dimStyle.Render("idle")
	}
	b.WriteString(sectionStyle.Render("Deployer") + "  " + status + "\n")

	if dm.Deployment != nil {
		d := dm.Deployment
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
		operation := d.Operation
		b.WriteString("  " + labelStyle.Render("Operation: ") + operation + "\n")

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

// ManagerModel holds the current manager state and renders it.
type ManagerModel struct {
	NeedToReboot  bool
	IsSuspended   bool
	Hostname      string
	ConnectionMsg string
	Fetcher       FetcherModel
	Builder       BuilderModel
	Deployer      DeployerModel
}

func (mm ManagerModel) View() string {
	var b strings.Builder

	header := titleStyle.Render("comin")
	if mm.Hostname != "" {
		header += "  " + mm.Hostname
	}
	b.WriteString(header + "\n")
	if mm.ConnectionMsg != "" {
		b.WriteString("  " + warnStyle.Render("Disconnected: "+mm.ConnectionMsg) + "\n")
		return b.String()
	}
	b.WriteString("  " + dimStyle.Render("Connected") + "\n")
	b.WriteString("  " + labelStyle.Render("Reboot required: ") + boolToString(mm.NeedToReboot) + "\n")
	b.WriteString("  " + labelStyle.Render("Suspended:       ") + boolToString(mm.IsSuspended) + "\n")
	b.WriteString("\n")
	b.WriteString(mm.Fetcher.View())
	b.WriteString("\n")
	b.WriteString(mm.Builder.View())
	b.WriteString("\n")
	b.WriteString(mm.Deployer.View())
	return b.String()
}

// UpdateManager updates the ManagerModel based on the received event.
func UpdateManager(manager *ManagerModel, event *protobuf.Event) {
	switch e := event.Type.(type) {
	case *protobuf.Event_ManagerState_:
		state := e.ManagerState.State
		manager.NeedToReboot = state.NeedToReboot.GetValue()
		manager.IsSuspended = state.IsSuspended.GetValue()
		if state.Builder != nil {
			manager.Hostname = state.Builder.Hostname
			manager.Builder.IsEvaluating = state.Builder.IsEvaluating.GetValue()
			manager.Builder.IsBuilding = state.Builder.IsBuilding.GetValue()
			manager.Builder.IsSuspended = state.Builder.IsSuspended.GetValue()
			manager.Builder.Generation = state.Builder.Generation
		}
		if state.Deployer != nil {
			manager.Deployer.IsDeploying = state.Deployer.IsDeploying.GetValue()
			manager.Deployer.IsSuspended = state.Deployer.IsSuspended.GetValue()
			manager.Deployer.Deployment = state.Deployer.Deployment
		}
		if state.Fetcher != nil {
			manager.Fetcher.IsFetching = state.Fetcher.IsFetching.GetValue()
			manager.Fetcher.RepositoryStatus = state.Fetcher.RepositoryStatus
		}
	case *protobuf.Event_EvalStartedType:
		manager.Builder.IsEvaluating = true
		manager.Builder.Generation = e.EvalStartedType.Generation
	case *protobuf.Event_EvalFinishedType:
		manager.Builder.IsEvaluating = false
		manager.Builder.Generation = e.EvalFinishedType.Generation
	case *protobuf.Event_BuildStartedType:
		manager.Builder.IsBuilding = true
		manager.Builder.Generation = e.BuildStartedType.Generation
	case *protobuf.Event_BuildFinishedType:
		manager.Builder.IsBuilding = false
		manager.Builder.Generation = e.BuildFinishedType.Generation
	case *protobuf.Event_DeploymentStartedType:
		manager.Deployer.IsDeploying = true
		manager.Deployer.Deployment = e.DeploymentStartedType.Deployment
	case *protobuf.Event_DeploymentFinishedType:
		manager.Deployer.IsDeploying = false
		manager.Deployer.Deployment = e.DeploymentFinishedType.Deployment
	case *protobuf.Event_Fetched_:
		manager.Fetcher.IsFetching = false
		manager.Fetcher.RepositoryStatus = e.Fetched.RepositoryStatus
	case *protobuf.Event_Suspend_:
		manager.IsSuspended = true
	case *protobuf.Event_Resume_:
		manager.IsSuspended = false
	case *protobuf.Event_RebootRequired_:
		manager.NeedToReboot = true
	}
}
