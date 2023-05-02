package inotify

import (
	"github.com/fsnotify/fsnotify"
	"github.com/nlewo/comin/types"
	"github.com/nlewo/comin/worker"
	"github.com/sirupsen/logrus"
	"path/filepath"
)

// Run watches the cfg.RepositoryPath with inotify and beat the worker
// when the file .git/index file change.
func Run(worker worker.Worker, cfg types.Inotify) {
	if cfg.RepositoryPath == "" {
		logrus.Info("Inotify is not enabled because it has not been configured")
		return
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.Fatal(err)
	}
	defer watcher.Close()

	gitIndex := filepath.Join(cfg.RepositoryPath, ".git", "index")
	logrus.Infof("Inotify is watching the file '%s'", gitIndex)
	err = watcher.Add(gitIndex)
	if err != nil {
		logrus.Fatal(err)
	}
	for {
		select {
		case _, ok := <-watcher.Events:
			if !ok {
				return
			}
			if worker.Beat() {
				logrus.Infof("Inotify triggered a deployment because the file '%s' changed", gitIndex)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logrus.Error("Error in fsnotify: ", err)
		}
	}
}
