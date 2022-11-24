package poll

import (
	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"os"
	"fmt"
	"time"
	"github.com/nlewo/comin/types"
	"github.com/nlewo/comin/nix"
	cominGit "github.com/nlewo/comin/git"
	"github.com/go-git/go-git/v5"
	"io/ioutil"
	"encoding/json"
)

func Poller(hostname string, stateDir string, dryRun bool, repositories []string) error {
	s := gocron.NewScheduler(time.UTC)
	period := 1
	config := makeConfig(stateDir, hostname, dryRun, repositories)
	repository, err := cominGit.RepositoryOpen(config.GitConfig)
	if err != nil {
		return fmt.Errorf("Failed to open the repository: %s", err)
	}
	logrus.Infof("Polling every %d seconds to deploy the machine '%s'", period, hostname)
	job, _ := s.Every(period).Second().Tag("poll").Do(
		func () error {
			err := poll(repository, config)
			if err != nil {
				logrus.Error(err)
			}
			return err
		})
	job.SingletonMode()
	s.StartBlocking()
	return nil
}

func makeConfig(stateDir string, hostname string, dryRun bool, repositories []string) types.Config {
	return types.Config{
		Hostname: hostname,
		StateDir: stateDir,
		StateFile: fmt.Sprintf("%s/state.json", stateDir),
		GitConfig: types.GitConfig{
			Path: fmt.Sprintf("%s/repository", stateDir),
			Remote: types.Remote{
				Name: "origin",
				URL: repositories[0],
			},
			Main: "master",
			Testing: "testing",
		},
		DryRun: dryRun,
	}
}

func poll(repository *git.Repository, config types.Config) error {
	logrus.Debugf("Executing a poll iteration")

	hasNewCommits, isTesting, err := cominGit.RepositoryUpdate(repository, config.GitConfig)
	if err != nil {
		return err
	}
	operation := "switch"
	if isTesting {
		operation = "test"
	}

	var state types.State
        if _, err := os.Stat(config.StateFile); err == nil {
		logrus.Debugf("Loading state file")
		content, err := ioutil.ReadFile(config.StateFile)
		if err != nil {
			return err
		}
		err = json.Unmarshal(content, &state)
		if err != nil {
			return err
		}
		logrus.Debugf("State is %#v", state)
	}

	if !hasNewCommits && state.LastOperation == operation {
		return nil
	}

	err = nix.Deploy(config, operation)
	if err != nil {
		return err
	}
	state.LastOperation = operation

	res, err := json.MarshalIndent(state, "", "\t")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(config.StateFile, []byte(res), 0644)
	if err != nil {
		return err
	}

	return nil
}
