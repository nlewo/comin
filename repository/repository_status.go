package repository

import (
	"encoding/json"
	"fmt"
	"github.com/nlewo/comin/types"
	"time"
)

type Branch struct {
	Name string
	CommitId string
	Error error
}

type MainBranch struct {
	Name string
	CommitId string
	ErrorMsg string

	OnTopOf string
}

type TestingBranch struct {
	Name string
	CommitId string
	ErrorMsg string

	OnTopOf string
}

type Remote struct {
	Name string
	Url string
	FetchErrorMsg string
	Main *MainBranch
	Testing *TestingBranch
	FetchedAt time.Time
	Fetched bool
}

type RepositoryStatus struct {
	// This is the deployed Main commit ID. It is used to ensure
	// fast forward
	SelectedCommitId string
	SelectedRemoteName string
	SelectedBranchName string
	SelectedBranchIsTesting bool
	MainCommitId string
	MainRemoteName string
	MainBranchName string
	Remotes []*Remote
}

func NewRepositoryStatus(config types.GitConfig, repositoryStatus RepositoryStatus) RepositoryStatus {
	r := RepositoryStatus{
		MainCommitId: repositoryStatus.MainCommitId,
	}
	r.Remotes = make([]*Remote, len(config.Remotes))
	for i, remote := range(config.Remotes) {
		r.Remotes[i] = &Remote{
			Name: remote.Name,
			
			Url: remote.URL,
			Main: &MainBranch{
				Name: remote.Branches.Main.Name,
			},
			Testing: &TestingBranch{
				Name: remote.Branches.Testing.Name,
			},
		}
	}
	return r
}

func (r RepositoryStatus) IsTesting() bool {
	return r.SelectedBranchIsTesting
}

func (r RepositoryStatus) remoteExists(remoteName string) bool {
	for _, remote := range r.Remotes {
		if remote.Name == remoteName {
			return true
		}
	}
	return false
}

func (r RepositoryStatus) Show() {
	res, _ := json.MarshalIndent(r, "", "\t")
	fmt.Printf("\n%s\n", string(res))
}

func (r RepositoryStatus) GetRemote(remoteName string) *Remote {
	for _, remote := range r.Remotes {
		if remote.Name == remoteName {
			return remote
		}
	}
	return nil
}
