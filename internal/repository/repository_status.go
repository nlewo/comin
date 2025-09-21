package repository

import (
	pb "github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/types"
)

func NewRepositoryStatus(config types.GitConfig, mainCommitId string) *pb.RepositoryStatus {
	r := &pb.RepositoryStatus{
		MainCommitId: mainCommitId,
	}
	r.Remotes = make([]*pb.Remote, len(config.Remotes))
	for i, remote := range config.Remotes {
		r.Remotes[i] = &pb.Remote{
			Name: remote.Name,

			Url: remote.URL,
			Main: &pb.Branch{
				Name: remote.Branches.Main.Name,
			},
			Testing: &pb.Branch{
				Name: remote.Branches.Testing.Name,
			},
		}
	}
	return r
}

// func (r RepositoryStatus) IsTesting() bool {
// 	return r.SelectedBranchIsTesting
// }

func GetRemote(r *pb.RepositoryStatus, remoteName string) *pb.Remote {
	for _, remote := range r.Remotes {
		if remote.Name == remoteName {
			return remote
		}
	}
	return nil
}
