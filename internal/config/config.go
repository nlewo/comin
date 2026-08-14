package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nlewo/comin/internal/types"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

func Read(path string) (config types.Configuration, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close() // nolint

	d := yaml.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return config, err
	}
	for i, remote := range config.Fetcher.Git.Remotes {
		if remote.Auth.AccessTokenPath != "" {
			content, err := os.ReadFile(remote.Auth.AccessTokenPath)
			if err != nil {
				return config, err
			}
			config.Fetcher.Git.Remotes[i].Auth.AccessToken = strings.TrimSpace(string(content))
		}
		// On GitLab and GitHub, any non blank username is working
		if remote.Auth.Username == "" {
			config.Fetcher.Git.Remotes[i].Auth.Username = "comin"
		}
		if remote.Timeout == 0 {
			config.Fetcher.Git.Remotes[i].Timeout = 300
		}
		if remote.Branches.Main.Operation == "" {
			config.Fetcher.Git.Remotes[i].Branches.Main.Operation = "switch"
		}
		if remote.Branches.Testing.Operation == "" {
			config.Fetcher.Git.Remotes[i].Branches.Testing.Operation = "test"
		}

	}

	if config.ApiServer.ListenAddress == "" {
		config.ApiServer.ListenAddress = "127.0.0.1"
	}
	if config.ApiServer.Port == 0 {
		config.ApiServer.Port = 4242
	}
	if config.Exporter.ListenAddress == "" {
		config.Exporter.ListenAddress = "0.0.0.0"
	}
	if config.Exporter.Port == 0 {
		config.Exporter.Port = 4243
	}
	if config.StateFilepath == "" {
		config.StateFilepath = filepath.Join(config.StateDir, "state.json")
	}
	if config.Fetcher.Git.RepositorySubdir == "" {
		config.Fetcher.Git.RepositorySubdir = "."
	}
	supportedRepositoryTypes := []string{"flake", "nix"}
	if !slices.Contains(supportedRepositoryTypes, config.Fetcher.Git.RepositoryType) {
		return config, fmt.Errorf("config: repository type is '%s' while it be one of '%s'", config.Fetcher.Git.RepositoryType, supportedRepositoryTypes)
	}
	supportedFetcherTypes := []string{"git", "niks3"}
	if !slices.Contains(supportedFetcherTypes, config.Fetcher.Type) {
		return config, fmt.Errorf("config: fetcher type is '%s' while it should be one of '%s'", config.Fetcher.Type, supportedFetcherTypes)
	}
	if config.Grpc.UnixSocketPath == "" {
		config.Grpc.UnixSocketPath = filepath.Join(config.StateDir, "grpc.sock")
	}
	if config.EvalTimeout == 0 {
		config.EvalTimeout = 1800
	}
	if config.BuildTimeout == 0 {
		config.BuildTimeout = 1800
	}
	logrus.Debugf("Config is '%#v'", config)
	return
}

func MkGitConfig(config types.Configuration) types.GitConfig {
	return types.GitConfig{
		Path:                  filepath.Join(config.StateDir, "repository"),
		Dir:                   config.Fetcher.Git.RepositorySubdir,
		Remotes:               config.Fetcher.Git.Remotes,
		GpgPublicKeyPaths:     config.Fetcher.Git.GpgPublicKeyPaths,
		SshAllowedSignersPath: config.Fetcher.Git.SshAllowedSignersPath,
		Submodules:            config.Fetcher.Git.Submodules,
		SystemAttr:            config.Fetcher.Git.SystemAttr,
	}
}
