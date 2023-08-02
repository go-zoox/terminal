package host

import "github.com/go-zoox/terminal/server/container"

type Host interface {
	container.Container
}

type host struct {
	cfg *Config
}

func New(cfg *Config) Host {
	if cfg.Shell == "" {
		cfg.Shell = "/bin/sh"
	}

	return &host{
		cfg: cfg,
	}
}
