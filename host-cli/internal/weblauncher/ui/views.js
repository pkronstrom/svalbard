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

export async function ebook(item, panel) {
  const base = "/api/ebook?" + new URLSearchParams({ source: item.source });
  const result = await request(base);
  if (selection !== item) return;
  if (!result.chapters?.length) throw Error("This edition has no readable chapters. Use the original file.");
  const paths = result.chapters.map((part) => part.path);
  const picker = node("select");
  picker.setAttribute("aria-label", "Chapter");
  paths.forEach((path, i) => picker.add(new Option(i + 1 + " · " + path.split("/").pop(), String(i))));
  const article = node("article", "prose");
  const toolbar = node("div", "source-tabs");
  const previous = button("← Previous", "quiet-button", () => load(picker.selectedIndex - 1));
  const next = button("Next →", "quiet-button", () => load(picker.selectedIndex + 1));
  toolbar.append(previous, picker, next);
  panel.replaceChildren(toolbar, article);
  const key = "epub:" + item.source;
  let revision = 0;
  async function load(index, anchor = "") {
    const current = ++revision;
    picker.selectedIndex = index;
    previous.disabled = index === 0;
    next.disabled = index === paths.length - 1;
    article.textContent = "Opening chapter…";
    try {
      const data = await request(base + "&file=" + encodeURIComponent(paths[index]));
      if (current !== revision || !article.isConnected) return;
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
        if (image) element.src = base + "&file=" + encodeURIComponent(file);
        else if (paths.includes(file)) {
          element.href = "#" + item.route;
          element.onclick = (event) => {
            event.preventDefault();
            load(paths.indexOf(file), decodeURIComponent(address.hash.slice(1)));
          };
        }
      }
      article.replaceChildren(fragment);
      if (anchor) article.querySelector("#" + CSS.escape(anchor))?.scrollIntoView({ block: "start" });
      try {
        sessionStorage.setItem(key, String(index));
      } catch {}
    } catch (error) {
      if (current === revision) note(article, error.message, true);
    }
  }
  picker.onchange = () => load(picker.selectedIndex);
  let index = 0;
  try {
    index = Number(sessionStorage.getItem(key));
  } catch {}
  await load(Number.isInteger(index) && index >= 0 && index < paths.length ? index : 0);
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
