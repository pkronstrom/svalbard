// Loads a page in headless Chromium and Firefox with the network off and
// reports console errors, uncaught exceptions, failed loads and any request
// that left the computer. Development-time only; run it through
// scripts/html-check.sh, which finds the browsers.
//
//   node scripts/html-check.mjs [options] <page.html>
//
//   --browser chromium|firefox|both   (default both)
//   --expect '<js expression>'        must become truthy within --wait seconds
//   --wait N                          seconds to wait for --expect (default 10)
//   --serve DIR                       serve DIR on 127.0.0.1 (with range
//                                     requests) and load the page from there
//   --click SELECTOR --download FILE  click, wait for the download, save it
//                                     (Chromium only)
//   --chromium PATH, --firefox PATH   browser executables
//
// Exits 0 when every browser passes, 1 when one fails, 77 when no browser
// was found.
import { spawn } from "node:child_process";
import fs from "node:fs";
import http from "node:http";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";

const args = process.argv.slice(2);
const opts = { browser: "both", wait: 10 };
for (let i = 0; i < args.length; i++) {
  const a = args[i];
  const next = () => args[++i];
  if (a === "--browser") opts.browser = next();
  else if (a === "--expect") opts.expect = next();
  else if (a === "--wait") opts.wait = Number(next());
  else if (a === "--serve") opts.serve = path.resolve(next());
  else if (a === "--click") opts.click = next();
  else if (a === "--download") opts.download = path.resolve(next());
  else if (a === "--chromium") opts.chromium = next();
  else if (a === "--firefox") opts.firefox = next();
  else if (a.startsWith("--")) { console.error(`unknown option ${a}`); process.exit(2); }
  else opts.page = path.resolve(a);
}
if (!opts.page || !fs.existsSync(opts.page)) {
  console.error("usage: html-check.mjs [options] <page.html>");
  process.exit(2);
}

const sleep = ms => new Promise(r => setTimeout(r, ms));
const scratch = fs.mkdtempSync(path.join(process.env.HTML_CHECK_TMP || os.tmpdir(), "html-check-"));

// A static server for --serve: GET and HEAD with byte ranges, nothing else.
async function startServer(root) {
  const types = { ".html": "text/html", ".js": "text/javascript", ".mjs": "text/javascript", ".css": "text/css", ".json": "application/json", ".wasm": "application/wasm", ".pmtiles": "application/octet-stream", ".svg": "image/svg+xml", ".png": "image/png", ".woff2": "font/woff2" };
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, "http://localhost");
    const file = path.join(root, decodeURIComponent(url.pathname));
    if (!file.startsWith(root) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) { res.writeHead(404); res.end(); return; }
    const size = fs.statSync(file).size;
    const headers = { "Content-Type": types[path.extname(file)] || "application/octet-stream", "Accept-Ranges": "bytes" };
    const range = /bytes=(\d+)-(\d*)/.exec(req.headers.range || "");
    if (range) {
      const start = Number(range[1]), end = range[2] ? Math.min(Number(range[2]), size - 1) : size - 1;
      res.writeHead(206, { ...headers, "Content-Range": `bytes ${start}-${end}/${size}`, "Content-Length": end - start + 1 });
      if (req.method === "HEAD") return res.end();
      fs.createReadStream(file, { start, end }).pipe(res);
      return;
    }
    res.writeHead(200, { ...headers, "Content-Length": size });
    if (req.method === "HEAD") return res.end();
    fs.createReadStream(file).pipe(res);
  });
  await new Promise(r => server.listen(0, "127.0.0.1", r));
  return server;
}

function pageURL(server) {
  if (!server) return pathToFileURL(opts.page).href;
  const rel = path.relative(opts.serve, opts.page).split(path.sep).map(encodeURIComponent).join("/");
  return `http://127.0.0.1:${server.address().port}/${rel}`;
}

// A request is local when it stays on this computer: a file, inline data,
// or the --serve server.
function isLocal(url) {
  return /^(file|data|blob|about|chrome|devtools|chrome-extension|moz-extension|resource):/.test(url) || /^https?:\/\/127\.0\.0\.1(:\d+)?\//.test(url);
}

// ---------------------------------------------------------------------------
// Chromium over the DevTools protocol on a pipe (no TCP port).
// ---------------------------------------------------------------------------
async function runChromium(exe, url) {
  const pending = new Map();
  const profile = fs.mkdtempSync(path.join(scratch, "chromium-"));
  const downloads = path.join(profile, "downloads");
  fs.mkdirSync(downloads);
  const child = spawn(exe, [
    "--headless=new", "--remote-debugging-pipe", `--user-data-dir=${profile}`,
    "--no-first-run", "--no-default-browser-check", "--disable-background-networking", "--disable-component-update",
    "--disable-sync", "--disable-features=Translate,OptimizationHints,MediaRouter", "--metrics-recording-only",
    "--proxy-server=127.0.0.1:9", "--proxy-bypass-list=<-loopback>;127.0.0.1",
    "--use-angle=swiftshader", "--enable-unsafe-swiftshader",
    // Inside html-check.sh's own sandbox (macOS sandbox-exec), Chromium
    // cannot start its nested one.
    ...(process.env.HTML_CHECK_CONFINED ? ["--no-sandbox"] : []),
    "about:blank",
  ], { stdio: ["ignore", "ignore", "pipe", "pipe", "pipe"] });
  let stderr = "";
  child.stderr.on("data", d => { stderr += d; });
  const toBrowser = child.stdio[3], fromBrowser = child.stdio[4];
  // A browser that exits fails every call still waiting.
  child.once("exit", code => {
    for (const reply of pending.values()) reply({ error: { message: `Chromium exited (${code}): ${stderr.split("\n").filter(Boolean).slice(-2).join(" ")}` } });
    pending.clear();
  });
  let id = 0, buffer = "";
  const listeners = [];
  fromBrowser.on("data", chunk => {
    buffer += chunk;
    let at;
    while ((at = buffer.indexOf("\0")) >= 0) {
      const msg = JSON.parse(buffer.slice(0, at));
      buffer = buffer.slice(at + 1);
      if (msg.id && pending.has(msg.id)) { pending.get(msg.id)(msg); pending.delete(msg.id); }
      else listeners.forEach(l => l(msg));
    }
  });
  const send = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
    const msg = { id: ++id, method, params };
    if (sessionId) msg.sessionId = sessionId;
    pending.set(msg.id, m => m.error ? reject(new Error(`${method}: ${m.error.message}`)) : resolve(m.result));
    toBrowser.write(JSON.stringify(msg) + "\0");
  });
  const problems = [];
  let downloaded = null;
  try {
    const { targetId } = await send("Target.createTarget", { url: "about:blank" });
    const { sessionId } = await send("Target.attachToTarget", { targetId, flatten: true });
    listeners.push(msg => {
      if (msg.sessionId !== sessionId && msg.method !== "Browser.downloadProgress") return;
      const p = msg.params || {};
      switch (msg.method) {
        case "Runtime.exceptionThrown":
          problems.push(`uncaught: ${p.exceptionDetails.exception?.description || p.exceptionDetails.text}`.split("\n")[0]);
          break;
        case "Runtime.consoleAPICalled":
          if (p.type === "error") problems.push(`console.error: ${p.args.map(a => a.value ?? a.description).join(" ")}`.slice(0, 300));
          break;
        case "Log.entryAdded":
          if (p.entry.level === "error" && !/favicon/.test(p.entry.url || "")) problems.push(`log: ${p.entry.text} ${p.entry.url || ""}`.slice(0, 300));
          break;
        case "Network.requestWillBeSent":
          if (!isLocal(p.request.url)) problems.push(`network request: ${p.request.url.slice(0, 200)}`);
          break;
        case "Network.loadingFailed":
          if (!p.canceled) problems.push(`failed load: ${p.errorText} ${p.type || ""}`);
          break;
        case "Browser.downloadProgress":
          if (p.state === "completed") downloaded = path.join(downloads, p.guid);
          break;
      }
    });
    await send("Runtime.enable", {}, sessionId);
    await send("Log.enable", {}, sessionId);
    await send("Network.enable", {}, sessionId);
    if (!url.startsWith("http://127.0.0.1")) {
      await send("Network.emulateNetworkConditions", { offline: true, latency: 0, downloadThroughput: -1, uploadThroughput: -1 }, sessionId);
    }
    await send("Browser.setDownloadBehavior", { behavior: "allowAndName", downloadPath: downloads, eventsEnabled: true });
    await send("Page.enable", {}, sessionId);
    await send("Page.navigate", { url }, sessionId);
    const evaluate = async expr => {
      const r = await send("Runtime.evaluate", { expression: expr, awaitPromise: true, returnByValue: true }, sessionId);
      if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text);
      return r.result.value;
    };
    await waitFor(() => evaluate("document.readyState === 'complete'"), 15);
    await sleep(500);
    if (opts.expect && !(await waitFor(() => evaluate(opts.expect), opts.wait))) problems.push(`expected ${opts.expect}`);
    if (opts.click) {
      await evaluate(`(() => { const el = document.querySelector(${JSON.stringify(opts.click)}); if (!el) throw new Error("no element matches ${opts.click.replace(/"/g, "'")}"); el.click(); return true; })()`);
      if (opts.download) {
        if (!(await waitFor(async () => downloaded && fs.existsSync(downloaded), opts.wait))) problems.push(`no download after clicking ${opts.click}`);
        else fs.copyFileSync(downloaded, opts.download);
      }
    }
    await sleep(300);
  } catch (e) {
    problems.push(`check failed: ${e.message}`);
  } finally {
    await stop(child);
  }
  if (problems.length && stderr && /error/i.test(stderr) && problems.some(p => p.startsWith("check failed"))) problems.push(stderr.split("\n").slice(-3).join(" "));
  return problems;
}

async function waitFor(fn, seconds) {
  const end = Date.now() + seconds * 1000;
  while (Date.now() < end) {
    try { if (await fn()) return true; } catch (e) { if (process.env.HTML_CHECK_DEBUG) console.error("waiting:", e.message); }
    await sleep(200);
  }
  return false;
}

// ---------------------------------------------------------------------------
// Firefox over WebDriver BiDi on a loopback port.
// ---------------------------------------------------------------------------
async function freePort() {
  const srv = net.createServer();
  await new Promise(r => srv.listen(0, "127.0.0.1", r));
  const port = srv.address().port;
  await new Promise(r => srv.close(r));
  return port;
}

async function runFirefox(exe, url) {
  const profile = fs.mkdtempSync(path.join(scratch, "firefox-"));
  // The network goes to a closed port; the page's own requests are watched
  // as well, so a request that would leave is reported either way.
  fs.writeFileSync(path.join(profile, "user.js"), [
    'user_pref("network.proxy.type", 1);', 'user_pref("network.proxy.http", "127.0.0.1");', 'user_pref("network.proxy.http_port", 9);',
    'user_pref("network.proxy.ssl", "127.0.0.1");', 'user_pref("network.proxy.ssl_port", 9);', 'user_pref("network.proxy.no_proxies_on", "127.0.0.1");',
    'user_pref("network.proxy.allow_hijacking_localhost", false);',
    'user_pref("browser.shell.checkDefaultBrowser", false);', 'user_pref("datareporting.policy.dataSubmissionEnabled", false);',
    'user_pref("app.update.enabled", false);', 'user_pref("browser.startup.homepage_override.mstone", "ignore");',
    'user_pref("toolkit.telemetry.enabled", false);', 'user_pref("browser.safebrowsing.malware.enabled", false);',
    'user_pref("browser.safebrowsing.phishing.enabled", false);', 'user_pref("extensions.update.enabled", false);',
    'user_pref("webgl.force-enabled", true);', 'user_pref("remote.prefs.recommended", true);',
  ].join("\n"));
  const port = await freePort();
  // file:// pages count as system access to Firefox's remote agent.
  // Inside html-check.sh's sandbox its content processes cannot start their
  // own, as for Chromium.
  const env = { ...process.env, MOZ_REMOTE_ALLOW_SYSTEM_ACCESS: "1" };
  if (process.env.HTML_CHECK_CONFINED) Object.assign(env, { MOZ_DISABLE_CONTENT_SANDBOX: "1", MOZ_DISABLE_GMP_SANDBOX: "1", MOZ_DISABLE_RDD_SANDBOX: "1", MOZ_DISABLE_SOCKET_PROCESS_SANDBOX: "1", MOZ_DISABLE_GPU_SANDBOX: "1", MOZ_DISABLE_UTILITY_SANDBOX: "1" });
  const child = spawn(exe, ["--headless", "--no-remote", "--profile", profile, `--remote-debugging-port=${port}`, "about:blank"], { stdio: ["ignore", "ignore", "pipe"], env });
  let stderr = "";
  child.stderr.on("data", d => { stderr += d; });
  const problems = [];
  let ws;
  try {
    const end = Date.now() + 30000;
    while (!ws) {
      try {
        ws = await new Promise((resolve, reject) => {
          const sock = new WebSocket(`ws://127.0.0.1:${port}/session`);
          sock.onopen = () => resolve(sock);
          sock.onerror = () => reject(new Error("no BiDi socket"));
        });
      } catch (e) {
        if (Date.now() > end) throw new Error(`Firefox did not open its BiDi port: ${stderr.split("\n").slice(-3).join(" ")}`);
        await sleep(300);
      }
    }
    let id = 0;
    const pending = new Map();
    const events = [];
    ws.onmessage = ev => {
      const msg = JSON.parse(ev.data);
      if (msg.id && pending.has(msg.id)) { pending.get(msg.id)(msg); pending.delete(msg.id); }
      else if (msg.type === "event") events.push(msg);
    };
    const send = (method, params = {}) => new Promise((resolve, reject) => {
      const msg = { id: ++id, method, params };
      pending.set(msg.id, m => m.type === "error" ? reject(new Error(`${method}: ${m.error} ${m.message}`)) : resolve(m.result));
      ws.send(JSON.stringify(msg));
    });
    await send("session.new", { capabilities: { alwaysMatch: { acceptInsecureCerts: false } } });
    await send("session.subscribe", { events: ["log.entryAdded", "network.beforeRequestSent", "network.fetchError"] });
    // Right after start Firefox replaces its first window; retry until a
    // tab holds.
    let context;
    for (let attempt = 0; !context; attempt++) {
      try {
        ({ context } = await send("browsingContext.create", { type: "tab" }));
      } catch (e) {
        if (attempt >= 20) throw e;
        await sleep(500);
      }
    }
    await send("browsingContext.navigate", { context, url, wait: "complete" });
    const evaluate = async expr => {
      const r = await send("script.evaluate", { expression: expr, target: { context }, awaitPromise: true, resultOwnership: "none" });
      if (r.type === "exception") throw new Error(r.exceptionDetails.text);
      return r.result.value;
    };
    await sleep(500);
    if (opts.expect && !(await waitFor(() => evaluate(opts.expect), opts.wait))) problems.push(`expected ${opts.expect}`);
    if (opts.click) await evaluate(`(() => { const el = document.querySelector(${JSON.stringify(opts.click)}); if (el) el.click(); return !!el; })()`);
    await sleep(500);
    for (const ev of events) {
      const p = ev.params;
      if (ev.method === "log.entryAdded" && p.level === "error") problems.push(`${p.type === "javascript" ? "uncaught" : "console.error"}: ${p.text}`.slice(0, 300));
      if (ev.method === "network.beforeRequestSent" && !isLocal(p.request.url)) problems.push(`network request: ${p.request.url.slice(0, 200)}`);
      if (ev.method === "network.fetchError" && !isLocal(p.request.url)) problems.push(`failed load: ${p.errorText} ${p.request.url.slice(0, 120)}`);
    }
  } catch (e) {
    problems.push(`check failed: ${e.message}`);
  } finally {
    try { ws?.close(); } catch { /* closing */ }
    await stop(child);
  }
  return problems;
}

// stop ends a browser and waits for it, unless it has already gone.
async function stop(child) {
  if (child.exitCode !== null || child.signalCode !== null) return;
  const exited = new Promise(r => child.once("exit", r));
  child.kill("SIGTERM");
  const timer = setTimeout(() => child.kill("SIGKILL"), 5000);
  await exited;
  clearTimeout(timer);
}

// ---------------------------------------------------------------------------

const server = opts.serve ? await startServer(opts.serve) : null;
const url = pageURL(server);
const browsers = [];
if ((opts.browser === "both" || opts.browser === "chromium") && opts.chromium) browsers.push(["chromium", () => runChromium(opts.chromium, url)]);
if ((opts.browser === "both" || opts.browser === "firefox") && opts.firefox) browsers.push(["firefox", () => runFirefox(opts.firefox, url)]);
if (browsers.length === 0) {
  console.log("SKIP: no browser found (set HTML_CHECK_CHROMIUM / HTML_CHECK_FIREFOX)");
  process.exit(77);
}
let failed = false;
for (const [name, run] of browsers) {
  const problems = [...new Set(await run())];
  if (problems.length) {
    failed = true;
    console.log(`FAIL ${name} ${url}`);
    for (const p of problems) console.log(`  ${p}`);
  } else {
    console.log(`PASS ${name} ${url}`);
  }
}
server?.close();
fs.rmSync(scratch, { recursive: true, force: true });
process.exit(failed ? 1 : 0);
