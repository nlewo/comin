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

type Store struct {
	data               *protobuf.Store
	mu                 sync.Mutex
	filename           string
	generationGcRoot   string
	bootEntryCapacity  int
	successfulCapacity int
	anyCapacity        int

	lastEvalStarted   *protobuf.Generation
	lastEvalFinished  *protobuf.Generation
	lastBuildStarted  *protobuf.Generation
	lastBuildFinished *protobuf.Generation

	broker *broker.Broker
}

func New(broker *broker.Broker, filename, gcRootsDir string, bootEntryCapacity, successfulCapacity, anyCapacity int) (*Store, error) {
	if bootEntryCapacity < 1 {
		return nil, fmt.Errorf("store: bootEntryCapacity cannot be < 1")
	}
	if successfulCapacity < 1 {
		return nil, fmt.Errorf("store: successfulCapacity cannot be < 1")
	}
	if anyCapacity < 1 {
		return nil, fmt.Errorf("store: anyCapacity cannot be < 1")
	}

	data := &protobuf.Store{
		Deployments: make([]*protobuf.Deployment, 0),
		Generations: make([]*protobuf.Generation, 0),
		Deployer:    &protobuf.DeployerState{},
	}
	st := Store{
		filename:           filename,
		generationGcRoot:   gcRootsDir + "/last-built-generation",
		bootEntryCapacity:  bootEntryCapacity,
		successfulCapacity: successfulCapacity,
		anyCapacity:        anyCapacity,
		data:               data,
		broker:             broker,
	}
	if err := os.MkdirAll(gcRootsDir, os.ModeDir); err != nil {
		return nil, err
	}
	logrus.Infof("store: init with generationGcRoot=%s bootEntryCapacity=%d successfulCapacity=%d anyCapacity=%d", st.generationGcRoot, st.bootEntryCapacity, st.successfulCapacity, st.anyCapacity)

	return &st, nil
}

func (s *Store) GetState() *protobuf.Store {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

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

func (s *Store) DeployerUpdate(state *protobuf.DeployerState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Deployer = state
	s.Commit()
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
	if s.data.Deployer == nil {
		s.data.Deployer = &protobuf.DeployerState{}
	}

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
