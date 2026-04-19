package server

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/go-zoox/logger"
	"github.com/go-zoox/zoox"
)

//go:embed static/xterm/xterm.css
var xtermCSS string

//go:embed static/xterm/xterm.js
var xtermJS string

//go:embed static/xterm/xterm-addon-attach.js
var xtermAddonAttachJS string

//go:embed static/xterm/xterm-addon-fit.js
var xtermAddonFitJS string

func RenderXTerm(data zoox.H) string {
	jd, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("failed json marshal data in render XTerm: %v", err)
	}

	var b strings.Builder
	b.Grow(len(xtermCSS) + len(xtermJS) + len(xtermAddonAttachJS) + len(xtermAddonFitJS) + 4096)

	b.WriteString(`<!doctype html>
<html>
	<head>
		<title>Web Terminal</title>
		<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
		<style>`)
	b.WriteString(xtermCSS)
	b.WriteString(`</style>
		<style>
			* {
				padding: 0;
				margin: 0;
				box-sizing: border-box;
			}

			body {
				margin: 8px;
				background-color: #000;
			}

			#terminal {
				width: calc(100vw - 16px);
				height: calc(100vh - 16px);
			}
		</style>
	</head>
	<body>
		<div id="terminal"></div>
		<script>`)
	b.WriteString(xtermJS)
	b.WriteString(`</script>
		<script>`)
	b.WriteString(xtermAddonAttachJS)
	b.WriteString(`</script>
		<script>`)
	b.WriteString(xtermAddonFitJS)
	b.WriteString(`</script>
		<script>
			var messageType = {
				Connect: '0',
				Key: '1',
				Resize: '2',
				Output: '6',
				HeartBeat: '8',
			};
			var config = `)
	b.Write(jd)
	b.WriteString(`;

			var url = new URL(window.location.href);
			var query = new URLSearchParams(url.search);
			var protocol = url.protocol === 'https:' ? 'wss' : 'ws';

			if (query.get('title') && document.querySelector('title')) {
				document.querySelector('title').innerText = query.get('title');
			}
			var term = new Terminal({
				fontFamily: 'Menlo, Monaco, "Courier New", monospace',
				fontWeight: 400,
				fontSize: 14,
			});
			var fitAddon = new FitAddon.FitAddon();
			term.loadAddon(fitAddon);

			var ws = new WebSocket(protocol + '://' + url.host + config.wsPath + window.location.search);
			ws.binaryType = 'arraybuffer';
			ws.onclose = () => {
				term.write('\r\n\x1b[31mConnection Closed.\x1b[m\r\n');
			};
			ws.onopen = () => {
				ws.send(messageType.Connect);
			}
			window._data = [];
			ws.onmessage = evt => {
				var rawMsg = evt.data;
				if (!(rawMsg instanceof ArrayBuffer)) {
					console.error('unknown message type, need ArrayBuffer', rawMsg);
					return;
				}

				var buffer = new Uint8Array(rawMsg);
				var typ = buffer[0];
				var payload = buffer.slice(1);

				if (typ === messageType.Output.charCodeAt(0)) {
					term.write(payload);
				} else if (typ === messageType.Connect.charCodeAt(0)) {
					term.open(document.getElementById('terminal'));
					fitAddon.fit();

					if (!!config.welcomeMessage) {
						term.write(config.welcomeMessage + " \r\n")
					}

					term.focus();
				} else if (typ === messageType.HeartBeat.charCodeAt(0)) {
					ws.send(messageType.HeartBeat + 'null');
				}
			};

			term.onResize(({ cols, rows }) => {
				ws.send(messageType.Resize + JSON.stringify({ cols, rows }));
			});

			term.onData((data) => {
				ws.send(messageType.Key + data);
			})

			window.addEventListener("resize", () => {
				fitAddon.fit()
			}, false);

		</script>
	</body>
</html>`)

	return b.String()
}
