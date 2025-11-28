package repository

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/nlewo/comin/internal/prometheus"
	pb "github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/types"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type repository struct {
	Repository       *git.Repository
	GitConfig        types.GitConfig
	RepositoryStatus *pb.RepositoryStatus
	prometheus       prometheus.Prometheus
	gpgPubliKeys     []string
}

type Repository interface {
	FetchAndUpdate(ctx context.Context, remoteNames []string) (rsCh chan *pb.RepositoryStatus)
	// GetRepositoryStatus is currently not thread safe and is only used to initialize the fetcher
	GetRepositoryStatus() *pb.RepositoryStatus
}

// repositoryStatus is the last saved repositoryStatus
func New(config types.GitConfig, mainCommitId string, prometheus prometheus.Prometheus) (r *repository, err error) {
	gpgPublicKeys := make([]string, len(config.GpgPublicKeyPaths))
	for i, path := range config.GpgPublicKeyPaths {
		k, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open the GPG public key file %s: %w", path, err)
		}
		_, err = openpgp.ReadArmoredKeyRing(bytes.NewReader(k))
		if err != nil {
			return nil, fmt.Errorf("failed to read the GPG public key %s: %w", path, err)
		}
		gpgPublicKeys[i] = string(k)
	}

	r = &repository{
		prometheus:   prometheus,
		gpgPubliKeys: gpgPublicKeys,
	}

	r.GitConfig = config
	r.Repository, err = repositoryOpen(config)
	if err != nil {
		return
	}
	err = manageRemotes(r.Repository, config.Remotes)
	if err != nil {
		return
	}
	r.RepositoryStatus = NewRepositoryStatus(config, mainCommitId)

	return
}

func (r *repository) GetRepositoryStatus() *pb.RepositoryStatus {
	return proto.CloneOf(r.RepositoryStatus)
}

func (r *repository) FetchAndUpdate(ctx context.Context, remoteNames []string) (rsCh chan *pb.RepositoryStatus) {
	rsCh = make(chan *pb.RepositoryStatus)
	go func() {
		// FIXME: switch to the FetchContext to clean resource up on timeout
		r.Fetch(remoteNames)
		_ = r.Update()
		rsCh <- proto.CloneOf(r.RepositoryStatus)
	}()
	return rsCh
}

func (r *repository) Fetch(remoteNames []string) {
	var err error
	var status string
	r.RepositoryStatus.ErrorMsg = ""
	logrus.Debugf("repository: fetching %s", remoteNames)
	for _, remote := range r.GitConfig.Remotes {
		repositoryStatusRemote := GetRemote(r.RepositoryStatus, remote.Name)
		if !slices.Contains(remoteNames, remote.Name) {
			continue
		}
		if err = fetch(*r, remote); err != nil {
			repositoryStatusRemote.FetchErrorMsg = err.Error()
			status = "failed"
		} else {
			repositoryStatusRemote.FetchErrorMsg = ""
			repositoryStatusRemote.Fetched = wrapperspb.Bool(true)
			status = "succeeded"
		}
		repositoryStatusRemote.FetchedAt = timestamppb.New(time.Now().UTC())
		r.prometheus.IncFetchCounter(remote.Name, status)
	}
}

func (r *repository) Update() error {
	selectedCommitId := ""

	// We first walk on all Main branches in order to get a commit
	// from a Main branch. Once found, we could then walk on all
	// Testing branches to get a testing commit on top of the Main
	// commit.
	for _, remote := range r.RepositoryStatus.Remotes {
		// If an fetch error occured, we skip this remote
		if remote.FetchErrorMsg != "" {
			logrus.Debugf(
				"The remote %s is  skipped because of the fetch error: %s",
				remote.Name,
				remote.FetchErrorMsg)
			continue
		}
		head, msg, err := getHeadFromRemoteAndBranch(
			*r,
			remote.Name,
			remote.Main.Name,
			r.RepositoryStatus.MainCommitId)
		if err != nil {
			remote.Main.ErrorMsg = err.Error()
			logrus.Debugf("Failed to getHeadFromRemoteAndBranch: %s", err)
			continue
		} else {
			remote.Main.ErrorMsg = ""
		}

		remote.Main.CommitId = head.String()
		remote.Main.CommitMsg = msg
		remote.Main.OnTopOf = r.RepositoryStatus.MainCommitId

		if selectedCommitId == "" {
			selectedCommitId = head.String()
			r.RepositoryStatus.SelectedCommitMsg = msg
			r.RepositoryStatus.SelectedBranchName = remote.Main.Name
			r.RepositoryStatus.SelectedRemoteName = remote.Name
			r.RepositoryStatus.SelectedBranchIsTesting = wrapperspb.Bool(false)
		}
		if head.String() != r.RepositoryStatus.MainCommitId {
			selectedCommitId = head.String()
			r.RepositoryStatus.SelectedCommitMsg = msg
			r.RepositoryStatus.SelectedBranchName = remote.Main.Name
			r.RepositoryStatus.SelectedBranchIsTesting = wrapperspb.Bool(false)
			r.RepositoryStatus.SelectedRemoteName = remote.Name
			r.RepositoryStatus.MainCommitId = head.String()
			r.RepositoryStatus.MainBranchName = remote.Main.Name
			r.RepositoryStatus.MainRemoteName = remote.Name
			break
		}
	}

	for _, remote := range r.RepositoryStatus.Remotes {
		// If an fetch error occured, we skip this remote
		if remote.FetchErrorMsg != "" {
			logrus.Debugf(
				"The remote %s is  skipped because of the fetch error: %s",
				remote.Name,
				remote.FetchErrorMsg)
			continue
		}
		if remote.Testing.Name == "" {
			continue
		}

		head, msg, err := getHeadFromRemoteAndBranch(
			*r,
			remote.Name,
			remote.Testing.Name,
			r.RepositoryStatus.MainCommitId)
		if err != nil {
			remote.Testing.ErrorMsg = err.Error()
			logrus.Debugf("Failed to getHeadFromRemoteAndBranch: %s", err)
			continue
		} else {
			remote.Testing.ErrorMsg = ""
		}

		remote.Testing.CommitId = head.String()
		remote.Testing.CommitMsg = msg
		remote.Testing.OnTopOf = r.RepositoryStatus.MainCommitId

		if head.String() != selectedCommitId && head.String() != r.RepositoryStatus.MainCommitId {
			selectedCommitId = head.String()
			r.RepositoryStatus.SelectedCommitMsg = msg
			r.RepositoryStatus.SelectedBranchName = remote.Testing.Name
			r.RepositoryStatus.SelectedBranchIsTesting = wrapperspb.Bool(true)
			r.RepositoryStatus.SelectedRemoteName = remote.Name
			break
		}
	}

	if selectedCommitId != "" {
		r.RepositoryStatus.SelectedCommitId = selectedCommitId
	}

	if err := hardReset(*r, plumbing.NewHash(selectedCommitId)); err != nil {
		r.RepositoryStatus.ErrorMsg = err.Error()
		return err
	}

	if len(r.gpgPubliKeys) > 0 {
		r.RepositoryStatus.SelectedCommitShouldBeSigned = wrapperspb.Bool(true)
		signedBy, err := headSignedBy(r.Repository, r.gpgPubliKeys)
		if err != nil {
			r.RepositoryStatus.ErrorMsg = err.Error()
		}
		if signedBy == nil {
			r.RepositoryStatus.SelectedCommitSigned = wrapperspb.Bool(false)
			r.RepositoryStatus.SelectedCommitSignedBy = ""
		} else {
			r.RepositoryStatus.SelectedCommitSigned = wrapperspb.Bool(true)
			r.RepositoryStatus.SelectedCommitSignedBy = signedBy.PrimaryIdentity().Name
		}
	} else {
		r.RepositoryStatus.SelectedCommitShouldBeSigned = wrapperspb.Bool(false)
	}
	return nil
}
