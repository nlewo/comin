package types

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
	Hostname string
	StateDir string
	StateFile string
	GitConfig GitConfig
	DryRun bool
}

type Remote struct {
	Name string
	URL string
	Auth Auth
}

type GitConfig struct {
	// The repository Path
	Path string
	Remote Remote
	Remotes []Remote
	GpgPublicKeyPaths []string
	Main string
	Testing string
}

type Auths map[string]Auth

type Auth struct {
	AccessToken string
}
