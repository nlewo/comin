package poller

import (
	"github.com/nlewo/comin/types"
	"github.com/nlewo/comin/worker"
	"github.com/sirupsen/logrus"
	"time"
)

func Poller(w worker.Worker, remotes []types.Remote) {
	poll := false
	for _, remote := range remotes {
		if remote.Poller.Period != 0 {
			logrus.Infof("Starting the poller for the remote '%s' with period %ds", remote.Name, remote.Poller.Period)
			poll = true
		}
	}
	if !poll {
		return
	}
	counter := 0
	for {
		for _, remote := range remotes {
			if remote.Poller.Period != 0 && counter%remote.Poller.Period == 0 {
				params := worker.Params{
					RemoteName: remote.Name,
				}
				w.Beat(params)
			}
		}
		time.Sleep(time.Second)
		counter += 1
	}
}
