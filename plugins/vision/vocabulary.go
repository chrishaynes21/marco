package main

import "strings"

// Normalising a detector's own words into Marco's closed vision vocabulary.
//
// # Why this lives in the plugin and not in the benchmark
//
// Because it has now been in the wrong place twice, and both times the symptom was identical:
// a detector ran perfectly and every one of its detections was discarded as an unknown class.
// Experiment-001 recorded it for Grounding DINO ("the adapter and the acceptance filter spoke
// different vocabularies... twelve of thirteen detections were discarded and the report blamed
// the model"), and ScreenParser reproduced it exactly — 14 detections produced, 14 rejected.
//
// The rule that prevents a third occurrence: **a model's own vocabulary ends at the plugin
// boundary.** Everything above this line — Director, fusion, visionbench, policy — speaks only
// Marco's classes, and no consumer should ever need to know which model produced a detection
// in order to understand it.
//
// # Why it is generic rather than per-model
//
// There is no `if model == screenparser` here, deliberately. UI detectors converge on much the
// same words — a button is called Button by nearly all of them — so one table over the union of
// those words serves every backend and costs nothing per model. A per-model mapper would be a
// second place for the same knowledge to drift, which is the failure this file exists to end.
//
// # Unknown stays unknown
//
// Anything unmapped keeps its native word and is refused downstream. That is the correct
// outcome: refusing an unrecognised class is how the closed vocabulary stays closed, and the
// temptation to map an ambiguous class onto `button` to lift a nameable score is precisely the
// inflation NameablePrecision was built to catch.

// marcoClass maps a detector's native class word onto Marco's vision vocabulary.
//
// Keys are lowercased and separator-insensitive, so `List Item`, `list_item` and `listitem`
// all resolve. The Marco side is the closed set defined by internal/director/perception/
// providers/vision: button, icon, text, field, checkbox, radio, slot, bar, panel, menu, image.
var marcoClass = map[string]string{
	// Pressable controls.
	"button": "button", "utility button": "button", "push button": "button",
	"pushbutton": "button", "btn": "button", "clickable": "button", "control": "button",

	// Menus and their entries. `menu item` and `list item` are the rows of a menu, which is
	// what a game's pause screen is made of — validated against real menu evidence rather
	// than assumed.
	"menu": "menu", "contextmenu": "menu", "context menu": "menu", "dockmenu": "menu",
	"editmenu": "menu", "popup menu": "menu", "popupmenu": "menu", "menu option": "menu",
	"menu entry": "menu", "menu item": "menu", "menuitem": "menu", "list item": "menu",
	"listitem": "menu", "dropdown": "menu",

	// Tabs.
	"tab": "button", "tab bar": "panel", "tabbar": "panel", "tablist": "panel",

	// Two-state.
	"checkbox": "checkbox", "check box": "checkbox", "switch": "checkbox",
	"toggle": "checkbox", "toggles": "checkbox",
	"radio": "radio", "radiobox": "radio", "radio button": "radio", "radiobutton": "radio",

	// Inputs.
	"field": "field", "text input": "field", "textinput": "field", "input": "field",
	"text field": "field", "search field": "field", "search bar": "field",
	"searchbar": "field", "select": "field", "picker": "field",
	"date-time picker": "field", "combobox": "field",

	// Proportional indicators.
	"bar": "bar", "progress bar": "bar", "progressbar": "bar", "slider": "bar",
	"meter": "bar", "gauge": "bar", "rating indicator": "bar", "steppers": "bar",

	// Containers. Large bounded regions that hold other things.
	"panel": "panel", "window": "panel", "screen": "panel", "side bar": "panel",
	"sidebar": "panel", "toolbar": "panel", "navigation bar": "panel",
	"status bar": "panel", "alert": "panel", "notification": "panel",
	"tooltip": "panel", "bottom navigation": "panel", "card": "panel",
	"column/browser": "panel", "table": "panel", "list": "panel", "carousel": "panel",
	"chart": "panel", "scroll": "panel", "dialog": "panel", "group": "panel",
	"container": "panel", "calendar": "panel", "modal": "panel", "hud": "panel",

	// Rendered text. Readable structure, never actionable — a heading is something to read,
	// not something to press, and mapping it to a control would invite a click.
	"text": "text", "heading": "text", "label": "text", "caption": "text",
	"title": "text", "text region": "text", "code snippet": "text", "link": "text",
	"badge": "text", "breadcrumb": "text", "pagination": "text", "page control": "text",
	"numeric display": "text", "counter": "text",

	// Pictorial.
	"icon": "icon", "app icon": "icon", "file icon": "icon", "logo": "icon",
	"avatar": "icon",
	"image":  "image", "picture": "image", "video": "image",

	// Grid cells.
	"slot": "slot", "grid cell": "slot", "cell": "slot", "tile": "slot",
	"inventory slot": "slot",
}

// normaliseClass maps a native detector word onto Marco's vocabulary.
//
// Returns the native word unchanged when nothing matches, so an unmapped class is REFUSED
// downstream rather than silently becoming something plausible. The second return says which
// happened, for the diagnostics that report how much of a model's vocabulary this build
// understands.
func normaliseClass(native string) (string, bool) {
	key := normaliseKey(native)
	if key == "" {
		return native, false
	}
	if c, ok := marcoClass[key]; ok {
		return c, true
	}
	// Singular/plural is the one variation worth absorbing: "buttons" for "button" invents
	// nothing. Anything beyond that is guessing.
	if trimmed := strings.TrimSuffix(key, "s"); trimmed != key {
		if c, ok := marcoClass[trimmed]; ok {
			return c, true
		}
	}
	return native, false
}

// normaliseKey lowercases and collapses separators so `List Item`, `list_item` and `List-Item`
// are one key.
func normaliseKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("_", " ", "-", " ", ".", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// VocabularyCoverage reports how much of a model's class list this build understands.
//
// Surfaced rather than silent: a model whose vocabulary is mostly unmapped will produce mostly
// refused detections, and that is worth knowing at startup rather than inferring from a
// benchmark result of zero.
func VocabularyCoverage(labels []string) (mapped, unknown int) {
	for _, l := range labels {
		if _, ok := normaliseClass(l); ok {
			mapped++
		} else {
			unknown++
		}
	}
	return mapped, unknown
}
