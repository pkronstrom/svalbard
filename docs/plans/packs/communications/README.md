# Pack: Communications

Ham radio, emergency communications, mesh networking, and radio technology references.

## Audience

Ham radio operators, emergency communicators, off-grid networking enthusiasts.

## Status

Idea -- collecting sources

---

## Sub-Topics

### Amateur Radio Fundamentals

License exam preparation, band plans, operating procedures, Q-codes, phonetic alphabet. The ARRL Handbook is the standard reference but is copyrighted; open alternatives are needed.

### Antenna Design and RF Engineering

Dipoles, verticals, yagis, wire antennas, impedance matching, SWR measurement, transmission line theory. Practical build guides matter more than pure theory for this pack.

### Digital Modes and Packet Radio

FT8, JS8Call, PSK31, APRS, Winlink, AX.25 packet radio. These are the modes most useful for emergency text-based communication over HF and VHF.

### Emergency Communications Protocols

ARES/RACES procedures, NIMS/ICS messaging, Winlink gateway operation, emergency traffic handling, SKYWARN. Structured procedures that make ham radio useful in disaster response.

### Mesh Networking

Meshtastic (LoRa-based off-grid text messaging), AREDN (Amateur Radio Emergency Data Network), Reticulum/NomadNet. Low-infrastructure communication for groups without internet.

### SDR (Software-Defined Radio)

RTL-SDR guides, GNU Radio basics, signal identification, spectrum monitoring. Useful for receivers and experimental setups with cheap hardware.

### Frequency Allocation and Regulation

ITU Radio Regulations (relevant excerpts), national band plans (US FCC, IARU Region 1/2/3), frequency allocation charts. Essential reference material for any radio operator.

---

## Source Candidates

### Already in Svalbard Recipes

| Source | Recipe ID | Type | Size | Notes |
|--------|-----------|------|------|-------|
| Amateur Radio Stack Exchange | `stackexchange-amateur-radio` | ZIM | ~0.5 GB | Q&A covering licensing, antennas, operating, digital modes |
| Electronics Stack Exchange | `stackexchange-electronics` | ZIM | ~2.0 GB | RF circuit design, component selection, test equipment |
| Network Engineering Stack Exchange | `stackexchange-networkengineering` | ZIM | ~124 MB | TCP/IP, routing, switching, network design Q&A |

### New ZIM Sources (Kiwix library)

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| Wikipedia (radio/electronics articles) | ZIM | included | -- | Already in base presets; covers fundamentals |

### Candidate Sources Requiring Research

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| ARRL Emergency Communication Handbook (excerpts) | -- | -- | -- | Copyrighted; no open equivalent found. May need to assemble from public ARES/RACES docs |
| KB6NU No-Nonsense Study Guides | PDF | ~5 MB | core | Free Technician/General/Extra exam guides; redistribution terms need checking |
| Meshtastic documentation | zimit scrape | ~50-100 MB | core | meshtastic.org docs; firmware images are hardware-specific but docs are universally useful |
| AREDN documentation | zimit scrape | ~30-50 MB | extended | arednmesh.org docs; mesh networking for ham operators |
| Reticulum / NomadNet docs | zimit scrape | ~10-20 MB | extended | markqvist.net; Reticulum network stack for off-grid messaging |
| ITU Radio Regulations (excerpts) | PDF | ~20-50 MB | reference | ITU publishes partial excerpts freely; full text is paid |
| IARU Region 1 Band Plan | PDF | ~2-5 MB | core | Freely available; essential for European ham operators |
| FCC Part 97 | PDF/HTML | ~1 MB | core | US amateur radio regulations; public domain |
| RTL-SDR Blog tutorials | zimit scrape | ~100-200 MB | extended | rtl-sdr.com; practical SDR guides and project ideas |
| GNU Radio wiki/tutorials | zimit scrape | ~50-100 MB | extended | wiki.gnuradio.org; SDR software framework documentation |
| AC6V Amateur Radio Reference | zimit scrape | ~10-20 MB | core | Compact reference pages: Q-codes, band plans, propagation |
| Signal Identification Wiki | zimit scrape | ~200-500 MB | extended | sigidwiki.com; identify signals by waterfall pattern. Image-heavy |
| Emergency Communications pocket guides (FEMA/DHS) | PDF | ~5-10 MB | core | Public domain US government publications |
| Practical Antenna Handbook (expired edition) | -- | -- | -- | Copyrighted; need to find open antenna design references instead |
| RSGB/DARC band plan references | PDF | ~2-5 MB | reference | European amateur radio society publications |

### Software and Firmware (documentation only)

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| CHIRP documentation | zimit scrape | ~10 MB | extended | Radio programming software docs |
| Dire Wolf documentation | static docs | ~5 MB | extended | Software TNC/APRS docs (GitHub) |
| fldigi documentation | zimit scrape | ~10 MB | extended | Digital mode software docs |
| JS8Call documentation | zimit scrape | ~5 MB | extended | Keyboard-to-keyboard HF messaging |
| Winlink Express guides | PDF | ~5-10 MB | core | Email-over-radio; critical for emergency comms |

---

## Tiering Notes

- **Core** (~0.6-0.8 GB): Amateur Radio SE (already have), exam study guides, band plans, IARU/FCC regulations, Meshtastic docs, emergency comms pocket guides, Winlink guides.
- **Extended** (~2-4 GB): Add Electronics SE (already have), SDR guides (RTL-SDR, GNU Radio), Signal ID Wiki, AREDN docs, digital mode software docs, antenna design references.
- **Reference** (~4-6 GB): Add ITU excerpts, RSGB/DARC materials, deeper RF engineering references from LibreTexts Engineering.

Most of the core material is small. The pack's total size is driven by image-heavy sources (Signal ID Wiki) and the two Stack Exchange ZIMs that already exist.

---

## Relationship to Existing Content

- `stackexchange-amateur-radio` and `stackexchange-electronics` already ship in default-256 preset. This pack would formalize them as the radio/comms core and add focused supplementary sources.
- `libretexts-engineering` covers some RF and electrical engineering theory.
- The deferred-sources doc already notes Meshtastic, NomadNet, and ham radio software (CHIRP, Dire Wolf, fldigi, JS8Call) as tracked but not yet packaged.

---

## Open Questions

- Is there an openly licensed alternative to the ARRL Handbook that covers similar ground? The KB6NU study guides cover exam prep but not operating reference.
- Should Meshtastic firmware images be bundled alongside docs, or kept as docs-only? Firmware is hardware-specific and changes frequently.
- Should this pack include a pre-built frequency database (e.g., CSV of repeater directories for specific regions), or is that better handled per-region?
- How much overlap with the Electronics SE is worth keeping vs. splitting into a separate electronics/maker pack?
- Should APRS maps or repeater maps be included as static snapshots, or is that too ephemeral to be useful offline?
