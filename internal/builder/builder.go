package builder

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/repository"
	"github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"
)

type EvalFunc func(ctx context.Context, flakeUrl string, hostname string) (drvPath string, outPath string, machineId string, err error)
type BuildFunc func(ctx context.Context, drvPath string) error

type Builder struct {
	store          *store.Store
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

	// GenerationUUID is the generation UUID currently managed by
	// the builder. This generation can be evaluating, evaluated,
	// building or built.
	GenerationUUID *uuid.UUID

	// EvaluationDone is used to be notified a evaluation is finished. Be careful since only a single goroutine can listen it.
	EvaluationDone chan uuid.UUID
	// BuildDone is used to be notified a build is finished. Be careful since only a single goroutine can listen it.
	BuildDone chan uuid.UUID

	evaluator   Exec
	evaluatorWg *sync.WaitGroup

	buildator   Exec
	buildatorWg *sync.WaitGroup
}

func New(store *store.Store, repositoryPath, repositoryDir, hostname string, evalTimeout time.Duration, evalFunc EvalFunc, buildTimeout time.Duration, buildFunc BuildFunc) *Builder {
	logrus.Infof("builder: initialization with repositoryPath=%s, repositoryDir=%s, hostname=%s, evalTimeout=%fs, buildTimeout=%fs, )",
		repositoryPath, repositoryDir, hostname, evalTimeout.Seconds(), buildTimeout.Seconds())
	return &Builder{
		store:          store,
		repositoryPath: repositoryPath,
		repositoryDir:  repositoryDir,
		hostname:       hostname,
		evalFunc:       evalFunc,
		evalTimeout:    evalTimeout,
		buildFunc:      buildFunc,
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
			logrus.Error("builder: generation %s not found in the store: %s", generationUUID, err)
		}
	}

	return State{
		Hostname:       b.hostname,
		IsBuilding:     b.IsBuilding,
		IsEvaluating:   b.IsEvaluating,
		Generation:     generation,
		GenerationUUID: generationUUID,
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
func (b *Builder) Eval(rs repository.RepositoryStatus) error {
	ctx := context.TODO()
	// This is to prempt the builder since we don't need to allow
	// several evaluation in parallel
	b.Stop()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.IsEvaluating = true

	g := b.store.NewGeneration(b.hostname, b.repositoryPath, b.repositoryDir, rs)
	if err := b.store.GenerationEvalStarted(g.UUID); err != nil {
		return err
	}
	b.GenerationUUID = &g.UUID

	evaluator := &Evaluator{
		hostname: b.hostname,
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
		if err := b.store.GenerationEvalFinished(
			g.UUID,
			evaluator.drvPath,
			evaluator.outPath,
			evaluator.machineId,
			b.evaluator.err,
		); err != nil {
			logrus.Errorf("builder: %s", err)
		}

		b.IsEvaluating = false
		select {
		case b.EvaluationDone <- g.UUID:
		default:
		}
	}()
	return nil
}

// Build builds a generation which has been previously evaluated.
func (b *Builder) Build(generationUUID uuid.UUID) error {
	ctx := context.TODO()
	b.mu.Lock()
	defer b.mu.Unlock()
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
	if b.IsBuilding {
		return fmt.Errorf("the builder is already building")
	}
	if generation.BuildStatus == store.Built {
		return fmt.Errorf("the generation is already built")
	}

	if err := b.store.GenerationBuildStart(generationUUID); err != nil {
		return err
	}
	b.IsBuilding = true
	buildator := &Buildator{
		drvPath:   generation.DrvPath,
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
		err := b.store.GenerationBuildFinished(generationUUID, b.buildator.err)
		if err != nil {
			logrus.Error(err)
		}
		b.IsBuilding = false
		select {
		case b.BuildDone <- generationUUID:
		default:
		}
	}()
	return nil
}
