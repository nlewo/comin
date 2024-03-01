package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/deployment"
	"github.com/nlewo/comin/internal/generation"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func generationFactory() generation.Generation {
	return generation.Generation{
		UUID:               uuid.NewString(),
		SelectedRemoteName: "remote",
		SelectedBranchName: "main",
		SelectedCommitId:   "commit-id",
		SelectedCommitMsg:  "commit-msg",
	}
}

func deploymentFactory(g generation.Generation) deployment.Deployment {
	return deployment.Deployment{
		UUID:       uuid.NewString(),
		Generation: g,
	}
}

func TestNew(t *testing.T) {
	// New("/tmp/bla4.sqlite")
	New(":memory:")

}

func TestDeploymentInsert(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	g1 := generationFactory()
	s, err := New(":memory:")
	defer s.Close()
	assert.Nil(t, err)
	err = s.GenerationInsert(g1)
	assert.Nil(t, err)
	d1 := deploymentFactory(g1)
	date, _ := time.Parse(time.DateOnly, "2021-11-22")
	d1.StartAt = date
	err = s.DeploymentInsert(d1)
	assert.Nil(t, err)

	d, ok := s.DeploymentGet(d1.UUID)
	assert.True(t, ok)
	assert.Equal(t, date, d.StartAt)
	assert.Equal(t, d1, d)
	assert.Equal(t, d.Generation.UUID, g1.UUID)
}

func TestDeploymentList(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	g1 := generationFactory()
	g2 := generationFactory()
	g3 := generationFactory()
	s, err := New(":memory:")
	defer s.Close()
	assert.Nil(t, err)

	s.GenerationInsert(g1)
	s.GenerationInsert(g2)
	s.GenerationInsert(g3)
	d1 := deploymentFactory(g1)
	d2 := deploymentFactory(g2)
	d3 := deploymentFactory(g3)
	s.DeploymentInsert(d1)
	s.DeploymentInsert(d2)
	s.DeploymentInsert(d3)

	l, err := s.DeploymentList(context.TODO(), 2)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(l))

}
func TestGenerationInsert(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	g1 := generationFactory()
	s, err := New(":memory:")
	defer s.Close()
	assert.Nil(t, err)
	err = s.GenerationInsert(g1)
	assert.Nil(t, err)

	g, ok := s.GenerationGetByUuid(g1.UUID)
	assert.True(t, ok)
	assert.Equal(t, g1, g)

	g, ok = s.GenerationGetByUuid("not-existing")
	assert.False(t, ok)
}

func TestGenerationList(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	g1 := generationFactory()
	g2 := generationFactory()
	s, err := New(":memory:")
	defer s.Close()
	assert.Nil(t, err)

	err = s.GenerationInsert(g1)
	assert.Nil(t, err)

	err = s.GenerationInsert(g2)
	assert.Nil(t, err)

	gs, err := s.GenerationList(context.TODO())
	assert.Equal(t, gs[0], g1)
	assert.Equal(t, gs[1], g2)
}
