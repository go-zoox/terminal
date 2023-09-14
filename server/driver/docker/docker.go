package docker

import (
	"github.com/go-zoox/terminal/server/driver"
)

type Docker interface {
	driver.Driver
}

type docker struct {
	cfg *Config
}

func New(cfg *Config) Docker {
	if cfg.Image == "" {
		cfg.Image = "whatwewant/zmicro:v1"
	}

	if cfg.Shell == "" {
		cfg.Shell = "/bin/sh"
	}

	return &docker{
		cfg: cfg,
	}
}
