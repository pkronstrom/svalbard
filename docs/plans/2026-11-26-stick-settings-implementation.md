# Stick settings: implementation record

Status: done 2026-11-26 on macOS arm64. Not run on Linux, Intel Mac or a physical stick.

Scope: a settings store on the drive, a generated Settings screen in the drive menu, a `./run settings` command, two built-in settings and one recipe-declared setting. The web-search switch, settings for LAN serving, environment export to shell actions and a host-side editor are follow-ups.

## Resulting user path

1. `./run`, then **Settings** (the last entry). Each row is one setting; Enter changes it, `r` returns it to its default, Esc goes back. Every change is saved at once.
2. Or from a shell or a script: `./run settings` lists, `./run settings <key>` prints one value, `./run settings <key> <value>` changes it, `./run settings --reset <key>` returns it to the default.
3. The next Chat, AI client or Open WebUI launch uses the saved choices.
4. Adding a recipe that declares settings and applying adds its group to both; removing it and applying takes the group away and keeps the saved values.

```text
 recipe YAML                      host: svalbard apply            drive: .svalbard/
┌──────────────────┐ installed  ┌───────────────────────┐ writes ┌────────────────────┐
│ settings:        │───────────▶│ toolkit.Generate      │───────▶│ settings.schema    │
│  - name, type,   │ recipes    │  writeSettingsSchema  │        │   .json (generated)│
│    default, ...  │            └───────────────────────┘        │ settings.json      │
└──────────────────┘                                             │   (the user's)     │
                                                                 └─────────▲──────────┘
 built-ins (ai.*), compiled in ──┐                                         │ read, write
                                 ▼                                         │
                      ┌──────────────────────────────────────┐             │
                      │ drive-runtime/internal/settings      │─────────────┘
                      │  schema + values, validate, write    │
                      └──────────────────────────────────────┘
                         ▲               ▲                ▲
              ./run settings     menu: Settings     Chat, AI clients,
              (plain text)       screen             Open WebUI (read only)
```

## The recipe field

A recipe declares the settings it needs. On the drive the full key is `<recipe id>.<name>`.

```yaml
settings:
  - name: open_browser        # [a-z][a-z0-9_]*, unique within the recipe
    type: bool                # bool | choice | number | text | secret
    default: true             # required, of that type
    label: Open browser on start
    description: Open the browser when Open WebUI is ready.
    # choices: [a, b]         required for type choice; the default is one of them
    # min: 1                  optional, type number only
    # max: 10
```

The catalog refuses to load when a setting has an unknown type, no default, a default of the wrong type or outside its choices or range, no label or description, `choices` or `min`/`max` on a type that does not take them, or a name used twice. It also refuses any recipe whose id is `ai`, `network`, `drive` or `svalbard`: those own the built-in settings. The field is documented on `catalog.Item.Settings` and `catalog.SettingSpec`; there is no other recipe field reference in the repository.

## Decisions

- **Values file** `<drive>/.svalbard/settings.json`, nested by owner. Only values that differ from the default are stored, so setting a value back to its default removes the key, and an owner left empty is removed too. Keys are written sorted; hand-written key order and spacing are not kept.
- **Hand edits.** An unknown owner or key is written back as found, including a top-level entry that is not an object. A value of the wrong type or outside its choices or range is ignored, the default is used, and the command prints one `warning:` line for it on stderr; the menu says `Ignored:` on the selected row. A missing, empty or `{}` file means every default.
- **A file that does not parse** is an error naming the path and the reason (the line, for a syntax error). The command prints it and exits non-zero for every form, including the list. The Settings screen shows the defaults and the reason and refuses changes. Nothing ever rewrites such a file.
- **Chat, the AI clients and Open WebUI carry on with defaults when the file does not parse**, after one line saying so. The design did not decide this. Refusing to start would leave a stick unusable until someone edits JSON.
- **Writes** go to a temporary file in `.svalbard/`, are synced, then renamed over the old file. The file is read again just before each write, so a change made by the command while the menu is open is not lost.
- **Schema file** `<drive>/.svalbard/settings.schema.json`, written by apply from the installed recipes that declare settings, one group per recipe, and removed when none does. Apply never touches `settings.json`. The runtime drops a group that names a reserved owner and any entry it could not show, so a hand-edited schema cannot replace a built-in.
- **Group label**: the recipe's `menu.label`, else the label of a fixed built-in menu entry with the recipe's id, else the id. The middle step is an addition to the design: Open WebUI's menu label lives in code (its licence fixes the name), so without it the group would read `open-webui`.
- **Built-ins** are compiled into `drive-runtime/internal/settings`, so the command and the screen work with no schema file:
  - `ai.default_model`: `ask` (default) or one of the chat models in `models/`, named without `.gguf`, read from the drive each time. With `ask` the launch is exactly as before (`llamaserve.ResolveModel` takes the first file). A model named by the menu entry or on the command line always wins. If the saved model has left the drive the launch prints one line and behaves as `ask`.
  - `ai.context`: `ask` (default, the picker at each launch), `auto`, `fast`, `balanced`, `max`. Anything but `ask` skips the picker.
  - oh-my-pi is still refused below 32K. When the size came from the setting, the refusal adds where it came from, because "pick Balanced or Max" alone would point at a picker that was not shown.
- **`open-webui.open_browser`** (bool, default on) is declared by the recipe. With it off the action prints the URL and opens nothing. A drive whose schema predates the setting opens the browser as before.
- **The command prints plain text always**, on a terminal too. It is handled in `run()` before `actions.json` is read, and the settings package imports no terminal library.
- **The Settings entry is generated like Search**: a menu group with one built-in action that the menu handles itself. Its order is 1000 because a group the registry does not know gets 999.
- **Display names**: the menu shows `ask` as `ask every time` and the context choices capitalised, with sizes in the picker. The command and the file use the plain choice, since that is what is typed.
- **The screen's scroll marks sit on the blank lines** above and below the list, so rows do not move when a mark appears, and no line is spent on them.

## Verified, and how

Unit tests (`make verify`, 57 tested packages, up from 55: `settings` and `chat` are new; `make build`):

- Catalog: the field parses for all five types; 23 invalid declarations are each refused with the reason; the real catalog declares `open_browser` on `open-webui` and no recipe uses a reserved id.
- Schema file: written for installed recipes only, once for a two-platform install, with the three label sources; removed when stale; `settings.json` left byte-for-byte alone.
- Menu generation: Settings is the last group, after an unregistered recipe group.
- Store: defaults with no file and with an empty one; unknown owners and keys kept across a rewrite; four kinds of bad value ignored and reported; five kinds of unparsable file refused with path and reason and left unchanged; a failed write leaves the old file whole and no temporary file behind; a second writer's change is kept; nothing is created outside a drive.
- Command: list, get, set, reset, secret masking, one-line errors that name the valid choices, no escape or carriage-return byte in any output; `run()` handles `settings` on a drive with no `actions.json`.
- Screen: the 80x24 render is compared whole and follows the wireframe line for line (the choices fit one line at 80 columns); bool toggle; choice picker; number, text and secret edit, with a refused value kept on the edit line; reset; scrolling through 42 rows in 24 lines with the cursor row, title and footer always on screen; a 33-choice picker; the unparsable-file and ignored-value states.
- Consumers: Chat and the agent launcher skip the picker and pass the saved size and model to `llama-server`; a named model wins; a missing default model falls back with the message; oh-my-pi is refused at `fast` before any process starts; Open WebUI started against stand-in servers does not call the opener with the switch off and still prints the URL.

Real run on this Mac (64 GB), CLI and runtime built from the branch, scratch vault with `llama-server` and `qwen-0.8b`:

- `./run settings` listed the two built-ins; set, get, reset and both error forms behaved as above with exit status 1 on errors.
- Chat from the menu in tmux with `ai.context` at `ask` showed the picker; with `balanced` it did not, and `llama-server` reported `n_ctx = 32768` on `/props` (8192 after picking Fast in the first run). `open` was shadowed on `PATH` by a script that records its argument, so no browser opened; it recorded the Chat URL both times.
- The Settings screen in tmux at 80x24: picked a context size and a model, reset one, and `settings.json` followed each step.
- `open-webui` was not installed (580 MB and a Docker build). Its realized entry was written into the manifest by hand and the item added to `desired`; `svalbard apply` then generated the schema file itself, and the Open WebUI group appeared in the command and on the screen, where the switch was toggled. `svalbard remove open-webui` and apply removed the schema file and the group and kept the saved value.
- A hand-written 27-setting schema at 80x16 exercised scrolling and the number, text and secret edits in a real terminal. This run found that a view ending in a newline is one line too tall when the screen is full, which scrolled the title away; fixed, and the test now counts that line.
- Start-up, 40 interleaved runs each: `./run settings` median 29.0 ms, `./run __native-inspect` 29.9 ms; the binary alone 11.7 ms and 12.0 ms. No escape byte on a pipe or on a pseudo-terminal.

## Not verified

- Linux and Intel Mac, and a physical stick: write behaviour on exFAT, and pulling the stick mid-write.
- A real Open WebUI launch with the switch off. The code path is covered by the stand-in test only.
- The AI clients (pi, oh-my-pi, Goose, OpenCode) started for real with a saved context or model. The decision is tested up to the arguments `llama-server` would get.
- A terminal narrower than 80 columns. Long values are not truncated.

## Found on the way

- **Serve Everything ignores both built-ins.** It starts the first chat model at the Auto size, as before. It belongs with the LAN serving settings.
- Chat ends with "Action failed: exit status 1" in the menu after Ctrl+C. Existing behaviour.
- `ask` for the default model does not ask: with no model named, the runtime takes the first file. The wording follows the agreed design; a real model picker at launch would make it true.

## Follow-ups

Sizes: S one module, M several modules or a small design choice, L needs a design first.

- **Serve Everything honours `ai.default_model` and `ai.context`**: S. An M if it should also get its own bind and port settings, which is the LAN serving item below.
- **Web-search switch and the web tools**: L. The switch itself is one declared setting; the size is the tools and what they may reach.
- **Export settings to shell actions as environment variables**: M. An L if secrets must not reach every child process.
- **Settings for LAN serving** (bind address, ports, what is shared): M. An L if it changes which services start by default.
- **Settings editor in the host CLI and TUI**: M. An L if the host should also set values at apply time, since the drive owns the values file today.
- **Model picker at launch when `ai.default_model` is `ask`**: S, unless it should remember the last pick, then an M.
- **Truncate long rows on narrow terminals**: S.
