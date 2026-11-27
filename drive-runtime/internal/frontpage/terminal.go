package frontpage

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
)

func (d *dashboard) runSettings(w http.ResponseWriter, r *http.Request) {
	var input struct{ Args []string }
	if !readJSON(w, r, &input) {
		return
	}
	var output bytes.Buffer
	d.settingsMu.Lock()
	err := settings.Run(&output, &output, d.root, input.Args)
	d.settingsMu.Unlock()
	if err != nil {
		fmt.Fprintln(&output, err)
	}
	writeJSON(w, struct {
		Output string `json:"output"`
		Failed bool   `json:"failed"`
	}{Output: output.String(), Failed: err != nil})
}
