package poll

import (
	"github.com/go-git/go-git/v5"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/nlewo/comin/types"
	"github.com/stretchr/testify/assert"
)

func commitFile(remoteRepository *git.Repository, dir, branch, content string) (err error) {
	w, err := remoteRepository.Worktree()
	if err != nil {
		return
	}
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
		Force: true,
	})

	filename := filepath.Join(dir, content)
	err = ioutil.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		return
	}
	_, err = w.Add(content)
	if err != nil {
		return
	}
	_, err = w.Commit(content, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})
	if err != nil {
		return
	}
	return
}

func initRemoteRepostiory(dir string) (remoteRepository *git.Repository,err error) {
	remoteRepository, err = git.PlainInit(dir, false)
	if err != nil {
		return
	}

	err = commitFile(remoteRepository, dir, "main", "file-1")
	if err != nil {
		return
	}
	err = commitFile(remoteRepository, dir, "main", "file-2")
	if err != nil {
		return
	}
	err = commitFile(remoteRepository, dir, "main", "file-3")
	if err != nil {
		return
	}

	headRef, err := remoteRepository.Head()
	if err != nil {
		return
	}
	ref := plumbing.NewHashReference("refs/heads/main", headRef.Hash())
	err = remoteRepository.Storer.SetReference(ref)
	if err != nil {
		return
	}
	ref = plumbing.NewHashReference("refs/heads/testing", headRef.Hash())
	err = remoteRepository.Storer.SetReference(ref)
	if err != nil {
		return
	}
	return
}

func TestIsAncestor(t *testing.T) {
	remoteRepositoryDir := t.TempDir()
	repository, err := initRemoteRepostiory(remoteRepositoryDir)
	assert.Nil(t, err)

	iter, err := repository.Log(&git.LogOptions{})
	assert.Nil(t, err)

	commits := make([]object.Commit, 3)
	idx := 0
	err = iter.ForEach(func (commit *object.Commit) error {
		commits[idx] = *commit
		idx += 1
		return nil
	})

	ret, _ := isAncestor(repository, commits[1].Hash, commits[0].Hash)
	assert.True(t, ret)

	ret, _ = isAncestor(repository, commits[0].Hash, commits[1].Hash)
	assert.False(t, ret)

	ret, _ = isAncestor(repository, commits[0].Hash, commits[0].Hash)
	assert.False(t, ret)

	ret, _ = isAncestor(repository, commits[2].Hash, commits[0].Hash)
	assert.True(t, ret)

	//time.Sleep(100*time.Second)
}

func TestRepositoryUpdateTesting(t *testing.T) {
	remoteRepositoryDir := t.TempDir()
	cominRepositoryDir := t.TempDir()
	remoteRepository, err := initRemoteRepostiory(remoteRepositoryDir)
	assert.Nil(t, err)

	gitConfig := types.GitConfig{
		Path: cominRepositoryDir,
		Remote: types.Remote{
			Name: "origin",
			URL: remoteRepositoryDir,
		},
		Main: "main",
		Testing: "testing",
	}
	cominRepository, err := RepositoryOpen(gitConfig)
	assert.Nil(t, err)

	// The remote repository is initially checkouted on main
	updated, isTesting, err := RepositoryUpdate(cominRepository, gitConfig)
	assert.Nil(t, err)
	assert.True(t, updated)
	assert.False(t, isTesting)

	// A new commit is pushed to the testing branch remote repository: the local
	// repository is updated
	err = commitFile(remoteRepository, remoteRepositoryDir, "testing", "file-4")
	assert.Nil(t, err)
	updated, isTesting, err = RepositoryUpdate(cominRepository, gitConfig)
	assert.Nil(t, err)
	assert.True(t, updated)
	assert.True(t, isTesting)

	// A new commit is pushed to the testing branch remote repository: the local
	// repository is updated
	err = commitFile(remoteRepository, remoteRepositoryDir, "testing", "file-4")
	assert.Nil(t, err)
	updated, isTesting, err = RepositoryUpdate(cominRepository, gitConfig)
	assert.Nil(t, err)
	assert.True(t, updated)
	assert.True(t, isTesting)

	// The main branch is rebased on top of testing: we switch
	// back the the main branch
	testingHeadRef, err := remoteRepository.Reference(
		plumbing.ReferenceName("refs/heads/testing"),
		true)
	ref := plumbing.NewHashReference("refs/heads/main", testingHeadRef.Hash())
	err = remoteRepository.Storer.SetReference(ref)
	if err != nil {
		return
	}
	updated, isTesting, err = RepositoryUpdate(cominRepository, gitConfig)
	assert.Nil(t, err)
	assert.False(t, updated)
	assert.False(t, isTesting)

	// time.Sleep(100*time.Second)
}

func TestRepositoryUpdateMain(t *testing.T) {
	remoteRepositoryDir := t.TempDir()
	cominRepositoryDir := t.TempDir()
	remoteRepository, err := initRemoteRepostiory(remoteRepositoryDir)
	assert.Nil(t, err)

	gitConfig := types.GitConfig{
		Path: cominRepositoryDir,
		Remote: types.Remote{
			Name: "origin",
			URL: remoteRepositoryDir,
		},
		Main: "main",
		Testing: "testing",
	}
	cominRepository, err := RepositoryOpen(gitConfig)
	assert.Nil(t, err)

	// The remote repository is initially checkouted
	updated, isTesting, err := RepositoryUpdate(cominRepository, gitConfig)
	assert.Nil(t, err)
	assert.True(t, updated)
	assert.False(t, isTesting)

	// Without any new remote commits, the local repository is not updated
	updated, isTesting, err = RepositoryUpdate(cominRepository, gitConfig)
	assert.Nil(t, err)
	assert.False(t, updated)
	assert.False(t, isTesting)

	// A new commit is pushed to the remote repository: the local
	// repository is updated
	err = commitFile(remoteRepository, remoteRepositoryDir, "main", "file-4")
	assert.Nil(t, err)
	updated, isTesting, err = RepositoryUpdate(cominRepository, gitConfig)
	assert.Nil(t, err)
	assert.True(t, updated)
	assert.False(t, isTesting)

	// A commit is pushed to the testing branch which is currently
	// behind the main branch: the repository is not updated
	err = commitFile(remoteRepository, remoteRepositoryDir, "testing", "file-5")
	assert.Nil(t, err)
	updated, isTesting, err = RepositoryUpdate(cominRepository, gitConfig)
	assert.Nil(t, err)
	assert.False(t, updated)
	assert.False(t, isTesting)

	// time.Sleep(100*time.Second)
}
