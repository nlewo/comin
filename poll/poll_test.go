// These tests has to be improved a lot! Currently, i run them and
// read stdout to ensure everything is ok :/

package poll

import (
	"path/filepath"
	"io/ioutil"
	"testing"
	"time"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestPull(t *testing.T) {
	tmpDir := t.TempDir()
	remoteRepository, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Error(err)
	}
	w, err := remoteRepository.Worktree()
	if err != nil {
		t.Error(err)
	}

	filename := filepath.Join(tmpDir, "file-1")
	err = ioutil.WriteFile(filename, []byte("hello world!"), 0644)
	if err != nil {
		t.Error(err)
	}
	_, err = w.Add("file-1")
	if err != nil {
		t.Error(err)
	}
	firstCommit, err := w.Commit("First commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Error(err)
	}

	// We create a repository which contains the configuration:
	// from this repository, we try to pull the above repository.
	repositoryPath := t.TempDir()
	repository, err := git.PlainInit(repositoryPath, false)
	if err != nil {
		t.Error(err)
	}
	remoteConfigs := []config.RemoteConfig{
		config.RemoteConfig{
			Name: "origin",
			URLs: []string{tmpDir},
		},
	}
	err = manageRemotes(repository, remoteConfigs)
	if err != nil {
		t.Error(err)
	}

	err = pull(repository, remoteConfigs)
	if err != nil {
		t.Error(err)
	}


	// We add a second commit
	filename = filepath.Join(tmpDir, "file-2")
	err = ioutil.WriteFile(filename, []byte("hello world!"), 0644)
	if err != nil {
		t.Error(err)
	}
	_, err = w.Add("file-2")
	if err != nil {
		t.Error(err)
	}
	_, err = w.Commit("Second commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Error(err)
	}

	err = pull(repository, remoteConfigs)
	if err != nil {
		t.Error(err)
	}

	// Hard reset to first commit
	w.Reset(&git.ResetOptions{
		Commit: firstCommit,
		Mode: git.HardReset,
	})

	err = pull(repository, remoteConfigs)
	if err != nil {
		t.Error(err)
	}
}

func TestManageRemotes(t *testing.T) {
	r, err := git.Init(memory.NewStorage(), nil)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Create a remote
	remoteUrls := []config.RemoteConfig{
		config.RemoteConfig{
			Name: "origin",
			URLs: []string{"url1"},
		},
	}
	err = manageRemotes(r, remoteUrls)
	// FIXME: ensure remotes are expected :/
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Add a new remote
	remoteUrls = []config.RemoteConfig{
		config.RemoteConfig{
			Name: "origin",
			URLs: []string{"url1"},
		},
		config.RemoteConfig{
			Name: "fallback-1",
			URLs: []string{"url2"},
		},
	}
	err = manageRemotes(r, remoteUrls)
	// FIXME: ensure remotes are expected :/
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Update remote origin
	remoteUrls = []config.RemoteConfig{
		config.RemoteConfig{
			Name: "origin",
			URLs: []string{"url3"},
		},
		config.RemoteConfig{
			Name: "fallback-1",
			URLs: []string{"url2"},
		},
	}
	err = manageRemotes(r, remoteUrls)
	// FIXME: ensure remotes are expected :/
	if err != nil {
		t.Fatalf("%v", err)
	}
}
