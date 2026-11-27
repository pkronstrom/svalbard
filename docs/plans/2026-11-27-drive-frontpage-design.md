# Offline drive frontpage

The drive's existing `./run` launcher accepts `web` to open an offline dashboard on a private loopback port. The terminal menu also offers it on newly applied vaults. The dashboard uses the installed groups, descriptions and arguments from `.svalbard/actions.json`, so it can open individual ZIMs in Kiwix, list document collections, and start maps, Open WebUI, Wiki and bundled web apps. It shows terminal instructions alongside actions that require a terminal.

The page uses the existing Svalbard banner and groups installed items by the same sections as the terminal menu. A catalog filter matches names and descriptions. Indexed search uses the drive's existing read-only search session. The saved HTML gallery discovers pages within `workspace/`, including project subfolders. It serves the pages through the existing confined workspace server on a separate origin. The Wiki action opens the existing SilverBullet editor and preserves its Markdown storage.

A managed action exposes its starting, running, stopped or failed state. The dashboard uses the running service's URL, deduplicates identical launches, and stops its services when it exits. Explicit local-network sharing uses the existing Share action. A loopback address, exact Host check, and same-origin POSTs constrain dashboard controls. Browser requests select a known action and, for Open WebUI, may select an installed model.

Verify the generated launcher and browser routes using a tiny vault with one cached archive, a test document and workspace pages. Exercise search and lifecycle behavior in tests and reuse the existing small launch fixtures for Open WebUI and Wiki.
