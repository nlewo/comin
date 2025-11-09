package config

import (
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
		FlakeSubdirectory:     ".",
		Remotes: []types.Remote{
			{
				Name: "origin",
				URL:  "https://framagit.org/owner/infra",
				Auth: types.Auth{
					AccessToken:     "my-secret",
					AccessTokenPath: "./secret",
				},
				Timeout: 300,
			},
			{
				Name: "local",
				URL:  "/home/owner/git/infra",
				Auth: types.Auth{
					AccessToken:     "",
					AccessTokenPath: "",
				},
				Timeout: 300,
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
			UnixSocketPath: "/var/lib/comin/grpc.sock",
		},
	}
	config, err := Read(configPath)
	assert.Nil(t, err)
	assert.Equal(t, expected, config)
}
