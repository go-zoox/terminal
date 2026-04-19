package server

import (
	"path"
	"strings"

	"github.com/go-zoox/zoox"
)

// RegisterOptions controls Register. Paths follow the same rules as
// zoox.RouterGroup.Get / WebSocket (joined with the group prefix when using Group).
type RegisterOptions struct {
	Config *Config

	// PagePath is the HTTP route for the HTML shell (default "/").
	PagePath string
	// WSPath is the WebSocket route (default "/ws").
	WSPath string

	// BasePath is the public URL prefix for HTML/JS (e.g. "/terminal" when
	// mounted under Group("/terminal", ...)). Used to compute the wsPath JSON
	// field so the browser targets the correct WebSocket URL. Leave empty when
	// registering on the root RouterGroup with absolute paths like "/ws".
	BasePath string

	WelcomeMessage string

	// DisablePage, if true, only registers the WebSocket route.
	DisablePage bool
}

// Register mounts the WebSocket handler and optionally the HTML page on g.
func Register(g *zoox.RouterGroup, opts RegisterOptions) error {
	cfg := normalizeConfig(opts.Config)

	wsPath := opts.WSPath
	if wsPath == "" {
		wsPath = "/ws"
	}

	pagePath := opts.PagePath
	if pagePath == "" {
		pagePath = "/"
	}

	if _, err := g.WebSocket(wsPath, WebSocketHandler(cfg)); err != nil {
		return err
	}

	if !opts.DisablePage {
		publicWS := effectivePublicWSPath(opts.BasePath, wsPath)
		g.Get(pagePath, PageHandler(PageConfig{
			WSPath:         publicWS,
			WelcomeMessage: opts.WelcomeMessage,
		}))
	}

	return nil
}

func effectivePublicWSPath(basePath, wsPath string) string {
	wsPath = strings.TrimSpace(wsPath)
	if wsPath == "" {
		wsPath = "/ws"
	}

	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		if !strings.HasPrefix(wsPath, "/") {
			return "/" + wsPath
		}
		return wsPath
	}

	seg := strings.Trim(wsPath, "/")
	return path.Join(basePath, seg)
}
