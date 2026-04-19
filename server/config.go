package server

import "time"

type Config struct {
	Shell string
	User  string
	//
	Username string
	Password string
	// Driver is the Driver runtime, options: host, docker, kubernetes, ssh, default: host
	Driver      string
	DriverImage string
	//
	InitCommand string
	// WorkDir is the default working directory for new sessions when the client
	// does not send one. Query string ?workdir= still overrides when set.
	WorkDir string
	//
	IsHistoryDisabled bool
	//
	ReadOnly bool
	//
	// SessionIdleRetention is how long a PTY session remains after the WebSocket
	// disconnects, allowing reconnect before eviction. Zero means use the default
	// (60 seconds) in Serve.
	SessionIdleRetention time.Duration
}
