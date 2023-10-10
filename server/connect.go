package server

import (
	"io"

	"github.com/go-zoox/command"
	"github.com/go-zoox/command/errors"
	"github.com/go-zoox/command/terminal"
	"github.com/go-zoox/logger"
	"github.com/go-zoox/terminal/message"
	"github.com/go-zoox/zoox"
	"github.com/go-zoox/zoox/components/application/websocket"
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
}

func connect(ctx *zoox.Context, client *websocket.Client, cfg *ConnectConfig) (session terminal.Terminal, err error) {
	cmd, err := command.New(&command.Config{
		Context: ctx.Context(),
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

				client.Write(websocket.BinaryMessage, msg.Msg())
			} else {
				logger.Errorf("failed to wait session: %s", err)
			}
		}

		client.Disconnect()
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := session.Read(buf)
			if err != nil && err != io.EOF {
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

			client.Write(websocket.BinaryMessage, msg.Msg())

			if err == io.EOF {
				return
			}
		}
	}()

	return
}
