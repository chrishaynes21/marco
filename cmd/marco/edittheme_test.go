package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

// THE CONTROL CENTRE OFFERS THE SHARED THEMES, in the widget's order (S10).
//
// Its Config view lists theme names in the page, where Go cannot reach them, so a palette added
// to or renamed in internal/widgetconfig would be one the control centre could not choose, or a
// name it offered that the widget and Studio resolve to the default.
func TestTheControlCentreOffersTheSharedThemes(t *testing.T) {
	m := regexp.MustCompile(`\{k:'theme'[^}]*opts:\[([^\]]*)\]`).FindStringSubmatch(editPage)
	if m == nil {
		t.Fatal("the control centre's theme field was not found, so this proves nothing")
	}
	var got []string
	for _, o := range strings.Split(m[1], ",") {
		got = append(got, strings.Trim(strings.TrimSpace(o), "'"))
	}
	if strings.Join(got, ",") != strings.Join(widgetconfig.PaletteNames(), ",") {
		t.Errorf("the control centre offers %v; the shared palettes are %v", got, widgetconfig.PaletteNames())
	}
}
