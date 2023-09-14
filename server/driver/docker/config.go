package docker

type Config struct {
	Shell       string
	Environment map[string]string
	WorkDir     string

	//
	User string

	Image string

	InitCommand string
}
