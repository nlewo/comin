package fetcher

import (
	"context"

	"github.com/nlewo/comin/internal/repository"
	"github.com/sirupsen/logrus"
)

type Fetcher struct {
	submitRemotes      chan []string
	RepositoryStatusCh chan repository.RepositoryStatus
	repo               repository.Repository
	IsFetching         bool
}

func NewFetcher(repo repository.Repository) *Fetcher {
	return &Fetcher{
		repo:               repo,
		submitRemotes:      make(chan []string),
		RepositoryStatusCh: make(chan repository.RepositoryStatus),
	}
}

func (f *Fetcher) TriggerFetch(remotes []string) {
	f.submitRemotes <- remotes
}

func (f *Fetcher) Start() {
	logrus.Info("fetcher: starting")
	go func() {
		remotes := make([]string, 0)
		var workerRepositoryStatusCh chan repository.RepositoryStatus
		for {
			select {
			case submittedRemotes := <-f.submitRemotes:
				remotes = union(remotes, submittedRemotes)
			case rs := <-workerRepositoryStatusCh:
				f.IsFetching = false
				f.RepositoryStatusCh <- rs
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
