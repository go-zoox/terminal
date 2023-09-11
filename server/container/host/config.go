package host

type Config struct {
	Shell       string
	Environment map[string]string
	WorkDir     string

	InitCommand string

	//
	IsHistoryDisabled bool
}
