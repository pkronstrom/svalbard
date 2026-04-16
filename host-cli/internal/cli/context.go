package cli

import (
	"os"

	"github.com/pkronstrom/svalbard/host-cli/internal/vault"
)

// ResolveVaultRoot determines the vault root directory.
//
// If explicit is non-empty it is used directly. Otherwise the current working
// directory is walked upward until a manifest.yaml is found.
func ResolveVaultRoot(explicit string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return vault.FindRoot(explicit, cwd)
}
