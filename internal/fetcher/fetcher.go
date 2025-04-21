package fetcher

import (
	"context"
	"time"

	"github.com/nlewo/comin/internal/repository"
	"github.com/sirupsen/logrus"
)

type Fetcher struct {
	State
	submitRemotes      chan []string
	RepositoryStatusCh chan repository.RepositoryStatus
	repo               repository.Repository
}

func NewFetcher(repo repository.Repository) *Fetcher {
	f := &Fetcher{
		repo:               repo,
		submitRemotes:      make(chan []string),
		RepositoryStatusCh: make(chan repository.RepositoryStatus),
	}
	f.RepositoryStatus = repo.GetRepositoryStatus()
	return f

}

func (f *Fetcher) TriggerFetch(remotes []string) {
	f.submitRemotes <- remotes
}

type RemoteState struct {
	Name      string    `json:"name"`
	FetchedAt time.Time `json:"fetched_at"`
}
type State struct {
	IsFetching       bool `jsona:"is_fetching"`
	RepositoryStatus repository.RepositoryStatus
}

// FIXME: make it thread safe
func (f *Fetcher) GetState() State {
	return State{
		IsFetching:       f.IsFetching,
		RepositoryStatus: f.RepositoryStatus,
	}
}

func (f *Fetcher) Start() {
	logrus.Info("fetcher: starting")
	go func() {
		remotes := make([]string, 0)
		var workerRepositoryStatusCh chan repository.RepositoryStatus
		for {
			select {
			case submittedRemotes := <-f.submitRemotes:
				logrus.Debugf("fetch: remotes submitted: %s", submittedRemotes)
				remotes = union(remotes, submittedRemotes)
			case rs := <-workerRepositoryStatusCh:
				f.IsFetching = false
				if rs.SelectedCommitId != f.RepositoryStatus.SelectedCommitId || rs.SelectedBranchIsTesting != f.RepositoryStatus.SelectedBranchIsTesting {
					f.RepositoryStatus = rs
					f.RepositoryStatusCh <- rs
				}
			}
			if !f.IsFetching && len(remotes) != 0 {
				f.IsFetching = true
				workerRepositoryStatusCh = f.repo.FetchAndUpdate(context.TODO(), remotes)
				remotes = []string{}
			}
		}
	}()
}

func union(array1, array2 []string) []string {
	for _, e2 := range array2 {
		exist := false
		for _, e1 := range array1 {
			if e2 == e1 {
				exist = true
				break
			}
		}
		if !exist {
			array1 = append(array1, e2)
		}
	}
	return array1
}
