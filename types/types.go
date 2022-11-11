package types

type Config struct {
	Hostname string
	StateDir string
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

