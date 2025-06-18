package utils

import (
	"fmt"
	"os"
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

func ReadMachineId() (machineId string, err error) {
	machineIdBytes, err := os.ReadFile("/etc/machine-id")
	machineId = strings.TrimSuffix(string(machineIdBytes), "\n")
	if err != nil {
		return "", fmt.Errorf("can not read file '/etc/machine-id': %s", err)
	}
	return
}
