package fetcher

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/repository"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Fetcher struct {
	isFetching         atomic.Bool
	repositoryStatus   *protobuf.RepositoryStatus
	mu                 sync.RWMutex
	submitRemotes      chan []string
	RepositoryStatusCh chan *protobuf.RepositoryStatus
	repo               repository.Repository
}

func NewFetcher(repo repository.Repository) *Fetcher {
	f := &Fetcher{
		repo:               repo,
		submitRemotes:      make(chan []string),
		RepositoryStatusCh: make(chan *protobuf.RepositoryStatus),
	}
	f.repositoryStatus = repo.GetRepositoryStatus()
	return f

}

func (f *Fetcher) IsFetching() bool {
	return f.isFetching.Load()
}

func (f *Fetcher) TriggerFetch(remotes []string) {
	f.submitRemotes <- remotes
}

type RemoteState struct {
	Name      string    `json:"name"`
	FetchedAt time.Time `json:"fetched_at"`
}

type State struct {
	IsFetching       bool
	RepositoryStatus *protobuf.RepositoryStatus
}

func (f *Fetcher) GetState() *protobuf.Fetcher {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return &protobuf.Fetcher{
		IsFetching:       wrapperspb.Bool(f.isFetching.Load()),
		RepositoryStatus: f.repo.GetRepositoryStatus(),
	}
}

func (f *Fetcher) Start() {
	logrus.Info("fetcher: starting")
	go func() {
		remotes := make([]string, 0)
		var workerRepositoryStatusCh chan *protobuf.RepositoryStatus
		for {
			select {
			case submittedRemotes := <-f.submitRemotes:
				logrus.Debugf("fetch: remotes submitted: %s", submittedRemotes)
				remotes = union(remotes, submittedRemotes)
			case rs := <-workerRepositoryStatusCh:
				f.isFetching.Store(false)
				f.mu.Lock()
				if rs.SelectedCommitId != f.repositoryStatus.SelectedCommitId || rs.SelectedBranchIsTesting.GetValue() != f.repositoryStatus.SelectedBranchIsTesting.GetValue() {
					f.repositoryStatus = rs
					f.RepositoryStatusCh <- rs
				}
				f.mu.Unlock()
			}
			if !f.isFetching.Load() && len(remotes) != 0 {
				f.isFetching.Store(true)
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
