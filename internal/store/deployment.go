package store

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/types"
	"github.com/nlewo/comin/pkg/protobuf"
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
	dpls, current, booted, bootEntries, successful, any := retention(s.persisted.Deployments, new, bootedStorepath, currentStorepath, int(s.persisted.DeploymentBootEntryCapacity), int(s.persisted.DeploymentSuccessfulCapacity), int(s.persisted.DeploymentAnyCapacity))

	s.persisted.Deployments = dpls
	if current.Uuid != s.persisted.DeploymentSwitched {
		logrus.Infof("store: retention: the current deployment has been updated from %s to %s", s.persisted.DeploymentSwitched, current.Uuid)
		s.persisted.DeploymentSwitched = current.Uuid
	}
	if booted.Uuid != s.persisted.DeploymentBooted {
		logrus.Infof("store: retention: the booted deployment has been updated from %s to %s", s.persisted.DeploymentBooted, booted.Uuid)
		s.persisted.DeploymentBooted = booted.Uuid
	}

	// Convert deployment slices to UUID slices for the persisted state
	bootEntriesUuids := make([]string, len(bootEntries))
	for i, d := range bootEntries {
		bootEntriesUuids[i] = d.Uuid
	}
	logDiff("boot entries", s.persisted.DeploymentsBootEntry, bootEntriesUuids)
	s.persisted.DeploymentsBootEntry = bootEntriesUuids

	successfulUuids := make([]string, len(successful))
	for i, d := range successful {
		successfulUuids[i] = d.Uuid
	}
	logDiff("deployments successful", s.persisted.DeploymentsSuccessful, successfulUuids)
	s.persisted.DeploymentsSuccessful = successfulUuids

	anyUuids := make([]string, len(any))
	for i, d := range any {
		anyUuids[i] = d.Uuid
	}
	logDiff("any deployments", s.persisted.DeploymentsAny, anyUuids)
	s.persisted.DeploymentsAny = anyUuids

	s.Commit()
}

func loadInhibitors(filepath string) map[string]string {
	if _, err := os.Stat(filepath); err != nil {
		return nil
	}
	file, err := os.Open(filepath)
	if err != nil {
		return nil
	}
	defer file.Close() // nolint: errcheck
	var inhibitors map[string]string
	if err := json.NewDecoder(file).Decode(&inhibitors); err != nil {
		return nil
	}
	return inhibitors
}

func (s *Store) NewDeployment(g *protobuf.Generation, operationSubmitted, reason, bootedStorepath, currentStorepath string) *protobuf.Deployment {
	currentInhibitors := loadInhibitors(path.Join(currentStorepath, "switch-inhibitors"))
	newInhibitors := loadInhibitors(path.Join(g.OutPath, "switch-inhibitors"))
	operation, operationreason := computeOperation(operationSubmitted, currentInhibitors, newInhibitors)

	s.mu.Lock()
	defer s.mu.Unlock()
	d := &protobuf.Deployment{
		Uuid:               uuid.New().String(),
		Generation:         g,
		OperationSubmitted: operationSubmitted,
		Operation:          operation,
		OperationReason:    operationreason,
		Reason:             reason,
		Status:             StatusToString(Init),
		CreatedAt:          timestamppb.New(time.Now().UTC()),
		CurrentInhibitors:  currentInhibitors,
		NewInhibitors:      newInhibitors,
	}
	s.updateDataDeployments(bootedStorepath, currentStorepath, d)
	s.Commit()
	return d

}

func (s *Store) deploymentGet(uuid string) (g *protobuf.Deployment, err error) {
	for _, d := range s.persisted.Deployments {
		if d.Uuid == uuid {
			return d, nil
		}
	}
	return nil, fmt.Errorf("store: no deployment with uuid %s has been found", uuid)
}

func (s *Store) GetDeploymentByOutpath(outpath string) (d *protobuf.Deployment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.persisted.Deployments {
		if d.Generation.OutPath == outpath {
			return d
		}
	}
	return
}

func (s *Store) GetDeployment(uuid string) (g *protobuf.Deployment, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deploymentGet(uuid)
}

func (s *Store) GetDeploymentLastest() (latest *protobuf.Deployment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.persisted.Deployments {
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
	for _, d := range s.persisted.Deployments {
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
	s.broker.Publish(&protobuf.Event{Type: &protobuf.Event_DeploymentStartedType{DeploymentStartedType: e}, CreatedAt: timestamppb.New(time.Now().UTC())})
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
	d.RestartComin = wrapperspb.Bool(cominNeedRestart)
	d.ProfilePath = profilePath
	e := &protobuf.Event_DeploymentFinished{Deployment: d}
	s.broker.Publish(&protobuf.Event{Type: &protobuf.Event_DeploymentFinishedType{DeploymentFinishedType: e}, CreatedAt: timestamppb.New(time.Now().UTC())})
	s.updateDataDeployments(bootedStorepath, currentStorepath, d)
	return nil
}

func isBootEntry(d *protobuf.Deployment) bool {
	return d.Operation == "boot" || d.Operation == "switch"
}

func retention(dpls []*protobuf.Deployment, new *protobuf.Deployment, bootedStorepath, switchedStorepath string, bootEntryCapacity, successfulCapacity, allCapacity int) ([]*protobuf.Deployment, *protobuf.Deployment, *protobuf.Deployment, []*protobuf.Deployment, []*protobuf.Deployment, []*protobuf.Deployment) {
	var current, booted *protobuf.Deployment
	deploymentsBootEntry := make([]*protobuf.Deployment, 0)
	deploymentsSuccessful := make([]*protobuf.Deployment, 0)
	deploymentsAny := make([]*protobuf.Deployment, 0)
	result := make([]*protobuf.Deployment, 0)

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
			booted = d
			if !slices.ContainsFunc(result, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
				result = append(result, d)
			}
			break
		}
	}

	// If no deployment has been found for the boot storepath, we create a dummy one. This could happen when the store.jon file has been deleted or at first comin run.
	if booted == nil {
		d := &protobuf.Deployment{
			Uuid: uuid.New().String(),
			Generation: &protobuf.Generation{
				OutPath: bootedStorepath,
			},
			Operation: types.OperationBoot,
			Reason:    fmt.Sprintf("Empty deployment because the booted storepath %s has not be found in the deployment list", bootedStorepath),
			Status:    StatusToString(Done),
		}
		logrus.Infof("store: dummy deployment %s created for the currently booted storepath %s", d.Uuid, d.Generation.OutPath)
		booted = d
		dpls = append(dpls, d)
		result = append(result, d)
		slices.SortFunc(dpls, endedAtCmp)
	}

	// 2. Always keep the current switched system
	for _, d := range dpls {
		if d.Status == "done" && d.Generation.OutPath == switchedStorepath {
			current = d
			if !slices.ContainsFunc(result, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
				result = append(result, d)
			}
			break
		}
	}

	// If no deployment has been found for the switched storepath, we create a dummy one. This could happen when the store.jon file has been deleted or at first comin run.
	if current == nil {
		d := &protobuf.Deployment{
			Uuid: uuid.New().String(),
			Generation: &protobuf.Generation{
				OutPath: switchedStorepath,
			},
			// We don't know if the currently switched storepath is comin from a test operation or switch operation.
			// So, we consider it comes from a switch opeartion in order to preserve it in the bootloader.
			Operation: types.OperationSwitch,
			Reason:    fmt.Sprintf("Empty deployment because the switched storepath %s has not be found in the deployment list", switchedStorepath),
			Status:    StatusToString(Done),
		}
		logrus.Infof("store: dummy deployment %s created for the currently switched storepath %s", d.Uuid, d.Generation.OutPath)
		current = d
		result = append(result, d)
		dpls = append(dpls, d)
		slices.SortFunc(dpls, endedAtCmp)
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
		if !slices.ContainsFunc(result, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
			result = append(result, d)
		}
		if !slices.ContainsFunc(deploymentsBootEntry, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
			deploymentsBootEntry = append(deploymentsBootEntry, d)
		}
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
		if !slices.ContainsFunc(result, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
			result = append(result, d)
		}
		if !slices.ContainsFunc(deploymentsSuccessful, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
			deploymentsSuccessful = append(deploymentsSuccessful, d)
		}
	}

	// 5. All deployments (any status)
	counter = 0
	for _, d := range dpls {
		if counter >= allCapacity {
			break
		}
		counter++
		if !slices.ContainsFunc(result, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
			result = append(result, d)
		}
		if !slices.ContainsFunc(deploymentsAny, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
			deploymentsAny = append(deploymentsAny, d)
		}
	}

	slices.SortFunc(result, endedAtCmp)

	return result, current, booted, deploymentsBootEntry, deploymentsSuccessful, deploymentsAny
}

type inhibitorChange struct {
	old string
	new string
}

// compareSwitchInhibitors is a Go implementation of https://github.com/nixos/nixpkgs/blob/ddce4d809a16b9f0614e76636063282f6fb02908/nixos/modules/system/activation/switchable-system.nix#L101
func compareSwitchInhibitors(current map[string]string, new map[string]string) (diff map[string]inhibitorChange) {
	diff = make(map[string]inhibitorChange)
	for k, v := range current {
		if _, ok := new[k]; ok {
			diff[k] = inhibitorChange{old: v}
		}

	}
	for k, v := range new {
		if existing, ok := diff[k]; ok {
			if existing.old != v {
				existing.new = v
				diff[k] = existing
			} else {
				delete(diff, k)
			}
		}
	}

	return
}

func computeOperation(operation string, currentInhibitors map[string]string, newInhibitors map[string]string) (computedOperation string, reason string) {
	diff := compareSwitchInhibitors(currentInhibitors, newInhibitors)
	switch operation {
	case types.OperationTest:
		if len(diff) != 0 {
			return types.OperationNull, "The test operation is not possible because of different switch inhibitors"
		}
	case types.OperationSwitch:
		if len(diff) != 0 {
			return types.OperationBoot, "Using the boot operation instead of switch operation because of diffrent switch inhibitors"
		}
	}
	return operation, ""
}
