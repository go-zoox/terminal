package server

import (
	"io"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-zoox/command/errors"
	"github.com/go-zoox/terminal/message"
	"github.com/go-zoox/zoox"
)

func TestWithQuery(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "http://example.com/terminal?init_command=echo+hi&read_only=1&shell=%2Fbin%2Fzsh&driver=docker&workdir=%2Ftmp&user=nobody&image=alpine&environment=FOO=bar&environment=EMPTY&wait_until_finished=1", nil)
	ctx := &zoox.Context{Request: req}
	cfg := &ConnectConfig{}

	withQuery(ctx, cfg)

	want := &ConnectConfig{
		InitCommand:       "echo hi",
		ReadOnly:          true,
		Shell:             "/bin/zsh",
		Driver:            "docker",
		WorkDir:           "/tmp",
		User:              "nobody",
		Image:             "alpine",
		Environment:       map[string]string{"FOO": "bar", "EMPTY": ""},
		WaitUntilFinished: true,
	}

	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("withQuery mismatch:\n got: %#v\nwant: %#v", cfg, want)
	}
}

func TestWithQuery_preservesExistingFields(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "http://example.com/terminal?shell=%2Fother&read_only=0", nil)
	ctx := &zoox.Context{Request: req}
	cfg := &ConnectConfig{
		Shell:    "/bin/bash",
		ReadOnly: true,
	}

	withQuery(ctx, cfg)

	if cfg.Shell != "/bin/bash" {
		t.Fatalf("expected existing Shell kept, got %q", cfg.Shell)
	}
	if !cfg.ReadOnly {
		t.Fatalf("expected ReadOnly to stay true when query read_only is false")
	}
}

type mockBridgeConn struct {
	writes     [][]byte
	closeCalls int
	// errAfterNWrites causes WriteBinaryMessage to return io.EOF after that many successful writes (0 = never).
	errAfterNWrites int
}

func (m *mockBridgeConn) WriteBinaryMessage(msg []byte) error {
	if m.errAfterNWrites > 0 && len(m.writes) >= m.errAfterNWrites {
		return io.EOF
	}
	m.writes = append(m.writes, append([]byte(nil), msg...))
	return nil
}

func (m *mockBridgeConn) Close() error {
	m.closeCalls++
	return nil
}

type mockTerminal struct {
	chunks   [][]byte
	idx      int
	waitErr  error
	exitCode int

	waitCalls  int
	closeCalls int
}

func (m *mockTerminal) Read(p []byte) (int, error) {
	if m.idx >= len(m.chunks) {
		return 0, io.EOF
	}
	n := copy(p, m.chunks[m.idx])
	m.idx++
	return n, nil
}

func (m *mockTerminal) Write(p []byte) (int, error) { return len(p), nil }

func (m *mockTerminal) Close() error {
	m.closeCalls++
	return nil
}

func (m *mockTerminal) Resize(rows, cols int) error { return nil }

func (m *mockTerminal) ExitCode() int { return m.exitCode }

func (m *mockTerminal) Wait() error {
	m.waitCalls++
	return m.waitErr
}

func TestRunTerminalBridgeDelayed_outputAndExit(t *testing.T) {
	conn := &mockBridgeConn{}
	sess := &mockTerminal{
		chunks:   [][]byte{[]byte("hello")},
		exitCode: 0,
	}

	runTerminalBridgeDelayed(conn, sess, 0)

	if sess.waitCalls != 1 {
		t.Fatalf("Wait calls = %d, want 1", sess.waitCalls)
	}
	if conn.closeCalls != 1 || sess.closeCalls != 1 {
		t.Fatalf("close calls conn=%d session=%d, want 1,1", conn.closeCalls, sess.closeCalls)
	}
	if len(conn.writes) != 2 {
		t.Fatalf("writes = %d, want 2 (output + exit)", len(conn.writes))
	}

	out, err := message.Deserialize(conn.writes[0])
	if err != nil || out.Type() != message.TypeOutput || string(out.Output()) != "hello" {
		t.Fatalf("first message: err=%v type=%v body=%q", err, out.Type(), string(out.Output()))
	}

	ex, err := message.Deserialize(conn.writes[1])
	if err != nil || ex.Type() != message.TypeExit || ex.Exit().Code != 0 {
		t.Fatalf("exit message: err=%v type=%v exit=%#v", err, ex.Type(), ex.Exit())
	}
}

func TestRunTerminalBridgeDelayed_exitError(t *testing.T) {
	conn := &mockBridgeConn{}
	sess := &mockTerminal{
		chunks: [][]byte{[]byte("x")},
		waitErr: &errors.ExitError{
			Code:    42,
			Message: "command failed",
		},
		exitCode: 99,
	}

	runTerminalBridgeDelayed(conn, sess, 0)

	if len(conn.writes) != 2 {
		t.Fatalf("writes = %d, want 2", len(conn.writes))
	}
	ex, err := message.Deserialize(conn.writes[1])
	if err != nil || ex.Exit().Code != 42 || ex.Exit().Message != "command failed" {
		t.Fatalf("exit: err=%v %#v", err, ex.Exit())
	}
}

func TestRunTerminalBridgeDelayed_writeEOFinReadLoopStillWaits(t *testing.T) {
	conn := &mockBridgeConn{errAfterNWrites: 1}
	sess := &mockTerminal{
		chunks:   [][]byte{[]byte("out")},
		exitCode: 0,
	}

	runTerminalBridgeDelayed(conn, sess, 0)

	if sess.waitCalls != 1 {
		t.Fatalf("Wait should run after write EOF, got waitCalls=%d", sess.waitCalls)
	}
}
