package generation

import (
	"context"
	"fmt"
	"time"

	"github.com/nlewo/comin/internal/repository"
)

type Status int64

const (
	Init Status = iota
	Evaluating
	Evaluated
	Building
	Built
)

// We consider each created genration is legit to be deployed: hard
// reset is ensured at RepositoryStatus creation.
type Generation struct {
	flakeUrl  string
	hostname  string
	machineId string

	Status Status

	RepositoryStatus repository.RepositoryStatus

	EvalStartedAt time.Time
	evalTimeout   time.Duration
	evalFunc      EvalFunc
	EvalEndedAt   time.Time
	EvalErr       error
	OutPath       string
	DrvPath       string
	EvalMachineId string

	BuildStartedAt time.Time
	BuildEndedAt   time.Time
	BuildErr       error
	buildFunc      BuildFunc
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
		RepositoryStatus: repositoryStatus,
		evalTimeout:      6 * time.Second,
		evalFunc:         evalFunc,
		buildFunc:        buildFunc,
		flakeUrl:         flakeUrl,
		hostname:         hostname,
		machineId:        machineId,
		Status:           Init,
	}
}

func (g Generation) UpdateEval(r EvalResult) Generation {
	g.EvalEndedAt = r.EndAt
	g.DrvPath = r.DrvPath
	g.OutPath = r.OutPath
	g.EvalErr = r.Err
	g.EvalMachineId = r.MachineId
	g.Status = Evaluated
	return g
}

func (g Generation) UpdateBuild(r BuildResult) Generation {
	g.BuildEndedAt = r.EndAt
	g.BuildErr = r.Err
	g.Status = Built
	return g
}

func (g Generation) Eval(ctx context.Context) (result chan EvalResult) {
	result = make(chan EvalResult)
	fn := func() {
		ctx, cancel := context.WithTimeout(ctx, g.evalTimeout)
		defer cancel()
		drvPath, outPath, machineId, err := g.evalFunc(ctx, g.flakeUrl, g.hostname)
		evaluationResult := EvalResult{
			EndAt: time.Now(),
		}
		if err == nil {
			evaluationResult.DrvPath = drvPath
			evaluationResult.OutPath = outPath
			if machineId != "" && g.machineId != machineId {
				evaluationResult.Err = fmt.Errorf("The evaluated comin.machineId '%s' is different from the /etc/machine-id '%s' of this machine",
					g.machineId, machineId)
			}
		} else {
			evaluationResult.Err = err
		}
		result <- evaluationResult
	}
	go fn()
	return result
}

func (g Generation) Build(ctx context.Context) (result chan BuildResult) {
	result = make(chan BuildResult)
	fn := func() {
		ctx, cancel := context.WithTimeout(ctx, g.evalTimeout)
		defer cancel()
		err := g.buildFunc(ctx, g.DrvPath)
		buildResult := BuildResult{
			EndAt: time.Now(),
		}
		buildResult.Err = err
		result <- buildResult
	}
	go fn()
	return result
}
