package main

import (
	"github.com/go-zoox/cli"
	"github.com/go-zoox/terminal"
	"github.com/go-zoox/terminal/cmd/commands"
)

func main() {
	app := cli.NewMultipleProgram(&cli.MultipleProgramConfig{
		Name:    "terminal",
		Usage:   "terminal is a portable terminal",
		Version: terminal.Version,
	})

	// server
	commands.RegistryServer(app)
	// client
	commands.RegistryClient(app)

	app.Run()
}
