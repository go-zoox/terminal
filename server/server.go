package server

import (
	"fmt"

	"github.com/go-zoox/command/terminal"
	"github.com/go-zoox/logger"
	"github.com/go-zoox/terminal/message"
	"github.com/go-zoox/websocket"
	"github.com/go-zoox/websocket/conn"
	"github.com/go-zoox/zoox"
)

type Server interface {
	// Serve() zoox.WsHandlerFunc
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

// func (s *server) Serve() zoox.WsHandlerFunc {
// 	return Serve(s.cfg)
// }

func Serve(cfg *Config) func(server websocket.Server) {
	if cfg == nil {
		panic("terminal serve config is nil")
	}

	if cfg.DriverImage == "" {
		cfg.DriverImage = "whatwewant/zmicro:v1"
	}

	return func(server websocket.Server) {

		// use context, never need close when on disconnect
		// client.OnDisconnect = func() {
		// 	if session != nil {
		// 		if err := session.Close(); err != nil {
		// 			logger.Errorf("Failed to close session: %s", err)
		// 		}
		// 	}
		// }

		server.OnClose(func(conn conn.Conn) error {
			logger.Infof("[close] conn %s", conn.ID())
			return nil
		})

		server.OnTextMessage(func(conn websocket.Conn, rawMsg []byte) error {
			msg, err := message.Deserialize(rawMsg)
			if err != nil {
				logger.Errorf("Failed to deserialize message: %s", err)
				return nil
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
				withQuery(&zoox.Context{Request: conn.Request()}, connectCfg)

				session, err := connect(conn.Context(), conn, connectCfg)
				if err != nil {
					logger.Errorf("failed to connect: %s", err)

					msg := &message.Message{}
					msg.SetType(message.TypeExit)
					msg.SetExit(&message.Exit{
						Code:    1,
						Message: fmt.Sprintf("failed to connect: %s", err.Error()),
					})
					if err := msg.Serialize(); err != nil {
						logger.Errorf("failed to serialize message: %s", err)
						return nil
					}

					conn.WriteBinaryMessage(msg.Msg())

					conn.Close()
					return nil
				}

				conn.Set("session", session)

				msg := &message.Message{}
				msg.SetType(message.TypeConnect)
				// msg.SetOutput(buf[:n])
				if err := msg.Serialize(); err != nil {
					logger.Errorf("failed to serialize message: %s", err)
					return nil
				}

				conn.WriteBinaryMessage(msg.Msg())
			case message.TypeKey:
				v, err := conn.Get("session")
				if err != nil {
					logger.Errorf("failed to get session: %s", err)
					conn.Close()
					return nil
				}
				session := v.(terminal.Terminal)

				session.Write(msg.Key())
			case message.TypeResize:
				v, err := conn.Get("session")
				if err != nil {
					logger.Errorf("failed to get session: %s", err)
					conn.Close()
					return nil
				}
				session := v.(terminal.Terminal)
				resize := msg.Resize()
				err = session.Resize(resize.Rows, resize.Columns)
				if err != nil {
					logger.Errorf("Failed to resize terminal: %s", err)
				}
			default:
				logger.Errorf("Unknown message type: %d", msg.Type())
			}

			return nil
		})

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

	if !cfg.WaitUntilFinished && ctx.Query().Get("wait_until_finished").Bool() {
		cfg.WaitUntilFinished = ctx.Query().Get("wait_until_finished").Bool()
	}
}
