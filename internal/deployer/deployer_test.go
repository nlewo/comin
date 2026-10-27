package deployer

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
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
		OutPath: "/storepath/commit-1",
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
		OutPath: "/storepath/commit-2",
	}, "test", false, "")
	d.Submit(&protobuf.Generation{
		Source: &protobuf.Source{
			Source: &protobuf.Source_Git{
				Git: &protobuf.Git{
					SelectedCommitId: "commit-3",
				},
			},
		},
		OutPath: "/storepath/commit-3",
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
		assert.Equal(c, "/storepath/commit-1", dpl.Generation.OutPath)
	}, 5*time.Second, 100*time.Millisecond)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		dpl := <-d.DeploymentDoneCh
		assert.Equal(c, "profile-path", dpl.ProfilePath)
		assert.Equal(c, "commit-3", dpl.Generation.Source.GetGit().SelectedCommitId)
		assert.Equal(c, "/storepath/commit-3", dpl.Generation.OutPath)
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
		assert.True(c, d.RunnerIsSuspended())
	}, 3*time.Second, 100*time.Millisecond)

	d.Resume()
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, d.RunnerIsSuspended())
		assert.True(c, d.IsDeploying())
	}, 3*time.Second, 100*time.Millisecond)
}

// TestIsAlreadyDeployedUnchangedSystem tests the issue where an unchanged system
// can be activated again because IsAlreadyDeployed compares against previousDeployment
// instead of the latest completed deployment.
// This reproduces the issue described in https://github.com/nlewo/comin/issues/192#issuecomment-6106796805
func TestIsAlreadyDeployedUnchangedSystem(t *testing.T) {
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

	// Start with deployment A (path-A)
	genA := &protobuf.Generation{
		Uuid:    "gen-a",
		OutPath: "path-A",
		Source: &protobuf.Source{
			Source: &protobuf.Source_Git{
				Git: &protobuf.Git{
					SelectedCommitId: "commit-a",
				},
			},
		},
	}

	d := New(s, deployFunc, nil, "", bk)
	d.Run(t.Context())

	// Submit and complete deployment A
	d.Submit(genA, "switch", false, "")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, d.IsDeploying())
	}, 5*time.Second, 100*time.Millisecond)

	deployDone <- struct{}{}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, d.IsDeploying())
		assert.NotNil(c, d.Deployment())
		assert.Equal(c, "gen-a", d.Deployment().Generation.Uuid)
		assert.Equal(c, "path-A", d.Deployment().Generation.OutPath)
	}, 5*time.Second, 100*time.Millisecond)

	// Drain DeploymentDoneCh
	select {
	case <-d.DeploymentDoneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for deployment done")
	}

	// Now submit deployment B with a different path (path-B)
	genB := &protobuf.Generation{
		Uuid:    "gen-b",
		OutPath: "path-B",
		Source: &protobuf.Source{
			Source: &protobuf.Source_Git{
				Git: &protobuf.Git{
					SelectedCommitId: "commit-b",
				},
			},
		},
	}

	d.Submit(genB, "switch", false, "")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, d.IsDeploying())
	}, 5*time.Second, 100*time.Millisecond)

	deployDone <- struct{}{}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, d.IsDeploying())
		assert.NotNil(c, d.Deployment())
		assert.Equal(c, "gen-b", d.Deployment().Generation.Uuid)
		assert.Equal(c, "path-B", d.Deployment().Generation.OutPath)
	}, 5*time.Second, 100*time.Millisecond)

	// Drain DeploymentDoneCh
	select {
	case <-d.DeploymentDoneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for deployment done")
	}

	// Now submit deployment C with the same path as A (path-A)
	// This should be deploy path-A
	genC := &protobuf.Generation{
		Uuid:    "gen-c",
		OutPath: "path-A",
		Source: &protobuf.Source{
			Source: &protobuf.Source_Git{
				Git: &protobuf.Git{
					SelectedCommitId: "commit-c",
				},
			},
		},
	}
	d.Submit(genC, "switch", false, "")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, d.IsDeploying())
	}, 5*time.Second, 100*time.Millisecond)

	deployDone <- struct{}{}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, d.IsDeploying())
		assert.NotNil(c, d.Deployment())
		assert.Equal(c, "gen-c", d.Deployment().Generation.Uuid)
		assert.Equal(c, "path-A", d.Deployment().Generation.OutPath)
	}, 5*time.Second, 100*time.Millisecond)
}

func newGeneration(uuid, outPath string) *protobuf.Generation {
	return &protobuf.Generation{
		Uuid:    uuid,
		OutPath: outPath,
		Source: &protobuf.Source{
			Source: &protobuf.Source_Git{
				Git: &protobuf.Git{
					SelectedCommitId: uuid,
				},
			},
		},
	}
}

func waitDeploymentDone(t *testing.T, d *Deployer) *protobuf.Deployment {
	t.Helper()
	select {
	case dpl := <-d.DeploymentDoneCh:
		return dpl
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for deployment done")
		return nil
	}
}

func endDeployment(t *testing.T, deployErrs chan<- error, err error) {
	t.Helper()
	select {
	case deployErrs <- err:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for a deployment to be started")
	}
}

func assertNoDeploymentDone(t *testing.T, d *Deployer) {
	t.Helper()
	select {
	case dpl := <-d.DeploymentDoneCh:
		t.Fatalf("generation %s has been deployed", dpl.Generation.Uuid)
	case <-time.After(1 * time.Second):
	}
}

// A failed deployment's out path is deployed again.
func TestIsAlreadyDeployedFailedDeployment(t *testing.T) {
	deployErrs := make(chan error)
	var deployFunc = func(context.Context, string, string, []string, io.WriteCloser, io.WriteCloser) (bool, string, error) {
		if err := <-deployErrs; err != nil {
			return false, "", err
		}
		return false, "profile-path", nil
	}

	tmp := t.TempDir()
	bk := broker.New()
	bk.Start()

	s, err := store.New(bk, tmp+"/state.json", tmp+"/gcroots", 1, 1, 1)
	assert.Nil(t, err)
	d := New(s, deployFunc, nil, "", bk)
	d.Run(t.Context())

	d.Submit(newGeneration("gen-a", "path-A"), "switch", false, "")
	endDeployment(t, deployErrs, errors.New("switch-to-configuration failed"))
	dpl := waitDeploymentDone(t, d)
	assert.Equal(t, "gen-a", dpl.Generation.Uuid)
	assert.NotEmpty(t, dpl.ErrorMsg)

	assert.False(t, d.IsAlreadyDeployed(newGeneration("gen-b", "path-A"), "switch"))
	d.Submit(newGeneration("gen-b", "path-A"), "switch", false, "")
	endDeployment(t, deployErrs, nil)
	dpl = waitDeploymentDone(t, d)
	assert.Equal(t, "gen-b", dpl.Generation.Uuid)
	assert.Empty(t, dpl.ErrorMsg)

	assert.True(t, d.IsAlreadyDeployed(newGeneration("gen-c", "path-A"), "switch"))
}

// Deployments end when an error or nil is sent on the channel.
func newDeployerWithBlockingDeployFunc(t *testing.T) (*Deployer, chan error, *atomic.Int32) {
	deployErrs := make(chan error)
	started := &atomic.Int32{}
	var deployFunc = func(context.Context, string, string, []string, io.WriteCloser, io.WriteCloser) (bool, string, error) {
		started.Add(1)
		if err := <-deployErrs; err != nil {
			return false, "", err
		}
		return false, "profile-path", nil
	}

	tmp := t.TempDir()
	bk := broker.New()
	bk.Start()

	s, err := store.New(bk, tmp+"/state.json", tmp+"/gcroots", 1, 1, 1)
	assert.Nil(t, err)
	d := New(s, deployFunc, nil, "", bk)
	d.Run(t.Context())
	return d, deployErrs, started
}

// Submitted during a failed deployment of its out path: deployed.
func TestIsAlreadyDeployedSubmittedDuringFailedDeployment(t *testing.T) {
	d, deployErrs, started := newDeployerWithBlockingDeployFunc(t)

	d.Submit(newGeneration("gen-a", "path-A"), "switch", false, "")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, int32(1), started.Load())
	}, 5*time.Second, 10*time.Millisecond)

	assert.False(t, d.IsAlreadyDeployed(newGeneration("gen-b", "path-A"), "switch"))
	d.Submit(newGeneration("gen-b", "path-A"), "switch", false, "")

	endDeployment(t, deployErrs, errors.New("switch-to-configuration failed"))
	dpl := waitDeploymentDone(t, d)
	assert.Equal(t, "gen-a", dpl.Generation.Uuid)
	assert.NotEmpty(t, dpl.ErrorMsg)

	endDeployment(t, deployErrs, nil)
	dpl = waitDeploymentDone(t, d)
	assert.Equal(t, "gen-b", dpl.Generation.Uuid)
	assert.Empty(t, dpl.ErrorMsg)
}

// Submitted during a successful deployment of its out path: skipped unless forced.
func TestIsAlreadyDeployedSubmittedDuringSuccessfulDeployment(t *testing.T) {
	d, deployErrs, started := newDeployerWithBlockingDeployFunc(t)

	d.Submit(newGeneration("gen-a", "path-A"), "switch", false, "")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, int32(1), started.Load())
	}, 5*time.Second, 10*time.Millisecond)

	d.Submit(newGeneration("gen-b", "path-A"), "switch", false, "")

	endDeployment(t, deployErrs, nil)
	dpl := waitDeploymentDone(t, d)
	assert.Equal(t, "gen-a", dpl.Generation.Uuid)

	assertNoDeploymentDone(t, d)
	assert.Equal(t, int32(1), started.Load())
	assert.Equal(t, "gen-a", d.Deployment().Generation.Uuid)
	d.mu.Lock()
	assert.Nil(t, d.GenerationToDeploy)
	d.mu.Unlock()

	d.Submit(newGeneration("gen-c", "path-A"), "switch", true, "")
	endDeployment(t, deployErrs, nil)
	dpl = waitDeploymentDone(t, d)
	assert.Equal(t, "gen-c", dpl.Generation.Uuid)
}

// Submitted while another generation is queued: the system stays on path-A.
func TestIsAlreadyDeployedSubmittedWhileQueued(t *testing.T) {
	d, deployErrs, started := newDeployerWithBlockingDeployFunc(t)

	d.Submit(newGeneration("gen-a", "path-A"), "switch", false, "")
	endDeployment(t, deployErrs, nil)
	waitDeploymentDone(t, d)

	d.Suspend("suspended for testing")
	d.Submit(newGeneration("gen-b", "path-B"), "switch", false, "")
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, d.RunnerIsSuspended())
	}, 5*time.Second, 10*time.Millisecond)

	assert.False(t, d.IsAlreadyDeployed(newGeneration("gen-c", "path-A"), "switch"))
	d.Submit(newGeneration("gen-c", "path-A"), "switch", false, "")
	d.Resume()

	assertNoDeploymentDone(t, d)
	assert.Equal(t, int32(1), started.Load())
	assert.Equal(t, "gen-a", d.Deployment().Generation.Uuid)
}
