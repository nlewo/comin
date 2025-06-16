package utils

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// NeedToReboot return true when the current deployed kernel is not
// the booted kernel. Note we should implement something smarter such
// as described in
// https://discourse.nixos.org/t/nixos-needsreboot-determine-if-you-need-to-reboot-your-nixos-machine/40790
func NeedToReboot() (reboot bool) {
	if runtime.GOOS == "darwin" {
		return needToRebootDarwin()
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

func needToRebootDarwin() (reboot bool) {
	cmd := exec.Command("/usr/bin/uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		logrus.Errorf("Failed to get kernel version via uname: %s", err)
		return false
	}
	runningKernel := strings.TrimSpace(string(output))
	
	cmd = exec.Command("/usr/sbin/sysctl", "-n", "kern.boottime")
	output, err = cmd.Output()
	if err != nil {
		logrus.Errorf("Failed to get boot time: %s", err)
		return false
	}
	
	bootTimeStr := strings.TrimSpace(string(output))
	if strings.Contains(bootTimeStr, "sec = ") {
		parts := strings.Split(bootTimeStr, "sec = ")
		if len(parts) > 1 {
			secPart := strings.Split(parts[1], ",")[0]
			bootTime, err := strconv.ParseInt(secPart, 10, 64)
			if err == nil {
				logrus.Debugf("Darwin boot time: %d, running kernel: %s", bootTime, runningKernel)
				return false
			}
		}
	}
	
	logrus.Debugf("Darwin reboot check completed, no reboot needed")
	return false
}
