package server

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/go-zoox/logger"
	"github.com/go-zoox/terminal/message"
	"github.com/go-zoox/terminal/server/driver/docker"
	"github.com/go-zoox/terminal/server/driver/host"
	"github.com/go-zoox/terminal/server/session"
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
}

func connect(ctx *zoox.Context, client *websocket.Client, cfg *ConnectConfig) (session session.Session, err error) {
	if cfg.Driver == "" {
		cfg.Driver = "host"
	}

	if cfg.Driver == "host" {
		if session, err = host.New(&host.Config{
			Shell:             cfg.Shell,
			Environment:       cfg.Environment,
			WorkDir:           cfg.WorkDir,
			User:              cfg.User,
			InitCommand:       cfg.InitCommand,
			IsHistoryDisabled: cfg.IsHistoryDisabled,
		}).Connect(ctx.Context()); err != nil {
			ctx.Logger.Errorf("[websocket] failed to connect host: %s", err)
			// client.Disconnect()
			return
		}
	} else if cfg.Driver == "docker" {
		if session, err = docker.New(&docker.Config{
			Shell:       cfg.Shell,
			Environment: cfg.Environment,
			WorkDir:     cfg.WorkDir,
			User:        cfg.User,
			InitCommand: cfg.InitCommand,
			//
			Image: cfg.Image,
		}).Connect(ctx.Context()); err != nil {
			ctx.Logger.Errorf("[websocket] failed to connect docker: %s", err)
			// client.Disconnect()
			return
		}
	} else {
		panic(fmt.Errorf("unknown mode: %s", cfg.Driver))
	}

	go func() {
		if err := session.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
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
				logger.Errorf("failed to read from session: %s", err)
				client.Write(websocket.BinaryMessage, []byte(err.Error()))
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
