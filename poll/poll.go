package poll

import (
	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"fmt"
	"time"
	"github.com/nlewo/comin/types"
	"github.com/go-git/go-git/v5"
)

func Poller(hostname string, stateDir string, dryRun bool, repositories []string) error {
	s := gocron.NewScheduler(time.UTC)
	period := 1
	config := makeConfig(stateDir, hostname, dryRun, repositories)
	repository, err := RepositoryOpen(config.GitConfig)
	if err != nil {
		return fmt.Errorf("Failed to open the repository: %s", err)
	}
	logrus.Infof("Polling every %d seconds to deploy the machine '%s'", period, hostname)
	job, _ := s.Every(period).Second().Tag("poll").Do(
		func () error {
			return poll(repository, config)
		})
	job.SingletonMode()
	s.StartBlocking()
	return nil
}

func makeConfig(stateDir string, hostname string, dryRun bool, repositories []string) types.Config {
	return types.Config{
		Hostname: hostname,
		StateDir: stateDir,
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

	hasNewCommits, isTesting, err := RepositoryUpdate(repository, config.GitConfig)
	if err != nil {
		logrus.Error(err)
		return err
	}
	if !hasNewCommits {
		return nil
	}

	operation := "switch"
	if isTesting {
		operation = "test"
	}
	err = Deploy(config, operation)
	if err != nil {
		logrus.Error(err)
		return err
	}
	return nil
}
