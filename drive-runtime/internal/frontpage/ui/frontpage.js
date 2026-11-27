"use strict";

const byId = id => document.getElementById(id);
const icons = {
  browse: "▤", documents: "≡", maps: "◇", chat: "◌", apps: "⊞",
  wiki: "✎", workspace: "↗", terminal: "›_", share: "⤢", settings: "⚙"
};
const cards = [];
const subscribers = new Map();
const states = new Map();
let shownSections = [];
let pages = [];
let inventory;
let polling = false;

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

function openLink(label, address) {
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

function subscribe(key, render) {
  if (!subscribers.has(key)) subscribers.set(key, []);
  subscribers.get(key).push(render);
  render(states.get(key) || {});
}

function showState(key, state) {
  if (JSON.stringify(states.get(key)) === JSON.stringify(state)) return;
  states.set(key, state);
  for (const render of subscribers.get(key) || []) render(state);
}

async function poll() {
  if (polling || document.hidden) return;
  polling = true;
  const connection = byId("connection");
  try {
    const current = await request("/api/services");
    for (const [key, state] of Object.entries(current)) showState(key, state);
    connection.classList.remove("offline");
    connection.lastChild.textContent = " Local session";
  } catch {
    connection.classList.add("offline");
    connection.lastChild.textContent = " Disconnected";
  } finally {
    polling = false;
  }
}

async function act(key, stop, model, status) {
  try {
    const state = await request("/api/services/" + key + (stop ? "/stop" : ""), model ? {model} : {});
    showState(key, state);
  } catch (error) {
    note(status, error.message, true);
  }
}

function cardShell(item) {
  const card = node("article", "card");
  const head = node("div", "card-head");
  const icon = node("span", "card-icon", icons[item.icon] || "·");
  icon.dataset.icon = item.icon;
  icon.setAttribute("aria-hidden", "true");
  const title = node("div");
  title.append(node("h4", "", item.label));
  const detail = [item.subheader, item.note].filter(Boolean).join(" · ");
  if (detail) title.append(node("p", "detail", detail));
  head.append(icon, title);
  card.append(head);
  if (item.description) card.append(node("p", "card-description", item.description));
  return card;
}

function launchControls(target, key, startLabel, models = []) {
  let selection = "";
  subscribe(key, data => {
    const focused = target.contains(document.activeElement);
    const status = node("p", "status");
    status.setAttribute("role", "status");
    const controls = node("div", "buttons");
    const active = ["starting", "running", "stopping"].includes(data.status);
    const parts = [];
    if (models.length > 1 && !active) {
      const select = node("select");
      select.setAttribute("aria-label", "Model for Open WebUI");
      select.append(new Option("Drive default model", ""));
      for (const model of models) select.append(new Option(model.label, model.file));
      select.value = selection;
      select.addEventListener("change", () => { selection = select.value; });
      parts.push(select);
    }
    for (const link of data.links || []) {
      if (link.label === "LAN address" || link.label === "MCP endpoint") {
        parts.push(node("code", "", link.url));
        if (link.label === "LAN address") controls.append(openLink("Open LAN address", link.url));
      } else {
        controls.append(openLink(link.label, link.url));
      }
    }
    if (active) {
      const stop = button("Stop", "quiet", () => {
        stop.disabled = true;
        act(key, true, "", status).finally(() => { stop.disabled = false; });
      });
      stop.disabled = data.status === "stopping";
      controls.append(stop);
      const messages = {starting: "Starting…", running: "Running", stopping: "Stopping…"};
      note(status, messages[data.status]);
    } else {
      const launch = button(data.status === "failed" ? "Retry" : startLabel, "secondary", () => {
        launch.disabled = true;
        note(status, "Starting…");
        act(key, false, selection, status).finally(() => { launch.disabled = false; });
      });
      controls.append(launch);
      note(status, data.status === "failed" ? data.text : "", data.status === "failed");
    }
    parts.push(controls, status);
    if (data.status === "done" && data.text) parts.push(node("pre", "", data.text));
    target.replaceChildren(...parts);
    if (focused) target.querySelector("button:not(:disabled), a")?.focus();
  });
}

function makeCard(item) {
  const card = cardShell(item);
  const actions = node("div", "card-actions");
  if (item.kind === "launch") {
    const labels = {
      browse: "Start Kiwix", maps: "Start map viewer", wiki: "Start Wiki",
      chat: "Start Open WebUI", documents: "Serve documents", apps: "Start app",
      workspace: "Serve workspace", settings: "Show details"
    };
    const label = item.label === "Share on Local Network" ? "Share drive" : (labels[item.icon] || "Start service");
    launchControls(actions, item.service, label, item.icon === "chat" ? inventory.models : []);
  } else {
    actions.append(node("span", "status", "In the terminal"), node("code", "", item.command));
  }
  card.append(actions);
  return card;
}

function renderInventory(data) {
  inventory = data;
  const nav = byId("nav");
  const searchPanel = byId("search-panel");
  searchPanel.hidden = !data.hasSearch;
  if (data.hasSearch) {
    const shortcut = node("a", "", "Search");
    shortcut.href = "#search-panel";
    nav.append(shortcut);
  }
  let itemCount = 0;
  data.groups.forEach((group, index) => {
    const items = group.items.filter(item => item.kind !== "search");
    if (items.length === 0) return;
    const section = node("section", "action-section");
    section.id = "group-" + index;
    const title = node("div", "group-title");
    const count = node("span", "count", String(items.length));
    title.append(node("h3", "", group.label), count);
    const list = node("div", "cards");
    const rows = items.map(item => {
      const card = makeCard(item);
      const match = [group.label, item.label, item.description, item.subheader, item.note]
        .filter(Boolean).join(" ").toLowerCase();
      list.append(card);
      return {card, match};
    });
    cards.push(...rows);
    section.append(title, list);
    byId("groups").append(section);
    shownSections.push({section, rows, count});
    itemCount += items.length;

    if (["Library", "Maps", "Local AI", "Apps", "Tools"].includes(group.label)) {
      const shortcut = node("a", "", group.label);
      shortcut.href = "#" + section.id;
      nav.append(shortcut);
    }
  });
  const saved = node("a", "", "Saved apps");
  saved.href = "#saved-apps";
  nav.append(saved);
  byId("vault").textContent = data.drive + (data.preset ? "  ·  " + data.preset : "");
  byId("inventory-summary").textContent = itemCount + (itemCount === 1 ? " action" : " actions") + " on this vault";
  note(byId("page-status"), itemCount ? "" : "There are no launchable items on this drive yet.");
  byId("filter").disabled = !itemCount;

  const workspaceActions = byId("workspace-actions");
  workspaceActions.className = "workspace-bar";
  launchControls(workspaceActions, data.workspace, "Serve workspace");
  subscribe(data.workspace, renderPages);
}

function renderPages(state = states.get(inventory.workspace) || {}) {
  const address = state.links?.[0]?.url;
  const list = byId("pages");
  list.replaceChildren(...pages.map(page => {
    const card = cardShell({label: page.name, icon: "apps", note: page.path});
    const actions = node("div", "card-actions");
    if (address) {
      const relative = page.path.split("/").map(encodeURIComponent).join("/");
      actions.append(openLink("Open app", new URL(relative, address).href));
    } else {
      const status = node("p", "status");
      const launch = button(state.status === "starting" ? "Starting…" : "Serve app", "secondary", () => {
        launch.disabled = true;
        act(inventory.workspace, false, "", status).finally(() => { launch.disabled = false; });
      });
      launch.disabled = state.status === "starting" || state.status === "stopping";
      actions.append(launch, status);
    }
    card.append(actions);
    return card;
  }));
}

async function refreshPages() {
  const refresh = byId("refresh");
  const status = byId("workspace-status");
  refresh.disabled = true;
  try {
    pages = await request("/api/workspace");
    renderPages();
    note(status, pages.length ? "" : "No saved HTML apps yet.");
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
  const status = node("p", "status");
  status.setAttribute("role", "status");
  title.addEventListener("click", async () => {
    title.disabled = true;
    note(status, "Opening…");
    try {
      const link = await request("/api/read", {source: item.source, path: item.path});
      const open = openLink(item.title || item.label, link.url);
      title.replaceWith(open);
      open.focus();
      note(status, "");
      location.assign(link.url);
    } catch (error) {
      note(status, error.message, true);
      title.disabled = false;
    }
  });
  row.append(title, node("p", "", item.label));
  if (item.snippet) row.append(node("p", "", item.snippet));
  row.append(status);
  return row;
}

byId("filter").addEventListener("input", event => {
  const terms = event.target.value.trim().toLowerCase().split(/\s+/);
  for (const {card, match} of cards) card.hidden = !terms.every(term => match.includes(term));
  for (const {section, rows, count} of shownSections) {
    const visible = rows.filter(({card}) => !card.hidden).length;
    count.textContent = visible;
    section.hidden = !visible;
  }
  note(byId("page-status"), cards.every(({card}) => card.hidden) ? "No installed items match." : "");
});

byId("search-form").addEventListener("submit", async event => {
  event.preventDefault();
  const button = event.currentTarget.querySelector("button");
  const results = byId("search-results");
  const status = byId("search-status");
  button.disabled = true;
  results.hidden = true;
  note(status, "Searching…");
  try {
    const data = await request("/api/search", {query: byId("query").value});
    results.replaceChildren(...data.hits.map(searchResult));
    results.hidden = !data.hits.length;
    note(status, data.status || (data.hits.length ? "" : "No matching articles or documents."));
  } catch (error) {
    note(status, error.message, true);
  } finally {
    button.disabled = false;
  }
});

byId("refresh").addEventListener("click", refreshPages);
request("/api/inventory")
  .then(data => {
    renderInventory(data);
    refreshPages();
    poll();
    setInterval(poll, 1000);
    document.addEventListener("visibilitychange", poll);
  })
  .catch(error => note(byId("page-status"), error.message, true));
