// Package videotext turns archived captions and host recognition into timed text.
package videotext

import (
	"bufio"
	"bytes"
	"html"
	"regexp"
	"strconv"
	"strings"
)

// Cue is text anchored to a position on the original video timeline.
type Cue struct {
	StartMS int64  `json:"start_ms"`
	Text    string `json:"text"`
}

var captionMarkup = regexp.MustCompile(`<[^>]*>`)

// ParseCaptions accepts WebVTT, including chapter tracks, and SubRip.
func ParseCaptions(data []byte) ([]Cue, error) {
	var cues []Cue
	var text []string
	start := int64(-1)
	flush := func() {
		if start >= 0 && len(text) != 0 {
			body := html.UnescapeString(captionMarkup.ReplaceAllString(strings.Join(text, " "), ""))
			if body = strings.Join(strings.Fields(body), " "); body != "" {
				cues = append(cues, Cue{start, body})
			}
		}
		start, text = -1, nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	scanner.Buffer(make([]byte, 4096), len(data)+1)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			flush()
			continue
		}
		if first, _, found := strings.Cut(line, "-->"); found {
			flush()
			start = captionTime(strings.TrimSpace(first))
			continue
		}
		if start >= 0 {
			text = append(text, line)
		}
	}
	flush()
	return cues, scanner.Err()
}

func captionTime(value string) int64 {
	parts := strings.Split(value, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return -1
	}
	last := parts[len(parts)-1]
	seconds, fraction, ok := strings.Cut(strings.ReplaceAll(last, ",", "."), ".")
	if !ok || len(fraction) != 3 {
		return -1
	}
	secs, secsErr := strconv.ParseInt(seconds, 10, 64)
	ms, msErr := strconv.ParseInt(fraction, 10, 64)
	if secsErr != nil || msErr != nil || secs < 0 || secs >= 60 || ms < 0 {
		return -1
	}
	var minutes int64
	for _, part := range parts[:len(parts)-1] {
		n, err := strconv.ParseInt(part, 10, 32)
		if err != nil || n < 0 {
			return -1
		}
		minutes = minutes*60 + n
	}
	return (minutes*60+secs)*1000 + ms
}
