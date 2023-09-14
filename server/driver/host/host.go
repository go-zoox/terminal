package host

import "github.com/go-zoox/terminal/server/driver"

type Host interface {
	driver.Driver
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
