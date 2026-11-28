<p align="center">
  <img src="docs/svalbard_banner.png" alt="An Arctic explorer holds a USB drive that projects a map, a botanical study, and a waterwheel drawing beneath the aurora." width="100%">
</p>

# Svalbard

**An offline library and a local AI assistant on one USB drive.** Svalbard fills a drive with reference archives, maps, manuals and tools that work without the internet. On a Mac or Linux computer, browse and search them yourself, or ask a local model to find the answer in them and save what it makes, such as a checklist or a small web page.

Build the drive while you’re online. Everything on it then works offline.

> **Alpha (v0.8.0).** Svalbard is under active development. Presets, datasets and curation still change, so expect rough edges.

## Ask the drive

The assistant runs on your own computer and answers from what is on the drive. It searches the archives and your documents, reads the pages it finds and names them. Check those pages before you act on an answer: a small local model can misread one.

> Is there anything in the archives about building a survival shelter?

> Car won’t start. Old Toyota, petrol engine, the starter just clicks.

> What should I put on a mild burn?

> I have a day in Tallinn. What should I see, and how do I get from the ferry port to the old town?

> When should I plant potatoes in southern Finland, and how deep?

> What tyre pressure does the manual for my 2012 Corolla give for a full load?

Ask the way you would ask a person. The assistant searches the survival, repair, first-aid, travel and gardening archives on the drive, reads what it finds and names the pages it used. The last question needs your car’s manual on the drive: add it as one of [your own documents](#your-own-documents). The first-aid material is reference, not medical advice.

## Make something

The assistant can also write things and save them in the drive’s `workspace/` folder.

> My cabin has a 200 W solar panel and a 100 Ah 12 V battery. Make me a page where I enter what I plug in and for how many hours a day, and it tells me whether the battery lasts the weekend. Take the numbers from the solar pages on the drive.

> Write a JSCAD model of a wall bracket for a 25 mm pipe, 4 mm thick, with two screw holes, so I can open it in JSCAD and export an STL.

> Write ESP32 firmware that reads a soil-moisture sensor on pin 34 every ten minutes and switches the pump relay on pin 26 when the soil is dry. Make it a PlatformIO project.

The calculator is one HTML file that opens in any browser. JSCAD runs from the drive’s **Apps** menu. The firmware builds offline with the PlatformIO toolchain in the `esp32-dev` pack (`./run embedded`); flashing needs the board plugged in.

> Make a world map of population by country, with a chart of the ten largest, from the data on the drive.

> Make a parametric spur gear I can export as STL.

With the AI clients come **skills**: short, tested procedures for dashboards, maps, diagrams, CAD parts, spreadsheets, PDFs, Word and PowerPoint files, research and firmware. The assistant finds them with `skill_search` and follows their steps. The JavaScript libraries they use (Chart.js, D3, Three.js, JSCAD, MapLibre and others) are pinned on the drive, and `svalbard-drive html pack` bundles a page and its libraries into one file that opens offline by double-click. Both prompts above produced such a file on Qwen3.8 27B and on Qwen3.5 4B.

### What a session looks like

Here a local Qwen3.5 4B model was asked how paper is made and how CARDIAC, a computer made of cardboard, works. It searched Wikiciv and Permacomputing, read both articles, and saved an interactive page that cites them.

![Open WebUI showing Qwen searching the vault, reading archived articles and writing an HTML page.](docs/open-webui-make-a-page.webp)

![The generated page opened as a local HTML file.](docs/offline-paper-visualization.webp)

The screenshots were captured on an Apple Silicon Mac with the vault on its internal SSD. To work this way, add the **Open WebUI** recipe and enable its file tools so the assistant can write to the vault’s `workspace/` folder.

## Explore the drive yourself

### Read the original in Kiwix

Choose **Library**, open an installed archive, and search for the article title. Here’s *Paper*, one of the two articles in the session above. Follow its links, look at the diagrams, and check what the assistant added.

![Kiwix displaying the archived Wikiciv article on paper with its search box and article text.](docs/kiwix-paper.webp)

### Maps without a connection

Open **Maps**, choose a basemap, and switch the overlays on or off. When the water is cut off in Helsinki, turn on HSY’s public water posts to find the nearest one; before a hike, add SYKE’s protected areas and rivers. Pan and zoom work offline. **Turn-by-turn directions and route calculation are still planned.**

![Svalbard Maps showing Helsinki harbour with the topographic basemap, public water posts and protected areas enabled.](docs/offline-map-helsinki.webp)

This capture used three local PMTiles extracts. The basemap came from National Land Survey of Finland sheet L41R. The `mml-maastokartta` recipe renders the same MML map series for Uusimaa at zooms 0–14, from the 1:500k to 1:50k sheets. Map data © National Land Survey of Finland, Helsinki Region Environmental Services (HSY), and Finnish Environment Institute (SYKE), CC BY 4.0. [Topographic sheet](https://kartat.kapsi.fi/files/maastokarttarasteri_100k_jhs180/kaikki/etrs89/png/L4/L41/) · [Water-post source](recipes/maps/hsy-vesipostit.yaml) · [Protected-areas source](recipes/maps/luonnonsuojelualueet.yaml)

## Install

Download the host binary for your Mac or Linux computer, extract it, and put `svalbard` on your path. The macOS binaries are not signed yet: if macOS refuses to open one you downloaded in a browser, run `xattr -d com.apple.quarantine svalbard` once. To build from a checkout with Go installed, run `make build` and use `bin/svalbard`. Standard downloads use the host binary directly. Recipes that build tools, including the optional Open WebUI install, use Docker during preparation. [Release downloads](https://github.com/pkronstrom/svalbard/releases) · [Usage guide](docs/usage.md)

### Prepare the drive

1. **Choose a destination.** Run `svalbard init` to open the guided setup. Pick a folder or mounted drive and the platforms you’ll use it on. A USB stick works best formatted as exFAT with small clusters (4 KB): large clusters waste gigabytes on the many small files of Python apps, and `init` warns when a stick uses them. [Preparing a stick](docs/usage.md)
2. **Choose the contents.** Start with a capacity preset, review its contents, then add or remove items. To ask questions like the ones above, add Open WebUI and a model that fits your computer’s memory; the screenshots use Qwen3.5 4B, which runs on an 8 GB machine. Keep the suggested dependencies and keyword indexing.
3. **Apply while online.** Let the downloads and selected indexing finish. The vault keeps both your selections and a record of what was installed. Reopen it later to change its contents.

<img src="docs/demo.gif" alt="Terminal setup wizard for choosing a vault and its contents" width="100%">

### Use the drive offline

Open the vault folder in a terminal and run `./run` to launch its keyboard-driven menu. **Library** opens installed archives through Kiwix. **Search** looks across indexed articles. **Maps** opens installed maps. With the SilverBullet recipe, **Wiki** is a notebook on the drive that the assistant can read and, once you allow it in **Settings**, write to. **Workspace** opens the pages the assistant saved. On a phone or tablet, a compatible reader such as Kiwix can open the ZIM files directly.

Standard presets include the optional **web-launcher** recipe. Double-click **Svalbard.app** on macOS or **Svalbard.desktop** on Linux to open the atlas. The small graphical wrappers can also be copied beside `run` on an existing atlas-enabled drive. They use the same vault runtime as `./run launch`. The terminal menu remains available through `./run`, and `./run web` starts a foreground dashboard. [Launcher, removal and packaging guide](docs/web-launcher.md)

For browser chat, choose **Open WebUI** under **Local AI** and select your installed model if prompted. The drive starts the model and its vault tools locally and opens the chat in your browser. Small models answer better when you ask them to search first and read the pages they found before they answer.

```text
Browser ── Open WebUI ──┬── local model (Qwen in the examples)
                        └── Svalbard search and read tools ── archives and documents
```

### Bring your own agent

Give your agent the vault path, storage budget and subjects you need. Ask it to check the installed binary’s available commands, show the plan, and report which downloads completed. For interactive research on a prepared drive, supported clients such as OpenCode and Goose can use Svalbard’s Model Context Protocol (MCP) tools to look up sources. [Command reference and vault layout](docs/usage.md)

### Custom presets

To repeat a setup, copy a built-in preset with `svalbard preset copy default-32 my-field-kit`. Edit the YAML under `~/.config/svalbard/presets/`, then select the named preset in the wizard. You can also import your own files into the vault. [Presets and imports](docs/usage.md)

### Your own documents

List your own PDFs in a collection file under `~/.config/svalbard/recipes/`: links, single files or whole folders. Add the collection like any recipe. Svalbard downloads or copies the documents into the vault’s `books/` folder and indexes their pages, so **Search** and the assistant can find, read and cite them. The vault keeps a copy of the collection file, so you can still update it from another computer. PDFs and EPUBs packed inside archives, such as field manuals in the zimgit collections, are indexed the same way. [Document collections](docs/usage.md#your-own-documents)

## License

Svalbard is licensed under [GPL-3.0](LICENSE). The archives, tools and models you choose retain their own licenses.
