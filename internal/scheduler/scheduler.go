package scheduler

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/nlewo/comin/internal/fetcher"
	"github.com/nlewo/comin/internal/types"
	"github.com/sirupsen/logrus"
)

type Scheduler struct {
	s gocron.Scheduler
}

func New() Scheduler {
	s, _ := gocron.NewScheduler()

	sched := Scheduler{
		s: s,
	}
	go sched.s.Start()
	return sched
}

func (s Scheduler) FetchRemotes(fetcher *fetcher.Fetcher, remotes []types.Remote) {
	for _, remote := range remotes {
		if remote.Poller.Period != 0 {
			logrus.Infof("scheduler: starting the period job for the remote '%s' with period %ds", remote.Name, remote.Poller.Period)
			_, _ = s.s.NewJob(
				gocron.DurationJob(
					time.Duration(remote.Poller.Period)*time.Second,
				),
				gocron.NewTask(
					func() {
						logrus.Debugf("scheduler: running task for remote %s", remote.Name)
						fetcher.TriggerFetch([]string{remote.Name})
					},
				),
				gocron.WithSingletonMode(gocron.LimitModeReschedule),
				gocron.WithName(fmt.Sprintf("fetch-remote-%s", remote.Name)),
			)
		}
	}
}
