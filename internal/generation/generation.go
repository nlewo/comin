package generation

import (
	"context"
	"fmt"
	"time"

	"github.com/nlewo/comin/internal/repository"
)

// We consider each created genration is legit to be deployed: hard
// reset is ensured at RepositoryStatus creation.
type Generation struct {
	flakeUrl  string
	hostname  string
	machineId string

	RepositoryStatus repository.RepositoryStatus

	EvalStartedAt time.Time
	EvalResult    EvalResult
	evalTimeout   time.Duration
	evalFunc      EvalFunc

	BuildStartedAt time.Time
	BuildResult    BuildResult
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
	}
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
			if g.machineId != "" && g.machineId != machineId {
				evaluationResult.Err = fmt.Errorf("The defined comin.machineId (%s) is different to the machine id (%s) of this machine",
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
		err := g.buildFunc(ctx, g.EvalResult.DrvPath)
		buildResult := BuildResult{
			EndAt: time.Now(),
		}
		buildResult.Err = err
		result <- buildResult
	}
	go fn()
	return result
}
