package server

import (
	"fmt"

	"github.com/go-zoox/zoox"
	"github.com/go-zoox/zoox/defaults"
)

type HTTPServer interface {
	Run() error
}

type HTTPServerConfig struct {
	Port int64
	//
	Shell string
	User  string
	//
	Username string
	Password string
	// Driver is the Driver runtime, options: host, docker, kubernetes, ssh, default: host
	Driver      string
	DriverImage string
	//
	Path string
	//
	InitCommand string
	WorkDir     string
	//
	IsHistoryDisabled bool
	//
	ReadOnly bool
}

type httpServer struct {
	cfg *HTTPServerConfig
}

func NewHTTPServer(cfg *HTTPServerConfig) HTTPServer {
	if cfg.Driver == "" {
		cfg.Driver = "host"
	}

	if cfg.Path == "" {
		cfg.Path = "/ws"
	}

	return &httpServer{
		cfg: cfg,
	}
}

func (s *httpServer) Run() error {
	cfg := s.cfg
	addr := fmt.Sprintf(":%d", cfg.Port)
	app := defaults.Application()

	app.Use(Middleware(MiddlewareOptions{
		Config: &Config{
			Shell:             cfg.Shell,
			User:              cfg.User,
			Driver:            cfg.Driver,
			DriverImage:       cfg.DriverImage,
			InitCommand:       cfg.InitCommand,
			WorkDir:           cfg.WorkDir,
			Username:          cfg.Username,
			Password:          cfg.Password,
			IsHistoryDisabled: cfg.IsHistoryDisabled,
			ReadOnly:          cfg.ReadOnly,
		},
		PagePath: "/",
		WSPath:   cfg.Path,
		Username: cfg.Username,
		Password: cfg.Password,
	}))

	app.Get("/hi", func(ctx *zoox.Context) {
		ctx.String(200, "hi")
	})

	return app.Run(addr)
}
