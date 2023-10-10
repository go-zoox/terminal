package server

import (
	"fmt"

	"github.com/go-zoox/command/terminal"
	"github.com/go-zoox/logger"
	"github.com/go-zoox/terminal/message"
	"github.com/go-zoox/zoox"
	"github.com/go-zoox/zoox/components/application/websocket"
)

type Server interface {
	Serve() zoox.WsHandlerFunc
}

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
}

type server struct {
	cfg *Config
}

func New(cfg *Config) Server {
	return &server{
		cfg: cfg,
	}
}

func (s *server) Serve() zoox.WsHandlerFunc {
	return Serve(s.cfg)
}

func Serve(cfg *Config) zoox.WsHandlerFunc {
	if cfg == nil {
		panic("terminal serve config is nil")
	}

	if cfg.DriverImage == "" {
		cfg.DriverImage = "whatwewant/zmicro:v1"
	}

	return func(ctx *zoox.Context, client *websocket.Client) {
		var session terminal.Terminal
		client.OnDisconnect = func() {
			if session != nil {
				if err := session.Close(); err != nil {
					logger.Errorf("Failed to close session: %s", err)
				}
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
				if data.Driver == "" {
					data.Driver = cfg.Driver
				}
				if data.Shell == "" {
					data.Shell = cfg.Shell
				}
				if data.InitCommand == "" {
					data.InitCommand = cfg.InitCommand
				}
				if data.Image == "" {
					data.Image = cfg.DriverImage
				}
				// if data.User == "" {
				// 	data.User = cfg.User
				// }

				connectCfg := &ConnectConfig{
					Driver:            data.Driver,
					Shell:             data.Shell,
					Environment:       data.Environment,
					WorkDir:           data.WorkDir,
					User:              data.User,
					InitCommand:       data.InitCommand,
					Image:             data.Image,
					IsHistoryDisabled: cfg.IsHistoryDisabled,
					ReadOnly:          cfg.ReadOnly,
				}

				// @TODO
				withQuery(ctx, connectCfg)

				if session, err = connect(ctx, client, connectCfg); err != nil {
					logger.Errorf("failed to connect: %s", err)

					msg := &message.Message{}
					msg.SetType(message.TypeExit)
					msg.SetExit(&message.Exit{
						Code:    1,
						Message: fmt.Sprintf("failed to connect: %s", err.Error()),
					})
					if err := msg.Serialize(); err != nil {
						logger.Errorf("failed to serialize message: %s", err)
						return
					}

					client.Write(websocket.BinaryMessage, msg.Msg())

					client.Close()
					return
				}

				msg := &message.Message{}
				msg.SetType(message.TypeConnect)
				// msg.SetOutput(buf[:n])
				if err := msg.Serialize(); err != nil {
					logger.Errorf("failed to serialize message: %s", err)
					return
				}

				client.Write(websocket.BinaryMessage, msg.Msg())
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

	}
}

type Resize struct {
	Columns int `json:"cols"`
	Rows    int `json:"rows"`
}

func withQuery(ctx *zoox.Context, cfg *ConnectConfig) {
	if cfg.InitCommand == "" && ctx.Query().Get("init_command").String() != "" {
		cfg.InitCommand = ctx.Query().Get("init_command").String()
	}

	if !cfg.ReadOnly && ctx.Query().Get("read_only").Bool() {
		cfg.ReadOnly = ctx.Query().Get("read_only").Bool()
	}

	if cfg.Shell == "" && ctx.Query().Get("shell").String() != "" {
		cfg.Shell = ctx.Query().Get("shell").String()
	}

	if cfg.Driver == "" && ctx.Query().Get("driver").String() != "" {
		cfg.Driver = ctx.Query().Get("driver").String()
	}

	if cfg.WorkDir == "" && ctx.Query().Get("workdir").String() != "" {
		cfg.WorkDir = ctx.Query().Get("workdir").String()
	}

	if cfg.User == "" && ctx.Query().Get("user").String() != "" {
		cfg.User = ctx.Query().Get("user").String()
	}

	if cfg.Image == "" && ctx.Query().Get("image").String() != "" {
		cfg.Image = ctx.Query().Get("image").String()
	}
}
