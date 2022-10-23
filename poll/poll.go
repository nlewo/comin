package poll

import (
	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"os/exec"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"
	"path/filepath"
	"github.com/nlewo/comin/types"
	"strings"
)

func Poller(hostname string, stateDir string, repositories []string) {
	s := gocron.NewScheduler(time.UTC)
	period := 1
	config := makeConfig(stateDir, hostname, repositories)
	logrus.Infof("Polling every %d seconds to deploy the machine '%s'", period, hostname)
	job, _ := s.Every(period).Second().Tag("poll").Do(
		func () error {
			return poll(config)
		})
	job.SingletonMode()
	s.StartBlocking()
}

func makeConfig(stateDir string, hostname string, repositories []string) types.Config {
	return types.Config{
		Hostname: hostname,
		StateDir: stateDir,
		GitConfig: types.GitConfig{
			Path: fmt.Sprintf("%s/repository", stateDir),
			Remotes: []types.Remote{
				types.Remote{
					Name: "origin",
					URL: repositories[0],
				},
			},
		},
	}
}

func poll(config types.Config) error {
	logrus.Debugf("Executing poll()")

	updated, err := RepositoryUpdate(config.GitConfig)
	if err != nil {
		logrus.Error(err)
		return err
	}
	if !updated {
		return nil
	}

	err = deploy(config)
	if err != nil {
		logrus.Error(err)
		return err
	}
	return nil
}

func eval(config types.Config) (drvPath string, outPath string, err error) {
	path := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", config.GitConfig.Path, config.Hostname)
	args := []string{
		"show-derivation",
		path,
		"-L",
	}
	logrus.Infof("Running nix %s", args)
	cmd := exec.Command("nix", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return
	}

	var output map[string]Derivation
	err = json.Unmarshal(stdout.Bytes(), &output)
	if err != nil {
		return
	}
	keys := make([]string, 0, len(output))
	for key := range output {
		keys = append(keys, key)
	}
	drvPath = keys[0]
	outPath = output[drvPath].Outputs.Out.Path
	logrus.Infof("Evaluated %s (%s)", drvPath, outPath)
	return
}

type Path struct {
	Path string `json:"path"`
}

type Output struct {
	Out Path `json:"out"`
}

type Derivation struct {
	Outputs Output `json:"outputs"`
}

func deploy(config types.Config) (err error) {
	err = os.MkdirAll(config.StateDir, 0750)
	if err != nil {
		return
	}

	drvPath, _, err := eval(config)

	gcRoot := filepath.Join(
		config.StateDir,
		fmt.Sprintf("switch-to-configuration-%s", config.Hostname))
	args := []string{
		"build",
		drvPath,
		"-L",
		"--out-link", gcRoot}
	logrus.Infof("Running nix %s", strings.Join(args, " "))
	cmd := exec.Command("nix", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		logrus.Errorf("Command nix %s fails with %s", strings.Join(args, " "), err)
		return
	}

	switchToConfiguration := filepath.Join(gcRoot, "bin", "switch-to-configuration")
	logrus.Infof("Running %s switch", switchToConfiguration)
	cmd = exec.Command(switchToConfiguration, "switch")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		logrus.Errorf("Command %s switch fails with %s", switchToConfiguration, err)
		return
	}
	logrus.Infof("Switch successfully terminated")
	return
}
