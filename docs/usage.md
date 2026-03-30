# Svalbard Usage Guide

Svalbard has two layers of state:

- Workspace state: reusable local sources, generated artifacts, and custom presets
- Drive state: one initialized bundle with its own manifest and config snapshot

The CLI is built around that split.

## Core Flow

1. Initialize a drive from a preset.
2. Add extra content into the workspace.
3. Attach selected workspace sources to a drive.
4. Sync the drive.

Example:

```bash
uv run svalbard init /Volumes/MyDrive/svalbard --preset default-128
uv run svalbard add https://www.youtube.com/watch?v=Zhu2NCpT7T8
uv run svalbard attach local:you-can-legally-claim-u-s-land /Volumes/MyDrive/svalbard
uv run svalbard sync /Volumes/MyDrive/svalbard
```

If you run `attach`, `detach`, `sync`, `status`, or `audit` from inside the drive directory, the path argument is optional.

## Workspace Location

Svalbard resolves the active workspace like this:

1. `--workspace <path>` if you pass it
2. The current directory if it already looks like a Svalbard workspace
3. `~/.local/share/svalbard` otherwise

When you run Svalbard from a source checkout, built-in presets stay in the repo and workspace-owned presets go under `.svalbard/presets/`.

Workspace directories:

- `generated/`: produced artifacts such as generated `.zim` files
- `local/`: local source sidecars
- `presets/` or `.svalbard/presets/`: user-owned custom presets

## Commands

### `svalbard add <input>`

Register or acquire content into the active workspace.

- Existing path: register it directly as a reusable source
- Website URL: crawl with Zimit and register the resulting ZIM
- Media URL: download with `yt-dlp` or `yle-dl`, package into a browsable ZIM, then register it

Useful flags:

- `--kind auto|local|web|media`
- `--runner auto|docker|host`
- `--quality 1080p|720p|480p|360p|source`
- `--audio-only`
- `-o, --output <name>`
- `--workspace <path>`

Remote ingestion currently defaults to Docker-backed runners.

### `svalbard attach <source-id> [drive-path]`

Attach a workspace-local source to a drive manifest and snapshot its metadata into the drive.

Example:

```bash
uv run svalbard attach local:example-docs /Volumes/MyDrive/svalbard
```

### `svalbard detach <source-id> [drive-path]`

Remove an attached source from the drive manifest and remove its drive-local snapshot.

### `svalbard sync [drive-path]`

Materialize the selected preset sources and attached local sources onto the drive.

### `svalbard preset list`

List both built-in presets and workspace-owned presets.

### `svalbard preset copy <built-in> <new-name>`

Copy a built-in preset into the active workspace so you can edit it safely.

Example:

```bash
uv run svalbard preset copy default-128 field-kit
```

## Drive Snapshot

Each initialized drive stores a reproducible config snapshot under `.svalbard/config/`.

That snapshot includes:

- the resolved preset YAML used to initialize the drive
- the built-in recipe YAMLs that preset depends on
- snapshots of attached local source metadata

This lets a drive stay understandable and reproducible even if the installed Svalbard package changes later.

## Notes

- `svalbard add` creates reusable sources in the workspace. It does not automatically include them in any drive.
- `attach` changes drive membership. `sync` copies the selected content onto the drive.
- Built-in presets are read-only; customize them with `preset copy` instead of editing package files.
