package repository

import (
	"context"
	"errors"
)

type flakery struct {
	repository *repository
}

func NewFlakery(repository *repository) (*flakery, error) {
	if repository == nil {
		return nil, errors.New("repository is nil")
	}
	return &flakery{
		repository: repository,
	}, nil
}

func (f *flakery) FetchAndUpdate(ctx context.Context, remoteNames []string) (rsCh chan RepositoryStatus) {
	rsc := f.repository.FetchAndUpdate(ctx, remoteNames)
	rsCh = make(chan RepositoryStatus)
	go func() {
		deploymentStatus, err := f.getDeploymentStatus()
		if err != nil {
			// FIXME: log error
			panic(err)
		}

		if deploymentStatus != "BUILDING" {
			rs := <-rsc
			rsCh <- rs
		}
	}()
	return rsCh
}

func (f *flakery) getDeploymentStatus() (string, error) {
	// todo wrap this repository with a flakery repository
	// that will pause when deployment status is building
	deplyomentID = panic("not implemented")
	userToken = panic("not implemented")
	return "BUILDING", nil
}

func WrapRepositoryWithFlakery(r *repository) (*flakery, error) {
	return NewFlakery(r)
}
