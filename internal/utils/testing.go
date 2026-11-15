package utils

import (
	"context"

	"github.com/nlewo/comin/pkg/protobuf"
)

type RepositoryMock struct {
	RsCh chan *protobuf.GitRepositoryStatus
}

func NewRepositoryMock() (r *RepositoryMock) {
	rsCh := make(chan *protobuf.GitRepositoryStatus, 5)
	return &RepositoryMock{
		RsCh: rsCh,
	}
}
func (r *RepositoryMock) FetchAndUpdate(ctx context.Context, remoteNames []string) (rsCh chan *protobuf.GitRepositoryStatus) {
	return r.RsCh
}
func (r *RepositoryMock) GetRepositoryStatus() *protobuf.GitRepositoryStatus {
	return &protobuf.GitRepositoryStatus{}
}
