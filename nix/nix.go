package nix

import (
	"github.com/sirupsen/logrus"
	"os/exec"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"github.com/nlewo/comin/types"
	"strings"
)

func eval(config types.Config) (drvPath string, outPath string, err error) {
	path := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", config.GitConfig.Path, config.Hostname)
	args := []string{
		"show-derivation",
		path,
		"-L",
	}
	logrus.Infof("Running 'nix %s'", strings.Join(args, " "))
	cmd := exec.Command("nix", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return
	}

	var output map[string]Derivation
	err = json.Unmarshal(stdout.Bytes(), &output)
	if err != nil {
		return
	}
	keys := make([]string, 0, len(output))
	for key := range output {
		keys = append(keys, key)
	}
	drvPath = keys[0]
	outPath = output[drvPath].Outputs.Out.Path
	logrus.Infof("The derivation path is %s", drvPath)
	logrus.Infof("The output path is %s", outPath)
	return
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

func Deploy(config types.Config, operation string) (err error) {
	err = os.MkdirAll(config.StateDir, 0750)
	if err != nil {
		return
	}

	drvPath, outPath, err := eval(config)

	args := []string{
		"build",
		drvPath,
		"-L",
		"--no-link"}
	logrus.Infof("Running 'nix %s'", strings.Join(args, " "))
	cmd := exec.Command("nix", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		logrus.Errorf("Command nix %s fails with %s", strings.Join(args, " "), err)
		return
	}

	switchToConfiguration := filepath.Join(outPath, "bin", "switch-to-configuration")
	logrus.Infof("Running '%s %s'", switchToConfiguration, operation)
	cmd = exec.Command(switchToConfiguration, operation)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if config.DryRun {
		logrus.Infof("Dry-run enabled: '%s switch' has not been executed", switchToConfiguration)
	} else {
		err = cmd.Run()
		if err != nil {
			logrus.Errorf("Command %s switch fails with %s", switchToConfiguration, err)
			return
		}
		logrus.Infof("Switch successfully terminated")

		gcRootDir := filepath.Join(config.StateDir, "gcroots")
		err = os.MkdirAll(gcRootDir, 0750)
		if err != nil {
			return
		}
		gcRoot := filepath.Join(
			gcRootDir,
			fmt.Sprintf("switch-to-configuration-%s", config.Hostname))
		// TODO: only remove if file already exists
		os.Remove(gcRoot)
		err = os.Symlink(outPath, gcRoot)
		if err != nil {
			logrus.Errorf("Failed to create symlink 'ln -s %s %s': %s", outPath, gcRoot, err)
			return
		}
		logrus.Infof("Creating gcroot '%s'", gcRoot)
	}
	return
}
