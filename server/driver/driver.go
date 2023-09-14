package driver

import (
	"context"

	"github.com/go-zoox/terminal/server/session"
)

type Driver interface {
	Connect(ctx context.Context) (s session.Session, err error)
}

type Config struct {
	Shell       string
	Environment map[string]string
	WorkDir     string
}
