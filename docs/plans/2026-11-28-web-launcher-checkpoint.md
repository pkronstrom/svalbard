# Optional web launcher checkpoint

The base dashboard on `7d1e96d` used the shared Go runtime to bind an ephemeral loopback port, render its embedded atlas and manage resource processes. Its foreground terminal owned the session. The default optional recipe now owns the identical atlas assets and OS-specific wrappers. Fresh standard vaults select it, and saved `desired.items` continues to govern existing vaults. The desktop entry launches the same server in a detached process. Foreground and graphical launches coordinate per physical directory.

```text
Finder app / Linux desktop entry / ./run web
                   │
           native drive runtime
                   │
         private per-vault socket
                   │
          loopback atlas server ── browser
                   │
           service register ───── resource viewers
```

The [launcher guide](../web-launcher.md) covers provisioning an existing drive and copying the small click wrappers. The distribution contains the recipe’s Svalbard.app, a ZIP of that app, the Linux shell launcher, the portable desktop entry and checksums. The app holds three files: its script, Info.plist and survey icon. Each wrapper delegates to the existing drive runtime.

## Verification

The acceptance run used tiny temporary vaults, including two independently created directory roots, a local read-only test container filesystem and a disposable exFAT disk image. The existing small reader fixture exercised installed Wikiciv and a 240-page PDF. Standard preset resolution was checked without downloading those presets. The platform coverage below distinguishes physical interfaces from native binaries, containers and emulation.

| ID | Result | Evidence |
|---|---|---|
| L01 | PASS | Base dashboard suite and host CLI/TUI checks ran before the launcher work. Final verification ran the complete repository suite. Runtime creation, root lookup, native dispatch, menu generation, recipe apply/remove, loopback binding and the service register were inspected. |
| L02 | PASS | All 16 standard default and Finland presets resolve web-launcher. Real default-32 init wrote it to desired state. A one-recipe vault used the same apply path and installed the complete atlas. |
| L03 | PASS | Real init with web-launcher deselected created a vault with no atlas archive, bundle or web menu row. The actual TUI rendered in a pseudoterminal. |
| L04 | PASS | Repeated apply, update-all, a saved pre-recipe desire with standard preset provenance, explicit add, update and repeated re-add all retained the intended selection. Changing the OS selection reconciled wrappers. |
| L05 | PASS | Removing the recipe retained user HTML, extra app-bundle files and the native drive runtime. Unit tests also preserved modified replacements and rejected collisions and symlink paths. |
| L06 | PASS | Both entry points use the same `runDashboard`, registry and loopback listener. No resource service was running immediately after launch. |
| L07 | PASS | Changed working directory, read-only mounts and space/Unicode paths passed. A real macOS LaunchServices test exposed Finder’s decomposed path. The final directory-identity lock let Terminal find and stop that same session. |
| L08 | PASS | Eight parallel Mac launches reused one address. Separate roots got separate servers. Foreground and detached launches reused the same instance. Linux repeated the concurrent and isolation checks offline. |
| L09 | PASS | The private control endpoint opened after the atlas returned HTTP 200. The held-lock regression exercised the bounded timeout without opening the browser. Opener failure reported the usable address and log. |
| L10 | PASS | An unrelated live listener survived launch and stop. The dashboard bound an OS-assigned port directly. Commands use the private socket and never act on a PID or a familiar TCP port. |
| L11 | BLOCKED in part | macOS arm64 LaunchServices opened the original managed ARM-only test build and the real default Firefox redeemed its handoff. An explicit Rosetta wrapper run repeated that result with only the ARM Mach-O present. The Intel binary served the atlas under Rosetta. Physical Intel Finder, manually clicking Finder, distribution quarantine, App Translocation and notarization were not verified. |
| L12 | BLOCKED in part | GIO parsed and dispatched the actual desktop entry from a read-only Unicode path in a bash-free arm64 Linux image. Debian arm64 and emulated Linux x86_64 loaded the dashboard. The Linux opener was a test stand-in. GNOME/KDE desktop trust prompts and a live Linux browser remain unverified. |
| L13 | PASS on Linux | An unprivileged container with networking disabled and a read-only vault served the atlas locally. Runtime browsing required no toolchain. On Mac the browser harness blocked external requests. Clean offline physical-host installation remains unverified. |
| L14 | PASS on tested paths | Missing recipe, per-file collisions, incomplete assets, expired authentication, unsupported execution and failed browser opening have actionable paths. The native macOS missing-root alert, browser error and dashboard were captured in screenshots. OS refusal to execute the wrapper itself is handled by the documented file and local-copy fallback. |
| L15 | PASS | The previous atlas assets were moved intact. The Service Register gained the final dashboard-close action. Browser checks loaded the compiled styles and local fonts, actual Wikiciv and the PDF viewer with zero external requests. |
| L16 | PASS | Opening Svalbard Files.html directly from disk rendered with no server and linked to the local vault directory. It identifies the directly readable and service-backed material. |
| L17 | PASS | Closing and reopening a tab reused the server with both test readers active. Initially the service register was empty. Stopping one vault preserved the other. |
| L18 | PASS | Prepare waited for services, settings and search, reported handle uncertainty or blockers, and exposed Close dashboard after preparing. HTTP closed and the owner lock released. The disposable exFAT image detached after shutdown. Physical-stick ejection remains unverified. |
| L19 | PASS | Private owner directory, per-vault lock and Unix socket, verified loopback origin, hostile Host/Origin rejection, short-lived one-use handoffs, owner/path checks and stop isolation were exercised. A graphical opener receives a local file path. The terminal credential stays out of its URL and logs. |
| L20 | PASS | The release build cross-compiled all four existing targets. Manifest file hashes, package acquisition identity and both ZIP sidecars were produced. The current transport ZIP contains only the three Mac wrapper files and matches the recipe bytes. Linux ships the matching shell wrapper and desktop entry. |
| L21 | PASS | The real `./run` TUI rendered with the recipe present and absent. Native inspect, foreground web, headless launch, status, stop and the full existing tests passed. The existing terminal entry is named `run`. |

The local evidence directories for this run were `.worktrees/launcher-evidence/` and `.worktrees/launcher-linux-evidence/`. They contain the CLI, GUI and container harnesses, platform logs and dashboard, static fallback and expired-launch screenshots. Checks that depend on host trust policy or physical hardware remain qualified above.

## Click-wrapper distribution

The shipped Mac ZIP contains exactly the recipe’s Info.plist, executable shell script and survey icon. The direct app and extracted app have identical contents. The Linux shell and desktop files match the recipe byte for byte. Package checksums cover the transport ZIP and both Linux files.

The wrapper release was tested from a temporary copy of an existing prepared drive’s configuration, UI archive and shared runtime. LaunchServices and a Rosetta shell selected the drive’s installed Apple silicon runtime and opened its local atlas in Firefox. Both Linux architectures dispatched the browser handoff with disabled networking, a read-only vault mount and a non-root account. The missing-runtime case reported how to prepare the existing drive. All starting file paths and hashes survived the test, including manifest, settings and personal HTML. All test dashboards stopped and the temporary drive copy was removed.

Local verification logs and the repeatable smoke harness are in `.worktrees/launcher-wrapper-evidence/`.
