package executor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nlewo/comin/internal/profile"
	"github.com/sirupsen/logrus"
)

// GetExpectedMachineId evals
// nixosConfigurations.MACHINE.config.services.comin.machineId and
// returns (machine-id, nil) is comin.machineId is set, ("", nil) otherwise.
func getExpectedMachineId(path, hostname string) (machineId string, err error) {
	expr := fmt.Sprintf("%s#nixosConfigurations.%s.config.services.comin.machineId", path, hostname)
	args := []string{
		"eval",
		expr,
		"--json",
	}
	var stdout bytes.Buffer
	err = runNixCommand(args, &stdout, os.Stderr)
	if err != nil {
		return
	}
	var machineIdPtr *string
	err = json.Unmarshal(stdout.Bytes(), &machineIdPtr)
	if err != nil {
		return
	}
	if machineIdPtr != nil {
		logrus.Debugf("nix: getting comin.machineId = %s", *machineIdPtr)
		machineId = *machineIdPtr
	} else {
		logrus.Debugf("nix: getting comin.machineId = null (not set)")
		machineId = ""
	}
	return
}

func runNixCommand(args []string, stdout, stderr io.Writer) (err error) {
	commonArgs := []string{"--extra-experimental-features", "nix-command", "--extra-experimental-features", "flakes", "--accept-flake-config"}
	args = append(commonArgs, args...)
	cmdStr := fmt.Sprintf("nix %s", strings.Join(args, " "))
	logrus.Infof("nix: running '%s'", cmdStr)
	cmd := exec.Command("nix", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command '%s' fails with %s", cmdStr, err)
	}
	return nil
}

func showDerivation(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, err error) {
	installable := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", flakeUrl, hostname)
	args := []string{
		"derivation",
		"show",
		installable,
		"-L",
		"--show-trace",
	}
	var stdout bytes.Buffer
	err = runNixCommand(args, &stdout, os.Stderr)
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
	logrus.Infof("nix: the derivation path is %s", drvPath)
	logrus.Infof("nix: the output path is %s", outPath)
	return
}

func build(ctx context.Context, drvPath string) (err error) {
	args := []string{
		"build",
		fmt.Sprintf("%s^*", drvPath),
		"-L",
		"--no-link"}
	err = runNixCommand(args, os.Stdout, os.Stderr)
	if err != nil {
		return
	}
	return
}

func cominUnitFileHash() string {
	logrus.Infof("nix: generating the comin.service unit file sha256: 'systemctl cat comin.service | sha256sum'")
	cmd := exec.Command("systemctl", "cat", "comin.service")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Infof("nix: command 'systemctl cat comin.service' fails with '%s'", err)
		return ""
	}
	sum := sha256.Sum256(stdout.Bytes())
	hash := fmt.Sprintf("%x", sum)
	logrus.Infof("nix: the comin.service unit file sha256 is '%s'", hash)
	return hash
}

func switchToConfiguration(operation string, outPath string, dryRun bool) error {
	switchToConfigurationExe := filepath.Join(outPath, "bin", "switch-to-configuration")
	logrus.Infof("nix: running '%s %s'", switchToConfigurationExe, operation)
	cmd := exec.Command(switchToConfigurationExe, operation)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if dryRun {
		logrus.Infof("nix: dry-run enabled: '%s switch' has not been executed", switchToConfigurationExe)
	} else {
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command %s switch fails with %s", switchToConfigurationExe, err)
		}
		logrus.Infof("nix: switch successfully terminated")
	}
	return nil
}

func deploy(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
	// FIXME: this check doesn't have to be here. It should be
	// done by the manager.
	beforeCominUnitFileHash := cominUnitFileHash()

	// This is required to write boot entries
	// Only do this is operation is switch or boot
	if profilePath, err = profile.SetSystemProfile(operation, outPath, false); err != nil {
		return
	}

	if err = switchToConfiguration(operation, outPath, false); err != nil {
		return
	}

	afterCominUnitFileHash := cominUnitFileHash()

	if beforeCominUnitFileHash != afterCominUnitFileHash {
		needToRestartComin = true
	}

	logrus.Infof("nix: deployment ended")

	return
}
