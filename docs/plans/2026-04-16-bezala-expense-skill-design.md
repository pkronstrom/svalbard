# Bezala Expense Skill Design

**Date:** 2026-04-16
**Location:** `hawk-hooks/builtins-private/skills/bezala/`
**Synced via:** `hawk sync`

## Purpose

Automate Reaktor expense reports (kululaskut) in Bezala via browser MCP. The skill navigates to Bezala, checks login state, scans for receipts, prefills form fields using Reaktor's expense policy, and lets the user review before saving/submitting.

## Files

```
bezala/
├── SKILL.md              # Main workflow (~entry point)
├── kululaskuohje.md       # Full Reaktor expense policy (Finnish + English)
└── bezala-fields.md       # Project codes + purchase types lookup tables
```

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Location | builtins-private/skills/ | Personal, not shipped with hawk-hooks |
| Kululaskuohje embedding | Full embed | Optimize later if token cost is a problem |
| Browser MCP | Tool-agnostic, detect at runtime | Many browser MCPs exist; skill describes intent, agent picks tool |
| Memory | Auto-memory system | Saves project codes and billing prefs after first use |
| Downloads scan | ~/Downloads, last 48h | If no recent files, ask user for path |
| Default action | New expense (if no args) | 95% use case; args override |
| Submit behavior | NEVER without explicit user OK | Show screenshot, user decides draft/send |

## Workflow

```
/bezala [optional instructions]
  → detect browser MCP (ToolSearch for navigate/fill/click/screenshot)
  → navigate to https://app.bezala.com/
  → screenshot + check login → ask user to login if needed
  → route: args → follow instructions; no args → new expense
  → scan ~/Downloads for recent PDFs/images (48h)
  → show candidates, ask user to confirm receipt
  → check auto-memory for bezala-preferences
  → ask user: selite, project, purchase type, billable, payment method
  → validate against kululaskuohje (taxi needs from-to, restaurant needs attendees, etc.)
  → fill form in Bezala, attach receipt
  → screenshot for review
  → user confirms → save draft or submit
  → offer to save new project code to memory
```

## Browser MCP Abstraction

The skill describes **intent**, not specific tools:
- "Navigate to URL"
- "Take screenshot"
- "Fill form field X with value Y"
- "Click button Z"
- "Upload file"

The agent uses ToolSearch to find whatever browser MCP is available and maps intent to tools. Preference order if multiple available: chrome-devtools-mcp > claude-in-chrome > any other.

## Memory Schema

```markdown
# ~/.claude/projects/.../memory/bezala_preferences.md
---
name: bezala-preferences
description: Saved Bezala project codes and expense defaults
type: user
---

## Frequently used projects
- **AI_tools** — AI subscriptions (Claude, Cursor, etc.)

## Defaults
- Payment method: own card
- Billable: No (unless customer project)
```

Grows organically from usage — no upfront config.
