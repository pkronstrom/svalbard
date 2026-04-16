package tui_test

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/pkronstrom/svalbard/tui"
)

// TestThemeHasAllRoles verifies that every style role in the Theme struct
// renders a non-empty string when given sample text.
func TestThemeHasAllRoles(t *testing.T) {
	theme := tui.DefaultTheme()

	v := reflect.ValueOf(theme)
	typ := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := typ.Field(i)
		style := v.Field(i)

		// Call Render("test") on each lipgloss.Style field
		renderMethod := style.MethodByName("Render")
		if !renderMethod.IsValid() {
			t.Fatalf("field %s does not have a Render method", field.Name)
		}

		result := renderMethod.Call([]reflect.Value{
			reflect.ValueOf("test"),
		})

		rendered := result[0].String()
		if len(rendered) == 0 {
			t.Errorf("role %s rendered an empty string", field.Name)
		}
	}
}

// TestDefaultThemeMatchesDriveRuntimeColors verifies that the Title style
// uses color "124", matching the existing drive-runtime hardcoded style.
func TestDefaultThemeMatchesDriveRuntimeColors(t *testing.T) {
	theme := tui.DefaultTheme()

	fg := theme.Title.GetForeground()
	c, ok := fg.(lipgloss.Color)
	if !ok {
		t.Fatalf("Title foreground is not a lipgloss.Color, got %T", fg)
	}

	if string(c) != "124" {
		t.Errorf("Title foreground should be color \"124\", got %q", string(c))
	}
}
