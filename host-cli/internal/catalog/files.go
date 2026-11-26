package catalog

// treeTypes are the types whose items are a directory of files on the vault
// rather than one file.
var treeTypes = map[string]bool{"python-package": true, "python-venv": true, "app": true, "skills": true}

// FileCountFor is how many files and directories the recipe puts on a vault
// that targets platforms, counted like SizeGBFor, and whether that is known.
// A single-file recipe is one file per copy; a tree is its FileCount per
// copy, and unknown without one.
func (it Item) FileCountFor(platforms []string) (int, bool) {
	copies := 1
	if it.PerPlatform() && len(platforms) > 0 {
		copies = 0
		for _, p := range platforms {
			if it.SupportsPlatform(p) {
				copies++
			}
		}
	}
	switch {
	case it.FileCount > 0:
		return copies * it.FileCount, true
	case treeTypes[it.Type]:
		return 0, false
	default:
		return copies, true
	}
}
