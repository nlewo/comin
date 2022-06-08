package poll

import (
	"github.com/go-co-op/gocron"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/sirupsen/logrus"
	"time"
	"fmt"
	"io/ioutil"
)

func Poller(repositories []string) {
	logrus.SetLevel(logrus.DebugLevel)

	s := gocron.NewScheduler(time.UTC)
	job, _ := s.Every(1).Second().Tag("poll").Do(poll)
	job.SingletonMode()
	s.StartBlocking()
}

func poll() error {
	logrus.Debugf("Executing poll()")
	err := fetch()
	if err != nil {
		logrus.Fatal(err)
		return err
	}
	return nil
}

type Remote struct {
	Name string
	URL string
}

func fetch() (err error) {
	localRepositoryPath := "/tmp/comin"
	repositoryUrl := "https://framagit.org/markas/infrastructure.git"
	keyring, err := ioutil.ReadFile("/tmp/keyring")
	if err != nil {
		return err
	}

	r, err := openOrInit(localRepositoryPath)

	remotes := []config.RemoteConfig{
		config.RemoteConfig{
			Name: "origin",
			URLs: []string{"bla"},
		},
		config.RemoteConfig{
			Name: "fallback-1",
			URLs: []string{repositoryUrl},
		},
	}

	err = manageRemotes(r, remotes)
	if err != nil {
		return err
	}

	err = pull(r, remotes)
	if err != nil {
		return err
	}

	err = verifyHead(r, string(keyring))
	if err != nil {
		return err
	}

	return nil
}

// openOrInit inits the repository if it's not already a Git
// repository and opens it otherwise
func openOrInit(repositoryPath string) (r *git.Repository, err error){
	r, err = git.PlainInit(repositoryPath, false)
	if err != nil {
		r, err = git.PlainOpen(repositoryPath)
		if err != nil {
			return
		}
		logrus.Debugf("Git repository located at %s has been opened", repositoryPath)
	} else {
		logrus.Infof("Git repository located at %s has been initialized", repositoryPath)
	}
	return
}

func pull(r *git.Repository, remotes []config.RemoteConfig) error {
	w, err := r.Worktree()
	if err != nil {
		return err
	}

	for _, remote := range remotes {
		logrus.Debugf("Pulling remote '%s'", remote.Name)
		err = w.Pull(&git.PullOptions{RemoteName: remote.Name})
		if err == nil {
			// TODO: get the list of new commits and return it.
			return nil
		} else if err != git.NoErrAlreadyUpToDate {
			logrus.Infof("Pull from remote '%s' failed: %s", remote.Name, err)
		}
	}
	return nil
}

func manageRemotes(r *git.Repository, remotes []config.RemoteConfig) error {
	for _, expectedRemoteConfig := range remotes {
		url := expectedRemoteConfig.URLs[0]
		name := expectedRemoteConfig.Name

		remote, err := r.Remote(name)
		if err == git.ErrRemoteNotFound {
			logrus.Infof("Adding remote %s (%s)", name, url)
			_, err = r.CreateRemote(&expectedRemoteConfig)
			if err != nil {
				return err
			}
			continue
		} else if err != nil {
			return err
		}

		remoteConfig := remote.Config()
		if remoteConfig.URLs[0] != url {
			if err := r.DeleteRemote(name); err != nil {
				return err
			}
			logrus.Infof("Updating remote %s (%s)", name, url)
			_, err = r.CreateRemote(&expectedRemoteConfig)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func verifyHead(r *git.Repository, keyring string) error {
	head, err := r.Head()
	if head == nil {
		return fmt.Errorf("Repository HEAD should not be nil")
	}
	logrus.Debugf("Repository HEAD is %s", head.Strings()[1])

	commit, err := r.CommitObject(head.Hash())
	if err != nil {
		return err
	}
	entity, err := commit.Verify(keyring)
	if err != nil {
		return err
	}
	logrus.Debugf("Commit %s signed by %s", head.Hash(), entity.PrimaryIdentity().Name)
	return nil
}
