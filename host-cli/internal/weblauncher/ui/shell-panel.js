/* The drive terminal: unlock with the private launch link, then open a shell.
   Loads after frontpage.js, before the inventory request resolves. */
let terminalKey = readTerminalKey();
const terminals = {regular: {}, sb: {}};
function readTerminalKey() {
  const hash = location.hash.slice(1);
  const key = hash.startsWith("terminal-key=") ? hash.slice("terminal-key=".length) : "";
  if (key) history.replaceState(null, "", location.pathname + location.search);
  try {
    if (key) sessionStorage.setItem("svalbard-terminal-key", key);
    return key || sessionStorage.getItem("svalbard-terminal-key") || "";
  } catch {
    return key;
  }
}

function selectedTerminal() {
  return terminals[selection?.terminalMode === "sb" ? "sb" : "regular"];
}

function renderTerminal() {
  const state = selectedTerminal();
  const command = byId("terminal-command");
  const start = byId("terminal-start");
  command.textContent = selection?.command || "";
  command.hidden = !command.textContent || Boolean(state.session);
  start.textContent = state === terminals.sb ? "Start sb shell" : "Start terminal";
  start.hidden = !terminalKey || Boolean(state.session);
  byId("terminal-unlock").hidden = Boolean(terminalKey);
  start.disabled = !terminalKey || state.loading;
  byId("terminal-stop").hidden = !state.session;
  byId("terminal-paste").hidden = !state.session || !selection?.command;
  for (const terminal of Object.values(terminals)) {
    if (terminal.screen) terminal.screen.hidden = state !== terminal;
  }
  const message = terminalKey ? state.message : "";
  note(byId("terminal-status"), message || "", state.failed);
}

byId("terminal-unlock").addEventListener("submit", async event => {
  event.preventDefault();
  const input = byId("launch-link");
  try {
    const link = new URL(input.value.trim());
    const key = new URLSearchParams(link.hash.slice(1)).get("terminal-key");
    if (link.origin !== location.origin || !key) throw Error();
    const res = await fetch("/api/unlock", {method:"POST", headers:{"X-Svalbard-Request":"1", "X-Svalbard-Key":key}});
    if (!res.ok) throw Error();
    terminalKey = key;
    input.value = "";
    try { sessionStorage.setItem("svalbard-terminal-key", key); } catch {}
    note(byId("unlock-error"), "");
    renderRoute();
    byId("terminal-start").focus();
  } catch {
    note(byId("unlock-error"), "Use the private link printed by the current ./run web session.", true);
    input.select();
  }
});

async function startTerminal() {
  const state = selectedTerminal();
  if (!terminalKey || state.loading || state.session) return;
  const mode = selection?.terminalMode;
  const prefill = selection?.command || "";
  state.loading = true;
  state.failed = false;
  state.message = "Connecting…";
  renderTerminal();
  try {
    const {openShell} = await import("/terminal.js");
    state.previous?.dispose();
    state.previous = undefined;
    if (!state.screen) {
      state.screen = node("div", "terminal-screen");
      byId("terminal-screens").append(state.screen);
    }
    state.session = openShell(state.screen, terminalKey, mode, prefill, connected => {
      state.message = connected ? "" : "Terminal disconnected.";
      if (!connected) {
        state.previous = state.session;
        state.session = undefined;
      }
      renderTerminal();
    });
  } catch (error) {
    state.message = error.message;
    state.failed = true;
  } finally {
    state.loading = false;
    renderTerminal();
  }
}
byId("terminal-start").addEventListener("click", startTerminal);
byId("terminal-stop").addEventListener("click", () => selectedTerminal().session?.disconnect());
byId("terminal-paste").addEventListener("click", () => selectedTerminal().session?.paste(selection.command));
window.addEventListener("pagehide", () => {
  for (const state of Object.values(terminals)) state.session?.disconnect();
});
