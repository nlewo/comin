package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/nlewo/comin/internal/types"
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

func TestHardReset(t *testing.T) {
	tests := []struct {
		name        string
		submodules  bool
		expectExist bool
	}{
		{"with submodules enabled", true, true},
		{"without submodules", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			submoduleDir := t.TempDir()
			mainRepoDir := t.TempDir()

			submoduleRepo, err := git.PlainInit(submoduleDir, false)
			assert.Nil(t, err)

			submoduleCommitId, err := commitFile(submoduleRepo, submoduleDir, "main", "submodule-content")
			assert.Nil(t, err)

			mainRepo, err := git.PlainInit(mainRepoDir, false)
			assert.Nil(t, err)

			_, err = commitFile(mainRepo, mainRepoDir, "main", "main-file")
			assert.Nil(t, err)

			err = addSubmoduleWithCommit(mainRepo, mainRepoDir, "mysub", submoduleDir, submoduleCommitId)
			assert.Nil(t, err)

			w, err := mainRepo.Worktree()
			assert.Nil(t, err)

			commitHash, err := w.Commit("add submodule", &git.CommitOptions{
				Author: &object.Signature{
					Name:  "John Doe",
					Email: "john@doe.org",
					When:  time.Unix(0, 0),
				},
			})
			assert.Nil(t, err)

			r := repository{
				Repository: mainRepo,
				GitConfig: types.GitConfig{
					Submodules: tt.submodules,
				},
			}

			err = hardReset(r, commitHash, nil)
			assert.Nil(t, err)

			submoduleContentPath := filepath.Join(mainRepoDir, "mysub", "submodule-content")
			_, err = os.Stat(submoduleContentPath)
			if tt.expectExist {
				assert.Nil(t, err, "submodule content should exist after hardReset with submodules enabled")
			} else {
				assert.True(t, os.IsNotExist(err), "submodule content should NOT exist after hardReset with submodules disabled")
			}
		})
	}
}

func addSubmoduleWithCommit(repo *git.Repository, repoDir, path, url, commitId string) error {
	gitmodulesContent := `[submodule "` + path + `"]
	path = ` + path + `
	url = ` + url + `
`
	err := os.WriteFile(filepath.Join(repoDir, ".gitmodules"), []byte(gitmodulesContent), 0644)
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	_, err = w.Add(".gitmodules")
	if err != nil {
		return err
	}

	cfg, err := repo.Config()
	if err != nil {
		return err
	}
	cfg.Submodules[path] = &gitConfig.Submodule{
		Path: path,
		URL:  url,
	}
	err = repo.SetConfig(cfg)
	if err != nil {
		return err
	}

	idx, err := repo.Storer.Index()
	if err != nil {
		return err
	}

	idx.Entries = append(idx.Entries, &index.Entry{
		Name: path,
		Hash: plumbing.NewHash(commitId),
		Mode: filemode.Submodule,
	})

	return repo.Storer.SetIndex(idx)
}
