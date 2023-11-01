# Ingress - A Easy, Powerful, Fexible Reverse Proxy

[![PkgGoDev](https://pkg.go.dev/badge/github.com/go-zoox/terminal)](https://pkg.go.dev/github.com/go-zoox/terminal)
[![Build Status](https://github.com/go-zoox/terminal/actions/workflows/release.yml/badge.svg?branch=master)](https://github.com/go-zoox/terminal/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-zoox/terminal)](https://goreportcard.com/report/github.com/go-zoox/terminal)
[![Coverage Status](https://coveralls.io/repos/github/go-zoox/terminal/badge.svg?branch=master)](https://coveralls.io/github/go-zoox/terminal?branch=master)
[![GitHub issues](https://img.shields.io/github/issues/go-zoox/terminal.svg)](https://github.com/go-zoox/terminal/issues)
[![Release](https://img.shields.io/github/tag/go-zoox/terminal.svg?label=Release)](https://github.com/go-zoox/terminal/tags)


## Installation
To install the package, run:

```bash
# with go
go install github.com/go-zoox/terminal/cmd/terminal@latest
```

if you dont have go installed, you can use the install script (zmicro package manager):

```bash
curl -o- https://raw.githubusercontent.com/zcorky/zmicro/master/install | bash

zmicro package install terminal
```

## Features
* [x] Server
  * [x] Authentication
    * [x] Basic Auth (username/password)
    * [ ] Bearer Token
    * [ ] OAuth2
    * [ ] Custom Auth Server
  * [x] Driver Runtime
    * [x] Host
    * [x] Docker
      * [x] Custom Docker Image
    * [ ] Kubernetes
    * [ ] SSH
  * [x] Read Only
  * [x] Init Command
* [x] Client
  * [x] Web Terminal/Client (Browser)
    * [x] Auth
      * [x] Basic Auth
  * [x] Command Line Client (CLI)
    * [x] Auth
      * [x] Basic Auth
    * [x] Custom Shell
    * [x] Custom Workdir
    * [x] Custom User
    * [x] Custom Env
    * [x] Env File
    * [x] Custom Docker Image
    * [x] Run Command Once
    * [x] Run Script File

## Quick Start

### Start Terminal Server

```bash
terminal server
```

### Connect Terminal with Client

```bash
terminal client --server ws://127.0.0.1:8838/ws
```

### Connect Terminal with browser

```bash
open http://127.0.0.1:8838
```

## Usage

### Server

```bash
terminal server --help

NAME:
   terminal server - terminal server

USAGE:
   terminal server [command options] [arguments...]

OPTIONS:
   --port value, -p value   server port (default: 8838) [$PORT]
   --shell value, -s value  specify terminal shell [$GO_ZOOX_TERMINAL_SHELL, $SHELL]
   --init-command value     the initial command [$GO_ZOOX_TERMINAL_INIT_COMMAND]
   --username value         Username for Basic Auth [$GO_ZOOX_TERMINAL_USERNAME]
   --password value         Password for Basic Auth [$GO_ZOOX_TERMINAL_PASSWORD]
   --driver value           Driver runtime, options: host, docker, kubernetes, ssh, default: host (default: "host") [$GO_ZOOX_TERMINAL_DRIVER]
   --driver-image value     Driver image for driver runtime, default: whatwewant/zmicro:v1 (default: "whatwewant/zmicro:v1") [$GO_ZOOX_TERMINAL_DRIVER_IMAGE]
   --disable-history        Disable history (default: false) [$GO_ZOOX_TERMINAL_DISABLE_HISTORY]
   --read-only              Read Only (default: false) [$GO_ZOOX_TERMINAL_READ_ONLY]
   --help, -h               show help
```

### Client

```bash
terminal client --help

NAME:
   terminal client - terminal client

USAGE:
   terminal client [command options] [arguments...]

OPTIONS:
   --server value, -s value                         server url [$SERVER]
   --username value                                 Username for Basic Auth [$USERNAME]
   --password value                                 Password for Basic Auth [$PASSWORD]
   --command value, -c value                        specify exec command [$COMMAND]
   --shell value                                    specify terminal shell
   --workdir value, -w value                        specify terminal workdir [$WORKDIR]
   --user value, -u value                           specify terminal user
   --env value, -e value [ --env value, -e value ]  specify terminal env [$ENV]
   --image value                                    specify image for container runtime [$IMAGE]
   --scriptfile value                               specify script file [$SCRIPTFILE]
   --envfile value                                  specify env file, format: key=value [$ENVFILE]
   --help, -h                                       show help
```


## License
GoZoox is released under the [MIT License](./LICENSE).
