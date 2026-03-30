# Mobile Companion Apps for Svalbard Drives

Recommended iOS and Android apps for viewing Svalbard drive contents directly from a USB-C stick plugged into a phone. Prioritizes open source / free, notes paid options where they're clearly better.

## Status

**Reference** — curated app list for documentation and user guides.

---

## Platform Notes

**iOS (iPhone 15+ / iPad with USB-C):** Since iOS 13, the Files app can browse exFAT/FAT32 USB drives. Third-party apps using the document picker can open files from USB. Large files (multi-GB models, map archives) typically need to be copied to internal storage first due to iOS sandbox restrictions.

**Android 11+:** Scoped storage makes direct USB access app-dependent. Most apps use Storage Access Framework. Termux offers the most flexible file access via `termux-setup-storage`. For other apps, copying to internal/SD storage may be needed.

---

## Critical Format Gap: PMTiles

Svalbard uses PMTiles as its primary map tile format. **No native mobile app on either platform can open local PMTiles files for vector rendering.** This is the biggest mobile compatibility issue.

**Workarounds:**
1. Convert PMTiles → MBTiles on desktop before deploying (`pmtiles convert`)
2. Bundle both formats in presets (costs ~2x storage for map tiles)
3. Run a local HTTP server on the phone (Termux + a simple server) and point MapLibre at it

**Recommendation for Svalbard:** Consider generating MBTiles alongside PMTiles for map recipes, or add a `svalbard export --mobile` command that converts map data to MBTiles. This is the single biggest thing blocking phone-as-viewer workflows.

---

## Recommended Apps by Category

### ZIM Files (Wikipedia, WikiMed, iFixit, Stack Exchange, etc.)

| App | Platform | FOSS | Price | Notes |
|-----|----------|------|-------|-------|
| **Kiwix** | iOS + Android | Yes | Free | The only option. Works well. |

### Offline Maps & Navigation

| App | Platform | FOSS | Price | Formats | Notes |
|-----|----------|------|-------|---------|-------|
| **Organic Maps** | iOS + Android | Yes (Apache 2.0) | Free | Own OSM format | Simplest offline nav. No custom tile import. Downloads its own maps. Best "just works" option. |
| **OsmAnd+** | iOS + Android | Yes (GPLv3) | Free on F-Droid / ~$10 Play Store / Free iOS (5 maps) | MBTiles, GeoJSON, GPX, KML, own OBF | Swiss army knife. Can load MBTiles as overlay/underlay. Complex UI but very capable. |
| **Guru Maps** | iOS | No | Free + IAP for MBTiles | MBTiles (raster) | Clean UI. MBTiles import is a paid feature. |
| **AlpineQuest** | Android | No | Free lite / ~$8 full | MBTiles, sqlitedb, raster tiles | Popular with hikers. Excellent raster tile stacking. |
| **Trekarta** | Android | Yes | Free | MBTiles (raster), sqlitedb | Lightweight hiking-focused map viewer. |

**Primary picks:** Organic Maps (simple nav) + OsmAnd (MBTiles/custom data).

### GeoPackage & GIS Data (.gpkg, GeoJSON, Shapefile)

| App | Platform | FOSS | Price | Formats | Notes |
|-----|----------|------|-------|---------|-------|
| **QField** | iOS + Android | Yes (GPL) | Free | GeoPackage, GeoJSON, Shapefile, GPX, MBTiles, GeoTIFF, WMS cache | Mobile QGIS. Best with pre-prepared .qgs project files. Can also open .gpkg directly. |
| **MapCache by NGA** | iOS | Yes | Free | GeoPackage (features + tiles) | Basic but functional GeoPackage viewer/editor. |
| **Mergin Maps** | iOS + Android | Yes (app) | Free app, paid cloud sync | GeoPackage, GeoJSON, Shapefile | QGIS-based like QField. Pushes cloud sync but works locally. |
| **SW Maps** | Android | No | Free | GeoPackage, Shapefile, GeoJSON, KML, MBTiles | Surprisingly capable free GIS data collector. |
| **Geodata Map Viewer** | iOS | No | Free | GeoJSON, KML, GPX | Simple free viewer for vector overlays. |
| **Mapit GIS** | iOS + Android | No | Free (limited) / subscription | GeoPackage, MBTiles, GeoJSON, Shapefile, KML, GPX, CSV, DXF | Multi-format but free tier limited to 1 project, 3 layers. |

**Primary pick:** QField (both platforms, FOSS, handles everything).

### Local LLM (GGUF Models)

| App | Platform | FOSS | Price | Notes |
|-----|----------|------|-------|-------|
| **PocketPal AI** | iOS + Android | Yes (MIT) | Free | Clean UI, built on llama.cpp. Can import local GGUF files. Best cross-platform FOSS option. |
| **ChatterUI** | Android | Yes | Free | Nice chat UI, active development. GGUF via llama.cpp backend. |
| **Off Grid Mobile AI** | iOS | Yes (MIT) | Free | GGUF + Stable Diffusion + Whisper. 15-30 tok/s on A17 Pro. Needs iPhone 12+ / 4GB RAM min. |
| **Enclave AI** | iOS | No | Free (local) / $10/mo (cloud) | Good UX. Browse + download GGUF from Hugging Face in-app. |
| **Termux + llama.cpp** | Android | Yes | Free | CLI. Maximum control. Compile llama.cpp, run any GGUF. Best for power users. |

**Primary pick:** PocketPal AI (both platforms, FOSS, simple).

**Practical note:** Phones with 8-12 GB RAM can run Q4-quantized models up to ~7-8B parameters. The bundled Qwen 3.5 9B (Q4_K_M, 5.9 GB) is right at the edge — works on recent flagship phones (iPhone 15 Pro+, high-end Android). The 1B tier would be more comfortable for older devices.

### SQLite Databases

| App | Platform | FOSS | Price | Notes |
|-----|----------|------|-------|-------|
| **SQLiteFlow** | iOS | No | Free trial / paid | Best iOS SQLite editor. Multi-query, syntax highlighting, export. Files app integration. |
| **Termux + sqlite3** | Android | Yes | Free | `pkg install sqlite` then full SQL from CLI. Most flexible. |

### PDF (WHO BEC manual, etc.)

Built-in readers on both platforms handle this — no special app needed. iOS Books/Files and Android's default PDF viewer work fine.

---

## Recommended Install Sets

### Minimal (covers core Svalbard content)

**iOS:**
1. Kiwix — ZIM library
2. Organic Maps — offline navigation
3. PocketPal AI — local LLM

**Android:**
1. Kiwix — ZIM library
2. Organic Maps — offline navigation
3. PocketPal AI — local LLM

### Full (all Svalbard formats)

**iOS:**
1. Kiwix — ZIM library
2. Organic Maps — simple offline nav
3. OsmAnd — MBTiles maps, GeoJSON overlays
4. QField — GeoPackage geodata
5. PocketPal AI — local LLM
6. Geodata Map Viewer — quick GeoJSON viewing
7. SQLiteFlow — SQLite databases (Fimea, Joukahainen)

**Android:**
1. Kiwix — ZIM library
2. Organic Maps — simple offline nav
3. OsmAnd+ (from F-Droid) — MBTiles maps, GeoJSON, custom tiles
4. QField — GeoPackage geodata
5. PocketPal AI — local LLM
6. Termux — swiss army knife (sqlite3, llama.cpp, file management)

---

## Format Compatibility Matrix

| Svalbard Format | What Uses It | iOS App | Android App |
|-----------------|-------------|---------|-------------|
| ZIM | Wikipedia, WikiMed, iFixit, SE, LibreTexts, etc. | Kiwix | Kiwix |
| PMTiles | Map tiles (basemap, LIPAS, etc.) | **No native app** | **No native app** |
| MBTiles | Map tiles (if converted) | OsmAnd, Guru Maps | OsmAnd, AlpineQuest, Trekarta |
| GeoPackage | Finnish geodata layers | QField, MapCache | QField, SW Maps |
| GeoJSON | Vector overlays | Geodata Map Viewer, OsmAnd | OsmAnd, QField |
| GGUF | Local LLM models | PocketPal AI | PocketPal AI, ChatterUI |
| SQLite | Fimea, Joukahainen | SQLiteFlow | Termux + sqlite3 |
| PDF | WHO BEC manual, FAO | Built-in | Built-in |

---

## App Download Sources

### iOS (App Store)
- Kiwix: https://apps.apple.com/us/app/kiwix/id997079563
- Organic Maps: https://apps.apple.com/us/app/organic-maps-offline-map/id1567437057
- OsmAnd: https://apps.apple.com/us/app/osmand-maps-travel-navigate/id934850257
- QField: https://apps.apple.com/us/app/qfield-for-qgis/id1531726814
- PocketPal AI: https://apps.apple.com/us/app/pocketpal-ai/id6502579498
- Geodata Map Viewer: https://apps.apple.com/us/app/geodata-map-viewer/id6444589175
- SQLiteFlow: https://apps.apple.com/us/app/sqliteflow-sqlite-editor/id1406266008
- MapCache by NGA: https://apps.apple.com/us/app/mapcache-by-nga/id1477252454

### Android (F-Droid preferred, then Play Store)
- Kiwix: https://f-droid.org/en/packages/org.kiwix.kiwixmobile/
- Organic Maps: https://f-droid.org/en/packages/app.organicmaps/
- OsmAnd+: https://f-droid.org/en/packages/net.osmand.plus/
- QField: https://play.google.com/store/apps/details?id=ch.opengis.qfield
- PocketPal AI: https://play.google.com/store/apps/details?id=com.pocketpalai
- ChatterUI: https://play.google.com/store/apps/details?id=com.Vali98.ChatterUI
- Termux: https://f-droid.org/en/packages/com.termux/
