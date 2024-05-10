package server

import (
	"encoding/json"
	"fmt"

	"github.com/go-zoox/logger"
	"github.com/go-zoox/zoox"
)

func RenderXTerm(data zoox.H) string {
	jd, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("failed json marshal data in render XTerm: %v", err)
	}

	return fmt.Sprintf(`<!doctype html>
	<html>
		<head>
			<title>Web Terminal</title>
			<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/xterm/css/xterm.css" />
			<script src="https://cdn.jsdelivr.net/npm/xterm/lib/xterm.js"></script>
			<script src="https://cdn.jsdelivr.net/npm/xterm-addon-attach"></script>
			<script src="https://cdn.jsdelivr.net/npm/xterm-addon-fit"></script>
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
			<script>
				var messageType = {
					Connect: '0',
					Key: '1',
					Resize: '2',
					Output: '6',
					HeartBeat: '8',
				};
				var config = %s;

				var url = new URL(window.location.href);
				var query = new URLSearchParams(window.location.search);
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

					// output
					if (typ === messageType.Output.charCodeAt(0)) {
						term.write(payload);
					} else if (typ === messageType.Connect.charCodeAt(0)) {
						// connect
						term.open(document.getElementById('terminal'));
						fitAddon.fit();

						if (!!config.welcomeMessage) {
							term.write(config.welcomeMessage + " \r\n")
						}

						term.focus();
					} else if (typ === messageType.HeartBeat.charCodeAt(0)) {
						ws.send(messageType.HeartBeat + 'null');

						// // send binary message
						// // const text = messageType.HeartBeat;
						// const text = messageType.HeartBeat + 'null';
						// const encoder = new TextEncoder();
						// const arrayBuffer = encoder.encode(text);
						// const uint8Array = new Uint8Array(arrayBuffer);
						// ws.send(uint8Array);
					}
				};
		
				term.onResize(({ cols, rows }) => {
					ws.send(messageType.Resize + JSON.stringify({ cols, rows }));
				});

				term.onData((data) => {
					ws.send(messageType.Key + data);
				})

				window.addEventListener("resize", () =>{
          fitAddon.fit()
        }, false);

			</script>
		</body>
	</html>`, jd)
}
