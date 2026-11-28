package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nlewo/comin/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	configPath := "./configuration.yaml"
	expected := types.Configuration{
		Hostname:              "machine",
		StateDir:              "/var/lib/comin",
		StateFilepath:         "/var/lib/comin/state.json",
		PostDeploymentCommand: "/some/path",
		RepositoryType:        "flake",
		RepositorySubdir:      ".",
		Remotes: []types.Remote{
			{
				Name: "origin",
				URL:  "https://framagit.org/owner/infra",
				Auth: types.Auth{
					AccessToken:     "",
					AccessTokenPath: "./secret",
					Username:        "comin",
				},
				Timeout: 300,
				Branches: types.Branches{
					Main:    types.Branch{Operation: "switch"},
					Testing: types.Branch{Operation: "test"},
				},
			},
			{
				Name: "local",
				URL:  "/home/owner/git/infra",
				Auth: types.Auth{
					AccessToken:     "",
					AccessTokenPath: "",
					Username:        "comin",
				},
				Timeout: 300,
				Branches: types.Branches{
					Main:    types.Branch{Operation: "switch"},
					Testing: types.Branch{Operation: "test"},
				},
			},
		},
		ApiServer: types.HttpServer{
			ListenAddress: "127.0.0.1",
			Port:          4242,
		},
		Exporter: types.HttpServer{
			ListenAddress: "0.0.0.0",
			Port:          4243,
		},
		Grpc: types.Grpc{
			UnixSocketPath:  "/run/comin-grpc.sock",
			UnixSocketMode:  "0770",
			UnixSocketGroup: "",
		},
		EvalTimeout:     1800,
		BuildTimeout:    1800,
		BuildRetryTimer: 30,
	}
	config, err := Read(configPath)
	assert.Nil(t, err)
	assert.Equal(t, expected, config)
}

func TestConfigWithMissingTokenFile(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "missing-token")
	configPath := filepath.Join(dir, "configuration.yaml")
	content := fmt.Sprintf("repository_type: flake\nremotes:\n  - auth:\n      access_token_path: %q\n", tokenPath)
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	config, err := Read(configPath)
	if err != nil {
		t.Fatalf("Read(%q) with missing token file = %v, want no error", configPath, err)
	}
	if len(config.Remotes) != 1 {
		t.Fatalf("Read(%q) remote count = %d, want 1", configPath, len(config.Remotes))
	}
	if got := config.Remotes[0].Auth.AccessTokenPath; got != tokenPath {
		t.Errorf("Read(%q) token path = %q, want %q", configPath, got, tokenPath)
	}
	if got := config.Remotes[0].Auth.AccessToken; got != "" {
		t.Errorf("Read(%q) cached token = %q, want empty", configPath, got)
	}
}
