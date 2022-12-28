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

func Poller(hostname string, stateDir string, authsFilepath string, dryRun bool, repositories []string) error {
	s := gocron.NewScheduler(time.UTC)
	period := 1
	config, err := makeConfig(stateDir, authsFilepath, hostname, dryRun, repositories)
	if err != nil {
		return err
	}
	logrus.Debugf("Config is '%#v'", config)
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

func makeConfig(stateDir string, authsFilepath string, hostname string, dryRun bool, repositories []string) (config types.Config, err error) {
	config = types.Config{
		Hostname: hostname,
		StateDir: stateDir,
		StateFile: fmt.Sprintf("%s/state.json", stateDir),
		GitConfig: types.GitConfig{
			Path: fmt.Sprintf("%s/repository", stateDir),
			Remote: types.Remote{
				Name: "origin",
				// TODO: support multiple repositories
				URL: repositories[0],
			},
			Main: "master",
			Testing: "testing",
		},
		DryRun: dryRun,
	}
	if authsFilepath != "" {
		auths, err := readAuths(authsFilepath)
		if err != nil {
			return config, fmt.Errorf("Failed to read auths file: '%s'", err)
		}
		auth, exists := auths[repositories[0]]
		if (exists) {
			config.GitConfig.Remote.Auth = auth
		}
	}
	return
}

func readAuths(authsFilepath string) (auths types.Auths, err error) {
	var content []byte
        if _, err = os.Stat(authsFilepath); err == nil {
		logrus.Debugf("Loading auths file located at '%s'", authsFilepath)
		content, err = ioutil.ReadFile(authsFilepath)
		if err != nil {
			return
		}
		err = json.Unmarshal(content, &auths)
		if err != nil {
			return
		}
		logrus.Debugf("Auths is %#v", auths)
	}
	return
}

func poll(repository *git.Repository, config types.Config) error {
	logrus.Debugf("Executing a poll iteration")

	commitHash, branch, err := cominGit.RepositoryUpdate(repository, config.GitConfig)
	if err != nil {
		return err
	}
	operation := "switch"
	if branch == config.GitConfig.Testing {
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

	if commitHash.String() == state.CommitId && state.Operation == operation {
		return nil
	}

	err = nix.Deploy(config, operation)
	state.Deployed = err == nil
	state.CommitId = commitHash.String()
	state.Operation = operation

	res, err := json.MarshalIndent(state, "", "\t")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(config.StateFile, []byte(res), 0644)
	if err != nil {
		return err
	}

	return err
}
