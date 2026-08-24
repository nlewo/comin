package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/nlewo/comin/internal/utils"
	"github.com/nlewo/comin/pkg/protobuf"
)

type GitNixFlake struct {
	systemAttr     string
	repositoryPath string
	submodules     bool
}

func NewGitNixFlake(systemAttr, repositoryPath string, submodules bool) (*GitNixFlake, error) {
	return &GitNixFlake{
		systemAttr:     systemAttr,
		repositoryPath: repositoryPath,
		submodules:     submodules,
	}, nil
}

func (n *GitNixFlake) ReadMachineId() (string, error) {
	if n.systemAttr == "darwinConfigurations" {
		return utils.ReadMachineIdDarwin()
	}
	return utils.ReadMachineIdLinux()
}

func (n *GitNixFlake) NeedToReboot(outPath, operation string) bool {
	if n.systemAttr == "darwinConfigurations" {
		// TODO: Implement proper reboot detection for Darwin
		// Unlike NixOS which has /run/current-system vs /run/booted-system paths,
		// Darwin/macOS doesn't have equivalent mechanisms for detecting when
		// a reboot is needed after nix-darwin configuration changes.
		// For now, conservatively assume no reboot is needed.
		return false
	}
	return utils.NeedToRebootLinux(outPath, operation)
}

func (n *GitNixFlake) IsStorePathExist(storePath string) bool {
	return isStorePathExist(storePath)
}

func (n *GitNixFlake) ShowDerivation(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, err error) {
	return showDerivationWithFlake(ctx, flakeUrl, hostname, n.systemAttr, os.Stdout, os.Stderr)
}

func (n *GitNixFlake) Eval(ctx context.Context, source *protobuf.Source, stdout, stderr io.WriteCloser) (drvPath string, outPath string, machineId string, err error) {
	gitSource := source.GetGit()
	if gitSource == nil {
		return "", "", "", fmt.Errorf("expected Git source, got nil")
	}
	flakeUrl := fmt.Sprintf("git+file://%s?dir=%s&rev=%s", n.repositoryPath, gitSource.RepositorySubdir, gitSource.SelectedCommitId)
	if n.submodules {
		flakeUrl += "&submodules=1"
	}
	drvPath, outPath, err = showDerivationWithFlake(ctx, flakeUrl, gitSource.Hostname, n.systemAttr, stdout, stderr)
	if err != nil {
		return
	}
	machineId, err = getExpectedMachineId(ctx, flakeUrl, gitSource.Hostname, n.systemAttr, stdout, stderr)
	return
}

func (n *GitNixFlake) Build(ctx context.Context, drvPath, outPath string, stdout, stdin io.WriteCloser) (err error) {
	return buildWithFlake(ctx, drvPath, outPath, stdout, stdin)
}

func (n *GitNixFlake) Deploy(ctx context.Context, outPath, operation string, profilePaths []string, stdout, stderr io.WriteCloser) (needToRestartComin bool, profilePath string, err error) {
	return deploy(ctx, outPath, operation, n.systemAttr, profilePaths, stdout, stderr)
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

type DerivationOutput struct {
	Version     int                   `json:"version"`
	Derivations map[string]Derivation `json:"derivations"`
}

type Show struct {
	NixosConfigurations  map[string]struct{} `json:"nixosConfigurations"`
	DarwinConfigurations map[string]struct{} `json:"darwinConfigurations"`
}

func (n *GitNixFlake) List(flakeUrl string) (hosts []string, err error) {
	args := []string{
		"flake",
		"show",
		"--json",
		flakeUrl,
	}
	var stdout bytes.Buffer
	err = runNixFlakeCommand(context.Background(), args, &NopWriteCloser{Writer: &stdout}, &NopWriteCloser{Writer: os.Stderr})
	if err != nil {
		return
	}

	var output Show
	err = json.Unmarshal(stdout.Bytes(), &output)
	if err != nil {
		return
	}

	var configurations map[string]struct{}
	if n.systemAttr == "darwinConfigurations" {
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
