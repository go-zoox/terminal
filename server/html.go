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
				Exit: '7',
				HeartBeat: '8',
			};
			var config = `)
	b.Write(jd)
	b.WriteString(`;
			var url = new URL(window.location.href);
			var query = new URLSearchParams(url.search);
			var protocol = url.protocol === 'https:' ? 'wss' : 'ws';

			var session = (function () {
				var key = 'go-zoox-terminal-session-id';

				return {
					get: function () {
						return sessionStorage.getItem(key);
					},
					set: function (value) {
						sessionStorage.setItem(key, value);
					},
					remove: function () {
						sessionStorage.removeItem(key);
					},
				};
			})();

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

			var handshakeComplete = false;

			var ws = new WebSocket(protocol + '://' + url.host + config.wsPath + window.location.search);
			ws.binaryType = 'arraybuffer';
			ws.onclose = function (ev) {
				handshakeComplete = false;
				// wasClean: normal close (e.g. tab navigation). Unclean: network or server error.
				if (ev && ev.wasClean) {
					return;
				}
				try {
					if (term.element) {
						term.write('\r\n\x1b[31mConnection Closed.\x1b[m\r\n');
					}
				} catch (e) {}
			};
			ws.onopen = function () {
				var sessionID = session.get();
				if (!!sessionID) {
					ws.send(messageType.Connect + JSON.stringify({ session_id: sessionID }));
				} else {
					ws.send(messageType.Connect);
				}
			}
			window._data = [];
			// Serialize inbound processing: Blob uses async arrayBuffer(); without chaining, later
			// ArrayBuffer frames (e.g. Output) can run before Connect and break handshake/order.
			var inboundChain = Promise.resolve();
			function queueInboundFrame(rawMsg) {
				var p;
				if (rawMsg instanceof Blob) {
					p = rawMsg.arrayBuffer();
				} else if (rawMsg instanceof ArrayBuffer) {
					p = Promise.resolve(rawMsg);
				} else {
					console.error('unknown WebSocket frame payload type', typeof rawMsg, rawMsg);
					return;
				}
				inboundChain = inboundChain.then(function () {
					return p;
				}).then(function (buf) {
					dispatchBinaryFrame(new Uint8Array(buf));
				}).catch(function (e) {
					console.error('failed to process ws frame', e);
				});
			}
			ws.onmessage = function (evt) {
				queueInboundFrame(evt.data);
			};

			function dispatchBinaryFrame(buffer) {
				var typ = buffer[0];
				var payload = buffer.slice(1);

				if (typ === messageType.Output.charCodeAt(0)) {
					if (!term.element) {
						return;
					}
					term.write(payload);
				} else if (typ === messageType.Connect.charCodeAt(0)) {
					if (!term.element) {
						term.open(document.getElementById('terminal'));
						handshakeComplete = true;
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
					} else {
						handshakeComplete = true;
						fitAddon.fit();
					}

					try {
						var data = JSON.parse(String.fromCharCode.apply(null, payload));
						if (data && data.session_id) {
							session.set(data.session_id);
						}
					} catch (e) {
						console.error('failed to parse connect data:', e)
					}

					term.focus();
					requestAnimationFrame(function () {
						scrollTermToBottomIfMobile();
						scrollIntoViewIfTyping();
					});
				} else if (typ === messageType.Exit.charCodeAt(0)) {
					handshakeComplete = false;
					try {
						var ex = JSON.parse(String.fromCharCode.apply(null, payload));
						console.warn('terminal session exit', ex);
					} catch (e) {}
				} else if (typ === messageType.HeartBeat.charCodeAt(0)) {
					if (ws.readyState === WebSocket.OPEN) {
						ws.send(messageType.HeartBeat + 'null');
					}
				}
			}

			term.onResize(({ cols, rows }) => {
				if (!handshakeComplete || ws.readyState !== WebSocket.OPEN) {
					return;
				}
				ws.send(messageType.Resize + JSON.stringify({ cols, rows }));
			});

			term.onData((data) => {
				if (!handshakeComplete || ws.readyState !== WebSocket.OPEN) {
					return;
				}
				ws.send(messageType.Key + data);
			})

			var refitRaf = 0;
			function refitTerminal() {
				if (!handshakeComplete || !term.element) {
					return;
				}
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
