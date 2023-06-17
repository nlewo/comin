package worker

import (
	"github.com/sirupsen/logrus"
)

type Params struct {
	RemoteName string
}

type Worker struct {
	params chan Params
	// works the function actually executed by the worker
	work func(remoteName string) error
}

func NewWorker(work func(remoteName string) error) (w Worker) {
	params := make(chan Params, 10)

	return Worker{
		params: params,
		work:   work,
	}
}

func (w Worker) Beat(params Params) bool {
	select {
	case w.params <- params:
		logrus.Debugf("Beat: tick the worker")
		return true
	default:
		logrus.Debugf("Beat: the worker is busy")
		return false
	}
}

func (w Worker) Run() {
	logrus.Infof("Starting the worker")
	for {
		params := <-w.params
		if err := w.work(params.RemoteName); err != nil {
			logrus.Debugf("The work function failed: %s", err)
		}
	}
}
