# Foraging Identification Guides — Research Notes

Companion to the foraging habitat layer. The habitat map answers "where to look",
these guides answer "what am I looking at".

## Already Available

- **Pinkka ZIM** — compiled locally, 120 mushroom + 250 plant species ID courses
  from University of Helsinki. Open Access.
- **Wikipedia ZIMs** — already in presets, has articles on every major edible species
  with photos. Can link from foraging layer popups.

## Best Candidates for New ZIMs

### Arktiset Aromit (arktisetaromit.fi)

Finnish wild food guide — mushrooms, berries, herbs. Identification info,
nutritional values, harvesting guidance, poisonous species warnings.
Government-funded (Ministry of Agriculture and Forestry).

- License: not explicitly stated, government-funded — may be permissive
- Action: contact them about offline ZIM for educational/survival use
- Scope: exactly what we need for Finnish foraging
- Could potentially build locally with zimit

### PFAF — Plants For A Future (pfaf.org)

8,000+ temperate plant species. Edibility ratings, medicinal uses, habitat,
cultivation. European focus.

- License: CC BY 4.0 (text), CC BY-NC-ND 3.0 (images)
- Action: contact PFAF before large-scale scraping, then zimit
- Scope: broader than needed but excellent edibility database
- Zimit scrape is technically straightforward

### Mushroom Observer (mushroomobserver.org)

500K+ observations, 1.6M photos, 20K species. Community-driven.

- License: CC BY-SA (user content)
- Action: use nightly CSV dumps + image downloads, build custom ZIM
- Scope: comprehensive but global — would need Finnish/Nordic filtering
- Prefers bulk data download over scraping

### FinBIF / laji.fi

Finnish species data with images via open API. All Finnish biodiversity.

- License: CC BY 4.0
- Action: build custom species guide ZIM from API data
- Scope: all Finnish species, would need curation for foraging focus
- R package (finbif) and REST API available

### Pinkka — Expand Coverage

Already have the base ZIM. University of Helsinki may have additional
courses or materials beyond the 120 mushroom + 250 plant set.

- License: Open Access
- Action: check for additional course materials

## Not Viable

| Source | Reason |
|--------|--------|
| Luontoportti / NatureGate | All rights reserved, proprietary |
| First Nature | Individual photo copyrights, proprietary |
| Rogers Mushrooms | Commercial product |
| Wild Food UK | Commercial operation |

## Alternative Approaches

- **PlantNet offline mode** — downloadable ML model for plant ID from photos.
  BSD/MIT code, CC BY-SA observations. Finnish region pack may exist.
- **FungID** — open source ML mushroom ID (AGPL-3.0). Offline classification
  could complement reference ZIMs.
- **VicFlora-style PWA** — build an offline-first web app from FinBIF data
  rather than a ZIM. Service workers for offline, same result.

## Priority

1. Link Wikipedia articles from foraging layer popups (free, already available)
2. Include Pinkka ZIM in presets as tunnistusopas
3. Contact Arktiset Aromit about permissions
4. Build PFAF ZIM if Arktiset Aromit doesn't work out
