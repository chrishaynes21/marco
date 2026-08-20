package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// TestResultPrefixAndVocabularyArePinned holds the module boundary.
//
// The producer is cmd/marco/intake.go in the REPO ROOT — a different Go module — so no
// compiler and no cross-module test notices when either side is reworded. The failure that
// causes is silent and specific: the overlay stops finding a `[result] ` line, falls back
// to guessing from the exit code, and starts rendering refusals and cancellations as `ok`
// again, which is the exact defect this phase removed. These literals and theirs must be
// changed together.
func TestResultPrefixAndVocabularyArePinned(t *testing.T) {
	if resultPrefix != "[result] " {
		t.Errorf("result prefix drifted from cmd/marco/intake.go: %q", resultPrefix)
	}
	if routePrefix != "[route] " {
		t.Errorf("route prefix drifted from cmd/marco/perform.go: %q", routePrefix)
	}
	// Both are prefix-matched against a whole line, so a missing trailing space would
	// match lines the engine never meant to be consumed.
	for _, p := range []string{resultPrefix, routePrefix} {
		if !strings.HasSuffix(p, " ") {
			t.Errorf("wire prefix must end in a space: %q", p)
		}
	}
	want := []string{"performed", "clarify", "refused", "unavailable", "cancelled", "failed"}
	for _, w := range want {
		if _, ok := knownOutcome(w); !ok {
			t.Errorf("the engine's outcome %q is not one this overlay can render", w)
		}
	}
	// Six and only six. A seventh accepted word would be a state nobody defined, and
	// "canceled" with one L is the engine's OLD spelling — accepting it would hide a
	// drift rather than surface it.
	for _, bad := range []string{"ok", "canceled", "done", "success", ""} {
		if o, ok := knownOutcome(bad); ok {
			t.Errorf("%q was accepted as the outcome %q", bad, o)
		}
	}
}

// TestTheSixOutcomesComeFromTheWire is the point of the whole file: the outcome is READ,
// not derived. A refused play exits non-zero-or-zero depending on nothing the overlay can
// see, and three genuinely different things used to arrive as exit 0.
func TestTheSixOutcomesComeFromTheWire(t *testing.T) {
	boom := errors.New("child exited 5")
	cases := []struct {
		name string
		r    childRun
		want outcome
	}{
		{"performed", childRun{result: "performed"}, outcomePerformed},
		{"clarify", childRun{result: "clarify"}, outcomeClarify},
		{"refused", childRun{result: "refused"}, outcomeRefused},
		{"unavailable", childRun{result: "unavailable"}, outcomeUnavailable},
		{"cancelled", childRun{result: "cancelled"}, outcomeCancelled},
		{"failed", childRun{result: "failed"}, outcomeFailed},

		// THE WIRE WINS OVER THE EXIT CODE. A refusal and a clarification both exit
		// non-zero; reading the error first would put them back under "failed", which
		// is where they were and why nobody could tell them apart.
		{"a refusal that also exited non-zero", childRun{result: "refused", err: boom}, outcomeRefused},
		{"a question that also exited non-zero", childRun{result: "clarify", err: boom}, outcomeClarify},

		// No result line: only the OTHER subcommands (bind, forget, learn, …), which
		// announce none and either did the thing or errored.
		{"a verb that worked", childRun{}, outcomePerformed},
		{"a verb that errored", childRun{err: boom}, outcomeFailed},
		{"a child we killed", childRun{killed: true, err: boom}, outcomeCancelled},

		// A word nobody defined is not rendered as itself.
		{"a drifted engine", childRun{result: "ok", err: boom}, outcomeFailed},
	}
	for _, c := range cases {
		if got := c.r.outcome(); got != c.want {
			t.Errorf("%s: outcome = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestTheTeachOfferNeedsBothHalves is the honesty rule.
//
// Offering to record a demonstration is only truthful when NOTHING took the request: no
// play answered to the words and Director could not be reached either. A Director that ran
// and failed is not an unknown command — answering "I could not do that" with "shall I
// learn it?" is a non-sequitur about something the person just watched go wrong. And an
// unavailable play that RESOLVED already exists, so the offer would invite somebody to
// learn a play Marco already learned.
func TestTheTeachOfferNeedsBothHalves(t *testing.T) {
	if !(childRun{result: "unavailable"}).offersTeach() {
		t.Error("nothing took the request and no play resolved: that is the one honest offer")
	}
	no := []struct {
		name string
		r    childRun
	}{
		{"a resolved play whose bridge was unavailable",
			childRun{result: "unavailable", route: "open settings"}},
		{"Director ran it and it failed", childRun{result: "failed"}},
		{"Director ran it and refused", childRun{result: "refused"}},
		{"Director asked a question", childRun{result: "clarify"}},
		{"it was cancelled", childRun{result: "cancelled"}},
		{"it worked", childRun{result: "performed", route: "open settings"}},
		{"a plain verb that errored", childRun{err: errors.New("x")}},
	}
	for _, c := range no {
		if c.r.offersTeach() {
			t.Errorf("%s must NOT become an offer to learn", c.name)
		}
	}
}

// TestEverySixOutcomeHasItsOwnStatusLine keeps the HUD from saying "ran" about something
// that did not run. The wording is checked loosely — the words a person reads may be
// reworded — but the six must not collapse onto fewer lines.
func TestEverySixOutcomeHasItsOwnStatusLine(t *testing.T) {
	seen := map[string]outcome{}
	for _, o := range []outcome{outcomePerformed, outcomeClarify, outcomeRefused,
		outcomeUnavailable, outcomeCancelled, outcomeFailed} {
		s := o.status("my play")
		if !strings.Contains(s, "my play") {
			t.Errorf("%s: the status must name what it is about, got %q", o, s)
		}
		if prev, dup := seen[s]; dup {
			t.Errorf("%s and %s render the same status %q", prev, o, s)
		}
		seen[s] = o
	}
}

// TestEverySixOutcomeHasItsOwnGlyph is structural, because the thing it protects cannot be
// drawn in a test: drawResultIcon needs a live graphics context.
//
// The history row is the one place a person looks to find out whether the thing happened,
// and it used to draw the same green check for a play that worked and a play the door
// REFUSED. So every outcome must have its own arm in both the glyph and the colour switch;
// a missing arm draws nothing at all, silently.
func TestEverySixOutcomeHasItsOwnGlyph(t *testing.T) {
	data, err := readFileString("view.go")
	if err != nil {
		t.Fatalf("read view.go: %v", err)
	}
	names := []string{"outcomePerformed", "outcomeClarify", "outcomeRefused",
		"outcomeUnavailable", "outcomeCancelled", "outcomeFailed"}
	for _, fn := range []string{"func drawResultIcon", "func resultColor"} {
		body := funcBody(data, fn)
		if body == "" {
			t.Fatalf("%s moved; this test must move with it", fn)
		}
		for _, n := range names {
			if !strings.Contains(body, n) {
				t.Errorf("%s has no arm for %s — that outcome renders as nothing", fn, n)
			}
		}
		// And it must switch on the typed vocabulary, not on loose strings that no
		// compiler checks against the six.
		for _, stale := range []string{`case "ok"`, `case "canceled"`} {
			if strings.Contains(body, stale) {
				t.Errorf("%s still matches the old three-word vocabulary: %s", fn, stale)
			}
		}
	}
}

// funcBody returns the source of one top-level function, up to the next one.
func funcBody(src, decl string) string {
	i := strings.Index(src, decl)
	if i < 0 {
		return ""
	}
	body := src[i:]
	if j := strings.Index(body[1:], "\nfunc "); j > 0 {
		body = body[:j]
	}
	return body
}

func readFileString(name string) (string, error) {
	b, err := os.ReadFile(name)
	return string(b), err
}
