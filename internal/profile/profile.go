package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"

	"github.com/sirupsen/logrus"
)

func getSystemProfilesDir() string {
	if runtime.GOOS == "darwin" {
		return "/nix/var/nix/profiles/system-profiles"
	}
	return "/nix/var/nix/profiles/system-profiles"
}

func getCominProfileDir() string {
	return getSystemProfilesDir() + "/comin"
}

// setSystemProfile creates a link for the built system store path.
// Used by switch-to-configuration and as GC roots.
func SetSystemProfile(operation string, outPath string, dryRun bool) (profilePath string, err error) {
	systemProfilesDir := getSystemProfilesDir()
	cominProfileDir := getCominProfileDir()
	
	if operation == "switch" || operation == "boot" {
		err := os.MkdirAll(systemProfilesDir, os.ModeDir)
		if err != nil && !os.IsExist(err) {
			return profilePath, fmt.Errorf("nix: failed to create the profile directory: %s", systemProfilesDir)
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
			profilePath = path.Join(systemProfilesDir, dst)
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
