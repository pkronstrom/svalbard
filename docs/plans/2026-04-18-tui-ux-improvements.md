# TUI UX Improvements

## Changes

### 1. Responsive 2-pane layout (shell.go)
- Remove `CompactRight` field from `ShellLayout`
- Narrow mode (`< 80 chars`) stacks Left + Right vertically at full width
- Add subtle `───` separator between panes in narrow mode
- Remove all `CompactRight` assignments from screens

### 2. New Vault path picker (wizard/pathpicker.go)
- Remove `filepicker` dependency entirely
- Replace with inline text input pre-filled with `cwd + "/svalbard-vault"`
- Volume list remains as quick-select options that populate the text input
- Validate parent directory exists before accepting path
- Show inline error if parent doesn't exist

### 3. Status vault size (dashboard/context.go)
- Add `DiskUsedGB` as "Vault size" field in status detail pane

### 4. Empty Plan (plan/model.go)
- Show "Everything in sync."
- Footer: `Enter: browse | q/Esc: back`
- Enter navigates to Browse screen
- Remove redundant "No pending downloads or removals." line

### 5. KB legends (all screens)
- Audit every footer hint for accuracy
- Wizard: stage-appropriate hints instead of generic
- Plan/Index: consistent `q/Esc: back`
- Import: `q: back (when empty)` note

### 6. Import screen (importscreen/model.go)
- Expand description: supported sources (local files, HTTP/HTTPS URLs, YouTube links)
- Note Docker dependency for content that needs processing
- `q` already only exits when input is empty — no change needed
