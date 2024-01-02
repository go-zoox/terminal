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
	// Driver is the Driver runtime, options: host, docker, kubernetes, ssh, default: host
	Driver      string
	DriverImage string
	//
	Path string
	//
	InitCommand string
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

	app.WebSocket(cfg.Path, func(opt *zoox.WebSocketOption) {
		server, err := Serve(&Config{
			Shell:             cfg.Shell,
			Driver:            cfg.Driver,
			DriverImage:       cfg.DriverImage,
			InitCommand:       cfg.InitCommand,
			Username:          cfg.Username,
			Password:          cfg.Password,
			IsHistoryDisabled: cfg.IsHistoryDisabled,
			ReadOnly:          cfg.ReadOnly,
		})
		if err != nil {
			panic(fmt.Errorf("failed to create websocket server: %s", err))
		}

		opt.Server = server
	})

	app.Get("/", func(ctx *zoox.Context) {
		ctx.HTML(200, RenderXTerm(zoox.H{
			"wsPath": cfg.Path,
			// "welcomeMessage": "custom welcome message",
		}))
	})

	app.Get("/hi", func(ctx *zoox.Context) {
		ctx.String(200, "hi")
	})

	return app.Run(addr)
}
