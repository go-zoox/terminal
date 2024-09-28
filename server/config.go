package server

type Config struct {
	Shell    string
	Username string
	Password string
	// Driver is the Driver runtime, options: host, docker, kubernetes, ssh, default: host
	Driver      string
	DriverImage string
	//
	InitCommand string
	//
	IsHistoryDisabled bool
	//
	ReadOnly bool
	//
}
