package utils

import (
	"context"

	"github.com/nlewo/comin/internal/repository"
)

type RepositoryMock struct {
	RsCh chan repository.RepositoryStatus
}

func NewRepositoryMock() (r *RepositoryMock) {
	rsCh := make(chan repository.RepositoryStatus, 5)
	return &RepositoryMock{
		RsCh: rsCh,
	}
}
func (r *RepositoryMock) FetchAndUpdate(ctx context.Context, remoteNames []string) (rsCh chan repository.RepositoryStatus) {
	return r.RsCh
}
func (r *RepositoryMock) Fetch(remoteNames []string) {
}
func (r *RepositoryMock) Update() (repository.RepositoryStatus, error) {
	return repository.RepositoryStatus{}, nil
}
