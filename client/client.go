package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-zoox/logger"
	"github.com/go-zoox/safe"
	"github.com/go-zoox/terminal/message"
	"github.com/go-zoox/websocket"
	"golang.org/x/term"
)

type Client interface {
	Connect() error
	Close() error
	Resize() error
	Send(key []byte) error
	//
	OnExit(func(code int, message string))
}

type Config struct {
	Server string
	//
	Shell       string
	Environment map[string]string
	WorkDir     string
	Command     string
	User        string
	//
	Container string
	Image     string
	//
	Username string
	Password string
	//
	Stdout io.Writer
	Stderr io.Writer
}

type client struct {
	cfg *Config
	//
	stdout io.Writer
	stderr io.Writer
	//
	closeCh   chan struct{}
	messageCh chan []byte
	//
	exitCh chan *ExitError
}

type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("%s(exit code: %d)", e.Message, e.Code)
}

func New(cfg *Config) Client {
	stdout := cfg.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	return &client{
		cfg: cfg,
		//
		stdout: stdout,
		stderr: stderr,
		//
		closeCh:   make(chan struct{}),
		messageCh: make(chan []byte),
		//
		exitCh: make(chan *ExitError),
	}
}

func (c *client) Connect() error {
	u, err := url.Parse(c.cfg.Server)
	if err != nil {
		return fmt.Errorf("invalid caas server address: %s", err)
	}
	logger.Debugf("connecting to %s", u.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	headers := http.Header{}
	if c.cfg.Username != "" && c.cfg.Password != "" {
		headers.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(c.cfg.Username+":"+c.cfg.Password))))
	}

	wc, err := websocket.NewClient(func(opt *websocket.ClientOption) {
		opt.Context = ctx
		opt.Addr = u.String()
		opt.Headers = headers
	})
	if err != nil {
		cancel()
		return err
	}

	connectCh := make(chan struct{})

	wc.OnConnect(func(conn websocket.Conn) error {
		cancel()

		// close
		go func() {
			<-c.closeCh
			conn.Close()
		}()

		if c.cfg.Image != "" {
			c.cfg.Container = "docker"
		}

		msg := &message.Message{}
		msg.SetType(message.TypeConnect)
		msg.SetConnect(&message.Connect{
			Driver: c.cfg.Container,
			//
			Shell:       c.cfg.Shell,
			Environment: c.cfg.Environment,
			WorkDir:     c.cfg.WorkDir,
			User:        c.cfg.User,
			InitCommand: c.cfg.Command,
			//
			Image: c.cfg.Image,
			//
			Username: c.cfg.Username,
			Password: c.cfg.Password,
		})

		if err := msg.Serialize(); err != nil {
			return err
		}

		if err := conn.WriteTextMessage(msg.Msg()); err != nil {
			return err
		}

		for {
			select {
			case msg := <-c.messageCh:
				if err := conn.WriteTextMessage(msg); err != nil {
					return err
				}
			}
		}
	})

	wc.OnBinaryMessage(func(conn websocket.Conn, rawMsg []byte) error {
		msg, err := message.Deserialize(rawMsg)
		if err != nil {
			c.stderr.Write([]byte(fmt.Sprintf("failed to deserialize message: %s\n", err)))
			return nil
		}

		switch msg.Type() {
		case message.TypeConnect:
			connectCh <- struct{}{}
		case message.TypeOutput:
			c.stdout.Write(msg.Output())
		case message.TypeHeartBeat:
			msg := &message.Message{}
			msg.SetType(message.TypeHeartBeat)
			if err := msg.Serialize(); err != nil {
				c.stderr.Write([]byte(fmt.Sprintf("failed to serialize message: %s\n", err)))
				return nil
			}

			if err := conn.WriteBinaryMessage(msg.Msg()); err != nil {
				c.stderr.Write([]byte(fmt.Sprintf("failed to write message: %s\n", err)))
				return nil
			}
		case message.TypeExit:
			data := msg.Exit()

			c.exitCh <- &ExitError{
				Code:    data.Code,
				Message: data.Message,
			}
		case message.TypeError:
			data := msg.Error()
			c.stderr.Write([]byte(fmt.Sprintf("error: %s\n", data.Message)))
		default:
			c.stderr.Write([]byte(fmt.Sprintf("unknown message type: %v\n", msg.Type())))
		}

		return nil
	})

	if err := wc.Connect(); err != nil {
		return err
	}

	// wait for connect
	<-connectCh

	return nil
}

func (c *client) Close() error {
	return safe.Do(func() error {
		c.closeCh <- struct{}{}
		close(c.closeCh)
		return nil
	})
}

func (c *client) Resize() error {
	fd := int(os.Stdin.Fd())
	columns, rows, err := term.GetSize(fd)
	if err != nil {
		return err
	}

	msg := &message.Message{}
	msg.SetType(message.TypeResize)
	msg.SetResize(&message.Resize{
		Columns: columns,
		Rows:    rows,
	})
	if err := msg.Serialize(); err != nil {
		return err
	}

	c.messageCh <- msg.Msg()

	return nil
}

func (c *client) Send(key []byte) error {
	msg := &message.Message{}
	msg.SetType(message.TypeKey)
	msg.SetKey(key)
	if err := msg.Serialize(); err != nil {
		return err
	}

	c.messageCh <- msg.Msg()
	return nil
}

func (c *client) OnExit(cb func(code int, message string)) {
	exitErr := <-c.exitCh
	cb(exitErr.Code, exitErr.Message)
}
