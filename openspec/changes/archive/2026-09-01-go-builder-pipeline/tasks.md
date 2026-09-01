reviewed: true

## 1. Inventory and test language

- [x] 1.1 Record every current build recipe, parsed field, path, and status
- [x] 1.2 Add outcome-focused builder scenarios
- [x] 1.3 Add scenarios for pipeline artifacts, app assets, dependencies, and unsupported families

## 2. Preserve typed build configuration

- [x] 2.1 Move build schema into focused catalog types
- [x] 2.2 Preserve assets, tables, layers, requirements, network, and resource estimates
- [x] 2.3 Add canonical/embedded catalog coverage
- [x] 2.4 Surface declared build resources in plan summaries and TUI

## 3. Compile and execute procedures

- [x] 3.1 Define stable typed procedures and JSON Lines-compatible events
- [x] 3.2 Compile existing linear steps without changing order
- [x] 3.3 Add persistent vault staging and fingerprints
- [x] 3.4 Reuse valid completed procedures on retry

## 4. Order recipe dependencies

- [x] 4.1 Schedule dependency readiness levels
- [x] 4.2 Add Python runtime/tool dependency declarations
- [x] 4.3 Cover realized, missing, and cyclic dependencies
- [x] 4.4 Stop dependent levels after failures

## 5. Implement foundation families

- [x] 5.1 Implement app-bundle source archives and asset lists in Go
- [x] 5.2 Reject unsupported families without checkout-relative script fallback
- [x] 5.3 Keep portable Python output while running uv in the base appliance

## 6. Build isolated tools appliances

- [x] 6.1 Add base and browser Dockerfile targets
- [x] 6.2 Pin uv and zimit inputs
- [x] 6.3 Publish semver/SHA-tagged base and browser targets
- [x] 6.4 Map typed browser capabilities without recipe-selected images
- [x] 6.5 Pass Dockerfile static checks for both targets

## 7. Verification

- [x] 7.1 Run `make verify`
- [x] 7.2 Run focused procedure, dependency, plan-resource, and image-routing tests
- [x] 7.3 Validate all OpenSpec changes
