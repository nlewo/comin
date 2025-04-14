package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/nlewo/comin/internal/types"
	"github.com/sirupsen/logrus"
)

func RepositoryClone(directory, url, commitId, accessToken string) error {
	options := &git.CloneOptions{
		URL:        url,
		NoCheckout: true,
	}
	if accessToken != "" {
		options.Auth = &http.BasicAuth{
			Username: "comin",
			Password: accessToken,
		}
	}
	repository, err := git.PlainClone(directory, false, options)
	if err != nil {
		return err
	}
	worktree, err := repository.Worktree()
	if err != nil {
		return err
	}
	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(commitId),
	})
	if err != nil {
		return fmt.Errorf("cannot checkout the commit ID %s: '%s'", commitId, err)
	}
	return nil
}

func getRemoteCommitHash(r repository, remote, branch string) *plumbing.Hash {
	remoteBranch := fmt.Sprintf("refs/remotes/%s/%s", remote, branch)
	remoteHeadRef, err := r.Repository.Reference(
		plumbing.ReferenceName(remoteBranch),
		true)
	if err != nil {
		return nil
	}
	if remoteHeadRef == nil {
		return nil
	}
	commitId := remoteHeadRef.Hash()
	return &commitId
}

func hasNotBeenHardReset(r repository, branchName string, currentMainHash *plumbing.Hash, remoteMainHead *plumbing.Hash) error {
	if currentMainHash != nil && remoteMainHead != nil && *currentMainHash != *remoteMainHead {
		var ok bool
		ok, err := isAncestor(r.Repository, *currentMainHash, *remoteMainHead)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("this branch has been hard reset: its head '%s' is not on top of '%s'",
				remoteMainHead.String(), currentMainHash.String())
		}
	}
	return nil
}

func getHeadFromRemoteAndBranch(r repository, remoteName, branchName, currentMainCommitId string) (newHead plumbing.Hash, msg string, err error) {
	var currentMainHash *plumbing.Hash
	head := getRemoteCommitHash(r, remoteName, branchName)
	if head == nil {
		return newHead, "", fmt.Errorf("the branch '%s/%s' doesn't exist", remoteName, branchName)
	}
	if currentMainCommitId != "" {
		c := plumbing.NewHash(currentMainCommitId)
		currentMainHash = &c
	}

	if err = hasNotBeenHardReset(r, branchName, currentMainHash, head); err != nil {
		if r.GitConfig.AllowForcePushMain {
			logrus.Infof("Force-push detected but ignored due to 'allowForcePushMain' being set")
		} else {
			return
		}
	}

	commitObject, err := r.Repository.CommitObject(*head)
	if err != nil {
		return
	}

	return *head, commitObject.Message, nil
}

func hardReset(r repository, newHead plumbing.Hash) error {
	var w *git.Worktree
	w, err := r.Repository.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get the worktree")
	}
	err = w.Checkout(&git.CheckoutOptions{
		Hash:  newHead,
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("git reset --hard %s fails: '%s'", newHead, err)
	}
	return nil
}

// fetch fetches the config.Remote
func fetch(r repository, remote types.Remote) (err error) {
	logrus.Debugf("Fetching remote '%s'", remote.Name)
	fetchOptions := git.FetchOptions{
		RemoteName: remote.Name,
	}
	// TODO: support several authentication methods
	if remote.Auth.AccessToken != "" {
		fetchOptions.Auth = &http.BasicAuth{
			// On GitLab, any non blank username is
			// working.
			Username: "comin",
			Password: remote.Auth.AccessToken,
		}
	}

	// TODO: we should get a parent context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(remote.Timeout)*time.Second)
	defer cancel()
	// TODO: we should only fetch tracked branches
	err = r.Repository.FetchContext(ctx, &fetchOptions)
	if err == nil {
		logrus.Infof("New commits have been fetched from '%s'", remote.URL)
		return nil
	} else if err != git.NoErrAlreadyUpToDate {
		logrus.Errorf("Pull from remote '%s' failed: %s", remote.Name, err)
		return fmt.Errorf("'git fetch %s' fails: '%s'", remote.Name, err)
	} else {
		logrus.Debugf("No new commits have been fetched from the remote '%s'", remote.Name)
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
	_ = iter.ForEach(func(commit *object.Commit) error {
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

func repositoryOpen(config types.GitConfig) (r *git.Repository, err error) {
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
	return
}

func manageRemotes(r *git.Repository, remotes []types.Remote) error {
	for _, remote := range remotes {
		if err := manageRemote(r, remote); err != nil {
			return err
		}
	}
	return nil
}

func manageRemote(r *git.Repository, remote types.Remote) error {
	gitRemote, err := r.Remote(remote.Name)
	if err == git.ErrRemoteNotFound {
		logrus.Infof("Adding remote '%s' with url '%s'", remote.Name, remote.URL)
		_, err = r.CreateRemote(&gitConfig.RemoteConfig{
			Name: remote.Name,
			URLs: []string{remote.URL},
		})
		if err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}

	remoteConfig := gitRemote.Config()
	if remoteConfig.URLs[0] != remote.URL {
		if err := r.DeleteRemote(remote.Name); err != nil {
			return err
		}
		logrus.Infof("Updating remote %s (%s)", remote.Name, remote.URL)
		_, err = r.CreateRemote(&gitConfig.RemoteConfig{
			Name: remote.Name,
			URLs: []string{remote.URL},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func headSignedBy(r *git.Repository, publicKeys []string) (signedBy *openpgp.Entity, err error) {
	head, _ := r.Head()
	if head == nil {
		return nil, fmt.Errorf("repository HEAD should not be nil")
	}
	commit, err := r.CommitObject(head.Hash())
	if err != nil {
		return nil, err
	}
	for _, k := range publicKeys {
		entity, err := commit.Verify(k)
		if err == nil {
			logrus.Debugf("Commit %s signed by %s", head.Hash(), entity.PrimaryIdentity().Name)
			return entity, nil
		}
	}
	return nil, fmt.Errorf("commit %s is not signed", head.Hash())
}
