package store

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/nlewo/comin/internal/deployment"
	"github.com/sirupsen/logrus"
)

type Data struct {
	Version string `json:"version"`
	// Deployments are order from the most recent to older
	Deployments []deployment.Deployment `json:"deployments"`
}

type Store struct {
	Data
	filename        string
	capacityMain    int
	capacityTesting int
}

func New(filename string, capacityMain, capacityTesting int) Store {
	s := Store{
		filename:        filename,
		capacityMain:    capacityMain,
		capacityTesting: capacityTesting,
	}
	s.Deployments = make([]deployment.Deployment, 0)
	s.Version = "1"
	return s

}

func (s *Store) DeploymentInsertAndCommit(dpl deployment.Deployment) {
	ok, evicted := s.DeploymentInsert(dpl)
	if ok {
		logrus.Infof("The deployment %s has been removed from store.json file", evicted.UUID)
	}
	if err := s.Commit(); err != nil {
		logrus.Errorf("Error while commiting the store.json file: %s", err)
	}
	logrus.Infof("The new deployment %s has been commited to store.json file", dpl.UUID)
}

// DeploymentInsert inserts a deployment and return an evicted
// deployment because the capacity has been reached.
func (s *Store) DeploymentInsert(dpl deployment.Deployment) (getsEvicted bool, evicted deployment.Deployment) {
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
	s.Deployments = append([]deployment.Deployment{dpl}, s.Deployments...)
	return
}

func (s *Store) DeploymentList() []deployment.Deployment {
	return s.Deployments
}

func (s *Store) LastDeployment() (ok bool, d deployment.Deployment) {
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
