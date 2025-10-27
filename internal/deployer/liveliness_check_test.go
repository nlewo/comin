package deployer

import (
	"context"
	"testing"

	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestLivelinessCheck(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	assert.Nil(t, err)

	deployFunc := func(ctx context.Context, outPath, operation string) (bool, string, error) {
		return false, "", nil
	}

	// Test with a failing liveliness check
	d := New(s, deployFunc, nil, "", "sh -c 'exit 1'")
	d.Submit(&protobuf.Generation{Uuid: "a"})
	go d.Run()
	deployment := <-d.DeploymentDoneCh
	assert.Equal(t, store.StatusToString(store.Failed), deployment.Status)

	// Test with a succeeding liveliness check
	d = New(s, deployFunc, nil, "", "sh -c 'exit 0'")
	d.Submit(&protobuf.Generation{Uuid: "b"})
	go d.Run()
	deployment = <-d.DeploymentDoneCh
	assert.Equal(t, store.StatusToString(store.Done), deployment.Status)
}
