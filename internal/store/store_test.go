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

func deploymentUuids(rdpls []DeploymentRetention) (uuids []string) {
	for _, rdpl := range rdpls {
		uuids = append(uuids, rdpl.dpl.Uuid)
	}
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

	res := retention(dpls, nil, "st1", "", 1, 1)
	expected := []string{"5", "4"}
	assert.Equal(t, expected, deploymentUuids(res))

	res = retention(dpls, nil, "st1", "", 1, 2)
	expected = []string{"5", "4"}
	assert.Equal(t, expected, deploymentUuids(res))

	res = retention(dpls, nil, "st1", "", 1, 3)
	expected = []string{"5", "4", "3"}
	assert.Equal(t, expected, deploymentUuids(res))
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

	res := retention(dpls, new, "st1", "", 2, 1)
	expected := []string{"6", "5", "4", "1"}
	assert.Equal(t, expected, deploymentUuids(res))

	res = retention(dpls, new, "st2", "", 2, 1)
	expected = []string{"6", "5", "4", "1"}
	assert.Equal(t, expected, deploymentUuids(res))

	res = retention(dpls, new, "st1", "st2", 0, 0)
	expected = []string{"6", "4", "1"}
	assert.Equal(t, expected, deploymentUuids(res))

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
	res = retention(dpls, new, "st1", "", 2, 6)
	expected = []string{"1", "4", "5", "6", "7", "8", "9", "10", "11"}
	slices.Sort(expected)
	actual := deploymentUuids(res)
	slices.Sort(actual)

	assert.Equal(t, expected, actual)
}

func TestDeploymentCommitAndLoad(t *testing.T) {
	tmp := t.TempDir()
	filename := tmp + "/state.json"
	bk := broker.New()
	bk.Start()
	s, _ := New(bk, filename, tmp+"/gcroots", 2, 2)
	s.Commit()

	s1, _ := New(bk, filename, tmp+"/gcroots", 2, 2)
	err := s1.Load()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(s.data.Deployments))

	s.NewDeployment(&protobuf.Generation{}, "", "", "", "")
	s1, _ = New(bk, filename, tmp+"/gcroots", 2, 2)
	err = s1.Load()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(s.data.Deployments))
}

func TestLastDeployment(t *testing.T) {
	tmp := t.TempDir()
	bk := broker.New()
	bk.Start()
	s, _ := New(bk, "state.json", tmp+"/gcroots", 2, 2)
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
	s, _ := New(bk, tmp+"/filename", tmp+"/gcroots", 2, 2)
	s.NewGeneration("hostname", "repositoryPath", "repositoryDir", "systemAttr", &protobuf.RepositoryStatus{})
}
