package server

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/go-zoox/command/errors"
	"github.com/go-zoox/command/terminal"
	"github.com/go-zoox/logger"
	"github.com/go-zoox/terminal/message"
)

// SessionRegistryConfig configures idle eviction for reconnectable PTY sessions.
type SessionRegistryConfig struct {
	// TTL is how long a session may stay in the registry after its WebSocket disconnects.
	// While a client is connected (AttachWriter), this timer does not run. Each disconnect
	// starts (or restarts) the countdown; reconnect clears it. Zero or negative disables idle eviction.
	TTL time.Duration
	// SweepInterval is how often to scan for expired sessions. Zero defaults to max(TTL/4, 10s), or 10s when TTL<=0.
	SweepInterval time.Duration
}

// maxSessionReplayBytes caps how much PTY output we retain for reconnect screen restore.
const maxSessionReplayBytes = 512 * 1024

// maxKeyTailBytes retains recent client keystrokes for reconnect when the TTY did not echo them.
const maxKeyTailBytes = 2048

type sessionEntry struct {
	id      string
	session terminal.Terminal
	reg     *sessionRegistry

	mu       sync.Mutex
	ws       bridgeWSConn
	pumpOnce sync.Once
	// idleDeadline is non-zero only while no WebSocket is attached (or after transport loss);
	// the session is removed when now passes idleDeadline. Cleared in attachWriter on reconnect.
	idleDeadline time.Time

	replayMu sync.Mutex
	replay   []byte // recent PTY output (echoed keys appear here when the shell echoes)

	keyMu   sync.Mutex
	keyTail []byte // recent TypeKey payloads (for no-echo lines not present in replay)
}

func (e *sessionEntry) appendReplay(p []byte) {
	if len(p) == 0 {
		return
	}
	e.replayMu.Lock()
	e.replay = append(e.replay, p...)
	if len(e.replay) > maxSessionReplayBytes {
		e.replay = e.replay[len(e.replay)-maxSessionReplayBytes:]
	}
	e.replayMu.Unlock()
}

func (e *sessionEntry) snapshotReplay() []byte {
	e.replayMu.Lock()
	defer e.replayMu.Unlock()
	if len(e.replay) == 0 {
		return nil
	}
	out := make([]byte, len(e.replay))
	copy(out, e.replay)
	return out
}

func (e *sessionEntry) recordKeyTail(p []byte) {
	if len(p) == 0 {
		return
	}
	e.keyMu.Lock()
	e.keyTail = append(e.keyTail, p...)
	if len(e.keyTail) > maxKeyTailBytes {
		e.keyTail = e.keyTail[len(e.keyTail)-maxKeyTailBytes:]
	}
	e.keyMu.Unlock()
}

func (e *sessionEntry) snapshotKeyTail() []byte {
	e.keyMu.Lock()
	defer e.keyMu.Unlock()
	if len(e.keyTail) == 0 {
		return nil
	}
	out := make([]byte, len(e.keyTail))
	copy(out, e.keyTail)
	return out
}

func (e *sessionEntry) clearKeyTail() {
	e.keyMu.Lock()
	e.keyTail = nil
	e.keyMu.Unlock()
}

// closeAttachedWebSocket drops the pump writer and closes the WebSocket so the client cannot stay
// "connected" while the PTY or registry entry is gone.
func (e *sessionEntry) closeAttachedWebSocket() {
	e.mu.Lock()
	w := e.ws
	e.ws = nil
	e.mu.Unlock()
	if w != nil {
		_ = w.Close()
	}
}

func (e *sessionEntry) attachWriter(ws bridgeWSConn) {
	e.mu.Lock()
	e.ws = ws
	e.idleDeadline = time.Time{}
	e.mu.Unlock()
	e.startPump()
}

func (e *sessionEntry) startPump() {
	e.pumpOnce.Do(func() {
		go e.runPump()
	})
}

// runPump is the only goroutine that reads from session; it follows the current ws pointer so
// reconnect can swap the WebSocket without a second Read on the PTY.
func (e *sessionEntry) runPump() {
	const closeDelay = time.Second
	defer func() {
		if closeDelay > 0 {
			time.Sleep(closeDelay)
		}
		e.session.Close()
		e.mu.Lock()
		ws := e.ws
		e.ws = nil
		e.mu.Unlock()
		if ws != nil {
			ws.Close()
		}
		e.reg.deleteID(e.id)
	}()

	buf := make([]byte, 1024)
readLoop:
	for {
		n, err := e.session.Read(buf)
		if err != nil {
			break
		}
		e.appendReplay(buf[:n])

		msg := &message.Message{}
		msg.SetType(message.TypeOutput)
		msg.SetOutput(buf[:n])
		if err := msg.Serialize(); err != nil {
			logger.Errorf("failed to serialize message: %s", err)
			break readLoop
		}

		e.mu.Lock()
		ws := e.ws
		e.mu.Unlock()
		if ws == nil {
			continue
		}
		if err = ws.WriteBinaryMessage(msg.Msg()); err != nil {
			e.mu.Lock()
			if e.ws == ws {
				e.ws = nil
			}
			e.mu.Unlock()
			e.reg.noteDisconnected(e.id)
			// Tear down the half-dead socket; otherwise the browser stays "open" while the pump
			// no longer writes and idle TTL may later Close the PTY, leaving Write broken.
			_ = ws.Close()
		}
	}

	if err := e.session.Wait(); err != nil {
		if exitErr, ok := err.(*errors.ExitError); ok {
			logger.Errorf("[session] exit status: %d", exitErr.ExitCode())

			msg := &message.Message{}
			msg.SetType(message.TypeExit)
			msg.SetExit(&message.Exit{
				Code:    exitErr.ExitCode(),
				Message: exitErr.Error(),
			})
			if err := msg.Serialize(); err != nil {
				logger.Errorf("failed to serialize message: %s", err)
				return
			}

			e.mu.Lock()
			ws := e.ws
			e.mu.Unlock()
			if ws != nil {
				ws.WriteBinaryMessage(msg.Msg())
			}
			return
		}
		if strings.Contains(err.Error(), "signal: killed") {
			//
		} else {
			logger.Errorf("wait session error: %s", err)
		}
	}

	msg := &message.Message{}
	msg.SetType(message.TypeExit)
	msg.SetExit(&message.Exit{
		Code: e.session.ExitCode(),
	})
	if err := msg.Serialize(); err != nil {
		logger.Errorf("failed to serialize message: %s", err)
		return
	}

	e.mu.Lock()
	ws := e.ws
	e.mu.Unlock()
	if ws != nil {
		ws.WriteBinaryMessage(msg.Msg())
	}
}

type sessionRegistry struct {
	mu   sync.RWMutex
	byID map[string]*sessionEntry
	cfg  SessionRegistryConfig
}

func newSessionRegistry(cfg SessionRegistryConfig) *sessionRegistry {
	r := &sessionRegistry{
		byID: make(map[string]*sessionEntry),
		cfg:  cfg,
	}
	period := cfg.SweepInterval
	if period <= 0 && cfg.TTL > 0 {
		period = cfg.TTL / 4
		if period < 10*time.Second {
			period = 10 * time.Second
		}
	}
	if period <= 0 {
		period = 10 * time.Second
	}
	go r.sweepLoop(period)
	return r
}

func (r *sessionRegistry) sweepLoop(period time.Duration) {
	t := time.NewTicker(period)
	defer t.Stop()
	for range t.C {
		r.sweep()
	}
}

func (r *sessionRegistry) sweep() {
	if r.cfg.TTL <= 0 {
		return
	}
	now := time.Now()
	r.mu.Lock()
	for id, e := range r.byID {
		e.mu.Lock()
		d := e.idleDeadline
		e.mu.Unlock()
		if !d.IsZero() && now.After(d) {
			delete(r.byID, id)
			deadline := d
			go idleEvictSessionEntry(e, id, deadline)
		}
	}
	r.mu.Unlock()
}

func (r *sessionRegistry) deleteID(id string) {
	r.mu.Lock()
	delete(r.byID, id)
	r.mu.Unlock()
}

// noteDisconnected starts or restarts the idle TTL from now (WebSocket gone, PTY may still run).
func (r *sessionRegistry) noteDisconnected(id string) {
	if id == "" || r.cfg.TTL <= 0 {
		return
	}
	r.mu.RLock()
	e := r.byID[id]
	r.mu.RUnlock()
	if e == nil {
		return
	}
	deadline := time.Now().Add(r.cfg.TTL)
	e.mu.Lock()
	e.idleDeadline = deadline
	e.mu.Unlock()
	logger.Infof("[session %s] WebSocket disconnected: session will be evicted at %s if not reconnected (idle retention %v)", id, deadline.Format(time.RFC3339), r.cfg.TTL)
}

// Register records a new PTY session and returns its id. Call AttachWriter after the Connect ack is sent
// so the browser runs term.open before any TypeOutput frames.
func (r *sessionRegistry) Register(session terminal.Terminal) string {
	id := randomSessionID()
	e := &sessionEntry{
		id:      id,
		session: session,
		reg:     r,
	}
	r.mu.Lock()
	r.byID[id] = e
	r.mu.Unlock()
	return id
}

// WriteSessionReplay sends a snapshot of buffered PTY output as TypeOutput frames (for xterm after reconnect).
// Call after the Connect ack and before AttachWriter so the client opens the terminal before replay.
func (r *sessionRegistry) WriteSessionReplay(id string, ws bridgeWSConn) error {
	if id == "" || ws == nil {
		return nil
	}
	r.mu.RLock()
	e := r.byID[id]
	r.mu.RUnlock()
	if e == nil {
		return nil
	}
	data := e.snapshotReplay()
	const chunk = 2048
	for i := 0; i < len(data); i += chunk {
		end := i + chunk
		if end > len(data) {
			end = len(data)
		}
		msg := &message.Message{}
		msg.SetType(message.TypeOutput)
		msg.SetOutput(data[i:end])
		if err := msg.Serialize(); err != nil {
			return err
		}
		if err := ws.WriteBinaryMessage(msg.Msg()); err != nil {
			return err
		}
	}

	keys := e.snapshotKeyTail()
	if len(keys) > 0 && !bytes.HasSuffix(data, keys) {
		msg := &message.Message{}
		msg.SetType(message.TypeOutput)
		msg.SetOutput(keys)
		if err := msg.Serialize(); err != nil {
			return err
		}
		if err := ws.WriteBinaryMessage(msg.Msg()); err != nil {
			return err
		}
	}

	e.clearKeyTail()
	return nil
}

// RecordKeyTail stores recent keystrokes for optional reconnect replay when they are not echoed in replay.
func (r *sessionRegistry) RecordKeyTail(id string, p []byte) {
	if id == "" || len(p) == 0 {
		return
	}
	r.mu.RLock()
	e := r.byID[id]
	r.mu.RUnlock()
	if e == nil {
		return
	}
	e.recordKeyTail(p)
}

// AttachWriter binds the WebSocket used for binary frames and starts the output pump (once per session).
func (r *sessionRegistry) AttachWriter(id string, ws bridgeWSConn) bool {
	if id == "" || ws == nil {
		return false
	}
	r.mu.RLock()
	e := r.byID[id]
	r.mu.RUnlock()
	if e == nil {
		return false
	}
	e.attachWriter(ws)
	return true
}

// registerSessionOnly stores a session without starting the pump (tests / idle entries until Bind).
func (r *sessionRegistry) registerSessionOnly(session terminal.Terminal) string {
	id := randomSessionID()
	e := &sessionEntry{
		id:      id,
		session: session,
		reg:     r,
	}
	r.mu.Lock()
	r.byID[id] = e
	r.mu.Unlock()
	return id
}

// LookupSession returns the session for id if present and not past the post-disconnect idle deadline.
// Send the Connect ack, then call AttachWriter(id, conn).
func (r *sessionRegistry) LookupSession(id string) (terminal.Terminal, bool) {
	if id == "" {
		return nil, false
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok {
		return nil, false
	}
	e.mu.Lock()
	d := e.idleDeadline
	e.mu.Unlock()
	if r.cfg.TTL > 0 && !d.IsZero() && now.After(d) {
		delete(r.byID, id)
		dl := d
		go idleEvictSessionEntry(e, id, dl)
		return nil, false
	}
	return e.session, true
}

// Get returns a registered session or nil if missing or past the idle deadline (tests).
func (r *sessionRegistry) Get(id string) terminal.Terminal {
	if id == "" {
		return nil
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok {
		return nil
	}
	e.mu.Lock()
	d := e.idleDeadline
	e.mu.Unlock()
	if r.cfg.TTL > 0 && !d.IsZero() && now.After(d) {
		delete(r.byID, id)
		dl := d
		go idleEvictSessionEntry(e, id, dl)
		return nil
	}
	return e.session
}

// idleEvictSessionEntry logs, closes transport and PTY after idle deadline (sweep or lazy check).
func idleEvictSessionEntry(en *sessionEntry, id string, deadline time.Time) {
	logger.Infof("[session %s] idle deadline reached without reconnect: closing session and releasing PTY (scheduled eviction %s)", id, deadline.Format(time.RFC3339))
	en.closeAttachedWebSocket()
	if err := en.session.Close(); err != nil {
		logger.Errorf("[session %s] failed to close session: %v", id, err)
	} else {
		logger.Infof("[session %s] session closed", id)
	}
}

func randomSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
