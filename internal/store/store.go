package store

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
)

type Data struct {
	Version string `json:"version"`
	// Deployments are order from the most recent to older
	Deployments      []*protobuf.Deployment `json:"deployments"`
	Generations      []*protobuf.Generation `json:"generations"`
	RetentionReasons map[string]string      `json:"retention_reasons"`
}

type Store struct {
	data                *protobuf.Store
	mu                  sync.Mutex
	filename            string
	generationGcRoot    string
	numberOfBootentries int
	numberOfDeployment  int

	lastEvalStarted   *protobuf.Generation
	lastEvalFinished  *protobuf.Generation
	lastBuildStarted  *protobuf.Generation
	lastBuildFinished *protobuf.Generation

	broker *broker.Broker
}

func New(broker *broker.Broker, filename, gcRootsDir string, numberOfBootentries, numberOfDeployment int) (*Store, error) {
	if numberOfBootentries < 1 {
		return nil, fmt.Errorf("store: numberOfBootentries cannot be < 1")
	}
	if numberOfDeployment < 1 {
		return nil, fmt.Errorf("store: numberOfDeployment cannot be < 1")
	}

	data := &protobuf.Store{
		Deployments: make([]*protobuf.Deployment, 0),
		Generations: make([]*protobuf.Generation, 0),
	}
	st := Store{
		filename:            filename,
		generationGcRoot:    gcRootsDir + "/last-built-generation",
		numberOfBootentries: numberOfBootentries,
		numberOfDeployment:  numberOfDeployment,
		data:                data,
		broker:              broker,
	}
	if err := os.MkdirAll(gcRootsDir, os.ModeDir); err != nil {
		return nil, err
	}
	logrus.Infof("store: init with generationGcRoot=%s numberOfBootentries=%d numberOfDeployment=%d", st.generationGcRoot, st.numberOfBootentries, st.numberOfDeployment)

	return &st, nil
}

func (s *Store) GetState() *protobuf.Store {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

// DeploymentAdd inserts a deployment and return an evicted
// deployment because the capacity has been reached.
func (s *Store) DeploymentList() []*protobuf.Deployment {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Deployments
}

func (s *Store) LastDeployment() (ok bool, d *protobuf.Deployment) {
	if len(s.DeploymentList()) > 1 {
		return true, s.DeploymentList()[0]
	}
	return
}

func (s *Store) Load() (err error) {
	var data protobuf.Store
	content, err := os.ReadFile(s.filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return
	}
	unmarshaler := protojson.UnmarshalOptions{}
	err = unmarshaler.Unmarshal(content, &data)
	if err != nil {
		return
	}
	s.data = &data
	logrus.Infof("store: loaded %d deployments from %s", len(s.data.Deployments), s.filename)

	booted, current := utils.GetBootedAndCurrentStorepaths()
	s.updateDataDeployments(booted, current, nil)

	return
}

func (s *Store) Commit() {
	marshaler := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
		AllowPartial:    true,
	}
	buf, err := marshaler.Marshal(s.data)
	if err != nil {
		logrus.Errorf("store: cannot marshal store.data: %s", err)
		return
	}
	err = os.WriteFile(s.filename, buf, 0644)
	if err != nil {
		logrus.Errorf("store: cannot write store.data to %s: %s", s.filename, err)
	}
}
