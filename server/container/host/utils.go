package host

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
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
