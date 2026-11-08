package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

// THE CONTROL CENTRE'S STEP LIST NAMES A SCROLL, rather than showing the raw sentence. ADR-336.
//
// Dropping the Scroll case must fail this.
func TestTheControlCentreNamesAScroll(t *testing.T) {
	if got := humanizeAction("do OS's Scroll with p1."); got != "Scroll" {
		t.Errorf("a scroll is labelled %q", got)
	}
}

// THE CONTROL CENTRE'S CONFIG VIEW OFFERS EVERY SETTING THE WIDGET'S FORM DOES. It keeps its own
// list (ADR-331), so a setting added to the form and not here is one the control centre cannot
// change, which is how the header nearly shipped.
func TestTheControlCentreOffersEverySettingTheFormDoes(t *testing.T) {
	for _, f := range widgetconfig.Fields() {
		if !strings.Contains(editPage, "{k:'"+f.Key+"'") {
			t.Errorf("the control centre's Config view does not offer %q", f.Key)
		}
	}
}
