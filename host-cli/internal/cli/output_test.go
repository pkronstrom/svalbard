package cli

import (
	"bytes"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/apply"
	"github.com/pkronstrom/svalbard/tui"
)

func TestApplyLinesPrintsDoneMessages(t *testing.T) {
	var out bytes.Buffer
	progress := applyLines(&out)
	progress(apply.ProgressEvent{ID: "skills-core", Status: tui.StatusDone})
	progress(apply.ProgressEvent{ID: "svalbard", Status: tui.StatusDone, Message: "restored svalbard skill b (local edits replaced)"})
	progress(apply.ProgressEvent{ID: "svalbard", Status: tui.StatusDone, Message: "restored svalbard skill c (local edits replaced)"})
	want := "skills-core: done\n" +
		"svalbard: restored svalbard skill b (local edits replaced)\n" +
		"svalbard: restored svalbard skill c (local edits replaced)\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}
