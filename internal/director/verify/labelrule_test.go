package verify

// EVERY MESSAGE THIS PACKAGE PRODUCES GOES THROUGH THE ONE LABEL RULE.
//
// Measured live: Marco's ordinary verification line read "keyboard focus moved from Chris
// Haynes chris.haynes2112@gmail.com to Settings". The fix is `named`, which quotes what looks
// like a control's NAME and describes what looks like CONTENT — and a fix like that is only
// worth having if it is the rule everywhere. A careful `labelOf` beside twenty raw `%q`s is
// decoration, and the one that leaks is whichever nobody changed.
//
// So this reads the package's own source. A list of the sites that quote a label is a list
// somebody forgets to extend; the rule is that there are no such sites.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// labelFields are the ways this package gets hold of a label it might quote.
var labelFields = []string{"target.Label", "el.Label", ".Label,", ".Label)"}

// theOnlyPlacesThatMayTouchALabelDirectly are the lines allowed to name a label field without
// `named`.
//
// Each is the rule itself or a comparison rather than a message:
//   - `named` and `labelOf` ARE the rule.
//   - a label used in a CONDITION — picking a sample, testing emptiness — is not quoted.
var theOnlyPlacesThatMayTouchALabelDirectly = []string{
	"if el, ok := w.Element(id); ok && el.Label != \"\" {",
	"return named(el.Label)",
	"if sample == \"\" && el.Label != \"\" {",
	"sample = el.Label",
	"if sample == nil && el.Label != \"\" {",
	// A comparison, not a message: the outcome check asks whether any control BEARS the
	// text the goal named, and the text it is compared against came from the goal.
	"for _, text := range []string{el.Label, el.Value} {",
	"was.Label, gone)",
}

func TestNoVerificationMessageQuotesALabelWithoutTheRule(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		checked++
		for i, line := range strings.Split(string(b), "\n") {
			code := line
			if c := strings.Index(code, "//"); c >= 0 {
				code = code[:c]
			}
			if strings.Contains(code, "named(") || !mentionsALabel(code) {
				continue
			}
			allowed := false
			for _, ok := range theOnlyPlacesThatMayTouchALabelDirectly {
				if strings.Contains(strings.TrimSpace(code), ok) {
					allowed = true
				}
			}
			if !allowed {
				t.Errorf("%s:%d uses a label without the rule:\n    %s\n"+
					"A label can be an email address or a line of somebody's "+
					"document. Put it through `named`, which quotes a control's "+
					"name and describes content. See directorapi.QuotableLabel.",
					name, i+1, strings.TrimSpace(line))
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source files were read, so this guarded nothing")
	}
}

func mentionsALabel(code string) bool {
	for _, f := range labelFields {
		if strings.Contains(code, f) {
			return true
		}
	}
	return false
}

// AND THE RULE ITSELF BEHAVES, which the walk above cannot show.
func TestTheLabelRuleQuotesNamesAndDescribesContent(t *testing.T) {
	if got := named("Bluetooth & devices"); got != `"Bluetooth & devices"` {
		t.Errorf("an ordinary control name reads as %s", got)
	}
	leak := "Chris Haynes chris.haynes2112@gmail.com"
	got := named(leak)
	if strings.Contains(got, "@") {
		t.Errorf("the measured leak survives: %s", got)
	}
	if !strings.Contains(got, "address") {
		t.Errorf("it hid the label without saying what kind: %s", got)
	}
	// And the same rule, through the same door, for an element.
	el := &directorapi.Element{Role: directorapi.RoleButton, Label: leak}
	if strings.Contains(el.ForAccount(), "@") {
		t.Errorf("ForAccount leaks where named does not: %s", el.ForAccount())
	}
}
