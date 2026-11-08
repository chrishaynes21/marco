package widgetconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// A SAVE FROM A FORM WRITES THE KEYS IT WAS SENT AND KEEPS EVERY OTHER, A FIELD NOBODY HERE
// KNOWS INCLUDED.
func TestASaveFromAFormWritesOnlyWhatWasSent(t *testing.T) {
	p := settingsAt(t, `{"theme":"nord","leader":"capslock","someoneElses":7}`)
	keys, err := Save([]byte(`{"theme":"dracula","width":400}`))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(keys, []string{"theme", "width"}) {
		t.Errorf("saved %v, want [theme width]", keys)
	}
	got := onDisk(t, p)
	if string(got["theme"]) != `"dracula"` || string(got["width"]) != `400` {
		t.Errorf("the sent settings were not written: %v", got)
	}
	if string(got["leader"]) != `"capslock"` || string(got["someoneElses"]) != `7` {
		t.Errorf("a setting nobody sent changed: %v", got)
	}
	if _, ok := got["idle"]; ok {
		t.Errorf("a default nobody sent was written: %v", got)
	}
}

// AND REFUSES, WRITING NOTHING, WHAT THE WIDGET WOULD NOT USE AS WRITTEN.
func TestASaveRefusesWhatTheWidgetWouldNotUse(t *testing.T) {
	for _, sent := range []string{
		`{"leader":"f13"}`, `{"leader":"ctrl"}`, `{"theme":"neon"}`, `{"corner":"middle"}`,
		`{"idle":0.1}`, `{"width":5000}`, `{"maxLines":0}`, `{"monitor":-1}`, `{"wake":"  "}`,
		`{"search":"ftp://x/{query}"}`, `{"colour":"red"}`, `{"width":"wide"}`,
		`{"theme":"dracula","corner":"middle"}`,
	} {
		p := settingsAt(t, `{"theme":"nord"}`)
		if _, err := Save([]byte(sent)); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: err = %v, want a refusal", sent, err)
		}
		if got := onDisk(t, p); len(got) != 1 || string(got["theme"]) != `"nord"` {
			t.Errorf("%s: a refused save wrote %v", sent, got)
		}
	}
	for _, sent := range []string{`{"leader":"backtick"}`, `{"leader":"f8"}`, `{"leader":"capslock"}`,
		`{"search":"https://example.com/?q={query}"}`, `{"search":""}`, `{"idle":0.2}`} {
		settingsAt(t, `{}`)
		if _, err := Save([]byte(sent)); err != nil {
			t.Errorf("%s: refused a value the widget uses: %v", sent, err)
		}
	}
}

// THE FORM OFFERS THE WIDGET'S OWN CHOICES, AND EVERY ONE OF THEM SAVES.
func TestTheFormOffersTheWidgetsChoicesAndEachSaves(t *testing.T) {
	for _, f := range Fields() {
		if !Known(f.Key) {
			t.Errorf("the form offers %q, which is not a setting", f.Key)
		}
		for _, c := range f.Choices {
			settingsAt(t, `{}`)
			if _, err := Save([]byte(`{"` + f.Key + `":"` + strings.ReplaceAll(c.Value, `\`, `\\`) + `"}`)); err != nil {
				t.Errorf("%s: the form offers %q and a save of it is refused: %v", f.Key, c.Value, err)
			}
		}
	}
	var theme Field
	for _, f := range Fields() {
		if f.Key == "theme" {
			theme = f
		}
	}
	if len(theme.Choices) != len(PaletteNames()) {
		t.Errorf("the form offers %d themes, the widget has %d", len(theme.Choices), len(PaletteNames()))
	}
}

// A RUNNING WIDGET FOLLOWS THE FILE, AND KEEPS WHAT ITS OWN PANEL HAS NOT SAVED.
func TestFollowKeepsWhatThePanelHasNotSaved(t *testing.T) {
	have := Defaults()
	have.Theme, have.Width = "nord", 500
	next := Defaults()
	next.Theme, next.Width, next.Corner = "dracula", 300, "top-left"
	changed := Follow(&have, next, map[string]bool{"width": true})
	if !slices.Equal(changed, []string{"corner", "theme"}) {
		t.Errorf("changed %v, want [corner theme]", changed)
	}
	if have.Theme != "dracula" || have.Corner != "top-left" {
		t.Errorf("the saved change was not taken: %+v", have)
	}
	if have.Width != 500 {
		t.Errorf("the panel's unsaved width was overwritten with %d", have.Width)
	}
	if again := Follow(&have, next, nil); !slices.Equal(again, []string{"width"}) {
		t.Errorf("a second follow with nothing kept changed %v, want [width]", again)
	}
}
