/* PDF reading positions. The document frame reports its page; the drive keeps
   it, because the dashboard and document origins change with every launch. */
const pdfResume = new Map();
let pdfSave;
function loadPDFPosition(item) {
  if (item.format !== "PDF" || !item.source || pdfResume.has(item.source)) return;
  pdfResume.set(item.source, 0);
  request("/api/reading?" + new URLSearchParams({source:item.source})).then(saved => {
    const page = saved?.chapter === "page" ? saved.block : 0;
    if (page > 1) pdfResume.set(item.source, page);
    if (page > 1 && selection === item) resumePDF(page);
  }).catch(() => {});
}
// A frame that opened before the position arrived moves to it without adding a Back step.
function resumePDF(page) {
  const frame = byId("reader-frame");
  if (!frame?.src.includes("/.svalbard-reader/") || frame.dataset.resumed) return;
  const address = new URL(frame.src), file = address.searchParams.get("file") || "";
  if (file.includes("#")) return;
  address.searchParams.set("file", file + "#page=" + page);
  frame.dataset.resumed = "1";
  frame.contentWindow?.location.replace(address.href);
}
function pdfStartPage(item) {
  return item?.source ? pdfResume.get(item.source) || 0 : 0;
}
window.addEventListener("message", event => {
  const frame = byId("reader-frame"), report = event.data?.svalbardReader;
  if (!frame || event.source !== frame.contentWindow || !report || articleOpen || !selection?.source?.startsWith("books/")) return;
  const page = Math.trunc(report.page), pages = Math.trunc(report.pages);
  if (!(page >= 1 && pages >= page)) return;
  const source = selection.source;
  pdfResume.set(source, page);
  frame.dataset.resumed = "1";
  clearTimeout(pdfSave);
  pdfSave = setTimeout(() => {
    request("/api/reading?" + new URLSearchParams({source}), {chapter:"page", block:page, percent:Math.round(page / pages * 100)}).catch(() => {});
  }, 1500);
});
