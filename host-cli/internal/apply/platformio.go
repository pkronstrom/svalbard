package apply

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

const platformIORegistryHost = "dl.registry.platformio.org"

// platformIOArchive identifies the registry package identity used by Core.
type platformIOArchive struct {
	Owner string
	Kind  string
	Name  string
}

func parsePlatformIOArchive(raw string) (platformIOArchive, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != platformIORegistryHost {
		return platformIOArchive{}, false
	}
	parts := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	if len(parts) != 6 || parts[0] != "download" || parts[1] == "" ||
		(parts[2] != "tool" && parts[2] != "platform") || parts[3] == "" || parts[4] == "" || parts[5] == "" {
		return platformIOArchive{}, false
	}
	return platformIOArchive{Owner: parts[1], Kind: parts[2], Name: parts[3]}, true
}

func platformIODestination(rawURL, platform string) (string, error) {
	if _, ok := parsePlatformIOArchive(rawURL); !ok {
		return "", fmt.Errorf("not a PlatformIO registry archive: %s", rawURL)
	}
	if platform == "" {
		return "tools/platformio/packages", nil
	}
	return path.Join("tools/platformio/packages", platform), nil
}
