package utils

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
)

func CominServiceRestart() error {
	logrus.Infof("The comin.service unit file changed. Comin systemd service is now restarted...")
	logrus.Infof("Restarting the systemd comin.service: 'systemctl restart --no-block comin.service'")
	cmd := exec.Command("systemctl", "restart", "--no-block", "comin.service")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Command 'systemctl restart --no-block comin.service' fails with %s", err)
	}
	return nil
}
