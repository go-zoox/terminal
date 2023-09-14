package host

import (
	"os"
	"os/exec"
	"os/user"
	"syscall"

	"github.com/creack/pty"
	"github.com/go-zoox/core-utils/cast"
	"github.com/go-zoox/logger"
)

type ResizableHostTerminal struct {
	*os.File
	Cmd *exec.Cmd
}

func (rt *ResizableHostTerminal) Resize(rows, cols int) error {
	return pty.Setsize(rt.File, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

func (rt *ResizableHostTerminal) Wait() error {
	return rt.Cmd.Wait()
}

func (rt *ResizableHostTerminal) ExitCode() int {
	return rt.Cmd.ProcessState.ExitCode()
}

func setCmdUser(cmd *exec.Cmd, username string) error {
	userX, err := user.Lookup(username)
	if err != nil {
		return err
	}

	logger.Infof("[command] uid=%s gid=%s", userX.Uid, userX.Gid)

	uid := cast.ToInt(userX.Uid)
	gid := cast.ToInt(userX.Gid)

	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid: uint32(uid),
		Gid: uint32(gid),
	}

	cmd.Env = append(
		cmd.Env,
		"USER="+username,
		"HOME="+userX.HomeDir,
		"LOGNAME="+username,
		"UID="+userX.Uid,
		"GID="+userX.Gid,
	)

	return nil
}
