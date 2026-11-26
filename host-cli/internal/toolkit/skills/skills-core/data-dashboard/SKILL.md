---
name: data-dashboard
description: Make a dashboard page from a table of data (CSV, JSON, or rows from a drive database) with charts and a sortable table, packed into one HTML file that opens by double-click offline. Includes a "load another file" button.
requires: [js-core]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned (keep the quotes
around it in commands: a drive's path can hold spaces). Work in the user's
working folder, never inside `<folder>`.

1. Put the data in the working folder as `data.csv`, header row first
   (`month,sales,costs`, then one row per line). From a drive database:
   query it with `query_sql` and write the rows you got as CSV. A JSON list
   of objects works too: save it as `data.json`.
2. Copy the template:

   ```sh
   cp "<folder>"/templates/dashboard.html dashboard.html
   ```

3. Edit only the two marked regions of `dashboard.html`:
   - `EDIT: data`: the file to import, `import data from "./data.csv";`
     (or `"./data.json"`). Keep the import: never fetch the file.
   - `EDIT: settings`: the title, `label` (the column for the x axis),
     `values` (number columns to chart) and the chart types (`bar`, `line`,
     `pie`, `doughnut`).
4. Pack it into one file:

   ```sh
   svalbard-drive html pack dashboard.html -o dashboard-packed.html
   ```

   It prints `wrote dashboard-packed.html (…, single-file)` or the problems
   to fix (a CDN link, a fetch, a missing file).
5. Check your result. It must print `ok`:

   ```sh
   sh "<folder>"/examples/check.sh dashboard-packed.html
   ```

6. Tell the user the packed file's full path. It opens by double-click on
   any computer; "Load another file" reads another CSV or JSON file.

## Changing more

- Chart.js options (stacked bars, axis titles, colours): see the Chart.js
  quickstart, `skill_search` "chart.js" with `kind` library.
- More charts: add a `<canvas>` and another `make("id", "bar")` call.
- Never add a script tag that points at the internet: every library is on
  the drive and `svalbard-drive html pack` puts it in the page.
