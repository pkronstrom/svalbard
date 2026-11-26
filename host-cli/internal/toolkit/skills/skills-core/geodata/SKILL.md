---
name: geodata
description: Make a world or regional map from a GeoPackage on the drive (Natural Earth countries, places, rivers). Exports a layer to GeoJSON with a stdlib-only script and draws a choropleth with D3 and a bar chart, packed into one HTML file that opens offline.
requires: [js-core, py-docs]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned (keep the quotes
around it in commands: a drive's path can hold spaces). Work in the user's
working folder, never inside `<folder>`.

1. Find the data. `vault_databases` lists the GeoPackages (for example
   `natural_earth_vector.gpkg`) and their layers. `query_describe` with a
   layer as `table` shows its columns; `query_sql` shows values, for
   example `SELECT NAME, POP_EST FROM ne_110m_admin_0_countries ORDER BY
   POP_EST DESC LIMIT 5`. Countries: `ne_110m_admin_0_countries` (177,
   small); `ne_50m_admin_0_countries` has more detail.
2. Export the layer with the columns you need. It prints `wrote
   countries.geojson: 177 features`:

   ```sh
   py-docs "<folder>"/scripts/gpkg_to_geojson.py natural_earth_vector.gpkg ne_110m_admin_0_countries countries.geojson --columns NAME,POP_EST
   ```

3. Copy the template:

   ```sh
   cp "<folder>"/templates/choropleth.html map.html
   ```

4. Edit only the marked regions of `map.html`:
   - `EDIT: data`: `import geo from "./countries.geojson";` (your file).
     Keep the import: never fetch the file.
   - `EDIT: settings`: `value` (the number column that colours the map),
     `name`, `title`, `subtitle`, `label`, `scale` (`log` for population,
     `linear` for rates and percentages) and `top`.
5. Pack it into one file:

   ```sh
   svalbard-drive html pack map.html -o map-packed.html
   ```

6. Check your result. It must print `ok`:

   ```sh
   sh "<folder>"/examples/check.sh map-packed.html
   ```

7. Tell the user the packed file's full path. It opens by double-click,
   with the data inside.

## More

- `py-docs "<folder>"/scripts/gpkg_to_geojson.py natural_earth_vector.gpkg --list`
  lists every layer with its feature count and columns.
- `--where "CONTINENT = 'Europe'"` keeps some rows; `--precision 2` makes a
  smaller file. Shapes come out as Point, LineString, Polygon or Multi;
  anything else is reported as unsupported.
- Useful Natural Earth columns: `POP_EST`, `GDP_MD`, `CONTINENT`,
  `INCOME_GRP`, `ISO_A3`. Places: `ne_10m_populated_places` (points);
  rivers: `ne_110m_rivers_lake_centerlines` (lines).
- The D3 and Chart.js quickstarts (`skill_search` with `kind` library) show
  other projections and charts.
