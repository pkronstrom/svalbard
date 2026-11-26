package main

import (
	"fmt"
	"strconv"
	"strings"
)

// version is a parsed semantic version. Build metadata is ignored.
type version struct {
	major, minor, patch int
	pre                 string
	raw                 string
}

func parseVersion(s string) (version, bool) {
	raw := s
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s, _, _ = strings.Cut(s, "+")
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return version{}, false
		}
		nums[i] = n
	}
	return version{nums[0], nums[1], nums[2], pre, raw}, true
}

func (v version) String() string { return v.raw }

func compareVersions(a, b version) int {
	for _, d := range [3]int{a.major - b.major, a.minor - b.minor, a.patch - b.patch} {
		if d != 0 {
			if d < 0 {
				return -1
			}
			return 1
		}
	}
	switch {
	case a.pre == b.pre:
		return 0
	case a.pre == "":
		return 1
	case b.pre == "":
		return -1
	case a.pre < b.pre:
		return -1
	default:
		return 1
	}
}

// comparator is one bound of a range: op is one of < <= > >= =.
type comparator struct {
	op string
	v  version
}

func (c comparator) allows(v version) bool {
	d := compareVersions(v, c.v)
	switch c.op {
	case "<":
		return d < 0
	case "<=":
		return d <= 0
	case ">":
		return d > 0
	case ">=":
		return d >= 0
	default:
		return d == 0
	}
}

// versionRange is a set of alternatives ("||"), each a set of comparators
// that must all hold.
type versionRange struct {
	alternatives [][]comparator
	raw          string
}

// parseRange reads the npm range forms that registries' dependencies use:
// exact, x-ranges (4.x, 1.11.x, *, 4), caret, tilde, comparators separated
// by spaces, hyphen ranges and "||" alternatives.
func parseRange(s string) (versionRange, error) {
	r := versionRange{raw: s}
	s = strings.TrimSpace(s)
	if s == "" || s == "latest" {
		s = "*"
	}
	for _, alt := range strings.Split(s, "||") {
		alt = strings.TrimSpace(alt)
		var set []comparator
		if lo, hi, ok := strings.Cut(alt, " - "); ok {
			low, err := partialBounds(strings.TrimSpace(lo))
			if err != nil {
				return r, err
			}
			high, err := partialBounds(strings.TrimSpace(hi))
			if err != nil {
				return r, err
			}
			set = append(set, comparator{">=", low.lower})
			if high.upper != nil {
				set = append(set, comparator{"<", *high.upper})
			} else {
				set = append(set, comparator{"<=", high.lower})
			}
			r.alternatives = append(r.alternatives, set)
			continue
		}
		fields := strings.Fields(alt)
		for i := 0; i < len(fields); i++ {
			field := fields[i]
			// "> 1.2.3" written with a space.
			if (field == ">" || field == ">=" || field == "<" || field == "<=" || field == "=") && i+1 < len(fields) {
				field += fields[i+1]
				i++
			}
			cs, err := comparatorsFor(field)
			if err != nil {
				return r, fmt.Errorf("range %q: %w", s, err)
			}
			set = append(set, cs...)
		}
		r.alternatives = append(r.alternatives, set)
	}
	return r, nil
}

// bounds of a partial version such as 4, 4.5, 4.x: lower inclusive, upper
// exclusive (nil for "*").
type bounds struct {
	lower version
	upper *version
	exact bool
	parts int // how many of major, minor, patch were given
}

func partialBounds(s string) (bounds, error) {
	s = strings.TrimPrefix(s, "v")
	if s == "*" || s == "x" || s == "X" || s == "" {
		return bounds{lower: version{raw: "0.0.0"}}, nil
	}
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	var nums []int
	for _, p := range parts {
		if p == "x" || p == "X" || p == "*" {
			break
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return bounds{}, fmt.Errorf("version %q", s)
		}
		nums = append(nums, n)
	}
	b := bounds{parts: len(nums)}
	for len(nums) < 3 {
		nums = append(nums, 0)
	}
	b.lower = version{nums[0], nums[1], nums[2], pre, s}
	switch b.parts {
	case 0:
	case 1:
		b.upper = &version{major: nums[0] + 1, raw: "upper"}
	case 2:
		b.upper = &version{major: nums[0], minor: nums[1] + 1, raw: "upper"}
	default:
		b.exact = true
	}
	return b, nil
}

func comparatorsFor(field string) ([]comparator, error) {
	for _, op := range []string{">=", "<=", ">", "<", "="} {
		if rest, ok := strings.CutPrefix(field, op); ok {
			b, err := partialBounds(rest)
			if err != nil {
				return nil, err
			}
			switch {
			case b.exact || op == ">=" || op == "<":
				return []comparator{{op, b.lower}}, nil
			case op == ">":
				if b.upper == nil {
					return []comparator{{"<", version{raw: "0.0.0"}}}, nil
				}
				return []comparator{{">=", *b.upper}}, nil
			case op == "<=":
				if b.upper == nil {
					return nil, nil
				}
				return []comparator{{"<", *b.upper}}, nil
			default: // "=" with a partial version
				return xRange(b), nil
			}
		}
	}
	if rest, ok := strings.CutPrefix(field, "^"); ok {
		b, err := partialBounds(rest)
		if err != nil {
			return nil, err
		}
		lo := b.lower
		var hi version
		switch {
		case lo.major > 0 || b.parts == 1:
			hi = version{major: lo.major + 1, raw: "upper"}
		case lo.minor > 0 || b.parts == 2:
			hi = version{minor: lo.minor + 1, raw: "upper"}
		default:
			hi = version{patch: lo.patch + 1, raw: "upper"}
		}
		return []comparator{{">=", lo}, {"<", hi}}, nil
	}
	if rest, ok := strings.CutPrefix(field, "~"); ok {
		b, err := partialBounds(strings.TrimPrefix(rest, ">"))
		if err != nil {
			return nil, err
		}
		lo := b.lower
		hi := version{major: lo.major, minor: lo.minor + 1, raw: "upper"}
		if b.parts == 1 {
			hi = version{major: lo.major + 1, raw: "upper"}
		}
		return []comparator{{">=", lo}, {"<", hi}}, nil
	}
	b, err := partialBounds(field)
	if err != nil {
		return nil, err
	}
	if b.exact {
		return []comparator{{"=", b.lower}}, nil
	}
	return xRange(b), nil
}

func xRange(b bounds) []comparator {
	cs := []comparator{{">=", b.lower}}
	if b.upper != nil {
		cs = append(cs, comparator{"<", *b.upper})
	}
	return cs
}

// allows reports whether v is in the range. A pre-release is allowed only by
// a comparator that names the same major.minor.patch with a pre-release, as
// npm does.
func (r versionRange) allows(v version) bool {
	for _, set := range r.alternatives {
		ok := true
		preAllowed := v.pre == ""
		for _, c := range set {
			if !c.allows(v) {
				ok = false
				break
			}
			if c.v.pre != "" && c.v.major == v.major && c.v.minor == v.minor && c.v.patch == v.patch {
				preAllowed = true
			}
		}
		if ok && preAllowed {
			return true
		}
	}
	return false
}

// maxSatisfying returns the highest version in available that the range
// allows.
func maxSatisfying(available []string, r versionRange) (version, bool) {
	var best version
	found := false
	for _, s := range available {
		v, ok := parseVersion(s)
		if !ok || !r.allows(v) {
			continue
		}
		if !found || compareVersions(v, best) > 0 {
			best, found = v, true
		}
	}
	return best, found
}
