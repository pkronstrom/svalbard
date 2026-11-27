---
name: offline-map
description: Make an interactive map page over the drive's own offline maps (PMTiles basemaps) with MapLibre, with your places, routes or areas drawn on it. The map reads tiles from the drive, so it is packed into the drive's workspace folder and opens through the drive's maps server, not by double-click.
requires: [js-maps, type:pmtiles]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned (keep the quotes
around it in commands: a drive's path can hold spaces). Work in the user's
working folder, never inside `<folder>`.

1. Find the maps: `vault_maps` lists the drive's map files. A basemap is
   an OpenStreetMap one (for example `osm-uusimaa.pmtiles`, vector) or a
   topographic sheet (raster); the template draws either.
2. Copy the template:

   ```sh
   cp -n "<folder>"/templates/map.html map.html
   ```

   `-n` keeps a file of that name already there: then pick another name.

3. Edit only the marked regions of `map.html`:
   - `EDIT: settings`: the title, `basemap` (a file name from step 1),
     `center` as `[longitude, latitude]` and `zoom` (5 country, 11 city,
     15 street).
   - `EDIT: places`: your points as GeoJSON, each with a `name`. For many
     places put them in `places.geojson` beside the page and import it, as
     the comment shows; never fetch it. Lines and areas work too (see the
     MapLibre quickstart).
4. Pack it as a folder on the drive, in its `workspace` folder. `<drive>`
   is the start of `<folder>` before `/skills/` (keep the quotes):

   ```sh
   svalbard-drive html pack map.html --dir -o "<drive>/workspace/my-map"
   ```

   It prints `wrote …/workspace/my-map (…, directory)`.
5. Check your result. It must print `ok`:

   ```sh
   sh "<folder>"/examples/check.sh "<drive>/workspace/my-map"
   ```

6. Tell the user how to open it: in the drive's menu (`<drive>/run`, which
   is the user's to run, not yours) open any map under Maps; that starts the
   maps server and prints an address such as `http://localhost:8081`. Open
   that address followed by `/workspace/my-map/` in the browser. A
   double-click on the file cannot read the map tiles.

## Notes

- Only the drive's own maps work offline; never add a tile URL or style
  from the internet.
- A map without tiles (just GeoJSON over a plain background) works by
  double-click: see the MapLibre quickstart (`skill_search` "maplibre"
  with `kind` library) and pack it without `--dir`.
- Distances, areas and buffers: the Turf quickstart. Coordinates in
  another projection (EPSG:3067): the proj4 quickstart.
