package types

// The state is only used to avoid unnecessary rebuilds and doesn't
// need to be persisted.
type State struct {
	// LastOperation is the last nixos-rebuild operation
	// (basically, test or switch)
	LastOperation string
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

