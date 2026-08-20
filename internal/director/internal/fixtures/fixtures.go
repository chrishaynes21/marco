// Package fixtures loads recorded desktops for the Director's tests.
//
// It exists so that every test speaks in terms of "the save-dialog desktop" rather
// than in terms of file paths and JSON shapes, and so that the wire format is
// decoded in exactly one place. It lives under internal/ within the Director because
// it is test scaffolding, not part of the system.
//
// Note what it does NOT do: it does not construct observations by hand. The inputs
// are real UI Automation trees captured from real windows (see fixtures/README.md),
// so a test that passes here is a test that passes against the shape of an actual
// desktop, not against the shape a developer imagined.
package fixtures

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// At is the timestamp every loaded fixture is stamped with, so conversions are
// byte-reproducible and a test never depends on the wall clock.
var At = time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

// wireSnapshot mirrors plugins/uia's Snapshot response payload.
type wireSnapshot struct {
	WindowID      string `json:"WindowId"`
	WindowTitle   string `json:"WindowTitle"`
	App           string `json:"App"`
	ProcessID     int    `json:"ProcessId"`
	WindowX       int    `json:"WindowX"`
	WindowY       int    `json:"WindowY"`
	WindowW       int    `json:"WindowW"`
	WindowH       int    `json:"WindowH"`
	Minimized     bool   `json:"Minimized"`
	Maximized     bool   `json:"Maximized"`
	WindowVisible bool   `json:"WindowVisible"`
	Partial       bool   `json:"Partial"`
	Reason        string `json:"Reason"`

	Elements []wireElement `json:"Elements"`
}

type wireElement struct {
	ID           string `json:"Id"`
	ParentID     string `json:"ParentId"`
	Role         string `json:"Role"`
	ControlType  string `json:"ControlType"`
	Label        string `json:"Label"`
	Value        string `json:"Value"`
	Description  string `json:"Description"`
	X            int    `json:"X"`
	Y            int    `json:"Y"`
	W            int    `json:"W"`
	H            int    `json:"H"`
	Enabled      bool   `json:"Enabled"`
	Visible      bool   `json:"Visible"`
	Focused      bool   `json:"Focused"`
	Selected     bool   `json:"Selected"`
	Offscreen    bool   `json:"Offscreen"`
	AutomationID string `json:"AutomationId"`
	ClassName    string `json:"ClassName"`
	Depth        int    `json:"Depth"`
}

// Desktop is one recorded moment, decoded into the Director's own types.
type Desktop struct {
	Name         string
	Observations []directorapi.Observation
	Window       directorapi.Window
	App          directorapi.Application
	Partial      bool
	Reason       string
}

// root locates the repository root from this file's own path, so tests work
// regardless of which package directory they run in.
func root() string {
	_, file, _, _ := runtime.Caller(0)
	// .../internal/director/internal/fixtures/fixtures.go → up five
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
}

// Load reads a recorded desktop by fixture name.
func Load(t *testing.T, name string) Desktop {
	t.Helper()

	path := filepath.Join(root(), "fixtures", name, "accessibility.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %q: %v\n"+
			"Record fixtures with: powershell -File plugins/uia/record-fixtures.ps1", name, err)
	}

	var wire wireSnapshot
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("decoding fixture %q: %v", name, err)
	}
	if len(wire.Elements) == 0 {
		t.Fatalf("fixture %q has no elements — it was probably recorded before the window's tree settled", name)
	}

	d := Desktop{
		Name:    name,
		Partial: wire.Partial,
		Reason:  wire.Reason,
		Window: directorapi.Window{
			ID:          directorapi.WindowID(wire.WindowID),
			Application: wire.App,
			Title:       wire.WindowTitle,
			Bounds: directorapi.Rect{
				X: wire.WindowX, Y: wire.WindowY, Width: wire.WindowW, Height: wire.WindowH,
			},
			Focused: true, Visible: wire.WindowVisible,
			Minimized: wire.Minimized, Maximized: wire.Maximized,
		},
		App: directorapi.Application{
			ID: wire.App, Name: wire.App, ProcessID: wire.ProcessID,
		},
	}

	for _, e := range wire.Elements {
		enabled, visible, focused, selected := e.Enabled, e.Visible, e.Focused, e.Selected
		d.Observations = append(d.Observations, directorapi.Observation{
			ID:          directorapi.ObservationID("acc:" + e.ID),
			Source:      directorapi.SourceAccessibility,
			Timestamp:   At,
			WindowID:    d.Window.ID,
			Role:        role(e.Role),
			Label:       e.Label,
			Value:       e.Value,
			Description: e.Description,
			Bounds: directorapi.Rect{
				X: e.X, Y: e.Y, Width: e.W, Height: e.H,
			},
			Enabled: &enabled, Visible: &visible,
			Focused: &focused, Selected: &selected,
			Confidence:     1.0,
			NativeID:       e.ID,
			ParentNativeID: e.ParentID,
			Attributes: map[string]any{
				"automation_id": e.AutomationID,
				"class_name":    e.ClassName,
				"control_type":  e.ControlType,
				"depth":         e.Depth,
				"offscreen":     e.Offscreen,
			},
		})
	}
	return d
}

// Find returns the observations in the desktop with exactly this label.
func (d Desktop) Find(label string) []directorapi.Observation {
	var out []directorapi.Observation
	for _, o := range d.Observations {
		if o.Label == label {
			out = append(out, o)
		}
	}
	return out
}

// roles is the same closed vocabulary the real provider client maps into.
var roles = map[string]directorapi.ElementRole{
	"button": directorapi.RoleButton, "text_field": directorapi.RoleTextField,
	"checkbox": directorapi.RoleCheckbox, "radio": directorapi.RoleRadio,
	"combo_box": directorapi.RoleComboBox, "list": directorapi.RoleList,
	"list_item": directorapi.RoleListItem, "menu": directorapi.RoleMenu,
	"menu_item": directorapi.RoleMenuItem, "tab": directorapi.RoleTab,
	"tab_list": directorapi.RoleTabList, "link": directorapi.RoleLink,
	"image": directorapi.RoleImage, "icon": directorapi.RoleIcon,
	"text": directorapi.RoleText, "heading": directorapi.RoleHeading,
	"dialog": directorapi.RoleDialog, "window": directorapi.RoleWindow,
	"pane": directorapi.RolePane, "group": directorapi.RoleGroup,
	"toolbar": directorapi.RoleToolbar, "scroll_bar": directorapi.RoleScrollBar,
	"slider": directorapi.RoleSlider, "progress_bar": directorapi.RoleProgressBar,
	"table": directorapi.RoleTable, "row": directorapi.RoleRow,
	"cell": directorapi.RoleCell, "tree": directorapi.RoleTree,
	"tree_item": directorapi.RoleTreeItem, "toggle": directorapi.RoleToggle,
}

func role(s string) directorapi.ElementRole {
	if r, ok := roles[s]; ok {
		return r
	}
	return directorapi.RoleUnknown
}
