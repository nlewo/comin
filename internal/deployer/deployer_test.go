package deployer

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/store"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/stretchr/testify/assert"
)

func TestDeployerBasic(t *testing.T) {
	deployDone := make(chan struct{})
	var deployFunc = func(context.Context, string, string, []string, io.WriteCloser, io.WriteCloser) (bool, string, error) {
		<-deployDone
		return false, "profile-path", nil
	}

	tmp := t.TempDir()
	bk := broker.New()
	bk.Start()

	s, err := store.New(bk, tmp+"/state.json", tmp+"/gcroots", 1, 1, 1)
	assert.Nil(t, err)
	d := New(s, deployFunc, nil, "", bk)
	d.Run(t.Context())
	assert.False(t, d.IsDeploying())

	g := &protobuf.Generation{
		Source: &protobuf.Source{
			Source: &protobuf.Source_Git{
				Git: &protobuf.Git{
					SelectedCommitId: "commit-1",
				},
			},
		},
	}
	d.Submit(g, "test", false, "")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, d.IsDeploying())
	}, 5*time.Second, 100*time.Millisecond)

	deployDone <- struct{}{}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, d.IsDeploying())
		assert.Equal(c, "profile-path", d.Deployment().ProfilePath)
	}, 5*time.Second, 100*time.Millisecond)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		dpl := <-d.DeploymentDoneCh
		assert.Equal(c, "profile-path", dpl.ProfilePath)
		assert.Equal(c, "commit-1", dpl.Generation.Source.GetGit().SelectedCommitId)
	}, 5*time.Second, 100*time.Millisecond)
}

func TestDeployerSubmit(t *testing.T) {
	deployDone := make(chan struct{})
	var deployFunc = func(context.Context, string, string, []string, io.WriteCloser, io.WriteCloser) (bool, string, error) {
		<-deployDone
		return false, "profile-path", nil
	}

	tmp := t.TempDir()
	bk := broker.New()
	bk.Start()

	s, err := store.New(bk, tmp+"/state.json", tmp+"/gcroots", 1, 1, 1)
	assert.Nil(t, err)
	d := New(s, deployFunc, nil, "", bk)
	d.Run(t.Context())
	assert.False(t, d.IsDeploying())

	d.Submit(&protobuf.Generation{
		Source: &protobuf.Source{
			Source: &protobuf.Source_Git{
				Git: &protobuf.Git{
					SelectedCommitId: "commit-1",
				},
			},
		},
	}, "test", false, "")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, d.IsDeploying())
		assert.Nil(c, d.GenerationToDeploy)
	}, 5*time.Second, 100*time.Millisecond)

	d.Submit(&protobuf.Generation{
		Source: &protobuf.Source{
			Source: &protobuf.Source_Git{
				Git: &protobuf.Git{
					SelectedCommitId: "commit-2",
				},
			},
		},
	}, "test", false, "")
	d.Submit(&protobuf.Generation{
		Source: &protobuf.Source{
			Source: &protobuf.Source_Git{
				Git: &protobuf.Git{
					SelectedCommitId: "commit-3",
				},
			},
		},
	}, "test", false, "")
	assert.NotNil(t, d.GenerationToDeploy)

	// To simulate the end of 2 deployments (commit-1 and commit-3)
	deployDone <- struct{}{}
	deployDone <- struct{}{}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, d.IsDeploying())
		assert.Equal(c, "profile-path", d.Deployment().ProfilePath)
		assert.Nil(t, d.GenerationToDeploy)
	}, 5*time.Second, 100*time.Millisecond)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		dpl := <-d.DeploymentDoneCh
		assert.Equal(c, "profile-path", dpl.ProfilePath)
		assert.Equal(c, "commit-1", dpl.Generation.Source.GetGit().SelectedCommitId)
	}, 5*time.Second, 100*time.Millisecond)
}

func TestDeployerSuspend(t *testing.T) {
	deployDone := make(chan struct{})
	var deployFunc = func(context.Context, string, string, []string, io.WriteCloser, io.WriteCloser) (bool, string, error) {
		<-deployDone
		return false, "profile-path", nil
	}

	tmp := t.TempDir()
	bk := broker.New()
	bk.Start()

	s, err := store.New(bk, tmp+"/state.json", tmp+"/gcroots", 1, 1, 1)
	assert.Nil(t, err)
	d := New(s, deployFunc, nil, "", bk)
	d.Run(t.Context())
	assert.False(t, d.IsSuspended())
	d.Suspend("suspended for testing")
	assert.True(t, d.IsSuspended())
	assert.False(t, d.IsDeploying())
	assert.False(t, d.RunnerIsSuspended())

	d.Submit(&protobuf.Generation{
		Source: &protobuf.Source{
			Source: &protobuf.Source_Git{
				Git: &protobuf.Git{
					SelectedCommitId: "commit-1",
				},
			},
		},
	}, "test", false, "")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(t, d.RunnerIsSuspended())
	}, 3*time.Second, 100*time.Millisecond)

	d.Resume()
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(t, d.RunnerIsSuspended())
		assert.True(t, d.IsDeploying())
	}, 3*time.Second, 100*time.Millisecond)
}

func TestIsAlreadyDeployed(t *testing.T) {
	const outPath = "/nix/store/aaa-nixos-system"

	previous := func(operation string) *protobuf.Deployment {
		return &protobuf.Deployment{
			Uuid:       "previous",
			Operation:  operation,
			Status:     store.StatusToString(store.Done),
			Generation: &protobuf.Generation{Uuid: "g1", OutPath: outPath},
		}
	}
	generation := &protobuf.Generation{Uuid: "g2", OutPath: outPath}

	newDeployer := func(t *testing.T, previous *protobuf.Deployment, current string) *Deployer {
		t.Helper()
		tmp := t.TempDir()
		bk := broker.New()
		bk.Start()
		s, err := store.New(bk, tmp+"/state.json", tmp+"/gcroots", 1, 1, 1)
		assert.Nil(t, err)
		d := New(s, nil, previous, "", bk)
		d.currentStorepath = func() string { return current }
		return d
	}

	t.Run("no previous deployment", func(t *testing.T) {
		d := newDeployer(t, nil, outPath)
		assert.False(t, d.IsAlreadyDeployed(generation, "switch"))
	})

	t.Run("a different out path is a new deployment", func(t *testing.T) {
		d := newDeployer(t, previous("switch"), outPath)
		other := &protobuf.Generation{Uuid: "g2", OutPath: "/nix/store/bbb-nixos-system"}
		assert.False(t, d.IsAlreadyDeployed(other, "switch"))
	})

	t.Run("a different operation is a new deployment", func(t *testing.T) {
		d := newDeployer(t, previous("switch"), outPath)
		assert.False(t, d.IsAlreadyDeployed(generation, "test"))
	})

	t.Run("switch is deployed whatever the running system", func(t *testing.T) {
		// A switch writes a boot entry, so it survives a reboot. `boot` never
		// activates at all, so neither may be judged by the running system.
		for _, operation := range []string{"switch", "boot"} {
			d := newDeployer(t, previous(operation), "/nix/store/ccc-nixos-system")
			assert.True(t, d.IsAlreadyDeployed(generation, operation), operation)
		}
	})

	t.Run("test is deployed while it is the running system", func(t *testing.T) {
		d := newDeployer(t, previous("test"), outPath)
		assert.True(t, d.IsAlreadyDeployed(generation, "test"))
	})

	// The regression: a reboot discards a test activation but keeps the record of
	// it, so comin skipped the redeploy and left the machine on the booted system.
	t.Run("test is not deployed once a reboot reverted it", func(t *testing.T) {
		d := newDeployer(t, previous("test"), "/nix/store/ccc-nixos-system")
		assert.False(t, d.IsAlreadyDeployed(generation, "test"))
	})
}
