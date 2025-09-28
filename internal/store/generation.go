package store

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func StringToEvalStatus(statusStr string) EvalStatus {
	switch statusStr {
	case "initialized":
		return EvalInit
	case "evaluating":
		return Evaluating
	case "evaluated":
		return Evaluated
	case "failed":
		return EvalFailed
	default:
		return EvalInit // Default value
	}
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

func StringToBuildStatus(statusStr string) BuildStatus {
	switch statusStr {
	case "initialized":
		return BuildInit
	case "building":
		return Building
	case "built":
		return Built
	case "failed":
		return BuildFailed
	default:
		return BuildInit
	}
}

func (s *Store) NewGeneration(hostname, repositoryPath, repositoryDir string, rs *protobuf.RepositoryStatus) (g protobuf.Generation) {
	g = protobuf.Generation{
		Uuid:                    uuid.New().String(),
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
		EvalStatus:              EvalInit.String(),
		BuildStatus:             BuildInit.String(),
	}
	s.data.Generations = append(s.data.Generations, &g)
	return
}

func GenerationShow(g *protobuf.Generation) {
	padding := "    "
	fmt.Printf("%sGeneration UUID %s\n", padding, g.Uuid)
	fmt.Printf("%sCommit ID %s from %s/%s\n", padding, g.SelectedCommitId, g.SelectedRemoteName, g.SelectedBranchName)
	fmt.Printf("%sCommit message: %s\n", padding, strings.Trim(g.SelectedCommitMsg, "\n"))

	if g.EvalStatus == EvalInit.String() {
		fmt.Printf("%sNo evaluation started\n", padding)
		return
	}
	if g.EvalStatus == Evaluating.String() {
		fmt.Printf("%sEvaluation started %s\n", padding, humanize.Time(g.EvalStartedAt.AsTime()))
		return
	}
	switch g.EvalStatus {
	case Evaluated.String():
		fmt.Printf("%sEvaluation succedded %s\n", padding, humanize.Time(g.EvalEndedAt.AsTime()))
		fmt.Printf("%s  DrvPath: %s\n", padding, g.DrvPath)
	case EvalFailed.String():
		fmt.Printf("%sEvaluation failed %s\n", padding, humanize.Time(g.EvalEndedAt.AsTime()))
	}
	if g.BuildStatus == BuildInit.String() {
		fmt.Printf("%sNo build started\n", padding)
		return
	}
	if g.BuildStatus == Building.String() {
		fmt.Printf("%sBuild started %s\n", padding, humanize.Time(g.BuildStartedAt.AsTime()))
		return
	}
	switch g.BuildStatus {
	case Built.String():
		fmt.Printf("%sBuilt %s\n", padding, humanize.Time(g.BuildEndedAt.AsTime()))
		fmt.Printf("%s  Outpath:  %s\n", padding, g.OutPath)
	case BuildFailed.String():
		fmt.Printf("%sBuild failed %s\n", padding, humanize.Time(g.BuildEndedAt.AsTime()))
	}
}

// generationsGC garbage collects unwanted generations. This is not thread safe.
func (s *Store) generationsGC() {
	alive := make([]*protobuf.Generation, 0)
	for _, g := range s.data.Generations {
		if g == s.lastEvalStarted || g == s.lastEvalFinished || g == s.lastBuildStarted || g == s.lastBuildFinished {
			alive = append(alive, g)
		}
	}
	for _, g := range s.data.Generations {
		keep := false
		for _, a := range alive {
			if g == a {
				keep = true
				break
			}
		}
		if !keep {
			logrus.Infof("store: generation %s removed from the store", g.Uuid)
		}
	}
	s.data.Generations = alive
}

func (s *Store) GenerationEvalStarted(uuid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.generationGet(uuid)
	if err != nil {
		return err
	}
	g.EvalStartedAt = timestamppb.New(time.Now().UTC())
	g.EvalStatus = Evaluating.String()
	s.lastEvalStarted = g
	s.generationsGC()
	return nil
}

func (s *Store) GenerationEvalFinished(uuid string, drvPath, outPath, machineId string, evalErr error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.generationGet(uuid)
	if err != nil {
		return err
	}
	if evalErr != nil {
		g.EvalErr = evalErr.Error()
		g.EvalStatus = EvalFailed.String()
	} else {
		g.EvalStatus = Evaluated.String()
	}
	g.DrvPath = drvPath
	g.OutPath = outPath
	g.MachineId = machineId
	g.EvalEndedAt = timestamppb.New(time.Now().UTC())
	s.lastEvalFinished = g
	s.generationsGC()
	return nil
}

func (s *Store) GenerationBuildStart(uuid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.generationGet(uuid)
	if err != nil {
		return err
	}
	g.BuildStartedAt = timestamppb.New(time.Now().UTC())
	g.BuildStatus = Building.String()
	s.lastBuildStarted = g
	s.generationsGC()
	return nil
}

func (s *Store) GenerationBuildFinished(uuid string, buildErr error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.generationGet(uuid)
	if err != nil {
		return err
	}
	g.BuildEndedAt = timestamppb.New(time.Now().UTC())
	if buildErr == nil {
		g.BuildStatus = Built.String()
		// We create a gcroots for the last built generation
		// in order to avoid the Nix garbage collector to
		// remove this store path which could be used later by
		// the deployment step.
		if _, err := os.Lstat(s.generationGcRoot); err == nil {
			if err := os.Remove(s.generationGcRoot); err != nil {
				logrus.Error(err)
			}
		}
		if err := os.Symlink(g.OutPath, s.generationGcRoot); err != nil {
			logrus.Errorf("Could not create the gcroot symlink for the generation %s: %s", g.Uuid, err)
		}
	} else {
		g.BuildStatus = BuildFailed.String()
		g.BuildErr = buildErr.Error()
	}
	s.lastBuildFinished = g
	s.generationsGC()
	return nil
}

// GenerationGet is thread safe and returns a copy
func (s *Store) GenerationGet(uuid string) (protobuf.Generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.data.Generations {
		if g.Uuid == uuid {
			return *(proto.CloneOf(g)), nil
		}
	}
	return protobuf.Generation{}, fmt.Errorf("store: no generation with uuid %s has been found", uuid)
}

func (s *Store) generationGet(uuid string) (g *protobuf.Generation, err error) {
	for _, g := range s.data.Generations {
		if g.Uuid == uuid {
			return g, nil
		}
	}
	return nil, fmt.Errorf("store: no generation with uuid %s has been found", uuid)
}

func GenerationHasToBeBuilt(g *protobuf.Generation) bool {
	return g.EvalStatus == Evaluated.String() && g.BuildStatus != Built.String()
}
