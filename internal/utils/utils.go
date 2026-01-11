package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

func FormatCommitMsg(msg string) string {
	split := strings.Split(msg, "\n")
	formatted := ""
	for i, s := range split {
		if i == len(split)-1 && s == "" {
			continue
		}
		if i == 0 {
			formatted += s
		} else {
			formatted += "\n    " + s
		}
	}
	return formatted
}

func GetBootedAndCurrentStorepaths() (booted, current string) {
	booted, err := os.Readlink("/run/booted-system")
	if err != nil {
		logrus.Errorf("utils: can not read the link '/run/booted-system': %s", err)
		return
	}
	current, err = os.Readlink("/run/current-system")
	if err != nil {
		logrus.Errorf("utils: can not read the link '/run/current-system': %s", err)
		return
	}
	return
}

func ReadMachineIdLinux() (machineId string, err error) {
	machineIdBytes, err := os.ReadFile("/etc/machine-id")
	machineId = strings.TrimSuffix(string(machineIdBytes), "\n")
	if err != nil {
		return "", fmt.Errorf("can not read file '/etc/machine-id': %s", err)
	}
	return
}

func ReadMachineIdDarwin() (machineId string, err error) {
	cmd := exec.Command("/usr/sbin/system_profiler", "SPHardwareDataType")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get hardware UUID on macOS: %s", err)
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		if strings.Contains(line, "Hardware UUID:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				machineId = strings.TrimSpace(parts[1])
				return machineId, nil
			}
		}
	}
	return "", fmt.Errorf("could not find Hardware UUID in system_profiler output")
}
