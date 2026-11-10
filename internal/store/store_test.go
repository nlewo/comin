package store

import (
	"context"
	"errors"
	"testing"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/stretchr/testify/assert"
)


func TestDeploymentCommitAndLoad(t *testing.T) {
	tmp := t.TempDir()
	filename := tmp + "/state.json"
	bk := broker.New()
	bk.Start()
	s, _ := New(bk, filename, tmp+"/gcroots", 2, 2, 5, 5)
	s.Commit()

	s1, _ := New(bk, filename, tmp+"/gcroots", 2, 2, 5, 5)
	err := s1.Load()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(s.persisted.Deployments))

	s.NewDeployment(&protobuf.Generation{}, "", "", "", "")
	s1, _ = New(bk, filename, tmp+"/gcroots", 2, 2, 5, 5)
	err = s1.Load()
	assert.Nil(t, err)
	assert.Equal(t, 2, len(s.persisted.Deployments))
}

func TestLastDeployment(t *testing.T) {
	tmp := t.TempDir()
	bk := broker.New()
	bk.Start()
	s, _ := New(bk, "state.json", tmp+"/gcroots", 2, 2, 5, 5)
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
	s, _ := New(bk, tmp+"/filename", tmp+"/gcroots", 2, 2, 5, 5)
	s.NewGeneration("hostname", "repositoryDir", "systemAttr", &protobuf.GitRepositoryStatus{}, 0)
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

// A build preempted by a newer commit finishes with context.Canceled. It must
// not be recorded as a failure: comin_last_build_failed is derived from this
// status, so folding cancellation into BuildFailed makes every pair of
// closely-spaced commits look like a broken build until the retry completes.
func TestGenerationBuildCanceledIsNotAFailure(t *testing.T) {
	tmp := t.TempDir()
	bk := broker.New()
	bk.Start()
	s, err := New(bk, tmp+"/filename", tmp+"/gcroots", 2, 2, 5, 5)
	assert.Nil(t, err)

	g := s.NewGeneration("hostname", "repositoryDir", "systemAttr", &protobuf.GitRepositoryStatus{}, 0)
	assert.Nil(t, s.GenerationEvalStarted(g.Uuid))
	assert.Nil(t, s.GenerationEvalFinished(g.Uuid, "drvPath", "outPath", "machineId", context.Canceled))

	got, err := s.GenerationGet(g.Uuid)
	assert.Nil(t, err)
	assert.Equal(t, EvalCanceled.String(), got.EvalStatus)
	assert.NotEqual(t, EvalFailed.String(), got.EvalStatus)

	g2 := s.NewGeneration("hostname", "repositoryDir", "systemAttr", &protobuf.GitRepositoryStatus{}, 0)
	assert.Nil(t, s.GenerationEvalStarted(g2.Uuid))
	assert.Nil(t, s.GenerationEvalFinished(g2.Uuid, "drvPath", "outPath", "machineId", nil))
	assert.Nil(t, s.GenerationBuildStart(g2.Uuid, "reason"))
	assert.Nil(t, s.GenerationBuildFinished(g2.Uuid, context.Canceled))

	got2, err := s.GenerationGet(g2.Uuid)
	assert.Nil(t, err)
	assert.Equal(t, BuildCanceled.String(), got2.BuildStatus)
	assert.NotEqual(t, BuildFailed.String(), got2.BuildStatus)
	// The output path was never realised, so the generation still has to be
	// built and must not be mistaken for a successful one.
	assert.True(t, GenerationHasToBeBuilt(&got2))
}

// A genuine build error must still be recorded as a failure.
func TestGenerationBuildErrorIsAFailure(t *testing.T) {
	tmp := t.TempDir()
	bk := broker.New()
	bk.Start()
	s, err := New(bk, tmp+"/filename", tmp+"/gcroots", 2, 2, 5, 5)
	assert.Nil(t, err)

	g := s.NewGeneration("hostname", "repositoryDir", "systemAttr", &protobuf.GitRepositoryStatus{}, 0)
	assert.Nil(t, s.GenerationEvalStarted(g.Uuid))
	assert.Nil(t, s.GenerationEvalFinished(g.Uuid, "drvPath", "outPath", "machineId", nil))
	assert.Nil(t, s.GenerationBuildStart(g.Uuid, "reason"))
	assert.Nil(t, s.GenerationBuildFinished(g.Uuid, errors.New("boom")))

	got, err := s.GenerationGet(g.Uuid)
	assert.Nil(t, err)
	assert.Equal(t, BuildFailed.String(), got.BuildStatus)
}
