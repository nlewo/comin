package store

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/sirupsen/logrus"
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

func logDiff(name string, old, new []string) {
	for _, o := range old {
		if !slices.Contains(new, o) {
			logrus.Infof("store: removing from the list '%s' the deployment of %s ", name, o)
		}
	}
	for _, n := range new {
		if !slices.Contains(old, n) {
			logrus.Infof("store: adding to the list '%s' the deployment %s", name, n)
		}
	}
}

// updateDataDeployments is not thread safe
func (s *Store) updateDataDeployments(bootedStorepath, currentStorepath string, new *protobuf.Deployment) {
	current, booted, bootEntries, successful, any := retention(s.data.Deployments, new, bootedStorepath, currentStorepath, s.bootEntryCapacity, s.successfulCapacity, s.anyCapacity)

	if current != s.data.DeploymentSwitched {
		logrus.Infof("store: retention: the current deployment has been updated from %s to %s", s.data.DeploymentSwitched, current)
		s.data.DeploymentSwitched = current
	}
	if booted != s.data.DeploymentBooted {
		logrus.Infof("store: retention: the booted deployment has been updated from %s to %s", s.data.DeploymentSwitched, current)
		s.data.DeploymentBooted = booted
	}
	logDiff("boot entries", s.data.DeploymentsBootEntry, bootEntries)
	s.data.DeploymentsBootEntry = bootEntries

	logDiff("deployments successful", s.data.DeploymentsSuccessful, successful)
	s.data.DeploymentsSuccessful = successful

	logDiff("any deployments", s.data.DeploymentsAny, any)
	s.data.DeploymentsAny = any

	dpls := s.data.Deployments
	if new != nil {
		dpls = append(s.data.Deployments, new)
	}
	allUuids := []string{}
	for _, uuid := range slices.Concat([]string{current, booted}, s.data.DeploymentsBootEntry, s.data.DeploymentsSuccessful, s.data.DeploymentsAny) {
		if !slices.Contains(allUuids, uuid) {
			allUuids = append(allUuids, uuid)
		}
	}
	acc := make([]*protobuf.Deployment, 0)
	for _, d := range dpls {
		if slices.Contains(allUuids, d.Uuid) && !slices.ContainsFunc(acc, func(a *protobuf.Deployment) bool { return a.Uuid == d.Uuid }) {
			acc = append(acc, d)
		}
	}
	endedAtCmp := func(a, b *protobuf.Deployment) int {
		return -a.CreatedAt.AsTime().Compare(b.CreatedAt.AsTime())
	}
	slices.SortFunc(acc, endedAtCmp)

	s.data.Deployments = acc
	s.Commit()
}

func (s *Store) NewDeployment(g *protobuf.Generation, operation, reason, bootedStorepath, currentStorepath string) *protobuf.Deployment {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := &protobuf.Deployment{
		Uuid:       uuid.New().String(),
		Generation: g,
		Operation:  operation,
		Reason:     reason,
		Status:     StatusToString(Init),
		CreatedAt:  timestamppb.New(time.Now().UTC()),
	}
	s.updateDataDeployments(bootedStorepath, currentStorepath, d)
	s.Commit()
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

func (s *Store) GetDeploymentLastest() (latest *protobuf.Deployment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.data.Deployments {
		if latest == nil || d.EndedAt != nil && d.CreatedAt.AsTime().After(latest.CreatedAt.AsTime()) {
			latest = d
		}
	}
	return
}

func (s *Store) GetDeploymentProfilePaths() []string {
	m := make(map[string]struct{})
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.data.Deployments {
		if d.ProfilePath != "" {
			m[d.ProfilePath] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(m))
}

func (s *Store) DeploymentStarted(uuid, bootedStorepath, currentStorepath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.deploymentGet(uuid)
	if err != nil {
		return err
	}
	d.StartedAt = timestamppb.New(time.Now().UTC())
	d.Status = StatusToString(Running)
	e := &protobuf.Event_DeploymentStarted{Deployment: d}
	s.broker.Publish(&protobuf.Event{Type: &protobuf.Event_DeploymentStartedType{DeploymentStartedType: e}})
	s.updateDataDeployments(bootedStorepath, currentStorepath, d)
	return nil
}

func (s *Store) DeploymentFinished(uuid string, deploymentErr error, cominNeedRestart bool, profilePath string, bootedStorepath, currentStorepath string) error {
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
	e := &protobuf.Event_DeploymentFinished{Deployment: d}
	s.broker.Publish(&protobuf.Event{Type: &protobuf.Event_DeploymentFinishedType{DeploymentFinishedType: e}})
	s.updateDataDeployments(bootedStorepath, currentStorepath, d)
	return nil
}

func isBootEntry(d *protobuf.Deployment) bool {
	return d.Operation == "boot" || d.Operation == "switch"
}

func retention(dpls []*protobuf.Deployment, new *protobuf.Deployment, bootedStorepath, switchedStorepath string, bootEntryCapacity, successfulCapacity, allCapacity int) (string, string, []string, []string, []string) {
	var current, booted string
	deploymentsBootEntry := make([]string, 0)
	deploymentsSuccessful := make([]string, 0)
	deploymentsAny := make([]string, 0)

	if new != nil {
		dpls = append(dpls, new)
	}

	// Sort deployments from newer to older
	endedAtCmp := func(a, b *protobuf.Deployment) int {
		return -a.CreatedAt.AsTime().Compare(b.CreatedAt.AsTime())
	}
	slices.SortFunc(dpls, endedAtCmp)

	// 1. Always keep the current booted system
	for _, d := range dpls {
		if isBootEntry(d) && d.Status == "done" && d.Generation.OutPath == bootedStorepath {
			booted = d.Uuid
			break
		}
	}

	// 2. Always keep the current switched system
	for _, d := range dpls {
		if d.Operation == "switch" && d.Status == "done" && d.Generation.OutPath == switchedStorepath {
			current = d.Uuid
			break
		}
	}

	// 3. Boot entries with storepath deduplication
	storepaths := make(map[string]struct{})
	for _, d := range dpls {
		if len(storepaths) >= bootEntryCapacity {
			break
		}
		if !isBootEntry(d) || d.Status != "done" {
			continue
		}
		if _, exists := storepaths[d.Generation.OutPath]; exists {
			continue
		}
		storepaths[d.Generation.OutPath] = struct{}{}
		deploymentsBootEntry = append(deploymentsBootEntry, d.Uuid)
	}

	// 4. Successful deployments (any operation)
	counter := 0
	for _, d := range dpls {
		if d.Status != "done" {
			continue
		}
		if counter >= successfulCapacity {
			break
		}
		counter++
		deploymentsSuccessful = append(deploymentsSuccessful, d.Uuid)
	}

	// 5. All deployments (any status)
	counter = 0
	for _, d := range dpls {
		if counter >= allCapacity {
			break
		}
		counter++
		deploymentsAny = append(deploymentsAny, d.Uuid)
	}

	return current, booted, deploymentsBootEntry, deploymentsSuccessful, deploymentsAny
}
