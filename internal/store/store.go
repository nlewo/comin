package store

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

type State struct {
	Deployments []Deployment  `json:"deployments"`
	Generations []*Generation `json:"generations"`
}

type Data struct {
	Version string `json:"version"`
	// Deployments are order from the most recent to older
	Deployments []Deployment  `json:"deployments"`
	Generations []*Generation `json:"generations"`
}

type Store struct {
	Data
	mu               sync.Mutex
	filename         string
	generationGcRoot string
	capacityMain     int
	capacityTesting  int

	lastEvalStarted   *Generation
	lastEvalFinished  *Generation
	lastBuildStarted  *Generation
	lastBuildFinished *Generation
}

func New(filename, gcRootsDir string, capacityMain, capacityTesting int) (*Store, error) {
	st := Store{
		filename:         filename,
		generationGcRoot: gcRootsDir + "/last-built-generation",
		capacityMain:     capacityMain,
		capacityTesting:  capacityTesting,
	}
	if err := os.MkdirAll(gcRootsDir, os.ModeDir); err != nil {
		return nil, err
	}
	st.Deployments = make([]Deployment, 0)
	st.Generations = make([]*Generation, 0)
	st.Version = "1"
	return &st, nil
}

func (s *Store) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return State{
		Generations: s.Generations,
		Deployments: s.Deployments,
	}
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
