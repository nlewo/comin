package store

import (
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/types"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
)

func isBootEntry(d *protobuf.Deployment) bool {
	return d.Operation == "boot" || d.Operation == "switch"
}

// bootedStorepath + switchedStorepath <= successfulCapacity <= allCapacity <= totalCapacity
func retention(dpls []*protobuf.Deployment, new *protobuf.Deployment, bootedStorepath, switchedStorepath string, bootEntryOutpathCapacity, successfulOutpathCapacity, anyStorepathCapacity, capacity int) (result []*protobuf.Deployment, current, booted *protobuf.Deployment, deploymentsBootEntry, deploymentsSuccessful, deploymentsAny, gcs []*protobuf.Deployment) {
	current = nil
	booted = nil
	deploymentsBootEntry = make([]*protobuf.Deployment, 0)
	deploymentsSuccessful = make([]*protobuf.Deployment, 0)
	deploymentsAny = make([]*protobuf.Deployment, 0)
	gcs = make([]*protobuf.Deployment, 0)
	result = make([]*protobuf.Deployment, 0)

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
		if isBootEntry(d) && d.Status == "done" && d.Generation.OutPath == bootedStorepath && !d.Gc {
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
		if d.Status == "done" && d.Generation.OutPath == switchedStorepath && !d.Gc {
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
		if len(storepaths) >= bootEntryOutpathCapacity {
			break
		}
		if !isBootEntry(d) || d.Status != "done" && !d.Gc {
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

	// 4. Successful deployments (any operation) with storepath deduplication
	storepaths = make(map[string]struct{})
	for _, d := range dpls {
		if d.Status != "done" && !d.Gc {
			continue
		}
		if len(storepaths) >= successfulOutpathCapacity {
			break
		}
		if _, exists := storepaths[d.Generation.OutPath]; exists {
			continue
		}
		storepaths[d.Generation.OutPath] = struct{}{}
		if !slices.ContainsFunc(result, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
			result = append(result, d)
		}
		if !slices.ContainsFunc(deploymentsSuccessful, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
			deploymentsSuccessful = append(deploymentsSuccessful, d)
		}
	}

	// 5. All deployments (any status) with storepath deduplication
	storepaths = make(map[string]struct{})
	for _, d := range dpls {
		if d.Gc {
			continue
		}
		if len(storepaths) >= anyStorepathCapacity {
			break
		}
		if _, exists := storepaths[d.Generation.OutPath]; exists {
			continue
		}
		storepaths[d.Generation.OutPath] = struct{}{}
		if !slices.ContainsFunc(result, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
			result = append(result, d)
		}
		if !slices.ContainsFunc(deploymentsAny, func(e *protobuf.Deployment) bool { return e.Uuid == d.Uuid }) {
			deploymentsAny = append(deploymentsAny, d)
		}
	}

	// Add first deployments from result list ensuring capacity is not reached
	if capacity > 0 && len(result) < capacity {
		remaining := capacity - len(result)
		for i := 0; i < len(dpls) && remaining > 0; i++ {
			if !slices.ContainsFunc(result, func(e *protobuf.Deployment) bool { return e.Uuid == dpls[i].Uuid }) {
				result = append(result, dpls[i])
				gcs = append(gcs, dpls[i])
				remaining--
			}
		}
	}
	slices.SortFunc(result, endedAtCmp)

	return
}
