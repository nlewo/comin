package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/repository"
	"github.com/sirupsen/logrus"
)

type Data struct {
	Version string `json:"version"`
	// Deployments are order from the most recent to older
	Deployments []Deployment  `json:"deployments"`
	Generations []*Generation `json:"generations"`
}

type Store struct {
	Data
	mu              sync.Mutex
	filename        string
	capacityMain    int
	capacityTesting int
}

func New(filename string, capacityMain, capacityTesting int) *Store {
	st := Store{
		filename:        filename,
		capacityMain:    capacityMain,
		capacityTesting: capacityTesting,
	}
	st.Deployments = make([]Deployment, 0)
	st.Generations = make([]*Generation, 0)
	st.Version = "1"
	return &st

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

func (s *Store) GenerationEvalStarted(uuid uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.generationGet(uuid)
	if err != nil {
		return err
	}
	g.EvalEndedAt = time.Now().UTC()
	g.EvalStatus = Evaluating
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
	} else {
		g.BuildStatus = BuildFailed
		g.BuildErrStr = buildErr.Error()
	}
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

func (s *Store) DeploymentInsertAndCommit(dpl Deployment) (ok bool, evicted Deployment) {
	ok, evicted = s.DeploymentInsert(dpl)
	if ok {
		logrus.Infof("The deployment %s has been removed from store.json file", evicted.UUID)
	}
	if err := s.Commit(); err != nil {
		logrus.Errorf("Error while commiting the store.json file: %s", err)
		return
	}
	logrus.Infof("The new deployment %s has been commited to store.json file", dpl.UUID)
	return
}

// DeploymentInsert inserts a deployment and return an evicted
// deployment because the capacity has been reached.
func (s *Store) DeploymentInsert(dpl Deployment) (getsEvicted bool, evicted Deployment) {
	var qty, older int
	capacity := s.capacityMain
	if dpl.IsTesting() {
		capacity = s.capacityTesting
	}
	for i, d := range s.Deployments {
		if dpl.IsTesting() == d.IsTesting() {
			older = i
			qty += 1
		}
	}
	// If the capacity is reached, we remove the older elements
	if qty >= capacity {
		evicted = s.Deployments[older]
		getsEvicted = true
		s.Deployments = append(s.Deployments[:older], s.Deployments[older+1:]...)
	}
	s.Deployments = append([]Deployment{dpl}, s.Deployments...)
	return
}

func (s *Store) DeploymentList() []Deployment {
	return s.Deployments
}

func (s *Store) LastDeployment() (ok bool, d Deployment) {
	if len(s.DeploymentList()) > 1 {
		return true, s.DeploymentList()[0]
	}
	return
}

func (s *Store) Load() (err error) {
	var data Data
	content, err := os.ReadFile(s.filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return
	}
	err = json.Unmarshal(content, &data)
	if err != nil {
		return
	}
	// FIXME: we should check the version
	s.Deployments = data.Deployments
	logrus.Infof("Loaded %d deployments from %s", len(s.Deployments), s.filename)
	return
}

func (s *Store) Commit() (err error) {
	content, err := json.Marshal(s)
	if err != nil {
		return
	}
	err = os.WriteFile(s.filename, content, 0644)
	return
}
