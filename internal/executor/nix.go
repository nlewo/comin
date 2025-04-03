package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
)

type NixLocal struct{}

func NewNixExecutor() (*NixLocal, error) {
	return &NixLocal{}, nil
}

func (n *NixLocal) ShowDerivation(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, err error) {
	return showDerivation(ctx, flakeUrl, hostname)
}

func (n *NixLocal) Eval(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, machineId string, err error) {
	drvPath, outPath, err = showDerivation(ctx, flakeUrl, hostname)
	if err != nil {
		return
	}
	machineId, err = getExpectedMachineId(flakeUrl, hostname)
	return
}

func (n *NixLocal) Build(ctx context.Context, drvPath string) (err error) {
	return build(ctx, drvPath)
}

func (n *NixLocal) Deploy(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
	return deploy(ctx, outPath, operation)
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
	NixosConfigurations map[string]struct{} `json:"nixosConfigurations"`
}

func (n *NixLocal) List(flakeUrl string) (hosts []string, err error) {
	args := []string{
		"flake",
		"show",
		"--json",
		flakeUrl,
	}
	var stdout bytes.Buffer
	err = runNixCommand(args, &stdout, os.Stderr)
	if err != nil {
		return
	}

	var output Show
	err = json.Unmarshal(stdout.Bytes(), &output)
	if err != nil {
		return
	}
	hosts = make([]string, 0, len(output.NixosConfigurations))
	for key := range output.NixosConfigurations {
		hosts = append(hosts, key)
	}
	return
}
