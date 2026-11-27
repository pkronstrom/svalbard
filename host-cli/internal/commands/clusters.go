package commands

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/tui"
)

// LargeClusterBytes is the allocation unit from which init warns. USB sticks
// come formatted as exFAT with 128 KB clusters or more, where every file and
// directory takes at least one cluster: a tree of small files then takes
// several times its size (Open WebUI: 3.20 GB per platform at 128 KB, about
// 0.6 GB at 4 KB).
const LargeClusterBytes = 32 << 10

const smallClusterBytes = 4 << 10

// ClusterSize measures the allocation unit of the filesystem that holds dir,
// or the nearest existing parent of dir, by writing a one-byte file there and
// reading its allocated size; statfs reports 512 bytes on macOS's exFAT.
func ClusterSize(dir string) (int64, error) {
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return 0, fmt.Errorf("no directory at %s", dir)
		}
		dir = parent
	}
	f, err := os.CreateTemp(dir, ".svalbard-cluster-*")
	if err != nil {
		// A disk this user cannot write: the files already there.
		return clusterFromFiles(dir)
	}
	defer os.Remove(f.Name())
	_, err = f.Write([]byte{0})
	if err == nil {
		err = f.Sync()
	}
	// macOS's exFAT reports no blocks for a file that is still open.
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(f.Name())
	if err != nil {
		return 0, err
	}
	return allocatedBytes(info), nil
}

// clusterFromFiles reads the cluster size from the files directly in dir:
// the greatest common divisor of what they take on disk. That is the cluster
// size, or a multiple of it, so it is trusted only when it is a power of two
// and some file holds at most half of it: such a file takes exactly one
// cluster, which a multiple could not divide. A directory of large files
// alone gives no answer rather than a wrong one.
func clusterFromFiles(dir string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	var divisor int64
	var sizes []int64
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if allocated := allocatedBytes(info); allocated > 0 {
			divisor = gcd(divisor, allocated)
			sizes = append(sizes, info.Size())
		}
	}
	if divisor >= 512 && divisor&(divisor-1) == 0 {
		for _, size := range sizes {
			if size > 0 && size <= divisor/2 {
				return divisor, nil
			}
		}
	}
	return 0, fmt.Errorf("cannot measure the cluster size of %s", dir)
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func allocatedBytes(info fs.FileInfo) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return int64(st.Blocks) * 512
	}
	return info.Size()
}

// SpaceEstimate is what a selection takes on a filesystem with a given
// cluster size, for the items whose file count is known.
type SpaceEstimate struct {
	ClusterBytes int64
	AllocatedGB  float64 // with ClusterBytes
	SmallGB      float64 // with 4 KB clusters
	// Trees are the measured trees of small files, the most costly first.
	Trees []string
	// Unknown are trees with no measured file count.
	Unknown []string
}

// EstimateSpace estimates what ids take on a vault for platforms whose disk
// has clusterBytes clusters. Each file is counted as its share of the size
// plus one cluster, which came within 10% of the Open WebUI tree measured on
// exFAT at 4, 32 and 128 KB.
func EstimateSpace(cat *catalog.Catalog, ids, platforms []string, clusterBytes int64) SpaceEstimate {
	e := SpaceEstimate{ClusterBytes: clusterBytes}
	cost := map[string]float64{}
	for _, id := range ids {
		item, ok := cat.RecipeByID(id)
		if !ok {
			continue
		}
		files, known := item.FileCountFor(platforms)
		if !known {
			e.Unknown = append(e.Unknown, id)
			continue
		}
		size := item.SizeGBFor(platforms)
		e.AllocatedGB += size + float64(files)*float64(clusterBytes)/1e9
		e.SmallGB += size + float64(files)*smallClusterBytes/1e9
		if item.FileCount > 1 { // a single file (cyberchef's zip) is no tree
			e.Trees = append(e.Trees, id)
			cost[id] = float64(files)
		}
	}
	sort.SliceStable(e.Trees, func(i, j int) bool { return cost[e.Trees[i]] > cost[e.Trees[j]] })
	sort.Strings(e.Unknown)
	return e
}

// WriteClusterWarning tells w what the disk's large clusters cost the
// selection and how to avoid it. It writes nothing for small clusters.
func WriteClusterWarning(w io.Writer, e SpaceEstimate) {
	if e.ClusterBytes < LargeClusterBytes {
		return
	}
	cluster := clusterLabel(e.ClusterBytes)
	fmt.Fprintf(w, "warning: this disk stores files in %s clusters, so every file takes at least %s.\n", cluster, cluster)
	switch extra := e.AllocatedGB - e.SmallGB; {
	case len(e.Trees) > 0:
		fmt.Fprintf(w, "  The selection takes about %s here, %s more than with 4 KB clusters: %s %s many small files.\n",
			tui.FormatSize(e.AllocatedGB), tui.FormatSize(extra), listNames(e.Trees, 3), holdVerb(len(e.Trees)))
	case extra >= 0.05:
		fmt.Fprintf(w, "  The selection takes about %s more here than with 4 KB clusters.\n", tui.FormatSize(extra))
	default:
		fmt.Fprintln(w, "  The selection is mostly large files, so it loses little to that.")
	}
	if len(e.Unknown) > 0 {
		fmt.Fprintf(w, "  %s also %s many small files, not measured: expect several times their size.\n", listNames(e.Unknown, 4), holdVerb(len(e.Unknown)))
	}
	fmt.Fprintln(w, "  Formatting the disk with 4 KB clusters avoids this but erases it: see \"Preparing a stick\" in docs/usage.md.")
}

func holdVerb(n int) string {
	if n == 1 {
		return "holds"
	}
	return "hold"
}

func listNames(ids []string, limit int) string {
	if len(ids) <= limit {
		return strings.Join(ids, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(ids[:limit], ", "), len(ids)-limit)
}

func clusterLabel(bytes int64) string {
	if bytes >= 1<<20 && bytes%(1<<20) == 0 {
		return fmt.Sprintf("%d MB", bytes>>20)
	}
	return fmt.Sprintf("%d KB", bytes>>10)
}

// VaultUsage is what the files of a vault hold and what they take on disk.
type VaultUsage struct {
	ContentBytes   int64 `json:"content_bytes"`
	AllocatedBytes int64 `json:"allocated_bytes"`
	// ClusterBytes is the smallest space a file takes, the disk's cluster
	// size when the vault holds a file smaller than one; 0 if unknown.
	ClusterBytes int64 `json:"cluster_bytes,omitempty"`
}

// MeasureVault adds up the size and the allocated space of every file and
// directory under root. It writes nothing, so it works on a read-only stick.
func MeasureVault(root string) (VaultUsage, error) {
	var u VaultUsage
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil // a file removed while walking
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		allocated := allocatedBytes(info)
		u.AllocatedBytes += allocated
		if info.Mode().IsRegular() {
			u.ContentBytes += info.Size()
			if allocated > 0 && (u.ClusterBytes == 0 || allocated < u.ClusterBytes) {
				u.ClusterBytes = allocated
			}
		}
		return nil
	})
	return u, err
}

// WriteVaultUsage prints the vault's content size beside the space it takes.
func WriteVaultUsage(w io.Writer, u VaultUsage) {
	line := fmt.Sprintf("Size: %s in files, %s on disk", humanSize(u.ContentBytes), humanSize(u.AllocatedBytes))
	if u.ClusterBytes >= LargeClusterBytes {
		line += fmt.Sprintf(" (%s clusters)", clusterLabel(u.ClusterBytes))
	}
	fmt.Fprintln(w, line)
}
