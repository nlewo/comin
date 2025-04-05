package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

func commitFile(remoteRepository *git.Repository, dir, branch, content string) (commitId string, err error) {
	return commitFileAndSign(remoteRepository, dir, branch, content, nil)
}

func commitFileAndSign(remoteRepository *git.Repository, dir, branch, content string, signKey *openpgp.Entity) (commitId string, err error) {
	w, err := remoteRepository.Worktree()
	if err != nil {
		return
	}
	_ = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
		Force:  true,
	})

	filename := filepath.Join(dir, content)
	err = os.WriteFile(filename, []byte(content), 0644)
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
		SignKey: signKey,
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
	_ = iter.ForEach(func(commit *object.Commit) error {
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

func TestHeadSignedBy(t *testing.T) {
	dir := t.TempDir()
	remoteRepository, _ := git.PlainInit(dir, false)

	r, _ := os.Open("./test.private")
	entityList, _ := openpgp.ReadArmoredKeyRing(r)
	_, _ = commitFileAndSign(remoteRepository, dir, "main", "file-1", entityList[0])

	failPublic, _ := os.ReadFile("./fail.public")
	testPublic, _ := os.ReadFile("./test.public")
	signedBy, err := headSignedBy(remoteRepository, []string{string(failPublic), string(testPublic)})
	assert.Nil(t, err)
	assert.Equal(t, "test <test@comin.space>", signedBy.PrimaryIdentity().Name)

	signedBy, err = headSignedBy(remoteRepository, []string{string(failPublic)})
	assert.ErrorContains(t, err, "is not signed")
	assert.Nil(t, signedBy)

	_, _ = commitFileAndSign(remoteRepository, dir, "main", "file-2", nil)
	signedBy, err = headSignedBy(remoteRepository, []string{string(failPublic), string(testPublic)})
	assert.ErrorContains(t, err, "is not signed")
	assert.Nil(t, signedBy)

}
