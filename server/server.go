package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-zoox/command/terminal"
	"github.com/go-zoox/logger"
	"github.com/go-zoox/terminal/message"
	"github.com/go-zoox/websocket"
	"github.com/go-zoox/websocket/conn"
	"github.com/go-zoox/zoox"
)

type Server interface {
	Serve() (server websocket.Server, err error)
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
	//
}

type server struct {
	cfg *Config
}

func New(cfg *Config) Server {
	return &server{
		cfg: cfg,
	}
}

func (s *server) Serve() (server websocket.Server, err error) {
	return Serve(s.cfg)
}

func Serve(cfg *Config) (server websocket.Server, err error) {
	server, err = websocket.NewServer()
	if err != nil {
		return nil, err
	}

	if cfg == nil {
		panic("terminal serve config is nil")
	}

	if cfg.DriverImage == "" {
		cfg.DriverImage = "whatwewant/zmicro:v1"
	}

	// use context, never need close when on disconnect
	// client.OnDisconnect = func() {
	// 	if session != nil {
	// 		if err := session.Close(); err != nil {
	// 			logger.Errorf("Failed to close session: %s", err)
	// 		}
	// 	}
	// }

	server.Use(func(conn conn.Conn, next func()) {
		closeCh := make(chan struct{})

		// heartbeat send
		go func() {
			logger.Infof("[ID: %s][heartbeat] created", conn.ID())

			for {
				select {
				// @TODO
				case <-time.After(13 * time.Second):
					logger.Infof("[ID: %s][heartbeat] send ...", conn.ID())

					msg := &message.Message{}
					msg.SetType(message.TypeHeartBeat)
					if err := msg.Serialize(); err != nil {
						logger.Errorf("[ID: %s]failed to serialize message: %s", conn.ID(), err)
						break
					}

					conn.WriteBinaryMessage(msg.Msg())
				case <-closeCh:
					logger.Infof("[ID: %s][heartbeat] destroyed", conn.ID())
					return
				}
			}
		}()

		next()

		closeCh <- struct{}{}
	})

	server.OnConnect(func(conn conn.Conn) error {
		return nil
	})

	server.OnClose(func(conn conn.Conn, code int, message string) error {
		logger.Infof("[ID: %s][close] code: %d, message: %s", conn.ID(), code, message)
		return nil
	})

	server.OnTextMessage(func(conn websocket.Conn, rawMsg []byte) error {
		msg, err := message.Deserialize(rawMsg)
		if err != nil {
			logger.Errorf("[ID: %s] Failed to deserialize message: %s", conn.ID(), err)
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
				logger.Errorf("[ID: %s] failed to connect: %s", conn.ID(), err)

				msg := &message.Message{}
				msg.SetType(message.TypeExit)
				msg.SetExit(&message.Exit{
					Code:    1,
					Message: fmt.Sprintf("failed to connect: %s", err.Error()),
				})
				if err := msg.Serialize(); err != nil {
					logger.Errorf("[ID: %s] failed to serialize message: %s", conn.ID(), err)
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
				logger.Errorf("ID: %s] failed to serialize message: %s", conn.ID(), err)
				return nil
			}

			conn.WriteBinaryMessage(msg.Msg())
		case message.TypeKey:
			v := conn.Get("session")
			if v == nil {
				logger.Errorf("ID: %s] failed to get session", conn.ID())
				conn.Close()
				return nil
			}
			session := v.(terminal.Terminal)

			session.Write(msg.Key())
		case message.TypeResize:
			v := conn.Get("session")
			if v == nil {
				logger.Errorf("ID: %s] failed to get session", conn.ID())
				conn.Close()
				return nil
			}
			session := v.(terminal.Terminal)
			resize := msg.Resize()
			err = session.Resize(resize.Rows, resize.Columns)
			if err != nil {
				logger.Errorf("ID: %s] Failed to resize terminal: %s", conn.ID(), err)
			}
		case message.TypeHeartBeat:
			logger.Infof("[ID: %s][heartbeat] receive ...", conn.ID())
		default:
			logger.Errorf("ID: %s] Unknown message type: %d", conn.ID(), msg.Type())
		}

		return nil
	})

	return
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

	if environments, err := ctx.Request.URL.Query()["environment"]; err {
		if cfg.Environment == nil {
			cfg.Environment = make(map[string]string)
		}

		for _, env := range environments {
			kv := strings.Split(env, "=")
			if len(kv) == 1 && kv[0] != "" {
				cfg.Environment[kv[0]] = ""
				continue
			} else if len(kv) == 2 {
				cfg.Environment[kv[0]] = kv[1]
			}
		}
	}

	if !cfg.WaitUntilFinished && ctx.Query().Get("wait_until_finished").Bool() {
		cfg.WaitUntilFinished = ctx.Query().Get("wait_until_finished").Bool()
	}
}
