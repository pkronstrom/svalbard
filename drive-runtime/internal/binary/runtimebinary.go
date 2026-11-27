package binary

import (
	"fmt"
	"os/exec"

	"github.com/pkronstrom/svalbard/bintree"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

func Resolve(name, driveRoot string) (string, error) {
	if _, err := platform.Detect(); err != nil {
		return "", err
	}
	if path, err := ResolveOnDrive(name, driveRoot); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s not found", name)
}

// ResolveOnDrive is Resolve without the host's PATH, for a tool whose
// version matters: a host install would be another release, started with
// state written for the one on the drive. Archives are unpacked and found as
// the host CLI does it (see bintree.Find).
func ResolveOnDrive(name, driveRoot string) (string, error) {
	platformName, err := platform.Detect()
	if err != nil {
		return "", err
	}
	return bintree.Find(name, driveRoot, platformName)
}
