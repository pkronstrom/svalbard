//go:build darwin || linux

package volumes

import (
	"os"
	"testing"
)

func TestDetectVolumesReturnsSlice(t *testing.T) {
	vols := Detect()
	if vols == nil {
		t.Fatal("Detect() returned nil, expected non-nil slice")
	}

	// Verify sort invariant: local volumes before network volumes.
	seenNetwork := false
	for _, v := range vols {
		if v.Network {
			seenNetwork = true
		} else if seenNetwork {
			t.Errorf("local volume %q appeared after a network volume", v.Name)
		}
	}

	// Verify all paths end with /svalbard.
	for _, v := range vols {
		if v.Path == "" {
			t.Error("volume has empty Path")
		}
		if len(v.Path) < 9 || v.Path[len(v.Path)-9:] != "/svalbard" {
			t.Errorf("volume path %q does not end with /svalbard", v.Path)
		}
	}
}

func TestHomeSvalbardOption(t *testing.T) {
	vol := HomeSvalbardVolume()

	if vol.Path == "" {
		t.Error("HomeSvalbardVolume() returned empty path")
	}
	if vol.Name != "~/svalbard/" {
		t.Errorf("expected name %q, got %q", "~/svalbard/", vol.Name)
	}
	if vol.FreeGB <= 0 {
		t.Errorf("expected FreeGB > 0, got %f", vol.FreeGB)
	}
	if vol.TotalGB <= 0 {
		t.Errorf("expected TotalGB > 0, got %f", vol.TotalGB)
	}
	if vol.Network {
		t.Error("HomeSvalbardVolume() should not be a network volume")
	}
}

func TestFreeSpaceGB(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot determine home dir: %v", err)
	}

	gb := FreeSpaceGB(home)
	if gb <= 0 {
		t.Errorf("expected FreeSpaceGB(%q) > 0, got %f", home, gb)
	}
}

func TestFreeSpaceGBNonexistentPath(t *testing.T) {
	// Should walk up to an existing parent and return a positive value.
	gb := FreeSpaceGB("/tmp/nonexistent/deeply/nested/path")
	if gb <= 0 {
		t.Errorf("expected FreeSpaceGB for nonexistent path > 0, got %f", gb)
	}
}

func TestResolveExistingParent(t *testing.T) {
	resolved := resolveExistingParent("/tmp/does/not/exist")
	if _, err := os.Stat(resolved); err != nil {
		t.Errorf("resolveExistingParent returned non-existent path: %q", resolved)
	}
}

func TestNetworkFSTypes(t *testing.T) {
	expected := []string{"smbfs", "nfs", "nfs4", "afpfs", "cifs", "9p", "fuse.sshfs"}
	for _, fs := range expected {
		if !networkFSTypes[fs] {
			t.Errorf("expected %q to be in networkFSTypes", fs)
		}
	}
}
