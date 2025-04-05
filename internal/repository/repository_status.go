package repository

import (
	"time"

	deepcopy "github.com/barkimedes/go-deepcopy"
	"github.com/nlewo/comin/internal/types"
)

type MainBranch struct {
	Name      string `json:"name,omitempty"`
	CommitId  string `json:"commit_id,omitempty"`
	CommitMsg string `json:"commit_msg,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
	OnTopOf   string `json:"on_top_of,omitempty"`
}

type TestingBranch struct {
	Name      string `json:"name,omitempty"`
	CommitId  string `json:"commit_id,omitempty"`
	CommitMsg string `json:"commit_msg,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
	OnTopOf   string `json:"on_top_of,omitempty"`
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
	SelectedCommitId        string `json:"selected_commit_id"`
	SelectedCommitMsg       string `json:"selected_commit_msg"`
	SelectedRemoteName      string `json:"selected_remote_name"`
	SelectedBranchName      string `json:"selected_branch_name"`
	SelectedBranchIsTesting bool   `json:"selected_branch_is_testing"`
	SelectedCommitSigned    bool   `json:"selected_commit_signed"`
	SelectedCommitSignedBy  string `json:"selected_commit_signed_by"`
	// True if public keys were available when the commit has been checked out
	SelectedCommitShouldBeSigned bool      `json:"selected_commit_should_be_signed"`
	MainCommitId                 string    `json:"main_commit_id"`
	MainRemoteName               string    `json:"main_remote_name"`
	MainBranchName               string    `json:"main_branch_name"`
	Remotes                      []*Remote `json:"remotes"`
	Error                        error     `json:"-"`
	ErrorMsg                     string    `json:"error_msg"`
}

func NewRepositoryStatus(config types.GitConfig, mainCommitId string) RepositoryStatus {
	r := RepositoryStatus{
		MainCommitId: mainCommitId,
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

func (r RepositoryStatus) GetRemote(remoteName string) *Remote {
	for _, remote := range r.Remotes {
		if remote.Name == remoteName {
			return remote
		}
	}
	return nil
}

func (r RepositoryStatus) Copy() RepositoryStatus {
	rs, err := deepcopy.Anything(r)
	if err != nil {
		return RepositoryStatus{}
	}
	return rs.(RepositoryStatus)
}
