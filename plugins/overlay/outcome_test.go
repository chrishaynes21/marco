package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/outcome"
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
	if outcome.ResultPrefix != "[result] " {
		t.Errorf("result prefix drifted from cmd/marco/intake.go: %q", outcome.ResultPrefix)
	}
	if routePrefix != "[route] " {
		t.Errorf("route prefix drifted from cmd/marco/perform.go: %q", routePrefix)
	}
	// Both are prefix-matched against a whole line, so a missing trailing space would
	// match lines the engine never meant to be consumed.
	for _, p := range []string{outcome.ResultPrefix, routePrefix} {
		if !strings.HasSuffix(p, " ") {
			t.Errorf("wire prefix must end in a space: %q", p)
		}
	}
	want := []string{"performed", "clarify", "refused", "unavailable", "cancelled", "failed"}
	for _, w := range want {
		if _, ok := outcome.Parse(w); !ok {
			t.Errorf("the engine's outcome %q is not one this overlay can render", w)
		}
	}
	// Six and only six. A seventh accepted word would be a state nobody defined, and
	// "canceled" with one L is the engine's OLD spelling — accepting it would hide a
	// drift rather than surface it.
	for _, bad := range []string{"ok", "canceled", "done", "success", ""} {
		if o, ok := outcome.Parse(bad); ok {
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
		want outcome.Outcome
	}{
		{"performed", childRun{result: "performed"}, outcome.Performed},
		{"clarify", childRun{result: "clarify"}, outcome.Clarify},
		{"refused", childRun{result: "refused"}, outcome.Refused},
		{"unavailable", childRun{result: "unavailable"}, outcome.Unavailable},
		{"cancelled", childRun{result: "cancelled"}, outcome.Cancelled},
		{"failed", childRun{result: "failed"}, outcome.Failed},

		// THE WIRE WINS OVER THE EXIT CODE. A refusal and a clarification both exit
		// non-zero; reading the error first would put them back under "failed", which
		// is where they were and why nobody could tell them apart.
		{"a refusal that also exited non-zero", childRun{result: "refused", err: boom}, outcome.Refused},
		{"a question that also exited non-zero", childRun{result: "clarify", err: boom}, outcome.Clarify},

		// No result line: only the OTHER subcommands (bind, forget, learn, …), which
		// announce none and either did the thing or errored.
		{"a verb that worked", childRun{}, outcome.Performed},
		{"a verb that errored", childRun{err: boom}, outcome.Failed},
		{"a child we killed", childRun{killed: true, err: boom}, outcome.Cancelled},

		// A word nobody defined is not rendered as itself.
		{"a drifted engine", childRun{result: "ok", err: boom}, outcome.Failed},
	}
	for _, c := range cases {
		if got := c.r.outcome(); got != c.want {
			t.Errorf("%s: outcome = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestTheLearnOfferNeedsBothHalves is the honesty rule.
//
// Offering to record a demonstration is only truthful when NOTHING took the request: no
// play answered to the words and Director could not be reached either. A Director that ran
// and failed is not an unknown command — answering "I could not do that" with "shall I
// learn it?" is a non-sequitur about something the person just watched go wrong. And an
// unavailable play that RESOLVED already exists, so the offer would invite somebody to
// learn a play Marco already learned.
func TestTheLearnOfferNeedsBothHalves(t *testing.T) {
	if !(childRun{result: "unavailable"}).offersLearn() {
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
		if c.r.offersLearn() {
			t.Errorf("%s must NOT become an offer to learn", c.name)
		}
	}
}

// TestEverySixOutcomeHasItsOwnStatusLine keeps the HUD from saying "ran" about something
// that did not run. The wording is checked loosely — the words a person reads may be
// reworded — but the six must not collapse onto fewer lines.
func TestEverySixOutcomeHasItsOwnStatusLine(t *testing.T) {
	// Walked from outcome.All rather than from a list written out here. A list would be a
	// seventh place the six words are recorded, and the whole point of the shared package is
	// that there is one — a word added to the vocabulary and not to the HUD must fail HERE,
	// which it cannot do if this test only knows the words it was told about.
	seen := map[string]outcome.Outcome{}
	for _, o := range outcome.All {
		s := statusLine(o, "my play")
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
	names := []string{"outcome.Performed", "outcome.Clarify", "outcome.Refused",
		"outcome.Unavailable", "outcome.Cancelled", "outcome.Failed"}
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

// TestTheRoutePrefixIsStillPinnedByHand holds the LAST cross-module literal.
//
// `[result] ` and the six words stopped being a duplicate the day internal/outcome became the
// one vocabulary: both sides now share a constant, and a compiler watches them. `[route] ` did
// not move with them, because it answers a different question — WHICH play a loose phrase became,
// not what happened to it — and its producer is cmd/marco in the ROOT module. Nothing links the
// two at compile time, so the pin is this test and there is no second line of defence.
//
// The failure it exists to catch is silent: the overlay stops finding a `[route] ` line, every
// run reports no resolved play, and `offersLearn` — which needs `route == ""` to mean "nothing
// took this" — starts offering to record a demonstration for plays Marco already knows.
//
// So the assertion is not "the constant equals what this file says"; that is a constant checked
// against itself. It reads the producer's own source across the module boundary and requires the
// two to still say the same thing.
func TestTheRoutePrefixIsStillPinnedByHand(t *testing.T) {
	if routePrefix != "[route] " {
		t.Errorf("the route prefix drifted to %q", routePrefix)
	}
	// Prefix-matched against a whole line: without the trailing space it would swallow lines
	// the engine never meant to be consumed.
	if !strings.HasSuffix(routePrefix, " ") {
		t.Errorf("a wire prefix must end in a space: %q", routePrefix)
	}

	// THE PRODUCER, not merely the literal somewhere in that package.
	//
	// Finding the string anywhere in cmd/marco is not enough, and the first version of this
	// test proved it: runaccount.go keeps its own hand-matched CONSUMER copy of the same
	// prefix, so rewording the producing Printf left the search satisfied and this test
	// green. What has to be found is the line that WRITES the tag.
	const producerDir = "../../cmd/marco"
	entries, err := os.ReadDir(producerDir)
	if err != nil {
		t.Fatalf("the producing package could not be read: %v", err)
	}
	// A print call whose format string opens with a bracketed wire tag.
	writes := regexp.MustCompile(
		`(?:Printf|Println|Print|Fprintf|Fprintln|Fprint)\((?:[A-Za-z0-9_.]+,\s*)?"(\[[a-z]+\] )`)
	tags := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := readFileString(producerDir + "/" + name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		for _, m := range writes.FindAllStringSubmatch(src, -1) {
			tags[m[1]] = name
		}
	}
	if len(tags) == 0 {
		t.Fatalf("no wire line is written anywhere in %s. This test can no longer see the "+
			"producer and must be pointed at wherever it moved to.", producerDir)
	}
	if _, ok := tags[routePrefix]; !ok {
		named := make([]string, 0, len(tags))
		for tag, in := range tags {
			named = append(named, tag+" ("+in+")")
		}
		sort.Strings(named)
		t.Errorf("nothing in %s writes a line beginning %q; the wire tags it does write "+
			"are: %s.\nThe overlay consumes this prefix (acts.go) and nothing compiles the "+
			"two together — the engine ships as a separate module. If the producer was "+
			"reworded, reword this constant with it; otherwise every run reports no "+
			"resolved play, and the learn offer starts offering to record demonstrations "+
			"for plays Marco already knows.", producerDir, routePrefix, strings.Join(named, ", "))
	}
}

// TestTheHudRendersEveryOutcomeInTheSet walks outcome.All rather than a list written out here.
//
// The defect this whole vocabulary removed was two genuinely different endings rendering as one
// line — "Marco refused to do that" and "it worked" reading identically in the one place a person
// looks to find out whether the thing happened. A statusLine missing an arm falls through to
// `default`, which says "failed", and puts the same defect back one word further along.
//
// Walking [outcome.All] is the point: a seventh word added to the engine arrives here as a
// failing test rather than as a HUD line that silently reads "failed".
func TestTheHudRendersEveryOutcomeInTheSet(t *testing.T) {
	const disp = "open the test"
	said := map[string]outcome.Outcome{}
	for _, o := range outcome.All {
		line := statusLine(o, disp)
		if strings.TrimSpace(line) == "" {
			t.Errorf("%s renders as nothing at all on the HUD", o)
			continue
		}
		// The sentence has to name what it is talking about; "refused:" alone tells a
		// person a refusal happened to something.
		if !strings.Contains(line, disp) {
			t.Errorf("%s renders %q, which never names what it happened to", o, line)
		}
		if prev, clash := said[line]; clash {
			t.Errorf("%s and %s both render as %q. Two different endings on one line is "+
				"the exact defect this vocabulary exists to remove: a missing arm falls "+
				"through to the default and reports a refusal as a failure.", prev, o, line)
		}
		said[line] = o
	}
	// And the one that must be right whatever else drifts: performed does not read as failure.
	if strings.Contains(statusLine(outcome.Performed, disp), "failed") {
		t.Error("a performed play renders as a failure")
	}
}

// The overlay's route prefix IS the engine's constant — not a second copy that happens to agree.
//
// # Why comparing the values proves nothing
//
// `routePrefix != "[route] "` is checked twice in this file already, and both checks pass just as
// happily against `const routePrefix = "[route] "` written out by hand. That is the mutation an
// independent run made, and both modules stayed green: the overlay's constant is a `string`
// either way, with the same eight characters in it, so no assertion about its VALUE can tell a
// shared definition from a duplicate of one.
//
// The defect a duplicate produces is not a wrong value today. It is that [outcome.RoutePrefix]
// can then be reworded on the engine side, both suites stay green, and the HUD silently stops
// finding `[route] ` lines — at which point `offersLearn`, which needs `route == ""` to mean
// "nothing took this", starts offering to record a demonstration for plays Marco already knows.
//
// So this asserts the LINK: the declaration reads `= outcome.RoutePrefix`, which is what makes a
// compiler responsible for the agreement.
func TestTheRoutePrefixIsTheEnginesConstantItself(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "outcome.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing outcome.go: %v", err)
	}
	var found bool
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "routePrefix" ||
				len(vs.Values) != 1 {
				continue
			}
			found = true
			sel, ok := vs.Values[0].(*ast.SelectorExpr)
			if !ok {
				t.Fatalf("routePrefix is declared as %T, not as the engine's constant.\n"+
					"A hand-written literal here agrees with the engine until "+
					"somebody rewords the engine's, and then nothing fails to "+
					"compile and the HUD stops reading route lines.",
					vs.Values[0])
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "outcome" || sel.Sel.Name != "RoutePrefix" {
				t.Errorf("routePrefix is declared from %s.%s, want outcome.RoutePrefix",
					types.ExprString(sel.X), sel.Sel.Name)
			}
		}
	}
	if !found {
		t.Fatal("outcome.go no longer declares routePrefix as a constant, so nothing here " +
			"is bound to the engine's wire line at all")
	}
}
