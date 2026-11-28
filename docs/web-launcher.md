# Open the atlas on a Svalbard drive

**macOS:** Copy **Svalbard.app** beside your drive’s existing `run` file, then double-click the app. If you downloaded **Svalbard-macOS.zip**, extract the app first.

**Linux:** Copy **Svalbard-Linux** beside `run` and open it. For a desktop entry, put **Svalbard.desktop** in the same folder and open that instead. If the copy lost its executable permission, run `chmod +x Svalbard-Linux Svalbard.desktop` in the vault folder. Omit the desktop filename if you copied only the shell launcher.

The click wrappers call the drive’s `./run launch`. That command starts or reuses the loopback dashboard, waits for readiness, and opens the default browser. The atlas displays this drive’s resources. Reader and inference services start on demand. Closing a tab leaves the dashboard available to reopen.

## Prepare an existing drive

Drives provisioned with the **web-launcher** recipe already have the atlas assets, runtime and matching wrappers. If the wrapper reports missing assets or an older runtime, use the Svalbard host CLI with the existing vault path:

```sh
svalbard --vault /path/to/vault add web-launcher
svalbard --vault /path/to/vault plan
svalbard --vault /path/to/vault apply
```

If a managed launcher has an update, run `svalbard --vault /path/to/vault update web-launcher`. Prior drives with the foreground dashboard can also use `./run web`. The optional recipe installs the atlas, fonts, viewer scripts, static **Svalbard Files.html** and wrappers. The download contains only the graphical entry points. Copying one onto a prepared drive leaves its manifest, settings and personal files intact.

Standard default and Finland presets select the recipe. Clear it in the wizard or pass `--remove web-launcher` during vault creation for terminal-only use. Existing saved selections remain authoritative.

## System tray
A drive prepared with `make build` and the web-launcher recipe includes a small tray helper for the chosen Mac or Linux architectures. Opening the graphical wrapper opens the frontpage and leaves a Svalbard icon in the menu bar or system tray. The click wrappers also work on older drives and Linux desktops without a tray.

- **Open Frontpage** opens the active drive in the browser.
- **Services** lists installed readers and local apps. A checked item is active. Click to serve or stop it.
- **Quit Tray** closes the menu-bar app. The dashboard and its services keep running.
- **Stop Svalbard and Quit** stops the dashboard and its services, then closes the tray. The drive stays mounted.
- **Unmount Drive and Quit** appears for a recognized mounted disk. It checks for open handles, stops the dashboard and asks the OS to unmount that volume. On Linux it uses `findmnt` and `udisksctl`. The menu stays open if the operation is blocked.

Set `SVALBARD_NO_TRAY=1` before running a wrapper to open the browser by itself. The tray runs from a temporary host-local copy so it can unmount its own drive. A fresh `make build` embeds the helper in the host CLI for installing or updating the recipe. The wrapper download stays small and uses the tray helper already installed on the drive.

## Close or remove the launcher

In the Service Register, choose **Prepare for removal**. Read the reported processes and handles, close any remaining users of the drive, then choose **Close dashboard**. Use the operating system’s Eject command before unplugging the drive.

From the vault folder, these commands provide terminal access.

```sh
./run                       # terminal menu
./run web --no-open         # foreground dashboard
./run launch --no-open      # start or reuse a detached dashboard
./run launch status         # show its local address
./run launch stop           # stop and wait for the services
```

Delete a manually copied wrapper to remove that graphical entry point. For a recipe-managed installation, stop the dashboard, run `svalbard --vault /path/to/vault remove web-launcher`, then apply. The recipe tracks each installed file. Removal preserves personal HTML, modified replacements, shared runtimes and the terminal menu.

## Platform notes

The macOS wrapper selects the drive’s available native binary for Apple silicon or Intel, including a Finder shell running under Rosetta. The bundle is unsigned and unnotarized. A quarantined or translocated app can lose access to the vault around it. If host policy permits, use a trusted local copy of the complete vault.

Linux uses the drive’s x86_64 or arm64 executable, POSIX shell and `xdg-open`. File managers and removable-media trust policies vary. A headless host can use `./run launch --no-open`. The static **Svalbard Files.html** page works when native execution is unavailable. Newly generated `run` scripts use a POSIX shell. On a Bash-free host with an older drive that shipped a Bash dispatcher, **Svalbard-Linux** can call the same drive runtime directly. Windows is outside this package.

## Package the click wrappers

From the source checkout:

```sh
scripts/package-web-launcher.sh dist/launcher
(cd dist/launcher && shasum -a 256 -c SHA256SUMS)
```

The output contains the recipe’s **Svalbard.app**, a ZIP carrying just that app, **Svalbard-Linux**, **Svalbard.desktop** and their checksums. The Mac bundle contains its launch script, Info.plist and survey icon.
