package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

func CominServiceRestart() error {
	logrus.Infof("The comin.service unit file changed. Comin systemd service is now restarted...")
	logrus.Infof("Restarting the systemd comin.service: 'systemctl restart --no-block comin.service'")
	cmd := exec.Command("systemctl", "restart", "--no-block", "comin.service")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command 'systemctl restart --no-block comin.service' fails with %s", err)
	}
	return nil
}

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
