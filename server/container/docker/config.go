package docker

type Config struct {
	Shell       string
	Environment map[string]string
	WorkDir     string

	Image string

	InitCommand string
}
