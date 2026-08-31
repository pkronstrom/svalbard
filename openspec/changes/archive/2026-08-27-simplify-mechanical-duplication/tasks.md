reviewed: true

## 1. Remove unused TUI flexibility

- [x] 1.1 Remove palette verb/freeform fields, matching, messages, and obsolete tests
- [x] 1.2 Preserve palette ID, label, alias, case, empty, and no-match coverage
- [x] 1.3 Remove disabled navigation state, branches, and disabled-only tests
- [x] 1.4 Preserve navigation boundary, rendering, numbering, and selection coverage

## 2. Replace mechanical duplication

- [x] 2.1 Add image conversion tests for RGBA identity, non-zero bounds, and pixels
- [x] 2.2 Replace the manual pixel loop with `image/draw.Draw`
- [x] 2.3 Add or confirm builder dispatch-precedence coverage
- [x] 2.4 Replace the one-entry special builder map with a direct `python-venv` condition

## 3. Verify modules

- [x] 3.1 Run full `tui`, `build-tools`, and `host-cli` tests
