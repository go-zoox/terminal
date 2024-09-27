package commands

import (
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/go-zoox/cli"
	"github.com/go-zoox/fs"
	"github.com/go-zoox/terminal/client"

	gio "github.com/go-zoox/core-utils/io"
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

			c.OnExit(func(code int, message string) {
				os.Stdout.Write([]byte(message + "\n"))
				os.Exit(code)
			})

			if err := c.Connect(); err != nil {
				return err
			}
			defer c.Close()

			// resize
			if err := c.Resize(); err != nil {
				return err
			}

			go func() {
				sigc := make(chan os.Signal, 1)
				signal.Notify(sigc, syscall.SIGWINCH, syscall.SIGINT, syscall.SIGTERM)
				for {
					s := <-sigc
					switch s {
					case syscall.SIGWINCH:
						c.Resize()
					case syscall.SIGINT, syscall.SIGTERM:
						//
					}
				}
			}()

			// for {
			// 	reader := bufio.NewReader(os.Stdin)
			// 	str, err := reader.ReadString('\n')
			// 	if err == io.EOF {
			// 		return nil
			// 	}

			// 	fmt.Fprint(toStdin(c), str)
			// }

			if _, err := io.Copy(toStdin(c), os.Stdin); err != nil {
				return err
			}

			return nil
		},
	})
}

func toStdin(c client.Client) io.Writer {
	return gio.WriterWrapFunc(func(p []byte) (n int, err error) {
		if err := c.Send(p); err != nil {
			return 0, err
		}

		return len(p), nil
	})
}
