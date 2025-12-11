package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/nlewo/comin/internal/utils"
)

type NixFlakeLocal struct {
	configurationAttr string
}

func NewNixFlakeExecutor(configurationAttr string) (*NixFlakeLocal, error) {
	return &NixFlakeLocal{configurationAttr: configurationAttr}, nil
}

func (n *NixFlakeLocal) ReadMachineId() (string, error) {
	if n.configurationAttr == "darwinConfigurations" {
		return utils.ReadMachineIdDarwin()
	}
	return utils.ReadMachineIdLinux()
}

func (n *NixFlakeLocal) NeedToReboot() bool {
	if n.configurationAttr == "darwinConfigurations" {
		// TODO: Implement proper reboot detection for Darwin
		// Unlike NixOS which has /run/current-system vs /run/booted-system paths,
		// Darwin/macOS doesn't have equivalent mechanisms for detecting when
		// a reboot is needed after nix-darwin configuration changes.
		// For now, conservatively assume no reboot is needed.
		return false
	}
	return utils.NeedToRebootLinux()
}

func (n *NixFlakeLocal) IsStorePathExist(storePath string) bool {
	if _, err := os.Stat(storePath); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func (n *NixFlakeLocal) ShowDerivation(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, err error) {
	return showDerivationWithFlake(ctx, flakeUrl, hostname, n.configurationAttr)
}

func (n *NixFlakeLocal) Eval(ctx context.Context, repositoryPath, repositorySubdir, commitId, configurationAttr, hostname string) (drvPath string, outPath string, machineId string, err error) {
	flakeUrl := fmt.Sprintf("git+file://%s?dir=%s&rev=%s", repositoryPath, repositorySubdir, commitId)
	drvPath, outPath, err = showDerivationWithFlake(ctx, flakeUrl, hostname, n.configurationAttr)
	if err != nil {
		return
	}
	machineId, err = getExpectedMachineId(ctx, flakeUrl, hostname, n.configurationAttr)
	return
}

func (n *NixFlakeLocal) Build(ctx context.Context, drvPath string) (err error) {
	return buildWithFlake(ctx, drvPath)
}

func (n *NixFlakeLocal) Deploy(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
	return deploy(ctx, outPath, operation, n.configurationAttr)
}

type Path struct {
	Path string `json:"path"`
}

type Output struct {
	Out Path `json:"out"`
}

type Derivation struct {
	Outputs Output `json:"outputs"`
}

type Show struct {
	NixosConfigurations  map[string]struct{} `json:"nixosConfigurations"`
	DarwinConfigurations map[string]struct{} `json:"darwinConfigurations"`
}

func (n *NixFlakeLocal) List(flakeUrl string) (hosts []string, err error) {
	args := []string{
		"flake",
		"show",
		"--json",
		flakeUrl,
	}
	var stdout bytes.Buffer
	err = runNixFlakeCommand(context.Background(), args, &stdout, os.Stderr)
	if err != nil {
		return
	}

	var output Show
	err = json.Unmarshal(stdout.Bytes(), &output)
	if err != nil {
		return
	}

	var configurations map[string]struct{}
	if n.configurationAttr == "darwinConfigurations" {
		configurations = output.DarwinConfigurations
	} else {
		configurations = output.NixosConfigurations
	}

	hosts = make([]string, 0, len(configurations))
	for key := range configurations {
		hosts = append(hosts, key)
	}
	return
}
