package server

import (
	"github.com/go-zoox/logger"
	"github.com/go-zoox/terminal/message"
	"github.com/go-zoox/terminal/server/session"
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
	// Container is the Container runtime, options: host, docker, kubernetes, ssh, default: host
	Container string
	Image     string
	//
	InitCommand string
	//
	IsHistoryDisabled bool
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

	if cfg.Image == "" {
		cfg.Image = "whatwewant/zmicro:v1"
	}

	return func(ctx *zoox.Context, client *websocket.Client) {
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
				if data.Shell == "" {
					data.Shell = cfg.Shell
				}
				if data.InitCommand == "" {
					data.InitCommand = cfg.InitCommand
				}
				if data.Image == "" {
					data.Image = cfg.Image
				}

				connectCfg := &ConnectConfig{
					Container:         data.Container,
					Shell:             data.Shell,
					Environment:       data.Environment,
					WorkDir:           data.WorkDir,
					InitCommand:       data.InitCommand,
					Image:             data.Image,
					IsHistoryDisabled: cfg.IsHistoryDisabled,
				}
				if connectCfg.InitCommand == "" && ctx.Query().Get("init_command").String() != "" {
					connectCfg.InitCommand = ctx.Query().Get("init_command").String()
				}

				if session, err = connect(ctx, client, connectCfg); err != nil {
					logger.Errorf("failed to connect: %s", err)
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

	}

}

type Resize struct {
	Columns int `json:"cols"`
	Rows    int `json:"rows"`
}
