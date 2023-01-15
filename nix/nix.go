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

func eval(path, hostname string) (drvPath string, outPath string, err error) {
	installable := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", path, hostname)
	args := []string{
		"show-derivation",
		installable,
		"-L",
	}
	cmdStr := fmt.Sprintf("nix %s", strings.Join(args, " "))
	logrus.Infof("Running '%s'", cmdStr)
	cmd := exec.Command("nix", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return "", "", fmt.Errorf("Command '%s' fails with %s", cmdStr, err)
	}
	logrus.Infof("After '%s'", cmdStr)

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

type Show struct {
	NixosConfigurations map[string]struct{} `json:"nixosConfigurations"`
}

func List() (hosts []string, err error) {
	args := []string{
		"flake",
		"show",
		"--json"}
	logrus.Infof("Running 'nix %s'", strings.Join(args, " "))
	cmd := exec.Command("nix", args...)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return hosts, fmt.Errorf("Command nix %s fails with %s", strings.Join(args, " "), err)
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

func Build(path, hostname string) (outPath string, err error) {
	drvPath, outPath, err := eval(path, hostname)
	if err != nil {
		return
	}

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
		return outPath, fmt.Errorf("Command nix %s fails with %s", strings.Join(args, " "), err)
	}
	return
}

func Deploy(config types.Config, path, operation string) (err error) {
	err = os.MkdirAll(config.StateDir, 0750)
	if err != nil {
		return
	}

	outPath, err := Build(path, config.Hostname)
	if err != nil {
		return
	}

	if operation == "switch" || operation == "boot" {
		cmdStr := fmt.Sprintf("nix-env --profile /nix/var/nix/profiles/system --set %s", outPath)
		logrus.Infof("Running '%s'", cmdStr)
		cmd := exec.Command("nix-env", "--profile", "/nix/var/nix/profiles/system", "--set", outPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if config.DryRun {
			logrus.Infof("Dry-run enabled: '%s' has not been executed", cmdStr)
		} else {
			err = cmd.Run()
			if err != nil {
				return fmt.Errorf("Command '%s' fails with %s", cmdStr, err)
			}
			logrus.Infof("Command '%s' succeeded", cmdStr)
		}
	}

	switchToConfiguration := filepath.Join(outPath, "bin", "switch-to-configuration")
	logrus.Infof("Running '%s %s'", switchToConfiguration, operation)
	cmd := exec.Command(switchToConfiguration, operation)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if config.DryRun {
		logrus.Infof("Dry-run enabled: '%s switch' has not been executed", switchToConfiguration)
	} else {
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("Command %s switch fails with %s", switchToConfiguration, err)
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
			return fmt.Errorf("Failed to create symlink 'ln -s %s %s': %s", outPath, gcRoot, err)
		}
		logrus.Infof("Creating gcroot '%s'", gcRoot)
	}
	return
}
