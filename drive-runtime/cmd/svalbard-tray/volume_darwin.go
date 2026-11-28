package main

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"

	"golang.org/x/sys/unix"
)

func checkDesktop() error { return nil }

// A tray launched as a standalone executable must initialise AppKit
// before gogpu/systray creates its first status item.
func initialiseTray() error {
	appKit, err := ffi.LoadLibrary("/System/Library/Frameworks/AppKit.framework/AppKit")
	if err != nil {
		return err
	}
	load, err := ffi.GetSymbol(appKit, "NSApplicationLoad")
	if err != nil {
		return err
	}
	var call types.CallInterface
	if err := ffi.PrepareCallInterface(&call, types.DefaultCall, types.UInt8TypeDescriptor, nil); err != nil {
		return err
	}
	var ok uintptr
	if _, err := ffi.CallFunction(&call, load, unsafe.Pointer(&ok), nil); err != nil {
		return err
	}
	if ok == 0 {
		return fmt.Errorf("could not initialise AppKit")
	}
	return nil
}

func mountedVolume(root string) (volume, error) {
	var fs unix.Statfs_t
	if err := unix.Statfs(root, &fs); err != nil {
		return volume{}, err
	}
	v := volume{mount: unix.ByteSliceToString(fs.Mntonname[:]), device: unix.ByteSliceToString(fs.Mntfromname[:])}
	if !strings.HasPrefix(v.mount, "/Volumes/") || !strings.HasPrefix(v.device, "/dev/disk") {
		return volume{}, fmt.Errorf("the vault is not on a disk mounted under /Volumes")
	}
	return v, nil
}

func unmount(v volume) error {
	_, err := systemCommand("/usr/sbin/diskutil", "unmount", v.device)
	return err
}
