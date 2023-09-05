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
	Port     int64
	Shell    string
	Username string
	Password string
	// Container is the Container runtime, options: host, docker, kubernetes, ssh, default: host
	Container string
	//
	Path string
	//
	InitCommand string
}

type httpServer struct {
	cfg *HTTPServerConfig
}

func NewHTTPServer(cfg *HTTPServerConfig) HTTPServer {
	if cfg.Container == "" {
		cfg.Container = "host"
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

	if cfg.Username != "" && cfg.Password != "" {
		app.Use(func(ctx *zoox.Context) {
			user, pass, ok := ctx.Request.BasicAuth()
			if !ok {
				ctx.Set("WWW-Authenticate", `Basic realm="go-zoox"`)
				ctx.Status(401)
				return
			}

			if !(user == cfg.Username && pass == cfg.Password) {
				ctx.Status(401)
				return
			}

			ctx.Next()
		})
	}

	app.WebSocket(cfg.Path, Serve(&Config{
		Shell:       cfg.Shell,
		Container:   cfg.Container,
		InitCommand: cfg.InitCommand,
		Username:    cfg.Username,
		Password:    cfg.Password,
	}))

	app.Get("/", func(ctx *zoox.Context) {
		ctx.HTML(200, RenderXTerm(zoox.H{
			"wsPath": cfg.Path,
			// "welcomeMessage": "custom welcome message",
		}))
	})

	return app.Run(addr)
}
