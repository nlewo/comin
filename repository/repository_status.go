package repository

import (
	"encoding/json"
	"fmt"
	"github.com/nlewo/comin/types"
	"time"
)

type MainBranch struct {
	Name     string `json:"name,omitempty"`
	CommitId string `json:"commit_id,omitempty"`
	ErrorMsg string `json:"error_msg,omitempty"`
	OnTopOf  string `json:"on_top_of,omitempty"`
}

type TestingBranch struct {
	Name     string `json:"name,omitempty"`
	CommitId string `json:"commit_id,omitempty"`
	ErrorMsg string `json:"error_msg,omitempty"`
	OnTopOf  string `json:"on_top_of,omitempty"`
}

type Remote struct {
	Name          string         `json:"name,omitempty"`
	Url           string         `json:"url,omitempty"`
	FetchErrorMsg string         `json:"fetch_error_msg,omitempty"`
	Main          *MainBranch    `json:"main,omitempty"`
	Testing       *TestingBranch `json:"testing,omitempty"`
	FetchedAt     time.Time      `json:"fetched_at,omitempty"`
	Fetched       bool           `json:"fetched,omitempty"`
}

type RepositoryStatus struct {
	// This is the deployed Main commit ID. It is used to ensure
	// fast forward
	SelectedCommitId        string    `json:"selected_commit_id"`
	SelectedRemoteName      string    `json:"selected_remote_name"`
	SelectedBranchName      string    `json:"selected_branch_name"`
	SelectedBranchIsTesting bool      `json:"selected_branch_is_testing"`
	MainCommitId            string    `json:"main_commit_id"`
	MainRemoteName          string    `json:"main_remote_name"`
	MainBranchName          string    `json:"main_branch_name"`
	Remotes                 []*Remote `json:"remotes"`
}

func NewRepositoryStatus(config types.GitConfig, repositoryStatus RepositoryStatus) RepositoryStatus {
	r := RepositoryStatus{
		MainCommitId: repositoryStatus.MainCommitId,
	}
	r.Remotes = make([]*Remote, len(config.Remotes))
	for i, remote := range config.Remotes {
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
