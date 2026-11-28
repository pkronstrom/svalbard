# Map label glyphs

MapLibre draws label text from these signed-distance-field glyph ranges. They
are rendered from the full OFL releases of EB Garamond (Regular and Italic,
`ofl/ebgaramond` in github.com/google/fonts) and IBM Plex Mono Regular
(`ofl/ibmplexmono`), covering Latin, Latin Extended, Greek, Cyrillic and
general punctuation. The licences are beside this file.

Regenerate with the build tool, from `build-tools/`:

```sh
go run ./cmd/map-glyphs -name "EB Garamond Regular" -out ../host-cli/internal/mapview/glyphs -ranges 0-1279,7680-8447 EBGaramond[wght].ttf
go run ./cmd/map-glyphs -name "EB Garamond Italic" -out ../host-cli/internal/mapview/glyphs -ranges 0-1279,7680-8447 EBGaramond-Italic[wght].ttf
go run ./cmd/map-glyphs -name "Plex Mono Regular" -out ../host-cli/internal/mapview/glyphs -ranges 0-1279,7680-8447 IBMPlexMono-Regular.ttf
```
