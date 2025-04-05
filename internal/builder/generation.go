package builder

import (
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

type EvalStatus int64

const (
	EvalInit EvalStatus = iota
	Evaluating
	Evaluated
	EvalFailed
)

func (s EvalStatus) String() string {
	switch s {
	case EvalInit:
		return "initialized"
	case Evaluating:
		return "evaluating"
	case Evaluated:
		return "evaluated"
	case EvalFailed:
		return "failed"
	}
	return "unknown"
}

type BuildStatus int64

const (
	BuildInit BuildStatus = iota
	Building
	Built
	BuildFailed
)

func (s BuildStatus) String() string {
	switch s {
	case BuildInit:
		return "initialized"
	case Building:
		return "building"
	case Built:
		return "built"
	case BuildFailed:
		return "failed"
	}
	return "unknown"
}

// We consider each created genration is legit to be deployed: hard
// reset is ensured at RepositoryStatus creation.
type Generation struct {
	UUID     string `json:"uuid"`
	FlakeUrl string `json:"flake-url"`
	Hostname string `json:"hostname"`

	SelectedRemoteUrl       string `json:"remote-url"`
	SelectedRemoteName      string `json:"remote-name"`
	SelectedBranchName      string `json:"branch-name"`
	SelectedCommitId        string `json:"commit-id"`
	SelectedCommitMsg       string `json:"commit-msg"`
	SelectedBranchIsTesting bool   `json:"branch-is-testing"`

	MainCommitId   string `json:"main-commit-id"`
	MainRemoteName string `json:"main-remote-name"`
	MainBranchName string `json:"main-branch-name"`

	EvalStatus    EvalStatus `json:"eval-status"`
	EvalStartedAt time.Time  `json:"eval-started-at"`
	EvalEndedAt   time.Time  `json:"eval-ended-at"`
	EvalErr       error      `json:"-"`
	EvalErrStr    string     `json:"eval-err"`
	OutPath       string     `json:"outpath"`
	DrvPath       string     `json:"drvpath"`

	MachineId string `json:"machine-id"`

	BuildStatus    BuildStatus `json:"build-status"`
	BuildStartedAt time.Time   `json:"build-started-at"`
	BuildEndedAt   time.Time   `json:"build-ended-at"`
	BuildErr       error       `json:"-"`
	BuildErrStr    string      `json:"build-err"`
}

func GenerationShow(g Generation) {
	padding := "    "
	fmt.Printf("%sGeneration UUID %s\n", padding, g.UUID)
	fmt.Printf("%sCommit ID %s from %s/%s\n", padding, g.SelectedCommitId, g.SelectedRemoteName, g.SelectedBranchName)
	fmt.Printf("%sCommit message: %s\n", padding, strings.Trim(g.SelectedCommitMsg, "\n"))

	if g.EvalStatus == EvalInit {
		fmt.Printf("%sNo evaluation started\n", padding)
		return
	}
	if g.EvalStatus == Evaluating {
		fmt.Printf("%sEvaluation started %s\n", padding, humanize.Time(g.EvalStartedAt))
		return
	}
	switch g.EvalStatus {
	case Evaluated:
		fmt.Printf("%sEvaluation succedded %s\n", padding, humanize.Time(g.EvalEndedAt))
		fmt.Printf("%s  DrvPath: %s\n", padding, g.DrvPath)
	case EvalFailed:
		fmt.Printf("%sEvaluation failed %s\n", padding, humanize.Time(g.EvalEndedAt))
	}
	if g.BuildStatus == BuildInit {
		fmt.Printf("%sNo build started\n", padding)
		return
	}
	if g.BuildStatus == Building {
		fmt.Printf("%sBuild started %s\n", padding, humanize.Time(g.BuildStartedAt))
		return
	}
	switch g.BuildStatus {
	case Built:
		fmt.Printf("%sBuilt %s\n", padding, humanize.Time(g.BuildEndedAt))
		fmt.Printf("%s  Outpath:  %s\n", padding, g.OutPath)
	case BuildFailed:
		fmt.Printf("%sBuild failed %s\n", padding, humanize.Time(g.BuildEndedAt))
	}
}
