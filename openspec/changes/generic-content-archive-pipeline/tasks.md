## 1. Characterize the existing engine

- [ ] 1.1 Inventory Python stages, strategies, site configs, enrichers, state, and outputs
- [ ] 1.2 Build bounded old-builder golden fixtures and manifest measurements

## 2. Implement generic blocks

- [ ] 2.1 Implement source link extraction and JSONL manifest
- [ ] 2.2 Implement link verification/classification
- [ ] 2.3 Implement generic fetch, metadata, asset, HTML transform, render, and verify blocks
- [ ] 2.4 Implement collect and ZIM package blocks

## 3. Implement cached map

- [ ] 3.1 Add manifest-driven map block with stable per-project directories
- [ ] 3.2 Reuse content-addressed outputs per project and sub-stage
- [ ] 3.3 Prove template-only invalidation does not re-fetch

## 4. Express site behavior

- [ ] 4.1 Define typed site rules and migrate generic sites
- [ ] 4.2 Implement Printables, Thingiverse, GitHub, and Instructables enrichers
- [ ] 4.3 Add browser fallback through tools-browser

## 5. Prove MIY and cut over

- [ ] 5.1 Compare manifests, metadata, links, assets, pages, and archive inventory
- [ ] 5.2 Prove interruption and cross-host resume
- [ ] 5.3 Switch MIY recipe only after parity and canonical verification
