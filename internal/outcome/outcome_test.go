package outcome

import (
	"bytes"
	"testing"
)

// TestTheWireLineIsOneSharedLiteral — three surfaces match this literal across two Go modules.
// Nothing would fail to compile if it drifted, which is exactly why it is asserted.
func TestTheWireLineIsOneSharedLiteral(t *testing.T) {
	if ResultPrefix != "[result] " {
		t.Fatalf("the wire prefix changed to %q — plugins/overlay and cmd/marco/edit.go match this "+
			"literal and ship as separate modules, so nothing else would notice", ResultPrefix)
	}
	// RoutePrefix was named by this test's comment and NOT checked by it — found by an
	// independent mutation gate, which changed it to "[play] " and watched the whole tree stay
	// green. A claim that names a test is only worth what the test actually asserts, and this
	// one asserted half of what it said.
	if RoutePrefix != "[route] " {
		t.Fatalf("the route prefix changed to %q. plugins/overlay reads this to learn WHICH play "+
			"a loose phrase became; when it stops matching, the HUD quietly loses the play name "+
			"and its unknown-command offer to learn misfires", RoutePrefix)
	}
}

// TestBothWireLinesReadBackWhatTheyWrote — the two prefixes must not be able to shadow each
// other. A route line is not a result and a result line is not a route, and each reader is fed
// every line the child prints.
func TestBothWireLinesReadBackWhatTheyWrote(t *testing.T) {
	if _, ok := FromLine(Line(Performed)); !ok {
		t.Fatal("a result line did not read back as a result")
	}
	if _, ok := RouteFromLine(Line(Performed)); ok {
		t.Fatal("a RESULT line was read as a route — the two prefixes overlap")
	}
	name, ok := RouteFromLine(RoutePrefix + "open the test")
	if !ok || name != "open the test" {
		t.Fatalf("a route line read back as %q (ok=%v)", name, ok)
	}
	if _, ok := FromLine(RoutePrefix + "open the test"); ok {
		t.Fatal("a ROUTE line was read as an outcome")
	}
	// And an empty name is not a route. A bare prefix says nothing, and reporting "" as the
	// play a phrase resolved to would render a blank where the name belongs.
	if _, ok := RouteFromLine(RoutePrefix + "   "); ok {
		t.Fatal("a route line naming nothing was accepted")
	}
}

// TestEveryOutcomeSurvivesTheWire — announce it, read it back, get the same word. This is the
// whole contract between the engine and every front end.
func TestEveryOutcomeSurvivesTheWire(t *testing.T) {
	for _, want := range All {
		var b bytes.Buffer
		Announce(&b, want)
		got, ok := FromLine(b.String())
		if !ok {
			t.Fatalf("%q was announced as %q and did not read back as an outcome", want, b.String())
		}
		if got != want {
			t.Fatalf("%q went over the wire and came back as %q", want, got)
		}
	}
}

// TestTheSixWordsAreAClosedSet — a seventh word is a product decision, so it may not arrive by
// accident. This test is the place that decision has to be made.
func TestTheSixWordsAreAClosedSet(t *testing.T) {
	if len(All) != 6 {
		t.Fatalf("there are now %d outcomes. Six is the product vocabulary; adding one means "+
			"every surface must render it, so change this test deliberately or not at all", len(All))
	}
	seen := map[Outcome]bool{}
	for _, o := range All {
		if seen[o] {
			t.Fatalf("%q appears twice in All", o)
		}
		seen[o] = true
	}
	for _, want := range []Outcome{Performed, Clarify, Refused, Unavailable, Cancelled, Failed} {
		if !seen[want] {
			t.Fatalf("%q is not in All, so a surface walking All would silently never render it", want)
		}
	}
}

// TestNoTwoOutcomesShareAnExitCode is the promise to a front end that can see only the code: a
// refusal must be impossible to mistake for a success.
func TestNoTwoOutcomesShareAnExitCode(t *testing.T) {
	byCode := map[int]Outcome{}
	for _, o := range All {
		c := o.Exit()
		if prev, dup := byCode[c]; dup {
			t.Fatalf("%q and %q both exit %d — a surface reading only the code cannot tell them apart",
				prev, o, c)
		}
		byCode[c] = o
	}
	if Performed.Exit() != 0 {
		t.Fatalf("performed must exit 0; got %d", Performed.Exit())
	}
	if Unavailable.Exit() != 3 {
		t.Fatalf("unavailable must keep exit 3 — plugins/overlay already reads that number as "+
			"'never delivered, try something else'; got %d", Unavailable.Exit())
	}
}

// TestAnUnrecognisedResultIsNotSalvaged — a prefix followed by a word from another build is a
// disagreement, not a partial answer. Guessing which of the six was meant is how a refusal becomes
// a success on somebody's screen.
func TestAnUnrecognisedResultIsNotSalvaged(t *testing.T) {
	for _, line := range []string{
		"[result] succeeded",
		"[result] performed but only sort of",
		"[result] ",
		"[result] PERFORMED",
		"result: performed",
		"[route] open the test",
		"performed",
		"",
	} {
		if o, ok := FromLine(line); ok {
			t.Fatalf("%q was read as the outcome %q; it announces none", line, o)
		}
	}
}

// TestOrdinaryOutputIsNeverMistakenForAResult — a child's stdout also carries the play's own
// logging, and a surface streams every line of it through FromLine.
func TestOrdinaryOutputIsNeverMistakenForAResult(t *testing.T) {
	for _, line := range []string{
		"[INFO] typing into the search box",
		"[route] open the test",
		"[intake] source=typed decision=play play=open-the-test",
		"a line that merely mentions [result] performed in the middle",
	} {
		if _, ok := FromLine(line); ok {
			t.Fatalf("ordinary output was read as a result: %q", line)
		}
	}
}

// TestAResultLineSurvivesItsLineEnding — the line is read off a pipe, and on Windows it arrives
// with a carriage return.
func TestAResultLineSurvivesItsLineEnding(t *testing.T) {
	for _, line := range []string{"[result] refused\n", "[result] refused\r\n", "[result] refused"} {
		got, ok := FromLine(line)
		if !ok || got != Refused {
			t.Fatalf("%q did not read back as refused (got %q, ok=%v)", line, got, ok)
		}
	}
}

// TestParseIsExact — Valid and Parse are the only admission point, and they do not fold case or
// trim rubbish. A surface that received something else received something else.
func TestParseIsExact(t *testing.T) {
	if Valid("Performed") || Valid("performed ") || Valid("perform") || Valid("") {
		t.Fatal("Parse is matching something other than the exact six words")
	}
	for _, o := range All {
		if !Valid(string(o)) {
			t.Fatalf("%q is one of the six and did not validate", o)
		}
	}
}
