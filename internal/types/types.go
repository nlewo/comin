package types

type Remote struct {
	Name     string
	URL      string
	Auth     Auth
	Branches Branches `yaml:"branches"`
	Timeout  int      `yaml:"timeout"`
	// The period to poll the remote in second
	Poller Poller `yaml:"poller"`
}

type Poller struct {
	Period int `yaml:"period"`
}

type GitConfig struct {
	// The repository Path
	Path              string
	Remotes           []Remote
	GpgPublicKeyPaths []string
}

type Auth struct {
	AccessToken     string
	AccessTokenPath string `yaml:"access_token_path"`
}

type Branch struct {
	Name string `yaml:"name"`
	// TODO: use it
	Protected bool `yaml:"protected"`
}

type Branches struct {
	Main    Branch `yaml:"main"`
	Testing Branch `yaml:"testing"`
}

type HttpServer struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

type Configuration struct {
	Hostname      string     `yaml:"hostname"`
	StateDir      string     `yaml:"state_dir"`
	StateFilepath string     `yaml:"state_filepath"`
	Remotes       []Remote   `yaml:"remotes"`
	HttpServer    HttpServer `yaml:"http_server"`
}
