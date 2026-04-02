# Pack: Sciences

Biology, chemistry, physics, mathematics, and earth sciences references and textbooks.

## Audience

Students, researchers, educators, and curious minds.

## Status

Idea -- collecting sources

---

## Sub-Topics

### Biology

Cell biology, genetics, ecology, evolution, microbiology, anatomy, botany, zoology. Field identification guides for plants, birds, insects, and fungi.

### Chemistry

General, organic, analytical, physical, and biochemistry. Periodic table references, reaction databases, safety data.

### Physics

Classical mechanics, thermodynamics, electromagnetism, optics, quantum mechanics, relativity. Practical physics for engineering and construction contexts.

### Mathematics

Algebra, calculus, linear algebra, differential equations, discrete math, statistics, probability. Both pure reference and applied/computational math.

### Earth Sciences

Geology, meteorology, oceanography, soil science, hydrology, climate science. Mineral and rock identification, weather pattern interpretation.

### Astronomy

Star charts, constellation guides, celestial navigation, solar system reference. Practical for navigation and timekeeping without electronics.

---

## Source Candidates

### Already in Svalbard Recipes

| Source | Recipe ID | Type | Size | Notes |
|--------|-----------|------|------|-------|
| LibreTexts Biology | `libretexts-biology` | ZIM | ~2.1 GB | Cell bio, ecology, genetics, microbiology |
| LibreTexts Chemistry | `libretexts-chemistry` | ZIM | ~2.0 GB | General, organic, analytical, physical chemistry |
| LibreTexts Physics | `libretexts-physics` | ZIM | ~0.53 GB | Mechanics, thermo, E&M, quantum |
| LibreTexts Math | `libretexts-math` | ZIM | ~0.8 GB | Algebra, calculus, linear algebra, discrete math |
| LibreTexts Geosciences | `libretexts-geosciences` | ZIM | ~1.1 GB | Geology, meteorology, oceanography, soil science |
| LibreTexts Statistics | `libretexts-statistics` | ZIM | ~0.2 GB | Probability and statistical methods |
| LibreTexts Medicine | `libretexts-medicine` | ZIM | ~1.1 GB | Anatomy, physiology, pharmacology (bio-adjacent) |
| Khan Academy | `khan-academy` | ZIM | ~10 GB | Video + articles across math, physics, chemistry, biology |
| Stack Exchange Biology | `stackexchange-biology` | ZIM | ~0.5 GB | Q&A on biology topics |
| Stack Exchange Chemistry | `stackexchange-chemistry` | ZIM | ~0.5 GB | Q&A on chemistry topics |
| Stack Exchange Physics | `stackexchange-physics` | ZIM | ~2.0 GB | Q&A on physics topics |
| Stack Exchange Math | `stackexchange-math` | ZIM | ~10 GB | Q&A on mathematics (very large) |
| Wikipedia EN | various | ZIM | varies | Already in presets; broad science coverage |

### Candidate Sources Requiring Research

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| OpenStax Textbooks | PDF bundle | ~2-4 GB | core | CC BY 4.0 college textbooks (bio, chem, physics, math, astronomy, geology). Not on Kiwix as ZIM yet; would need custom PDF packaging |
| PubChem Compound Reference (subset) | SQLite | ~1-5 GB | extended | Open chemical compound database. Full dump is enormous; a curated subset of common compounds with structures and safety data would be practical |
| NIST Chemistry WebBook (subset) | zimit scrape | ~200-500 MB | extended | Thermochemical, spectral, and ion data for chemical species. Public domain (US gov) |
| Periodic Table (interactive) | static HTML/JS | ~5-10 MB | core | Ptable.com or similar interactive periodic table app. Small, high-value reference |
| Stellarium Web (offline) | static app | ~50-100 MB | extended | Browser-based planetarium. Needs investigation on offline feasibility |
| HYG Star Database | CSV/SQLite | ~5 MB | core | ~120K stars with position, magnitude, spectral class. Public domain |
| Messier/NGC object catalog | CSV/SQLite | ~1 MB | core | Deep sky object reference for astronomy |
| Mineral identification guides | zimit scrape | ~100-300 MB | extended | mindat.org (check license) or USGS mineral resources. Rock and mineral field ID |
| USGS publications | PDF | ~100-500 MB | extended | Geology, hydrology, and earth science references. Public domain |
| Audubon/Merlin bird guides | -- | -- | -- | Copyrighted; need open alternative. Xeno-canto (bird sounds, CC) is possible but large |
| iNaturalist field guides (subset) | API/custom build | ~500 MB-2 GB | extended | CC-licensed observations; could build regional species guides. Huge dataset, needs filtering |
| OEIS (Online Encyclopedia of Integer Sequences) | SQLite | ~1-2 GB | reference | Freely available data dumps. Niche but definitive math reference |
| Project Euler problems | static HTML | ~10 MB | reference | Math/programming problems. Small, educational |
| Wolfram MathWorld | -- | -- | -- | Copyrighted; not available for redistribution |
| CRC Handbook of Chemistry and Physics | -- | -- | -- | Copyrighted; no open equivalent at the same depth |

### Field Guides and Identification

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| Plants For A Future (PFAF) | zimit scrape | ~300-500 MB | extended | 8000+ temperate plant species; CC BY 4.0 text. Noted in foraging-identification-guides.md |
| Mushroom Observer | custom build | ~500 MB-2 GB | extended | CC BY-SA; would need regional filtering for practical use |
| FinBIF / laji.fi species data | API build | ~200-500 MB | regional | CC BY 4.0; Finnish biodiversity. Better suited for Finland regional pack |
| BugGuide (US insects) | -- | -- | -- | Copyrighted images; limited redistribution |
| Xeno-canto bird sounds | audio + metadata | ~large | reference | CC-licensed bird recordings; potentially huge, would need regional subset |

---

## Tiering Notes

- **Core** (~8-10 GB): All LibreTexts ZIMs (already have, ~7.8 GB total), periodic table app, star/object catalogs, OpenStax PDFs if packaged.
- **Extended** (~20-30 GB): Add Khan Academy (~10 GB), Stack Exchange science sites (~13 GB total for bio+chem+physics+math), field guides (PFAF, minerals), PubChem subset, NIST subset.
- **Reference** (~35-50 GB): Add OEIS, deeper USGS publications, Stellarium, iNaturalist subsets, Xeno-canto regional subsets.

The LibreTexts collection alone provides a solid science library at under 8 GB. Khan Academy is the single largest item and should be optional in smaller tiers.

Stack Exchange Math at ~10 GB is disproportionately large relative to the other SE sites. Consider whether it belongs in the core science pack or should be an optional add-on.

---

## Relationship to Existing Content

This pack formalizes content that is already partially present in Svalbard presets:

- **default-256** already includes all LibreTexts ZIMs, Khan Academy, and several science Stack Exchanges.
- **default-64** includes Wikipedia (no-pic) which covers science broadly but shallowly.
- The medical/biology boundary overlaps with the existing medical content (`wikimed`, `medlineplus`, `libretexts-medicine`). This pack should include `libretexts-medicine` for anatomy/physiology but defer clinical medicine to the medical sources.

The sciences pack would be the formal home for these sources, making it clear which presets should pull them in.

---

## Open Questions

- Should OpenStax textbooks be packaged as individual PDFs, a combined ZIM, or wait for Kiwix to publish official ZIMs? The deferred-sources doc notes that Kiwix does not currently host OpenStax ZIMs.
- Is a PubChem subset practical? The full database is terabytes; defining a useful subset (e.g., top 10K compounds by relevance, all compounds in LibreTexts chemistry) needs scoping.
- Should field guides (birds, plants, minerals) live in this pack or in a separate "Field Guides" pack? They serve both science education and practical survival use cases.
- How to handle the Stack Exchange Math size problem (~10 GB)? Options: include it only in 256 GB+ tiers, build a curated subset, or accept the size.
- Is there an openly licensed interactive star chart / planetarium that works fully offline in a browser?
