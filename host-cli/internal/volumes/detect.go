// Package volumes detects mounted volumes suitable for Svalbard vaults.
//
// On macOS it scans /Volumes/, on Linux it scans /media/$USER/ and /mnt/.
// Time Machine and system volumes are excluded. Each detected volume path has
// "/svalbard" appended so it points at the expected vault root.
package volumes

import (
	"bufio"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// Volume represents a mounted filesystem that could host a Svalbard vault.
type Volume struct {
	Path    string  // e.g. "/Volumes/KINGSTON/svalbard"
	Name    string  // e.g. "KINGSTON"
	TotalGB float64
	FreeGB  float64
	Network bool
}

// networkFSTypes lists filesystem types that indicate a network mount.
var networkFSTypes = map[string]bool{
	"smbfs":       true,
	"nfs":         true,
	"nfs4":        true,
	"afpfs":       true,
	"cifs":        true,
	"9p":          true,
	"fuse.sshfs":  true,
}

// skipNames lists volume names to exclude on macOS.
var skipNames = map[string]bool{
	"Macintosh HD":       true,
	"Macintosh HD - Data": true,
}

// timeMachineMarkers are paths within a volume that indicate a Time Machine mount.
var timeMachineMarkers = []string{
	".timemachine",
	"Backups.backupdb",
	"com.apple.TimeMachine.localsnapshots",
}

// Detect returns mounted volumes suitable for Svalbard usage.
//
// On macOS it scans /Volumes/. On Linux it scans /media/$USER/ and /mnt/.
// System volumes and Time Machine mounts are excluded. Results are sorted with
// local volumes first (ascending by total size), followed by network volumes.
// Each volume path has "/svalbard" appended.
func Detect() []Volume {
	var vols []Volume

	switch runtime.GOOS {
	case "darwin":
		vols = detectDarwin()
	case "linux":
		vols = detectLinux()
	default:
		return []Volume{}
	}

	// Parse mount output for network classification.
	fsTypes := parseMountOutput()

	for i := range vols {
		basePath := strings.TrimSuffix(vols[i].Path, "/svalbard")
		if ft, ok := fsTypes[basePath]; ok {
			vols[i].Network = networkFSTypes[ft]
		}
	}

	// Ensure non-nil return even when no volumes are found.
	if vols == nil {
		vols = []Volume{}
	}

	// Sort: local first by size ascending, then network by size ascending.
	sort.Slice(vols, func(i, j int) bool {
		if vols[i].Network != vols[j].Network {
			return !vols[i].Network // local first
		}
		return vols[i].TotalGB < vols[j].TotalGB
	})

	return vols
}

// HomeSvalbardVolume returns a Volume representing ~/svalbard/ with disk space
// info for the home directory filesystem.
func HomeSvalbardVolume() Volume {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}

	svalbardPath := filepath.Join(home, "svalbard")
	free := FreeSpaceGB(home)
	total := totalSpaceGB(home)

	return Volume{
		Path:    svalbardPath,
		Name:    "~/svalbard/",
		TotalGB: total,
		FreeGB:  free,
		Network: false,
	}
}

// FreeSpaceGB returns free disk space in gigabytes at the given path. If the
// path does not exist, it walks up to the nearest existing parent directory.
func FreeSpaceGB(path string) float64 {
	path = resolveExistingParent(path)
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0
	}
	// Available blocks * block size, converted to GB.
	return float64(stat.Bavail) * float64(stat.Bsize) / (1 << 30)
}

// totalSpaceGB returns total disk space in gigabytes at the given path.
func totalSpaceGB(path string) float64 {
	path = resolveExistingParent(path)
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0
	}
	return float64(stat.Blocks) * float64(stat.Bsize) / (1 << 30)
}

// resolveExistingParent walks up the directory tree until it finds a path that
// exists. This lets callers query space for paths that don't yet exist.
func resolveExistingParent(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	for {
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return abs
		}
		abs = parent
	}
}

// detectDarwin scans /Volumes/ for external volumes on macOS.
func detectDarwin() []Volume {
	entries, err := os.ReadDir("/Volumes")
	if err != nil {
		return []Volume{}
	}

	var vols []Volume
	for _, e := range entries {
		name := e.Name()

		// Skip system volumes by name.
		if skipNames[name] {
			continue
		}

		volPath := filepath.Join("/Volumes", name)

		// Skip Time Machine mounts.
		if isTimeMachine(volPath) {
			continue
		}

		total := totalSpaceGB(volPath)
		free := FreeSpaceGB(volPath)

		vols = append(vols, Volume{
			Path:    filepath.Join(volPath, "svalbard"),
			Name:    name,
			TotalGB: total,
			FreeGB:  free,
		})
	}

	return vols
}

// detectLinux scans /media/$USER/ and /mnt/ for mounted volumes on Linux.
func detectLinux() []Volume {
	var vols []Volume

	// Scan /media/$USER/
	u, err := user.Current()
	if err == nil {
		mediaDir := filepath.Join("/media", u.Username)
		vols = append(vols, scanDir(mediaDir)...)
	}

	// Scan /mnt/
	vols = append(vols, scanDir("/mnt")...)

	return vols
}

// scanDir reads a directory and returns a Volume for each entry.
func scanDir(dir string) []Volume {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var vols []Volume
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		name := e.Name()
		volPath := filepath.Join(dir, name)

		total := totalSpaceGB(volPath)
		free := FreeSpaceGB(volPath)

		vols = append(vols, Volume{
			Path:    filepath.Join(volPath, "svalbard"),
			Name:    name,
			TotalGB: total,
			FreeGB:  free,
		})
	}

	return vols
}

// isTimeMachine checks whether a volume path contains Time Machine markers.
func isTimeMachine(volPath string) bool {
	for _, marker := range timeMachineMarkers {
		candidate := filepath.Join(volPath, marker)
		if _, err := os.Stat(candidate); err == nil {
			return true
		}
	}
	return false
}

// parseMountOutput runs `mount` and returns a map from mount point to
// filesystem type. Example mount line on macOS:
//
//	/dev/disk3s1 on /Volumes/KINGSTON (msdos, local, nodev, nosuid, noowners)
//
// Example on Linux:
//
//	//server/share on /mnt/nas type cifs (rw,relatime)
func parseMountOutput() map[string]string {
	result := make(map[string]string)

	out, err := exec.Command("mount").Output()
	if err != nil {
		return result
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()

		if runtime.GOOS == "darwin" {
			// Format: <device> on <path> (<type>, ...)
			onIdx := strings.Index(line, " on ")
			if onIdx < 0 {
				continue
			}
			rest := line[onIdx+4:]
			parenIdx := strings.Index(rest, " (")
			if parenIdx < 0 {
				continue
			}
			mountPoint := rest[:parenIdx]
			typeStr := rest[parenIdx+2:]
			if closeIdx := strings.Index(typeStr, ")"); closeIdx >= 0 {
				typeStr = typeStr[:closeIdx]
			}
			// First token before comma is the fs type.
			parts := strings.SplitN(typeStr, ",", 2)
			fsType := strings.TrimSpace(parts[0])
			result[mountPoint] = fsType
		} else {
			// Linux format: <device> on <path> type <fstype> (<options>)
			onIdx := strings.Index(line, " on ")
			if onIdx < 0 {
				continue
			}
			rest := line[onIdx+4:]
			typeIdx := strings.Index(rest, " type ")
			if typeIdx < 0 {
				continue
			}
			mountPoint := rest[:typeIdx]
			fsAndOpts := rest[typeIdx+6:]
			parts := strings.Fields(fsAndOpts)
			if len(parts) > 0 {
				result[mountPoint] = parts[0]
			}
		}
	}

	return result
}
