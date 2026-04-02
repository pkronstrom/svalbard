# Pack: Survival & Medical

Practical survival knowledge and medical references for emergency and off-grid scenarios.

## Audience
Anyone preparing for emergencies, off-grid living, or disruptions -- from weekend hikers to long-term grid-down planners.

## Status
Idea -- collecting sources (many already exist as recipes, several used in fi-survival pack)

## Already in recipes

The following content already has recipe YAML files in `recipes/content/` and overlaps
heavily with this pack's scope:

**Medical (6 recipes)**
- `wikimed` -- WikiMed medical encyclopedia (2.0 GB ZIM)
- `wikem` -- WikEM emergency medicine wiki (42 MB ZIM)
- `who-basic-emergency-care` -- WHO Basic Emergency Care triage manual (4 MB PDF)
- `fas-military-medicine` -- US military field medicine manuals (78 MB ZIM)
- `zimgit-medicine` -- practical medicine reference (67 MB ZIM)
- `quick-guides-medicine` -- medical procedure videos (500 MB ZIM)
- `medlineplus` -- NIH consumer health encyclopedia, drugs, lab tests (1.8 GB ZIM)
- `libretexts-medicine` -- open textbooks: anatomy, pharmacology, clinical (1.1 GB ZIM)

**Preparedness & disaster (4 recipes)**
- `ready-gov` -- FEMA emergency preparedness guides (2.3 GB ZIM)
- `zimgit-post-disaster` -- post-disaster shelter, water, sanitation (600 MB ZIM)
- `canadian-prepper-bugout-concepts` -- bugout strategy videos (2.7 GB ZIM)
- `canadian-prepper-bugout-roll` -- gear review videos (930 MB ZIM)
- `canadian-prepper-winter-prepping` -- cold weather preparedness videos (1.2 GB ZIM)
- `canadian-prepper-prepping-food` -- food prepping and storage videos (2.0 GB ZIM)
- `urban-prepper` -- urban survival and preparedness videos (2.1 GB ZIM)
- `s2-underground` -- SIGINT, OSINT, intelligence tradecraft videos (4.4 GB ZIM)
- `lrn-self-reliance` -- bushcraft, log cabin, off-grid living videos (3.7 GB ZIM)

**Food & water (5 recipes)**
- `zimgit-water` -- water sourcing and purification (20 MB ZIM)
- `zimgit-food-preparation` -- food preparation and preservation (90 MB ZIM)
- `based-cooking` -- simple recipe collection (15 MB ZIM)
- `grimgrains` -- plant-based minimal-provision recipes (24 MB ZIM)
- `usda-nutrition` -- USDA nutrient database (19 MB ZIM)

**Practical skills (5 recipes)**
- `zimgit-knots` -- knot-tying reference (27 MB ZIM)
- `ifixit` -- repair guides for electronics and appliances
- `practical-action` -- appropriate technology guides (1.0 GB ZIM)
- `cd3wd` -- appropriate technology library (550 MB ZIM)
- `appropedia` -- sustainability and appropriate technology wiki (550 MB ZIM)

**Q&A (4 recipes)**
- `stackexchange-survival` -- Outdoors & Survival SE (500 MB ZIM)
- `stackexchange-gardening` -- Gardening SE (1.0 GB ZIM)
- `stackexchange-cooking` -- Cooking SE (1.5 GB ZIM)
- `stackexchange-diy` -- DIY SE

## Source candidates

Sources not yet in `recipes/content/` that should be evaluated for this pack.

### Emergency medicine & pharmacology

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| NHS.uk Medicines | ZIM | 17 MB | core | Drug monographs -- dosages, interactions, side effects. Already on `download.kiwix.org/zim/zimit/nhs.uk_en_medicines_2025-12.zim` |
| CDC Travelers' Health / MMWR | ZIM | 170 MB | core | CDC disease references. `download.kiwix.org/zim/zimit/wwwnc.cdc.gov_en_all_2024-11.zim` |
| Libre Pathology | ZIM | 80 MB | extended | Pathology knowledge base with 7k articles. `download.kiwix.org/zim/other/librepathology_en_all_maxi_2025-09.zim` |
| Hesperian "Where There Is No Doctor" | PDF/scrape | ~30 MB | core | Classic low-resource medicine manual. Free online at `en.hesperian.org`. No ZIM yet -- would need zimit scrape or PDF bundle |
| Hesperian "Where There Is No Dentist" | PDF/scrape | ~10 MB | core | Dental emergency care for non-dentists. Same source |
| Hesperian "Where Women Have No Doctor" | PDF/scrape | ~20 MB | extended | Reproductive health for low-resource settings |
| Hesperian "A Book for Midwives" | PDF/scrape | ~20 MB | extended | Childbirth and prenatal care in austere conditions |
| WHO Essential Medicines List | PDF | ~5 MB | extended | Model list of essential medicines with dosing guidance. Free from `who.int` |
| Psychological First Aid (WHO) | PDF | ~2 MB | core | PFA field guide for crisis response. Free from `who.int/publications` |
| SPHERE Handbook | PDF | ~10 MB | extended | Humanitarian standards for disaster response (shelter, WASH, food, health) |
| TCCC Guidelines (public) | PDF | ~5 MB | core | Tactical Combat Casualty Care -- hemorrhage control, airway, circulation. Public guidelines from `deployedmedicine.com` |
| Tintinalli's Emergency Medicine (open chapters) | -- | -- | research | Check whether any open-access chapters exist |

### Wilderness survival & bushcraft

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| Wikibooks (full EN) | ZIM | 2.9 GB (nopic) | extended | Contains "Outdoor Survival", "First Aid", "Orienteering" and other relevant wikibooks. Already have a recipe but not in fi-survival |
| WikiCiv | ZIM | 4.5 MB | core | "Wiki manual for building civilization from scratch." `download.kiwix.org/zim/other/wikiciv_en_all_maxi_2025-11.zim` |
| Survivor Library | ZIM | 235 GB | archive | Massive historical self-sufficiency archive. `download.kiwix.org/zim/zimit/survivorlibrary.com_en_all_2025-12.zim`. Too large for most drives; consider curated subset |
| FM 21-76 US Army Survival Manual | PDF | ~15 MB | core | Public domain classic. Available from multiple sources |
| SAS Survival Handbook (summary) | -- | -- | research | Copyrighted -- check for equivalent open content |

### Food, water & agriculture

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| FAO food preservation manuals | PDF | ~50 MB | extended | UN FAO guides on drying, smoking, canning, fermentation. Free at `fao.org` |
| Wikiversity (EN) | ZIM | 1.5 GB (nopic) | extended | Contains agriculture, nutrition, and biology courses. `download.kiwix.org/zim/wikiversity/` |
| Energypedia | ZIM | 760 MB | extended | Already has recipe -- renewable energy + water/sanitation. Relevant crossover |

### Nuclear, chemical, biological response

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| FEMA Nuclear War Survival Skills (Cresson Kearny) | PDF | ~15 MB | core | Public domain (ORNL). Classic nuclear preparedness manual |
| EPA Protective Action Guides | PDF | ~10 MB | extended | Radiation emergency response guidance |
| REMM (Radiation Emergency Medical Management) | scrape | ~50 MB | extended | HHS radiation injury guidance at `remm.hhs.gov`. Would need zimit |
| CDC CBRN guidance pages | included | -- | core | Partially covered by CDC ZIM above |

### Mental health & psychological resilience

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| WHO Psychological First Aid | PDF | ~2 MB | core | (listed above) |
| Hesperian "Promoting Community Mental Health" | PDF/scrape | ~15 MB | extended | Community mental health in resource-limited settings |
| IASC Mental Health Guidelines in Emergencies | PDF | ~3 MB | extended | Inter-Agency Standing Committee guidelines. Free at `interagencystandingcommittee.org` |

### Veterinary & animal care

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| Hesperian "Where There Is No Animal Doctor" | PDF/scrape | ~20 MB | extended | Livestock health for remote settings |
| Merck Veterinary Manual (online) | scrape | ~200 MB | research | Free online at `merckvetmanual.com` -- check ToS for offline archival |

## Subsections

### Emergency medicine
Core clinical references for triage, trauma, and common emergencies.
Key sources: WikEM, WikiMed, WHO BEC, FAS Military Medicine, TCCC, Hesperian WTIND.
Covers: airway management, hemorrhage control, fractures, burns, bites/stings, infections, medication dosing.

### Wilderness survival
Orientation, shelter, fire, signaling, navigation, and field craft in austere environments.
Key sources: SE Survival, lrn-self-reliance, FM 21-76, WikiCiv, zimgit-knots.
Covers: shelter construction, fire starting, land navigation, weather reading, rope work, tool making.

### Food & water
Sourcing, purifying, preserving, and preparing food and water with minimal infrastructure.
Key sources: zimgit-water, zimgit-food-preparation, based-cooking, USDA nutrition, FAO manuals, cd3wd.
Covers: water purification methods, food preservation (drying, smoking, fermenting, canning), foraging safety, caloric planning, cooking with limited fuel.

### Preparedness & resilience
Planning, kits, communications, psychological readiness, and community coordination.
Key sources: ready-gov, urban-prepper, Canadian Prepper series, WHO PFA, NWSS.
Covers: emergency kits (72h / 2-week / long-term), communication plans, radio basics, nuclear/CBRN response, psychological first aid, community organizing.

## Tiering notes

- **2 GB** (USB stick / minimal): WikEM (42 MB) + WHO BEC (4 MB) + zimgit-medicine (67 MB) + zimgit-water (20 MB) + zimgit-food-preparation (90 MB) + zimgit-knots (27 MB) + zimgit-post-disaster (600 MB) + NHS medicines (17 MB) + CDC (170 MB) + WikiCiv (4.5 MB) + Hesperian PDFs (~80 MB) + NWSS PDF (~15 MB) + FM 21-76 (~15 MB) + TCCC (~5 MB) + WHO PFA (~2 MB) = **~1.2 GB**. Pure text/reference, no video. Fits on the smallest drives with room to spare.

- **32 GB** (SD card / small SSD): Everything from 2 GB tier + WikiMed (2 GB) + MedlinePlus (1.8 GB) + ready-gov (2.3 GB) + USDA nutrition (19 MB) + based-cooking (15 MB) + grimgrains (24 MB) + SE Survival (500 MB) + quick-guides-medicine video (500 MB) + Libre Pathology (80 MB) + LibreTexts Medicine (1.1 GB) + practical-action (1 GB) + cd3wd (550 MB) + appropedia (550 MB) + FAO manuals (~50 MB) = **~12 GB**. Deep medical references plus appropriate technology. Leaves room for a small LLM or map tiles.

- **128+ GB** (large SSD): Everything from 32 GB tier + all Canadian Prepper videos (6.8 GB) + urban-prepper (2.1 GB) + lrn-self-reliance (3.7 GB) + s2-underground (4.4 GB) + wikibooks-en (2.9 GB) + SE Gardening (1 GB) + SE Cooking (1.5 GB) + energypedia (760 MB) + Wikiversity (1.5 GB) + Survivor Library subset (TBD) = **~40 GB** of survival/medical content. Comprehensive video library plus deep textbook coverage.

## Open questions

- **Hesperian scrape feasibility**: The Hesperian HealthWiki (`en.hesperian.org`) has no official ZIM. Need to test zimit scrape quality and confirm license allows redistribution. Their books are CC BY-NC but verify terms.
- **Survivor Library curation**: At 235 GB the full archive is impractical. Need to identify a curated subset of the most useful titles (pre-industrial technology, medicine, agriculture) at perhaps 5-20 GB.
- **TCCC / deployed medicine**: The public TCCC guidelines are freely available but confirm the exact redistribution terms for packaging.
- **Overlap with fi-survival**: This pack shares most of its core with the fi-survival pack. Should survival-medical be a superset that fi-survival inherits from, or should they remain independent with shared recipes?
- **Non-English medical content**: MedlinePlus and ready.gov both have Spanish ZIMs on Kiwix. Should we include multilingual medical references for broader utility?
- **Merck Vet Manual**: Free to read online but ToS may prohibit offline archival. Need to verify.
- **REMM scrape**: Radiation Emergency Medical Management at `remm.hhs.gov` is US government content (likely public domain) but needs zimit testing.
- **Video vs. text ratio**: At the 32 GB tier, should we prioritize more text references or include some video (quick-guides-medicine)? Video is high-value for procedures but storage-expensive.
- **Dental emergency gap**: No dedicated dental emergency ZIM exists. Hesperian's "Where There Is No Dentist" is the best candidate but needs packaging work.
- **LLM pairing**: A medical-tuned GGUF model (e.g., medicine-focused Llama fine-tune) could add interactive triage capability. Research which models exist and their licensing.
