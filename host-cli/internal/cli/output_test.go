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

func TestApplyLinesPrintsTransferSizeAndCompletedItems(t *testing.T) {
	var out bytes.Buffer
	progress := applyLines(&out)
	progress(apply.ProgressEvent{ID: "archive", Status: tui.StatusActive})
	progress(apply.ProgressEvent{ID: "archive", Status: tui.StatusActive, Downloaded: 1, Total: 3 << 30})
	progress(apply.ProgressEvent{ID: "archive", Status: tui.StatusActive, Downloaded: 2, Total: 3 << 30})
	progress(apply.ProgressEvent{ID: "other", Status: tui.StatusFailed, Error: "network error"})
	progress(apply.ProgressEvent{ID: "archive", Status: tui.StatusDone, ItemsDone: 1, ItemsTotal: 2})
	want := "archive: downloading\n" +
		"archive: downloading 3.0 GB\n" +
		"other: failed: network error\n" +
		"archive: [1/2] done\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}
