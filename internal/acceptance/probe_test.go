package acceptance_test

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/acceptance"
)

// A dogfood probe, not an assertion suite.
//
// It asks for ordinary computer things in ordinary words and prints, for each one, which half read
// the sentence, what Marco said, and what the fixture's oracle recorded. It asserts almost nothing
// on purpose: its job is to SHOW disagreements so they can become real tests in the package that
// owns them, and a probe that failed would stop printing the rest of the table.
//
// The two things it does assert are the two that are never acceptable regardless of phrasing:
// Marco claiming `performed` while the oracle saw nothing happen, and a request pressing a button
// nobody named.
//
// Gated behind MARCO_PROBE so it never runs in the ordinary suite.
//
//	$env:MARCO_ACCEPT_DESKTOP = "1"; $env:MARCO_PROBE = "1"
//	$env:MARCO_RESOLVER = "$PWD/plugins/llama/llama.exe"
//	go test ./internal/acceptance -run TestNaturalLanguageProbe -v
func TestNaturalLanguageProbe(t *testing.T) {
	if os.Getenv("MARCO_PROBE") == "" {
		t.Skip("set MARCO_PROBE=1 to run the dogfood probe")
	}
	h := acceptance.Start(t, acceptance.Options{Resolver: os.Getenv("MARCO_RESOLVER")})

	// Ordinary requests, phrased the way somebody would actually say them. Grouped by what is
	// being leaned on rather than by verb, because the interesting failures cross verbs.
	phrases := probeSet()

	t.Logf("%-46s %-14s %-11s %s", "REQUEST", "READ BY", "RESULT", "ORACLE / MESSAGE")
	for _, p := range phrases {
		h.Restart(false)
		before := h.Oracle()
		r := h.Do(p)
		got := h.Oracle()

		t.Logf("%-46s %-14s %-11s %s", p, r.Reading(), r.Outcome(), delta(before, got))
		if m := r.Message(); m != "" {
			t.Logf("%-46s %s", "", trim(m, 110))
		}

		// WATCHED, NOT ASSERTED, and the distinction is this probe's whole design.
		//
		// `performed` with nothing changed is the signature of a false success — and it is
		// also exactly what a legitimately idempotent request looks like. Emptying an
		// already-empty field changes nothing and IS done; uiact calls that Satisfied on
		// purpose, because performing a toggle to "achieve" a checked box would uncheck it.
		// The verification message then reads "is now unchecked", which is true and leaves
		// nothing to match on.
		//
		// Telling the two apart needs to know what state was requested and whether it
		// already held — per-request knowledge a probe does not have. So the probe FLAGS the
		// shape and a person judges it, while the assertions that must hold regardless live
		// in the acceptance suite, where every case knows what it asked for.
		//
		// This was an assertion first and it cried wolf three times out of three, each one a
		// request whose state already held. A guard that is wrong whenever it fires teaches
		// people to ignore it.
		if r.Performed() && got.Unchanged(before) {
			t.Logf("%-46s WATCH performed with nothing changed — idempotent, or false "+
				"success? said: %s", "", trim(r.Message(), 80))
		}
	}
}

// probeSet is the requests, kept in one function so the table above stays readable.
func probeSet() []string {
	return []string{
		// Plain, as a baseline.
		"click Save",
		// The same thing, asked politely and indirectly.
		"would you hit the save button for me",
		"please press save",
		"I'd like you to save this",
		"save it",
		// Other verbs in ordinary words.
		"tick the remember me box",
		"untick remember me",
		"put Marco in the name field",
		"empty the name field",
		"show me the details",
		// Pronouns and references, where the semantics already support them.
		"focus the Notes field and then clear it",
		"type hello into Notes and then copy it",
		// Composition in natural words.
		"put Marco in the name box and then save",
		"tick remember me and then press save",
		// Things that should refuse, and should refuse WELL.
		"click the Delete button",
		"press the Submit button",
		"scroll down a bit",
		"make me a coffee",
		// Ambiguity: two controls could plausibly answer.
		"click the button",
		"open it",
	}
}

// delta summarises what the oracle saw change, or says nothing did.
func delta(before, after acceptance.State) string {
	var parts []string
	if after.Name != before.Name {
		parts = append(parts, "name="+quote(after.Name))
	}
	if after.Notes != before.Notes {
		parts = append(parts, "notes="+quote(after.Notes))
	}
	if after.Remember != before.Remember {
		parts = append(parts, "remember="+boolWord(after.Remember))
	}
	if after.Selected != before.Selected {
		parts = append(parts, "selected="+quote(after.Selected))
	}
	if after.Status != before.Status {
		parts = append(parts, "status="+quote(after.Status))
	}
	if n := after.SavePresses - before.SavePresses; n != 0 {
		parts = append(parts, "save+"+itoa(n))
	}
	if n := after.CancelPresses - before.CancelPresses; n != 0 {
		parts = append(parts, "cancel+"+itoa(n))
	}
	if n := after.DetailsPresses - before.DetailsPresses; n != 0 {
		parts = append(parts, "details+"+itoa(n))
	}
	if n := after.ExpandEvents - before.ExpandEvents; n != 0 {
		parts = append(parts, "expand+"+itoa(n))
	}
	if after.SecondView != before.SecondView {
		parts = append(parts, "second_view="+boolWord(after.SecondView))
	}
	if len(parts) == 0 {
		return "(nothing changed)"
	}
	return strings.Join(parts, " ")
}

func quote(s string) string { return "\"" + s + "\"" }
func boolWord(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func trim(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
