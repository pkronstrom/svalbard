package downloader

import "golang.org/x/sys/unix"

// cloneFile makes dst a copy-on-write clone of src. It works only when both
// are on the same APFS volume; the caller copies instead when it fails.
func cloneFile(src, dst string) error {
	return unix.Clonefile(src, dst, 0)
}
