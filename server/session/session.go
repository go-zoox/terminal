package session

import "io"

type Session interface {
	io.ReadWriteCloser

	//
	Resize(rows, cols int) error
	Wait() error

	//
	ExitCode() int
}
