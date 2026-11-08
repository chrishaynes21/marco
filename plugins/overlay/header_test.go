package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

// THE HEADER CAN BE HIDDEN, AND THE WIDGET SHRINKS BY IT. The header is the clock, the date and
// the name at the top of the full widget. A file saved before the setting existed says nothing
// about it and keeps it shown.
//
// Hiding the header and not shrinking the window must fail this.
func TestHidingTheHeaderShrinksTheWidget(t *testing.T) {
	isolatedConfig(t, `{"width":400}`) // written before the setting existed
	if !cfgSnapshot().Header {
		t.Fatal("a settings file that says nothing about the header hides it")
	}
	s := snapshot{}
	shown := layoutHeight(s)

	configChange(12, 1) // the panel's header row
	if cfgSnapshot().Header {
		t.Fatal("the panel's header row did not hide the header")
	}
	hidden := layoutHeight(s)
	// The crown (69px with its rule) goes, and the hint line starts at the top.
	if want := shown - (int(crownTop(true)) + 18 + 15 + 16 + 12 - bareTop); hidden != want {
		t.Errorf("with the header hidden the widget is %dpx tall, want %d (it was %d)",
			hidden, want, shown)
	}
	// AND WITH CPU AND MEMORY ON, they stay, with the rule under them.
	cfgMu.Lock()
	cfg.Metrics = true
	cfgMu.Unlock()
	if got, want := layoutHeight(s), hidden+int(crownTop(false))+15+12-bareTop; got != want {
		t.Errorf("header hidden with CPU and memory shown is %dpx, want %d", got, want)
	}
	if !widgetconfig.Known("header") {
		t.Error("the settings file does not know the header key")
	}
}
