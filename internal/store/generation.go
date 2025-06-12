package store

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/repository"
	"github.com/sirupsen/logrus"
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
	UUID     uuid.UUID `json:"uuid"`
	FlakeUrl string    `json:"flake-url"`
	Hostname string    `json:"hostname"`

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

func (s *Store) NewGeneration(hostname, repositoryPath, repositoryDir string, rs repository.RepositoryStatus) (g Generation) {
	g = Generation{
		UUID:                    uuid.New(),
		FlakeUrl:                fmt.Sprintf("git+file://%s?dir=%s&rev=%s", repositoryPath, repositoryDir, rs.SelectedCommitId),
		Hostname:                hostname,
		SelectedRemoteName:      rs.SelectedRemoteName,
		SelectedBranchName:      rs.SelectedBranchName,
		SelectedCommitId:        rs.SelectedCommitId,
		SelectedCommitMsg:       rs.SelectedCommitMsg,
		SelectedBranchIsTesting: rs.SelectedBranchIsTesting,
		MainRemoteName:          rs.MainBranchName,
		MainBranchName:          rs.MainBranchName,
		MainCommitId:            rs.MainCommitId,
	}
	s.Generations = append(s.Generations, &g)
	return
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

// generationsGC garbage collects unwanted generations. This is not thread safe.
func (s *Store) generationsGC() {
	alive := make([]*Generation, 0)
	for _, g := range s.Generations {
		if g == s.lastEvalStarted || g == s.lastEvalFinished || g == s.lastBuildStarted || g == s.lastBuildFinished {
			alive = append(alive, g)
		}
	}
	for _, g := range s.Generations {
		keep := false
		for _, a := range alive {
			if g == a {
				keep = true
				break
			}
		}
		if !keep {
			logrus.Infof("store: generation %s removed from the store", g.UUID)
		}
	}
	s.Generations = alive
}

func (s *Store) GenerationEvalStarted(uuid uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.generationGet(uuid)
	if err != nil {
		return err
	}
	g.EvalStartedAt = time.Now().UTC()
	g.EvalStatus = Evaluating
	s.lastEvalStarted = g
	s.generationsGC()
	return nil
}

func (s *Store) GenerationEvalFinished(uuid uuid.UUID, drvPath, outPath, machineId string, evalErr error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.generationGet(uuid)
	if err != nil {
		return err
	}
	g.EvalErr = evalErr
	if evalErr != nil {
		g.EvalErrStr = evalErr.Error()
		g.EvalStatus = EvalFailed
	} else {
		g.EvalStatus = Evaluated
	}
	g.DrvPath = drvPath
	g.OutPath = outPath
	g.MachineId = machineId
	g.EvalEndedAt = time.Now().UTC()
	s.lastEvalFinished = g
	s.generationsGC()
	return nil
}

func (s *Store) GenerationBuildStart(uuid uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.generationGet(uuid)
	if err != nil {
		return err
	}
	g.BuildStartedAt = time.Now().UTC()
	g.BuildStatus = Building
	s.lastBuildStarted = g
	s.generationsGC()
	return nil
}

func (s *Store) GenerationBuildFinished(uuid uuid.UUID, buildErr error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.generationGet(uuid)
	if err != nil {
		return err
	}
	g.BuildEndedAt = time.Now().UTC()
	g.BuildErr = buildErr
	if buildErr == nil {
		g.BuildStatus = Built
		// We create a gcroots for the last built generation
		// in order to avoid the Nix garbage collector to
		// remove this store path which could be used later by
		// the deployment step.
		if _, err := os.Lstat(s.generationGcRoot); err == nil {
			os.Remove(s.generationGcRoot)
		}
		if err := os.Symlink(g.OutPath, s.generationGcRoot); err != nil {
			logrus.Error(err)
		}
	} else {
		g.BuildStatus = BuildFailed
		g.BuildErrStr = buildErr.Error()
	}
	s.lastBuildFinished = g
	s.generationsGC()
	return nil
}

// GenerationGet is thread safe
func (s *Store) GenerationGet(uuid uuid.UUID) (g Generation, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.Generations {
		if g.UUID == uuid {
			return *g, nil
		}
	}
	return Generation{}, fmt.Errorf("store: no generation with uuid %s has been found", uuid)
}

func (s *Store) generationGet(uuid uuid.UUID) (g *Generation, err error) {
	for _, g := range s.Generations {
		if g.UUID == uuid {
			return g, nil
		}
	}
	return &Generation{}, fmt.Errorf("store: no generation with uuid %s has been found", uuid)
}
