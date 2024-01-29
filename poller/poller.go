package poller

import (
	"time"

	"github.com/nlewo/comin/manager"
	"github.com/nlewo/comin/types"
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
		for _, remote := range remotes {
			if remote.Poller.Period != 0 && counter%remote.Poller.Period == 0 {
				m.Fetch(remote.Name)
			}
		}
		time.Sleep(time.Second)
		counter += 1
	}
}
