//go:build !darwin

package downloader

import "errors"

func cloneFile(src, dst string) error {
	return errors.ErrUnsupported
}
