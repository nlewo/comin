package store

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/protobuf"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Status int64

const (
	Init Status = iota
	Running
	Done
	Failed
)

func StatusToString(status Status) string {
	switch status {
	case Init:
		return "init"
	case Running:
		return "running"
	case Done:
		return "done"
	case Failed:
		return "failed"
	}
	return ""
}

func StringToStatus(statusStr string) Status {
	switch statusStr {
	case "init":
		return Init
	case "running":
		return Running
	case "done":
		return Done
	case "failed":
		return Failed
	}
	return Init
}

func IsTesting(d *protobuf.Deployment) bool {
	return d.Operation == "test"
}

func (s *Store) NewDeployment(g *protobuf.Generation, operation string) *protobuf.Deployment {
	d := &protobuf.Deployment{
		Uuid:       uuid.New().String(),
		Generation: g,
		Operation:  operation,
		Status:     StatusToString(Init),
	}
	s.data.Deployments = append(s.data.Deployments, d)
	return d

}

func (s *Store) deploymentGet(uuid string) (g *protobuf.Deployment, err error) {
	for _, d := range s.data.Deployments {
		if d.Uuid == uuid {
			return d, nil
		}
	}
	return nil, fmt.Errorf("store: no deployment with uuid %s has been found", uuid)
}

func (s *Store) DeploymentStarted(uuid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.deploymentGet(uuid)
	if err != nil {
		return err
	}
	d.StartedAt = timestamppb.New(time.Now().UTC())
	d.Status = StatusToString(Running)
	return nil
}

func (s *Store) DeploymentFinished(uuid string, deploymentErr error, cominNeedRestart bool, profilePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.deploymentGet(uuid)
	if err != nil {
		return err
	}
	if deploymentErr != nil {
		d.ErrorMsg = deploymentErr.Error()
		d.Status = StatusToString(Failed)
	} else {
		d.Status = StatusToString(Done)
	}
	d.EndedAt = timestamppb.New(time.Now().UTC())
	d.Status = StatusToString(Done)
	d.RestartComin = wrapperspb.Bool(cominNeedRestart)
	d.ProfilePath = profilePath
	return nil
}
