# Svalbard skills

Agent skills that ship inside host-cli. Apply writes `<recipe>/<skill>/` from
here into `skills/<recipe>/<skill>/` on a vault, but only the skills whose
`requires:` are installed there, and the drive's search indexes them for
`skill_search` and `skill_read`. This README is not written to vaults.

They are read by small local models with no network. Write for that reader.

## Layout

```text
<recipe>/<skill>/
  SKILL.md            the procedure (required)
  references/*.md     details the procedure points to, read on demand
  scripts/            what the procedure runs (made executable on the vault)
  templates/          files the agent copies and edits
  examples/check.sh   the offline run check (required)
```

The recipe folder is a catalog recipe of type `skills` (`skills-core`). The
skill folder's name is the skill's name.

## SKILL.md

- Frontmatter: `name` (the folder's name), `description` (one or two
  sentences saying what the skill makes and when to use it; search matches
  it), `requires:` (may be `[]`). Each requirement is a recipe id, or
  `type:<recipe type>` for any installed recipe of that type
  (`type:pmtiles`).
- The first section is `## Steps`, a numbered procedure. Background,
  options and troubleshooting come after it or in `references/`.
- 300 to 800 tokens. The model has the harness prompt, the tools, the drive
  context and the page it is writing in the same window.
- Give exact commands. Name the skill's own files by the path `skill_read`
  returns as `folder`, quoted: write `"<folder>"/scripts/x.py` and say to
  keep the quotes, since a drive's path can hold spaces (a second stick
  mounts as `/Volumes/SVALBARD 1`). Never write "from this skill's folder":
  a model told that copies its work into the skill. Work goes in the
  user's working folder.
- Python scripts run with `py-docs <script> <args>`: the drive's Python with
  the document libraries, on PATH in every harness shell.
- The last step is "Check your result", running `examples/check.sh` on what
  the agent made.

## Offline rules

`make verify` fails a skill (`skills_lint.go`) that:

- requires a recipe the catalog lacks, or a type no recipe has;
- tells the agent to run a package manager, a download or a clone, or loads
  a script, stylesheet, font, image or module from a network address, on any
  line of its markdown, HTML, Python, shell or JS, except inside a fenced
  block whose info string is `not-for-this-stick`.

Plain links in prose are allowed for attribution.

## examples/check.sh

`sh examples/check.sh [artifact] [options]`, with `DRIVE` set to the vault
and `<vault>/bin/<platform>` and the folder of `svalbard-drive` first on
PATH:

- with an argument it validates that file (what a model produced), and
  writes nothing in the current folder;
- without one it is a self-test: it produces the reference output by the
  skill's own steps and validates it, in its own `mktemp -d` folder, never
  the current one. A model that drops the argument must not have its work
  overwritten, so the self-test's file names also differ from the ones the
  steps use (`example-*`).

It exits 0 only when validation passes and prints the reason on its last
line when it fails. When the vault has nothing to test (no search index,
for example) it exits 77 and says why: the run check reports SKIP, not
PASS. It finds the skill's files from its own path
(`$(dirname "$0")/..`). `scripts/skill-run-check.sh <vault> [skill...]` runs
the checks with the network denied.

## Licences

Everything here is written for Svalbard. The document skills (spreadsheet,
PDF, Word, slides) are written from the use case alone: do not read or adapt
Anthropic's `docx`, `pdf`, `pptx` or `xlsx` skills, whose licence forbids
derivative works. Apache-2.0 skills may be adapted with attribution.
