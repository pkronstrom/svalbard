package videotext

import (
	"slices"
	"testing"
)

func TestParseCaptionsUsesOriginalTimeline(t *testing.T) {
	data := []byte("WEBVTT\n\n00:00.000 --> 00:02.000\n<c>Chapter &amp; intro</c>\n\n2\n01:02:03,456 --> 01:02:05,000\nTaimet\nkasvavat.\n")
	got, err := ParseCaptions(data)
	want := []Cue{{0, "Chapter & intro"}, {3723456, "Taimet kasvavat."}}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("ParseCaptions = %+v, %v, want %+v", got, err, want)
	}
}
