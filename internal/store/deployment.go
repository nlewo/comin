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

const (
	RETENTION_REASON_HISTORY = "history"
	// The current running system
	RETENTION_REASON_CURRENT = "current"
	// The current booted system
	RETENTION_REASON_BOOTED = "booted"
	RETENTION_REASON_BOOT   = "boot"
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

func (s *Store) updateDataDeployments(bootedStorepath, currentStorepath string, new *protobuf.Deployment) {
	dr := retention(s.data.Deployments, new, bootedStorepath, currentStorepath, s.numberOfBootentries, s.numberOfDeployment)

	for _, d := range s.data.Deployments {
		if !slices.ContainsFunc(dr, func(a DeploymentRetention) bool { return a.dpl.Uuid == d.Uuid }) {
			logrus.Infof("store: removing the deployment %s because of the retention policy", d.Uuid)
		}
	}
	for _, r := range dr {
		if !slices.ContainsFunc(s.data.Deployments, func(a *protobuf.Deployment) bool { return a.Uuid == r.dpl.Uuid }) {
			logrus.Infof("store: adding the deployment %s", r.dpl.Uuid)
		}
	}

	dpls := make([]*protobuf.Deployment, 0)
	drs := make(map[string]string, 0)
	for _, e := range dr {
		dpls = append(dpls, e.dpl)
		drs[e.dpl.Uuid] = e.reason
	}
	s.data.Deployments = dpls
	s.data.RetentionReasons = drs
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

func (s *Store) DeploymentStarted(uuid string) error {
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
	s.Commit()
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
	s.updateDataDeployments(bootedStorepath, currentStorepath, nil)
	s.Commit()
	return nil
}

type DeploymentRetention struct {
	dpl    *protobuf.Deployment
	reason string
}

func isBootEntry(d *protobuf.Deployment) bool {
	return d.Operation == "boot" || d.Operation == "switch"
}

// Retention algo
// 1. We keep the last deployment
// 2. We keep the current booted system
// 3. We keed the current switched system
// 4. We keep N last boot and switch successful deployment excluding the current one
// 5. We keep M last deployment excluding the current one
func retention(dpls []*protobuf.Deployment, new *protobuf.Deployment, bootedStorepath, switchedStorepath string, numberOfBootentries, numberOfDeployment int) []DeploymentRetention {
	// dpls are sorted from newer to older
	endedAtCmp := func(a, b *protobuf.Deployment) int {
		return -a.CreatedAt.AsTime().Compare(b.CreatedAt.AsTime())
	}
	slices.SortFunc(dpls, endedAtCmp)

	res := make([]DeploymentRetention, 0)

	// 1. We keep the last deployment
	if new != nil {
		res = append(res, DeploymentRetention{dpl: new, reason: "new"})
	}

	// 2. We keep the current booted systems
	for _, d := range dpls {
		if d.Operation == "boot" || d.Operation == "switch" &&
			d.Status == "done" &&
			(d.Generation.OutPath == bootedStorepath) {
			res = append(res, DeploymentRetention{dpl: d, reason: RETENTION_REASON_BOOTED})
			break
		}
	}
	// 3. We keep the current switched systems
	for _, d := range dpls {
		if d.Operation == "switch" &&
			d.Status == "done" &&
			(d.Generation.OutPath == switchedStorepath) {
			res = append(res, DeploymentRetention{dpl: d, reason: RETENTION_REASON_CURRENT})
			break
		}
	}

	// 4. We keep N last boot and switch successful deployment having different storepaths
	storepaths := make(map[string]struct{}, 0)
	for _, d := range dpls {
		if isBootEntry(d) &&
			d.Status == "done" {
			if len(storepaths) >= numberOfBootentries {
				break
			}
			_, ok := storepaths[d.Generation.OutPath]
			if ok {
				continue
			}
			resAlreadyHasStorepath := slices.ContainsFunc(res, func(r DeploymentRetention) bool {
				return isBootEntry(r.dpl) && r.dpl.Generation.OutPath == d.Generation.OutPath
			})
			storepaths[d.Generation.OutPath] = struct{}{}
			if !resAlreadyHasStorepath {
				res = append(res, DeploymentRetention{dpl: d, reason: RETENTION_REASON_BOOT})
			}
		}
	}

	// 5. We keep M last deployment excluding the current one
	counter := 0
	for _, d := range dpls {
		if counter >= numberOfDeployment {
			break
		}
		hasUuid := slices.ContainsFunc(res, func(r DeploymentRetention) bool {
			return r.dpl.Uuid == d.Uuid
		})
		if !hasUuid {
			res = append(res, DeploymentRetention{dpl: d, reason: RETENTION_REASON_HISTORY})
		}
		counter++
	}

	slices.SortFunc(res, func(a, b DeploymentRetention) int {
		return -a.dpl.CreatedAt.AsTime().Compare(b.dpl.CreatedAt.AsTime())
	})

	return res
}
