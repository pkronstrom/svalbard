// duckdb-browser.mjs imports the bare specifier "apache-arrow". index.html maps
// that specifier here with an import map, and this module re-exports the
// self-contained Arrow UMD bundle that index.html loads first, so the page
// needs no bundler.
const Arrow = globalThis.Arrow;
if (!Arrow) {
  throw new Error("apache-arrow did not load (arrow/Arrow.esnext.min.js)");
}

export const {
  DataType,
  Field,
  RecordBatch,
  RecordBatchReader,
  Schema,
  Table,
  Type,
  Vector,
  tableFromIPC,
  tableToIPC,
} = Arrow;
export default Arrow;
