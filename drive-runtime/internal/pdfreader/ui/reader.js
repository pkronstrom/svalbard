const get = id => document.getElementById(id);

async function openDocument() {
  const file = new URL(new URLSearchParams(location.search).get("file") || "/", location.origin);
  if (file.origin !== location.origin || !file.pathname.toLowerCase().endsWith(".pdf")) throw Error();
  const explicit = Number(new URLSearchParams(file.hash.slice(1)).get("page"));
  const position = "svalbard-page:"+file.pathname;
  let initialPage = explicit || 1;
  try { if(!explicit) initialPage=Number(sessionStorage.getItem(position)) || 1; } catch {}
  file.hash = "";
  const pdfjs = await import("./vendor/build/pdf.min.mjs");
  const { EventBus, PDFLinkService, PDFFindController, PDFViewer, LinkTarget } = await import("./vendor/web/pdf_viewer.mjs");
  pdfjs.GlobalWorkerOptions.workerSrc = "./vendor/build/pdf.worker.min.mjs";
  const events = new EventBus();
  const links = new PDFLinkService({eventBus: events, externalLinkTarget: LinkTarget.BLANK});
  const find = new PDFFindController({linkService:links, eventBus:events});
  const viewer = new PDFViewer({
    container: get("pages"), eventBus: events, linkService: links, findController:find,
    annotationEditorMode: pdfjs.AnnotationEditorType.DISABLE
  });
  links.setViewer(viewer);
  const doc = await pdfjs.getDocument({
    url: file.href, isEvalSupported: false, enableXfa: false,
    cMapUrl: new URL("vendor/cmaps/", location.href).href, cMapPacked: true,
    standardFontDataUrl: new URL("vendor/standard_fonts/", location.href).href,
    wasmUrl: new URL("vendor/wasm/", location.href).href
  }).promise;
  const page = get("page"), zoom = get("zoom");
  page.max = doc.numPages;
  get("total").textContent = "of " + doc.numPages;
  function currentPage() {
    page.value = viewer.currentPageNumber;
    try {sessionStorage.setItem(position,page.value);} catch {}
    get("previous").disabled = viewer.currentPageNumber <= 1;
    get("next").disabled = viewer.currentPageNumber >= doc.numPages;
  }
  get("previous").onclick = () => viewer.currentPageNumber--;
  get("next").onclick = () => viewer.currentPageNumber++;
  page.onchange = () => {
    viewer.currentPageNumber = Math.max(1, Math.min(doc.numPages, Math.trunc(Number(page.value)) || 1));
    currentPage();
  };
  zoom.onchange = () => { viewer.currentScaleValue = zoom.value; };
  function search(type, backwards = false) {
    events.dispatch("find", {source:viewer, type, query:get("query").value, phraseSearch:true, highlightAll:true, findPrevious:backwards});
  }
  get("find").onclick = () => {
    const form = get("pdf-find");
    form.hidden = !form.hidden;
    get("find").setAttribute("aria-expanded", String(!form.hidden));
    if (form.hidden) events.dispatch("findbarclose", {source:viewer});
    else get("query").focus();
  };
  get("query").oninput = () => search("");
  get("pdf-find").onsubmit = event => { event.preventDefault(); search("again"); };
  get("prev-match").onclick = () => search("again",true);
  const updateMatches = ({matchesCount}) => {
    if (matchesCount) get("matches").textContent = matchesCount.total ? matchesCount.current + " / " + matchesCount.total : "";
  };
  events.on("updatefindmatchescount", updateMatches);
  events.on("updatefindcontrolstate", event => {
    updateMatches(event);
    if (event.state === 1) get("matches").textContent = "No matches";
  });
  events.on("pagesinit", () => {
    viewer.currentScaleValue = zoom.value;
    viewer.currentPageNumber = Math.max(1, Math.min(doc.numPages, initialPage));
    currentPage();
    page.disabled = zoom.disabled = get("find").disabled = false;
    get("status").hidden = true;
  });
  events.on("pagechanging", currentPage);
  new ResizeObserver(() => {
    if (viewer.pagesCount && zoom.value.startsWith("page-")) viewer.currentScaleValue = zoom.value;
  }).observe(get("pages"));
  links.setDocument(doc);
  viewer.setDocument(doc);
}

openDocument().catch(() => {
  get("status").textContent = "This PDF could not be displayed here. Choose Open original to use your browser’s reader.";
});
