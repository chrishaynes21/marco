package main

import (
	"image/color"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

// THE WIDGET DRAWS FROM THE SHARED PALETTES, AND DRAWS WHAT IT DREW (S10).
//
// The palettes moved to internal/widgetconfig, where TestThePalettesAreTheWidgetsOwn holds their
// values. This holds the other half: every name the settings panel cycles is one of them, each
// resolves to its own palette rather than quietly to the default, and an unknown name falls back
// as it always did.
func TestTheWidgetDrawsTheSharedPalettes(t *testing.T) {
	seen := map[theme]string{}
	for _, n := range themeNames {
		_, p := widgetconfig.PaletteFor(n)
		got := themeByName(n)
		if got != themeFrom(p) {
			t.Errorf("theme %q is not drawn from its shared palette", n)
		}
		if other, dup := seen[got]; dup {
			t.Errorf("themes %q and %q draw the same colours", n, other)
		}
		seen[got] = n
	}
	if len(themeNames) != 10 {
		t.Errorf("the panel cycles %d themes, want the 10 the widget shipped", len(themeNames))
	}
	// Two values read back as the widget had them, so the conversion is not merely consistent.
	if b := themeByName("Nord").bg; b != (color.RGBA{0x2E, 0x34, 0x40, 0xff}) {
		t.Errorf("nord's background is %v, want #2E3440", b)
	}
	if e := themeByName("light").errc; e != (color.RGBA{0xCF, 0x22, 0x2E, 0xff}) {
		t.Errorf("light's error colour is %v, want #CF222E", e)
	}
	if themeByName("no such theme") != themeByName("default") {
		t.Error("an unknown theme does not fall back to the default")
	}
}
