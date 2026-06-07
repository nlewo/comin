package store

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/types"
	"github.com/nlewo/comin/internal/utils"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
)

type Store struct {
	persisted          *protobuf.Store
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
		Deployments:                  make([]*protobuf.Deployment, 0),
		Generations:                  make([]*protobuf.Generation, 0),
		Deployer:                     &protobuf.DeployerState{},
		DeploymentBootEntryCapacity:  int32(bootEntryCapacity),
		DeploymentSuccessfulCapacity: int32(successfulCapacity),
		DeploymentAnyCapacity:        int32(anyCapacity),
	}
	st := Store{
		filename:           filename,
		generationGcRoot:   gcRootsDir + "/last-built-generation",
		bootEntryCapacity:  bootEntryCapacity,
		successfulCapacity: successfulCapacity,
		anyCapacity:        anyCapacity,
		persisted:          data,
		broker:             broker,
	}
	if err := os.MkdirAll(gcRootsDir, os.ModeDir); err != nil {
		return nil, err
	}
	logrus.Infof("store: init with generationGcRoot=%s deploymentBootEntryCapacity=%d deploymentSuccessfulCapacity=%d deploymentAnyCapacity=%d", st.generationGcRoot, bootEntryCapacity, successfulCapacity, anyCapacity)

	return &st, nil
}

func (s *Store) GetState() *protobuf.Store {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persisted
}

func (s *Store) DeploymentList() []*protobuf.Deployment {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persisted.Deployments
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
	s.persisted.Deployer = state
	s.Commit()
}

func (s *Store) Load() (err error) {
	var data protobuf.Store
	content, err := os.ReadFile(s.filename)
	if errors.Is(err, os.ErrNotExist) {
		logrus.Infof("store: the store file %s doesn't exist and will be initialized", s.filename)
		err = nil
	} else if err != nil {
		return
	} else {
		unmarshaler := protojson.UnmarshalOptions{}
		err = unmarshaler.Unmarshal(content, &data)
		if err != nil {
			return
		}
	}
	if s.persisted != nil {
		s.compareAndLogCapacities(&data, s.persisted)
	}
	// We update stored capacities with the ones provided by the comin configuration
	data.DeploymentAnyCapacity = s.persisted.DeploymentAnyCapacity
	data.DeploymentBootEntryCapacity = s.persisted.DeploymentBootEntryCapacity
	data.DeploymentSuccessfulCapacity = s.persisted.DeploymentSuccessfulCapacity
	s.persisted = &data

	logrus.Infof("store: loaded %d deployments from %s", len(s.persisted.Deployments), s.filename)
	if s.persisted.Deployer == nil {
		s.persisted.Deployer = &protobuf.DeployerState{}
	}

	booted, current := utils.GetBootedAndCurrentStorepaths()

	// If the booted deployment has not been found in the deployment persisted list, we create a dummy one. This could happen when starting from an empty store.json file.
	if s.GetDeploymentByOutpath(booted) == nil {
		d := &protobuf.Deployment{
			Uuid: uuid.New().String(),
			Generation: &protobuf.Generation{
				OutPath: booted,
			},
			Operation: types.OperationBoot,
			Reason:    "Dummy deployment because not found in the deployment list",
			Status:    StatusToString(Done),
		}
		logrus.Infof("store: dummy deployment %s created for the currently booted storepath %s", d.Uuid, d.Generation.OutPath)
		s.persisted.Deployments = append(s.persisted.Deployments, d)
	}

	// If the booted deployment has not been found in the deployment persisted list, we create a dummy one. This could happen when starting from an empty store.json file.
	if s.GetDeploymentByOutpath(current) == nil {
		d := &protobuf.Deployment{
			Uuid: uuid.New().String(),
			Generation: &protobuf.Generation{
				OutPath: current,
			},
			Operation: types.OperationSwitch,
			Reason:    "Dummy deployment because not found in the deployment list",
			Status:    StatusToString(Done),
		}
		logrus.Infof("store: dummy deployment %s created for the currently switched storepath %s", d.Uuid, d.Generation.OutPath)
		s.persisted.Deployments = append(s.persisted.Deployments, d)
	}

	s.updateDataDeployments(booted, current, nil)

	return
}

func (s *Store) Commit() {
	marshaler := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
		AllowPartial:    true,
	}
	buf, err := marshaler.Marshal(s.persisted)
	if err != nil {
		logrus.Errorf("store: cannot marshal store.data: %s", err)
		return
	}
	err = os.WriteFile(s.filename, buf, 0644)
	if err != nil {
		logrus.Errorf("store: cannot write store.data to %s: %s", s.filename, err)
	}
}

func (s *Store) compareAndLogCapacities(old, new *protobuf.Store) {
	if old.DeploymentBootEntryCapacity != new.DeploymentBootEntryCapacity {
		logrus.Infof("store: deploymentBootEntryCapacity changed from %d to %d", old.DeploymentBootEntryCapacity, new.DeploymentBootEntryCapacity)
	}
	if old.DeploymentSuccessfulCapacity != new.DeploymentSuccessfulCapacity {
		logrus.Infof("store: deploymentSuccessfulCapacity changed from %d to %d", old.DeploymentSuccessfulCapacity, new.DeploymentSuccessfulCapacity)
	}
	if old.DeploymentAnyCapacity != new.DeploymentAnyCapacity {
		logrus.Infof("store: deploymentAnyCapacity changed from %d to %d", old.DeploymentAnyCapacity, new.DeploymentAnyCapacity)
	}
}
