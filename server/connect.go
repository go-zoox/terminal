package server

import (
	"context"
	"io"

	"github.com/go-zoox/command"
	"github.com/go-zoox/command/config"
	"github.com/go-zoox/command/terminal"
	"github.com/go-zoox/logger"
	"github.com/go-zoox/terminal/message"
	"github.com/go-zoox/websocket"
)

type ConnectConfig struct {
	Driver      string
	Shell       string
	Environment map[string]string
	WorkDir     string
	User        string
	InitCommand string
	//
	Image string
	//
	IsHistoryDisabled bool
	//
	ReadOnly bool
	//
	WaitUntilFinished bool
}

func connect(ctx context.Context, conn websocket.Conn, cfg *ConnectConfig) (session terminal.Terminal, err error) {
	if cfg.WaitUntilFinished {
		ctx = context.Background()
	}

	cmd, err := command.New(&config.Config{
		Context: ctx,
		//
		Engine:            cfg.Driver,
		Command:           cfg.InitCommand,
		Environment:       cfg.Environment,
		WorkDir:           cfg.WorkDir,
		User:              cfg.User,
		Shell:             cfg.Shell,
		Image:             cfg.Image,
		IsHistoryDisabled: cfg.IsHistoryDisabled,
		ReadOnly:          cfg.ReadOnly,
	})
	if err != nil {
		return nil, err
	}

	session, err = cmd.Terminal()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := session.Read(buf)
			if err != nil {
				// logger.Errorf("failed to read from session: %s", err)
				// client.Write(websocket.BinaryMessage, []byte(err.Error()))
				return
			}

			msg := &message.Message{}
			msg.SetType(message.TypeOutput)
			msg.SetOutput(buf[:n])
			if err := msg.Serialize(); err != nil {
				logger.Errorf("failed to serialize message: %s", err)
				return
			}

			if err = conn.WriteBinaryMessage(msg.Msg()); err == io.EOF {
				return
			}
		}
	}()

	return
}
