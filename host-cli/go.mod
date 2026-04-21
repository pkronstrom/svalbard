module github.com/pkronstrom/svalbard/host-cli

go 1.25.6

require (
	github.com/ncruces/go-sqlite3 v0.33.3
	github.com/pkronstrom/svalbard/host-tui v0.0.0-00010101000000-000000000000
	github.com/pkronstrom/svalbard/tui v0.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.9.1
	github.com/stazelabs/gozim v0.1.0
	golang.org/x/sys v0.43.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/charmbracelet/bubbles v1.0.0 // indirect
	github.com/charmbracelet/bubbletea v1.3.10 // indirect
	github.com/charmbracelet/colorprofile v0.4.1 // indirect
	github.com/charmbracelet/lipgloss v1.1.0 // indirect
	github.com/charmbracelet/x/ansi v0.11.6 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.15 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.9.0 // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/ncruces/go-sqlite3-wasm v1.1.1-0.20260409221933-87e4b35a38d0 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/text v0.36.0 // indirect
)

replace github.com/pkronstrom/svalbard/host-tui => ../host-tui

replace github.com/pkronstrom/svalbard/tui => ../tui
