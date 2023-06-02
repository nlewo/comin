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
func Run(w worker.Worker, cfg types.Inotify) {
	if cfg.RepositoryPath == "" {
		logrus.Info("Inotify is not enabled because it has not been configured")
		return
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.Fatal(err)
	}
	defer watcher.Close()

	gitDir := filepath.Join(cfg.RepositoryPath, ".git")
	gitIndex := filepath.Join(cfg.RepositoryPath, ".git", "index")
	gitCommitEditMsg := filepath.Join(cfg.RepositoryPath, ".git", "COMMIT_EDITMSG")
	logrus.Infof("Inotify is watching the directory '%s'", gitDir)
	err = watcher.Add(gitDir)
	if err != nil {
		logrus.Fatal(err)
	}
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			logrus.Debugf("fsnotify - event: %v", event)

			isHardReset := event.Name == gitIndex
			isCommit := event.Name == gitCommitEditMsg && event.Has(fsnotify.Rename)
			if isHardReset || isCommit {
				if w.Beat(worker.Params{}) {
					logrus.Infof("Inotify triggered a deployment because the directory '%s' changed", gitDir)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logrus.Error("Error in fsnotify: ", err)
		}
	}
}
