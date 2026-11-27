import {Terminal} from "./vendor/xterm/xterm.mjs";
import {FitAddon} from "./vendor/xterm/addon-fit.mjs";

export function openShell(target, key, mode, prefill, onConnection) {
  target.hidden = false;
  const term = new Terminal({
    fontFamily: "ui-monospace, Menlo, monospace",
    fontSize: 14,
    cursorBlink: true,
    screenReaderMode: true,
    scrollback: 2000,
    theme: {background: "#1c3029", foreground: "#eef6ee", cursor: "#eef6ee"},
  });
  const fit = new FitAddon();
  term.loadAddon(fit);
  term.open(target);
  term.textarea?.setAttribute("aria-label", "Drive terminal input");

  const address = "ws://" + location.host + "/api/terminal" + (mode === "sb" ? "?mode=sb" : "");
  const socket = new WebSocket(address, ["svalbard-terminal", key]);
  socket.binaryType = "arraybuffer";
  const encoder = new TextEncoder();
  const send = data => {
    if (socket.readyState === WebSocket.OPEN) socket.send(data);
  };
  function writeBytes(bytes) {
    for (let i = 0; i < bytes.length; i += 16384) send(bytes.subarray(i, i + 16384));
  }
  function write(data) {
    writeBytes(encoder.encode(data));
  }
  function resize() {
    if (target.clientWidth && target.clientHeight) fit.fit();
    send(JSON.stringify({type: "resize", rows: term.rows, cols: term.cols}));
  }
  const observer = new ResizeObserver(resize);
  observer.observe(target);
  term.onData(write);
  term.onBinary(data => writeBytes(Uint8Array.from(data, ch => ch.charCodeAt(0))));
  socket.addEventListener("open", () => {
    onConnection(true);
    resize();
    if (!target.closest("[hidden]")) term.focus();
  });
  socket.addEventListener("message", event => {
    term.write(new Uint8Array(event.data), () => {
      send('{"type":"ack"}');
      if (prefill) {
        write(prefill);
        prefill = "";
      }
    });
  });
  socket.addEventListener("close", () => {
    observer.disconnect();
    onConnection(false);
  });

  return {
    paste: command => {
      term.paste(command);
      term.focus();
    },
    disconnect: () => socket.close(),
    dispose: () => term.dispose(),
  };
}
