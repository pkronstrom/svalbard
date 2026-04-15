# Portable AI Pack Layering Design

Date: 2026-04-15
Status: Approved for planning

## Summary

Svalbard should add portable local AI support through a three-layer composition model:

1. `recipes` define concrete artifacts such as specific models, runtimes, and apps.
2. `packs` define reusable capability bundles.
3. `presets` define curated end-user drive builds.

The public pack surface for v1 should stay simple. AI model packs should be grouped by host RAM tier, not by abstract "small/medium/large" labels and not by a public general-vs-coding split. Each RAM-tier pack may include both:

- one general/tool-call-oriented model
- one coding-oriented model

Higher-capacity presets must be additive. They add stronger model options without removing smaller models that remain useful on older or lower-RAM host machines.

## Goals

- Preserve compatibility across a wide range of target machines, including constrained laptops and higher-RAM Macs.
- Ship both a general/tool-use local model and a coding-oriented local model in stock presets.
- Keep the stock preset surface simple.
- Make custom preset authoring easy through pack-based composition or direct recipe references.
- Keep the design flexible so individual model choices can change over time without restructuring presets.

## Non-Goals

- Automatic host hardware detection in v1.
- A public role-specific pack matrix for v1.
- Choosing the exact final model recipes in this design document. The structure is the contract; concrete recipes can evolve separately.

## Design Decisions

### 1. Composition Hierarchy

The primary architecture is:

- `recipe -> pack -> preset`

Definitions:

- `recipe`: one concrete downloadable artifact
- `pack`: one reusable bundle of recipes
- `preset`: one curated distribution intended for end users

This should be the default composition path for built-in presets. Direct recipe references in presets remain supported, but they are mainly for custom user presets or exceptional cases.

### 2. Recipe Layout

Concrete AI artifacts should be stored by artifact type:

- `recipes/models/`
- `recipes/tools/`
- `recipes/apps/`

Examples:

- `recipes/models/gemma-4-<variant>.yaml`
- `recipes/models/qwen3-coder-<variant>.yaml`
- `recipes/tools/llama-server.yaml`
- `recipes/apps/opencode.yaml`
- `recipes/apps/crush.yaml`
- `recipes/apps/goose.yaml`

Rationale:

- model recipes stay model-specific
- runtime binaries stay tool-specific
- interactive coding/agent apps get a clearer home than `tools/`

The existing recipe loader already scans `recipes/**/*.yaml`, so this layout is compatible with current loading behavior.

### 3. Pack Layout

Built-in AI packs should live under:

- `presets/packs/ai/`

The v1 public pack surface should be:

- `ai/harnesses`
- `ai/models-8gb-ram`
- `ai/models-16gb-ram`
- `ai/models-24gb-ram`
- `ai/models-64gb-ram`

Rationale:

- `8gb-ram`, `16gb-ram`, `24gb-ram`, and `64gb-ram` communicate target host-memory classes more clearly than `low/mid/high`
- the `-ram` suffix avoids ambiguity with storage sizes or model file sizes
- a small public pack surface keeps stock presets readable and documentation simpler

### 4. Pack Semantics

#### `ai/harnesses`

`ai/harnesses` is the shared AI tooling dependency pack.

It may include:

- serving/runtime binaries such as `llama-server`
- agent/coding apps such as `opencode`, `crush`, and `goose`
- shared support artifacts such as embedding models if required by the tooling

It should not be limited to interactive frontends only. If a shared runtime or auxiliary model is required for the portable AI workflow, it belongs here.

#### `ai/models-<tier>-ram`

Each built-in RAM-tier model pack must include both:

- one general/tool-call-oriented model
- one coding-oriented model

Examples of intended roles:

- Gemma 4 variants for general/tool-call tasks
- Qwen coder variants or similar for coding tasks

These role distinctions remain real, but they are not exposed as separate public packs in v1. The role split exists at the recipe-selection level and in documentation, not as the primary pack namespace.

### 5. Preset Semantics

Built-in presets must be additive capability bundles.

Rules:

- higher-capacity presets add stronger options
- higher-capacity presets do not remove smaller, lower-RAM-compatible models
- model choice happens at use time, not during preset resolution

Initial stock mapping:

- `default-512`
  - includes `ai/harnesses`
  - includes `ai/models-8gb-ram`
- `default-1tb`
  - extends `default-512`
  - adds `ai/models-16gb-ram`
- `default-2tb`
  - extends `default-1tb`
  - adds `ai/models-24gb-ram`
  - adds `ai/models-64gb-ram`

This mapping is intentionally conservative. If later storage-budget analysis shows that `default-1tb` can comfortably carry `24gb-ram` models too, that can be changed without altering the overall architecture.

### 6. Launcher and Runtime UX

The first version should not rely on automatic hardware detection.

Instead, the launcher/menu should:

- show available models explicitly
- preserve smaller-tier options on larger presets
- avoid auto-selecting the largest model by default
- help the user choose based on host capability and task

Suggested presentation:

- label models with both role and RAM intent
- examples:
  - `general · 8 GB RAM`
  - `coding · 16 GB RAM`
  - `general · 24 GB RAM`

This keeps runtime behavior predictable on random target machines and avoids incorrect assumptions about available RAM.

### 7. Customization Contract

Built-in presets are curated examples, not the only supported shape.

Users must remain free to:

- compose custom presets from built-in packs
- reference individual recipes directly in custom presets
- build role-heavy or machine-specific bundles for their own needs

Examples of valid custom shapes:

- harnesses only
- one RAM-tier model pack only
- a coding-heavy custom preset using raw recipe references
- a Mac-focused preset with larger models only

This reinforces the intended boundary:

- stock presets optimize for simplicity
- custom presets optimize for user-specific control

### 8. Naming Principles

Pack names should describe capability classes, not exact artifact identity.

Examples:

- `ai/models-8gb-ram`
- `ai/harnesses`

Recipe ids should stay concrete and artifact-specific.

Examples:

- `gemma-4-4b-tool`
- `gemma-4-12b-tool`
- `qwen3-coder-7b`
- `qwen3-coder-14b`

This allows:

- stable pack names even if recipe choices change later
- honest representation of actual downloaded artifacts
- straightforward swapping of recipes in custom presets

## Error Handling and Operational Constraints

- If a preset references a pack, and the pack references missing recipes, preset resolution should fail clearly with the missing recipe id.
- The v1 design should avoid hidden runtime selection logic based on heuristics that users cannot inspect.
- Pack boundaries should remain coarse enough that built-in presets stay readable, but recipes must stay specific enough that failures remain diagnosable.

## Testing Implications

The implementation plan should cover at least:

- preset loader support for the new `presets/packs/ai/*` layout
- resolution tests for each new AI pack
- stock preset tests confirming additive AI pack composition
- tests confirming higher presets retain lower-tier model availability
- tests confirming custom presets can still include individual recipes directly

If launcher/menu integration changes, the implementation should also verify:

- the AI options list remains stable when multiple RAM tiers are present
- smaller models remain visible even when larger ones are also installed

## Migration and Documentation Notes

- Existing `default-*` presets should be updated incrementally to reference AI packs rather than hard-coding AI recipes directly where practical.
- Documentation should describe RAM tiers as target host-memory classes, not hard guarantees.
- Documentation should explain that stock packs are intentionally coarse, while custom presets can mix packs and individual recipes.

## Community Recommendations and Default Runtime Guidance

An April 15, 2026 `r/LocalLLaMA` community thread is useful as directional input for v1 model selection and runtime defaults, but it should be treated as anecdotal evidence rather than as a benchmark source of truth.

Directional takeaways to reflect during implementation:

- Qwen 3.5 27B and Qwen 3.5 35B-A3B are strong candidates for higher-tier coding and agentic workloads.
- Gemma 4 E4B looks like a strong smaller general/tool-use candidate, especially on 16 GB Apple Silicon class machines.
- Gemma 4 26B A4B appears promising for higher-end general use, but stability should be verified locally before it is treated as a stock default.
- Smaller 4-bit variants appear more practical than 8-bit variants on constrained machines or longer-context workloads.

These are recommendations for implementation-time evaluation, not hard commitments in this design.

### Runtime Defaults to Evaluate First

For llama.cpp-backed agentic flows, the first implementation pass should evaluate the following defaults:

- `parallel_tool_calls=true` for agent workflows
- `chat_template_kwargs.enable_thinking=false` as the safer default
- `--jinja` enabled to preserve template correctness
- conservative context use instead of running close to the maximum by default

If Gemma-based configurations show instability during local validation, the implementation should explicitly test:

- reduced `--cache-ram`, including `4096` and `0`
- lower default effective context occupancy
- 4-bit variants before moving to heavier quants

### Role Guidance for Initial Recipe Selection

The initial recipe shortlist should bias toward:

- small RAM tiers:
  - a Gemma 4 general/tool-call model
  - a compact coding-oriented model
- larger RAM tiers:
  - stronger Qwen 3.5 coding-oriented models
  - a validated general/tool-use model, which may remain Gemma if stability and tooling behavior are acceptable

The final stock recipes should be chosen only after local validation against Svalbard's actual portable workflow requirements.

## Final Contract

The approved v1 contract is:

- recipes are concrete artifacts
- packs are simple reusable bundles
- presets are curated additive products
- AI packs are RAM-tier-first
- each RAM-tier AI pack contains both a general model and a coding model
- higher presets add stronger options without dropping smaller-machine compatibility
- stock presets stay simple
- custom presets may use either packs or individual recipes directly
