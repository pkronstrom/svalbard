# Platform Picker — Wizard Stage

## Context

The vault manifest already has `desired.options.host_platforms` but the wizard never sets it. Currently `apply.go` downloads binaries only for the host platform. If you build a vault on Mac but want it to work on Linux, those binaries are missing. 13 recipes have platform-specific binaries across 4 platforms.

## Design

New wizard stage between Vault Path and Choose Preset:

```
 Target Platforms                              1 selected
─────────────────────────────────────────────────────────
  h  This host only
  a  All platforms
  ─────────────────────────────────────────
  [✓] macos-arm64          (this host)
  [ ] macos-x86_64
  [ ] linux-arm64
  [ ] linux-x86_64

  Binaries: ~0.5 GB for 1 platform
─────────────────────────────────────────────────────────
 space toggle · h this host · a all · enter continue
```

- `h` checks only the current host platform
- `a` checks all 4 platforms
- `space` toggles individual checkboxes
- `enter` continues to preset selection
- `esc` goes back to path picker
- "(this host)" label on the detected platform
- Footer shows estimated binary size based on selection count

## Wizard Stage Order

```
1  Vault Path
2  Target Platforms    ← new
3  Choose Preset
4  Pack Picker
5  Review
```

## Data Flow

- Output: `[]string` of selected platform keys (e.g. `["macos-arm64", "linux-arm64"]`)
- Stored in wizard's accumulated state, passed through to WizardResult
- Written to `manifest.desired.options.host_platforms` during init
- Apply reads `host_platforms` and downloads binaries for each selected platform

## Changes Required

1. New `platformpicker.go` in `host-tui/internal/wizard/`
2. Add `platformPickerModel` as new wizard stage
3. Update wizard `model.go` — new stage constant, transition logic
4. Update `WizardResult` to include `HostPlatforms []string`
5. Detect current host platform for "(this host)" label
6. Add platform list to `WizardConfig` (or derive from available recipe platforms)
7. Update `apply.go` to respect `manifest.Desired.Options.HostPlatforms`
8. Update `commands/init.go` to write `host_platforms` to manifest
9. Update review stage to show selected platforms
