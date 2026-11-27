package htmlpack

import (
	"fmt"
	"regexp"
	"strings"
)

// The page's own code (its inline scripts, its script files and the local
// files they import, not the libraries) is small and written by an agent, so
// it is scanned strictly before bundling: any network address in a string,
// and the ways of loading a file beside the page that a single file cannot
// serve and esbuild does not bundle.

var (
	// A string that starts with a network address.
	ownRemoteRe = regexp.MustCompile("[\"'`]((?:https?:)?//[A-Za-z0-9][^\"'`\\s]*)")
	// XML namespaces name, they do not load.
	namespaceRe   = regexp.MustCompile(`^(https?:)?//www\.w3\.org/`)
	importMetaRe  = regexp.MustCompile(`\bimport\.meta\.url\b`)
	workerRe      = regexp.MustCompile("\\bnew\\s+(?:Shared)?Worker\\s*\\(\\s*(?:[\"'`]|new\\s+URL\\s*\\()")
	dynamicImport = regexp.MustCompile("\\bimport\\s*\\(\\s*[^\"'`\\s)]")
	// p5's loaders fetch the file they name.
	p5LoadRe = regexp.MustCompile("\\bload(?:Image|JSON|Strings|Table|Font|Sound|Model|Shader|XML|Bytes|Pixels)\\s*\\(\\s*[\"'`]([^\"'`]+)[\"'`]")
)

// scanOwnSource returns the problems in a piece of the page's own code.
func scanOwnSource(where, code string) []Problem {
	var problems []Problem
	add := func(kind, target, format string, args ...any) {
		problems = append(problems, Problem{Kind: kind, Detail: where + ": " + fmt.Sprintf(format, args...), Target: target})
	}
	for _, m := range ownRemoteRe.FindAllStringSubmatch(code, -1) {
		if !namespaceRe.MatchString(m[1]) {
			add("remote-resource", "", "%s is a network address, which a drive does not have; put what it loads beside the page and import it", m[1])
		}
	}
	if importMetaRe.MatchString(code) {
		add("relative-fetch", "", "import.meta.url finds files beside the script, which a packed page does not have; import the file instead")
	}
	if workerRe.MatchString(code) {
		add("relative-fetch", "", "a worker loads its script from a file, which a packed page does not have; run the code in the page or build the worker from a Blob")
	}
	if dynamicImport.MatchString(code) {
		add("relative-fetch", "", "import() of a computed name cannot be packed; import each module by its name")
	}
	for _, m := range p5LoadRe.FindAllStringSubmatch(code, -1) {
		if target := m[1]; isRemote(target) {
			add("remote-resource", "", "%s loads from the network", target)
		} else if !isInline(target) {
			add("relative-fetch", target, "%s (%s): import the file instead; it arrives as a data URL that the loader takes", relativeFetchMessage, target)
		}
	}
	return problems
}

var sourceMapRe = regexp.MustCompile(`(?m)^[ \t]*//[#@][ \t]*sourceMappingURL=.*$|/\*[#@][ \t]*sourceMappingURL=[^*]*\*/`)

// stripSourceMaps removes source-map comments, which name files (or
// addresses) the page does not have.
func stripSourceMaps(code string) string {
	if !strings.Contains(code, "sourceMappingURL") {
		return code
	}
	return sourceMapRe.ReplaceAllString(code, "")
}
