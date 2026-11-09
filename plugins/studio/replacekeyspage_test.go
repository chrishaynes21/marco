package main

// THE REPLACE CARD SAYS A ROUTINE'S SHORTCUTS ARE KEPT, RENDERED.
//
// The owner's ruling (2026-11-03): an agreed replacement is a new version of the same routine, so
// its shortcuts are kept, and the replacement confirmation says so once. Studio's card writes its
// own sentence rather than the owner's `says`, so the page is what has to say it. Built on
// `takenHarness` (redraw_test.go), with the collision carrying shortcuts. The note is read just
// before Replace is pressed, because a replacement that lands puts the card away.

import (
	"regexp"
	"strings"
	"testing"
)

func aReplaceCardWith(t *testing.T, collisionExtra string) string {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	h := strings.Replace(takenHarness, "says: 'there is already a routine saved there'}",
		"says: 'there is already a routine saved there'"+collisionExtra+"}", 1)
	h = strings.Replace(h, "  window.__replacedBySave = window.__replaced",
		"  window.__note = document.getElementById('capReplaceNote').textContent\n"+
			"  window.__replacedBySave = window.__replaced", 1)
	h = strings.Replace(h, "' as=' +", "' kept=' + (window.__note.includes("+
		"'is kept and will run the new version') ? 1 : 0) +"+
		" ' unknown=' + (window.__note.includes('could not be read') ? 1 : 0) +"+
		" ' keyed=' + (window.__note.includes('\\x60e in arkascended') ? 1 : 0) +"+
		" ' as=' +", 1)
	if !strings.Contains(h, "window.__note") || !strings.Contains(h, "' keyed='") {
		t.Fatal("the harness this builds on has changed shape")
	}
	return renderForCounters(t, browser, h)
}

func counter(t *testing.T, title, key string) string {
	t.Helper()
	m := regexp.MustCompile(`\b` + key + `=(\S+)`).FindStringSubmatch(title)
	if m == nil {
		t.Fatalf("no %s in %q", key, title)
	}
	return m[1]
}

func TestTheReplaceCardSaysItsShortcutsAreKept(t *testing.T) {
	title := aReplaceCardWith(t, ", shortcuts: ['\\x60e in arkascended']")
	if counter(t, title, "kept") != "1" || counter(t, title, "keyed") != "1" {
		t.Fatalf("the Replace card does not say its shortcut is kept: %q", title)
	}
}

func TestTheReplaceCardSaysNothingAboutShortcutsItDoesNotHave(t *testing.T) {
	title := aReplaceCardWith(t, "")
	if counter(t, title, "kept") != "0" || counter(t, title, "unknown") != "0" {
		t.Fatalf("a play with no shortcuts is said to keep some: %q", title)
	}
}

func TestTheReplaceCardSaysWhenItCannotTell(t *testing.T) {
	title := aReplaceCardWith(t, ", shortcuts_unknown: true")
	if counter(t, title, "unknown") != "1" || counter(t, title, "kept") != "0" {
		t.Fatalf("an unreadable shortcuts file is not said: %q", title)
	}
}
