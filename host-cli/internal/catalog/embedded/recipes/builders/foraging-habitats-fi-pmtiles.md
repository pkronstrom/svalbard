# Foraging Habitat Layer Builder

Builds a map layer that answers: **"What can I eat in this forest?"**

## Why

Finnish forests are not uniform — a dry pine forest grows lingonberries,
a mesic spruce forest grows bilberries and chanterelles, a herb-rich
forest grows nettles and ground elder, and a bog grows cloudberries.
Experienced foragers read the forest type instinctively. This layer
makes that knowledge visible on a map for anyone.

## How it works

Finland's national forest inventory (LUKE MS-NFI) maps every 16m cell
of Finnish forest with site fertility class, soil type, and tree species
volumes. This script downloads those rasters, resamples them to a
practical grid size, and reclassifies each cell into one of 7 foraging
habitat types based on well-established ecological associations:

| Forest type | What grows there |
|-------------|-----------------|
| Herb-rich (lehto) | Nettle, ground elder, raspberry, wood sorrel |
| Mixed berry (OMT) | Bilberry, raspberry, chanterelle |
| Bilberry forest (MT) | Bilberry, cep, trumpet chanterelle |
| Lingonberry forest (VT) | Lingonberry, slippery jack |
| Dry/barren (CT) | Lingonberry, crowberry, heather |
| Bog forest | Cloudberry, cranberry, bog bilberry |
| Open bog | Cloudberry, cranberry |

Mushroom associations are refined using dominant tree species (pine,
spruce, birch) since most edible mushrooms form mycorrhizal relationships
with specific trees.

The output is a vector PMTiles file that slots into the svalbard map
viewer as a toggleable overlay. Tap any area to see the habitat type,
expected edible species, season, and safety warnings.

## Data sources

All CC BY 4.0, from LUKE (Natural Resources Institute Finland):

- `kasvupaikka` — site fertility class (determines berry habitat)
- `paatyyppi` — mineral soil vs peatland (determines bog species)
- `maaluokka` — land class (filters to forest land only)
- `manty/kuusi/koivu` — pine/spruce/birch volume (determines mushroom associations)

## Disclaimer

Species associations are indicative estimates based on forest type, not
field observations. Poisonous species occur in the same habitats. This
layer does not replace a field identification guide. Always identify
before consuming.
