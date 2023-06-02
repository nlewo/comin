package types

import (
	"github.com/go-git/go-git/v5"
)

type Config struct {
	Hostname  string
	StateDir  string
	StateFile string
	DryRun    bool
}

type Remote struct {
	Name string
	URL  string
	Auth Auth
}

type GitConfig struct {
	// The repository Path
	Path              string
	Remotes           []Remote
	GpgPublicKeyPaths []string
	Main              string
	Testing           string
}

type Auth struct {
	AccessToken     string
	AccessTokenPath string `yaml:"access_token_path"`
}

type Repository struct {
	Repository *git.Repository
	GitConfig  GitConfig
}

type Branch struct {
	Name      string `yaml:"name"`
	Protected bool   `yaml:"protected"`
}

type Branches struct {
	Main    Branch `yaml:"main"`
	Testing Branch `yaml:"testing"`
}

type Poller struct {
	RemoteName string `yaml:"remote_name"`
	Period     int    `yaml:"period"`
}

type Inotify struct {
	RepositoryPath string `yaml:"repository_path"`
}

type Webhook struct {
	Address    string `yaml:"address"`
	Port       int    `yaml:"port"`
	Secret     string
	SecretPath string `yaml:"secret_path"`
}

type Configuration struct {
	Hostname      string   `yaml:"hostname"`
	StateDir      string   `yaml:"state_dir"`
	StateFilepath string   `yaml:"state_filepath"`
	Remotes       []Remote `yaml:"remotes"`
	Branches      Branches `yaml:"branches"`
	Pollers       []Poller `yaml:"pollers"`
	Webhook       Webhook  `yaml:"webhook"`
	Inotify       Inotify  `yaml:"inotify"`
}
