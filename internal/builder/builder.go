// This package is used to evaluate and build the Nix expression
// describing a system to be deployed.
//
// It works asyncronously but evaluation and build are
// sequentials. When an evaluation is started, it cancels a running
// evaluation or running build.

// The Builder can only manage a single generation. This generation
// first needs to be evaluated and then built.
package builder

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/executor"
	"github.com/nlewo/comin/internal/repository"
	"github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"
)

type Builder struct {
	store          *store.Store
	executor       executor.Executor
	hostname       string
	repositoryPath string
	repositoryDir  string
	evalTimeout    time.Duration
	buildTimeout   time.Duration

	mu           sync.Mutex
	isEvaluating atomic.Bool
	isBuilding   atomic.Bool

	// GenerationUUID is the generation UUID currently managed by
	// the builder. This generation can be evaluating, evaluated,
	// building or built.
	// To access this generation, you need to query the store.
	GenerationUUID *uuid.UUID

	// EvaluationDone is used to be notified a evaluation is finished. Be careful since only a single goroutine can listen it.
	EvaluationDone chan uuid.UUID
	// BuildDone is used to be notified a build is finished. Be careful since only a single goroutine can listen it.
	BuildDone chan uuid.UUID

	evaluator   Exec
	evaluatorWg *sync.WaitGroup

	buildator   Exec
	buildatorWg *sync.WaitGroup

	isSuspended bool
}

func New(store *store.Store, executor executor.Executor, repositoryPath, repositoryDir, hostname string, evalTimeout time.Duration, buildTimeout time.Duration) *Builder {
	logrus.Infof("builder: initialization with repositoryPath=%s, repositoryDir=%s, hostname=%s, evalTimeout=%fs, buildTimeout=%fs, )",
		repositoryPath, repositoryDir, hostname, evalTimeout.Seconds(), buildTimeout.Seconds())
	return &Builder{
		store:          store,
		executor:       executor,
		repositoryPath: repositoryPath,
		repositoryDir:  repositoryDir,
		hostname:       hostname,
		evalTimeout:    evalTimeout,
		buildTimeout:   buildTimeout,
		EvaluationDone: make(chan uuid.UUID, 1),
		BuildDone:      make(chan uuid.UUID, 1),
		evaluatorWg:    &sync.WaitGroup{},
		buildatorWg:    &sync.WaitGroup{},
	}
}

type State struct {
	Hostname       string            `json:"hostname"`
	IsBuilding     bool              `json:"is_building"`
	IsEvaluating   bool              `json:"is_evaluating"`
	Generation     *store.Generation `json:"generation"`
	GenerationUUID string            `json:"generation_uuid"`
	IsSuspended    bool              `json:"is_suspended"`
}

func (b *Builder) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	var generation *store.Generation
	var generationUUID string

	if b.GenerationUUID != nil {
		generationUUID = b.GenerationUUID.String()
		if g, err := b.store.GenerationGet(*b.GenerationUUID); err == nil {
			generation = &g
		} else {
			logrus.Errorf("builder: generation %s not found in the store: %s", generationUUID, err)
		}
	}
	return State{
		Hostname:       b.hostname,
		IsBuilding:     b.isBuilding.Load(),
		IsEvaluating:   b.isEvaluating.Load(),
		Generation:     generation,
		GenerationUUID: generationUUID,
		IsSuspended:    b.isSuspended,
	}
}

func (b *Builder) IsEvaluating() bool {
	return b.isEvaluating.Load()
}

func (b *Builder) stopEval() {
	b.evaluator.Stop()
	b.evaluatorWg.Wait()
	b.isEvaluating.Store(false)
}

// stopBuild stops the build. If a build is actually running, it
// returns the building generationUUID, otherwise, it returns nil.
// Because the builder is not locked during the whole stop process, it
// is possible to return a generationUUID while the build of this
// generation is finished. This is however not a important issue since
// the builder will just rebuild the generation.
func (b *Builder) stopBuild() {
	b.buildator.Stop()
	b.buildatorWg.Wait()

	b.mu.Lock()
	defer b.mu.Unlock()
	b.isBuilding.Store(false)
}

// Stop stops the evaluator and the builder is required and wait until
// they have been actually stopped.
func (b *Builder) Stop() {
	b.stopEval()
	b.stopBuild()

	b.mu.Lock()
	defer b.mu.Unlock()
}

type Evaluator struct {
	flakeUrl string
	hostname string

	evalFunc executor.EvalFunc

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
	buildFunc executor.BuildFunc
}

func (r *Buildator) Run(ctx context.Context) (err error) {
	return r.buildFunc(ctx, r.drvPath)
}

// Eval evaluates a generation. It cancels current any generation
// evaluation or build.
//
// At the end of the evaluation, if the storepath is already in the
// Nix store, it then consider the build is done. In this case, it
// doesn't notify for the end of the evaluation but for the end of the
// build.
func (b *Builder) Eval(rs repository.RepositoryStatus) error {
	ctx := context.TODO()
	// This is to prempt the builder since we don't need to allow
	// several evaluation in parallel
	b.Stop()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.isEvaluating.Store(true)

	g := b.store.NewGeneration(b.hostname, b.repositoryPath, b.repositoryDir, rs)
	if err := b.store.GenerationEvalStarted(g.UUID); err != nil {
		return err
	}
	b.GenerationUUID = &g.UUID

	evaluator := &Evaluator{
		hostname: b.hostname,
		flakeUrl: g.FlakeUrl,
		evalFunc: b.executor.Eval,
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
		if err := b.store.GenerationEvalFinished(
			g.UUID,
			evaluator.drvPath,
			evaluator.outPath,
			evaluator.machineId,
			b.evaluator.getErr(),
		); err != nil {
			logrus.Errorf("builder: %s", err)
		}

		b.isEvaluating.Store(false)
		if b.executor.IsStorePathExist(evaluator.outPath) {
			if err := b.store.GenerationBuildStart(g.UUID); err != nil {
				logrus.Errorf("builder: %s", err)
			}
			if err := b.store.GenerationBuildFinished(g.UUID, nil); err != nil {
				logrus.Errorf("builder: %s", err)
			}
			select {
			case b.BuildDone <- g.UUID:
			default:
			}

		} else {
			select {
			case b.EvaluationDone <- g.UUID:
			default:
			}
		}
	}()
	return nil
}

func (b *Builder) Suspend() error {
	if b.isSuspended {
		return fmt.Errorf("the builder is already suspended")
	}
	b.stopBuild()
	logrus.Infof("builder: builder is suspended")

	b.mu.Lock()
	defer b.mu.Unlock()
	b.isSuspended = true
	return nil
}

func (b *Builder) Resume() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.isSuspended {
		return fmt.Errorf("the builder is not suspended")
	} else {
		b.isSuspended = false
		generation, err := b.store.GenerationGet(*b.GenerationUUID)
		if err != nil {
			return err
		}
		if store.GenerationHasToBeBuilt(generation) {
			logrus.Infof("builder: builder is resumed and generation %s has to be built", b.GenerationUUID)
			// TODO: expose the error in the builder state
			if err := b.build(*b.GenerationUUID); err != nil {
				logrus.Error(err)
			}
		} else {
			logrus.Infof("builder: builder is resumed while no generation has to be built")
		}
	}
	return nil
}

// SubmitBuild submits a generation for building. If the builder is
// suspended, the generation is only built once resumed, otherwise, it
// is built immediately.
func (b *Builder) SubmitBuild(generationUUID uuid.UUID) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.isSuspended {
		logrus.Infof("builder: build submitted for generation %s while the builder is suspended", generationUUID.String())
	} else {
		// TODO: expose the error in the builder state
		if err := b.build(generationUUID); err != nil {
			logrus.Error(err)
		}
	}
}

// build builds a generation which has been previously evaluated. This is not thread safe.
func (b *Builder) build(generationUUID uuid.UUID) error {
	logrus.Infof("builder: build of generation %s is starting", generationUUID.String())
	ctx := context.TODO()
	if b.GenerationUUID != nil && generationUUID != *b.GenerationUUID {
		return fmt.Errorf("another generation is evaluating or evaluated")
	}

	generation, err := b.store.GenerationGet(generationUUID)
	if err != nil {
		return err
	}
	if generation.EvalStatus != store.Evaluated {
		return fmt.Errorf("the generation is not evaluated")
	}
	if b.isBuilding.Load() {
		return fmt.Errorf("the builder is already building")
	}
	if generation.BuildStatus == store.Built {
		return fmt.Errorf("the generation is already built")
	}

	if err := b.store.GenerationBuildStart(generationUUID); err != nil {
		return err
	}
	b.isBuilding.Store(true)
	buildator := &Buildator{
		drvPath:   generation.DrvPath,
		buildFunc: b.executor.Build,
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
		err := b.store.GenerationBuildFinished(generationUUID, b.buildator.getErr())
		if err != nil {
			logrus.Error(err)
		}
		b.isBuilding.Store(false)
		select {
		case b.BuildDone <- generationUUID:
		default:
		}
	}()
	return nil
}
