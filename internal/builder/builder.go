package builder

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/repository"
	"github.com/sirupsen/logrus"
)

type EvalFunc func(ctx context.Context, flakeUrl string, hostname string) (drvPath string, outPath string, machineId string, err error)
type BuildFunc func(ctx context.Context, drvPath string) error

type Builder struct {
	hostname       string
	repositoryPath string
	repositoryDir  string
	evalTimeout    time.Duration
	buildTimeout   time.Duration
	evalFunc       EvalFunc
	buildFunc      BuildFunc

	mu           sync.Mutex
	IsEvaluating bool
	IsBuilding   bool

	generation *Generation

	// EvaluationDone is used to be notified a evaluation is finished. Be careful since only a single goroutine can listen it.
	EvaluationDone chan Generation
	// BuildDone is used to be notified a build is finished. Be careful since only a single goroutine can listen it.
	BuildDone chan Generation

	evaluator   Exec
	evaluatorWg *sync.WaitGroup

	buildator   Exec
	buildatorWg *sync.WaitGroup
}

func New(repositoryPath, repositoryDir, hostname string, evalTimeout time.Duration, evalFunc EvalFunc, buildTimeout time.Duration, buildFunc BuildFunc) *Builder {
	logrus.Infof("builder: initialization with repositoryPath=%s, repositoryDir=%s, hostname=%s, evalTimeout=%fs, buildTimeout=%fs, )",
		repositoryPath, repositoryDir, hostname, evalTimeout.Seconds(), buildTimeout.Seconds())
	return &Builder{
		repositoryPath: repositoryPath,
		repositoryDir:  repositoryDir,
		hostname:       hostname,
		evalFunc:       evalFunc,
		evalTimeout:    evalTimeout,
		buildFunc:      buildFunc,
		buildTimeout:   buildTimeout,
		EvaluationDone: make(chan Generation, 1),
		BuildDone:      make(chan Generation, 1),
		evaluatorWg:    &sync.WaitGroup{},
		buildatorWg:    &sync.WaitGroup{},
	}
}

func (b *Builder) GetGeneration() Generation {
	b.mu.Lock()
	defer b.mu.Unlock()
	return *b.generation
}

type State struct {
	Hostname     string      `json:"is_hostname"`
	IsBuilding   bool        `json:"is_building"`
	IsEvaluating bool        `json:"is_evaluating"`
	Generation   *Generation `json:"generation"`
}

func (b *Builder) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return State{
		Hostname:     b.hostname,
		IsBuilding:   b.IsBuilding,
		IsEvaluating: b.IsEvaluating,
		Generation:   b.generation,
	}
}

// Stop stops the evaluator and the builder is required and wait until
// they have been actually stopped.
func (b *Builder) Stop() {
	b.evaluator.Stop()
	b.buildator.Stop()

	b.evaluatorWg.Wait()
	b.buildatorWg.Wait()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.IsEvaluating = false
	b.IsBuilding = false
}

type Evaluator struct {
	flakeUrl string
	hostname string

	evalFunc EvalFunc

	drvPath   string
	outPath   string
	machineId string
}

func (r *Evaluator) Run(ctx context.Context) (err error) {
	r.drvPath, r.outPath, r.machineId, err = r.evalFunc(ctx, r.flakeUrl, r.hostname)
	return err
}

type Buildator struct {
	drvPath   string
	buildFunc BuildFunc
}

func (r *Buildator) Run(ctx context.Context) (err error) {
	return r.buildFunc(ctx, r.drvPath)
}

// Eval evaluates a generation. It cancels current any generation
// evaluation or build.
func (b *Builder) Eval(rs repository.RepositoryStatus) {
	ctx := context.TODO()
	// This is to prempt the builder since we don't need to allow
	// several evaluation in parallel
	b.Stop()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.IsEvaluating = true
	g := Generation{
		UUID:                    uuid.NewString(),
		FlakeUrl:                fmt.Sprintf("git+file://%s?dir=%s&rev=%s", b.repositoryPath, b.repositoryDir, rs.SelectedCommitId),
		Hostname:                b.hostname,
		SelectedRemoteName:      rs.SelectedRemoteName,
		SelectedBranchName:      rs.SelectedBranchName,
		SelectedCommitId:        rs.SelectedCommitId,
		SelectedCommitMsg:       rs.SelectedCommitMsg,
		SelectedBranchIsTesting: rs.SelectedBranchIsTesting,
		MainRemoteName:          rs.MainBranchName,
		MainBranchName:          rs.MainBranchName,
		MainCommitId:            rs.MainCommitId,
		EvalStartedAt:           time.Now().UTC(),
		EvalStatus:              Evaluating,
	}
	b.generation = &g

	evaluator := &Evaluator{
		hostname: g.Hostname,
		flakeUrl: g.FlakeUrl,
		evalFunc: b.evalFunc,
	}
	b.evaluator = NewExec(evaluator, b.evalTimeout)

	// This is to wait until the evaluator is stopped
	b.evaluatorWg.Add(1)
	b.evaluator.Start(ctx)

	go func() {
		defer b.evaluatorWg.Done()
		b.evaluator.Wait()
		b.mu.Lock()
		defer b.mu.Unlock()
		b.generation.EvalErr = b.evaluator.err
		if b.evaluator.err != nil {
			b.generation.EvalErrStr = b.evaluator.err.Error()
			b.generation.EvalStatus = EvalFailed
		} else {
			b.generation.EvalStatus = Evaluated
		}
		b.generation.EvalErr = b.evaluator.err
		b.generation.DrvPath = evaluator.drvPath
		b.generation.OutPath = evaluator.outPath
		b.generation.MachineId = evaluator.machineId
		b.generation.EvalEndedAt = time.Now().UTC()
		b.IsEvaluating = false
		select {
		case b.EvaluationDone <- *b.generation:
		default:
		}
	}()
}

// Build builds a generation which has been previously evaluated.
func (b *Builder) Build() error {
	ctx := context.TODO()
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.generation == nil || b.generation.EvalStatus != Evaluated {
		return fmt.Errorf("the generation is not evaluated")
	}
	if b.IsBuilding {
		return fmt.Errorf("the builder is already building")
	}
	if b.generation.BuildStatus == Built {
		return fmt.Errorf("the generation is already built")
	}
	b.generation.BuildStartedAt = time.Now().UTC()
	b.generation.BuildStatus = Building
	b.IsBuilding = true

	buildator := &Buildator{
		drvPath:   b.generation.DrvPath,
		buildFunc: b.buildFunc,
	}
	b.buildator = NewExec(buildator, b.buildTimeout)

	// This is to wait until the evaluator is stopped
	b.buildatorWg.Add(1)
	b.buildator.Start(ctx)

	go func() {
		defer b.buildatorWg.Done()
		b.buildator.Wait()
		b.mu.Lock()
		defer b.mu.Unlock()
		b.generation.BuildEndedAt = time.Now().UTC()
		b.generation.BuildErr = b.buildator.err
		if b.buildator.err == nil {
			b.generation.BuildStatus = Built
		} else {
			b.generation.BuildStatus = BuildFailed
			b.generation.BuildErrStr = b.buildator.err.Error()
		}
		b.IsBuilding = false
		select {
		case b.BuildDone <- *b.generation:
		default:
		}
	}()
	return nil
}
