package frontpage

import (
	"crypto/subtle"
	"net/http"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
)

type preference struct {
	Key         string        `json:"key"`
	Group       string        `json:"group"`
	Label       string        `json:"label"`
	Description string        `json:"description"`
	Type        settings.Type `json:"type"`
	Value       any           `json:"value,omitempty"`
	Choices     []string      `json:"choices,omitempty"`
	Min         *float64      `json:"min,omitempty"`
	Max         *float64      `json:"max,omitempty"`
}

func (d *dashboard) preferences(w http.ResponseWriter, r *http.Request) {
	d.settingsMu.Lock()
	defer d.settingsMu.Unlock()
	store, err := settings.Open(d.root)
	if err != nil {
		http.Error(w, "Drive settings could not be read.", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodPost {
		d.mu.Lock()
		draining := d.draining
		d.mu.Unlock()
		if draining {
			http.Error(w, "Drive is preparing for removal", 409)
			return
		}
		if d.terminalKey == "" || subtle.ConstantTimeCompare([]byte(d.terminalKey), []byte(r.Header.Get("X-Svalbard-Key"))) != 1 {
			http.Error(w, "Use the private link printed by ./run web to edit settings.", http.StatusForbidden)
			return
		}
		var input struct{ Key, Value string }
		if !readJSON(w, r, &input) {
			return
		}
		if err := store.Set(input.Key, input.Value); err != nil {
			http.Error(w, "The setting could not be saved. Check its value and drive permissions.", http.StatusBadRequest)
			return
		}
	}
	out := []preference{}
	for _, g := range store.Groups() {
		for _, s := range g.Settings {
			item := preference{Key: s.Key(), Group: g.Label, Label: s.Label, Description: s.Description, Type: s.Type, Choices: s.Choices, Min: s.Min, Max: s.Max}
			if s.Type != settings.TypeSecret {
				item.Value = store.Value(s.Key())
			}
			out = append(out, item)
		}
	}
	writeJSON(w, out)
}
