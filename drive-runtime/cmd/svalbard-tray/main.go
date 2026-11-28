package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gogpu/systray"
	"golang.org/x/sys/unix"
)

func init() { runtime.LockOSThread() }

func main() {
	root := flag.String("root", "", "absolute path of the drive")
	bin := flag.String("runtime", "", "absolute path to the drive runtime")
	foreground := flag.Bool("foreground", false, "run the tray in this process")
	temporary := flag.Bool("temporary-copy", false, "manage the detached tray executable")
	flag.Parse()
	d := drive{*root, *bin}
	if err := d.verifyRuntime(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var err error
	cleanup := func() {}
	if *temporary {
		exe, lookupErr := os.Executable()
		base := filepath.Base(exe)
		if lookupErr != nil || !strings.HasPrefix(base, "svalbard-tray-") || (filepath.Dir(exe) != "/tmp" && filepath.Dir(exe) != "/private/tmp") {
			fmt.Fprintln(os.Stderr, "invalid detached tray path")
			os.Exit(1)
		}
		cleanup = func() { _ = os.Remove(exe) }
	}
	if *foreground {
		err = showTray(d, cleanup)
	} else {
		err = launchTray(d)
	}
	cleanup()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func launchTray(d drive) error {
	if err := checkDesktop(); err != nil {
		return err
	}
	info, err := os.Stat(d.root)
	if err != nil {
		return err
	}
	stat := info.Sys().(*syscall.Stat_t)
	dir := "/tmp"
	identity := sha256.Sum256([]byte(fmt.Sprintf("%d/%d", stat.Dev, stat.Ino)))
	lockPath := filepath.Join(dir, fmt.Sprintf("svalbard-tray-%d-%x.lock", os.Getuid(), identity[:10]))
	fd, err := unix.Open(lockPath, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	lock := os.NewFile(uintptr(fd), lockPath)
	defer lock.Close()
	lockInfo, err := lock.Stat()
	if err != nil {
		return err
	}
	if !lockInfo.Mode().IsRegular() || lockInfo.Mode().Perm() != 0600 || lockInfo.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
		return fmt.Errorf("cannot use tray instance lock")
	}
	if err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); errors.Is(err, unix.EWOULDBLOCK) {
		return nil
	}
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	source, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer source.Close()
	copyFile, err := os.CreateTemp(dir, "svalbard-tray-*")
	if err != nil {
		return err
	}
	name := copyFile.Name()
	_, copyErr := io.Copy(copyFile, source)
	if err := errors.Join(copyErr, copyFile.Chmod(0700), copyFile.Close()); err != nil {
		_ = os.Remove(name)
		return err
	}
	// Run from a host-local copy so the tray itself never holds the vault open.
	// Keep the host path available while platform libraries identify the app.
	cmd := exec.Command(name, "-foreground", "-root", d.root, "-runtime", d.runtime, "-temporary-copy")
	cmd.Dir = "/"
	cmd.ExtraFiles = []*os.File{lock}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return cmd.Process.Release()
}

func showTray(d drive, cleanup func()) error {
	if err := checkDesktop(); err != nil {
		return err
	}
	if err := initialiseTray(); err != nil {
		return err
	}
	icon := trayIcon()
	tray := systray.New().SetIcon(icon).SetTemplateIcon(icon).SetTooltip("Svalbard · " + filepath.Base(d.root))
	menu := systray.NewMenu()
	status := menu.Add("Svalbard", nil)
	status.SetDisabled(true)
	var action menuAction
	act := func(label string, fn func() error) func() {
		return func() {
			if !action.begin(label) {
				return
			}
			status.SetLabel(label)
			go func() {
				err := fn()
				action.finish(err)
				if err != nil {
					status.SetLabel(err.Error())
					tray.ShowNotification("Svalbard", err.Error())
				}
			}()
		}
	}
	menu.Add("Open Frontpage", act("Opening frontpage…", func() error { _, err := d.command(); return err }))
	services := systray.NewMenu()
	items := map[serviceEntry]*systray.MenuItem{}
	if origin, _ := d.address(); origin != "" {
		var inv inventory
		if dashboardRequest(origin, "/api/inventory", nil, &inv) == nil {
			for _, entry := range inv.localServices() {
				items[entry] = services.AddCheckbox(entry.Label, false, act("Updating "+entry.Label+"…", func() error {
					origin, err := d.address()
					if err != nil {
						return err
					}
					if origin == "" {
						return fmt.Errorf("open the frontpage first")
					}
					var current registry
					if err := dashboardRequest(origin, "/api/registry", nil, &current); err != nil {
						return err
					}
					return setService(origin, entry.Service, active(current.Jobs[entry.Service].Status))
				}))
			}
		}
	}
	if len(items) > 0 {
		menu.AddSubmenu("Services", services)
	}
	menu.AddSeparator()
	if vol, err := mountedVolume(d.root); err == nil {
		menu.Add("Unmount Drive and Quit", act("Unmounting drive…", func() error {
			if err := d.prepareAndStop(true); err != nil {
				return err
			}
			current, err := mountedVolume(d.root)
			if err != nil || current != vol {
				return fmt.Errorf("the selected drive mount changed")
			}
			if err := unmount(vol); err != nil {
				return err
			}
			cleanup()
			os.Exit(0)
			return nil
		}))
	}
	menu.Add("Stop Dashboard and Quit", act("Stopping dashboard…", func() error {
		if err := d.prepareAndStop(false); err != nil {
			return err
		}
		cleanup()
		os.Exit(0)
		return nil
	}))
	updateMenu(d, status, items, &action)
	tray.SetMenu(menu).Show()
	go func() {
		t := time.NewTicker(3 * time.Second)
		defer t.Stop()
		for range t.C {
			updateMenu(d, status, items, &action)
		}
	}()
	defer tray.Remove()
	return tray.Run()
}

func active(status string) bool {
	return status == "starting" || status == "running" || status == "stopping"
}

type menuAction struct {
	mu    sync.Mutex
	busy  bool
	label string
}

func (a *menuAction) begin(label string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.busy {
		return false
	}
	a.busy, a.label = true, label
	return true
}

func (a *menuAction) finish(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.busy, a.label = false, ""
	if err != nil {
		a.label = err.Error()
	}
}

func (a *menuAction) message() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.label
}

func updateMenu(d drive, status *systray.MenuItem, items map[serviceEntry]*systray.MenuItem, action *menuAction) {
	origin, err := d.address()
	var current registry
	if err == nil && origin != "" {
		err = dashboardRequest(origin, "/api/registry", nil, &current)
	}
	count := 0
	for _, record := range current.Records {
		if record.Status == "running" {
			count++
		}
	}
	text := fmt.Sprintf("Svalbard · %d running", count)
	if origin == "" {
		text = "Svalbard · Dashboard stopped"
	}
	if err != nil {
		text = "Svalbard · Dashboard unavailable"
	}
	if current.Draining {
		text = "Svalbard · Preparing for removal"
	}
	if message := action.message(); message != "" {
		text = message
	}
	status.SetLabel(text)
	for entry, item := range items {
		state := current.Jobs[entry.Service].Status
		label := entry.Label
		switch state {
		case "starting":
			label += " · Starting"
		case "stopping":
			label += " · Stopping"
		case "failed":
			label += " · Failed"
		}
		item.SetLabel(label)
		item.SetChecked(active(state))
		item.SetDisabled(err != nil || origin == "" || current.Draining || state == "stopping")
	}
}

func trayIcon() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 44, 44))
	for y := 6; y < 38; y++ {
		for x := 6; x < 38; x++ {
			dx, dy := x-22, y-22
			squared := dx*dx + dy*dy
			if (squared >= 110 && squared <= 169) || (dx >= -1 && dx <= 1) || (dy >= -1 && dy <= 1) {
				img.Set(x, y, color.Black)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
