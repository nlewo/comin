package store

import (
	"slices"
	"testing"
	"time"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func retentionUuids(current, booted string, bootEntries, successful, any []string) []string {
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

	add(current)
	add(booted)
	for _, uuid := range bootEntries {
		add(uuid)
	}
	for _, uuid := range successful {
		add(uuid)
	}
	for _, uuid := range any {
		add(uuid)
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

	current, booted, bootEntries, successful, any := retention(dpls, nil, "st1", "", 1, 1, 1)
	result := retentionUuids(current, booted, bootEntries, successful, any)
	expected := []string{"4", "5"}
	slices.Sort(expected)
	assert.Equal(t, expected, result)

	current, booted, bootEntries, successful, any = retention(dpls, nil, "st1", "", 1, 2, 2)
	result = retentionUuids(current, booted, bootEntries, successful, any)
	expected = []string{"4", "5"}
	slices.Sort(expected)
	assert.Equal(t, expected, result)

	current, booted, bootEntries, successful, any = retention(dpls, nil, "st1", "", 1, 3, 3)
	result = retentionUuids(current, booted, bootEntries, successful, any)
	expected = []string{"3", "4", "5"}
	slices.Sort(expected)
	assert.Equal(t, expected, result)
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

	current, booted, bootEntries, successful, any := retention(dpls, new, "st1", "", 2, 2, 2)
	result := retentionUuids(current, booted, bootEntries, successful, any)
	expected := []string{"1", "4", "5", "6"}
	slices.Sort(expected)
	assert.Equal(t, expected, result)

	current, booted, bootEntries, successful, any = retention(dpls, new, "st2", "", 2, 2, 2)
	result = retentionUuids(current, booted, bootEntries, successful, any)
	expected = []string{"1", "4", "5", "6"}
	slices.Sort(expected)
	assert.Equal(t, expected, result)

	current, booted, bootEntries, successful, any = retention(dpls, new, "st1", "st2", 1, 1, 1)
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
	current, booted, bootEntries, successful, any = retention(dpls, new, "st1", "", 2, 7, 7)
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
	assert.Equal(t, 0, len(s.data.Deployments))

	s.NewDeployment(&protobuf.Generation{}, "", "", "", "")
	s1, _ = New(bk, filename, tmp+"/gcroots", 2, 2, 5)
	err = s1.Load()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(s.data.Deployments))
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
