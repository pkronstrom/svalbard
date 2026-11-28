/* Optional readers load only after the user opens their resource. */
import DOMPurify from "/vendor/markdown/purify.mjs";
import { marked } from "/vendor/markdown/marked.mjs";

export function prose(text, markdown = true) {
  const fragment = DOMPurify.sanitize(markdown ? marked.parse(text) : text, {
    RETURN_DOM_FRAGMENT: true,
    ALLOWED_TAGS:
      "p a img h1 h2 h3 h4 h5 h6 blockquote pre code ol ul li em strong br hr table thead tbody tr th td dl dt dd figure figcaption sup sub span section div b i".split(
        " "
      ),
    ALLOWED_ATTR: ["href", "src", "alt", "title", "id", "colspan", "rowspan"],
    ALLOW_DATA_ATTR: false
  });
  for (const link of fragment.querySelectorAll("a[href]")) {
    link.target = "_blank";
    link.rel = "noopener noreferrer";
  }
  return fragment;
}

// The reading position and text size live in this browser, per drive and book.
const ebookScales = [0.9, 1, 1.1, 1.2, 1.35, 1.5];
const blockSelector = "p,h1,h2,h3,h4,h5,h6,li,blockquote,pre,figure,table,hr,dt,dd";

export async function ebook(item, panel) {
  const base = "/api/ebook?" + new URLSearchParams({ source: item.source });
  const book = await request(base);
  if (selection !== item) return;
  const chapters = book.chapters || [];
  if (!chapters.length) throw Error("This edition has no readable chapters. Use the original file.");
  const paths = chapters.map((part) => part.path);
  const sizes = chapters.map((part) => Math.max(1, part.bytes || 1));
  const total = sizes.reduce((sum, size) => sum + size, 0);
  const contents = book.contents?.length
    ? book.contents
    : chapters.map((part, i) => ({ label: "Section " + (i + 1), path: part.path, depth: 0 }));
  const chapterTitle = (index) =>
    (contents.find((entry) => entry.path === paths[index] && !entry.anchor) ||
      contents.find((entry) => entry.path === paths[index]))?.label || "Section " + (index + 1);
  const positionKey = "svalbard-epub:" + inventory.drive + ":" + item.source;
  const scaleKey = "svalbard-epub-scale";
  const stored = (key) => { try { return JSON.parse(localStorage.getItem(key)); } catch { return null; } };
  const store = (key, value) => { try { localStorage.setItem(key, JSON.stringify(value)); } catch {} };

  const root = node("div", "ebook");
  const bar = node("div", "ebook-bar");
  const toggle = button("Contents", "quiet-button ebook-toggle", () => {
    const open = root.classList.toggle("contents-open");
    toggle.setAttribute("aria-expanded", String(open));
    if (open) toc.querySelector("[aria-current]")?.focus();
  });
  toggle.setAttribute("aria-expanded", "false");
  const previous = button("←", "quiet-button", () => open(current - 1));
  previous.setAttribute("aria-label", "Previous chapter");
  const next = button("→", "quiet-button", () => open(current + 1));
  next.setAttribute("aria-label", "Next chapter");
  const place = node("p", "ebook-place");
  place.setAttribute("aria-live", "polite");
  const smaller = button("A−", "quiet-button", () => scale(-1));
  smaller.setAttribute("aria-label", "Smaller text");
  const larger = button("A+", "quiet-button", () => scale(1));
  larger.setAttribute("aria-label", "Larger text");
  const sizing = node("div", "ebook-sizing");
  sizing.append(smaller, larger);
  const meter = node("div", "ebook-meter");
  const fill = node("span");
  meter.append(fill);
  meter.setAttribute("aria-hidden", "true");
  bar.append(toggle, previous, next, place, sizing, meter);

  const toc = node("nav", "ebook-contents");
  toc.setAttribute("aria-label", "Book contents");
  const tocList = node("ol");
  for (const entry of contents) {
    const row = node("li");
    row.style.setProperty("--depth", String(Math.min(entry.depth || 0, 4)));
    const go = node("a", "", entry.label);
    go.href = "#" + item.route;
    go.dataset.path = entry.path;
    go.onclick = (event) => {
      event.preventDefault();
      root.classList.remove("contents-open");
      toggle.setAttribute("aria-expanded", "false");
      open(paths.indexOf(entry.path), { anchor: entry.anchor });
    };
    row.append(go);
    tocList.append(row);
  }
  toc.append(tocList);

  const resume = node("p", "ebook-resume");
  resume.hidden = true;
  const article = node("article", "prose ebook-text");
  if (book.language) article.lang = book.language;
  const turn = node("div", "ebook-turn");
  const body = node("div", "ebook-body");
  const column = node("div", "ebook-column");
  column.append(resume, article, turn);
  body.append(toc, column);
  root.append(bar, body);
  panel.replaceChildren(root);

  let current = -1;
  let revision = 0;
  let scaleIndex = ebookScales.indexOf(stored(scaleKey));
  if (scaleIndex < 0) scaleIndex = 1;
  function scale(step) {
    const block = visibleBlock();
    scaleIndex = Math.min(ebookScales.length - 1, Math.max(0, scaleIndex + step));
    article.style.setProperty("--ebook-scale", String(ebookScales[scaleIndex]));
    smaller.disabled = scaleIndex === 0;
    larger.disabled = scaleIndex === ebookScales.length - 1;
    store(scaleKey, ebookScales[scaleIndex]);
    if (step) reveal(block);
  }
  scale(0);

  function blocks() { return article.querySelectorAll(blockSelector); }
  function barBottom() { return Math.max(0, bar.getBoundingClientRect().bottom); }
  function visibleBlock() {
    const top = barBottom() + 4;
    const all = blocks();
    for (let i = 0; i < all.length; i++) if (all[i].getBoundingClientRect().bottom > top) return i;
    return 0;
  }
  function reveal(index) {
    const target = blocks()[index];
    if (!target) return;
    window.scrollTo(0, Math.max(0, target.getBoundingClientRect().top + window.scrollY - barBottom() - 12));
  }
  function progress() {
    const rect = article.getBoundingClientRect();
    const read = Math.min(1, Math.max(0, (barBottom() - rect.top) / Math.max(1, rect.height - innerHeight / 2)));
    const done = sizes.slice(0, current).reduce((sum, size) => sum + size, 0) + read * sizes[current];
    return Math.round((done / total) * 100);
  }
  function mark() {
    const percent = progress();
    fill.style.width = percent + "%";
    place.textContent = chapterTitle(current) + " · " + percent + "%";
    return percent;
  }

  // The drive keeps the position so it survives a new launch on any computer;
  // this browser keeps a copy for read-only drives.
  let saveTimer, unsaved = null;
  function persist(keepalive = false) {
    if (!unsaved) return;
    const position = unsaved;
    unsaved = null;
    fetch("/api/reading?" + new URLSearchParams({ source: item.source }), {
      method: "POST", keepalive,
      headers: { "Content-Type": "application/json", "X-Svalbard-Request": "1" },
      body: JSON.stringify({ chapter: position.chapter, block: position.block, percent: position.percent })
    }).catch(() => {});
  }
  function remember() {
    clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      if (!root.isConnected || current < 0) return;
      unsaved = { chapter: paths[current], block: visibleBlock(), percent: mark(), at: Date.now() };
      store(positionKey, unsaved);
      persist();
    }, 1200);
  }
  const listening = new AbortController();
  window.addEventListener("scroll", () => {
    if (!root.isConnected) return listening.abort();
    mark();
    remember();
  }, { passive: true, signal: listening.signal });
  document.addEventListener("keydown", (event) => {
    if (!root.isConnected) return listening.abort();
    if (event.altKey || event.ctrlKey || event.metaKey || event.target.closest?.("input,select,textarea,[contenteditable]")) return;
    if (event.key === "ArrowRight" && current < paths.length - 1) { event.preventDefault(); open(current + 1); }
    if (event.key === "ArrowLeft" && current > 0) { event.preventDefault(); open(current - 1); }
  }, { signal: listening.signal });
  window.addEventListener("pagehide", () => persist(true), { signal: listening.signal });

  async function open(index, { anchor = "", block = 0, initial = false } = {}) {
    if (index < 0 || index >= paths.length) return;
    const attempt = ++revision;
    previous.disabled = index === 0;
    next.disabled = index === paths.length - 1;
    article.setAttribute("aria-busy", "true");
    try {
      const data = await request(base + "&file=" + encodeURIComponent(paths[index]));
      if (attempt !== revision || !article.isConnected) return;
      const fragment = prose(data.text, false);
      const local = new URL("/" + paths[index], "https://book.invalid");
      for (const element of fragment.querySelectorAll("img[src],a[href]")) {
        const image = element.tagName === "IMG";
        const attr = image ? "src" : "href";
        const address = new URL(element.getAttribute(attr), local);
        element.removeAttribute(attr);
        if (address.origin !== local.origin) {
          if (!image && ["https:", "http:"].includes(address.protocol)) element.href = address.href;
          continue;
        }
        const file = decodeURIComponent(address.pathname.slice(1));
        if (image) {
          element.src = base + "&file=" + encodeURIComponent(file);
          element.loading = "lazy";
        } else if (paths.includes(file)) {
          element.removeAttribute("target");
          element.href = "#" + item.route;
          element.onclick = (event) => {
            event.preventDefault();
            open(paths.indexOf(file), { anchor: decodeURIComponent(address.hash.slice(1)) });
          };
        }
      }
      current = index;
      article.replaceChildren(fragment);
      article.removeAttribute("aria-busy");
      for (const link of tocList.querySelectorAll("a")) {
        if (link.dataset.path === paths[index]) link.setAttribute("aria-current", "true");
        else link.removeAttribute("aria-current");
      }
      turn.replaceChildren();
      if (index < paths.length - 1) turn.append(button("Next · " + chapterTitle(index + 1) + " →", "secondary", () => open(index + 1)));
      else turn.append(node("p", "ebook-end", "End of the book"));
      const target = anchor && article.querySelector("#" + CSS.escape(anchor));
      if (target) window.scrollTo(0, target.getBoundingClientRect().top + window.scrollY - barBottom() - 12);
      else if (block) reveal(block);
      else if (!initial) window.scrollTo(0, Math.max(0, root.getBoundingClientRect().top + window.scrollY - 8));
      mark();
      remember();
    } catch (error) {
      if (attempt === revision) note(article, error.message, true);
    }
  }

  const local = stored(positionKey);
  const onDrive = await request("/api/reading?" + new URLSearchParams({ source: item.source })).catch(() => null);
  if (selection !== item || !root.isConnected) return;
  const saved = (onDrive?.at || 0) >= (local?.at || 0) ? onDrive || local : local;
  const start = Math.max(0, paths.indexOf(saved?.chapter));
  if (saved && (start > 0 || saved.block > 0)) {
    resume.hidden = false;
    resume.append("Continuing at " + chapterTitle(start) + (Number.isFinite(saved.percent) ? ", " + saved.percent + "% through. " : ". "));
    resume.append(button("Start from the beginning", "text-button", () => { resume.hidden = true; open(0); }));
  }
  await open(start, { block: saved?.chapter === paths[start] ? saved.block : 0, initial: true });
}

export async function preferences(item, panel, key) {
  const settings = await request("/api/preferences");
  if (selection !== item) return;
  panel.replaceChildren();
  if (!key)
    panel.append(node("p", "item-recipe", "Open the private link printed by ./run web to edit settings."));
  const groups = new Map();
  for (const setting of settings) {
    if (!groups.has(setting.group)) {
      const section = node("section", "preference-group");
      section.append(node("h2", "", setting.group));
      groups.set(setting.group, section);
      panel.append(section);
    }
    const form = node("form", "preference");
    const label = node("label", "", setting.label);
    const id = "preference-" + setting.key;
    label.htmlFor = id;
    const hint = node("p", "quiet", setting.description);
    hint.id = id + "-description";
    const field = node(setting.choices?.length || setting.type === "bool" ? "select" : "input");
    field.id = id;
    field.setAttribute("aria-describedby", hint.id);
    if (field.tagName === "SELECT")
      for (const v of setting.choices || ["true", "false"]) field.add(new Option(v, v));
    else {
      field.type = { secret: "password", number: "number" }[setting.type] || "text";
      if (field.type === "number") {
        field.step = "any";
        if (setting.min != null) field.min = setting.min;
        if (setting.max != null) field.max = setting.max;
      }
    }
    field.value = String(setting.value ?? "");
    field.disabled = !key;
    const save = node("button", "button secondary", "Save");
    save.disabled = !key;
    const status = node("span", "item-recipe");
    status.setAttribute("role", "status");
    form.append(label, hint, field, save, status);
    form.onsubmit = async (event) => {
      event.preventDefault();
      save.disabled = true;
      field.disabled = true;
      try {
        const response = await fetch("/api/preferences", {
          method: "POST",
          headers: { "Content-Type": "application/json", "X-Svalbard-Request": "1", "X-Svalbard-Key": key },
          body: JSON.stringify({ key: setting.key, value: field.value })
        });
        if (!response.ok) throw Error((await response.text()).trim());
        if (field.type === "password") field.value = "";
        note(status, "Saved");
      } catch (error) {
        note(status, error.message, true);
      } finally {
        save.disabled = false;
        field.disabled = false;
      }
    };
    groups.get(setting.group).append(form);
  }
}

export async function dataset(item, panel, table = "") {
  const mount = node("div");
  panel.replaceChildren(mount);
  const data = await request("/api/dataset?" + new URLSearchParams({ id: item.id, table }));
  if (!mount.isConnected || selection !== item) return;
  if (data.tables && !table) {
    mount.append(node("h2", "", "Tables"));
    for (const t of data.tables)
      mount.append(
        button(t.name, "secondary", () =>
          dataset(item, panel, t.name).catch((e) => note(panel, e.message, true))
        )
      );
    if (data.note) mount.append(node("p", "quiet", data.note));
    return;
  }
  if (table)
    mount.append(
      button("← Tables", "quiet-button", () =>
        dataset(item, panel).catch((e) => note(panel, e.message, true))
      ),
      node("h2", "", table)
    );
  const fields = data.columns || data.tables?.[0]?.columns || [];
  const rows = data.rows || (data.tables?.[0]?.samples || []).map((row) => fields.map((c) => row[c]));
  const grid = node("table", "sample-grid");
  const header = grid.createTHead().insertRow();
  for (const c of fields) header.append(node("th", "", c));
  const body = grid.createTBody();
  for (const sample of rows) {
    const tr = body.insertRow();
    for (const value of sample) tr.insertCell().textContent = String(value ?? "");
  }
  mount.append(node("p", "item-recipe", rows.length + " sample rows"), grid);
}
