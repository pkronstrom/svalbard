package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

func initialiseTray() error { return nil }

func checkDesktop() error {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return fmt.Errorf("no desktop D-Bus session")
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	var owner bool
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, "org.kde.StatusNotifierWatcher").Store(&owner)
	if err != nil {
		return err
	}
	if !owner {
		return fmt.Errorf("the desktop has no system tray")
	}
	return nil
}

func mountedVolume(root string) (volume, error) {
	if _, err := exec.LookPath("udisksctl"); err != nil {
		return volume{}, err
	}
	out, err := systemCommand("findmnt", "--json", "--output", "SOURCE,TARGET", "--target", root)
	if err != nil {
		return volume{}, err
	}
	var info struct {
		Filesystems []struct{ Source, Target string }
	}
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		return volume{}, err
	}
	if len(info.Filesystems) != 1 {
		return volume{}, fmt.Errorf("drive mount is ambiguous")
	}
	fs := info.Filesystems[0]
	if !(strings.HasPrefix(fs.Target, "/media/") || strings.HasPrefix(fs.Target, "/run/media/") || strings.HasPrefix(fs.Target, "/mnt/")) ||
		!strings.HasPrefix(fs.Source, "/dev/") {
		return volume{}, fmt.Errorf("the vault is not on a removable disk mount")
	}
	return volume{mount: fs.Target, device: fs.Source}, nil
}

func unmount(v volume) error {
	_, err := systemCommand("udisksctl", "unmount", "--block-device", v.device, "--no-user-interaction")
	return err
}
