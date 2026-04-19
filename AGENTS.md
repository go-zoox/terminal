# Agent notes — `go-zoox/terminal`

Operational context for anyone changing the WebSocket PTY server, reconnect flow, or session lifecycle.

## Command engine context must not be the WebSocket context

`command.New` wires `cfg.Context` to engine teardown. The WebSocket request context is cancelled as soon as the browser disconnects. If you pass `conn.Context()` (or any WS-bound context), the engine cancels immediately, the PTY dies, and **reconnect by `session_id` cannot work**. Use a **detached** context (this codebase uses `context.Background()`); teardown is **`terminal.Terminal.Close`**, the output pump exiting, and **session registry idle eviction**.

## Session registry and reconnect

- New PTYs are registered with an id; the Connect ack carries `session_id` for the client to store.
- **Reconnect:** client sends `TypeConnect` with the same `session_id`. Server looks up the entry, sends Connect ack, **replays buffered PTY output** (and optional key tail), then **`AttachWriter`** so a single pump keeps reading the PTY and writes to the **current** WebSocket.
- **Idle TTL:** when the WebSocket is gone, `noteDisconnected` sets a deadline (`SessionRegistryConfig.TTL`). Reconnect clears it. If the deadline passes without reconnect, the entry is evicted: log → close attached socket → **`session.Close()`** → log.
- **Sweep** runs on an interval; **`LookupSession` / `Get`** also evict lazily if the deadline has passed (same teardown helper as sweep).
- Default idle retention in **`Serve`** is **60s** when `Config.SessionIdleRetention` is zero; the **`server`** CLI exposes **`--session-idle-retention`** (duration string, e.g. `10s`, `5m`) and env **`GO_ZOOX_TERMINAL_SESSION_IDLE_RETENTION`**.

## Why “session closed” could still leave children running (fixed upstream)

`host.Terminal.Close` in **`go-zoox/command`** historically only closed the PTY master FD. **`Process.Kill()` on the shell PID** still leaves **`/bin/sh -c "…"`** children in the same process group alive. Teardown must **signal the process group** (e.g. `syscall.Kill(-pid, SIGKILL)` with fallback). Keep **`command`** at a version that includes that host `Close` behavior (tests live under `command/engine/host/terminal_close_test.go`).

## Transport and pump

- If the pump cannot write to the WebSocket, it clears the writer, calls **`noteDisconnected`**, and **closes the socket** so the client does not stay “connected” with a dead pump.
- **`TypeKey`** should surface **`session.Write`** errors and **close** the connection when the PTY is gone.

## Embedded terminal page (`server/html.go`)

- **Rendering:** use xterm’s **default canvas** renderer only — **do not** load `xterm-addon-webgl` in this page. WebGL often produces a **blank/black** terminal under **Chrome device emulation** and on many mobile GPUs.
- **Layout:** `body` is a **column flex** container and `#terminal` uses **`flex: 1; min-height: 0`** so the fit addon gets a non-zero size on narrow viewports (pure `height: 100%` chains can collapse). After Connect, **`scheduleFitAfterLayout()`** runs `fit` immediately, on `rAF`, and at **50/200/500 ms** so DevTools viewport changes settle.
- **Mobile-style disconnect UI:** on **unclean** WebSocket close, show a **modal** (Chinese copy + **「重新连接」**) when **`useMobileDisconnectOverlay()`** is true at **that moment** (not only at page load): `(max-width: 768px)` **or** `(pointer: coarse)` **or** `(hover: none)`. This matches real phones/tablets and **Chrome DevTools device emulation** (often `pointer: fine` but narrow viewport / `hover: none`). Otherwise only the red “Connection Closed” line in xterm.
- Reconnect closes the existing socket with code 1000 when needed, then opens a new WebSocket; **`sessionStorage`** still holds `session_id` so the server can restore the PTY when within idle TTL.
- **Page Visibility:** when `document.visibilityState` becomes **`visible`** and the WebSocket is **`CLOSED`**, the page **automatically** calls `openWebSocket()` (short debounce). Hides the mobile disconnect modal if it was open.

## Verification

```bash
go test ./...
```

For reconnect and TTL behavior, prefer exercising **`server`** integration manually or extending **`server`** tests; registry unit tests cover registration, expiry, and writer binding.
