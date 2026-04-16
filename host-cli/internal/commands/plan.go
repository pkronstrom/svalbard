package commands

import (
	"fmt"
	"io"

	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

// WritePlan writes a human-readable summary of a reconciliation plan.
func WritePlan(w io.Writer, plan planner.Plan) error {
	fmt.Fprintf(w, "download: %d\n", len(plan.ToDownload))
	for _, id := range plan.ToDownload {
		fmt.Fprintf(w, "  + %s\n", id)
	}

	fmt.Fprintf(w, "remove: %d\n", len(plan.ToRemove))
	for _, id := range plan.ToRemove {
		fmt.Fprintf(w, "  - %s\n", id)
	}

	fmt.Fprintf(w, "unmanaged: %d\n", len(plan.Unmanaged))
	for _, id := range plan.Unmanaged {
		fmt.Fprintf(w, "  ? %s\n", id)
	}

	return nil
}
