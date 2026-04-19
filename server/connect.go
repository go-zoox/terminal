package server

import (
	"context"

	"github.com/go-zoox/command"
	"github.com/go-zoox/command/config"
	"github.com/go-zoox/command/terminal"
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

func connect(cfg *ConnectConfig) (session terminal.Terminal, err error) {
	// command.New attaches a goroutine: when cfg.Context is done, it calls eg.Cancel().
	// conn.Context() from the WebSocket upgrade is canceled as soon as the browser disconnects
	// (refresh, tab close), which tears down the PTY and breaks session reconnect — the user
	// then sees TypeExit (e.g. code -1) right after resize or any later message.
	// Run the engine under a detached context; cleanup is session.Close, pump exit, and registry TTL.

	cmd, err := command.New(&config.Config{
		Context: context.Background(),
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

	return cmd.Terminal()
}
