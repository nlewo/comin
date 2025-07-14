package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
)

type NixLocal struct {
	configurationAttr string
}

func NewNixExecutor(configurationAttr string) (*NixLocal, error) {
	return &NixLocal{configurationAttr: configurationAttr}, nil
}

func (n *NixLocal) ShowDerivation(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, err error) {
	return showDerivation(ctx, flakeUrl, hostname, n.configurationAttr)
}

func (n *NixLocal) Eval(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, machineId string, err error) {
	drvPath, outPath, err = showDerivation(ctx, flakeUrl, hostname, n.configurationAttr)
	if err != nil {
		return
	}
	machineId, err = getExpectedMachineId(ctx, flakeUrl, hostname, n.configurationAttr)
	return
}

func (n *NixLocal) Build(ctx context.Context, drvPath string) (err error) {
	return build(ctx, drvPath)
}

func (n *NixLocal) Deploy(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
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

func (n *NixLocal) List(flakeUrl string) (hosts []string, err error) {
	args := []string{
		"flake",
		"show",
		"--json",
		flakeUrl,
	}
	var stdout bytes.Buffer
	err = runNixCommand(context.Background(), args, &stdout, os.Stderr)
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
