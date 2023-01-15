package poll

import (
	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"os"
	"fmt"
	"time"
	"github.com/nlewo/comin/types"
	"github.com/nlewo/comin/deploy"
	cominGit "github.com/nlewo/comin/git"
	"io/ioutil"
	"encoding/json"
)

func Poller(period int, hostname string, stateDir string, authsFilepath string, dryRun bool, repositories []string) error {
	s := gocron.NewScheduler(time.UTC)
	config, gitConfig, err := makeConfigs(stateDir, authsFilepath, hostname, dryRun, repositories)
	if err != nil {
		return err
	}
	logrus.Debugf("Config is '%#v'", config)
	repository, err := cominGit.RepositoryOpen(gitConfig)
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

func makeConfigs(stateDir string, authsFilepath string, hostname string, dryRun bool, repositories []string) (config types.Config, gitConfig types.GitConfig, err error) {
	config = types.Config{
		Hostname: hostname,
		StateDir: stateDir,
		StateFile: fmt.Sprintf("%s/state.json", stateDir),
		DryRun: dryRun,
	}
	gitConfig = types.GitConfig{
		Path: fmt.Sprintf("%s/repository", stateDir),
		Remote: types.Remote{
			Name: "origin",
			// TODO: support multiple repositories
			URL: repositories[0],
		},
		Main: "master",
		Testing: "testing",
	}

	if authsFilepath != "" {
		auths, err := readAuths(authsFilepath)
		if err != nil {
			return config, gitConfig, fmt.Errorf("Failed to read auths file: '%s'", err)
		}
		auth, exists := auths[repositories[0]]
		if (exists) {
			gitConfig.Remote.Auth = auth
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

func poll(repository types.Repository, config types.Config) error {
	logrus.Debugf("Executing a poll iteration")
	return deploy.Deploy(repository, config)
}
