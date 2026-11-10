package store

import (
	"slices"
	"testing"
	"time"

	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func uuids(dpls []*protobuf.Deployment) (uuids []string) {
	for _, d := range dpls {
		uuids = append(uuids, d.Uuid)
	}
	return
}

// retentionUuids collects all unique UUIDs from the retention results (current, booted, bootEntries, successful, any)
// and returns them as a sorted slice for easy comparison in tests.
func retentionUuids(current, booted *protobuf.Deployment, bootEntries, successful, any []*protobuf.Deployment) []string {
	seen := make(map[string]struct{})
	var uuids []string

	add := func(uuid string) {
		if uuid != "" {
			if _, exists := seen[uuid]; !exists {
				seen[uuid] = struct{}{}
				uuids = append(uuids, uuid)
			}
		}
	}

	if current != nil {
		add(current.Uuid)
	}
	if booted != nil {
		add(booted.Uuid)
	}
	for _, d := range bootEntries {
		add(d.Uuid)
	}
	for _, d := range successful {
		add(d.Uuid)
	}
	for _, d := range any {
		add(d.Uuid)
	}

	slices.Sort(uuids)
	return uuids
}

// TestDeploymentRetentionMinimal tests the retention policy with minimal capacity settings.
// It verifies that when capacities are set to 1, only the most recent deployments are retained.
// With capacity 1 for bootEntry, successful, and any, it ensures that only the current booted
// deployment and the most recent successful deployment are kept.
func TestDeploymentRetentionMinimal(t *testing.T) {
	secs := func(s int) *timestamppb.Timestamp {
		return timestamppb.New(time.Date(1970, time.January, 01, 0, 0, s, 0, time.UTC))
	}
	gWithSt := func(st string) *protobuf.Generation {
		return &protobuf.Generation{OutPath: st}
	}
	dpls := []*protobuf.Deployment{
		{Uuid: "5", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(5)},
		{Uuid: "4", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(4)},
		{Uuid: "3", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(3)},
		{Uuid: "2", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(2)},
		{Uuid: "1", Operation: "switch", Status: "done", Generation: gWithSt("st2"), CreatedAt: secs(1)},
	}

	_, current, booted, bootEntries, successful, any := retention(dpls, nil, "st1", "st1", 1, 1, 1, 1)
	assert.Equal(t, current.Uuid, "5")
	assert.Equal(t, booted.Uuid, "4")
	assert.Equal(t, uuids(bootEntries), []string{"4"})
	assert.Equal(t, uuids(successful), []string{"5"})
	assert.Equal(t, uuids(any), []string{"5"})

	_, current, booted, bootEntries, successful, any = retention(dpls, nil, "st1", "st1", 1, 2, 2, 2)
	assert.Equal(t, current.Uuid, "5")
	assert.Equal(t, booted.Uuid, "4")
	assert.Equal(t, uuids(bootEntries), []string{"4"})
	assert.Equal(t, uuids(successful), []string{"5", "1"})
	assert.Equal(t, uuids(any), []string{"5", "1"})

	_, current, booted, bootEntries, successful, any = retention(dpls, nil, "st1", "st1", 1, 3, 3, 3)
	assert.Equal(t, current.Uuid, "5")
	assert.Equal(t, booted.Uuid, "4")
	assert.Equal(t, uuids(bootEntries), []string{"4"})
	assert.Equal(t, uuids(successful), []string{"5", "1"})
	assert.Equal(t, uuids(any), []string{"5", "1"})
}

// TestDeploymentRetentionInitialization tests the retention policy when starting with an empty deployment list.
// It verifies that dummy deployments are created for the booted and current storepaths when they don't exist.
// This test ensures the retention policy handles initialization correctly and maintains stability across
// multiple executions with the same parameters.
func TestDeploymentRetentionInitialization(t *testing.T) {
	// Start with empty deployments - should create dummy entries for booted (st1) and current (st2)
	newDpls, current, booted, bootEntries, successful, any := retention([]*protobuf.Deployment{}, nil, "st1", "st2", 3, 3, 5, 5)
	assert.Equal(t, 2, len(bootEntries))
	assert.Equal(t, 2, len(successful))
	assert.Equal(t, 2, len(any))
	assert.NotNil(t, current)
	assert.NotNil(t, booted)
	assert.Equal(t, "st1", booted.Generation.OutPath)
	assert.Equal(t, "st2", current.Generation.OutPath)
	assert.Equal(t, 2, len(newDpls))

	// Ensure this is stable across execution - running again with same params should give same results
	newDpls, current, booted, bootEntries, successful, any = retention(newDpls, nil, "st1", "st2", 3, 3, 5, 5)
	assert.Equal(t, 2, len(bootEntries))
	assert.Equal(t, 2, len(successful))
	assert.Equal(t, 2, len(any))
	assert.NotNil(t, current)
	assert.NotNil(t, booted)
	assert.Equal(t, "st1", booted.Generation.OutPath)
	assert.Equal(t, "st2", current.Generation.OutPath)
	assert.Equal(t, 2, len(newDpls))

	// Add a new test deployment and verify it's included in the results
	test := &protobuf.Deployment{
		Uuid:      "1",
		Operation: "test",
		Status:    "done",
		Generation: &protobuf.Generation{
			OutPath: "st3",
		},
	}
	newDpls, current, booted, bootEntries, successful, any = retention(newDpls, test, "st1", "st3", 3, 3, 5, 5)
	assert.Equal(t, 2, len(bootEntries))
	assert.Equal(t, 3, len(successful))
	assert.Equal(t, 3, len(any))
	assert.NotNil(t, current)
	assert.NotNil(t, booted)
	assert.Equal(t, "st1", booted.Generation.OutPath)
	assert.Equal(t, "st3", current.Generation.OutPath)
	assert.Equal(t, 3, len(newDpls))

}

// TestDeploymentRetention tests various retention policy scenarios with different capacity settings.
// It verifies that the retention function correctly:
// - Keeps the current booted and switched deployments
// - Respects capacity limits for boot entries, successful deployments, and all deployments
// - Handles new deployments being added to the list
// - Maintains the correct order (newest first)
func TestDeploymentRetention(t *testing.T) {
	secs := func(s int) *timestamppb.Timestamp {
		return timestamppb.New(time.Date(1970, time.January, 01, 0, 0, s, 0, time.UTC))
	}
	gWithSt := func(st string) *protobuf.Generation {
		return &protobuf.Generation{OutPath: st}
	}

	// Test with capacity 2,2,2 - should keep current booted, current switched, and 2 of each category
	new := &protobuf.Deployment{Uuid: "6", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(6)}
	dpls := []*protobuf.Deployment{
		{Uuid: "5", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(5)},
		{Uuid: "4", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(4)},
		{Uuid: "3", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(3)},
		{Uuid: "2", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(2)},
		{Uuid: "1", Operation: "switch", Status: "done", Generation: gWithSt("st2"), CreatedAt: secs(1)},
	}

	// Both booted and switched are st1, so should keep: current (1), booted (4), and 2 successful (5,6)
	_, current, booted, bootEntries, successful, any := retention(dpls, new, "st1", "st1", 2, 2, 2, 2)
	assert.Equal(t, current.Uuid, "6")
	assert.Equal(t, booted.Uuid, "4")
	assert.Equal(t, uuids(bootEntries), []string{"4", "1"})
	assert.Equal(t, uuids(successful), []string{"6", "1"})
	assert.Equal(t, uuids(any), []string{"6", "1"})

	// Both booted and switched are st2, so should keep: current (1), booted (1), and 2 successful (5,6)
	_, current, booted, bootEntries, successful, any = retention(dpls, new, "st2", "st2", 2, 2, 2, 2)
	assert.Equal(t, current.Uuid, "1")
	assert.Equal(t, booted.Uuid, "1")
	assert.Equal(t, uuids(bootEntries), []string{"4", "1"})
	assert.Equal(t, uuids(successful), []string{"6", "1"})
	assert.Equal(t, uuids(any), []string{"6", "1"})

	// With capacity 1,1,1 - should keep only the essentials: current (1), booted (4), and 1 successful (6)
	_, current, booted, bootEntries, successful, any = retention(dpls, new, "st1", "st2", 1, 1, 1, 1)
	assert.Equal(t, current.Uuid, "1")
	assert.Equal(t, booted.Uuid, "4")
	assert.Equal(t, uuids(bootEntries), []string{"4"})
	assert.Equal(t, uuids(successful), []string{"6"})
	assert.Equal(t, uuids(any), []string{"6"})

	// Test with larger dataset and higher capacities
	// With capacity 2,7,7 - should keep current (1), booted (4), 2 boot entries, and 7 successful
	new = &protobuf.Deployment{Uuid: "11", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(11)}
	dpls = []*protobuf.Deployment{
		{Uuid: "10", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(10)},
		{Uuid: "9", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(9)},
		{Uuid: "8", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(8)},
		{Uuid: "7", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(7)},
		{Uuid: "6", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(6)},
		{Uuid: "5", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(5)},
		{Uuid: "4", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(4)},
		{Uuid: "3", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(3)},
		{Uuid: "2", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(2)},
		{Uuid: "1", Operation: "switch", Status: "done", Generation: gWithSt("st2"), CreatedAt: secs(1)},
	}
	_, current, booted, bootEntries, successful, any = retention(dpls, new, "st1", "st1", 2, 7, 7, 7)
	assert.Equal(t, current.Uuid, "11")
	assert.Equal(t, booted.Uuid, "4")
	assert.Equal(t, uuids(bootEntries), []string{"4", "1"})
	assert.Equal(t, uuids(successful), []string{"11", "1"})
	assert.Equal(t, uuids(any), []string{"11", "1"})
}

func TestDeploymentNewAlwaysPreserved(t *testing.T) {
	secs := func(s int) *timestamppb.Timestamp {
		return timestamppb.New(time.Date(1970, time.January, 01, 0, 0, s, 0, time.UTC))
	}
	gWithSt := func(st string) *protobuf.Generation {
		return &protobuf.Generation{OutPath: st}
	}

	// Test with capacity 2,2,2 - should keep current booted, current switched, and 2 of each category
	new := &protobuf.Deployment{Uuid: "6", Operation: "test", Status: "init", Generation: gWithSt("st3"), CreatedAt: secs(6)}
	dpls := []*protobuf.Deployment{
		{Uuid: "5", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(5)},
		{Uuid: "4", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(4)},
		{Uuid: "3", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(3)},
		{Uuid: "2", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(2)},
		{Uuid: "1", Operation: "switch", Status: "done", Generation: gWithSt("st2"), CreatedAt: secs(1)},
	}

	// Both booted and switched are st1, so should keep: current (1), booted (4), and 2 successful (5,6)
	_, current, booted, bootEntries, successful, any := retention(dpls, new, "st1", "st2", 1, 1, 1, 1)
	assert.Equal(t, current.Uuid, "1")
	assert.Equal(t, booted.Uuid, "4")
	assert.Equal(t, uuids(bootEntries), []string{"4"})
	assert.Equal(t, uuids(successful), []string{"5"})
	assert.Equal(t, uuids(any), []string{"6"})
}

func TestDeploymentCapacity(t *testing.T) {
	secs := func(s int) *timestamppb.Timestamp {
		return timestamppb.New(time.Date(1970, time.January, 01, 0, 0, s, 0, time.UTC))
	}
	gWithSt := func(st string) *protobuf.Generation {
		return &protobuf.Generation{OutPath: st}
	}

	// Test with capacity 2,2,2 - should keep current booted, current switched, and 2 of each category
	new := &protobuf.Deployment{Uuid: "6", Operation: "test", Status: "init", Generation: gWithSt("st3"), CreatedAt: secs(6)}
	dpls := []*protobuf.Deployment{
		{Uuid: "5", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(5)},
		{Uuid: "4", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(4)},
		{Uuid: "3", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(3)},
		{Uuid: "2", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(2)},
		{Uuid: "1", Operation: "switch", Status: "done", Generation: gWithSt("st2"), CreatedAt: secs(1)},
	}

	// Both booted and switched are st1, so should keep: current (1), booted (4), and 2 successful (5,6)
	result, current, booted, bootEntries, successful, any := retention(dpls, new, "st1", "st2", 1, 1, 1, 4)
	assert.Equal(t, current.Uuid, "1")
	assert.Equal(t, booted.Uuid, "4")
	assert.Equal(t, uuids(bootEntries), []string{"4"})
	assert.Equal(t, uuids(successful), []string{"5"})
	assert.Equal(t, uuids(any), []string{"6"})
	assert.Equal(t, uuids(result), []string{"6", "5", "4", "1"})
}
