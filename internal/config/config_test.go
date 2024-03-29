package config

import (
	"github.com/nlewo/comin/internal/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConfig(t *testing.T) {
	configPath := "./configuration.yaml"
	expected := types.Configuration{
		Hostname:      "machine",
		StateDir:      "/var/lib/comin",
		StateFilepath: "/var/lib/comin/state.json",
		Remotes: []types.Remote{
			{
				Name: "origin",
				URL:  "https://framagit.org/owner/infra",
				Auth: types.Auth{
					AccessToken:     "my-secret",
					AccessTokenPath: "./secret",
				},
			},
			{
				Name: "local",
				URL:  "/home/owner/git/infra",
				Auth: types.Auth{
					AccessToken:     "",
					AccessTokenPath: "",
				},
			},
		},
		HttpServer: types.HttpServer{
			Address: "127.0.0.1",
			Port:    4242,
		},
	}
	config, err := Read(configPath)
	assert.Nil(t, err)
	assert.Equal(t, expected, config)
}
