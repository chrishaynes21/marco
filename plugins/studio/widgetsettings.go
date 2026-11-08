package main

// THE WIDGET'S SETTINGS, IN STUDIO.
//
// The pane draws itself from widgetconfig.Fields, so it offers the choices the widget's own panel
// cycles through, and saves through widgetconfig.Save, the one check the control centre's save
// goes through too: only the setting that changed is written, and a value the widget would not use
// is refused with a sentence rather than written. A running widget takes the change within about a
// second (plugins/overlay reload.go); the activation phrase waits for the widget to restart, and
// the field says so. The theme is Studio's as well (theme.go), so the page redraws with it at once.
//
// THE MONITOR IS COUNTED FROM 1 HERE, as the widget's panel shows it, and from 0 in the file,
// where it always was: the conversion is in these two functions and nowhere on the page.

import (
	"encoding/json"
	"errors"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

// WidgetSettingsForm is the pane: where the file is, what it offers, and what the widget runs with.
type WidgetSettingsForm struct {
	Path   string               `json:"path"`
	Fields []widgetconfig.Field `json:"fields"`
	Values map[string]any       `json:"values"`
}

// WidgetSaved is the answer to a save: what was written, or why nothing was.
type WidgetSaved struct {
	Saved   []string `json:"saved"`
	Refused bool     `json:"refused"`
	Says    string   `json:"says"`
}

// WidgetSettings is the widget's settings as the widget runs with them.
//
// Showing the monitor as the file has it must fail TestTheWidgetSettingsCountMonitorsFromOne.
func (a *Acts) WidgetSettings() WidgetSettingsForm {
	vals := map[string]any{}
	if b, err := json.Marshal(widgetconfig.Load()); err == nil {
		_ = json.Unmarshal(b, &vals)
	}
	if m, ok := vals["monitor"].(float64); ok {
		vals["monitor"] = m + 1
	}
	return WidgetSettingsForm{Path: widgetconfig.Path(), Fields: widgetconfig.Fields(), Values: vals}
}

// SaveWidgetSettings writes the settings sent, which are the ones the person changed.
func (a *Acts) SaveWidgetSettings(sent map[string]any) WidgetSaved {
	if len(sent) == 0 {
		return WidgetSaved{Says: "Nothing changed."}
	}
	if m, ok := sent["monitor"].(float64); ok {
		sent["monitor"] = m - 1
	}
	b, err := json.Marshal(sent)
	if err != nil {
		return WidgetSaved{Refused: true, Says: err.Error()}
	}
	keys, err := widgetconfig.Save(b)
	switch {
	case errors.Is(err, widgetconfig.ErrRefused):
		return WidgetSaved{Refused: true, Says: sentence(err.Error())}
	case err != nil:
		return WidgetSaved{Says: "Not saved: " + err.Error()}
	}
	return WidgetSaved{Saved: keys, Says: "Saved."}
}
