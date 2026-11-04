package main

// THE SHORTCUT TAB, RENDERED (S3).
//
// What only the page can get wrong: that Replace sends the key, the place and the VERSION the
// banner showed rather than whatever is in the form by then; that a key in use and a refusal leave
// the typed key where it was; that a Change and a Removal send the version of the row they were
// pressed on; that a shortcut set up by name is sent with the plan it was shown; that one routine's
// keys never appear under another's name; that a routine about no application has no place chosen
// for it; and that an Only-in routine shows everywhere as unavailable, with its one reason once.
//
// The backend is stubbed by testdata/shortcutspage.js, which the reviewed screenshots were rendered
// from too. `shortcutsacts_test.go` asks the same questions of the real `Acts`.

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func aRenderedShortcutTab(t *testing.T) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	stub, err := os.ReadFile(filepath.Join("testdata", "shortcutspage.js"))
	if err != nil {
		t.Fatal(err)
	}
	title := renderForCounters(t, browser, string(stub))
	if !strings.Contains(title, "err=") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	return title, func(key string) int {
		m := regexp.MustCompile(`\b` + key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
}

// Every claim, one assertion each, so a failure names the claim.
func TestTheShortcutTabDoesWhatItShows(t *testing.T) {
	title, n := aRenderedShortcutTab(t)
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	for _, c := range []struct{ key, says string }{
		{"chips", "the routine's row shows its key as it is pressed"},
		{"rowsays", "the tab lists the key and where it is listened for"},
		{"legacy", "a shortcut set up by name is shown, with Reassign"},
		{"leader", "the configured leader is said"},
		{"defaultwhere", "a routine about an application offers that application first"},
		{"setdisabledempty", "Set is unavailable until a key is pressed"},
		{"boxlower", "the key box takes the key pressed, as one lower-case key"},
		{"taken", "a key in use is shown with what it runs"},
		{"kept", "the typed key stays where it was when the key is in use"},
		{"firstset", "the first Set carries no consent"},
		{"replacesent", "Replace sends the key, the place and the version the banner showed"},
		{"replacenotform", "Replace does not send what the form says by then"},
		{"takengone", "the banner goes once the replacement lands"},
		{"changelabel", "a Change says which key it is changing"},
		{"cancelshown", "a Change can be cancelled"},
		{"changesent", "a Change sends the version of the row it was started on"},
		{"reassignsent", "a reassignment sends the version and the plan it was shown"},
		{"reassignsaid", "what a reassignment came to is said"},
		{"removesent", "a Remove sends the version of its row"},
		{"removesaid", "what a removal came to is said"},
		{"refused", "a refusal is the owner's sentence"},
		{"refusedbad", "a refusal is drawn as one"},
		{"refusedkept", "a refused key stays where it was typed"},
		{"copyclean", "another routine's tab shows none of the first one's keys"},
		{"copynodefault", "a routine about no application has no place chosen for it"},
		{"copyshadow", "a global key says where an application's own key hides it"},
		{"copysetdisabled", "Set is unavailable until a place is chosen"},
		{"pdfeverywhere", "an Only-in routine shows everywhere as unavailable"},
		{"pdfwhy", "an Only-in routine gives its one reason once"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %s=%d", c.says, c.key, n(c.key))
		}
	}
}
