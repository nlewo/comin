package git

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/nlewo/comin/types"
	"github.com/sirupsen/logrus"
	"io/ioutil"
)

// checkout only checkouts the branch under specific condition
func RepositoryUpdate(r *git.Repository, config types.GitConfig) (updated, isTesting bool, err error) {
	var head, mainHead, testingHead plumbing.Hash
	err = fetch(r, config)
	if err != nil {
		return
	}

	headRef, err := r.Reference(plumbing.HEAD, true)
	if headRef != nil && err == nil {
		head = headRef.Hash()
	}

	mainBranch := fmt.Sprintf("refs/remotes/%s/%s", config.Remote.Name, config.Main)
	mainHeadRef, err := r.Reference(
		plumbing.ReferenceName(mainBranch),
		true)
	if err != nil || mainHeadRef == nil {
		return updated, isTesting, fmt.Errorf("The remote branch '%s' doesn't exist", mainBranch)
	}
	mainHead = mainHeadRef.Hash()
	newHead := mainHead
	fromBranch := config.Main

	testingBranch := fmt.Sprintf("refs/remotes/%s/%s", config.Remote.Name, config.Testing)
	testingHeadRef, err := r.Reference(
		plumbing.ReferenceName(testingBranch),
		true)
	if err != nil || testingHeadRef == nil {
		logrus.Debugf("The remote branch '%s' doesn't exist", testingBranch)
	} else {
		testingHead = testingHeadRef.Hash()
	}

	if testingHeadRef != nil {
		// If the testing branch is on top of the main branch, we hard
		// reset to the testing branch
		var ancestor bool
		ancestor, err = isAncestor(r, mainHead, testingHead)
		if err != nil {
			return
		}
		if (ancestor) {
			newHead = testingHead
			fromBranch = config.Testing
			isTesting = true
		}
	} else {
		// The main branch can not be hard reset: HEAD has to
		// be an ancestor of the remote main branch.
		var ok bool
		ok, err = isAncestor(r, mainHead, head)
		if err != nil {
			return
		}
		if !ok {
			return false, false, fmt.Errorf("The remote main branch '%s' has been hard reseted")
		}
	}

	if newHead != head {
		var w *git.Worktree
		w, err = r.Worktree()
		if err != nil {
			return false, false, fmt.Errorf("Failed to get the worktree")
		}
		err = w.Checkout(&git.CheckoutOptions{
			Hash: newHead,
			Force: true,
		})
		if err != nil {
			return false, false, fmt.Errorf("git reset --hard %s fails: '%s'", newHead, err)
		}
		updated = true
		logrus.Infof("The commit '%s' from branch '%s' has been checked out", newHead, fromBranch)
	}
	return updated, isTesting, nil
}

// fetch fetches the config.Remote
func fetch(r *git.Repository, config types.GitConfig) (err error) {
	logrus.Debugf("Fetching remote '%s'", config.Remote.Name)
	// TODO: should only fetch tracked branches
	err = r.Fetch(&git.FetchOptions{RemoteName: config.Remote.Name})
	if err == nil {
		logrus.Infof("New commits have been fetched from '%s'", config.Remote.URL)
		return nil
	} else if err != git.NoErrAlreadyUpToDate {
		logrus.Infof("Pull from remote '%s' failed: %s", config.Remote.Name, err)
		return fmt.Errorf("'git fetch %s' fails: '%s'", config.Remote.Name, err)
	} else {
		logrus.Debugf("No new commits have been fetched")
		return nil
	}
}

// isAncestor returns true when the commitId is an ancestor of the branch branchName
func isAncestor(r *git.Repository, base, top plumbing.Hash) (found bool, err error) {
	iter, err := r.Log(&git.LogOptions{From: top})
	if err != nil {
		return false, fmt.Errorf("git log %s fails: '%s'", top, err)
	}

	// To skip the first commit
	isFirst := true
	iter.ForEach(func (commit *object.Commit) error {
		if !isFirst && commit.Hash == base {
			found = true
			// This error is ignored and used to terminate early the loop :/
			return fmt.Errorf("base commit is ancestor of top commit")
		}
		isFirst = false
		return nil
	})
	return
}

// openOrInit inits the repository if it's not already a Git
// repository and opens it otherwise
func RepositoryOpen(config types.GitConfig) (r *git.Repository, err error) {
	r, err = git.PlainInit(config.Path, false)
	if err != nil {
		r, err = git.PlainOpen(config.Path)
		if err != nil {
			return
		}
		logrus.Debugf("The local Git repository located at '%s' has been opened", config.Path)
	} else {
		logrus.Infof("The local Git repository located at '%s' has been initialized", config.Path)
	}
	err = manageRemote(r, config)
	if err != nil {
		return
	}
	return
}

func manageRemote(r *git.Repository, config types.GitConfig) error {
	remote, err := r.Remote(config.Remote.Name)
	if err == git.ErrRemoteNotFound {
		logrus.Infof("Adding remote '%s' with url '%s'", config.Remote.Name, config.Remote.URL)
		_, err = r.CreateRemote(&gitConfig.RemoteConfig{
			Name: config.Remote.Name,
			URLs: []string{config.Remote.URL},
		})
		if err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}

	remoteConfig := remote.Config()
	if remoteConfig.URLs[0] != config.Remote.URL {
		if err := r.DeleteRemote(config.Remote.Name); err != nil {
			return err
		}
		logrus.Infof("Updating remote %s (%s)", config.Remote.Name, config.Remote.URL)
		_, err = r.CreateRemote(&gitConfig.RemoteConfig{
			Name: config.Remote.Name,
			URLs: []string{config.Remote.URL},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func verifyHead(r *git.Repository, config types.GitConfig) error {
	head, err := r.Head()
	if head == nil {
		return fmt.Errorf("Repository HEAD should not be nil")
	}
	logrus.Debugf("Repository HEAD is %s", head.Strings()[1])

	commit, err := r.CommitObject(head.Hash())
	if err != nil {
		return err
	}

	for _, keyPath := range config.GpgPublicKeyPaths {
		key, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return err
		}
		entity, err := commit.Verify(string(key))
		if err != nil {
			logrus.Debug(err)
		} else {
			logrus.Debugf("Commit %s signed by %s", head.Hash(), entity.PrimaryIdentity().Name)
			return nil
		}

	}
	return fmt.Errorf("Commit %s is not signed", head.Hash())
}
