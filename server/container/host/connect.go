package host

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/go-zoox/terminal/server/session"
)

func (h *host) Connect(ctx context.Context) (session session.Session, err error) {
	args := []string{}
	if h.cfg.InitCommand != "" {
		args = append(args, "-c", h.cfg.InitCommand)
	}

	cmd := exec.Command(h.cfg.Shell, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm")
	cmd.Dir = h.cfg.WorkDir

	if h.cfg.DisableHistory {
		cmd.Env = append(cmd.Env, "HISTFILE=/dev/null")
	}

	for k, v := range h.cfg.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	terminal, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	return &ResizableHostTerminal{
		File: terminal,
		Cmd:  cmd,
	}, nil
}
