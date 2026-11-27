//go:build unix

package workspace

import "syscall"

// If a regular file is swapped for a pipe, opening it must not wait for a
// writer before the file type can be checked again.
const readNonblock = syscall.O_NONBLOCK
