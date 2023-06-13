package config

import (
	"github.com/nlewo/comin/types"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
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
			content, err := ioutil.ReadFile(remote.Auth.AccessTokenPath)
			if err != nil {
				return config, err
			}
			config.Remotes[i].Auth.AccessToken = string(content)
		}
	}

	if config.Webhook.Address == "" {
		config.Webhook.Address = "127.0.0.1"
	}
	if config.Webhook.Port == 0 {
		config.Webhook.Port = 4242
	}
	if config.Webhook.SecretPath != "" {
		content, err := ioutil.ReadFile(config.Webhook.SecretPath)
		if err != nil {
			return config, err
		}
		config.Webhook.Secret = string(content)
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
