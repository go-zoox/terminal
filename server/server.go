package server

import (
	"fmt"

	"github.com/go-zoox/logger"
	"github.com/go-zoox/terminal/message"
	"github.com/go-zoox/terminal/server/session"
	"github.com/go-zoox/zoox"
	"github.com/go-zoox/zoox/components/application/websocket"
	"github.com/go-zoox/zoox/defaults"
)

type Server interface {
	Run() error
}

type Config struct {
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

type server struct {
	cfg *Config
}

func New(cfg *Config) Server {
	if cfg.Container == "" {
		cfg.Container = "host"
	}

	if cfg.Path == "" {
		cfg.Path = "/ws"
	}

	return &server{
		cfg: cfg,
	}
}

func (s *server) Run() error {
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

	app.WebSocket(cfg.Path, func(ctx *zoox.Context, client *websocket.Client) {
		var session session.Session
		client.OnDisconnect = func() {
			if session != nil {
				session.Close()
			}
		}

		client.OnTextMessage = func(rawMsg []byte) {
			msg, err := message.Deserialize(rawMsg)
			if err != nil {
				logger.Errorf("Failed to deserialize message: %s", err)
				return
			}

			switch msg.Type() {
			case message.TypeConnect:
				data := msg.Connect()
				if data.Container == "" {
					data.Container = cfg.Container
				}
				// if data.Shell == "" {
				// 	data.Shell = cfg.Shell
				// }
				if data.InitCommand == "" {
					data.InitCommand = cfg.InitCommand
				}
				// if data.Image == "" {
				// 	data.Image = cfg.Image
				// }

				if session, err = connect(ctx, client, &ConnectConfig{
					Container:   data.Container,
					Shell:       data.Shell,
					Environment: data.Environment,
					WorkDir:     data.WorkDir,
					InitCommand: data.InitCommand,
					Image:       data.Image,
				}); err != nil {
					logger.Errorf("failed to connect: %s", err)
					return
				}

				msg := &message.Message{}
				msg.SetType(message.TypeConnect)
				// msg.SetOutput(buf[:n])
				if err := msg.Serialize(); err != nil {
					logger.Errorf("failed to serialize message: %s", err)
					return
				}

				client.WriteMessage(websocket.BinaryMessage, msg.Msg())
			case message.TypeKey:
				if session == nil {
					ctx.Logger.Errorf("session is not ready")
					client.Disconnect()
					return
				}

				session.Write(msg.Key())
			case message.TypeResize:
				if session == nil {
					ctx.Logger.Errorf("session is not ready")
					client.Disconnect()
					return
				}

				resize := msg.Resize()
				err = session.Resize(resize.Rows, resize.Columns)
				if err != nil {
					logger.Errorf("Failed to resize terminal: %s", err)
				}
			default:
				logger.Errorf("Unknown message type: %d", msg.Type())
			}
		}

	})

	app.Get("/", func(ctx *zoox.Context) {
		ctx.HTML(200, RenderXTerm(zoox.H{
			"wsPath": cfg.Path,
			// "welcomeMessage": "custom welcome message",
		}))
	})

	return app.Run(addr)
}

type Resize struct {
	Columns int `json:"cols"`
	Rows    int `json:"rows"`
}
