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
	for i, remote := range config.Remotes {
		if remote.Auth.AccessTokenPath != "" {
			content, err := os.ReadFile(remote.Auth.AccessTokenPath)
			if err != nil {
				return config, err
			}
			config.Remotes[i].Auth.AccessToken = strings.TrimSpace(string(content))
		}
		// On GitLab and GitHub, any non blank username is working
		if remote.Auth.Username == "" {
			config.Remotes[i].Auth.Username = "comin"
		}
		if remote.Timeout == 0 {
			config.Remotes[i].Timeout = 300
		}
		if remote.Branches.Main.Operation == "" {
			config.Remotes[i].Branches.Main.Operation = "switch"
		}
		if remote.Branches.Testing.Operation == "" {
			config.Remotes[i].Branches.Testing.Operation = "test"
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
	if config.RepositorySubdir == "" {
		config.RepositorySubdir = "."
	}
	supportedRepositoryTypes := []string{"flake", "nix"}
	if !slices.Contains(supportedRepositoryTypes, config.RepositoryType) {
		return config, fmt.Errorf("config: repository type is '%s' while it be one of '%s'", config.RepositoryType, supportedRepositoryTypes)
	}
	if config.ExecutorConfig.Type == "" {
		config.ExecutorConfig.Type = "nix"
	}
	supportedExecutorTypes := []string{"nix", "garnix", "hydra"}
	if !slices.Contains(supportedExecutorTypes, config.ExecutorConfig.Type) {
		return config, fmt.Errorf("config: executor type is '%s' while it must be one of '%s'", config.ExecutorConfig.Type, supportedExecutorTypes)
	}
	if config.ExecutorConfig.Type == "garnix" && config.RepositoryType != "flake" {
		return config, fmt.Errorf("config: executor type 'garnix' requires repository_type 'flake', got '%s'", config.RepositoryType)
	}
	if config.ExecutorConfig.Type == "hydra" {
		if config.RepositoryType != "flake" {
			return config, fmt.Errorf("config: executor type 'hydra' requires repository_type 'flake', got '%s'", config.RepositoryType)
		}
		if config.ExecutorConfig.HydraConfig.BaseUrl == "" {
			return config, fmt.Errorf("config: executor type 'hydra' requires executor.hydra.base_url to be set")
		}
		if config.ExecutorConfig.HydraConfig.Project == "" {
			return config, fmt.Errorf("config: executor type 'hydra' requires executor.hydra.project to be set")
		}
		if config.ExecutorConfig.HydraConfig.Jobset == "" {
			return config, fmt.Errorf("config: executor type 'hydra' requires executor.hydra.jobset to be set")
		}
		if config.ExecutorConfig.HydraConfig.JobName == "" {
			config.ExecutorConfig.HydraConfig.JobName = config.Hostname
		}
		if config.ExecutorConfig.HydraConfig.RetryInterval == 0 {
			config.ExecutorConfig.HydraConfig.RetryInterval = 60
		}
		if config.ExecutorConfig.HydraConfig.MaxEvalPages == 0 {
			config.ExecutorConfig.HydraConfig.MaxEvalPages = 5
		}
	}
	if config.Grpc.UnixSocketPath == "" {
		config.Grpc.UnixSocketPath = filepath.Join(config.StateDir, "grpc.sock")
	}
	logrus.Debugf("Config is '%#v'", config)
	return
}

func MkGitConfig(config types.Configuration) types.GitConfig {
	return types.GitConfig{
		Path:              filepath.Join(config.StateDir, "repository"),
		Dir:               config.RepositorySubdir,
		Remotes:           config.Remotes,
		GpgPublicKeyPaths: config.GpgPublicKeyPaths,
		Submodules:        config.Submodules,
	}
}
