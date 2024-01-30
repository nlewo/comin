package config

import (
	"github.com/nlewo/comin/internal/types"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

func Read(path string) (config types.Configuration, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	d := yaml.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return config, err
	}
	for i, remote := range config.Remotes {
		if remote.Auth.AccessTokenPath != "" {
			content, err := os.ReadFile(remote.Auth.AccessTokenPath)
			if err != nil {
				return config, err
			}
			config.Remotes[i].Auth.AccessToken = string(content)
		}
	}

	if config.HttpServer.Address == "" {
		config.HttpServer.Address = "127.0.0.1"
	}
	if config.HttpServer.Port == 0 {
		config.HttpServer.Port = 4242
	}
	if config.StateFilepath == "" {
		config.StateFilepath = filepath.Join(config.StateDir, "state.json")
	}
	logrus.Debugf("Config is '%#v'", config)
	return
}

func MkGitConfig(config types.Configuration) types.GitConfig {
	return types.GitConfig{
		Path:    filepath.Join(config.StateDir, "repository"),
		Remotes: config.Remotes,
	}
}
