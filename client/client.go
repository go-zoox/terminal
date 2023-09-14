package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-zoox/logger"
	"github.com/go-zoox/terminal/message"
	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

type Client interface {
	Connect() error
	Close() error
	Resize() error
	Send(key []byte) error
	//
	OnClose() chan error
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
	conn *websocket.Conn
	//
	stdout io.Writer
	stderr io.Writer
	//
	closeCh chan error
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
		closeCh: make(chan error),
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

	conn, response, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		if response == nil || response.Body == nil {
			cancel()
			return fmt.Errorf("failed to connect at %s (error: %s)", u.String(), err)
		}

		body, errB := ioutil.ReadAll(response.Body)
		if errB != nil {
			cancel()
			return fmt.Errorf("failed to connect at %s (status: %s, error: %s)", u.String(), response.Status, err)
		}

		cancel()
		return fmt.Errorf("failed to connect at %s (status: %d, response: %s, error: %v)", u.String(), response.StatusCode, string(body), err)
	}
	c.conn = conn
	defer cancel()

	// connect
	if err := c.connect(); err != nil {
		return err
	}
	connectCh := make(chan struct{})

	// listen
	go func() {
		for {
			messageType, rawMsg, err := conn.ReadMessage()
			if err != nil {
				// if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				// 	return
				// }

				if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
					err = nil
				}

				// if websocket.IsCloseError(err, websocket.CloseGoingAway) {
				// 	return
				// }

				c.closeCh <- err
				return
			}

			if messageType != websocket.BinaryMessage {
				c.stderr.Write([]byte(fmt.Sprintf("only binary message is supported: %d\n", messageType)))
				continue
			}

			// c.stdout.Write(rawMsg)

			msg, err := message.Deserialize(rawMsg)
			if err != nil {
				c.stderr.Write([]byte(fmt.Sprintf("failed to deserialize message: %s\n", err)))
				continue
			}

			switch msg.Type() {
			case message.TypeConnect:
				connectCh <- struct{}{}
			case message.TypeOutput:
				c.stdout.Write(msg.Output())
			case message.TypeExit:
				data := msg.Exit()
				c.closeCh <- &ExitError{
					Code:    data.Code,
					Message: data.Message,
				}
			default:
				c.stderr.Write([]byte(fmt.Sprintf("unknown message type: %v\n", msg.Type())))
			}
		}
	}()

	// wait for connect
	<-connectCh

	return nil
}

func (c *client) Close() error {
	close(c.closeCh)
	return c.conn.Close()
}

func (c *client) connect() error {
	if c.cfg.Image != "" {
		c.cfg.Container = "docker"
	}

	msg := &message.Message{}
	msg.SetType(message.TypeConnect)
	msg.SetConnect(&message.Connect{
		Container: c.cfg.Container,
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

	return c.conn.WriteMessage(websocket.TextMessage, msg.Msg())
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

	return c.conn.WriteMessage(websocket.TextMessage, msg.Msg())
}

func (c *client) Send(key []byte) error {
	msg := &message.Message{}
	msg.SetType(message.TypeKey)
	msg.SetKey(key)
	if err := msg.Serialize(); err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, msg.Msg())
}

func (c *client) OnClose() chan error {
	return c.closeCh
}
