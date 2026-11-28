# Open a Svalbard vault

Extract the complete Svalbard folder to a location where you can run trusted applications. On macOS, double-click **Svalbard.app**. On Linux, open **Svalbard.desktop** in a file manager that supports desktop entries. Both open the atlas in your default browser. You can close the tab and reopen the launcher to return to the same running vault. Resource readers and inference start when you use them.

For directly readable files, open **Svalbard Files.html**. It links to the vault directory and explains where to find installed books and standalone HTML. Search, archives, server-backed viewers and local AI use the dashboard. The standalone launcher package starts as an empty vault. Add the resources you want with the Svalbard host CLI.

On a restricted desktop, use the launcher's trusted local folder copy or open the static files. GNOME and KDE distributions vary in how they handle desktop files on removable media. Linux graphical launch requires the host’s browser, POSIX shell and `xdg-open`. The executable wrapper `Svalbard-Linux` selects the installed native Linux binary. An existing trusted terminal on a Linux host can run `./Svalbard-Linux` directly. The portable `./run` menu uses Bash.

The macOS bundle dispatches to the native drive binary. Keep the app beside the vault’s `run` file. The bundle is unsigned and unnotarized. A quarantined or translocated app can lose access to the rest of the vault. Use a trusted local copy according to your administrator’s policy. The release targets Apple silicon and Intel macOS, and arm64 and x86_64 Linux. Windows is outside this package.

## Close the drive

In the Service Register, choose **Prepare for removal**. Read the reported processes and open handles, close any remaining users of the drive, then choose **Close dashboard**. Use the operating system’s Eject command before unplugging the drive.

From the vault’s folder, these commands provide the same controls.

```sh
./run                         # terminal menu
./run web --no-open           # foreground dashboard
./run launch --no-open        # start or reuse a detached dashboard
./run launch status           # show its local address
./run launch stop             # stop it and wait for its services
```

Browser launch uses a private, short-lived host-local file to unlock the terminal in that browser tab. A plain dashboard URL can be used to browse the catalogue. Relaunch to open a trusted terminal tab. Failed browser opens show the address and private log path. The log and lock directory is under `/tmp/svalbard-desktop-<uid>/` on the host. Reading a vault uses its existing files. Content-editing features need a writable vault.

## Choose the recipe

Standard default and Finland presets select **web-launcher**. Clear it in the creation wizard’s item selection to make a terminal-only vault. Saved selections from earlier versions retain their contents. Owners of an existing vault can add the launcher explicitly. Stop a running dashboard before replacing or removing its files.

```sh
# Create a standard vault with terminal-only access
svalbard init /path/to/vault --preset default-32 --remove web-launcher

# Add the launcher to an existing vault
svalbard --vault /path/to/vault add web-launcher
svalbard --vault /path/to/vault plan
svalbard --vault /path/to/vault apply

# Review or install an updated launcher
svalbard --vault /path/to/vault update
svalbard --vault /path/to/vault update web-launcher

# Remove the launcher, then apply the saved selection
svalbard --vault /path/to/vault remove web-launcher
svalbard --vault /path/to/vault apply
```

The recipe owns individual wrappers, the static file and the atlas archive. The manifest records their paths, SHA-256 checksums and acquisition identity. Removal preserves personal HTML, extra bundle files, modified replacements and the shared drive runtimes. Reapplying a saved terminal-only vault keeps its selection. New launcher assets conflict safely with unrelated files at the same names.

## Build a portable copy

From the repository, run the following commands. Packaging uses the freshly built CLI to install only this recipe, then compresses the resulting vault. It requires Python 3.11 or later on the build host. The extracted vault includes the compiled drive runtime.

```sh
make build
scripts/package-web-launcher.sh dist/launcher macos-arm64
scripts/package-web-launcher.sh dist/launcher all
(cd dist/launcher && shasum -a 256 -c Svalbard-Launcher-macos-arm64.zip.sha256)
```
