package poll

import (
	"fmt"
	"github.com/go-co-op/gocron"
	"github.com/nlewo/comin/config"
	"github.com/nlewo/comin/deploy"
	cominGit "github.com/nlewo/comin/git"
	"github.com/nlewo/comin/types"
	"github.com/sirupsen/logrus"
	"time"
)

func Poller(dryRun bool, cfg types.Configuration) error {
	s := gocron.NewScheduler(time.UTC)
	gitConfig := config.MkGitConfig(cfg)
	repository, err := cominGit.RepositoryOpen(gitConfig)
	if err != nil {
		return fmt.Errorf("Failed to open the repository: %s", err)
	}
	logrus.Infof("Polling every %d seconds to deploy the machine '%s'", cfg.Poller.Period, cfg.Hostname)
	job, _ := s.Every(cfg.Poller.Period).Second().Tag("poll").Do(
		func() error {
			logrus.Debugf("Executing a poll iteration")
			err = deploy.Deploy(repository, cfg, dryRun)
			if err != nil {
				logrus.Error(err)
			}
			return err
		})
	job.SingletonMode()
	s.StartBlocking()
	return nil
}
