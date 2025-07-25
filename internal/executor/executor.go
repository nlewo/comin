package executor

import (
	"context"

	"github.com/sirupsen/logrus"
)

// Executor contains the function used by comin to actually do actions
// on the host. This allows us to abstract the way Nix expression are
// evaluated, built and deployed. This could be for instance used by a
// Garnix implementation (such as proposed in
// https://github.com/nlewo/comin/pull/74)
type Executor interface {
	Eval(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, machineId string, err error)
	Build(ctx context.Context, drvPath string) (err error)
	Deploy(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error)
	NeedToReboot() bool
	ReadMachineId() (string, error)
}

func NewNixOS() (e Executor, err error) {
	logrus.Info("executor: creating a NixOS executor")
	e, err = NewNixExecutor("nixosConfigurations")
	return
}
func NewNixDarwin() (e Executor, err error) {
	logrus.Info("executor: creating a nix-darwin executor")
	e, err = NewNixExecutor("darwinConfigurations")
	return
}
