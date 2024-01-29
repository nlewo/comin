package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/nlewo/comin/types"
	"github.com/sirupsen/logrus"
)

type repository struct {
	Repository       *git.Repository
	GitConfig        types.GitConfig
	RepositoryStatus RepositoryStatus
}

type Repository interface {
	FetchAndUpdate(ctx context.Context, remoteName string) (rsCh chan RepositoryStatus)
}

// repositoryStatus is the last saved repositoryStatus
func New(config types.GitConfig, repositoryStatus RepositoryStatus) (r *repository, err error) {
	r = &repository{}
	r.GitConfig = config
	r.Repository, err = repositoryOpen(config)
	if err != nil {
		return
	}
	err = manageRemotes(r.Repository, config.Remotes)
	if err != nil {
		return
	}
	r.RepositoryStatus = NewRepositoryStatus(config, repositoryStatus)
	return
}

func (r *repository) FetchAndUpdate(ctx context.Context, remoteName string) (rsCh chan RepositoryStatus) {
	rsCh = make(chan RepositoryStatus)
	go func() {
		// FIXME: switch to the FetchContext to clean resource up on timeout
		err := r.Fetch(remoteName)
		if err == nil {
			r.Update()
		}
		rsCh <- r.RepositoryStatus
	}()
	return rsCh
}

func (r *repository) Fetch(remoteName string) (err error) {
	var remotes []types.Remote
	var found bool
	r.RepositoryStatus.Error = nil
	if remoteName != "" {
		for _, remote := range r.GitConfig.Remotes {
			if remote.Name == remoteName {
				remotes = append(remotes, remote)
				found = true
			}
		}
		if !found {
			r.RepositoryStatus.Error = err
			return fmt.Errorf("The remote '%s' doesn't exist", remoteName)
		}
	} else {
		remotes = r.GitConfig.Remotes
	}

	for _, remote := range remotes {
		repositoryStatusRemote := r.RepositoryStatus.GetRemote(remote.Name)
		if err = fetch(*r, remote); err != nil {
			repositoryStatusRemote.FetchErrorMsg = err.Error()
		} else {
			repositoryStatusRemote.FetchErrorMsg = ""
			repositoryStatusRemote.Fetched = true
		}
		repositoryStatusRemote.FetchedAt = time.Now()
	}
	return
}

func (r *repository) Update() error {
	// We first walk on all Main branches in order to get a commit
	// from a Main branch. Once found, we could then walk on all
	// Testing branches to get a testing commit on top of the Main
	// commit.
	for _, remote := range r.RepositoryStatus.Remotes {
		// If an fetch error occured, we skip this remote
		if remote.FetchErrorMsg != "" {
			logrus.Debugf(
				"The remote %s is  skipped because of the fetch error: %s",
				remote.Name,
				remote.FetchErrorMsg)
			continue
		}
		head, msg, err := getHeadFromRemoteAndBranch(
			*r,
			remote.Name,
			remote.Main.Name,
			r.RepositoryStatus.MainCommitId)
		if err != nil {
			remote.Main.ErrorMsg = err.Error()
			logrus.Debugf("Failed to getHeadFromRemoteAndBranch: %s", err)
			continue
		} else {
			remote.Main.ErrorMsg = ""
		}

		remote.Main.CommitId = head.String()
		remote.Main.CommitMsg = msg
		remote.Main.OnTopOf = r.RepositoryStatus.MainCommitId

		if r.RepositoryStatus.SelectedCommitId == "" {
			r.RepositoryStatus.SelectedCommitId = head.String()
			r.RepositoryStatus.SelectedCommitMsg = msg
			r.RepositoryStatus.SelectedBranchName = remote.Main.Name
			r.RepositoryStatus.SelectedRemoteName = remote.Name
			r.RepositoryStatus.SelectedBranchIsTesting = false
		}
		if head.String() != r.RepositoryStatus.MainCommitId {
			r.RepositoryStatus.SelectedCommitId = head.String()
			r.RepositoryStatus.SelectedCommitMsg = msg
			r.RepositoryStatus.SelectedBranchName = remote.Main.Name
			r.RepositoryStatus.SelectedBranchIsTesting = false
			r.RepositoryStatus.SelectedRemoteName = remote.Name
			r.RepositoryStatus.MainCommitId = head.String()
			r.RepositoryStatus.MainBranchName = remote.Main.Name
			r.RepositoryStatus.MainRemoteName = remote.Name
			break
		}
	}

	for _, remote := range r.RepositoryStatus.Remotes {
		// If an fetch error occured, we skip this remote
		if remote.FetchErrorMsg != "" {
			logrus.Debugf(
				"The remote %s is  skipped because of the fetch error: %s",
				remote.Name,
				remote.FetchErrorMsg)
			continue
		}
		if remote.Testing.Name == "" {
			continue
		}

		head, msg, err := getHeadFromRemoteAndBranch(
			*r,
			remote.Name,
			remote.Testing.Name,
			r.RepositoryStatus.MainCommitId)
		if err != nil {
			remote.Testing.ErrorMsg = err.Error()
			logrus.Debugf("Failed to getHeadFromRemoteAndBranch: %s", err)
			continue
		} else {
			remote.Testing.ErrorMsg = ""
		}

		remote.Testing.CommitId = head.String()
		remote.Testing.CommitMsg = msg
		remote.Testing.OnTopOf = r.RepositoryStatus.MainCommitId

		if head.String() != r.RepositoryStatus.SelectedCommitId && head.String() != r.RepositoryStatus.MainCommitId {
			r.RepositoryStatus.SelectedCommitId = head.String()
			r.RepositoryStatus.SelectedCommitMsg = msg
			r.RepositoryStatus.SelectedBranchName = remote.Testing.Name
			r.RepositoryStatus.SelectedBranchIsTesting = true
			r.RepositoryStatus.SelectedRemoteName = remote.Name
			break
		}
	}

	if err := hardReset(*r, plumbing.NewHash(r.RepositoryStatus.SelectedCommitId)); err != nil {
		r.RepositoryStatus.Error = err
		return err
	}
	return nil
}
