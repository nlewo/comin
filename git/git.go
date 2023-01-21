package git

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/nlewo/comin/types"
	"github.com/sirupsen/logrus"
	"io/ioutil"
)

// checkout only checkouts the branch under specific condition
func RepositoryUpdate(r types.Repository) (newHead plumbing.Hash, fromBranch string, err error) {
	var head, mainHead, remoteMainHead, remoteTestingHead plumbing.Hash
	err = fetch(r)
	if err != nil {
		return
	}

	// This is the checkouted commit and it is used to detect
	// local updates.
	headRef, err := r.Repository.Reference(plumbing.HEAD, true)
	if headRef != nil && err == nil {
		head = headRef.Hash()
	}

	remoteMainBranch := fmt.Sprintf("refs/remotes/%s/%s", r.GitConfig.Remote.Name, r.GitConfig.Main)
	remoteMainHeadRef, err := r.Repository.Reference(
		plumbing.ReferenceName(remoteMainBranch),
		true)
	if err != nil || remoteMainHeadRef == nil {
		return newHead, fromBranch, fmt.Errorf("The remote branch '%s' doesn't exist", remoteMainBranch)
	}
	remoteMainHead = remoteMainHeadRef.Hash()
	newHead = remoteMainHead
	fromBranch = r.GitConfig.Main

	// The main branch can not be hard reseted: HEAD has to be an
	// ancestor of the remote main branch. The main branch is used
	// to ensure the remote branch has not been hard reset.
	mainBranch := fmt.Sprintf("refs/heads/%s", r.GitConfig.Main)
	mainHeadRef, err := r.Repository.Reference(
		plumbing.ReferenceName(mainBranch),
		true)
	if err == nil && mainHeadRef != nil {
		mainHead = mainHeadRef.Hash()
	}
	if mainHeadRef != nil && remoteMainHeadRef != nil && mainHead != remoteMainHead {
		var ok bool
		ok, err = isAncestor(r.Repository, mainHead, remoteMainHead)
		if err != nil {
			return
		}
		if !ok {
			return newHead, fromBranch, fmt.Errorf("The remote main branch '%s' has been hard reset, refusing to check it out", r.GitConfig.Main)
		}
	}
	// Since we know the main remote branch has not been hard
	// reset, we can pull remote main branch into the local main
	// branch.
	if remoteMainHead != mainHead {
		logrus.Infof("The local branch '%s' has been reset on the remote branch '%s' (commit '%s')",
			r.GitConfig.Main, r.GitConfig.Main, remoteMainHead)
		ref := plumbing.NewHashReference(plumbing.ReferenceName(mainBranch), remoteMainHead)
		err = r.Repository.Storer.SetReference(ref)
		if err != nil {
			return newHead, fromBranch, fmt.Errorf("Failed to set the reference '%s': '%s'", ref, err)
		}
	}
	remoteTestingBranch := fmt.Sprintf("refs/remotes/%s/%s", r.GitConfig.Remote.Name, r.GitConfig.Testing)
	remoteTestingHeadRef, err := r.Repository.Reference(
		plumbing.ReferenceName(remoteTestingBranch),
		true)
	if err != nil || remoteTestingHeadRef == nil {
		logrus.Debugf("The remote branch '%s' doesn't exist", remoteTestingBranch)
	} else {
		remoteTestingHead = remoteTestingHeadRef.Hash()
	}

	if remoteTestingHeadRef != nil {
		// If the testing branch is on top of the main branch, we hard
		// reset to the testing branch
		var ancestor bool
		ancestor, err = isAncestor(r.Repository, remoteMainHead, remoteTestingHead)
		if err != nil {
			return
		}
		if ancestor {
			newHead = remoteTestingHead
			fromBranch = r.GitConfig.Testing
		}
	}

	if newHead != head {
		var w *git.Worktree
		w, err = r.Repository.Worktree()
		if err != nil {
			return newHead, fromBranch, fmt.Errorf("Failed to get the worktree")
		}
		err = w.Checkout(&git.CheckoutOptions{
			Hash:  newHead,
			Force: true,
		})
		if err != nil {
			return newHead, fromBranch, fmt.Errorf("git reset --hard %s fails: '%s'", newHead, err)
		}
		logrus.Infof("The commit '%s' from branch '%s' has been checked out", newHead, fromBranch)
	}
	return newHead, fromBranch, nil
}

// fetch fetches the config.Remote
func fetch(r types.Repository) (err error) {
	logrus.Debugf("Fetching remote '%s'", r.GitConfig.Remote.Name)
	fetchOptions := git.FetchOptions{
		RemoteName: r.GitConfig.Remote.Name,
	}
	// TODO: support several authentication methods
	if r.GitConfig.Remote.Auth.AccessToken != "" {
		fetchOptions.Auth = &http.BasicAuth{
			// On GitLab, any non blank username is
			// working.
			Username: "comin",
			Password: r.GitConfig.Remote.Auth.AccessToken,
		}
	}

	// TODO: should only fetch tracked branches
	err = r.Repository.Fetch(&fetchOptions)
	if err == nil {
		logrus.Infof("New commits have been fetched from '%s'", r.GitConfig.Remote.URL)
		return nil
	} else if err != git.NoErrAlreadyUpToDate {
		logrus.Infof("Pull from remote '%s' failed: %s", r.GitConfig.Remote.Name, err)
		return fmt.Errorf("'git fetch %s' fails: '%s'", r.GitConfig.Remote.Name, err)
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
	iter.ForEach(func(commit *object.Commit) error {
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
func RepositoryOpen(config types.GitConfig) (r types.Repository, err error) {
	r.GitConfig = config
	r.Repository, err = git.PlainInit(config.Path, false)
	if err != nil {
		r.Repository, err = git.PlainOpen(config.Path)
		if err != nil {
			return
		}
		logrus.Debugf("The local Git repository located at '%s' has been opened", config.Path)
	} else {
		logrus.Infof("The local Git repository located at '%s' has been initialized", config.Path)
	}
	err = manageRemote(r.Repository, config)
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
