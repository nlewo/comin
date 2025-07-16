package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
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
