package generation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/repository"
	"github.com/sirupsen/logrus"
)

type Status int64

const (
	Init Status = iota
	Evaluating
	EvaluationSucceeded
	EvaluationFailed
	Building
	BuildSucceeded
	BuildFailed
)

func StatusToString(status Status) string {
	switch status {
	case Init:
		return "init"
	case Evaluating:
		return "evaluating"
	case EvaluationSucceeded:
		return "evaluation-succeeded"
	case EvaluationFailed:
		return "evaluation-failed"
	case Building:
		return "building"
	case BuildSucceeded:
		return "build-succeeded"
	case BuildFailed:
		return "build-failed"
	}
	return ""
}

func StatusFromString(status string) Status {
	switch status {
	case "init":
		return Init
	case "evaluating":
		return Evaluating
	case "evaluation-succeeded":
		return EvaluationSucceeded
	case "evaluation-failed":
		return EvaluationFailed
	case "building":
		return Building
	case "build-succeeded":
		return BuildSucceeded
	case "build-failed":
		return BuildFailed
	}
	return Init
}

// We consider each created genration is legit to be deployed: hard
// reset is ensured at RepositoryStatus creation.
type Generation struct {
	UUID      string `json:"uuid"`
	FlakeUrl  string `json:"flake-url"`
	Hostname  string `json:"hostname"`
	MachineId string `json:"machine-id"`

	Status Status `json:"status"`

	SelectedRemoteName      string `json:"remote-name"`
	SelectedBranchName      string `json:"branch-name"`
	SelectedCommitId        string `json:"commit-id"`
	SelectedCommitMsg       string `json:"commit-msg"`
	SelectedBranchIsTesting bool   `json:"branch-is-testing"`

	EvalStartedAt time.Time `json:"eval-started-at"`
	evalTimeout   time.Duration
	evalFunc      EvalFunc
	evalCh        chan EvalResult

	EvalEndedAt   time.Time `json:"eval-ended-at"`
	EvalErr       error     `json:"-"`
	OutPath       string    `json:"outpath"`
	DrvPath       string    `json:"drvpath"`
	EvalMachineId string    `json:"eval-machine-id"`

	BuildStartedAt time.Time `json:"build-started-at"`
	BuildEndedAt   time.Time `json:"build-ended-at"`
	buildErr       error     `json:"-"`
	buildFunc      BuildFunc
	buildCh        chan BuildResult
}

type EvalFunc func(ctx context.Context, flakeUrl string, hostname string) (drvPath string, outPath string, machineId string, err error)
type BuildFunc func(ctx context.Context, drvPath string) error

type BuildResult struct {
	EndAt time.Time
	Err   error
}

type EvalResult struct {
	EndAt     time.Time
	OutPath   string
	DrvPath   string
	MachineId string
	Err       error
}

func New(repositoryStatus repository.RepositoryStatus, flakeUrl, hostname, machineId string, evalFunc EvalFunc, buildFunc BuildFunc) Generation {
	return Generation{
		UUID:                    uuid.NewString(),
		SelectedRemoteName:      repositoryStatus.SelectedRemoteName,
		SelectedBranchName:      repositoryStatus.SelectedBranchName,
		SelectedCommitId:        repositoryStatus.SelectedCommitId,
		SelectedCommitMsg:       repositoryStatus.SelectedCommitMsg,
		SelectedBranchIsTesting: repositoryStatus.SelectedBranchIsTesting,
		evalTimeout:             6 * time.Second,
		evalFunc:                evalFunc,
		buildFunc:               buildFunc,
		FlakeUrl:                flakeUrl,
		Hostname:                hostname,
		MachineId:               machineId,
		Status:                  Init,
	}
}

func (g Generation) EvalCh() chan EvalResult {
	return g.evalCh
}

func (g Generation) BuildCh() chan BuildResult {
	return g.buildCh
}

func (g Generation) UpdateEval(r EvalResult) Generation {
	logrus.Debugf("Eval done with %#v", r)
	g.EvalEndedAt = r.EndAt
	g.DrvPath = r.DrvPath
	g.OutPath = r.OutPath
	g.EvalMachineId = r.MachineId
	g.EvalErr = r.Err
	if g.EvalErr == nil {
		g.Status = EvaluationSucceeded
	} else {
		g.Status = EvaluationFailed
	}
	return g
}

func (g Generation) UpdateBuild(r BuildResult) Generation {
	logrus.Debugf("Build done with %#v", r)
	g.BuildEndedAt = r.EndAt
	g.buildErr = r.Err
	if g.buildErr == nil {
		g.Status = BuildSucceeded
	} else {
		g.Status = BuildFailed
	}
	return g
}

func (g Generation) Eval(ctx context.Context) Generation {
	g.evalCh = make(chan EvalResult)
	g.EvalStartedAt = time.Now()
	g.Status = Evaluating

	fn := func() {
		ctx, cancel := context.WithTimeout(ctx, g.evalTimeout)
		defer cancel()
		drvPath, outPath, machineId, err := g.evalFunc(ctx, g.FlakeUrl, g.Hostname)
		evaluationResult := EvalResult{
			EndAt: time.Now(),
		}
		if err == nil {
			evaluationResult.DrvPath = drvPath
			evaluationResult.OutPath = outPath
			evaluationResult.MachineId = machineId
			if machineId != "" && g.MachineId != machineId {
				evaluationResult.Err = fmt.Errorf("The evaluated comin.machineId '%s' is different from the /etc/machine-id '%s' of this machine",
					machineId, g.MachineId)
			}
		} else {
			evaluationResult.Err = err
		}
		g.evalCh <- evaluationResult
	}
	go fn()
	return g
}

func (g Generation) Build(ctx context.Context) Generation {
	g.buildCh = make(chan BuildResult)
	g.BuildStartedAt = time.Now()
	g.Status = Building
	fn := func() {
		ctx, cancel := context.WithTimeout(ctx, g.evalTimeout)
		defer cancel()
		err := g.buildFunc(ctx, g.DrvPath)
		buildResult := BuildResult{
			EndAt: time.Now(),
		}
		buildResult.Err = err
		g.buildCh <- buildResult
	}
	go fn()
	return g
}
