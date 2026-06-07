package store

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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

	_, current, booted, bootEntries, successful, any := retention(dpls, nil, "st1", "st1", 1, 1, 1)
	result := retentionUuids(current, booted, bootEntries, successful, any)
	expected := []string{"4", "5"}
	slices.Sort(expected)
	assert.Equal(t, expected, result)

	_, current, booted, bootEntries, successful, any = retention(dpls, nil, "st1", "st1", 1, 2, 2)
	result = retentionUuids(current, booted, bootEntries, successful, any)
	expected = []string{"4", "5"}
	slices.Sort(expected)
	assert.Equal(t, expected, result)

	_, current, booted, bootEntries, successful, any = retention(dpls, nil, "st1", "st1", 1, 3, 3)
	result = retentionUuids(current, booted, bootEntries, successful, any)
	expected = []string{"3", "4", "5"}
	slices.Sort(expected)
	assert.Equal(t, expected, result)
}

func TestDeploymentRetentionInitialization(t *testing.T) {
	newDpls, current, booted, bootEntries, successful, any := retention([]*protobuf.Deployment{}, nil, "st1", "st2", 3, 3, 5)
	assert.Equal(t, 2, len(bootEntries))
	assert.Equal(t, 2, len(successful))
	assert.Equal(t, 2, len(any))
	assert.NotNil(t, current)
	assert.NotNil(t, booted)
	assert.Equal(t, "st1", booted.Generation.OutPath)
	assert.Equal(t, "st2", current.Generation.OutPath)
	assert.Equal(t, 2, len(newDpls))

	// Ensure this is stable across execution
	newDpls, current, booted, bootEntries, successful, any = retention(newDpls, nil, "st1", "st2", 3, 3, 5)
	assert.Equal(t, 2, len(bootEntries))
	assert.Equal(t, 2, len(successful))
	assert.Equal(t, 2, len(any))
	assert.NotNil(t, current)
	assert.NotNil(t, booted)
	assert.Equal(t, "st1", booted.Generation.OutPath)
	assert.Equal(t, "st2", current.Generation.OutPath)
	assert.Equal(t, 2, len(newDpls))

	// Ensure this is stable across execution
	test := &protobuf.Deployment{
		Uuid:      "1",
		Operation: "test",
		Status:    "done",
		Generation: &protobuf.Generation{
			OutPath: "st3",
		},
	}
	newDpls, current, booted, bootEntries, successful, any = retention(newDpls, test, "st1", "st3", 3, 3, 5)
	assert.Equal(t, 2, len(bootEntries))
	assert.Equal(t, 3, len(successful))
	fmt.Printf("%#v", successful)
	assert.Equal(t, 3, len(any))
	assert.NotNil(t, current)
	assert.NotNil(t, booted)
	assert.Equal(t, "st1", booted.Generation.OutPath)
	assert.Equal(t, "st3", current.Generation.OutPath)
	assert.Equal(t, 3, len(newDpls))

}

func TestDeploymentRetention(t *testing.T) {
	secs := func(s int) *timestamppb.Timestamp {
		return timestamppb.New(time.Date(1970, time.January, 01, 0, 0, s, 0, time.UTC))
	}
	gWithSt := func(st string) *protobuf.Generation {
		return &protobuf.Generation{OutPath: st}
	}
	new := &protobuf.Deployment{Uuid: "6", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(6)}
	dpls := []*protobuf.Deployment{
		{Uuid: "5", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(5)},
		{Uuid: "4", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(4)},
		{Uuid: "3", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(3)},
		{Uuid: "2", Operation: "boot", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(2)},
		{Uuid: "1", Operation: "switch", Status: "done", Generation: gWithSt("st2"), CreatedAt: secs(1)},
	}

	_, current, booted, bootEntries, successful, any := retention(dpls, new, "st1", "st1", 2, 2, 2)
	result := retentionUuids(current, booted, bootEntries, successful, any)
	expected := []string{"1", "4", "5", "6"}
	slices.Sort(expected)
	assert.Equal(t, expected, result)

	_, current, booted, bootEntries, successful, any = retention(dpls, new, "st2", "st2", 2, 2, 2)
	result = retentionUuids(current, booted, bootEntries, successful, any)
	expected = []string{"1", "4", "5", "6"}
	slices.Sort(expected)
	assert.Equal(t, expected, result)

	_, current, booted, bootEntries, successful, any = retention(dpls, new, "st1", "st2", 1, 1, 1)
	result = retentionUuids(current, booted, bootEntries, successful, any)
	expected = []string{"1", "4", "6"}
	slices.Sort(expected)
	assert.Equal(t, expected, result)

	new = &protobuf.Deployment{Uuid: "11", Operation: "test", Status: "done", Generation: gWithSt("st1"), CreatedAt: secs(6)}
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
	_, current, booted, bootEntries, successful, any = retention(dpls, new, "st1", "st1", 2, 7, 7)
	result = retentionUuids(current, booted, bootEntries, successful, any)
	expected = []string{"1", "4", "5", "6", "7", "8", "9", "10", "11"}
	slices.Sort(expected)

	assert.Equal(t, expected, result)
}

func TestDeploymentCommitAndLoad(t *testing.T) {
	tmp := t.TempDir()
	filename := tmp + "/state.json"
	bk := broker.New()
	bk.Start()
	s, _ := New(bk, filename, tmp+"/gcroots", 2, 2, 5)
	s.Commit()

	s1, _ := New(bk, filename, tmp+"/gcroots", 2, 2, 5)
	err := s1.Load()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(s.persisted.Deployments))

	s.NewDeployment(&protobuf.Generation{}, "", "", "", "")
	s1, _ = New(bk, filename, tmp+"/gcroots", 2, 2, 5)
	err = s1.Load()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(s.persisted.Deployments))
}

func TestLastDeployment(t *testing.T) {
	tmp := t.TempDir()
	bk := broker.New()
	bk.Start()
	s, _ := New(bk, "state.json", tmp+"/gcroots", 2, 2, 5)
	ok, _ := s.LastDeployment()
	assert.False(t, ok)
	s.NewDeployment(&protobuf.Generation{Uuid: "1"}, "", "", "", "")
	s.NewDeployment(&protobuf.Generation{Uuid: "2"}, "", "", "", "")
	ok, last := s.LastDeployment()
	assert.True(t, ok)
	assert.Equal(t, "2", last.Generation.Uuid)
}

func TestNewGeneration(t *testing.T) {
	tmp := t.TempDir()
	bk := broker.New()
	bk.Start()
	s, _ := New(bk, tmp+"/filename", tmp+"/gcroots", 2, 2, 5)
	s.NewGeneration("hostname", "repositoryPath", "repositoryDir", "systemAttr", &protobuf.RepositoryStatus{})
}

func TestCompareSwitchInhibitors(t *testing.T) {
	oldInhibitors := map[string]string{
		"inhibitor1": "old-value-1",
		"inhibitor2": "old-value-2",
		"common":     "common",
	}
	newInhibitors := map[string]string{
		"inhibitor2": "new-value-2",
		"inhibitor3": "new-value-3",
		"common":     "common",
	}

	diff := compareSwitchInhibitors(oldInhibitors, newInhibitors)
	assert.Len(t, diff, 1)
	assert.Equal(t, inhibitorChange{old: "old-value-2", new: "new-value-2"}, diff["inhibitor2"])

	diff = compareSwitchInhibitors(map[string]string{}, newInhibitors)
	assert.Len(t, diff, 0)

	// Test with empty new map
	diff = compareSwitchInhibitors(oldInhibitors, map[string]string{})
	assert.Len(t, diff, 0)

	// Test with both maps empty
	diff = compareSwitchInhibitors(map[string]string{}, map[string]string{})
	assert.Len(t, diff, 0)
}
