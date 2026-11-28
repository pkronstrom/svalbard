"use strict";

const byId = id => document.getElementById(id);
const collections = [
  {name:"Reading room", types:["Archives", "Documents"], description:"Books, encyclopaedias & field guides"},
  {name:"Atlas", types:["Maps"], description:"Maps, geography & the natural world"},
  {name:"Instruments", types:["Apps", "Saved apps"], description:"Applications & saved work"},
  {name:"Intelligence", types:["Local AI", "Skills"], description:"Local models, skills & procedures"},
  {name:"Libraries", types:["Libraries","Datasets"], description:"Software, source & documentation"},
  {name:"Tools", types:["Tools"], description:"Utilities & shell access"},
  {name:"Settings", types:["Settings"], description:"Drive configuration"}
];
const legacyRoutes = new Map();
let indexCollapsed;
function collectionFilter(collection) {
  return collection.types.length > 1 ? "collection:" + collection.name : collection.types[0];
}
function collectionURL(collection) {
  return "#overview?" + new URLSearchParams({type:collectionFilter(collection)});
}
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
let pollTimer;
let searchRead = 0;
let articleService = "";
let articleOpen = false;
let searchRequest = 0;
let shownQuery = "";
let queuedLaunch;
const shellItem = {label: "Terminal", group: "Tools", icon: "terminal", terminal: true,
  description: "Open your shell in the vault folder."};
let terminalKey = readTerminalKey();
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
  return [item.group, item.label, item.summary, item.description, item.note, item.savedPath, item.recipeId, item.id, item.format, ...tags]
    .filter(Boolean).join(" ").toLowerCase();
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

const formats = {browse:"ZIM", documents:"Document", apps:"HTML", workspace:"HTML", maps:"Map", wiki:"Wiki", chat:"Model", library:"Library", skill:"Skill", terminal:"CLI", settings:"Action", share:"Service"};
function formatOf(item) { return item.format || formats[item.icon] || overviewType(item); }
const sizeNumber = new Intl.NumberFormat(undefined, {maximumFractionDigits:1});
function sizeOf(item) {
  if (!item.bytes) return "—";
  const power = Math.min(3, Math.floor(Math.log(item.bytes) / Math.log(1024)));
  return sizeNumber.format(item.bytes / 1024 ** power) + " " + ["B","KiB","MiB","GiB"][power];
}
function overviewRow(item, grouped) {
  const row = node("li", "overview-row");
  const link = node("a", "register-link");
  link.href = "#" + item.route;
  const accession = item.id || item.savedPath || "—";
  const id = node("span", "register-id", accession.length>22 ? accession.slice(0,12)+"…"+accession.slice(-6) : accession);
  id.title = accession;
  const content = node("div", "overview-copy");
  const copy = node("div");
  const title = node(grouped ? "h3" : "h2", "overview-name", displayTitle(item));
  copy.append(title);
  const summary = item.summary || item.description;
  if (summary && summary !== item.label) copy.append(node("p", "overview-summary", summary));
  if(item.missing) copy.append(node("p","error","File missing from drive"));
  content.append(copy);

  link.setAttribute("aria-label", "Open " + item.label);
  link.append(id, content, node("span","register-format",formatOf(item)), node("span","register-size",sizeOf(item)));
  if (item.service) link.lastElementChild.append(serviceMark(item.service));
  link.addEventListener("click", event => {
    if (event.button || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    history.replaceState({...history.state, selected:item.route, scroll:window.scrollY}, "");
    history.pushState({catalogue:location.hash,returnState:history.state}, "", link.hash);
    renderRoute();
    window.scrollTo(0,0);
    byId("item-title").focus({preventScroll:true});

  });
  row.append(link);
  return row;
}

function buildNavigation(data) {
  inventory = data;
  initIndex();
  byId("vault").textContent = data.drive + (data.preset ? " · " + data.preset : "");
  byId("search-nav").hidden = !data.hasSearch;

  data.groups.forEach((group, index) => {
    const items = group.items.filter(item => item.kind !== "search");
    if (!items.length) return;
    items.forEach((item, position) => {
      const legacy = "item-" + index + "-" + position;
      const stable = "resource/" + encodeURIComponent(group.id || group.label) + "/" + encodeURIComponent(item.id || item.source || item.label);
      const entry = {...item, group: group.label, route: entries.has(stable) ? legacy : stable,
        terminal: item.terminal || (item.kind === "terminal" && Boolean(item.command))};
      legacyRoutes.set(legacy, entry.route);
      entries.set(entry.route, entry);
    });
  });
  syncOverviewTypes();
}

function syncOverviewTypes() {
  const items = Array.from(entries.values());
  renderCollections(items);
  for (const [id,label,values] of [["overview-subject","All subjects",items.flatMap(item=>item.tags || [])], ["overview-format","All formats",items.map(formatOf)]]) {
    const select = byId(id), previous = select.value;
    select.replaceChildren(new Option(label,""), ...Array.from(new Set(values)).sort(nameOrder.compare).map(value=>new Option(value,value)));
    if (values.includes(previous)) select.value = previous;
  }
  const control = byId("overview-type");
  const selected = control.value;
  const available = new Set(Array.from(entries.values(), overviewType));
  const types = overviewTypes.filter(type => available.delete(type));
  types.push(...Array.from(available).sort(nameOrder.compare));
  control.replaceChildren(new Option("All types", ""), ...types.map(type => new Option(type, type)));
  const group = node("optgroup");
  group.label = "Collections";
  for (const collection of collections) {
    if (collection.types.length > 1 && collection.types.some(type => types.includes(type))) {
      group.append(new Option(collection.name, "collection:" + collection.name));
    }
  }
  if (group.childElementCount) control.append(group);
  if ([...control.options].some(option => option.value === selected)) control.value = selected;
}

function renderCollections(items) {
  const counts = new Map();
  for (const item of items) {
    const type = overviewType(item);
    counts.set(type, (counts.get(type) || 0) + 1);
  }
  const knownTypes = new Set(collections.flatMap(collection => collection.types));
  const additional = [...counts.keys()].filter(type => !knownTypes.has(type)).map(name => ({name, types:[name]}));
  const available = [...collections, ...additional].filter(collection => collection.types.some(type => counts.has(type)));
  byId("collections").replaceChildren(...available.map(collection => {
    const {name, types, description} = collection;
    const count = types.reduce((sum,type) => sum + (counts.get(type) || 0),0);
    const line = node("li");
    const link = node("a", "collection-link");
    link.href = collectionURL(collection);
    link.append(node("span", "collection-name", name), node("span", "collection-count", String(count).padStart(2, "0")));
    if (description) link.append(node("span", "collection-description", description));
    line.append(link);
    return line;
  }));
  renderIndex(available,items);
  byId("cover-total").textContent = items.length + (items.length === 1 ? " entry" : " entries") + " in the catalogue";
}

function renderHome() {
  const items = Array.from(entries.values());
  const count = items.length;

  byId("home-content").hidden = !count;
  const terms = byId("overview-filter").value.trim().toLowerCase().split(/\s+/);
  const query = byId("overview-filter").value.trim();
  const articles = byId("article-search");
  articles.hidden = !inventory?.hasSearch || !query;
  articles.href = "#search?" + new URLSearchParams({q:query});
  const selected = byId("overview-type").value;
  const collection = collections.find(group => selected === "collection:" + group.name);
  const visible = items.filter(item => {
    if (selected && !(collection ? collection.types.includes(overviewType(item)) : overviewType(item) === selected)) return false;
    if (byId("overview-subject").value && !item.tags?.includes(byId("overview-subject").value)) return false;
    if (byId("overview-format").value && formatOf(item) !== byId("overview-format").value) return false;
    const description = query ? itemSearchText(item) : "";
    return terms.every(term => description.includes(term));
  });
  visible.sort((a, b) => nameOrder.compare(a.label, b.label));
  const output = byId("home-links");
  output.replaceChildren();
  const resultCount = visible.length;
  const chapter = collections.find(group => collectionFilter(group) === selected);
  byId("home-title").textContent = chapter?.name || selected || "Catalogue";
  note(byId("collection-description"), chapter?.description || "");
  note(byId("inventory-summary"), count ? resultCount + (resultCount === 1 ? " entry" : " entries") : "Empty catalogue");
  byId("clear-filters").hidden = !selected && !query && !byId("overview-format").value && !byId("overview-subject").value;
  const filtered = query || byId("overview-format").value || byId("overview-subject").value;
  note(byId("overview-results"), filtered && !resultCount ? "No matches. Try another term or clear a filter." : "");

  const grouped = byId("overview-order").value === "grouped";
  const types = grouped ? Array.from(byId("overview-type").options, option => option.value).filter(type => type && !type.startsWith("collection:")) : [""];
  for (const type of types) {
    const rows = grouped ? visible.filter(item => overviewType(item) === type) : visible;
    if (!rows.length) continue;
    const section = grouped ? node("section", "register-group") : output;
    if (grouped) {
      const heading = node("h2", "overview-heading", type);
      section.append(heading);
      output.append(section);
    }
    const list = node("ul", "overview-rows");
    list.setAttribute("role", "list");
    list.append(...rows.map(item => overviewRow(item, grouped)));
    section.append(list);
  }
}

function markSelection() {
  for(const link of document.querySelectorAll(".resource-link[aria-current]")) if(link.hash!=="#"+selection?.route) link.removeAttribute("aria-current");
  const type = selection ? overviewType(selection) : byId("overview-type").value;
  const current = collections.find(c => collectionFilter(c) === type || c.types.includes(type));
  for (const link of document.querySelectorAll(".collection-label")) {
    if ((selection || route === "overview") && link.dataset.collection === (current ? collectionFilter(current) : type)) link.setAttribute("aria-current","location");
    else link.removeAttribute("aria-current");
  }
  const view = articleOpen ? "search" : selection && route !== "terminal" ? "overview" : route;
  for (const link of byId("work-nav").querySelectorAll("a")) {
    if (link.hash === "#" + view) link.setAttribute("aria-current", route === view ? "page" : "location");
    else link.removeAttribute("aria-current");
  }
}

function isActive(state) {
  return ["starting", "running", "stopping"].includes(state?.status);
}

function updateServiceMark(mark, state) {
  mark.hidden = !isActive(state);
  const label = serviceUnknown ? "Unknown" : state?.status === "running" ? "Served" : serviceStatus[state?.status] || "";
  mark.dataset.status=serviceUnknown?"unknown":state?.status || "stopped";
  mark.textContent = label;
  mark.title = label;
  mark.setAttribute("aria-label", label);
}

function focusControl() {
  const el=button("Focus view","quiet-button",async()=>{
    if(document.fullscreenElement) await document.exitFullscreen();
    else if(document.body.classList.contains("reader-focus"))document.body.classList.remove("reader-focus");
    else if(document.fullscreenEnabled) {
      try {await byId("viewport").requestFullscreen();} catch {document.body.classList.add("reader-focus");}
    } else document.body.classList.add("reader-focus");
    syncFocusLabels();
  });
  el.dataset.focusView="";
  el.textContent=document.fullscreenElement || document.body.classList.contains("reader-focus") ? "Exit focus view":"Focus view";
  return el;
}
function syncFocusLabels() {
  for(const el of document.querySelectorAll("[data-focus-view]"))el.textContent=document.fullscreenElement || document.body.classList.contains("reader-focus") ? "Exit focus view":"Focus view";
}
document.addEventListener("fullscreenchange",syncFocusLabels);
document.addEventListener("keydown",event=>{if(event.key==="Escape" && document.body.classList.contains("reader-focus")){document.body.classList.remove("reader-focus");syncFocusLabels();}});
function serviceDisclosure(container, id, state, stop) {
  const panel = node("div", "service-panel");
  panel.id = id;
  panel.hidden = true;
  const toggle = button("Inspector", "inspector-toggle", () => {
    panel.hidden = !panel.hidden;
    toggle.setAttribute("aria-expanded", String(!panel.hidden));
  });
  toggle.setAttribute("aria-expanded", "false");
  toggle.setAttribute("aria-controls", id);
  toggle.setAttribute("aria-label", "Inspector");
  const item=selection;
  for(const [label,value] of [["Accession",item?.id || item?.savedPath],["Path",item?.source || item?.savedPath || item?.parts?.[0]?.path],["Format",item && formatOf(item)],["Size",item?.bytes && sizeOf(item)],["Recipe",item?.recipeId]]) {
    if(value)panel.append(node("span","service-state",label+" · "+value));
  }
  if(item?.id)panel.append(button("Copy ID","quiet-button",async event=>{const el=event.currentTarget;try{await navigator.clipboard.writeText(item.id);el.textContent="Copied";}catch{el.textContent=item.id;}}));
  if(state) {
    panel.append(node("span", "service-state", "Service · " + registerState(state.status)));
    const service=item?.service || articleService;
    const records=registry.records.filter(r=>r.keys.includes(service));
    const affected=[...new Set(records.flatMap(r=>r.resources))];
    if(affected.length>1)panel.append(node("span","service-state","Shared by "+affected.join(", ")));
    panel.append(node("span","service-state","Stopping interrupts other open views of this resource."));
    if(stop)panel.append(stop);
    if(records.length===1 && records[0].status==="running")panel.append(button("Restart service","quiet-button",()=>controlServices("restart",records[0])));
    const register=node("a","button quiet-button","Service register");register.href="#services";panel.append(register);
  }
  container.append(toggle, panel);
}

function showState(key, state) {
  if (JSON.stringify(states.get(key)) === JSON.stringify(state)) return;
  states.set(key, state);
  for (const badge of document.querySelectorAll('[data-service="' + CSS.escape(key) + '"]')) updateServiceMark(badge, state);
  if (selection?.service === key && !articleOpen) renderItem();
  if (state.status === "stopped" && queuedLaunch?.service === key) {
    const item = queuedLaunch;
    queuedLaunch = undefined;
    if (selection === item) act(item);
  }
  if (articleOpen && articleService === key && unavailableReader(state)) {
    byId("reader-frame")?.remove();byId("archive-text")?.remove();byId("archive-tools")?.remove();
    const message=byId("reader-status");note(message,"Service stopped. ");
    message.append(button("Reopen","quiet-button",renderRoute));
  }
}

function schedulePoll(delay) {
  clearTimeout(pollTimer);
  if (!document.hidden) pollTimer = setTimeout(poll, delay);
}

async function poll() {
  if (polling || document.hidden) return;
  polling = true;
  const connection = byId("connection");
  const previouslyUnknown=serviceUnknown;
  try {
    registry = await request("/api/registry");
    serviceUnknown = false;
    for (const [key, state] of Object.entries(registry.jobs)) showState(key, state);
    connection.classList.remove("offline");
    connection.lastElementChild.textContent = "Local session";
  } catch {
    serviceUnknown=true;
    connection.classList.add("offline");
    connection.lastElementChild.textContent = "Disconnected";
  } finally {
    if(serviceUnknown!==previouslyUnknown) {
      for(const badge of document.querySelectorAll("[data-service]"))updateServiceMark(badge,states.get(badge.dataset.service));
      if(selection?.service && !articleOpen)renderItem();
    }
    renderServices();
    polling = false;
    const active = [...states.values()].some(isActive);
    schedulePoll(active || route === "services" || byId("service-drawer").open ? 1000 : 10000);
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
  if (result) { showState(item.service, result); schedulePoll(250); }
}

function closeReader() {
  byId("reader").hidden = true;
  delete byId("viewport").dataset.reader;
  byId("reader-frame")?.remove();
  byId("archive-tools")?.remove();
  byId("archive-text")?.remove();
  byId("archive-results")?.remove();
  byId("reader").classList.remove("archive");
  byId("search-disclosure").replaceChildren();
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
  if (frame?.dataset.original !== url.href) {
    frame?.remove();
    byId("archive-tools")?.remove();
    byId("archive-text")?.remove();
    byId("archive-results")?.remove();
    frame = node("iframe");
    frame.id = "reader-frame";
    frame.dataset.original = url.href;
    frame.referrerPolicy = "no-referrer";
    const pdf = url.pathname.toLowerCase().endsWith(".pdf");
    let embedded = url;
    if (kind === "documents" && pdf) {
      embedded = new URL("/.svalbard-reader/", url);
      embedded.searchParams.set("file", url.pathname + url.hash);
    } else if (!pdf) {
      frame.setAttribute("sandbox", "allow-scripts allow-same-origin allow-forms allow-downloads");
      if (kind === "browse" && url.pathname === "/viewer" && url.hash) {
        embedded = new URL("/content/" + encodeURIComponent(decodeURIComponent(url.hash.slice(1))) + "/", url);
      }
    }
    const archive = kind === "browse" && !pdf && selection?.source?.endsWith(".zim");
    byId("reader").classList.toggle("archive", Boolean(archive));
    if (archive) archiveToolbar(selection, frame, fromSearch);
    if (!archive) frame.src = embedded.href;
    else if(!frame.hidden) {frame.hidden=true;adaptArchive(selection,frame,url);}
    byId("reader").append(frame);
  }
  frame.title = label;
  byId("reader").hidden = false;
  byId("reader-bar").hidden = !fromSearch;
  byId("reader-link").href = url.href;
  byId("reader-title").textContent = label;
  byId("viewport").dataset.reader = fromSearch ? "search" : "item";
  return true;
}

function archiveToolbar(item, frame, fromSearch) {
  const tools = node("form", "archive-tools");
  tools.id = "archive-tools";
  const home = node("a", "button quiet-button", "Archive home");
  home.href = "#" + item.route;
  const field = node("input");
  field.type = "search"; field.required = true; field.maxLength = 500;
  field.placeholder = "Search this archive…"; field.setAttribute("aria-label", "Search this archive");
  const submit = node("button", "button secondary", "Search");
  submit.type = "submit";
  tools.append(home, field, submit);
  const results = node("div", "archive-results");
  results.id = "archive-results";
  byId("reader").append(tools, results);
  home.onclick = event => {
    if (location.hash === home.hash) { event.preventDefault(); renderRoute(); }
  };
  tools.onsubmit = event => {
    event.preventDefault();
    const query = field.value.trim();
    if (query) location.hash = item.route + "?" + new URLSearchParams({within:query});
  };
  const query = new URLSearchParams(location.hash.split("?")[1]).get("within");
  if (fromSearch || !query) return;
  field.value = query;
  frame.hidden = true;
  results.setAttribute("role", "region");
  results.setAttribute("aria-label", "Archive results");
  results.textContent = "Searching archive…";
  request("/api/archives/" + item.service + "/search", {query}).then(hits => {
    if (!results.isConnected) return;
    results.replaceChildren(node("p","catalog-count", hits?.length ? hits.length + " results" : "No matching articles."));
    const list = node("ol", "results");
    for (const hit of hits || []) {
      const row = node("li");
      const link = node("a", "result-link", hit.Title || hit.Path);
      link.href = "#" + item.route + "?" + new URLSearchParams({source:item.source,path:hit.Path,label:hit.Title});
      link.onclick = event => {
        if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
        event.preventDefault();
        history.pushState({fromArchiveSearch:true}, "", link.hash);
        renderRoute();
      };
      row.append(link, node("p", "", hit.Snippet));
      list.append(row);
    }
    results.append(list);
  }).catch(error => note(results, error.message, true));
}

// Colour a bounded opening excerpt; long minified bundles stay cheap to display.
function highlightSource(body, text) {
  const first = text.slice(0, 16000);
  const pattern = /("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')|(\/\/[^\n]*|\/\*[\s\S]*?\*\/)|\b(?:async|await|break|case|class|const|default|else|export|false|for|function|if|import|let|new|null|return|throw|true|typeof|var|while)\b/g;
  const fragments = document.createDocumentFragment();
  let offset = 0;
  for (const match of first.matchAll(pattern)) {
    fragments.append(first.slice(offset, match.index), node("span", match[1] ? "literal" : match[2] ? "comment" : "keyword", match[0]));
    offset = match.index + match[0].length;
  }
  fragments.append(text.slice(offset));
  body.replaceChildren(fragments);
}

function sourceSection(part) {
  if (part.label.startsWith("Examples/")) return "Examples";
  if (part.label.startsWith("references/")) return "References";
  if (part.label.startsWith("scripts/") || part.label === "Source") return "Files";
  return part.label;
}
async function renderSource(item, section, selectedPart) {
  const panel = byId("source-view");
  const sections = [...new Set([...(item.icon === "library" ? ["Overview"] : []), ...item.parts.map(sourceSection)])];
  section ??= sections[0];
  const parts = item.parts.filter(part=>sourceSection(part)===section);
  const active = selectedPart || parts[0];
  const tabs = node("nav", "source-tabs");
  tabs.setAttribute("aria-label", "Resource views");
  for (const name of sections) {
    const tab = button(name, "secondary", () => renderSource(item,name));
    tab.setAttribute("aria-pressed", String(name === section));
    tabs.append(tab);
  }
  panel.hidden=false;
  panel.replaceChildren(tabs);
  if (!active) {
    const overview=node("article","prose");
    overview.append(node("h2","",item.label),node("p","",item.description || item.note),node("p","quiet","No overview document is installed with this package."));
    panel.append(overview);
    return;
  }
  if (item.icon==="library" && section==="Files") {
    const tree=node("details","source-tree");tree.append(node("summary","","Package files"));
    for(const part of parts)tree.append(button(part.path,"quiet-button",()=>renderSource(item,section,part)));
    panel.append(tree);
  } else if(parts.length>1) {
    const picker=node("select");picker.setAttribute("aria-label","File");
    parts.forEach((part,i)=>picker.add(new Option(part.path,String(i))));
    picker.selectedIndex=parts.indexOf(active);
    picker.onchange=()=>renderSource(item,section,parts[picker.selectedIndex]);
    panel.append(picker);
  }
  const file = node("p", "item-recipe", active.path);
  const isMarkdown=/\.md$/i.test(active.path);
  const body = node(isMarkdown ? "article" : "pre", isMarkdown ? "prose" : "source-text", "Loading…");
  panel.append(file,body);
  try {
    const data = await request("/api/source?" + new URLSearchParams({id:item.id, view:active.label}));
    if (!body.isConnected) return;
    body.tabIndex = 0;
    if (/\.(m?js|json|cjs)$/.test(active.path)) highlightSource(body, data.text);
    else if (isMarkdown) {
      const {prose}=await import("/views.js");
      const fragment=prose(data.text);
      const base=new URL("/"+active.path,"https://source.invalid");
      for(const link of fragment.querySelectorAll("a[href]")) {
        const address=new URL(link.getAttribute("href"),base);
        if(address.origin!==base.origin)continue;
        const path=decodeURIComponent(address.pathname.slice(1));
        const part=item.parts.find(part=>part.path===path);
        link.removeAttribute("target");
        if(part){
          link.href="#"+item.route;
          link.onclick=event=>{event.preventDefault();if(part===active && address.hash) body.querySelector("#"+CSS.escape(decodeURIComponent(address.hash.slice(1))))?.scrollIntoView();else renderSource(item,sourceSection(part),part);};
        }else link.removeAttribute("href");
      }
      body.replaceChildren(fragment);
      if(item.format==="MD"||item.icon==="skill"){
        const headings=[...body.querySelectorAll("h2,h3")];
        if(headings.length>1) {
          const contents=node("details","article-contents");contents.append(node("summary","","Contents"));
          for(const heading of headings)contents.append(button(heading.textContent,"quiet-button",()=>heading.scrollIntoView()));
          body.prepend(contents);
        }
      }
      const raw=button("Original text","quiet-button",()=>{body.replaceChildren(node("pre","source-text",data.text));raw.remove();});
      tabs.append(raw);
      const example=[...body.querySelectorAll("h2,h3")].find(heading=>/^examples?$/i.test(heading.textContent.trim()));
      if(example && !sections.includes("Examples")) tabs.append(button("Examples","secondary",()=>{example.tabIndex=-1;example.focus();}));
    } else body.textContent = data.text;
    if (section==="Examples" && /\.html?$/i.test(active.path) && !data.truncated) {
      const preview=button("Preview markup","secondary",()=>{
        const frame=node("iframe","example-frame");
        frame.setAttribute("sandbox","");frame.title=active.path;
        frame.srcdoc='<meta http-equiv="Content-Security-Policy" content="default-src &apos;none&apos;; style-src &apos;unsafe-inline&apos;">'+data.text;
        body.after(frame);preview.remove();
      });
      tabs.append(preview);
    }
    if (data.truncated) file.append(" · First 256 KiB");
    const copy = button("Copy text", "quiet-button", async () => {
      try { await navigator.clipboard.writeText(data.text); copy.textContent = "Copied"; }
      catch { body.focus(); }
    });
    tabs.append(copy);
  } catch (error) {note(body, error.message, true);}
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

function renderItem() {
  const item = selection;
  const controls = byId("item-controls");
  const focused = controls.contains(document.activeElement);
  const status = byId("item-status");
  const output = byId("item-output");
  byId("terminal-panel").hidden = !item.terminal;
  controls.replaceChildren();
  note(status, "");
  note(output, "");
  if (item.missing) { serviceDisclosure(controls,"item-inspector-panel"); note(status,"The installed file is missing. Restore it from the host, then reopen the drive.",true);return; }
  if (item.icon === "library" || item.kind === "dataset" || ["EPUB","MD","PDF"].includes(item.format)) {
    const original=node("a","button secondary", item.icon === "library" ? "Download library archive" : "Download original");
    original.href="/api/download?"+new URLSearchParams({id:item.id});original.download="";controls.append(original);
  }
  if (["EPUB","MD"].includes(item.format)) {controls.append(focusControl());serviceDisclosure(controls,"item-inspector-panel");return;}
  if (!item.service) {
    if (item.parts?.length || item.kind==="dataset")controls.append(focusControl());
    serviceDisclosure(controls,"item-inspector-panel");
    if (item.terminal) {
      renderTerminal();
    } else if (item.command) {
      note(status, "Run this action from the drive’s terminal.");
      note(output, item.command);
    }
    return;
  }

  const state = states.get(item.service) || {status:serviceUnknown?"unknown":"stopped"};
  byId("item-kind").textContent=[formatOf(item),item.bytes && sizeOf(item),serviceUnknown?"Service unknown":state.status==="running"?"● Served":""].filter(Boolean).join(" · ");
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
      const label = reading ? "Open original" : link.label;
      controls.append(externalLink(label, address));
    }
  }
  if (!reading) closeReader();

  if (active) {
    const stop = button(reading ? "Stop serving" : "Stop", "quiet-button", () => {
      const record=registry.records.find(r=>r.keys.includes(item.service));
      if(record?.inference && !confirm("Stop this service and interrupt ongoing inference?"))return;
      if(record?.keys.length>1 && !confirm("Release "+displayTitle(item)+"? Other views of this resource will be interrupted. Shared resources: "+record.resources.join(", ")))return;
      act(item,true);
    });
    stop.disabled = state.status === "stopping";
    if(reading) controls.append(focusControl());

    serviceDisclosure(controls, "item-inspector-panel", state, stop);
    if (!reading) note(status, state.status==="starting" && item.icon==="browse" ? "Opening archive…" : serviceStatus[state.status]);
  }
  if (!active) serviceDisclosure(controls,"item-inspector-panel",state);
  if (state.status === "failed") note(status, state.text, true);
  if (state.status === "done") note(output, state.text);
  if (focused) controls.querySelector("button:not(:disabled), a, select")?.focus();
}

let drawerOpener;
function toggleSidebar(open) {
  if(open)drawerOpener=document.activeElement;
  byId("viewport").inert=open;
  byId("crumbs").inert=open;
  document.querySelector(".skip").inert=open;
  byId("mobile-search").hidden=open;
  document.body.classList.toggle("sidebar-open", open);
  byId("menu").setAttribute("aria-expanded", String(open));
  byId("menu").setAttribute("aria-label", open ? "Close drive index" : "Open drive index");
}

function updateRail(collapsed) {
  document.body.classList.toggle("rail", collapsed);
  const toggle = byId("rail-toggle");
  toggle.setAttribute("aria-expanded", String(!collapsed));
  toggle.title = collapsed ? "Expand catalogue margin" : "Collapse catalogue margin";
  toggle.setAttribute("aria-label", toggle.title);
}
byId("rail-toggle").addEventListener("click", event => {
  if (matchMedia("(max-width:768px)").matches) {
    const open=!document.body.classList.contains("sidebar-open");toggleSidebar(open);
    if(open && !event.detail)document.querySelector(".collection-label")?.focus();
    return;
  }
  indexCollapsed = !document.body.classList.contains("rail");
  updateRail(indexCollapsed); storeIndex();
});
function syncCatalogueURL() {
  const params = new URLSearchParams();
  if (byId("overview-filter").value) params.set("q", byId("overview-filter").value);
  if (byId("overview-type").value) params.set("type", byId("overview-type").value);
  if (byId("overview-order").value === "name") params.set("order", "name");
  for (const field of ["subject","format"]) if (byId("overview-"+field).value) params.set(field,byId("overview-"+field).value);
  history.replaceState(null, "", "#overview" + (params.size ? "?" + params : ""));
  renderHome();
  markSelection();
}

function renderRoute() {
  if (!inventory) return;
  const previousRoute = route;
  const returningFromResult = articleOpen;
  articleOpen=false;articleService="";
  const navigation = history.state;
  byId("catalogue-crumb").href = navigation?.catalogue?.startsWith("#overview") ? navigation.catalogue : "#overview";
  const [hashRoute, options] = location.hash.slice(1).split("?");
  route = legacyRoutes.get(hashRoute) || hashRoute || "cover";
  const params = new URLSearchParams(options);
  if (route === "overview") {
    for (const field of ["subject","format"]) byId("overview-"+field).value = params.get(field) || "";
    byId("overview-filter").value = params.get("q") || "";
    byId("overview-type").value = params.get("type") || "";
    byId("overview-order").value = params.get("order") === "name" ? "name" : "grouped";
    renderHome();
  }
  selection = route === "terminal" ? shellItem : entries.get(route);
  articleOpen = Boolean((inventory.hasSearch || selection?.icon === "browse") && params.has("source") && params.has("path") && (route==="read" || (selection?.source===params.get("source") || (selection?.source?.endsWith(".zim") && params.get("source").startsWith(selection.source+"/")))));
  const inSearch = inventory.hasSearch && (route === "search" || articleOpen);
  if (!selection && !inSearch && route !== "overview" && route !== "services") route = "cover";
  updateRail(route === "cover" || (indexCollapsed ?? false));
  byId("cover").hidden = route !== "cover";
  byId("crumbs").hidden = route === "cover";
  const parent = selection && collections.find(group => group.types.includes(overviewType(selection)));
  const crumb = byId("crumb-leaf");
  crumb.hidden = !parent && !inSearch;
  crumb.textContent = parent?.name || (inSearch ? "Search" : "");
  crumb.href = parent ? collectionURL(parent) : "#search";
  byId("source-view").classList.remove("error");
  byId("source-view").hidden = true;

  ++searchRead;
  model = "";
  if (queuedLaunch !== selection) queuedLaunch = undefined;
  closeReader();
  const details = byId("details");
  details.hidden = !selection || articleOpen;
  details.classList.toggle("terminal-page", Boolean(selection?.terminal));
  byId("overview").hidden = route !== "overview";
  byId("services-panel").hidden = route !== "services";
  if (route === "services") { renderServices(); schedulePoll(0); }
  byId("search-panel").hidden = route !== "search";
  note(byId("location"), parent?.name || "");
  markSelection();
  toggleSidebar(false);

  if (articleOpen) {
    openArticle(params,searchRead);
    return;
  }
  if (route === "search") {
    const focusResult = () => {
      if (returningFromResult && route === "search") {
        byId("search-results").querySelectorAll("a")[navigation?.position]?.focus({preventScroll:true});
        window.scrollTo(0,navigation.scroll || 0);
      }
    };
    const searchQuery = params.get("q") ?? navigation?.query;
    if (typeof searchQuery === "string") {
      byId("query").value = searchQuery;
      if (searchQuery !== shownQuery) {
        runSearch().then(focusResult);
      } else {
        focusResult();
      }
    }
    return;
  }

  if (selection) {
    revealResource(selection);
    byId("item-title").textContent = displayTitle(selection);
    byId("item-kind").textContent = [formatOf(selection),selection.bytes && sizeOf(selection)].filter(Boolean).join(" · ");
    byId("item-description").textContent = displayTitle(selection) !== selection.label ? selection.label.slice(displayTitle(selection).length).replace(/^[\s—–-]+/,"") : selection.summary || selection.description || "";
    note(byId("item-recipe"), "");
    const preview = byId("item-preview");
    preview.hidden = !previewURL(selection);
    if (!preview.hidden) preview.replaceChildren(itemPreview(selection, "detail-preview"));
    renderItem();
    const item = selection;
    if (previousRoute !== route && !item.missing && !["EPUB","MD"].includes(item.format) && item.service && readerKinds.has(item.icon)) {
      const status=states.get(item.service)?.status;
      if(status==="stopping")queuedLaunch=item;
      else if(status!=="running" && status!=="starting")act(item);
    }
    const panel=byId("source-view");
    if (item.parts?.length) renderSource(item);
    else {
      const adapter = item.format === "EPUB" ? "ebook" : item.kind === "dataset" ? "dataset" : item.command === "./run settings" ? "preferences" : "";
      if (adapter) {
        panel.hidden=false;panel.replaceChildren();
        import("/views.js").then(views => {
          if (selection!==item) return;
          return adapter==="preferences" ? views.preferences(item,panel,terminalKey) : views[adapter](item,panel);
        }).catch(error=>{if(selection===item)note(panel,error.message,true);});
      }
    }
  }
}

async function refreshPages() {
  const refresh = byId("refresh");
  const status = byId("workspace-status");
  refresh.disabled = true;
  try {
    const pages = await request("/api/workspace");
    for (const [route, entry] of entries) if (entry.savedPath) entries.delete(route);
    for (const page of pages) {
      const entry = {
        route: "saved/" + encodeURIComponent(page.path), label: page.name, note: page.path,
        group: "Saved apps", icon: "workspace", kind: "launch", service: inventory.workspace,
        savedPath: page.path
      };
      entries.set(entry.route, entry);
    }
    note(status, pages.length ? "" : "No saved HTML apps yet.");
    syncOverviewTypes();
    if (route === "overview") renderRoute();
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
  const title = node("a", "result-link", item.title || item.label);
  const owner = [...entries.values()].find(entry=>entry.source===item.source || (entry.source?.endsWith(".zim") && item.source.startsWith(entry.source+"/")));
  const params = new URLSearchParams({source:item.source,path:item.path,label:item.title || item.label});
  title.href = "#" + (owner?.route || "read") + "?" + params;
  title.onclick = event => {
    if(event.button || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey)return;
    event.preventDefault();
    history.replaceState({query:shownQuery,position,scroll:scrollY},"","#search?"+new URLSearchParams({q:shownQuery}));
    history.pushState({query:shownQuery,position,fromSearch:true},"",title.hash);
    window.scrollTo(0,0);renderRoute();
  };
  row.append(title, node("p", "quiet", item.label));
  if (item.snippet) row.append(node("p", "", item.snippet));
  return row;
}

async function openArticle(params,revision) {
  const label=params.get("label") || params.get("source");
  byId("reader").hidden=false;
  byId("reader-bar").hidden=false;
  byId("reader-title").textContent=label;
  byId("reader-title").focus({preventScroll:true});
  byId("reader-link").hidden=true;
  byId("back-to-search").textContent=(history.state?.fromSearch || history.state?.fromArchiveSearch) ? "← Results" : "← Catalogue";
  byId("search-disclosure").replaceChildren();
  note(byId("reader-status"),"Opening original…");
  try {
    const source=params.get("source");
    const link=await request("/api/read",{source,path:params.get("path")});
    if(revision!==searchRead)return;
    if(!showReader(label,link.url,source.startsWith("books/")?"documents":"browse",true))throw Error("The result has no local reader address.");
    articleService=link.service;
    if (link.service && unavailableReader(states.get(link.service))) states.delete(link.service);
    byId("reader-link").hidden=false;
    if (link.service) {
      const stop = button("Stop serving", "quiet-button", event => stopSearchReader(event.currentTarget));
      byId("search-disclosure").append(focusControl());
      serviceDisclosure(byId("search-disclosure"), "search-inspector-panel", {status:"running"}, stop);
      schedulePoll(250);
    }
    note(byId("reader-status"),"");
  } catch(error) { if(revision===searchRead)note(byId("reader-status"),error.message,true); }
}
function leaveArticle() {
  if(history.state?.fromSearch || history.state?.fromArchiveSearch) history.back();
  else location.hash=selection?.route || "overview";
}

function unavailableReader(state) {
  return ["stopping", "stopped", "failed"].includes(state?.status);
}

async function stopSearchReader(stop) {
  const service = articleService;
  if (!service) return;
  stop.disabled = true;
  try {
    const state = await request("/api/services/" + service + "/stop", {});
    showState(service, state);
  } catch (error) {
    if (articleService === service) note(byId("reader-status"), error.message, true);
  } finally {
    stop.disabled = false;
  }
}

async function runSearch() {
  ++searchRead;
  const query = byId("query").value.trim();
  const submitted = ++searchRequest;
  shownQuery = query;
  if (route === "search" && history.state?.query !== query) {
    history.replaceState({query}, "", "#search?" + new URLSearchParams({q:query}));
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
    const count = data.hits.length;
    const summary = count ? "Showing " + count + (count === 1 ? " result" : " results") : "No matching articles or documents.";
    note(status, data.status || summary);
  } catch (error) {
    if (submitted === searchRequest) {
      shownQuery = "";
      note(status, error.message, true);
    }
  } finally {
    if (submitted === searchRequest) button.disabled = false;
  }
}

byId("menu").addEventListener("click", event => {
  const open = !document.body.classList.contains("sidebar-open");
  toggleSidebar(open);
  if (open && !event.detail) byId("work-nav").querySelector("a").focus({preventScroll: true});
});
byId("dismiss-sidebar").addEventListener("click", () => {
  toggleSidebar(false);
  (drawerOpener || byId("menu")).focus();
});
byId("work-nav").addEventListener("click", event => {
  const link = event.target.closest("a");
  if (link && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) {
    const fromMenu = document.body.classList.contains("sidebar-open");
    toggleSidebar(false);
    if (fromMenu) byId("main").focus({preventScroll: true});
    window.scrollTo(0, 0);
  }
});
byId("nav").addEventListener("click", event => {
  if (event.target.closest("a")) toggleSidebar(false);
});
byId("catalogue-crumb").onclick=event=>{
  const origin=history.state;
  if(!origin?.catalogue?.startsWith("#overview") || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey)return;
  event.preventDefault();
  history.pushState(origin.returnState,"",origin.catalogue);renderRoute();
  const link=[...byId("home-links").querySelectorAll("a")].find(a=>a.hash.slice(1)===origin.returnState?.selected);
  (link || byId("home-title")).focus({preventScroll:true});window.scrollTo(0,origin.returnState?.scroll||0);
};
function openGlobalSearch() {
  const target = inventory?.hasSearch ? "search" : "overview";
  if (route !== target) {
    history.pushState(null, "", "#" + target);
    renderRoute();
  }
  byId(target === "search" ? "query" : "overview-filter").focus();
}
byId("mobile-search").addEventListener("click",openGlobalSearch);
matchMedia("(max-width:768px)").addEventListener("change",()=>toggleSidebar(false));
byId("search-shortcut").textContent = /Mac|iPhone|iPad/.test(navigator.platform) ? "⌘ K" : "Ctrl K";
byId("cover-search").addEventListener("submit", event => {
  event.preventDefault();
  const q = byId("cover-query").value.trim();
  location.hash = q ? "overview?" + new URLSearchParams({q}) : "overview";
});
document.addEventListener("keydown", event => {
  if ((event.metaKey || event.ctrlKey) && !event.altKey && event.key.toLowerCase() === "k") {
    event.preventDefault();
    openGlobalSearch();
  }
  if(event.key==="Tab" && document.body.classList.contains("sidebar-open")) {
    const controls=[byId("menu"), ...document.querySelectorAll(".spine a,.spine button, #sidebar a, #sidebar button:not(:disabled)")].filter(el=>el.getClientRects().length && !el.hidden);
    const index=controls.indexOf(document.activeElement);
    event.preventDefault();
    controls[(index+(event.shiftKey?-1:1)+controls.length)%controls.length].focus();
  }
  if (event.key === "Escape" && document.body.classList.contains("sidebar-open")) {
    toggleSidebar(false);
    (drawerOpener || byId("menu")).focus();
  }
});
document.querySelector(".skip").addEventListener("click", event => {
  event.preventDefault();
  byId("main").focus();
});
for (const field of ["subject","format"]) byId("overview-"+field).addEventListener("change",syncCatalogueURL);
byId("clear-filters").onclick=()=>{for (const id of ["filter","type","subject","format"])byId("overview-"+id).value="";syncCatalogueURL();byId("overview-filter").focus();};
byId("overview-filter").addEventListener("input", syncCatalogueURL);
byId("overview-type").addEventListener("change", syncCatalogueURL);
byId("overview-order").addEventListener("change", syncCatalogueURL);
byId("terminal-start").addEventListener("click", startTerminal);
byId("terminal-stop").addEventListener("click", () => selectedTerminal().session?.disconnect());
byId("terminal-paste").addEventListener("click", () => selectedTerminal().session?.paste(selection.command));
window.addEventListener("pagehide", () => {
  for (const state of Object.values(terminals)) state.session?.disconnect();
});
byId("refresh").addEventListener("click", refreshPages);
byId("back-to-search").addEventListener("click",leaveArticle);
byId("search-form").addEventListener("submit", event => {
  event.preventDefault();
  runSearch();
});
window.addEventListener("hashchange", () => {
  renderRoute();
  if (articleOpen) return;
  if (route === "overview") {
    const previous = history.state?.selected;
    const link = previous && [...byId("home-links").querySelectorAll("a")].find(a => a.hash.slice(1) === previous);
    (link || byId("home-title")).focus({preventScroll:true});
    if (link) window.scrollTo(0,history.state.scroll || 0);
  }
  else if (route === "services") byId("services-title").focus({preventScroll:true});
  else if (selection) byId("item-title").focus({preventScroll:true});
  else if (route === "search" && history.state?.position == null) byId("query").focus({preventScroll:true});
});

request("/api/inventory")
  .then(data => {
    initServiceDrawer();
    buildNavigation(data);
    renderRoute();
    refreshPages();
    poll();
    renderServices();
    document.addEventListener("visibilitychange", () => schedulePoll(0));
  })
  .catch(error => note(byId("cover-total"), error.message, true));
