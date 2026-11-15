package executor

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
)

type EvalFunc func(ctx context.Context, source *protobuf.Source, stdout, stderr io.WriteCloser) (drvPath string, outPath string, machineId string, err error)
type BuildFunc func(ctx context.Context, drvPath string, stdout, stdin io.WriteCloser) error

func New(repositoryType, repositoryPath string, submodules bool) (e Executor, err error) {
	switch repositoryType {
	case "flake":
		if runtime.GOOS == "darwin" {
			return NewGitNixFlakeDarwin(repositoryPath, submodules)
		} else {
			return NewGitNixFlakeNixOS(repositoryPath, submodules)
		}

	case "nix":
		return NewGitNixNixOS(repositoryPath, submodules)
	}
	return e, fmt.Errorf("failed to create the executor: %s", err)
}

// Executor contains the function used by comin to actually do actions
// on the host. This allows us to abstract the way Nix expression are
// evaluated, built and deployed. This could be for instance used by a
// Garnix implementation (such as proposed in
// https://github.com/nlewo/comin/pull/74)
type Executor interface {
	Eval(ctx context.Context, source *protobuf.Source, stdout, stderr io.WriteCloser) (drvPath string, outPath string, machineId string, err error)
	Build(ctx context.Context, drvPath string, stdout, stdin io.WriteCloser) (err error)
	Deploy(ctx context.Context, outPath, operation string, profilePaths []string, stdout, stderr io.WriteCloser) (needToRestartComin bool, profilePath string, err error)
	NeedToReboot(outPath, operation string) bool
	ReadMachineId() (string, error)
	// IsStorePathExist returns true if a storepath exists. This
	// is used to detect if a build will be required or not.
	IsStorePathExist(string) bool
}

func NewGitNixFlakeNixOS(repositoryPath string, submodules bool) (e Executor, err error) {
	logrus.Info("executor: creating a NixOS flake executor")
	e, err = NewGitNixFlake("nixosConfigurations", repositoryPath, submodules)
	return
}
func NewGitNixFlakeDarwin(repositoryPath string, submodules bool) (e Executor, err error) {
	logrus.Info("executor: creating a nix-darwin flake executor")
	e, err = NewGitNixFlake("darwinConfigurations", repositoryPath, submodules)
	return
}

func NewGitNixNixOS(repositoryPath string, submodules bool) (e Executor, err error) {
	logrus.Info("executor: creating a NixOS executor")
	e, err = NewGitNix(repositoryPath, submodules)
	return
}
