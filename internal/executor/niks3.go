package executor

import (
	"context"
	"fmt"
	"io"

	"github.com/nlewo/comin/internal/utils"
	"github.com/nlewo/comin/pkg/protobuf"
)

type Niks3 struct {
}

func NewNiks3() (*Niks3, error) {
	return &Niks3{}, nil
}

func (n *Niks3) ReadMachineId() (string, error) {
	return utils.ReadMachineIdLinux()
}

func (n *Niks3) IsStorePathExist(storePath string) bool {
	return isStorePathExist(storePath)
}

func (n *Niks3) NeedToReboot(outPath, operation string) bool {
	return utils.NeedToRebootLinux(outPath, operation)
}

func (n *Niks3) Eval(ctx context.Context, source *protobuf.Source, stdout, stderr io.WriteCloser) (drvPath string, outPath string, machineId string, err error) {
	niks3Source := source.GetNiks3()
	if niks3Source == nil {
		return "", "", "", fmt.Errorf("expected Niks3 source, got nil")
	}
	return "", niks3Source.StorePath, "", nil
}

// Build fetches an outPath from the binary caches configured in the Nix deamon.
func (n *Niks3) Build(ctx context.Context, drvPath, outPath string, stdout, stdin io.WriteCloser) (err error) {
	args := []string{
		"-r",
		outPath,
	}
	err = runNixCommand(ctx, "nix-store -r", args, stdout, stdin)
	if err != nil {
		return
	}
	return
}

func (n *Niks3) Deploy(ctx context.Context, outPath, operation string, profilePaths []string, stdout, stderr io.WriteCloser) (needToRestartComin bool, profilePath string, err error) {
	return deployLinux(ctx, outPath, operation, profilePaths, stdout, stderr)
}
