"use strict";

const byId = id => document.getElementById(id);
const icons = {
  browse: "▤", documents: "≡", maps: "◇", chat: "◌", apps: "⊞",
  wiki: "✎", workspace: "↗", terminal: "›_", share: "⤢", settings: "⚙"
};
const startLabels = {
  browse: "Start Kiwix", maps: "Start map viewer", wiki: "Start Wiki",
  chat: "Start Open WebUI", documents: "Serve documents", apps: "Start app",
  workspace: "Serve workspace", settings: "Show details"
};
const readerKinds = new Set(["browse", "documents", "apps", "maps", "workspace"]);
const serviceStatus = {
  starting: "Starting…", running: "Running", stopping: "Stopping…", failed: "Failed"
};
const entries = new Map();
const states = new Map();
let inventory;
let selection;
let model = "";
let route = "";
let polling = false;
let searchRead = 0;

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

function navLink(item, home = false) {
  const link = node("a", home ? "home-link" : "nav-link");
  link.href = "#" + item.route;
  const icon = node("span", "glyph", icons[item.icon] || "·");
  icon.setAttribute("aria-hidden", "true");
  link.append(icon, node("span", "nav-text", item.label));
  if (home) {
    link.append(node("span", "quiet", item.subheader || item.group));
  } else {
    link.title = item.label;
    link.dataset.match = [item.group, item.label, item.description, item.note, item.savedPath]
      .filter(Boolean).join(" ").toLowerCase();
    if (item.service) {
      link.dataset.service = item.service;
      const indicator = node("span", "nav-dot");
      indicator.hidden = true;
      link.append(indicator);
    }
  }
  return link;
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
  renderHome();
}

function renderHome() {
  const items = Array.from(entries.values());
  const readables = items.filter(item => readerKinds.has(item.icon));
  const visible = readables.length ? readables : items;
  const count = items.length;
  note(byId("inventory-summary"), count ? count + (count === 1 ? " item" : " items") + " available locally" : "There are no launchable items on this drive yet.");
  byId("home-links").replaceChildren(...visible.slice(0, 5).map(item => navLink(item, true)));
  byId("home-content").hidden = !visible.length;
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
  for (const link of byId("nav").querySelectorAll("a")) {
    if (link.hash === "#" + route) link.setAttribute("aria-current", "page");
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
}

function showState(key, state) {
  if (JSON.stringify(states.get(key)) === JSON.stringify(state)) return;
  states.set(key, state);
  renderNavStatus(key);
  if (selection?.service === key) renderItem();
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
  const status = byId("item-status");
  const controls = byId("item-controls");
  if (item === selection) {
    controls.querySelectorAll("button").forEach(button => { button.disabled = true; });
    note(status, stop ? "Stopping…" : "Starting…");
  }
  try {
    const data = model && item.icon === "chat" && !stop ? {model} : {};
    const result = await request("/api/services/" + item.service + (stop ? "/stop" : ""), data);
    showState(item.service, result);
  } catch (error) {
    if (item === selection) {
      controls.querySelectorAll("button").forEach(button => { button.disabled = false; });
      note(status, error.message, true);
    }
  }
}

function closeReader() {
  byId("reader").hidden = true;
  byId("viewport").classList.remove("has-reader");
  byId("reader-frame").removeAttribute("src");
}

function showReader(label, address, kind, fromSearch = false) {
  const url = new URL(address);
  const local = url.protocol === "http:" && ["127.0.0.1", "localhost"].includes(url.hostname);
  if (!local || url.origin === location.origin || url.username || url.password) return false;

  const frame = byId("reader-frame");
  const pdf = url.pathname.toLowerCase().endsWith(".pdf");
  if (kind === "documents" || pdf) frame.removeAttribute("sandbox");
  else frame.setAttribute("sandbox", "allow-scripts allow-same-origin allow-forms allow-downloads");
  frame.title = label;
  if (frame.getAttribute("src") !== url.href) frame.src = url.href;
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

function renderItem() {
  const item = selection;
  const controls = byId("item-controls");
  const focused = controls.contains(document.activeElement);
  const status = byId("item-status");
  const output = byId("item-output");
  controls.replaceChildren();
  note(status, "");
  note(output, "");
  if (item.kind === "terminal") {
    note(status, "Run this action from the drive’s terminal.");
    note(output, item.command);
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
    if (item.savedPath) start = "Serve app";
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
    const stop = button("Stop", "quiet-button", () => act(item, true));
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
  route = location.hash.slice(1) || "overview";
  selection = entries.get(route);
  if (!selection && route !== "search") route = "overview";
  if (route === "search" && !inventory.hasSearch) route = "overview";
  ++searchRead;
  model = "";
  closeReader();
  byId("details").hidden = !selection;
  byId("overview").hidden = route !== "overview";
  byId("search-panel").hidden = route !== "search";
  byId("location").textContent = selection?.group || (route === "search" ? "Search" : "Overview");
  markSelection();
  toggleSidebar(false);

  if (selection) {
    byId("item-title").textContent = selection.label;
    byId("item-kind").textContent = [selection.subheader, selection.note].filter(Boolean).join(" · ");
    byId("item-description").textContent = selection.description || "";
    renderItem();
    // Selecting a reader starts its local content service. Editing, model
    // and network tools still show their explicit launch controls.
    if (readerKinds.has(selection.icon) && !states.has(selection.service)) act(selection);
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
    renderNavStatus(inventory.workspace);
    renderHome();
    filterNavigation();
    if (location.hash.startsWith("#saved/") && (!selection || !entries.has(route))) renderRoute();
    else markSelection();
  } catch (error) {
    note(status, error.message, true);
  } finally {
    refresh.disabled = false;
  }
}

function searchResult(item) {
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
      const kind = item.source.startsWith("books/") ? "documents" : "browse";
      if (showReader(title.textContent, link.url, kind, true)) {
        byId("search-panel").hidden = true;
        byId("back-to-search").focus();
      }
      note(status, "");
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

async function runSearch() {
  ++searchRead;
  const query = byId("query").value;
  const button = byId("search-form").querySelector("button");
  const results = byId("search-results");
  const status = byId("search-status");
  button.disabled = true;
  results.hidden = true;
  note(status, "Searching…");
  try {
    const data = await request("/api/search", {query});
    results.replaceChildren(...data.hits.map(searchResult));
    results.hidden = !data.hits.length;
    note(status, data.status || (data.hits.length ? "" : "No matching articles or documents."));
  } catch (error) {
    note(status, error.message, true);
  } finally {
    button.disabled = false;
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
byId("refresh").addEventListener("click", refreshPages);
byId("back-to-search").addEventListener("click", () => {
  closeReader();
  byId("search-panel").hidden = false;
  byId("search-results").querySelector("button")?.focus();
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
