package commands

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/go-zoox/cli"
	"github.com/go-zoox/fs"
	"github.com/go-zoox/logger"
	"github.com/go-zoox/terminal/client"
)

func RegistryClient(app *cli.MultipleProgram) {
	app.Register("client", &cli.Command{
		Name:  "client",
		Usage: "terminal client",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "server",
				Usage:    "server url",
				Aliases:  []string{"s"},
				EnvVars:  []string{"SERVER"},
				Required: true,
			},
			&cli.StringFlag{
				Name:    "username",
				Usage:   "Username for Basic Auth",
				EnvVars: []string{"USERNAME"},
			},
			&cli.StringFlag{
				Name:    "password",
				Usage:   "Password for Basic Auth",
				EnvVars: []string{"PASSWORD"},
			},
			&cli.StringFlag{
				Name:    "command",
				Usage:   "specify exec command",
				Aliases: []string{"c"},
				EnvVars: []string{"COMMAND"},
			},
			&cli.StringFlag{
				Name:  "shell",
				Usage: "specify terminal shell",
			},
			&cli.StringFlag{
				Name:    "workdir",
				Usage:   "specify terminal workdir",
				Aliases: []string{"w"},
				EnvVars: []string{"WORKDIR"},
			},
			&cli.StringFlag{
				Name:    "user",
				Usage:   "specify terminal user",
				Aliases: []string{"u"},
				// EnvVars: []string{"USER"},
			},
			&cli.StringSliceFlag{
				Name:    "env",
				Usage:   "specify terminal env",
				Aliases: []string{"e"},
				EnvVars: []string{"ENV"},
			},
			&cli.StringFlag{
				Name:    "image",
				Usage:   "specify image for container runtime",
				EnvVars: []string{"IMAGE"},
			},
			//
			&cli.StringFlag{
				Name:    "scriptfile",
				Usage:   "specify script file",
				EnvVars: []string{"SCRIPTFILE"},
			},
			&cli.StringFlag{
				Name:    "envfile",
				Usage:   `specify env file, format: key=value`,
				EnvVars: []string{"ENVFILE"},
			},
		},
		Action: func(ctx *cli.Context) (err error) {
			env := map[string]string{}
			for _, e := range ctx.StringSlice("env") {
				kv := strings.SplitN(e, "=", 2)
				if len(kv) >= 2 {
					env[kv[0]] = strings.Join(kv[1:], "=")
				} else if len(kv) == 1 {
					env[kv[0]] = ""
				}
			}

			command := ctx.String("command")
			if ctx.String("scriptfile") != "" {
				command, err = fs.ReadFileAsString(ctx.String("scriptfile"))
				if err != nil {
					return err
				}
			}

			if ctx.String("envfile") != "" {
				envfile, err := fs.ReadFileAsString(ctx.String("envfile"))
				if err != nil {
					return err
				}

				for _, e := range strings.Split(envfile, "\n") {
					if strings.TrimSpace(e) == "" {
						continue
					}
					if strings.HasPrefix(e, "#") {
						continue
					}

					kv := strings.SplitN(e, "=", 2)
					if len(kv) >= 2 {
						env[kv[0]] = strings.Join(kv[1:], "=")
					} else if len(kv) == 1 {
						env[kv[0]] = ""
					}
				}
			}

			c := client.New(&client.Config{
				Server: ctx.String("server"),
				//
				Shell:   ctx.String("shell"),
				WorkDir: ctx.String("workdir"),
				//
				Command:     command,
				Environment: env,
				User:        ctx.String("user"),
				//
				Image: ctx.String("image"),
				//
				Username: ctx.String("username"),
				Password: ctx.String("password"),
			})

			go func() {
				err := <-c.OnClose()
				if err != nil {
					if e, ok := err.(*client.ExitError); ok {
						os.Stdout.Write([]byte(e.Message + "\n"))
						os.Exit(e.Code)
						return
					}

					logger.Errorf("server disconnect by %v", err)
					os.Exit(1)
				} else {
					// logger.Errorf("client disconnect")
					os.Exit(0)
				}
			}()

			if err := c.Connect(); err != nil {
				return err
			}

			// resize
			if err := c.Resize(); err != nil {
				return err
			}

			// 监听操作系统信号
			sigWinch := make(chan os.Signal, 1)
			signal.Notify(sigWinch, syscall.SIGWINCH)
			// 启动循环来检测终端窗口大小是否发生变化
			go func() {
				for {
					select {
					case <-sigWinch:
						c.Resize()
					default:
						time.Sleep(time.Millisecond * 100)
					}
				}
			}()

			if err := keyboard.Open(); err != nil {
				return err
			}
			defer func() {
				_ = keyboard.Close()
			}()

			for {
				char, key, err := keyboard.GetKey()
				if err != nil {
					return err
				}

				// fmt.Printf("You pressed: rune:%q, key %X\r\n", char, key)
				if key == keyboard.KeyCtrlC {
					break
				}
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
				}

				// key == 0 => char
				if key == 0 {
					err = c.Send([]byte{byte(char)})
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
				} else {
					switch key {
					case keyboard.KeyF1:
						err = c.Send([]byte{0x1b, 0x4f, 0x50})
					case keyboard.KeyF2:
						err = c.Send([]byte{0x1b, 0x4f, 0x51})
					case keyboard.KeyF3:
						err = c.Send([]byte{0x1b, 0x4f, 0x52})
					case keyboard.KeyF4:
						err = c.Send([]byte{0x1b, 0x4f, 0x53})
					case keyboard.KeyF5:
						err = c.Send([]byte{0x1b, 0x5b, 0x31, 0x35, 0x7e})
					case keyboard.KeyF6:
						err = c.Send([]byte{0x1b, 0x5b, 0x31, 0x37, 0x7e})
					case keyboard.KeyF7:
						err = c.Send([]byte{0x1b, 0x5b, 0x31, 0x38, 0x7e})
					case keyboard.KeyF8:
						err = c.Send([]byte{0x1b, 0x5b, 0x31, 0x39, 0x7e})
					case keyboard.KeyF9:
						err = c.Send([]byte{0x1b, 0x5b, 0x32, 0x30, 0x7e})
					case keyboard.KeyF10:
						err = c.Send([]byte{0x1b, 0x5b, 0x32, 0x31, 0x7e})
					case keyboard.KeyF11:
						err = c.Send([]byte{0x1b, 0x5b, 0x32, 0x33, 0x7e})
					case keyboard.KeyF12:
						err = c.Send([]byte{0x1b, 0x5b, 0x32, 0x34, 0x7e})
					case keyboard.KeyInsert:
						err = c.Send([]byte{0x1b, 0x5b, 0x32, 0x7e})
					case keyboard.KeyDelete:
						err = c.Send([]byte{0x1b, 0x5b, 0x33, 0x7e})
					case keyboard.KeyHome:
						err = c.Send([]byte{0x1b, 0x5b, 0x48})
					case keyboard.KeyEnd:
						err = c.Send([]byte{0x1b, 0x5b, 0x46})
					case keyboard.KeyPgup:
						err = c.Send([]byte{0x1b, 0x5b, 0x35, 0x7e})
					case keyboard.KeyPgdn:
						err = c.Send([]byte{0x1b, 0x5b, 0x36, 0x7e})
					case keyboard.KeyArrowUp:
						err = c.Send([]byte{0x1b, 0x5b, 0x41})
					case keyboard.KeyArrowDown:
						err = c.Send([]byte{0x1b, 0x5b, 0x42})
					case keyboard.KeyArrowRight:
						err = c.Send([]byte{0x1b, 0x5b, 0x43})
					case keyboard.KeyArrowLeft:
						err = c.Send([]byte{0x1b, 0x5b, 0x44})
					default:
						err = c.Send([]byte{byte(key)})
					}

					if err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
				}
			}

			return
		},
	})
}
