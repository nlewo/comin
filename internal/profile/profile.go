package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/sirupsen/logrus"
)

// setSystemProfile creates a link into the directory
// /nix/var/nix/profiles/system-profiles/comin to the built system
// store path. This is used by the switch-to-configuration script to
// install all entries into the bootloader.
// Note also comin uses these links as gcroots
// See https://github.com/nixos/nixpkgs/blob/df98ab81f908bed57c443a58ec5230f7f7de9bd3/pkgs/os-specific/linux/nixos-rebuild/nixos-rebuild.sh#L711
// and https://github.com/nixos/nixpkgs/blob/df98ab81f908bed57c443a58ec5230f7f7de9bd3/nixos/modules/system/boot/loader/systemd-boot/systemd-boot-builder.py#L247
func SetSystemProfile(operation string, outPath string, dryRun bool) (profilePath string, err error) {
	cominProfileDir := "/nix/var/nix/profiles/system-profiles/comin"

	if operation == "switch" || operation == "boot" {
		err := os.MkdirAll("/nix/var/nix/profiles/system-profiles", os.ModeDir)
		if err != nil && !os.IsExist(err) {
			return profilePath, fmt.Errorf("nix: failed to create the profile directory: %s", "/nix/var/nix/profiles/system-profiles")
		}
		cmdStr := fmt.Sprintf("nix-env --profile %s --set %s", cominProfileDir, outPath)
		logrus.Infof("nix: running '%s'", cmdStr)
		cmd := exec.Command("nix-env", "--profile", cominProfileDir, "--set", outPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if dryRun {
			logrus.Infof("nix: dry-run enabled: '%s' has not been executed", cmdStr)
		} else {
			err := cmd.Run()
			if err != nil {
				return profilePath, fmt.Errorf("nix: command '%s' fails with %s", cmdStr, err)
			}
			logrus.Infof("nix: command '%s' succeeded", cmdStr)
			dst, err := os.Readlink(cominProfileDir)
			if err != nil {
				return profilePath, fmt.Errorf("nix: failed to os.Readlink(%s)", cominProfileDir)
			}
			profilePath = path.Join("/nix/var/nix/profiles/system-profiles", dst)
			logrus.Infof("nix: the profile %s has been created", profilePath)
		}
	}
	return
}

// RemoveProfilePath removes a profile path.
func RemoveProfilePath(profilePath string) (err error) {
	logrus.Infof("Removing profile path %s", profilePath)
	err = os.Remove(profilePath)
	if err != nil {
		logrus.Errorf("Failed to remove profile path %s: %s", profilePath, err)
	}
	return
}
