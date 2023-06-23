package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/nlewo/comin/types"
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

func TestRepositoryUpdateTesting(t *testing.T) {
	remoteRepositoryDir := t.TempDir()
	cominRepositoryDir := t.TempDir()
	remoteRepository, err := initRemoteRepostiory(remoteRepositoryDir, true)
	assert.Nil(t, err)

	gitConfig := types.GitConfig{
		Path: cominRepositoryDir,
		Remotes: []types.Remote{
			types.Remote{
				Name: "origin",
				URL:  remoteRepositoryDir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
		},
	}
	cominRepository, err := RepositoryOpen(gitConfig)
	assert.Nil(t, err)

	// The remote repository is initially checkouted on main
	commitId, _, branch, _, err := RepositoryUpdate(cominRepository, "", "", "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, "f8c4e82c08aa789bb7a28f16a9070026cd7eb077")
	assert.Equal(t, branch, "main")

	// A new commit is pushed to the testing branch remote repository: the local
	// repository is updated
	commitId4, err := commitFile(remoteRepository, remoteRepositoryDir, "testing", "file-4")
	assert.Nil(t, err)
	commitId, _, branch, _, err = RepositoryUpdate(cominRepository, "", "f8c4e82c08aa789bb7a28f16a9070026cd7eb077", "")
	assert.Nil(t, err)
	assert.Equal(t, commitId4, commitId)
	assert.Equal(t, "testing", branch)

	// A new commit is pushed to the testing branch remote repository: the local
	// repository is updated
	commitId5, err := commitFile(remoteRepository, remoteRepositoryDir, "testing", "file-5")
	assert.Nil(t, err)
	commitId, _, branch, _, err = RepositoryUpdate(cominRepository, "", "f8c4e82c08aa789bb7a28f16a9070026cd7eb077", "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, commitId5)
	assert.Equal(t, branch, "testing")

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
	commitId, _, branch, _, err = RepositoryUpdate(cominRepository, "", commitId5, "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, commitId5)
	assert.Equal(t, branch, "main")

	// time.Sleep(100*time.Second)
}

func TestRepositoryUpdateHardResetMain(t *testing.T) {
	remoteRepositoryDir := t.TempDir()
	cominRepositoryDir := t.TempDir()
	remoteRepository, err := initRemoteRepostiory(remoteRepositoryDir, true)
	assert.Nil(t, err)

	gitConfig := types.GitConfig{
		Path: cominRepositoryDir,
		Remotes: []types.Remote{
			types.Remote{
				Name: "origin",
				URL:  remoteRepositoryDir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
		},
	}
	cominRepository, err := RepositoryOpen(gitConfig)
	assert.Nil(t, err)

	// The remote repository is initially checkouted
	commitId, _, branch, _, err := RepositoryUpdate(cominRepository, "", "", "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, "f8c4e82c08aa789bb7a28f16a9070026cd7eb077")
	assert.Equal(t, branch, "main")

	// Two commits are added to get a previous commit hash in
	// order to reset it.
	previousHash, err := commitFile(remoteRepository, remoteRepositoryDir, "main", "file-4")
	newCommitId, err := commitFile(remoteRepository, remoteRepositoryDir, "main", "file-5")

	commitId, _, branch, _, err = RepositoryUpdate(cominRepository, "", "f8c4e82c08aa789bb7a28f16a9070026cd7eb077", "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, newCommitId)
	assert.Equal(t, branch, "main")

	// The last commit of the main branch is removed.
	ref := plumbing.NewHashReference("refs/heads/main", plumbing.NewHash(previousHash))
	err = remoteRepository.Storer.SetReference(ref)
	if err != nil {
		return
	}
	commitId, _, branch, _, err = RepositoryUpdate(cominRepository, "", commitId, "")
	assert.ErrorContains(t, err, "No valid Main branch found on all remotes")
}

func TestRepositoryUpdateMain(t *testing.T) {
	remoteRepositoryDir := t.TempDir()
	cominRepositoryDir := t.TempDir()
	remoteRepository, err := initRemoteRepostiory(remoteRepositoryDir, true)
	assert.Nil(t, err)

	gitConfig := types.GitConfig{
		Path: cominRepositoryDir,
		Remotes: []types.Remote{
			types.Remote{
				Name: "origin",
				URL:  remoteRepositoryDir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
		},
	}
	cominRepository, err := RepositoryOpen(gitConfig)
	assert.Nil(t, err)

	// The remote repository is initially checkouted
	commitId, _, branch, _, err := RepositoryUpdate(cominRepository, "", "", "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, "f8c4e82c08aa789bb7a28f16a9070026cd7eb077")
	assert.Equal(t, branch, "main")

	// Without any new remote commits, the local repository is not updated
	commitId, _, branch, _, err = RepositoryUpdate(cominRepository, "", commitId, "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, "f8c4e82c08aa789bb7a28f16a9070026cd7eb077")
	assert.Equal(t, branch, "main")

	// A new commit is pushed to the remote repository: the local
	// repository is updated
	newCommitId, err := commitFile(remoteRepository, remoteRepositoryDir, "main", "file-4")
	assert.Nil(t, err)
	commitId, _, branch, _, err = RepositoryUpdate(cominRepository, "", commitId, "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, newCommitId)
	assert.Equal(t, branch, "main")

	// A commit is pushed to the testing branch which is currently
	// behind the main branch: the repository is not updated
	_, err = commitFile(remoteRepository, remoteRepositoryDir, "testing", "file-5")
	assert.Nil(t, err)
	commitId, _, branch, _, err = RepositoryUpdate(cominRepository, "", commitId, "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, newCommitId)
	assert.Equal(t, branch, "main")

	// time.Sleep(100*time.Second)
}

func TestWithoutTesting(t *testing.T) {
	var err error
	r1Dir := t.TempDir()
	cominRepositoryDir := t.TempDir()
	_, err = initRemoteRepostiory(r1Dir, false)
	assert.Nil(t, err)
	gitConfig := types.GitConfig{
		Path: cominRepositoryDir,
		Remotes: []types.Remote{
			types.Remote{
				Name: "r1",
				URL:  r1Dir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
		},
	}
	cominRepository, err := RepositoryOpen(gitConfig)
	assert.Nil(t, err)
	commitId, remote, branch, _, err := RepositoryUpdate(cominRepository, "", "", "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, "f8c4e82c08aa789bb7a28f16a9070026cd7eb077")
	assert.Equal(t, branch, "main")
	assert.Equal(t, remote, "r1")
}

func TestMultipleRemote(t *testing.T) {
	var err error
	r1Dir := t.TempDir()
	r2Dir := t.TempDir()
	cominRepositoryDir := t.TempDir()
	r1, err := initRemoteRepostiory(r1Dir, true)
	r2, err := initRemoteRepostiory(r2Dir, true)
	assert.Nil(t, err)

	gitConfig := types.GitConfig{
		Path: cominRepositoryDir,
		Remotes: []types.Remote{
			types.Remote{
				Name: "r1",
				URL:  r1Dir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
			types.Remote{
				Name: "r2",
				URL:  r2Dir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
		},
	}
	cominRepository, err := RepositoryOpen(gitConfig)
	assert.Nil(t, err)
	// r1/main: c1 - c2 - *c3
	// r2/main: c1 - c2 - c3
	commitId, remote, branch, _, err := RepositoryUpdate(cominRepository, "", "", "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, "f8c4e82c08aa789bb7a28f16a9070026cd7eb077")
	assert.Equal(t, branch, "main")
	assert.Equal(t, remote, "r1")

	// r1/main: c1 - c2 - c3 - *c4
	// r2/main: c1 - c2 - c3
	newCommitId, err := commitFile(r1, r1Dir, "main", "file-4")
	assert.Nil(t, err)
	commitId, _, branch, _, err = RepositoryUpdate(cominRepository, "", commitId, "")
	assert.Nil(t, err)
	assert.Equal(t, commitId, newCommitId)
	assert.Equal(t, branch, "main")
	assert.Equal(t, remote, "r1")

	// r1/main: c1 - c2 - c3 - c4
	// r2/main: c1 - c2 - c3 - c4 - c5
	// r2/main: c1 - c2 - c3 - c4 - c5 - c6
	commitFile(r2, r2Dir, "main", "file-4")
	newCommitId, err = commitFile(r2, r2Dir, "main", "file-5")
	assert.Nil(t, err)
	commitId, remote, branch, _, err = RepositoryUpdate(cominRepository, "", commitId, commitId)
	assert.Nil(t, err)
	assert.Equal(t, newCommitId, commitId)
	assert.Equal(t, "main", branch)
	assert.Equal(t, "r2", remote)

	// r1/main: c1 - c2 - c3 - c4 - *c5
	// r2/main: c1 - c2 - c3 - c4 - c5
	newCommitId, err = commitFile(r1, r1Dir, "main", "file-5")
	assert.Nil(t, err)
	commitId, remote, branch, _, err = RepositoryUpdate(cominRepository, "", commitId, commitId)
	assert.Nil(t, err)
	assert.Equal(t, newCommitId, commitId)
	assert.Equal(t, "main", branch)
	assert.Equal(t, "r1", remote)

	// r1/main: c1 - c2 - c3 - c4 - c5 - c6
	// r2/main: c1 - c2 - c3 - c4 - c5 - c6
	// r2/testing: c1 - c2 - c3 - c4 - c5 - c6 - *c7
	c6, _ := commitFile(r1, r1Dir, "main", "file-6")
	commitFile(r2, r2Dir, "main", "file-6")
	commitFile(r2, r2Dir, "testing", "file-4")
	commitFile(r2, r2Dir, "testing", "file-5")
	commitFile(r2, r2Dir, "testing", "file-6")
	c7, _ := commitFile(r2, r2Dir, "testing", "file-7")
	commitId, remote, branch, _, err = RepositoryUpdate(cominRepository, "", commitId, commitId)
	assert.Nil(t, err)
	assert.Equal(t, c7, commitId)
	assert.Equal(t, "testing", branch)
	assert.Equal(t, "r2", remote)

	// r1/main: c1 - c2 - c3 - c4 - c5 - c6
	// r2/main: c1 - c2 - c3 - c4 - c5 - c6
	// r2/testing: c1 - c2 - c3 - c4 - c5 - c6 - *c7
	commitId, remote, branch, _, err = RepositoryUpdate(cominRepository, "", c6, c6)
	assert.Nil(t, err)
	assert.Equal(t, c7, commitId)
	assert.Equal(t, "testing", branch)
	assert.Equal(t, "r2", remote)

	// TODO we should return the main commit ID in order to store it in the state
	// r1/main: c1 - c2 - c3 - c4 - c5 - c6 - *c8
	// r2/main: c1 - c2 - c3 - c4 - c5 - c6
	// r2/testing: c1 - c2 - c3 - c4 - c5 - c6 - c7
	c8, _ := commitFile(r1, r1Dir, "main", "file-8")
	commitId, remote, branch, _, err = RepositoryUpdate(cominRepository, "", c6, c7)
	assert.Nil(t, err)
	assert.Equal(t, c8, commitId)
	assert.Equal(t, "main", branch)
	assert.Equal(t, "r1", remote)

	// Only fetch r2 remote
	// r1/main: c1 - c2 - c3 - c4 - c5 - c6 - *c8 - c9
	// r2/main: c1 - c2 - c3 - c4 - c5 - c6
	// r2/testing: c1 - c2 - c3 - c4 - c5 - c6 - c7
	c9, _ := commitFile(r1, r1Dir, "main", "file-9")
	commitId, remote, branch, _, err = RepositoryUpdate(cominRepository, "r2", c6, c7)
	assert.Nil(t, err)
	assert.Equal(t, c8, commitId)
	assert.Equal(t, "main", branch)
	assert.Equal(t, "r1", remote)

	// Fetch the r1 remote
	// r1/main: c1 - c2 - c3 - c4 - c5 - c6 - c8 - *c9
	// r2/main: c1 - c2 - c3 - c4 - c5 - c6
	// r2/testing: c1 - c2 - c3 - c4 - c5 - c6 - c7
	commitId, remote, branch, _, err = RepositoryUpdate(cominRepository, "r1", c6, c7)
	assert.Nil(t, err)
	assert.Equal(t, c9, commitId)
	assert.Equal(t, "main", branch)
	assert.Equal(t, "r1", remote)
}

func TestTestingSwitch(t *testing.T) {
	var err error
	r1Dir := t.TempDir()
	r2Dir := t.TempDir()
	cominRepositoryDir := t.TempDir()
	_, err = initRemoteRepostiory(r1Dir, true)
	r2, err := initRemoteRepostiory(r2Dir, true)
	cMain := "f8c4e82c08aa789bb7a28f16a9070026cd7eb077"
	gitConfig := types.GitConfig{
		Path: cominRepositoryDir,
		Remotes: []types.Remote{
			types.Remote{
				Name: "r1",
				URL:  r1Dir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
			types.Remote{
				Name: "r2",
				URL:  r2Dir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
		},
	}
	cominRepository, err := RepositoryOpen(gitConfig)
	assert.Nil(t, err)
	// r1/main: c1 - c2 - *c3
	// r1/testing: c1 - c2 - c3
	// r2/main: c1 - c2 - c3
	// r2/testing: c1 - c2 - c3
	commitId, remote, branch, _, err := RepositoryUpdate(cominRepository, "", "", "")
	assert.Nil(t, err)
	assert.Equal(t, cMain, commitId)
	assert.Equal(t, "main", branch)
	assert.Equal(t, "r1", remote)

	// r1/main: c1 - c2 - c3
	// r1/testing: c1 - c2 - c3
	// r2/main: c1 - c2 - c3
	// r2/testing: c1 - c2 - c3 - *c4
	c4, err := commitFile(r2, r2Dir, "testing", "file-4")
	assert.Nil(t, err)
	commitId, remote, branch, _, err = RepositoryUpdate(cominRepository, "", cMain, cMain)
	assert.Nil(t, err)
	assert.Equal(t, c4, commitId)
	assert.Equal(t, "testing", branch)
	assert.Equal(t, "r2", remote)

	// r1/main: c1 - c2 - c3
	// r1/testing: c1 - c2 - c3
	// r2/main: c1 - c2 - c3
	// r2/testing: c1 - c2 - c3 - *c4
	commitId, remote, branch, _, err = RepositoryUpdate(cominRepository, "", cMain, c4)
	assert.Nil(t, err)
	assert.Equal(t, c4, commitId)
	assert.Equal(t, "testing", branch)
	assert.Equal(t, "r2", remote)
}

func TestPreferMain(t *testing.T) {
	var err error
	r1Dir := t.TempDir()
	cominRepositoryDir := t.TempDir()
	r1, err := initRemoteRepostiory(r1Dir, true)
	cMain := "f8c4e82c08aa789bb7a28f16a9070026cd7eb077"
	gitConfig := types.GitConfig{
		Path: cominRepositoryDir,
		Remotes: []types.Remote{
			types.Remote{
				Name: "r1",
				URL:  r1Dir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
		},
	}
	cominRepository, err := RepositoryOpen(gitConfig)
	assert.Nil(t, err)
	// r1/main: c1 - c2 - *c3
	// r1/testing: c1 - c2 - c3
	commitId, remote, branch, _, err := RepositoryUpdate(cominRepository, "", "", "")
	assert.Nil(t, err)
	assert.Equal(t, cMain, commitId)
	assert.Equal(t, "main", branch)
	assert.Equal(t, "r1", remote)

	// r1/main: c1 - c2 - c3
	// r1/testing: c1 - c2 - c3 - *c4
	c4, err := commitFile(r1, r1Dir, "testing", "file-4")
	commitId, remote, branch, _, err = RepositoryUpdate(cominRepository, "", cMain, cMain)
	assert.Nil(t, err)
	assert.Equal(t, c4, commitId)
	assert.Equal(t, "testing", branch)
	assert.Equal(t, "r1", remote)

	// r1/main: c1 - c2 - c3 - *c4
	// r1/testing: c1 - c2 - c3 - c4
	c4, err = commitFile(r1, r1Dir, "main", "file-4")
	commitId, remote, branch, _, err = RepositoryUpdate(cominRepository, "", cMain, cMain)
	assert.Nil(t, err)
	assert.Equal(t, c4, commitId)
	assert.Equal(t, "main", branch)
	assert.Equal(t, "r1", remote)
}

func TestContinueIfHardReset(t *testing.T) {
	r1Dir := t.TempDir()
	r2Dir := t.TempDir()
	cominRepositoryDir := t.TempDir()
	_, _ = initRemoteRepostiory(r1Dir, true)
	r2, _ := initRemoteRepostiory(r2Dir, true)
	cMain := "f8c4e82c08aa789bb7a28f16a9070026cd7eb077"
	gitConfig := types.GitConfig{
		Path: cominRepositoryDir,
		Remotes: []types.Remote{
			types.Remote{
				Name: "r1",
				URL:  r1Dir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
			types.Remote{
				Name: "r2",
				URL:  r2Dir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
		},
	}
	cominRepository, _ := RepositoryOpen(gitConfig)
	RepositoryUpdate(cominRepository, "", "", "")

	// r1/main: c1 - c2 - ^c3
	// r1/testing: c1 - c2 - c3
	// r2/main: c1 - c2 - c3
	// r2/testing: c1 - c2 - c3 - *c4
	c4, _ := commitFile(r2, r2Dir, "testing", "file-4")
	commitId, remote, branch, _, _ := RepositoryUpdate(cominRepository, "", cMain, cMain)
	assert.Equal(t, c4, commitId)
	assert.Equal(t, "testing", branch)
	assert.Equal(t, "r2", remote)

	// r1/main: c1 - c2 - c3
	// r1/testing: c1 - c2 - c3
	// r2/main: c1 - c2 - c3 - *c4
	// r2/testing: c1 - c2 - c3 - ^c4
	c4, _ = commitFile(r2, r2Dir, "main", "file-4")
	commitId, remote, branch, _, _ = RepositoryUpdate(cominRepository, "", cMain, c4)
	assert.Equal(t, c4, commitId)
	assert.Equal(t, "main", branch)
	assert.Equal(t, "r2", remote)
}


func TestMainCommitId(t *testing.T) {
	r1Dir := t.TempDir()
	cominRepositoryDir := t.TempDir()
	r1, _ := initRemoteRepostiory(r1Dir, true)
	cMain := "f8c4e82c08aa789bb7a28f16a9070026cd7eb077"
	gitConfig := types.GitConfig{
		Path: cominRepositoryDir,
		Remotes: []types.Remote{
			types.Remote{
				Name: "r1",
				URL:  r1Dir,
				Branches: types.Branches{
					Main: types.Branch{
						Name: "main",
					},
					Testing: types.Branch{
						Name: "testing",
					},
				},
			},
		},
	}
	cominRepository, _ := RepositoryOpen(gitConfig)
	RepositoryUpdate(cominRepository, "", "", "")

	// r1/main: c1 - c2 - c3 - c4
	// r1/testing: c1 - c2 - c3 - c4 -c5
	c4, _ := commitFile(r1, r1Dir, "main", "file-4")
	commitFile(r1, r1Dir, "testing", "file-4")
	c5, _ := commitFile(r1, r1Dir, "testing", "file-5")
	commitId, remote, branch, mainCommitId, _ := RepositoryUpdate(cominRepository, "", cMain, cMain)
	assert.Equal(t, c4, mainCommitId)
	assert.Equal(t, c5, commitId)
	assert.Equal(t, "testing", branch)
	assert.Equal(t, "r1", remote)
}
