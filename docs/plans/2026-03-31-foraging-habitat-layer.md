# Foraging Habitat Layer — Design

## Goal

A standalone build tool that produces a PMTiles vector layer mapping Finnish
forest types to edible/medicinal species. The output fits into the existing
svalbard map viewer with zero viewer changes. Contributors can regenerate
from source when LUKE publishes new data.

## Source Data

All from LUKE MS-NFI 2023, CC BY 4.0, EPSG:3067 (ETRS-TM35FIN), 16m resolution.

| Raster | File | Use |
|--------|------|-----|
| Site fertility (kasvupaikka) | `kasvupaikka_vmi1x_1923.tif` (432 MB) | Berry habitat class |
| Site main class (paatyyppi) | `paatyyppi_vmi1x_1923.tif` (255 MB) | Mineral vs peatland |
| Land class (maaluokka) | `maaluokka_vmi1x_1923.tif` (197 MB) | Filter non-forest |
| Pine volume (manty) | `manty_vmi1x_1923.tif` (~1 GB) | Pine-associated mushrooms |
| Spruce volume (kuusi) | `kuusi_vmi1x_1923.tif` (~1 GB) | Spruce-associated mushrooms |
| Birch volume (koivu) | `koivu_vmi1x_1923.tif` (~1 GB) | Birch-associated mushrooms |

Base URL: `https://www.nic.funet.fi/index/geodata/luke/vmi/2023/`

Total download: ~4 GB. Resampled to 500m immediately.

## Reclassification Logic

### Step 1: Filter to forest land

`maaluokka == 1` (forest land). Drop water, urban, agricultural, etc.

### Step 2: Determine foraging habitat class

From `kasvupaikka` x `paatyyppi`:

| kasvupaikka | paatyyppi=1 (mineral) | paatyyppi=2,3 (mire) | paatyyppi=4 (open bog) |
|-------------|----------------------|----------------------|----------------------|
| 1 (lehto) | `herb_rich` | `bog_forest` | `open_bog` |
| 2 (OMT) | `mixed_berry` | `bog_forest` | `open_bog` |
| 3 (MT) | `bilberry_forest` | `bog_forest` | `open_bog` |
| 4 (VT) | `lingonberry_forest` | `bog_forest` | `open_bog` |
| 5-6 (CT/ClT) | `dry_barren` | `bog_forest` | `open_bog` |

### Step 3: Determine dominant tree species

From max(manty, kuusi, koivu) per cell:

- `pine` — manty is highest volume
- `spruce` — kuusi is highest volume
- `birch` — koivu is highest volume
- `mixed` — no clear dominant (within 20% of each other)

This determines mushroom associations for mineral soil forest classes.

## Foraging Classes and Species

### herb_rich (Lehto — herb-rich forest)

**Berries:** Vadelma (raspberry) Jul-Aug
**Herbs:** Nokkonen (stinging nettle) May-Jul, Vuohenputki (ground elder) May-Jun,
Ketunleipä (wood sorrel) May-Sep, Mesiangervo (meadowsweet, tea) Jun-Aug
**Mushrooms:** Varies by tree species
**Caution:** Kielo (lily of the valley) — poisonous, resembles some edible greens

### mixed_berry (Lehtomainen kangas — OMT)

**Berries:** Mustikka (bilberry) Jul-Aug, Vadelma (raspberry) Jul-Aug
**Herbs:** Ketunleipä (wood sorrel) May-Sep
**Mushrooms:** Varies by tree species

### bilberry_forest (Tuore kangas — MT, mesic)

**Berries:** Mustikka (bilberry) Jul-Aug — peak habitat
**Mushrooms by dominant tree:**
- Spruce: Herkkutatti (cep/porcini) Aug-Oct, Suppilovahvero (trumpet chanterelle) Sep-Nov
- Birch: Kantarelli (chanterelle) Jul-Sep, Koivunpunikkitatti (birch bolete) Jul-Oct
- Pine: Kangastatti (pine bolete) Aug-Oct
- Any: Rouskut (milk caps) Aug-Oct

### lingonberry_forest (Kuivahko kangas — VT, sub-xeric)

**Berries:** Puolukka (lingonberry) Aug-Oct — peak habitat, Mustikka (bilberry) Jul-Aug
**Mushrooms by dominant tree:**
- Pine: Voitatti (slippery jack) Aug-Oct, Kangasrousku (rufous milkcap) Aug-Oct
- Spruce: Suppilovahvero (trumpet chanterelle) Sep-Nov

### dry_barren (Kuiva/karukkokangas — CT/ClT)

**Berries:** Puolukka (lingonberry) Aug-Oct
**Other:** Kanerva (heather, tea) Jun-Aug, Jäkälä (reindeer lichen, emergency food)
**Mushrooms:** Sparse — some Voitatti in pine stands

### bog_forest (Korpi/räme — forested peatland)

**Berries:** Lakka/hilla (cloudberry) Jul-Aug, Juolukka (bog bilberry) Aug-Sep,
Karpalo (cranberry) Sep-Oct
**Other:** Suopursu (marsh Labrador tea) — traditional tea, use sparingly
**Caution:** Suoputki (water hemlock) — extremely poisonous, grows in wet areas

### open_bog (Avosuo — open peatland)

**Berries:** Lakka (cloudberry) Jul-Aug, Karpalo (cranberry) Sep-Oct
**Other:** Rahkasammal (sphagnum moss) — wound dressing, water filtration
**Caution:** Suoputki (water hemlock) — extremely poisonous

## Polygon Properties

Each polygon carries flat string properties (MVT-compatible):

```
class       "bilberry_forest"
name        "Mustikkametsä / Bilberry forest (MT)"
tree        "spruce"
berries     "Mustikka (bilberry) Jul-Aug"
mushrooms   "Herkkutatti (cep) Aug-Oct, Suppilovahvero (trumpet ch.) Sep-Nov"
herbs       ""
caution     "Korvasieni (false morel) — toxic raw, requires extended parboiling"
disclaimer  "Suuntaa-antava / Indicative only — identify before consuming"
```

## Build Pipeline

```
tools/build-foraging-layer.py
```

Runs inside the existing `svalbard-tools` Docker container.

### Steps:

1. **Download** 6 LUKE rasters (~4 GB) to a temp/cache directory
2. **Resample** each to 500m with `gdal_translate -tr 500 500 -r mode` (categorical)
   and `gdal_translate -tr 500 500 -r average` (continuous volumes)
3. **Reclassify** with rasterio + numpy:
   - Read all 6 bands
   - Filter to maaluokka == 1
   - Compute foraging class from kasvupaikka x paatyyppi
   - Compute dominant tree from max(manty, kuusi, koivu)
   - Write combined classification raster
4. **Polygonize** with rasterio.features.shapes → GeoJSON polygons
5. **Dissolve** adjacent same-class polygons (group by class + tree)
6. **Simplify** geometry (tolerance ~200m for 500m source)
7. **Reproject** EPSG:3067 → EPSG:4326
8. **Attach species** properties from lookup table (class + tree → strings)
9. **Tippecanoe** → `foraging-habitats.pmtiles`
   - `-z12 -Z5` (zoom 5 to 12)
   - `--coalesce-densest-as-needed` to manage polygon count at low zoom

### Output:

- `foraging-habitats.pmtiles` (~20-40 MB estimated)
- Placed in `generated/` (gitignored) or uploaded for distribution

## Recipe

```yaml
id: foraging-habitats-fi
type: pmtiles
group: maps
tags: [food, foraging, survival]
depth: comprehensive
size_gb: 0.04
description: Foraging habitat map — edible plants, berries, and mushrooms by forest type
strategy: download
path: generated/foraging-habitats-fi.pmtiles   # local artifact
# url: <remote-url-tbd>                        # for distribution
builder: foraging-habitats-fi-pmtiles.py        # regenerate from source (requires Docker)
layer_name: foraging

viewer:
  name: Foraging Habitats
  name_fi: Keräilykartta
  category: food

license:
  id: CC-BY-4.0
  attribution: >
    Derived from LUKE MS-NFI 2023 (CC BY 4.0).
    Species associations are indicative — identify before consuming.
  url: https://www.luke.fi/en/statistics/multi-source-national-forest-inventory
```

## Builder-aware sync (future feature)

When a recipe has a `builder:` field, `sync_drive` should:

1. Check `url` → download pre-built artifact
2. Check `path` → copy local artifact
3. Neither exists → detect `builder` → prompt user:
   *"foraging-habitats-fi has no pre-built artifact. Build from source?
   (requires Docker/svalbard-tools, ~4 GB download, ~10 min)"*
4. User confirms → run builder inside svalbard-tools Docker → place on drive
5. User declines → skip

## Disclaimer (vastuuvapauslauseke)

Embedded in:
- Build script header
- PMTiles metadata (`tippecanoe --description`)
- Polygon `disclaimer` property (shown in popup footer)
- Recipe `license.attribution`

Finnish:
> Tämä aineisto on tuotettu LUKE:n avoimista metsävaratiedoista parhaalla
> mahdollisella tarkkuudella. Lajiyhdistelmät ovat suuntaa-antavia ja
> perustuvat kasvupaikkatyypin ja puuston perusteella tehtyyn arvioon.
> Myrkylliset lajit voivat esiintyä samoilla kasvupaikoilla.
> Aineisto ei korvaa lajintunnistusopasta. Käyttäjä vastaa itse
> keräämistään kasveista ja sienistä.

English:
> This dataset is derived from LUKE open forest resource data on a
> best-effort basis. Species associations are indicative estimates based
> on site fertility class and tree species composition. Poisonous species
> may occur in the same habitats. This dataset does not replace a field
> identification guide. User assumes all responsibility for species
> they collect and consume.

## Validation

Before finalizing the species lookup table:
1. Cross-reference against Nordic Wild Food Plant Inventory (Figshare, CC BY 4.0)
2. Verify mushroom-tree associations against Finnish Mycological Society data
3. Ensure all listed caution species are correct for the habitat
4. Have the table reviewed by someone with Finnish foraging experience

## File Layout

```
tools/
  build-foraging-layer.py    # standalone build script

recipes/
  maps/
    foraging-habitats.yaml   # recipe for the pre-built PMTiles

generated/                   # gitignored — build output lands here
```

## Attribution

LUKE MS-NFI: Natural Resources Institute Finland, Multi-source National
Forest Inventory 2023, CC BY 4.0.

Nordic Wild Food Plant Inventory: Svanberg et al., CC BY 4.0,
doi:10.6084/m9.figshare.27925893.v1
