package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

// NeedToReboot return true when the current deployed kernel is not
// the booted kernel. Note we should implement something smarter such
// as described in
// https://discourse.nixos.org/t/nixos-needsreboot-determine-if-you-need-to-reboot-your-nixos-machine/40790
func NeedToRebootLinux() (reboot bool) {
	current, err := os.Readlink("/run/current-system/kernel")
	if err != nil {
		logrus.Infof("nix: could not know if a reboot is required since it failed to read the symlink /run/current-system/kernel: %s", err)
		return
	}
	booted, err := os.Readlink("/run/booted-system/kernel")
	if err != nil {
		logrus.Infof("nix: could not know if a reboot is required since it failed to read the symlink /run/booted-system/kernel: %s", err)
		return
	}
	if current != booted {
		reboot = true
	}
	return
}
