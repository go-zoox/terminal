package commands

import (
	"github.com/go-zoox/cli"
	"github.com/go-zoox/terminal/server"
)

func RegistryServer(app *cli.MultipleProgram) {
	app.Register("server", &cli.Command{
		Name:  "server",
		Usage: "terminal server",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "port",
				Usage:   "server port",
				Aliases: []string{"p"},
				EnvVars: []string{"PORT"},
				Value:   8838,
			},
			&cli.StringFlag{
				Name:    "shell",
				Usage:   "specify terminal shell",
				Aliases: []string{"s"},
				EnvVars: []string{"SHELL"},
			},
			&cli.StringFlag{
				Name:    "init-command",
				Usage:   "the initial command",
				EnvVars: []string{"INIT_COMMAND"},
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
				Name:    "container",
				Usage:   "Container runtime, options: host, docker, kubernetes, ssh, default: host",
				EnvVars: []string{"CONTAINER"},
				Value:   "host",
			},
		},
		Action: func(ctx *cli.Context) (err error) {
			s := server.New(&server.Config{
				Port:     ctx.Int64("port"),
				Shell:    ctx.String("shell"),
				Username: ctx.String("username"),
				Password: ctx.String("password"),
				//
				Container: ctx.String("container"),
				//
				InitCommand: ctx.String("init-command"),
			})

			return s.Run()
		},
	})
}
