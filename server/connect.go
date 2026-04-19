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

func connect(ctx context.Context, cfg *ConnectConfig) (session terminal.Terminal, err error) {
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

	return cmd.Terminal()
}
