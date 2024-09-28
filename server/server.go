package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-zoox/command/errors"
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
			logger.Debugf("[ID: %s][heartbeat] created", conn.ID())

			for {
				select {
				// @TODO
				case <-time.After(13 * time.Second):
					logger.Debugf("[ID: %s][heartbeat] send ...", conn.ID())

					msg := &message.Message{}
					msg.SetType(message.TypeHeartBeat)
					if err := msg.Serialize(); err != nil {
						logger.Errorf("[ID: %s]failed to serialize message: %s", conn.ID(), err)
						break
					}

					conn.WriteBinaryMessage(msg.Msg())
				case <-closeCh:
					logger.Debugf("[ID: %s][heartbeat] destroyed", conn.ID())
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

			logger.Debugf("connect cfg: %v", connectCfg)

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
			go func() {
				defer func() {
					time.Sleep(1 * time.Second)
					session.Close()
					conn.Close()
				}()

				if err := session.Wait(); err != nil {
					if exitErr, ok := err.(*errors.ExitError); ok {
						logger.Errorf("[session] exit status: %d", exitErr.ExitCode())
						// client.Write(websocket.BinaryMessage, []byte(exitErr.Error()))

						msg := &message.Message{}
						msg.SetType(message.TypeExit)
						msg.SetExit(&message.Exit{
							Code:    exitErr.ExitCode(),
							Message: exitErr.Error(),
						})
						if err := msg.Serialize(); err != nil {
							logger.Errorf("failed to serialize message: %s", err)
							return
						}

						conn.WriteBinaryMessage(msg.Msg())
						return
					} else {
						// ignore signal error, like signal: killed
						if strings.Contains(err.Error(), "signal: killed") {
							//
						} else {
							logger.Errorf("wait session error: %s", err)
						}
					}
				}

				// logger.Infof("[session] exit status: %d", session.ExitCode())
				msg := &message.Message{}
				msg.SetType(message.TypeExit)
				msg.SetExit(&message.Exit{
					Code: session.ExitCode(),
				})
				if err := msg.Serialize(); err != nil {
					logger.Errorf("failed to serialize message: %s", err)
					return
				}

				conn.WriteBinaryMessage(msg.Msg())
			}()

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
			logger.Debugf("[ID: %s][heartbeat] receive ...", conn.ID())
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

	if v := ctx.Query().Get("read_only").Bool(); !cfg.ReadOnly && v {
		cfg.ReadOnly = v
	}

	if v := ctx.Query().Get("shell").String(); cfg.Shell == "" && v != "" {
		cfg.Shell = v
	}

	if v := ctx.Query().Get("driver").String(); cfg.Driver == "" && v != "" {
		cfg.Driver = v
	}

	if v := ctx.Query().Get("workdir").String(); cfg.WorkDir == "" && v != "" {
		cfg.WorkDir = v
	}

	if v := ctx.Query().Get("user").String(); cfg.User == "" && v != "" {
		cfg.User = v
	}

	if v := ctx.Query().Get("image").String(); cfg.Image == "" && v != "" {
		cfg.Image = v
	}

	if environments, ok := ctx.Request.URL.Query()["environment"]; ok {
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

	if v := ctx.Query().Get("wait_until_finished").Bool(); !cfg.WaitUntilFinished && v {
		cfg.WaitUntilFinished = v
	}
}
