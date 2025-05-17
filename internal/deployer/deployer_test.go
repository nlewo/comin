package deployer

import (
	"context"
	"testing"
	"time"

	"github.com/nlewo/comin/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestDeployerBasic(t *testing.T) {
	deployDone := make(chan struct{})
	var deployFunc = func(context.Context, string, string) (bool, string, error) {
		<-deployDone
		return false, "profile-path", nil
	}

	d := New(deployFunc, nil, "")
	d.Run()
	assert.False(t, d.IsDeploying)

	g := store.Generation{SelectedCommitId: "commit-1"}
	d.Submit(g)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, d.IsDeploying)
	}, 5*time.Second, 100*time.Millisecond)

	deployDone <- struct{}{}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, d.IsDeploying)
		assert.Equal(c, "profile-path", d.Deployment.ProfilePath)
	}, 5*time.Second, 100*time.Millisecond)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		dpl := <-d.DeploymentDoneCh
		assert.Equal(c, "profile-path", dpl.ProfilePath)
		assert.Equal(c, "commit-1", dpl.Generation.SelectedCommitId)
	}, 5*time.Second, 100*time.Millisecond)
}

func TestDeployerSubmit(t *testing.T) {
	deployDone := make(chan struct{})
	var deployFunc = func(context.Context, string, string) (bool, string, error) {
		<-deployDone
		return false, "profile-path", nil
	}

	d := New(deployFunc, nil, "")
	d.Run()
	assert.False(t, d.IsDeploying)

	d.Submit(store.Generation{SelectedCommitId: "commit-1"})
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, d.IsDeploying)
		assert.Nil(c, d.GenerationToDeploy)
	}, 5*time.Second, 100*time.Millisecond)

	d.Submit(store.Generation{SelectedCommitId: "commit-2"})
	d.Submit(store.Generation{SelectedCommitId: "commit-3"})
	assert.NotNil(t, d.GenerationToDeploy)

	// To simulate the end of 2 deployments (commit-1 and commit-3)
	deployDone <- struct{}{}
	deployDone <- struct{}{}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, d.IsDeploying)
		assert.Equal(c, "profile-path", d.Deployment.ProfilePath)
		assert.Nil(t, d.GenerationToDeploy)
	}, 5*time.Second, 100*time.Millisecond)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		dpl := <-d.DeploymentDoneCh
		assert.Equal(c, "profile-path", dpl.ProfilePath)
		assert.Equal(c, "commit-1", dpl.Generation.SelectedCommitId)
	}, 5*time.Second, 100*time.Millisecond)
}
