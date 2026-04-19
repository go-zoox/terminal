package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-zoox/headers"
	"github.com/go-zoox/zoox"
)

const (
	headerConnectionUpgrade = "upgrade"
	headerUpgradeWebSocket  = "websocket"
)

// MiddlewareOptions configures Middleware: full terminal mounting (PTY WebSocket +
// HTML shell), with optional Basic Auth ahead of those routes.
type MiddlewareOptions struct {
	Config *Config

	// PagePath is the GET route for the xterm page (default "/").
	PagePath string
	// WSPath is the WebSocket route (default "/ws").
	WSPath string

	// BasePath is the public URL prefix used to build the wsPath embedded in HTML
	// when routes sit under Group(...). Same semantics as RegisterOptions.BasePath.
	BasePath string

	WelcomeMessage string

	// DisablePage, if true, only handles WebSocket upgrades on WSPath (no HTML route).
	DisablePage bool

	// Username and Password enable Basic Auth for requests that reach this middleware.
	// When both are set, credentials are checked before terminal handling; failed
	// auth returns 401. Skip can bypass the check for selected requests.
	Username string
	Password string
	Skip     func(ctx *zoox.Context) bool
}

// Middleware returns zoox middleware that serves the terminal: WebSocket upgrades
// on WSPath are handled by the PTY server; GET PagePath serves the embedded page.
// Other requests call Next(). Optional Basic Auth runs first when Username and
// Password are both non-empty.
//
// Do not Register the same routes elsewhere; this middleware already owns PagePath
// and WSPath.
func Middleware(opts MiddlewareOptions) zoox.HandlerFunc {
	cfg := normalizeConfig(opts.Config)

	wsSrv, err := Serve(cfg)
	if err != nil {
		panic(fmt.Errorf("terminal Middleware: websocket server: %w", err))
	}

	pagePath := opts.PagePath
	if pagePath == "" {
		pagePath = "/"
	}

	wsPath := opts.WSPath
	if wsPath == "" {
		wsPath = "/ws"
	}

	publicWS := effectivePublicWSPath(opts.BasePath, wsPath)

	var pageFn zoox.HandlerFunc
	if !opts.DisablePage {
		pageFn = PageHandler(PageConfig{
			WSPath:         publicWS,
			WelcomeMessage: opts.WelcomeMessage,
		})
	}

	return func(ctx *zoox.Context) {
		if opts.Username != "" && opts.Password != "" {
			if opts.Skip == nil || !opts.Skip(ctx) {
				user, pass, ok := ctx.Request.BasicAuth()
				if !ok {
					ctx.Set("WWW-Authenticate", `Basic realm="go-zoox"`)
					ctx.Status(401)
					return
				}
				if user != opts.Username || pass != opts.Password {
					ctx.Status(401)
					return
				}
			}
		}

		if ctx.Method == http.MethodGet && ctx.Path == wsPath && isWebSocketUpgrade(ctx) {
			wsSrv.ServeHTTP(ctx.Writer, ctx.Request)
			return
		}

		if !opts.DisablePage && ctx.Method == http.MethodGet && ctx.Path == pagePath {
			pageFn(ctx)
			return
		}

		ctx.Next()
	}
}

func isWebSocketUpgrade(ctx *zoox.Context) bool {
	connection := ctx.Header().Get(headers.Connection)
	if connection == "" {
		return false
	}
	if strings.ToLower(connection) != headerConnectionUpgrade {
		return false
	}

	upgrade := ctx.Header().Get(headers.Upgrade)
	if upgrade == "" {
		return false
	}
	if strings.ToLower(upgrade) != headerUpgradeWebSocket {
		return false
	}

	return true
}

// BasicAuthConfig configures BasicAuth (authentication only; use with Register or
// hand-wired handlers when you do not use Middleware terminal mounting).
type BasicAuthConfig struct {
	Username string
	Password string

	Skip func(ctx *zoox.Context) bool
}

// BasicAuth returns middleware that only enforces Basic Auth when both Username
// and Password are set; otherwise it is a no-op and calls Next().
func BasicAuth(cfg BasicAuthConfig) zoox.HandlerFunc {
	return func(ctx *zoox.Context) {
		if cfg.Username == "" || cfg.Password == "" {
			ctx.Next()
			return
		}
		if cfg.Skip != nil && cfg.Skip(ctx) {
			ctx.Next()
			return
		}

		user, pass, ok := ctx.Request.BasicAuth()
		if !ok {
			ctx.Set("WWW-Authenticate", `Basic realm="go-zoox"`)
			ctx.Status(401)
			return
		}

		if user != cfg.Username || pass != cfg.Password {
			ctx.Status(401)
			return
		}

		ctx.Next()
	}
}
