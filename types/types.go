package types

import (
	"github.com/go-git/go-git/v5"
)

// The state is only used to avoid unnecessary rebuilds and doesn't
// need to be persisted.
type State struct {
	// Operation is the last nixos-rebuild operation
	// (basically, test or switch)
	Operation string
	// The last commit that has been tried to be deployed
	CommitId string
	Deployed bool
}

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
	Remote            Remote
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
	Period int `yaml:"period"`
}

type Webhook struct {
	Address    string `yaml:"address"`
	Port       int    `yaml:"port"`
	Secret     string
	SecretPath string `yaml:"secret_path"`
}

type Configuration struct {
	Hostname string   `yaml:"hostname"`
	StateDir string   `yaml:"state_dir"`
	Remotes  []Remote `yaml:"remotes"`
	Branches Branches `yaml:"branches"`
	Poller   Poller   `yaml:"poller"`
	Webhook  Webhook  `yaml:"webhook"`
}
