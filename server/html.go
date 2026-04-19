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

//go:embed static/xterm/xterm-addon-webgl.js
var xtermAddonWebglJS string

func RenderXTerm(data zoox.H) string {
	jd, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("failed json marshal data in render XTerm: %v", err)
	}

	var b strings.Builder
	b.Grow(len(xtermCSS) + len(xtermJS) + len(xtermAddonAttachJS) + len(xtermAddonFitJS) + len(xtermAddonWebglJS) + 4096)

	b.WriteString(`<!doctype html>
<html lang="en">
	<head>
		<meta charset="utf-8">
		<title>Web Terminal</title>
		<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, viewport-fit=cover, user-scalable=no">
		<meta name="theme-color" content="#000000">
		<meta name="mobile-web-app-capable" content="yes">
		<meta name="apple-mobile-web-app-capable" content="yes">
		<meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
		<style>`)
	b.WriteString(xtermCSS)
	b.WriteString(`</style>
		<style>
			* {
				padding: 0;
				margin: 0;
				box-sizing: border-box;
			}

			html {
				height: 100%;
			}

			body {
				height: 100%;
				min-height: 100vh;
				min-height: 100dvh;
				margin: 0;
				padding: max(8px, env(safe-area-inset-top)) max(8px, env(safe-area-inset-right)) max(8px, env(safe-area-inset-bottom)) max(8px, env(safe-area-inset-left));
				background-color: #000;
				overflow: hidden;
				overscroll-behavior: none;
				touch-action: manipulation;
				-webkit-tap-highlight-color: transparent;
			}

			#terminal {
				width: 100%;
				height: 100%;
				min-height: 0;
			}

			/* xterm scrollback lives in .xterm-viewport (overflow scroll), not the page.
			   Hint vertical pan + momentum so touch doesn't fight body; isolate overscroll. */
			#terminal .xterm-viewport {
				-webkit-overflow-scrolling: touch;
				touch-action: pan-y;
				overscroll-behavior: contain;
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
		<script>`)
	b.WriteString(xtermAddonWebglJS)
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
			var narrow = typeof matchMedia !== "undefined" && matchMedia("(max-width: 480px)").matches;
			var coarsePointer = typeof matchMedia !== "undefined" && matchMedia("(pointer: coarse)").matches;
			var scrollBottomOnFocus = narrow || coarsePointer;
			var term = new Terminal({
				fontFamily: 'Menlo, Monaco, "Courier New", monospace',
				fontWeight: 400,
				fontSize: narrow ? 12 : 14,
			});
			if (typeof WebglAddon !== "undefined") {
				try {
					term.loadAddon(new WebglAddon());
				} catch (err) {
					console.warn("xterm WebGL addon failed, using canvas renderer", err);
				}
			}
			var fitAddon = new FitAddon.FitAddon();
			term.loadAddon(fitAddon);

			function scrollTermToBottomIfMobile() {
				if (!scrollBottomOnFocus) {
					return;
				}
				try {
					term.scrollToBottom();
				} catch (e) {}
			}

			function scrollIntoViewIfTyping() {
				if (!scrollBottomOnFocus || !term.textarea || document.activeElement !== term.textarea) {
					return;
				}
				requestAnimationFrame(function () {
					requestAnimationFrame(function () {
						try {
							term.scrollToBottom();
							if (term.textarea.scrollIntoView) {
								term.textarea.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'auto' });
							}
						} catch (e) {}
					});
				});
			}

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

					if (scrollBottomOnFocus && term.textarea) {
						term.textarea.addEventListener("focus", function () {
							scrollTermToBottomIfMobile();
							scrollIntoViewIfTyping();
						}, true);
					}

					if (!!config.welcomeMessage) {
						term.write(config.welcomeMessage + " \r\n")
					}

					term.focus();
					requestAnimationFrame(function () {
						scrollTermToBottomIfMobile();
						scrollIntoViewIfTyping();
					});
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

			var refitRaf = 0;
			function refitTerminal() {
				try {
					fitAddon.fit();
				} catch (e) {}
			}
			function scheduleRefitTerminal() {
				if (refitRaf) {
					return;
				}
				refitRaf = requestAnimationFrame(function () {
					refitRaf = 0;
					refitTerminal();
				});
			}

			window.addEventListener("resize", scheduleRefitTerminal, false);
			window.addEventListener("orientationchange", function () {
				setTimeout(scheduleRefitTerminal, 100);
				setTimeout(scheduleRefitTerminal, 350);
			}, false);

			if (window.visualViewport) {
				window.visualViewport.addEventListener("resize", function () {
					scheduleRefitTerminal();
					scrollIntoViewIfTyping();
				});
			}

		</script>
	</body>
</html>`)

	return b.String()
}
