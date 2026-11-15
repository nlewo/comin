package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/nlewo/comin/internal/utils"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
)

type GitNix struct {
	repositoryPath string
	submodules     bool
}

func NewGitNix(repositoryPath string, submodules bool) (*GitNix, error) {
	return &GitNix{
		repositoryPath: repositoryPath,
		submodules:     submodules,
	}, nil
}

func (n *GitNix) ReadMachineId() (string, error) {
	return utils.ReadMachineIdLinux()
}

func (n *GitNix) IsStorePathExist(storePath string) bool {
	return isStorePathExist(storePath)
}

func (n *GitNix) NeedToReboot(outPath, operation string) bool {
	return utils.NeedToRebootLinux(outPath, operation)
}

func (n *GitNix) Eval(ctx context.Context, source *protobuf.Source, stdout, stderr io.WriteCloser) (drvPath string, outPath string, machineId string, err error) {
	gitSource := source.GetGit()
	if gitSource == nil {
		return "", "", "", fmt.Errorf("expected Git source, got nil")
	}
	tempDir, err := cloneRepoToTemp(n.repositoryPath, gitSource.SelectedCommitId, n.submodules)
	defer os.RemoveAll(tempDir) // nolint: errcheck
	if err != nil {
		return
	}
	logrus.Debugf("nix: temporary cloned into %s", tempDir)
	nixDir := path.Join(tempDir, gitSource.RepositorySubdir)
	return showDerivationWithNix(ctx, nixDir, gitSource.SystemAttr, stdout, stderr)
}

func (n *GitNix) Build(ctx context.Context, drvPath string, stdout, stdin io.WriteCloser) (err error) {
	return buildWithNix(ctx, drvPath, stdout, stdin)
}

func (n *GitNix) Deploy(ctx context.Context, outPath, operation string, profilePaths []string, stdout, stderr io.WriteCloser) (needToRestartComin bool, profilePath string, err error) {
	return deployLinux(ctx, outPath, operation, profilePaths, stdout, stderr)
}

func cloneRepoToTemp(remoteDir string, commitId string, submodules bool) (string, error) {
	dir, err := os.MkdirTemp("", "comin-git-clone-*")
	if err != nil {
		return "", err
	}
	logrus.Debugf("nix: cloning %s into temporary directory '%s'", remoteDir, dir)
	remoteRepo, err := git.PlainOpen(remoteDir)
	if err != nil {
		return "", fmt.Errorf("nix: failed to open the repository '%s': %s", remoteDir, err)
	}

	// We need to create this reference otherwise the checkout fails with "object not found".
	branchName := plumbing.NewBranchReferenceName("archive")
	ref := plumbing.NewHashReference(branchName, plumbing.NewHash(commitId))
	err = remoteRepo.Storer.SetReference(ref)
	if err != nil {
		return "", fmt.Errorf("nix: failed to set reference 'archive' to '%s' in %s: %s", commitId, remoteDir, err)
	}
	cloneOpts := &git.CloneOptions{
		URL:           remoteDir,
		NoCheckout:    true,
		ReferenceName: "refs/heads/archive",
		SingleBranch:  true,
	}
	if submodules {
		cloneOpts.RecurseSubmodules = git.DefaultSubmoduleRecursionDepth
	}
	r, err := git.PlainClone(dir, false, cloneOpts)
	if err != nil {
		return "", fmt.Errorf("nix: failed to clone '%s': %s", dir, err)
	}
	w, err := r.Worktree()
	if err != nil {
		return "", err
	}
	err = w.Checkout(&git.CheckoutOptions{
		Hash:  plumbing.NewHash(commitId),
		Force: true,
	})
	if err != nil {
		return "", fmt.Errorf("nix: failed to checkout the commit '%s' in '%s': %s", commitId, dir, err)
	}
	return dir, nil
}
