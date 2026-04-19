package server

import (
	"fmt"
	"strings"

	"github.com/go-zoox/zoox"
)

// PageConfig is passed to PageHandler and embedded in the HTML shell as JSON.
type PageConfig struct {
	// WSPath is the WebSocket path the browser connects to (must match how the
	// route is mounted, e.g. "/ws" or "/admin/terminal/ws").
	WSPath string
	// WelcomeMessage is shown in the terminal after connect (optional).
	WelcomeMessage string
}

// PageHandler returns a zoox handler that serves the embedded xterm HTML page.
// WSPath defaults to "/ws" if empty.
func PageHandler(page PageConfig) zoox.HandlerFunc {
	return func(ctx *zoox.Context) {
		ws := page.WSPath
		if ws == "" {
			ws = "/ws"
		}
		if !strings.HasPrefix(ws, "/") {
			ws = "/" + ws
		}
		h := zoox.H{"wsPath": ws}
		if page.WelcomeMessage != "" {
			h["welcomeMessage"] = page.WelcomeMessage
		}
		ctx.Data(200, "text/html; charset=utf-8", []byte(RenderXTerm(h)))
	}
}

// WebSocketHandler returns an option callback for zoox.RouterGroup.WebSocket.
// It wires the PTY server built from cfg. A nil cfg is treated as zero Config.
func WebSocketHandler(cfg *Config) func(opt *zoox.WebSocketOption) {
	c := normalizeConfig(cfg)
	return func(opt *zoox.WebSocketOption) {
		s, err := Serve(c)
		if err != nil {
			panic(fmt.Errorf("failed to create websocket server: %w", err))
		}
		opt.Server = s
	}
}

func normalizeConfig(cfg *Config) *Config {
	if cfg == nil {
		cfg = &Config{}
	}
	c := *cfg
	if c.Driver == "" {
		c.Driver = "host"
	}
	return &c
}
