package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
)

const (
	systemManagerProfileDir  = "/nix/var/nix/profiles/system-manager-profiles"
	systemManagerProfileName = "system-manager"
)

type SystemManagerFlake struct{}

func NewSystemManagerFlakeExecutor() (*SystemManagerFlake, error) {
	return &SystemManagerFlake{}, nil
}

func (s *SystemManagerFlake) ReadMachineId() (string, error) {
	return utils.ReadMachineIdLinux()
}

func (s *SystemManagerFlake) NeedToReboot(outPath, operation string) bool {
	return false
}

func (s *SystemManagerFlake) IsStorePathExist(storePath string) bool {
	return isStorePathExist(storePath)
}

func (s *SystemManagerFlake) Eval(ctx context.Context, repositoryPath, repositorySubdir, commitId, systemAttr, hostname string, submodules bool) (drvPath string, outPath string, machineId string, err error) {
	flakeUrl := fmt.Sprintf("git+file://%s?dir=%s&rev=%s", repositoryPath, repositorySubdir, commitId)
	if submodules {
		flakeUrl += "&submodules=1"
	}
	drvPath, outPath, err = showDerivationSystemManager(ctx, flakeUrl, hostname)
	if err != nil {
		return
	}
	// system-manager does not have a services.comin module, so machineId is not available
	machineId = ""
	return
}

func (s *SystemManagerFlake) Build(ctx context.Context, drvPath string) (err error) {
	return buildWithFlake(ctx, drvPath)
}

func (s *SystemManagerFlake) Deploy(ctx context.Context, outPath, operation string, profilePaths []string) (needToRestartComin bool, profilePath string, err error) {
	return deploySystemManager(ctx, outPath, operation)
}

func showDerivationSystemManager(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, err error) {
	// system-manager's makeSystemConfig returns the toplevel derivation directly,
	// so we evaluate systemConfigs."<hostname>" without any .config.system.build.toplevel suffix
	installable := fmt.Sprintf("%s#systemConfigs.\"%s\"", flakeUrl, hostname)
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
	return parseDerivationWithFlake(stdout)
}

func deploySystemManager(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
	beforeCominUnitFileHash := cominUnitFileHashLinux()

	enginePath := filepath.Join(outPath, "bin", "system-manager-engine")

	dryRun := operation == "dry-run"
	if dryRun {
		logrus.Infof("system-manager: dry-run enabled: register and activate have not been executed")
		return
	}

	// Register: creates nix profile generation (skip for "test" operation)
	if operation != "test" {
		logrus.Infof("system-manager: running '%s register --store-path %s'", enginePath, outPath)
		cmd := exec.CommandContext(ctx, enginePath, "register", "--store-path", outPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			err = fmt.Errorf("system-manager register failed: %w", err)
			return
		}
		logrus.Infof("system-manager: register succeeded")
	}

	// Activate: apply etc files and start services
	logrus.Infof("system-manager: running '%s activate --store-path %s'", enginePath, outPath)
	cmd := exec.CommandContext(ctx, enginePath, "activate", "--store-path", outPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		err = fmt.Errorf("system-manager activate failed: %w", err)
		return
	}
	logrus.Infof("system-manager: activate succeeded")

	// Read the profile path for GC root tracking
	profileLink := filepath.Join(systemManagerProfileDir, systemManagerProfileName)
	if dst, readErr := os.Readlink(profileLink); readErr == nil {
		profilePath = filepath.Join(systemManagerProfileDir, dst)
		logrus.Infof("system-manager: current profile path is %s", profilePath)
	} else {
		logrus.Warnf("system-manager: could not read profile symlink %s: %s", profileLink, readErr)
	}

	afterCominUnitFileHash := cominUnitFileHashLinux()
	if beforeCominUnitFileHash != afterCominUnitFileHash {
		needToRestartComin = true
	}

	logrus.Infof("system-manager: deployment ended")
	return
}
