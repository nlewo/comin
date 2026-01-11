package executor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nlewo/comin/internal/profile"
	"github.com/sirupsen/logrus"
)

// GetExpectedMachineId evals nixosConfigurations or darwinConfigurations based on systemAttr
// returns (machine-id, nil) is comin.machineId is set, ("", nil) otherwise.
func getExpectedMachineId(ctx context.Context, path, hostname, systemAttr string) (machineId string, err error) {
	expr := fmt.Sprintf("%s#%s.\"%s\".config.services.comin.machineId", path, systemAttr, hostname)
	args := []string{
		"eval",
		expr,
		"--json",
	}
	var stdout bytes.Buffer
	err = runNixFlakeCommand(ctx, args, &stdout, os.Stderr)
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

func runNixCommand(ctx context.Context, command string, args []string, stdout, stderr io.Writer) (err error) {
	logrus.Infof("nix: running %s %s", command, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command '%s' with args '%s' fails with %s", command, args, err)
	} else {
		logrus.Infof("nix: command '%s' with args '%s' successfully executed", command, args)
	}
	return nil
}

func runNixFlakeCommand(ctx context.Context, args []string, stdout, stderr io.Writer) (err error) {
	commonArgs := []string{"--extra-experimental-features", "flakes nix-command", "--accept-flake-config"}
	args = append(commonArgs, args...)
	cmdStr := fmt.Sprintf("nix %s", strings.Join(args, " "))
	logrus.Infof("nix: running '%s'", cmdStr)
	cmd := exec.CommandContext(ctx, "nix", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command '%s' fails with %s", cmdStr, err)
	} else {
		logrus.Infof("nix: command '%s' successfully executed", cmdStr)
	}
	return nil
}

func showDerivationWithNix(ctx context.Context, directory, systemAttr string) (drvPath, outPath, machineId string, err error) {
	var stdout bytes.Buffer

	// This is to create the .drv file
	toplevel := fmt.Sprintf("%s.toplevel", systemAttr)
	err = runNixCommand(ctx, "nix-instantiate", []string{directory, "-A", toplevel}, &stdout, os.Stderr)
	if err != nil {
		return
	}

	// This is to get some useful values such as the drvPath, outPath and the machineId
	exprTpl := `let imported = import %s; default = if builtins.typeOf imported == "lambda" then imported {} else imported; machineId=if default.%s.config.services.comin.machineId == null then "" else default.%s.config.services.comin.machineId;  in "${default.%s.toplevel.drvPath};${default.%s.toplevel.outPath};${machineId}"`
	expr := fmt.Sprintf(exprTpl, directory, systemAttr, systemAttr, systemAttr, systemAttr)

	// --raw is not supported by Lyx and --json doesn't work with Nix...
	err = runNixCommand(ctx, "nix-instantiate", []string{"--strict", "--eval", "-E", expr}, &stdout, os.Stderr)
	if err != nil {
		return
	}
	logrus.Debugf("nix: output of nix-instantiate: '%s'", stdout.String())
	lines := strings.Split(stdout.String(), "\n")
	if len(lines) < 2 {
		return "", "", "", fmt.Errorf("nix: nix-instantiate should return at least 2 lines")
	}
	sanitized := strings.TrimPrefix(lines[len(lines)-2], `"`)
	sanitized = strings.TrimSuffix(sanitized, `"`)
	elems := strings.Split(sanitized, ";")
	if len(elems) < 2 {
		err = fmt.Errorf("nix: the output of the evalucation Nix command must at least return 2 lines")
	}
	drvPath = elems[0]
	outPath = elems[1]
	if len(elems) >= 3 {
		machineId = elems[2]
	}
	return
}

func showDerivationWithFlake(ctx context.Context, flakeUrl, hostname, systemAttr string) (drvPath string, outPath string, err error) {
	installable := fmt.Sprintf("%s#%s.\"%s\".config.system.build.toplevel", flakeUrl, systemAttr, hostname)
	args := []string{
		"derivation",
		"show",
		installable,
		"-L",
		"--show-trace",
	}
	var stdout bytes.Buffer
	err = runNixFlakeCommand(ctx, args, &stdout, os.Stderr)
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
	drvKey := keys[0]
	outPath = output[drvKey].Outputs.Out.Path
	// derivation jsons do not contain store directory anymore since nix 2.32
	// see https://nix.dev/manual/nix/2.32/release-notes/rl-2.32.html#:~:text=derivation%20json%20format%20now%20uses%20store%20path%20basenames%20only
	if !strings.HasPrefix(drvKey, "/nix/store/") {
		drvPath = "/nix/store/" + drvKey
	} else {
		drvPath = drvKey
	}
	if !strings.HasPrefix(outPath, "/nix/store/") && outPath != "" {
		outPath = "/nix/store/" + outPath
	}
	logrus.Infof("nix: the derivation path is %s", drvPath)
	logrus.Infof("nix: the output path is %s", outPath)
	return
}

func buildWithFlake(ctx context.Context, drvPath string) (err error) {
	args := []string{
		"build",
		fmt.Sprintf("%s^*", drvPath),
		"-L",
		"--no-link"}
	err = runNixFlakeCommand(ctx, args, os.Stdout, os.Stderr)
	if err != nil {
		return
	}
	return
}

func buildWithNix(ctx context.Context, drvPath string) (err error) {
	args := []string{
		"-r",
		drvPath,
	}
	err = runNixCommand(ctx, "nix-store", args, os.Stdout, os.Stderr)
	if err != nil {
		return
	}
	return
}

func cominUnitFileHash(systemAttr string) string {
	if systemAttr == "darwinConfigurations" {
		return cominUnitFileHashDarwin()
	}
	return cominUnitFileHashLinux()
}

func cominUnitFileHashLinux() string {
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

func cominUnitFileHashDarwin() string {
	logrus.Infof("nix: generating the comin service plist file sha256: 'launchctl print system/com.github.nlewo.comin'")
	cmd := exec.Command("/bin/launchctl", "print", "system/com.github.nlewo.comin")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Infof("nix: command 'launchctl print system/com.github.nlewo.comin' fails with '%s'", err)
		return ""
	}
	sum := sha256.Sum256(stdout.Bytes())
	hash := fmt.Sprintf("%x", sum)
	logrus.Infof("nix: the comin service plist sha256 is '%s'", hash)
	return hash
}

func switchToConfiguration(operation string, outPath string, dryRun bool, systemAttr string) error {
	if systemAttr == "darwinConfigurations" {
		return switchToConfigurationDarwin(operation, outPath, dryRun)
	}
	return switchToConfigurationLinux(operation, outPath, dryRun)
}

func switchToConfigurationLinux(operation string, outPath string, dryRun bool) error {
	switchToConfigurationExe := filepath.Join(outPath, "bin", "switch-to-configuration")
	logrus.Infof("nix: running '%s %s'", switchToConfigurationExe, operation)
	cmd := exec.Command(switchToConfigurationExe, operation)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if dryRun {
		logrus.Infof("nix: dry-run enabled: '%s %s' has not been executed", switchToConfigurationExe, operation)
	} else {
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command %s %s fails with %s", switchToConfigurationExe, operation, err)
		}
		logrus.Infof("nix: switch successfully terminated")
	}
	return nil
}

func switchToConfigurationDarwin(operation string, outPath string, dryRun bool) error {
	activateUserExe := filepath.Join(outPath, "activate-user")
	activateExe := filepath.Join(outPath, "activate")

	if dryRun {
		logrus.Infof("nix: dry-run enabled: Darwin activation has not been executed")
		return nil
	}

	logrus.Infof("nix: activating user environment: '%s'", activateUserExe)
	userCmd := exec.Command(activateUserExe)
	userCmd.Stdout = os.Stdout
	userCmd.Stderr = os.Stderr
	if err := userCmd.Run(); err != nil {
		return fmt.Errorf("user activation command %s fails with %s", activateUserExe, err)
	}

	logrus.Infof("nix: activating system environment: '%s'", activateExe)
	cmd := exec.Command(activateExe)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("system activation command %s fails with %s", activateExe, err)
	}

	logrus.Infof("nix: Darwin activation successfully terminated")
	return nil
}

func deploy(ctx context.Context, outPath, operation, systemAttr string) (needToRestartComin bool, profilePath string, err error) {
	if systemAttr == "darwinConfigurations" {
		return deployDarwin(ctx, outPath, operation)
	}
	return deployLinux(ctx, outPath, operation)
}

func deployLinux(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
	// FIXME: this check doesn't have to be here. It should be
	// done by the manager.
	beforeCominUnitFileHash := cominUnitFileHashLinux()

	// This is required to write boot entries
	// Only do this is operation is switch or boot
	if profilePath, err = profile.SetSystemProfile(operation, outPath, false); err != nil {
		return
	}

	if err = switchToConfigurationLinux(operation, outPath, false); err != nil {
		return
	}

	afterCominUnitFileHash := cominUnitFileHashLinux()

	if beforeCominUnitFileHash != afterCominUnitFileHash {
		needToRestartComin = true
	}

	logrus.Infof("nix: deployment ended")

	return
}

func deployDarwin(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
	// FIXME: this check doesn't have to be here. It should be
	// done by the manager.
	beforeCominUnitFileHash := cominUnitFileHashDarwin()

	// This is required to write boot entries
	// Only do this is operation is switch or boot
	if profilePath, err = profile.SetSystemProfile(operation, outPath, false); err != nil {
		return
	}

	if err = switchToConfigurationDarwin(operation, outPath, false); err != nil {
		return
	}

	afterCominUnitFileHash := cominUnitFileHashDarwin()

	if beforeCominUnitFileHash != afterCominUnitFileHash {
		needToRestartComin = true
	}

	logrus.Infof("nix: deployment ended")

	return
}

func isStorePathExist(storePath string) bool {
	if _, err := os.Stat(storePath); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}
