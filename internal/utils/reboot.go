package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

// NeedToReboot return true when the current deployed kernel is not
// the booted kernel. Note we should implement something smarter such
// as described in
// https://discourse.nixos.org/t/nixos-needsreboot-determine-if-you-need-to-reboot-your-nixos-machine/40790
func NeedToReboot(configurationAttr string) (reboot bool) {
	if configurationAttr == "darwinConfigurations" {
		// TODO: Implement proper reboot detection for Darwin
		// Unlike NixOS which has /run/current-system vs /run/booted-system paths,
		// Darwin/macOS doesn't have equivalent mechanisms for detecting when
		// a reboot is needed after nix-darwin configuration changes.
		// For now, conservatively assume no reboot is needed.
		return false
	}
	return needToRebootLinux()
}

func needToRebootLinux() (reboot bool) {
	current, err := os.Readlink("/run/current-system/kernel")
	if err != nil {
		logrus.Errorf("Failed to read the symlink /run/current-system/kernel: %s", err)
		return
	}
	booted, err := os.Readlink("/run/booted-system/kernel")
	if err != nil {
		logrus.Errorf("Failed to read the symlink /run/booted-system/kernel: %s", err)
		return
	}
	if current != booted {
		reboot = true
	}
	return
}
