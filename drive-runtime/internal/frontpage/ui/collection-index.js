/* A drive keeps its own index preferences. Resource identity always comes from the inventory. */
let indexWidth = 232;
let expandedCollections = new Set();
function storeIndex() {
  try { localStorage.setItem("svalbard-index:" + inventory.drive, JSON.stringify({width:indexWidth, collapsed:indexCollapsed, expanded:[...expandedCollections]})); } catch {}
}
function resizeIndex(width) {
  indexWidth = Math.min(340, Math.max(220, Math.round(width)));
  document.body.style.setProperty("--margin-width", indexWidth + "px");
  const handle = byId("index-resize");
  handle.setAttribute("aria-valuenow", indexWidth);
  handle.setAttribute("aria-valuetext", indexWidth + " pixels");
}
function initIndex() {
  try {
    const saved = JSON.parse(localStorage.getItem("svalbard-index:" + inventory.drive));
    if (saved) {
      if (typeof saved.collapsed === "boolean") indexCollapsed = saved.collapsed;
      if (Array.isArray(saved.expanded)) expandedCollections = new Set(saved.expanded);
      if (Number.isFinite(saved.width)) resizeIndex(saved.width);
    }
  } catch {}
  const handle = byId("index-resize");
  handle.onkeydown = event => {
    const next = {ArrowLeft:indexWidth-12,ArrowRight:indexWidth+12,Home:232,End:320}[event.key];
    if (next) { event.preventDefault(); resizeIndex(next); storeIndex(); }
  };
  handle.onpointerdown = event => {
    handle.setPointerCapture(event.pointerId);
    handle.onpointermove = move => resizeIndex(move.clientX - document.querySelector(".spine").getBoundingClientRect().width);
    handle.onlostpointercapture = () => { handle.onpointermove = null; storeIndex(); };
  };
}
function displayTitle(item) {
  return item.label?.split(/\s+[—–]\s+/)[0] || item.label || item.id || "Untitled";
}
function indexResources(list, resources) {
  list.replaceChildren();
  if (list.hidden) return;
  for (const item of resources) {
    const line = node("li"), link = node("a","resource-link",displayTitle(item));
    link.href = "#" + item.route;
    if (selection?.route === item.route) link.setAttribute("aria-current","page");
    if (item.service) link.append(serviceMark(item.service));
    line.append(link); list.append(line);
  }
}
function renderIndex(available,items) {
  for (const system of [false,true]) {
    const dest = byId(system ? "system-collections" : "margin-collections");
    dest.replaceChildren();
    for (const collection of available.filter(c => ["Tools","Settings"].includes(c.name) === system)) {
      const children = items.filter(item => collection.types.includes(overviewType(item))).sort((a,b)=>nameOrder.compare(a.label,b.label));
      const line = node("li","index-collection");
      const link=node("a","collection-label",collection.name);
      link.href=collectionURL(collection); link.dataset.collection=collectionFilter(collection);
      const list=node("ul","index-resources");
      const toggle=node("button","index-expand");
      toggle.type="button";
      function refresh() {
        const expanded=expandedCollections.has(collection.name);
        list.hidden=!expanded;
        toggle.textContent=expanded ? "−" : "+";
        toggle.setAttribute("aria-expanded",String(expanded));
        toggle.setAttribute("aria-label",(expanded?"Collapse ":"Expand ")+collection.name);
        indexResources(list,children);
      }
      toggle.onclick=()=>{if(expandedCollections.has(collection.name)) expandedCollections.delete(collection.name); else { expandedCollections.add(collection.name); if(indexWidth<300)resizeIndex(320); } refresh();storeIndex();};
      refresh();
      line.append(link, toggle, list);
      dest.append(line);
    }
    if(system) byId("system-heading").hidden=!dest.childElementCount;
  }
}
function revealResource(item) {
  const collection=collections.find(c=>c.types.includes(overviewType(item)));
  if (collection && !expandedCollections.has(collection.name)) {
    expandedCollections.add(collection.name); storeIndex();
    renderCollections([...entries.values()]);
  }
  document.querySelectorAll(".resource-link").forEach(link=>{
    if(link.hash==="#"+item.route)link.setAttribute("aria-current","page");
    else link.removeAttribute("aria-current");
  });
}
function serviceMark(key) {
  const badge=node("small","service-mark");
  badge.dataset.service=key;
  updateServiceMark(badge,states.get(key));
  return badge;
}
