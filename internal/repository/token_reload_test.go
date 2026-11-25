package repository

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/nlewo/comin/internal/types"
)

func TestFetchReloadsTokenFile(t *testing.T) {
	seen := make(chan string, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, password, _ := r.BasicAuth()
		seen <- password
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	r, err := git.PlainInit(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateRemote(&gitConfig.RemoteConfig{Name: "origin", URLs: []string{server.URL}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "token")
	remote := types.Remote{Name: "origin", URL: server.URL, Timeout: 5, Auth: types.Auth{
		Username: "x-access-token", AccessToken: "cached-token", AccessTokenPath: path,
	}}
	for _, token := range []string{"first-token", "rotated-token"} {
		if err := os.WriteFile(path, []byte("  "+token+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := fetch(repository{Repository: r}, remote); err == nil {
			t.Fatal("fetch succeeded, want HTTP authorization error from test server")
		}
		select {
		case got := <-seen:
			if got != token {
				t.Fatalf("fetch used %q, want current file token %q", got, token)
			}
		default:
			t.Fatal("fetch did not reach test server")
		}
	}
	for _, content := range []string{"", " \n"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if err := fetch(repository{Repository: r}, remote); err == nil || !strings.Contains(err.Error(), "empty") {
			t.Fatalf("empty token should fail closed, got %v", err)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := fetch(repository{Repository: r}, remote); err == nil || !strings.Contains(err.Error(), "reading access token") {
		t.Fatalf("missing token should fail closed, got %v", err)
	}
	select {
	case <-seen:
		t.Fatal("invalid token file must not send cached credentials")
	default:
	}
}
