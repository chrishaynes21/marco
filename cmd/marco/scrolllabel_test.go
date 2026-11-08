package main

import "testing"

// THE CONTROL CENTRE'S STEP LIST NAMES A SCROLL, rather than showing the raw sentence. ADR-336.
//
// Dropping the Scroll case must fail this.
func TestTheControlCentreNamesAScroll(t *testing.T) {
	if got := humanizeAction("do OS's Scroll with p1."); got != "Scroll" {
		t.Errorf("a scroll is labelled %q", got)
	}
}
