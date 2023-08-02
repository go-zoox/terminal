package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerClient "github.com/docker/docker/client"
	"github.com/go-zoox/logger"
)

type ResizableContainerTerminal struct {
	Ctx         context.Context
	Client      *dockerClient.Client
	ContainerID string
	ReadCh      chan []byte
	Stream      *types.HijackedResponse
	//
	exitCode int
}

func (rct *ResizableContainerTerminal) Close() error {
	// if err := rct.Stream.CloseWrite(); err != nil {
	// 	return err
	// }

	rct.Stream.Close()

	return rct.Client.ContainerRemove(rct.Ctx, rct.ContainerID, types.ContainerRemoveOptions{
		Force: true,
	})
}

func (rct *ResizableContainerTerminal) Read(p []byte) (n int, err error) {
	return copy(p, <-rct.ReadCh), nil
}

func (rct *ResizableContainerTerminal) Write(p []byte) (n int, err error) {
	n, err = rct.Stream.Conn.Write(p)
	if err != nil {
		logger.Errorf("Failed to write to pty master: %s", err)
		return 0, err
	}

	return
}

func (rct *ResizableContainerTerminal) Resize(rows, cols int) error {
	return rct.Client.ContainerResize(rct.Ctx, rct.ContainerID, types.ResizeOptions{
		Height: uint(rows),
		Width:  uint(cols),
	})
}

func (rct *ResizableContainerTerminal) Wait() error {
	resultC, errC := rct.Client.ContainerWait(rct.Ctx, rct.ContainerID, container.WaitConditionNotRunning)
	select {
	case err := <-errC:
		if err != nil && err != io.EOF {
			return fmt.Errorf("container exit error: %#v", err)
		}

		logger.Infof("container exited")

	case result := <-resultC:
		if result.StatusCode != 0 {
			rct.exitCode = int(result.StatusCode)
			return fmt.Errorf("container exited with non-zero status: %d", result.StatusCode)
		}
	}

	return nil
}

func (rct *ResizableContainerTerminal) ExitCode() int {
	return rct.exitCode
}
