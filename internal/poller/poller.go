package poller

import (
	"time"

	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/types"
	"github.com/sirupsen/logrus"
)

func Poller(m manager.Manager, remotes []types.Remote) {
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
		toFetch := make([]string, 0)
		for _, remote := range remotes {
			if remote.Poller.Period != 0 && counter%remote.Poller.Period == 0 {
				toFetch = append(toFetch, remote.Name)
			}
		}
		if len(toFetch) > 0 {
			m.Fetch(toFetch)
		}
		time.Sleep(time.Second)
		counter += 1
	}
}
