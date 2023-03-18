package worker

import (
	"github.com/nlewo/comin/types"
	"github.com/sirupsen/logrus"
	"time"
)

type Worker struct {
	signal chan struct{}
	// works the function actually executed by the worker
	work func() error
}

func NewWorker(work func() error) (w Worker) {
	signal := make(chan struct{})

	return Worker{
		signal: signal,
		work:   work,
	}
}

func Scheduler(w Worker, period int) {
	for {
		w.Beat()
		time.Sleep(time.Duration(period) * time.Second)
	}
}

func (w Worker) Beat() bool {
	select {
	case w.signal <- struct{}{}:
		logrus.Debugf("Beat: tick the worker")
		return true
	default:
		logrus.Debugf("Beat: the worker is busy")
		return false
	}
}

func (w Worker) Run(dryRun bool, cfg types.Configuration) {
	logrus.Infof("Starting the worker")
	for {
		<-w.signal
		logrus.Debugf("Starting the run the work function")
		if err := w.work(); err != nil {
			logrus.Infof("The work function fails with: %#v", err)
		}
	}
}
