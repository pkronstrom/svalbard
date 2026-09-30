Builder scripts that produce artifacts from external data sources.

Naming: <name>-<output-type>.py
  pinkka-sienet-fi-zim.py          — Helsinki Pinkka mushroom course → ZIM
  foraging-habitats-fi-pmtiles.py  — LUKE MS-NFI → Finnish foraging habitat map layer

Each builder is a standalone Python script. They run inside the
svalbard-tools Docker container which provides libzim, ffmpeg, gdal,
tippecanoe, and other dependencies.
