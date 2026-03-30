# Deferred Sources and Tools

Sources and tools identified in a ChatGPT research audit (2026-03-30) of offline survival data bundles that were **not** added as recipes. Organized by reason for deferral and potential value.

## Status

**Parked** — these are reference items for future evaluation, not active work.

---

## Offline Routing (major feature gap)

The biggest capability gap in current presets: we have maps but no route planning.

### Valhalla
- **What**: Offline turn-by-turn routing engine with isochrone support
- **License**: MIT
- **Why deferred**: Not a simple download recipe — requires preprocessing OSM extracts into routing tile graphs. Needs its own design doc covering: which OSM extracts to preprocess, tile generation pipeline, how to serve routes (HTTP API? CLI?), integration with the map viewer.
- **Value**: Very high. Combines with LIPAS trail data and OSM road network for actual navigation.
- **URL**: https://github.com/valhalla/valhalla

### OSRM
- **What**: Fast offline routing engine
- **License**: BSD 2-Clause
- **Why deferred**: Same preprocessing pipeline issue as Valhalla. Narrower feature set (no isochrones) but faster.
- **URL**: https://project-osrm.org/

### GraphHopper
- **What**: Offline routing engine (Java)
- **License**: Apache 2.0
- **Why deferred**: Java runtime adds significant footprint. Same preprocessing issue.
- **URL**: https://github.com/graphhopper/graphhopper

**Recommendation**: Design an offline routing feature around Valhalla (MIT, C++, good isochrone support). Start with Finland OSM extract for finland-* presets.

---

## Datasets — auth or packaging barriers

### NASA SRTM Elevation (GL1 v003)
- **What**: Global 30m resolution elevation data
- **Size**: ~100 GB global, ~2–5 GB for Finland/Nordics
- **License**: Public domain (NASA)
- **Why deferred**: Requires NASA Earthdata login to download. Not automatable without credentials.
- **Value**: High for terrain-aware routing, slope analysis, flood modeling. Pairs well with map viewer and a future routing engine.
- **Catalog**: https://www.earthdata.nasa.gov/data/catalog/lpcloud-srtmgl1-003
- **Alternative**: MML Korkeusmalli 10m for Finland-specific elevation (already noted in finnish-open-data.md)

### Copernicus DEM (GLO-30 / GLO-90)
- **What**: Global 30m/90m digital elevation model from ESA
- **Why deferred**: Large dataset, distribution requires Copernicus access. Similar auth issue.
- **Value**: Alternative to SRTM with potentially newer data.

### OpenStax Textbooks
- **What**: Openly licensed (CC BY 4.0) college-level textbooks
- **Why deferred**: Not available as Kiwix ZIM (download.kiwix.org/zim/openstax/ returns 404). Would need custom PDF bundling or waiting for Kiwix to package them.
- **Value**: Medium. LibreTexts ZIMs now cover similar ground across disciplines.
- **URL**: https://openstax.org/

### FAO Seed Production Manuals
- **What**: Agricultural knowledge for food production and seed saving
- **License**: CC BY-NC-SA 3.0 IGO
- **Why deferred**: No single clean download URL found. FAO publications are spread across multiple pages with JS-heavy download flows.
- **Value**: Medium-high for long-term rebuilding scenarios. Practical Action partially covers this.
- **Potential source**: https://www.fao.org/publications/

---

## Tools — platform complexity or scope mismatch

### par2cmdline
- **What**: PAR2 parity/recovery tool — detect and repair data corruption
- **Why deferred**: No clean pre-built static binaries. Install via `brew install par2` (macOS) or `apt install par2` (Linux).
- **Value**: High for bundle integrity. **Better approach**: add a `svalbard audit` command that generates PAR2 recovery volumes for the drive contents, rather than bundling the binary.
- **URL**: https://github.com/Parchive/par2cmdline

### Syncthing
- **What**: Continuous file sync between devices
- **Why deferred**: Version-pinned release URLs are messy across platforms. Better recommended as a user-installed companion.
- **Value**: Medium. Useful for replicating bundles across multiple drives/devices.
- **URL**: https://syncthing.net/

### VeraCrypt
- **What**: Full-disk or container encryption with plausible deniability (hidden volumes)
- **Why deferred**: Requires admin/root privileges, heavy application, complex cross-platform support. Also has licensing tension with CC/ODbL "no additional restrictions" clauses if redistributing encrypted content.
- **Value**: Medium for personal use. Not something to bundle — document as a companion tool.
- **URL**: https://veracrypt.eu/

### ddrescue / TestDisk / PhotoRec
- **What**: Data recovery tools (failing drives, partition recovery, file carving)
- **Why deferred**: System-level recovery tools, not knowledge content. Outside Svalbard's scope.
- **Value**: High in a system rescue context, but that's a different tool.
- **URLs**: https://www.gnu.org/software/ddrescue/ / https://www.cgsecurity.org/wiki/TestDisk

### QGIS / QMapShack
- **What**: Desktop GIS analysis (QGIS) and offline map viewer/route planner (QMapShack)
- **Why deferred**: Too large to bundle. Users with GIS needs can install these separately; the bundled MapLibre web viewer covers basic map browsing.
- **URLs**: https://qgis.org/ / https://github.com/Maproom/qmapshack

---

## Communication Tools — niche but worth tracking

These serve off-grid comms scenarios. Not core to Svalbard's knowledge archive mission, but relevant for survival presets if scope expands.

### Meshtastic
- **What**: Open-source LoRa mesh networking — off-grid text messaging via cheap radio hardware
- **Why deferred**: Firmware images are hardware-specific; docs alone are small but limited value without the radios.
- **Value**: High if users have Meshtastic hardware. Consider bundling docs + firmware as a recipe in future.
- **URL**: https://meshtastic.org/

### Jami
- **What**: P2P encrypted communications (voice, video, messaging) — works on LAN without internet
- **Why deferred**: Full application, platform-specific builds, frequent updates.
- **URL**: https://jami.net/

### NomadNet
- **What**: Low-bandwidth/off-grid messaging over Reticulum network stack
- **Why deferred**: Python application with dependencies. Niche but interesting for radio-linked mesh messaging.
- **URL**: https://github.com/markqvist/nomadnet

### Ham Radio Software (CHIRP, Dire Wolf, fldigi, JS8Call)
- **What**: Radio programming (CHIRP), packet/APRS (Dire Wolf), digital modes (fldigi, JS8Call)
- **Why deferred**: Very niche. Platform-dependent. The amateur-radio Stack Exchange ZIM already covers the knowledge side. These only matter if users have ham radio hardware and licenses.
- **URLs**: https://chirpmyradio.com/ / https://github.com/wb2osz/direwolf / https://www.w1hkj.org/ / https://js8call.com/

---

## Future Feature Ideas (from the research)

### Bundle Integrity System
Generate SHA-256 manifests + PAR2 recovery volumes for the drive. Could be a `svalbard verify` / `svalbard repair` command pair. More valuable than bundling par2cmdline as a binary.

### Bootable Companion Guide
Document how to set up Ventoy + SystemRescue alongside a Svalbard data partition, rather than building bootable support into the tool. A `docs/guides/bootable-companion.md` would suffice.

### Encrypted Container Support
Optional VeraCrypt container generation for sensitive personal files (credentials, personal docs). Keeps CC/ODbL content unencrypted for license compliance while protecting private data.
