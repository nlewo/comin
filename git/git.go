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
		return fmt.Errorf("Cannot checkout the commit ID %s: '%s'", commitId, err)
	}
	return nil
}

func getRemoteCommitHash(r types.Repository, remote, branch string) *plumbing.Hash {
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

func hasNotBeenHardReset(r types.Repository, branchName string, currentMainHash *plumbing.Hash, remoteMainHead *plumbing.Hash) error {
	if currentMainHash != nil && remoteMainHead != nil && *currentMainHash != *remoteMainHead {
		var ok bool
		ok, err := isAncestor(r.Repository, *currentMainHash, *remoteMainHead)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("The remote main branch '%s' has been hard reset, refusing to check it out",
				branchName)
		}
	}
	return nil
}

func getHeadFromRemote(r types.Repository, remoteName, currentMainCommitId string) (newHead plumbing.Hash, fromBranch string, err error) {
	var currentMainHash, remoteMainHead, remoteTestingHead *plumbing.Hash
	remote, err := getRemote(r, remoteName)
	if err != nil {
		return newHead, fromBranch, err
	}
	fromBranch = remote.Branches.Main.Name
	if currentMainCommitId != "" {
		c := plumbing.NewHash(currentMainCommitId)
		currentMainHash = &c
	}

	remoteMainHead = getRemoteCommitHash(r, remoteName, remote.Branches.Main.Name)
	remoteTestingHead = getRemoteCommitHash(r, remoteName, remote.Branches.Testing.Name)

	if remoteMainHead == nil {
		return newHead, fromBranch, fmt.Errorf("The branch '%s/%s' doesn't exist", remoteName, remote.Branches.Main.Name)
	}

	newHead = *remoteMainHead

	if err = hasNotBeenHardReset(r, remote.Branches.Main.Name, currentMainHash, remoteMainHead); err != nil {
		return
	}

	if remoteTestingHead != nil {
		// If the testing branch is on top of the main branch, we hard
		// reset to the testing branch
		var ancestor bool
		// We previously ensured remoteMainHead is
		// currentMainCommitId or on top of
		// currentMainCommitId.
		ancestor, err = isAncestor(r.Repository, *remoteMainHead, *remoteTestingHead)
		if err != nil {
			return
		}
		if ancestor {
			newHead = *remoteTestingHead
			fromBranch = remote.Branches.Testing.Name
		}
	}
	return
}

// checkout only checkouts the branch under specific condition
// If remoteName is "", all remotes are fetched
func RepositoryUpdate(r types.Repository, remoteName string, currentMainCommitId string, lastDeployedCommitId string) (newHead plumbing.Hash, fromRemote, fromBranch string, err error) {
	var remotes []types.Remote
	var remote types.Remote

	if remoteName != "" {
		remote, err = getRemote(r, remoteName)
		if err != nil {
			return
		}
		remotes = append(remotes, remote)
	} else {
		remotes = r.GitConfig.Remotes
	}

	for _, remote := range remotes {
		if err = fetch(r, remote); err != nil {
			return
		}
	}

	for _, remote := range r.GitConfig.Remotes {
		newHead, fromBranch, err = getHeadFromRemote(r, remote.Name, currentMainCommitId)
		fromRemote = remote.Name
		if err != nil {
			return
		}
		if newHead.String() != lastDeployedCommitId {
			break
		}
	}

	if newHead.String() != lastDeployedCommitId {
		if err := hardReset(r, newHead); err != nil {
			return newHead, fromRemote, fromBranch, err
		}
		logrus.Infof("The current main commit is '%s'", currentMainCommitId)
		logrus.Infof("The last deployed commit was '%s'", lastDeployedCommitId)
		logrus.Infof("The commit '%s' from '%s/%s' has been checked out", newHead, fromRemote, fromBranch)
	}
	return newHead, fromRemote, fromBranch, nil
}

func hardReset(r types.Repository, newHead plumbing.Hash) error {
	var w *git.Worktree
	w, err := r.Repository.Worktree()
	if err != nil {
		return fmt.Errorf("Failed to get the worktree")
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

func IsTesting(r types.Repository, remoteName, branchName string) bool {
	remote, err := getRemote(r, remoteName)
	if err != nil {
		return false
	}
	return remote.Branches.Testing.Name == branchName
}

func getRemote(r types.Repository, remoteName string) (remote types.Remote, err error) {
	for _, remote := range r.GitConfig.Remotes {
		if remote.Name == remoteName {
			return remote, nil
		}
	}
	return remote, fmt.Errorf("The remote '%s' doesn't exist", remoteName)
}

// fetch fetches the config.Remote
func fetch(r types.Repository, remote types.Remote) (err error) {
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

	// TODO: should only fetch tracked branches
	err = r.Repository.Fetch(&fetchOptions)
	if err == nil {
		logrus.Infof("New commits have been fetched from '%s'", remote.URL)
		return nil
	} else if err != git.NoErrAlreadyUpToDate {
		logrus.Infof("Pull from remote '%s' failed: %s", remote.Name, err)
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
	err = manageRemotes(r.Repository, config.Remotes)
	if err != nil {
		return
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
