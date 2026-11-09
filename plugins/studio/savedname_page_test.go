package main

// A NAME TYPED AFTER A SAVE IS KEPT.
//
// Saving a recording clears the name field for the next one, and the poll that did it ran about
// two and a half times a second for as long as the "saved" sentence stood. So every letter typed
// into "what you will ask Marco for" after a save was wiped within half a second: measured live,
// "it's deleting my text i type". The field is cleared once, for that save.
//
// Clearing it on every poll again must fail this.

import (
	"strings"
	"testing"
)

func TestANameTypedAfterASaveIsKept(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, againHarnessWith(`{active: false, review: false,
		saved: 'Saved "cloud pull" as a routine.'}`)+`
let first = "?"
setTimeout(() => { document.getElementById("capName").value = "cloud pull" }, 0)
setTimeout(() => { first = document.getElementById("capName").value; document.getElementById("capName").value = "second routine" }, 1500)
setTimeout(() => {
  document.title = "first=[" + first + "] name=[" + document.getElementById("capName").value + "]"
}, 5200)`)
	if !strings.Contains(title, "name=") {
		t.Fatalf("the page never reported the field: %q", title)
	}
	if !strings.Contains(title, "first=[]") {
		t.Errorf("the save did not clear the name for the next play: %q", title)
	}
	if !strings.Contains(title, "name=[second routine]") {
		t.Errorf("what was typed after the save was wiped: %q", title)
	}
}
