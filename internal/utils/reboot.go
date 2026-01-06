package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

// NeedToReboot return true when the current deployed kernel is not
// the booted kernel. Note we should implement something smarter such
// as described in
// https://discourse.nixos.org/t/nixos-needsreboot-determine-if-you-need-to-reboot-your-nixos-machine/40790

// switch-to-configuration test: it updates the /run/current-system link
// switch-to-configuration boot: it doesn't update the /run/current-system link
// switch-to-configuration switch: it updates the /run/current-system link

// Be careful since the reboot information could not be correct with
// the sequence: boot -> test. This could lead to error because if the
// user reboot, it rollbacks to the boot deployment. For now, we
// accept this tradeoff since this is a corner case.
func NeedToRebootLinux(outPath, operation string) (reboot bool) {
	if operation == "boot" {
		currentSystem, err := os.Readlink("/run/current-system")
		if err != nil {
			logrus.Infof("nix: could not know if a reboot is required since it failed to read the symlink /run/current-system: %s", err)
			return
		}
		if outPath != currentSystem {
			return true
		}
	} else {
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
	}
	return
}
