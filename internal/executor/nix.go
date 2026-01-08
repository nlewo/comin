package executor

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
)

type NixLocal struct{}

func NewNixExecutor() (*NixLocal, error) {
	return &NixLocal{}, nil
}

func (n *NixLocal) ReadMachineId() (string, error) {
	return utils.ReadMachineIdLinux()
}

func (n *NixLocal) IsStorePathExist(storePath string) bool {
	return isStorePathExist(storePath)
}

func (n *NixLocal) NeedToReboot(outPath, operation string) bool {
	return utils.NeedToRebootLinux(outPath, operation)
}

func (n *NixLocal) Eval(ctx context.Context, repositoryPath, repositorySubdir, commitId, systemAttr, hostname string) (drvPath string, outPath string, machineId string, err error) {
	tempDir, err := cloneRepoToTemp(repositoryPath, commitId)
	defer os.RemoveAll(tempDir) // nolint: errcheck
	if err != nil {
		return
	}
	logrus.Debugf("nix: temporary cloned into %s", tempDir)
	nixDir := path.Join(tempDir, repositorySubdir)
	return showDerivationWithNix(ctx, nixDir, systemAttr)
}

func (n *NixLocal) Build(ctx context.Context, drvPath string) (err error) {
	return buildWithNix(ctx, drvPath)
}

func (n *NixLocal) Deploy(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
	return deployLinux(ctx, outPath, operation)
}

func cloneRepoToTemp(remoteDir string, commitId string) (string, error) {
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

	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL: remoteDir,
	})
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
