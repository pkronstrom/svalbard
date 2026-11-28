# Vault research benchmark

This benchmark measures whether a model can find, read, and combine evidence from an offline Svalbard vault through Open WebUI. It covers Finnish retrieval, cross-language synthesis, PDF page lookup, and a question the available material cannot answer.

## Test setup

Use one Open WebUI instance on a scratch vault. Serve each model with the same bundled llama.cpp build and a 16,384-token context. Enable Svalbard's native vault tools. Start each question in a fresh chat. Make one untimed chat-completion request before scoring each model to load it, then record end-to-end seconds for each scored question, including search and page reads. Send `temperature: 0` and `max_tokens: 850` in the chat request. Keep Open WebUI's normal tool-handling and compaction behavior and apply an 80-second limit per question. Save the final response and the actual tool calls and outputs.

Freeze the keyword index before the run. The 2026-11-27 corpus contains five ZIMs and one PDF, with 15,601 indexed articles and pages.

- `scoutwiki.org_fi_all_maxi_2026-11.zim`
- `wikibooks_fi_all_maxi_2026-10.zim`
- `phet_fi_all_2026-11.zim`
- `based.cooking_en_all_2026-11.zim`
- `foss.cooking_en_all_2026-11.zim`
- `books/TM_9-8000_Principles_of_Automotive_Vehicles_1985.pdf`

Use this system prompt for every model.

> Use the Svalbard vault tools as your only evidence. Search the vault, then read relevant archive articles or PDF pages. Use short searches and at most 4 results per search. Cite precise sources where requested. If evidence is missing, say what you could verify. Keep final answers under 120 words.

## Questions and scoring

Each semantically equivalent criterion is worth one point, for 16 points total. Award the retrieval point only when the tool trace returns the expected article or page with relevant evidence. Award citation points for an explicitly stated archive path, a working link returned by the tools, or an indexed PDF page number where requested. Treat a made-up URL as an invalid citation even if the URL contains the correct path. Record timeouts, tool errors, and unsupported claims separately. Rank by evidence score first, then compare latency and model size on tied or comparable scores.

### 1. Finnish archive lookup

> Etsi PartioWikin Jokamiehenoikeudet-sivu. Mitä siinä sanotaan luonnonmarjojen poimimisesta ja kuivuneen tai kaatuneen puun ottamisesta toisen maalta? Vastaa suomeksi ja ilmoita sivun nimi sekä polku tai linkki.

Award one point each for retrieving the article, saying the page allows picking wild berries, saying the page forbids taking dead or fallen wood from someone else's land, and citing `/Jokamiehenoikeudet` in PartioWiki.

### 2. Synthesis across Finnish and English sources

> Vertaa Based Cookingin Kropsua-reseptiä ja suomenkielisen Wikikirjaston Amerikkalainen pannukakku -reseptiä. Kumpi paistetaan uunissa, missä lämpötilassa ja kuinka kauan? Milloin toisessa reseptissä pannukakku käännetään? Vastaa suomeksi ja viittaa kumpaankin arkistosivuun nimellä sekä polulla tai linkillä.

Award one point each for retrieving both recipes, identifying Kropsua's oven method at 400 °F (about 204 °C) for 40 minutes, identifying the pan recipe's flip when bubbling slows and the edges dry slightly, and citing both `/based.cooking/kropsua/` and `/Keittokirja/Amerikkalainen_pannukakku` in the correct archives.

### 3. PDF page lookup

> In the vault PDF TM 9-8000 Principles of Automotive Vehicles, find the thermostat passage. What two problems does the manual say the thermostat addresses, and what position is it designed to fail in? Cite the indexed PDF page number and give a short answer.

Award one point each for retrieving section 9-9, identifying quick warmup and prevention of overcooling in cold weather, saying the thermostat is designed to fail open, and citing indexed PDF page 241 (`page/241`).

### 4. Evidence missing from the vault

> Can the copy of TM 9-8000 Principles of Automotive Vehicles in this vault establish the manufacturer-approved thermostat opening temperature for the 2026 Ford Mustang GT? Check what the archive actually covers. If it cannot establish the value, say so and cite what you checked.

Award one point each for inspecting the manual through the tools, identifying it as a 1985 general automotive manual or showing that its thermostat section is generic, declining to give a vehicle-specific opening temperature, and citing the manual and checked page or section. PDF page 1 establishes the date and scope. Page 241, section 9-9, establishes what the manual says about thermostats.

## Small-model results on 2026-11-27

Ran 20 scored chats through the same Open WebUI instance and model dropdown on an Apple M4 Max with 128 GiB RAM and llama.cpp b12774. The USB's separate indexer was running throughout. This is one run per question. The times describe this machine and run. Medians include timed-out questions at the recorded cutoff. Weights are decimal GB.

| Model | Weights | Finnish | Synthesis | PDF | Missing | Total | Median | Timeouts |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Qwen3.5 4B Q4_K_M | 3.01 GB | 4 | 3 | 4 | 4 | **15/16** | 14.0 s | 0 |
| Ternary Bonsai 2 4B Q2_0 | 1.21 GB | 3 | 4 | 4 | 1 | **12/16** | 13.3 s | 1 |
| Gemma 4 E4B IT QAT UD-Q4_K_XL | 4.22 GB | 4 | 4 | 0 | 3 | **11/16** | 18.0 s | 0 |
| MiniCPM5-2B Q4_K_M | 1.56 GB | 0 | 3 | 4 | 3 | **10/16** | 16.3 s | 1 |
| Nemotron 3.5 Bolt E4B Q4_K_M | 6.18 GB | 4 | 1 | 3 | 2 | **10/16** | 13.7 s | 0 |

- **Qwen** completed all four cases. It made up citation URLs for two recipes it actually retrieved, losing the synthesis citation point. Its missing-evidence answer correctly withheld the requested value and cited the thermostat section. Its claim about which manufacturers the entire manual mentions went beyond what its compound search established.
- **Bonsai** returned the correct recipe facts with both archive paths and correctly cited the thermostat passage. It gave the correct Finnish archive path alongside a made-up link and an unsupported exception about taking wood. On the missing-evidence case it read 25 pages near the back of the manual and timed out without a final answer.
- **Gemma** had clean citations on both Finnish tasks. It mistook an air-cooling passage on page 247 for the thermostat failure behavior on page 241. Its missing-evidence answer gave the correct date and scope, with an archive path but no page.
- **MiniCPM** cited both recipes but invented a 100 °C temperature for the pan recipe and missed the flip cue. In the Finnish lookup it repeatedly searched after finding the right article and timed out without reading it.
- **Bolt** found both recipe paths, then repeatedly opened archive home pages because it omitted the path on its read calls. It correctly retrieved the opening of page 241 but echoed the same page 247 failure explanation as Gemma. It declined to give a vehicle-specific temperature after a vault-wide search without inspecting the PDF itself.

Keep Qwen3.5 4B as the default for bilingual vault research. Bonsai is the strongest compact fallback in this run, using about 40% of Qwen's storage. Its failure to stop on missing evidence is the main reliability gap. Bolt scored lower at about twice Qwen's size.

## Extended benchmark for larger models

Keep the original four questions and their 80-second limit. Run the two additional questions below on both larger models and the Qwen3.5 4B reference. Allow 180 seconds for each additional question and keep the same keyword index, 16,384-token context, native tools, and chat settings. Award six points per new question and report those scores separately from the original 16 points.

Use the installed model quantizations. For the text-only run, omit the vision projectors and retain any configured drafting sidecars. Report warm response time separately from the first model request, which includes loading from the drive.

### 5. Distinguish duplicate evidence from an independent source

> The vault has a Kropsua recipe in both Based Cooking and FOSS Cooking, and an Amerikkalainen pannukakku recipe in Finnish Wikibooks. Read all three. Report the Kropsua oven temperature and time and one distinctive preparation step shared by both copies. Does finding the recipe in two archives give you two independent sources for its directions? Explain from the pages. Contrast when the Wikibooks pancake is flipped. Cite all three archive paths or working offline links.

1. Retrieve relevant evidence from all three recipe pages.
2. Report that both Kropsua pages specify 400 °F (about 204 °C) for 40 minutes.
3. Identify a distinctive shared step, such as melting butter in a 9 × 13 inch pan in the preheated oven before pouring in the batter.
4. Explain that matching text and instructions suggest copies of the same recipe and provide a specific textual comparison. The archive names alone do not establish independent provenance.
5. Explain that the Wikibooks pancake is flipped when bubbling slows and the edges dry slightly.
6. Cite `/based.cooking/kropsua/`, `/foss.cooking/recipe/albert-elmore/kropsua`, and `/Keittokirja/Amerikkalainen_pannukakku` in the correct archives or provide corresponding tool-verified links.

### 6. Combine a Finnish table and an English PDF

> Käytä Wikikirjaston taulukkoa Veden kiehumispiste eri paineissa ja TM 9-8000 -oppaan jäähdyttimen painekorkkia käsittelevää kohtaa. Mitkä ovat taulukon veden kiehumispisteet 100 ja 106,7 kPa:n paineissa, ja paljonko ne eroavat? Miksi painekorkki vähentää ylikiehumista, ja kumpi sen venttiileistä aukeaa, kun paine ylittää sallitun? Kerro, mitä arvoista voi päätellä auton todellisesta jäähdytysnesteestä. Vastaa suomeksi ja merkitse taulukon arkistopolku sekä oppaan PDF-sivu.

1. Retrieve the Finnish table and the radiator pressure cap section in the PDF.
2. Report 99.6 °C at 100 kPa and 101.4 °C at 106.7 kPa.
3. Compute a difference of 1.8 °C.
4. Explain that maintaining pressure raises the boiling point and that the pressure relief valve opens above the regulated maximum.
5. Identify the values as an illustration for water and avoid applying them directly to an actual vehicle coolant mixture or cap setting.
6. Cite `/Lukion_taulukot/Veden_kiehumispiste_eri_paineissa` in Finnish Wikibooks and PDF page 242 of TM 9-8000.

### Larger-model results on 2026-11-27

Ran the installed Qwen3.8 27B UD-IQ4_XS with MTP and Inkling 2 Mini UD-Q4_K_M with DSpark at 16K context. Used the same frozen corpus and M4 Max as the small-model run. The table shows weight storage including drafts and warm median response time on the original four cases, including the cutoff for the timed-out question.

| Model | Weights and draft | Original score | Original median | Duplicate sources | Pressure table | Total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Inkling 2 Mini + DSpark | 17.11 GB | 16/16 | 32.9 s | 6/6 in 35.1 s | 6/6 in 44.6 s | **28/28** |
| Qwen3.5 4B | 3.01 GB | 15/16 | 14.9 s | 5/6 in 31.1 s | 5/6 in 31.4 s | **25/28** |
| Qwen3.8 27B + MTP | 15.72 GB | 13/16 | 49.3 s | 6/6 in 46.6 s | 5/6 in 89.4 s | **24/28** |

- **Inkling** completed all six questions. It used the link tool for both recipes in the original synthesis case, and both offline URLs served the expected pages. It found the thermostat passage, cited the limits of the manual, and correctly combined the Finnish table with the pressure-cap section.
- **Qwen 27B** scored four points each on the Finnish lookup, recipe comparison, and PDF question. It inspected the relevant manual pages for the missing-evidence case and reached the 80-second cutoff before returning an answer. It identified the shared `Teapsoon` typo in both Kropsua copies. On the revised pressure case it gave the right table values and PDF page, then applied an unsupported pressure and boiling point to the actual coolant.
- **Qwen 4B** retrieved all three recipes and returned three working offline links. It treated different archive locations as independent evidence despite quoting the identical preparation steps. In the table case it calculated the right difference and found the correct PDF page, then incorrectly inferred a 6.7 kPa pressure increase for a vehicle from the selected table rows.
- **Pressure-case correction.** An initial question used 200 kPa, which is present in the frozen keyword index. The ZIM article reader returned only the range through 106.7 kPa. The corrected question uses two values present in both outputs. The initial Qwen 27B pressure attempt is kept as a diagnostic and excluded from the score.
- **Timing conditions.** The USB disconnected after Inkling's run. After reconnecting, the suite preserved completed results and ran the remaining cases. The separate USB indexer was active in the first part of the run and stopped for the resumed cases. Treat response times as observed rather than controlled hardware measurements.

Use Qwen3.5 4B for quick lookups and Inkling for research that needs source comparison, precise citations, or care with evidence gaps. This six-question run favors Inkling over Qwen 27B for the research role.
- **Hardware interruption.** macOS lost the external drive after Inkling's run. The pending cases need the mounted vault and its Open WebUI packages. The trace files and frozen index copy are on the host.

### Harness and tool changes to test

1. Test a 12-round cap for Open WebUI using `CHAT_RESPONSE_MAX_TOOL_CALL_ITERATIONS`. The bundled version defaults to 1,024 and already makes a final response request with tools disabled when the cap is reached. Exercise the three-source case alongside the two observed loops. Then test stopping earlier when identical reads or searches yield no new evidence.
2. Build citations from tool outputs. Keep the archive and path or PDF page in chat history. Resolve archive links against the current viewer on click. Reuse the exact path from the search hit when opening a result.
3. Improve PDF name lookup and article reads. Recognize a manual title as a source before ranking its pages, so users can search inside it without opening the back index first. Preserve the association between row headings and values when returning tables, and surface any omitted text or continuation.
