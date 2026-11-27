"use strict";

const byId = id => document.getElementById(id);
const icons = {
  browse: "▤", documents: "≡", maps: "◇", chat: "◌", apps: "⊞",
  wiki: "✎", workspace: "↗", terminal: "›_", share: "⤢", settings: "⚙"
};
const startLabels = {wiki: "Start Wiki", chat: "Start Open WebUI", settings: "Show details"};
const readerKinds = new Set(["browse", "documents", "apps", "maps", "workspace"]);
const overviewTypes = ["Maps", "Archives", "Documents", "Apps", "Saved apps", "Local AI", "Tools", "Settings"];
const itemTypes = {
  browse: "Archives", documents: "Documents", maps: "Maps",
  apps: "Apps", wiki: "Apps", chat: "Local AI"
};
const nameOrder = new Intl.Collator(undefined, {numeric: true, sensitivity: "base"});
const serviceStatus = {
  starting: "Starting…", running: "Running", stopping: "Stopping…", failed: "Failed"
};
const entries = new Map();
const states = new Map();
const activeRequests = new Set();
let inventory;
let selection;
let model = "";
let route = "";
let polling = false;
let searchRead = 0;
let searchRequest = 0;
let shownQuery = "";
let queuedLaunch;
const shellItem = {label: "Terminal", group: "Tools", icon: "terminal", terminal: true,
  description: "Open your shell in the vault folder."};
const terminalKey = readTerminalKey();
const terminals = {regular: {}, sb: {}};

function node(tag, css, text) {
  const element = document.createElement(tag);
  if (css) element.className = css;
  if (text) element.textContent = text;
  return element;
}

function button(label, css, click) {
  const element = node("button", "button " + css, label);
  element.type = "button";
  element.addEventListener("click", click);
  return element;
}

function externalLink(label, address) {
  const element = node("a", "button secondary", label + " ↗");
  element.href = address;
  element.target = "_blank";
  element.rel = "noopener noreferrer";
  return element;
}

function note(target, message, error = false) {
  target.textContent = message;
  target.classList.toggle("error", error);
  target.hidden = !message;
}

async function request(path, data) {
  const options = data === undefined ? {} : {
    method: "POST",
    headers: {"Content-Type": "application/json", "X-Svalbard-Request": "1"},
    body: JSON.stringify(data)
  };
  const response = await fetch(path, options);
  if (!response.ok) throw new Error((await response.text()).trim() || "The drive did not respond.");
  return response.json();
}

function itemSearchText(item) {
  const tags = Array.isArray(item.tags) ? item.tags : [];
  return [item.group, item.label, item.summary, item.description, item.note, item.savedPath, item.recipeId, ...tags]
    .filter(Boolean).join(" ").toLowerCase();
}

function navLink(item) {
  const link = node("a", "nav-link");
  link.href = "#" + item.route;
  link.title = item.label;
  link.dataset.match = itemSearchText(item);
  link.append(node("span", "nav-text", item.label));
  if (item.service) {
    link.dataset.service = item.service;
    const indicator = node("span", "nav-dot");
    indicator.hidden = true;
    link.append(indicator);
  }
  return link;
}

function overviewType(item) {
  if (item.group === "Saved apps" || item.icon === "workspace") return "Saved apps";
  if (item.group === "Local AI") return "Local AI";
  return itemTypes[item.icon] || item.group;
}

function previewURL(item) {
  if (!item.image?.url) return "";
  try {
    const url = new URL(item.image.url, location.origin);
    if (url.origin === location.origin && !url.username && !url.password) return url.href;
  } catch {
    // A missing or malformed preview uses the item icon.
  }
  return "";
}

function itemPreview(item, css) {
  const preview = node("div", "preview " + css);
  preview.dataset.type = overviewType(item);
  const icon = node("span", "glyph", icons[item.icon] || "·");
  icon.setAttribute("aria-hidden", "true");
  preview.append(icon);
  const src = previewURL(item);
  if (src) {
    const image = node("img");
    image.alt = item.image.alt || "";
    image.loading = "lazy";
    image.decoding = "async";
    image.className = "preview-image";
    image.addEventListener("load", () => { image.classList.add("loaded"); });
    image.addEventListener("error", () => { image.remove(); });
    preview.append(image);
    image.src = src;
  }
  return preview;
}

function overviewRow(item, grouped) {
  const row = node("li", "overview-row");
  const type = overviewType(item);
  row.classList.toggle("compact", type === "Tools" || type === "Settings");
  const content = node("div", "overview-copy");
  const link = node("a", "overview-name", item.label);
  link.href = "#" + item.route;
  content.append(link);
  const summary = item.summary || item.description;
  if (summary && summary !== item.label) content.append(node("p", "overview-summary", summary));
  const meta = [grouped ? "" : type, ...((item.tags || []).slice(0, 3)), item.note].filter(Boolean);
  if (meta.length) content.append(node("p", "overview-meta", meta.join(" · ")));
  row.append(itemPreview(item, "overview-preview"), content);

  if (item.service && readerKinds.has(item.icon)) {
    const launch = node("a", "button primary overview-launch", overviewLaunchLabel(item.service));
    launch.href = link.href;
    launch.dataset.service = item.service;
    launch.dataset.label = item.label;
    launch.setAttribute("aria-label", launch.textContent + " " + item.label);
    launch.addEventListener("click", event => {
      if (event.button || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
      event.preventDefault();
      history.pushState(null, "", launch.hash);
      renderRoute();
      window.scrollTo(0, 0);
      byId("item-title").focus({preventScroll: true});
      const state = states.get(item.service)?.status;
      if (state === "stopping") queuedLaunch = item;
      else if (state !== "starting" && state !== "running") act(item);
    });
    row.append(launch);
  }
  return row;
}

function overviewLaunchLabel(key) {
  return states.get(key)?.status === "running" ? "Open" : "Serve & open";
}

function buildNavigation(data) {
  inventory = data;
  byId("vault").textContent = data.drive + (data.preset ? " · " + data.preset : "");
  byId("search-nav").hidden = !data.hasSearch;
  byId("home-search").hidden = !data.hasSearch;

  data.groups.forEach((group, index) => {
    const items = group.items.filter(item => item.kind !== "search");
    if (!items.length) return;
    const section = node("details", "group");
    section.open = true;
    section.append(node("summary", "", group.label));
    items.forEach((item, position) => {
      const entry = {...item, group: group.label, route: "item-" + index + "-" + position};
      entries.set(entry.route, entry);
      section.append(navLink(entry));
    });
    byId("groups").append(section);
  });
  byId("filter").disabled = false;
  syncOverviewTypes();
  renderHome();
}

function syncOverviewTypes() {
  const control = byId("overview-type");
  const selected = control.value;
  const available = new Set(Array.from(entries.values(), overviewType));
  const types = overviewTypes.filter(type => available.delete(type));
  types.push(...Array.from(available).sort(nameOrder.compare));
  control.replaceChildren(new Option("All types", ""), ...types.map(type => new Option(type, type)));
  if (types.includes(selected)) control.value = selected;
}

function renderHome() {
  const items = Array.from(entries.values());
  const count = items.length;
  note(byId("inventory-summary"), count ? count + (count === 1 ? " item" : " items") + " available locally" : "There are no launchable items on this drive yet.");
  byId("home-content").hidden = !count;
  const terms = byId("overview-filter").value.trim().toLowerCase().split(/\s+/);
  const selected = byId("overview-type").value;
  const visible = items.filter(item => {
    if (selected && overviewType(item) !== selected) return false;
    const description = itemSearchText(item);
    return terms.every(term => description.includes(term));
  });
  visible.sort((a, b) => nameOrder.compare(a.label, b.label));
  const output = byId("home-links");
  output.replaceChildren();
  const resultCount = visible.length;
  const filtered = selected || terms[0];
  note(byId("overview-results"), filtered ? (
    resultCount ? resultCount + " of " + count + " items" : "No matching items."
  ) : "");

  if (byId("overview-order").value === "name") {
    const list = node("ul", "overview-rows");
    list.append(...visible.map(item => overviewRow(item, false)));
    output.append(list);
    return;
  }

  const groups = new Map(Array.from(byId("overview-type").options)
    .slice(1).map(option => [option.value, []]));
  for (const row of visible) groups.get(overviewType(row)).push(row);
  for (const [type, rows] of groups) {
    if (!rows.length) continue;
    const section = node("section", "overview-group");
    const list = node("ul", "overview-rows");
    list.append(...rows.map(item => overviewRow(item, true)));
    section.append(node("h2", "overview-group-title", type), list);
    output.append(section);
  }
}

function filterNavigation() {
  const terms = byId("filter").value.trim().toLowerCase().split(/\s+/);
  for (const link of byId("nav").querySelectorAll("[data-match]")) {
    link.hidden = !terms.every(term => link.dataset.match.includes(term));
  }
  for (const group of byId("nav").querySelectorAll(".group")) {
    if (!group.querySelector("[data-match]")) continue;
    group.hidden = !group.querySelector("[data-match]:not([hidden])");
    if (terms[0] && group instanceof HTMLDetailsElement) group.open = true;
  }
  note(byId("filter-status"), terms[0] && !byId("nav").querySelector("[data-match]:not([hidden])") ? "No matching items." : "");
}

function markSelection() {
  const active = route === "search-result" ? "#search" : "#" + route;
  for (const link of byId("nav").querySelectorAll("a")) {
    if (link.hash === active) link.setAttribute("aria-current", "page");
    else link.removeAttribute("aria-current");
  }
}

function renderNavStatus(key) {
  const status = states.get(key)?.status;
  for (const link of byId("nav").querySelectorAll("[data-service]")) {
    if (link.dataset.service !== key) continue;
    const dot = link.querySelector(".nav-dot");
    dot.hidden = !serviceStatus[status];
    dot.dataset.state = status || "";
    const label = link.querySelector(".nav-text").textContent;
    if (serviceStatus[status]) link.setAttribute("aria-label", label + ", " + serviceStatus[status]);
    else link.removeAttribute("aria-label");
  }
  for (const launch of byId("home-links").querySelectorAll(".overview-launch")) {
    if (launch.dataset.service !== key) continue;
    launch.textContent = overviewLaunchLabel(key);
    launch.setAttribute("aria-label", launch.textContent + " " + launch.dataset.label);
  }
}

function showState(key, state) {
  if (JSON.stringify(states.get(key)) === JSON.stringify(state)) return;
  states.set(key, state);
  renderNavStatus(key);
  if (selection?.service === key) renderItem();
  if (state.status === "stopped" && queuedLaunch?.service === key) {
    const item = queuedLaunch;
    queuedLaunch = undefined;
    if (selection === item) act(item);
  }
  if (route === "search-result" && history.state?.reader?.service === key && unavailableReader(state)) {
    renderRoute();
  }
}

async function poll() {
  if (polling || document.hidden) return;
  polling = true;
  const connection = byId("connection");
  try {
    const current = await request("/api/services");
    for (const [key, state] of Object.entries(current)) showState(key, state);
    connection.classList.remove("offline");
    connection.lastElementChild.textContent = "Local session";
  } catch {
    connection.classList.add("offline");
    connection.lastElementChild.textContent = "Disconnected";
  } finally {
    polling = false;
  }
}

async function act(item, stop = false) {
  if (activeRequests.has(item.service)) return;
  activeRequests.add(item.service);
  const status = byId("item-status");
  const controls = byId("item-controls");
  if (item === selection) {
    controls.querySelectorAll("button").forEach(button => { button.disabled = true; });
    note(status, stop ? "Stopping…" : "Starting…");
  }
  let result;
  try {
    const data = model && item.icon === "chat" && !stop ? {model} : {};
    result = await request("/api/services/" + item.service + (stop ? "/stop" : ""), data);
  } catch (error) {
    if (item === selection) {
      controls.querySelectorAll("button").forEach(button => { button.disabled = false; });
      note(status, error.message, true);
    }
  } finally {
    activeRequests.delete(item.service);
  }
  if (result) showState(item.service, result);
}

function closeReader() {
  byId("reader").hidden = true;
  byId("viewport").classList.remove("has-reader");
  byId("reader-frame")?.remove();
  byId("stop-search-reader").hidden = true;
  note(byId("reader-status"), "");
}

function readerURL(address) {
  try {
    const url = new URL(address);
    const local = url.protocol === "http:" && ["127.0.0.1", "localhost"].includes(url.hostname);
    if (local && url.origin !== location.origin && !url.username && !url.password) return url;
  } catch {
    // A missing or stale browser history entry has no usable address.
  }
  return null;
}

function showReader(label, address, kind, fromSearch = false) {
  const url = readerURL(address);
  if (!url) return false;

  let frame = byId("reader-frame");
  if (frame?.getAttribute("src") !== url.href) {
    frame?.remove();
    frame = node("iframe");
    frame.id = "reader-frame";
    frame.referrerPolicy = "no-referrer";
    const pdf = url.pathname.toLowerCase().endsWith(".pdf");
    if (kind !== "documents" && !pdf) {
      frame.setAttribute("sandbox", "allow-scripts allow-same-origin allow-forms allow-downloads");
    }
    frame.src = url.href;
    // The new frame starts at the reader URL without adding an intermediate
    // empty frame or reusing an older frame's navigation history.
    byId("reader").append(frame);
  }
  frame.title = label;
  byId("reader").hidden = false;
  byId("reader-bar").hidden = !fromSearch;
  byId("reader-link").href = url.href;
  byId("reader-title").textContent = label;
  byId("viewport").classList.add("has-reader");
  return true;
}

function savedAddress(item, base) {
  const path = item.savedPath.split("/").map(encodeURIComponent).join("/");
  return new URL(path, base).href;
}

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
  start.hidden = Boolean(state.session);
  start.disabled = !terminalKey || state.loading;
  byId("terminal-stop").hidden = !state.session;
  byId("terminal-paste").hidden = !state.session || !selection?.command;
  for (const terminal of Object.values(terminals)) {
    if (terminal.screen) terminal.screen.hidden = state !== terminal;
  }
  const message = terminalKey ? state.message : "Open the private link printed by ./run web to use the terminal.";
  note(byId("terminal-status"), message || "", state.failed);
}

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

function renderItem() {
  const item = selection;
  const controls = byId("item-controls");
  const focused = controls.contains(document.activeElement);
  const status = byId("item-status");
  const output = byId("item-output");
  byId("terminal").hidden = !item.terminal;
  controls.replaceChildren();
  note(status, "");
  note(output, "");
  if (!item.service) {
    if (item.terminal) {
      renderTerminal();
    } else if (item.command) {
      note(status, "Run this action from the drive’s terminal.");
      note(output, item.command);
    }
    return;
  }

  const state = states.get(item.service) || {};
  const active = ["starting", "running", "stopping"].includes(state.status);
  if (!active) {
    if (item.icon === "chat" && inventory.models?.length > 1) {
      const select = node("select");
      select.setAttribute("aria-label", "Model for Open WebUI");
      select.append(new Option("Drive default model", ""));
      for (const choice of inventory.models) select.append(new Option(choice.label, choice.file));
      select.value = model;
      select.addEventListener("change", () => { model = select.value; });
      controls.append(select);
    }
    let start = item.label === "Share on Local Network" ? "Share drive" : startLabels[item.icon] || "Start service";
    if (readerKinds.has(item.icon)) start = "Serve & open";
    if (state.status === "failed") start = "Retry";
    controls.append(button(start, "primary", () => act(item)));
  }
  let reading = false;
  if (state.status === "running") {
    const links = state.links || [];
    for (const link of links) {
      if (link.label === "MCP endpoint") {
        controls.append(node("code", "", link.url));
        continue;
      }
      const address = item.savedPath ? savedAddress(item, link.url) : link.url;
      if (!reading && readerKinds.has(item.icon)) reading = showReader(item.label, address, item.icon);
      const label = reading ? "Open in new tab" : link.label;
      controls.append(externalLink(label, address));
    }
  }
  if (!reading) closeReader();

  if (active) {
    const stop = button(reading ? "Stop serving" : "Stop", "quiet-button", () => act(item, true));
    stop.disabled = state.status === "stopping";
    controls.append(stop);
    if (!reading) note(status, serviceStatus[state.status]);
  }
  if (state.status === "failed") note(status, state.text, true);
  if (state.status === "done") note(output, state.text);
  if (focused) controls.querySelector("button:not(:disabled), a, select")?.focus();
}

function toggleSidebar(open) {
  document.body.classList.toggle("sidebar-open", open);
  byId("menu").setAttribute("aria-expanded", String(open));
}

function renderRoute() {
  if (!inventory) return;
  const returningFromResult = route === "search-result";
  const navigation = history.state;
  route = location.hash.slice(1) || "overview";
  const result = route === "search-result" && inventory.hasSearch && navigation?.reader;
  if (route === "search-result" && !result) {
    route = "search";
    history.replaceState(null, "", "#search");
  }
  selection = route === "terminal" ? shellItem : entries.get(route);
  const inSearch = inventory.hasSearch && (route === "search" || Boolean(result));
  if (!selection && !inSearch) route = "overview";

  ++searchRead;
  model = "";
  if (queuedLaunch !== selection) queuedLaunch = undefined;
  closeReader();
  byId("details").hidden = !selection;
  byId("overview").hidden = route !== "overview";
  byId("search-panel").hidden = route !== "search";
  byId("location").textContent = selection?.group || (inSearch ? "Search" : "Overview");
  markSelection();
  toggleSidebar(false);

  if (result) {
    if (unavailableReader(states.get(result.service)) || !showReader(result.label, result.address, result.kind, true)) {
      history.replaceState({query: navigation.query, position: navigation.position}, "", "#search");
      renderRoute();
    } else {
      const stop = byId("stop-search-reader");
      stop.hidden = !result.service;
      stop.disabled = false;
    }
    return;
  }

  if (route === "search") {
    const focusResult = () => {
      if (returningFromResult && route === "search") {
        byId("search-results").querySelectorAll("button")[navigation?.position]?.focus();
      }
    };
    if (typeof navigation?.query === "string") {
      byId("query").value = navigation.query;
      if (navigation.query !== shownQuery) {
        runSearch().then(focusResult);
      } else {
        focusResult();
      }
    }
    return;
  }

  if (selection) {
    byId("item-title").textContent = selection.label;
    byId("item-kind").textContent = [selection.subheader, selection.note].filter(Boolean).join(" · ");
    byId("item-description").textContent = selection.summary || selection.description || "";
    note(byId("item-recipe"), selection.recipeId ? "Recipe · " + selection.recipeId : "");
    byId("item-preview").replaceChildren(itemPreview(selection, "detail-preview"));
    renderItem();
  }
}

async function refreshPages() {
  const refresh = byId("refresh");
  const status = byId("workspace-status");
  refresh.disabled = true;
  try {
    const pages = await request("/api/workspace");
    for (const [route, entry] of entries) if (entry.savedPath) entries.delete(route);
    byId("saved-links").replaceChildren(...pages.map(page => {
      const entry = {
        route: "saved/" + encodeURIComponent(page.path), label: page.name, note: page.path,
        group: "Saved apps", icon: "workspace", kind: "launch", service: inventory.workspace,
        savedPath: page.path
      };
      entries.set(entry.route, entry);
      return navLink(entry);
    }));
    note(status, pages.length ? "" : "No saved HTML apps yet.");
    syncOverviewTypes();
    renderHome();
    renderNavStatus(inventory.workspace);
    filterNavigation();
    if (location.hash.startsWith("#saved/") && (!selection || !entries.has(route))) renderRoute();
    else markSelection();
  } catch (error) {
    note(status, error.message, true);
  } finally {
    refresh.disabled = false;
  }
}

function searchResult(item, position) {
  const row = node("li");
  const title = node("button", "result-link", item.title || item.label);
  title.type = "button";
  const status = node("p", "quiet");
  status.setAttribute("role", "status");
  title.addEventListener("click", async () => {
    const read = ++searchRead;
    title.disabled = true;
    note(status, "Opening…");
    try {
      const link = await request("/api/read", {source: item.source, path: item.path});
      if (read !== searchRead) return;
      const address = readerURL(link.url);
      if (!address) throw new Error("The result has no local reader address.");
      const kind = item.source.startsWith("books/") ? "documents" : "browse";
      const searchState = {query: shownQuery, position};
      history.replaceState(searchState, "", "#search");
      history.pushState({
        ...searchState,
        reader: {label: title.textContent, address: address.href, kind, service: link.service}
      }, "", "#search-result");
      note(status, "");
      renderRoute();
      byId("back-to-search").focus();
    } catch (error) {
      note(status, error.message, true);
    } finally {
      title.disabled = false;
    }
  });
  row.append(title, node("p", "quiet", item.label));
  if (item.snippet) row.append(node("p", "", item.snippet));
  row.append(status);
  return row;
}

function unavailableReader(state) {
  return ["stopping", "stopped", "failed"].includes(state?.status);
}

async function stopSearchReader() {
  const service = history.state?.reader?.service;
  if (!service) return;
  const stop = byId("stop-search-reader");
  stop.disabled = true;
  try {
    const state = await request("/api/services/" + service + "/stop", {});
    showState(service, state);
  } catch (error) {
    if (history.state?.reader?.service === service) note(byId("reader-status"), error.message, true);
  } finally {
    stop.disabled = false;
  }
}

async function runSearch() {
  ++searchRead;
  const query = byId("query").value.trim();
  const submitted = ++searchRequest;
  shownQuery = query;
  if (location.hash === "#search" && history.state?.query !== query) {
    history.replaceState({query}, "", "#search");
  }
  const button = byId("search-form").querySelector("button");
  const results = byId("search-results");
  const status = byId("search-status");
  button.disabled = true;
  results.hidden = true;
  note(status, "Searching…");
  try {
    const data = await request("/api/search", {query});
    if (submitted !== searchRequest) return;
    results.replaceChildren(...data.hits.map(searchResult));
    results.hidden = !data.hits.length;
    note(status, data.status || (data.hits.length ? "" : "No matching articles or documents."));
  } catch (error) {
    if (submitted === searchRequest) {
      shownQuery = "";
      note(status, error.message, true);
    }
  } finally {
    if (submitted === searchRequest) button.disabled = false;
  }
}

byId("menu").addEventListener("click", () => toggleSidebar(!document.body.classList.contains("sidebar-open")));
byId("nav").addEventListener("click", event => {
  if (event.target.closest("a")?.hash === "#" + route) toggleSidebar(false);
});
document.addEventListener("keydown", event => {
  if (event.key === "Escape" && document.body.classList.contains("sidebar-open")) {
    toggleSidebar(false);
    byId("menu").focus();
  }
});
document.querySelector(".skip").addEventListener("click", event => {
  event.preventDefault();
  byId("main").focus();
});
byId("filter").addEventListener("input", filterNavigation);
byId("overview-filter").addEventListener("input", renderHome);
byId("overview-type").addEventListener("change", renderHome);
byId("overview-order").addEventListener("change", renderHome);
byId("terminal-start").addEventListener("click", startTerminal);
byId("terminal-stop").addEventListener("click", () => selectedTerminal().session?.disconnect());
byId("terminal-paste").addEventListener("click", () => selectedTerminal().session?.paste(selection.command));
window.addEventListener("pagehide", () => {
  for (const state of Object.values(terminals)) state.session?.disconnect();
});
byId("stop-search-reader").addEventListener("click", stopSearchReader);
byId("refresh").addEventListener("click", refreshPages);
byId("back-to-search").addEventListener("click", () => {
  const {query, position} = history.state || {};
  history.pushState({query, position}, "", "#search");
  renderRoute();
});
byId("home-search").addEventListener("submit", event => {
  event.preventDefault();
  byId("query").value = byId("home-query").value;
  location.hash = "search";
  runSearch();
});
byId("search-form").addEventListener("submit", event => {
  event.preventDefault();
  runSearch();
});
window.addEventListener("hashchange", renderRoute);

request("/api/inventory")
  .then(data => {
    buildNavigation(data);
    renderRoute();
    refreshPages();
    poll();
    setInterval(poll, 1000);
    document.addEventListener("visibilitychange", poll);
  })
  .catch(error => note(byId("inventory-summary"), error.message, true));
