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
				EnvVars: []string{"GO_ZOOX_TERMINAL_SHELL"},
			},
			&cli.StringFlag{
				Name:    "init-command",
				Usage:   "the initial command",
				EnvVars: []string{"GO_ZOOX_TERMINAL_INIT_COMMAND"},
			},
			&cli.StringFlag{
				Name:    "username",
				Usage:   "Username for Basic Auth",
				EnvVars: []string{"GO_ZOOX_TERMINAL_USERNAME"},
			},
			&cli.StringFlag{
				Name:    "password",
				Usage:   "Password for Basic Auth",
				EnvVars: []string{"GO_ZOOX_TERMINAL_PASSWORD"},
			},
			&cli.StringFlag{
				Name:    "driver",
				Usage:   "Driver runtime, options: host, docker, kubernetes, ssh, default: host",
				EnvVars: []string{"GO_ZOOX_TERMINAL_DRIVER"},
				Value:   "host",
			},
			&cli.StringFlag{
				Name:    "driver-image",
				Usage:   "Driver image for driver runtime, default: whatwewant/zmicro:v1",
				EnvVars: []string{"GO_ZOOX_TERMINAL_DRIVER_IMAGE"},
				Value:   "whatwewant/zmicro:v1",
			},
			&cli.BoolFlag{
				Name:    "disable-history",
				Usage:   "Disable history",
				EnvVars: []string{"GO_ZOOX_TERMINAL_DISABLE_HISTORY"},
				Value:   false,
			},
		},
		Action: func(ctx *cli.Context) (err error) {
			s := server.NewHTTPServer(&server.HTTPServerConfig{
				Port:     ctx.Int64("port"),
				Shell:    ctx.String("shell"),
				Username: ctx.String("username"),
				Password: ctx.String("password"),
				//
				Driver:      ctx.String("container"),
				DriverImage: ctx.String("driver-image"),
				//
				InitCommand: ctx.String("init-command"),
				//
				IsHistoryDisabled: ctx.Bool("disable-history"),
			})

			return s.Run()
		},
	})
}
