package deployer

import (
	"context"
	"errors"
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

type deployResumerMock struct {
	outPath string
	resume  chan error
}

func (r deployResumerMock) InterruptedDeploy() (string, bool) {
	return r.outPath, true
}

func (r deployResumerMock) ResumeDeploy(stdout, stderr io.Writer) (string, error) {
	return "profile-path", <-r.resume
}

func TestDeployerResumesInterruptedDeployment(t *testing.T) {
	tests := []struct {
		name           string
		resumeErr      error
		expectedStatus string
	}{
		{
			name:           "the interrupted deployment succeeds",
			expectedStatus: store.StatusToString(store.Done),
		},
		{
			name:           "the interrupted deployment fails",
			resumeErr:      errors.New("the activation failed"),
			expectedStatus: store.StatusToString(store.Failed),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deployFunc = func(context.Context, string, string, []string, io.WriteCloser, io.WriteCloser) (bool, string, error) {
				return false, "", nil
			}
			tmp := t.TempDir()
			bk := broker.New()
			bk.Start()
			s, err := store.New(bk, tmp+"/state.json", tmp+"/gcroots", 1, 1, 1)
			assert.Nil(t, err)
			interrupted := s.NewDeployment(&protobuf.Generation{Uuid: "generation", OutPath: "out-path"}, "switch", "", "", "")
			assert.Nil(t, s.DeploymentStarted(interrupted.Uuid, "", ""))

			resumer := deployResumerMock{outPath: "out-path", resume: make(chan error)}
			d := New(s, deployFunc, nil, "", bk)
			d.DeployResumer = resumer
			d.Run(t.Context())
			assert.EventuallyWithT(t, func(c *assert.CollectT) {
				assert.True(c, d.IsDeploying())
			}, 5*time.Second, 100*time.Millisecond)

			resumer.resume <- tt.resumeErr
			dpl := <-d.DeploymentDoneCh
			assert.Equal(t, interrupted.Uuid, dpl.Uuid)
			assert.Equal(t, tt.expectedStatus, dpl.Status)
			assert.Equal(t, "profile-path", dpl.ProfilePath)
			assert.NotNil(t, dpl.EndedAt)
			assert.False(t, d.IsDeploying())
		})
	}
}
