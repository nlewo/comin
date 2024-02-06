package repository

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"
)

func commitFile(remoteRepository *git.Repository, dir, branch, content string) (commitId string, err error) {
	w, err := remoteRepository.Worktree()
	if err != nil {
		return
	}
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
		Force:  true,
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
	hash, err := w.Commit(content, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Unix(0, 0),
		},
	})
	if err != nil {
		return
	}
	return hash.String(), nil
}

func initRemoteRepostiory(dir string, initTesting bool) (remoteRepository *git.Repository, err error) {
	remoteRepository, err = git.PlainInit(dir, false)
	if err != nil {
		return
	}

	_, err = commitFile(remoteRepository, dir, "main", "file-1")
	if err != nil {
		return
	}
	_, err = commitFile(remoteRepository, dir, "main", "file-2")
	if err != nil {
		return
	}
	_, err = commitFile(remoteRepository, dir, "main", "file-3")
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
	if initTesting {
		ref = plumbing.NewHashReference("refs/heads/testing", headRef.Hash())
		err = remoteRepository.Storer.SetReference(ref)
		if err != nil {
			return
		}
	}
	return
}

func HeadCommitId(r *git.Repository) string {
	ref, err := r.Head()
	if err != nil {
		return ""
	}
	return ref.Hash().String()
}

func TestIsAncestor(t *testing.T) {
	remoteRepositoryDir := t.TempDir()
	repository, err := initRemoteRepostiory(remoteRepositoryDir, true)
	assert.Nil(t, err)

	iter, err := repository.Log(&git.LogOptions{})
	assert.Nil(t, err)

	commits := make([]object.Commit, 3)
	idx := 0
	err = iter.ForEach(func(commit *object.Commit) error {
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
