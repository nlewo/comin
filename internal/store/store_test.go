package store

import (
	"testing"

	"github.com/nlewo/comin/internal/protobuf"
	"github.com/stretchr/testify/assert"
)

func TestDeploymentCommitAndLoad(t *testing.T) {
	tmp := t.TempDir()
	filename := tmp + "/state.json"
	s, _ := New(filename, tmp+"/gcroots", 2, 2)
	err := s.Commit()
	assert.Nil(t, err)

	s1, _ := New(filename, tmp+"/gcroots", 2, 2)
	err = s1.Load()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(s.data.Deployments))

	s.DeploymentInsert(&protobuf.Deployment{Uuid: "1", Operation: "switch"})
	_ = s.Commit()
	assert.Nil(t, err)

	s1, _ = New(filename, tmp+"/gcroots", 2, 2)
	err = s1.Load()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(s.data.Deployments))
}

func TestLastDeployment(t *testing.T) {
	tmp := t.TempDir()
	s, _ := New("state.json", tmp+"/gcroots", 2, 2)
	ok, _ := s.LastDeployment()
	assert.False(t, ok)
	s.DeploymentInsert(&protobuf.Deployment{Uuid: "1", Operation: "switch"})
	s.DeploymentInsert(&protobuf.Deployment{Uuid: "2", Operation: "switch"})
	ok, last := s.LastDeployment()
	assert.True(t, ok)
	assert.Equal(t, "2", last.Uuid)
}

func TestDeploymentInsert(t *testing.T) {
	tmp := t.TempDir()
	s, _ := New("state.json", tmp+"/gcroots", 2, 2)
	var hasEvicted bool
	var evicted *protobuf.Deployment
	hasEvicted, _ = s.DeploymentInsert(&protobuf.Deployment{Uuid: "1", Operation: "switch"})
	assert.False(t, hasEvicted)
	hasEvicted, _ = s.DeploymentInsert(&protobuf.Deployment{Uuid: "2", Operation: "switch"})
	assert.False(t, hasEvicted)
	hasEvicted, evicted = s.DeploymentInsert(&protobuf.Deployment{Uuid: "3", Operation: "switch"})
	assert.True(t, hasEvicted)
	assert.Equal(t, "1", evicted.Uuid)
	expected := []*protobuf.Deployment{
		{Uuid: "3", Operation: "switch"},
		{Uuid: "2", Operation: "switch"},
	}
	assert.Equal(t, expected, s.DeploymentList())

	hasEvicted, _ = s.DeploymentInsert(&protobuf.Deployment{Uuid: "4", Operation: "test"})
	assert.False(t, hasEvicted)
	hasEvicted, _ = s.DeploymentInsert(&protobuf.Deployment{Uuid: "5", Operation: "test"})
	assert.False(t, hasEvicted)
	hasEvicted, evicted = s.DeploymentInsert(&protobuf.Deployment{Uuid: "6", Operation: "test"})
	assert.True(t, hasEvicted)
	assert.Equal(t, "4", evicted.Uuid)
	expected = []*protobuf.Deployment{
		{Uuid: "6", Operation: "test"},
		{Uuid: "5", Operation: "test"},
		{Uuid: "3", Operation: "switch"},
		{Uuid: "2", Operation: "switch"},
	}
	assert.Equal(t, expected, s.DeploymentList())

	hasEvicted, evicted = s.DeploymentInsert(&protobuf.Deployment{Uuid: "7", Operation: "switch"})
	assert.True(t, hasEvicted)
	assert.Equal(t, "2", evicted.Uuid)
	hasEvicted, evicted = s.DeploymentInsert(&protobuf.Deployment{Uuid: "8", Operation: "switch"})
	assert.True(t, hasEvicted)
	assert.Equal(t, "3", evicted.Uuid)
}

func TestNewGeneration(t *testing.T) {
	tmp := t.TempDir()
	s, _ := New(tmp+"/filename", tmp+"/gcroots", 2, 2)
	s.NewGeneration("hostname", "repositoryPath", "repositoryDir", &protobuf.RepositoryStatus{})
}
