package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// WHICH learned outcome, and whether it can be stopped.
//
// # Two defects, one shape
//
// Both were the same mistake in different clothes: a fact the system already held durably was
// re-derived at the boundary from something weaker.
//
//  1. THE SUBJECT. A play's identity is `routes.Origin.To`; its name is a label. The join was
//     Slug(phrase) -> prettyRoute -> EqualFold(Goal.Name), which holds only for plain
//     alphanumeric words. Measured: "open dad's settings" registers as open-dad-s-settings, is
//     asked for as "open dad s settings", and answers not_learned.
//  2. THE CONTEXT. `rehearse.Live.Perform` checks ctx before every step and has a cancelled
//     terminal ready. The only context ever handed in was context.Background(), so a learned play
//     could not be stopped and `director stop` said "nothing is running" while it typed.

// performApp is an application name no desktop has, so a test that reaches `bringForward` fails
// there honestly instead of raising somebody's real window.
const performApp = "marco-phase2-fixture"

// twoOutcomes is a cold Director holding two learned outcomes in one application.
//
// The DECOY is named exactly what `prettyRoute` makes of the real play's slug — "Open Dad S
// Settings" — and it is remembered FIRST, so it is what a name-first match would find. The real
// outcome keeps the punctuation the Audience actually used. This is the whole point of the
// fixture: with only one goal, testing the name before the subject still passes.
func twoOutcomes(t *testing.T) (rt *Runtime, decoy, real string) {
	t.Helper()
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	decoy, err := store.EstablishPlace(performApp, namedPlace(observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing the decoy's place: %v", err)
	}
	real, err = store.EstablishPlace(performApp, namedPlace(observe.TermSettings))
	if err != nil {
		t.Fatalf("establishing the outcome's place: %v", err)
	}
	if decoy == real {
		t.Fatalf("both places resolved to %q; the fixture cannot tell them apart", decoy)
	}
	for _, g := range []observe.Goal{
		{Name: "Open Dad S Settings", Application: performApp, Subject: decoy, Demonstrations: 1},
		{Name: "Open Dad's Settings", Application: performApp, Subject: real, Demonstrations: 1},
	} {
		if err := store.RememberGoal(performApp, g); err != nil {
			t.Fatalf("remembering %q: %v", g.Name, err)
		}
	}
	return &Runtime{observations: newObservationRegistry().withMemory(store)}, decoy, real
}

// oneOutcome is a cold Director holding a single learned outcome, named with punctuation.
//
// No decoy: this is the fixture for the join itself, where the only question is whether the words
// or the identity found it.
func oneOutcome(t *testing.T, called string) (*Runtime, string) {
	t.Helper()
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	subject, err := store.EstablishPlace(performApp, namedPlace(observe.TermSettings))
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	if err := store.RememberGoal(performApp, observe.Goal{
		Name: called, Application: performApp, Subject: subject, Demonstrations: 1,
	}); err != nil {
		t.Fatalf("remembering %q: %v", called, err)
	}
	return &Runtime{observations: newObservationRegistry().withMemory(store)}, subject
}

// THE SUBJECT IDENTIFIES THE OUTCOME. THE NAME IS ONLY A LABEL.
//
// # The mutations this kills
//
//   - drop the subject branch of namesOutcome: the punctuated phrase answers not_learned again,
//     which is the measured live defect.
//   - test the name BEFORE the subject: the decoy is performed instead of the outcome the caller
//     identified — a play running against the wrong screen because its label collided.
func TestTheSubjectIdentifiesTheOutcomeAndTheNameIsOnlyALabel(t *testing.T) {
	ctx := context.Background()

	// THE MEASURED DEFECT, one goal and no decoy: the words a slug turns back into are not
	// the words it was learned under. Held here so nobody mistakes the fix for "the name join
	// got cleverer" — it did not, it got demoted. Every punctuated phrase failed this way:
	// "Open Mouse Settings!", doubled spaces, "e-mail steve".
	//
	// The pairs are written out rather than derived, because deriving them would mean
	// importing internal/routes into the Director — and the Director knows nothing about
	// Plays. Each `asked` is what `prettyRoute(routes.Slug(taught))` produces on the marco
	// side: Slug keeps only [a-z0-9] and collapses everything else to one dash.
	for _, pair := range []struct{ taught, asked string }{
		{"Open Dad's Settings", "Open Dad S Settings"},
		{"Open Mouse Settings!", "Open Mouse Settings"},
		{"e-mail  steve", "E Mail Steve"},
	} {
		taught, asked := pair.taught, pair.asked
		alone, _ := oneOutcome(t, taught)
		lossy, err := alone.PerformGoal(ctx, service.PerformQuery{
			Application: performApp, Name: asked,
		})
		if err != nil {
			t.Fatalf("performing: %v", err)
		}
		if lossy.Refusal != "not_learned" {
			t.Fatalf("%q asked for as %q matched by name alone (%q). The premise of this "+
				"test is that the slug round trip is lossy.", taught, asked, lossy.Refusal)
		}

		// THE SAME WORDS, plus the identity. Now it is found.
		punctuated, subject := oneOutcome(t, taught)
		found, err := punctuated.PerformGoal(ctx, service.PerformQuery{
			Application: performApp, Name: asked, Subject: subject,
		})
		if err != nil {
			t.Fatalf("performing: %v", err)
		}
		if found.Refusal == "not_learned" {
			t.Fatalf("%q, whose durable subject is %q, was answered not_learned. The "+
				"subject and the goal were written in the same breath by the same learn "+
				"pass; if this cannot join them, no punctuated phrase can be performed.",
				taught, subject)
		}
		if found.Goal != taught {
			t.Errorf("the outcome found was %q, want %q", found.Goal, taught)
		}
	}

	// AND THE ORDER MATTERS, which one goal cannot show. The decoy is named exactly what
	// `prettyRoute` makes of the real play's slug, so a name-first match finds IT.
	rt, decoy, real := twoOutcomes(t)
	found, err := rt.PerformGoal(ctx, service.PerformQuery{
		Application: performApp, Name: "open dad s settings", Subject: real,
	})
	if err != nil {
		t.Fatalf("performing: %v", err)
	}
	if found.Goal != "Open Dad's Settings" {
		t.Errorf("the outcome found was %q, want the one whose subject was asked for. "+
			"%q is the decoy — the name was consulted before the identity, so a play "+
			"would run against a screen nobody identified.", found.Goal, "Open Dad S Settings")
	}

	// And the decoy is still reachable BY ITS OWN subject, so this is a join and not a
	// preference for one record.
	other, err := rt.PerformGoal(ctx, service.PerformQuery{
		Application: performApp, Name: "open dad s settings", Subject: decoy,
	})
	if err != nil {
		t.Fatalf("performing: %v", err)
	}
	if other.Goal != "Open Dad S Settings" {
		t.Errorf("asking for subject %q found %q", decoy, other.Goal)
	}
}

// A PLAY WITH NO SIDECAR STILL JOINS BY NAME.
//
// Older plays were saved before `routes.Origin` existed beside them, and a client that has not
// been rebuilt sends no Subject at all. Refusing those would break every play learned before this
// change — so the words remain the join of last resort, and only that.
//
// Deleting the name fallback must fail this.
func TestAPlayWithNoSidecarStillJoinsByName(t *testing.T) {
	rt, _, _ := twoOutcomes(t)

	v, err := rt.PerformGoal(context.Background(), service.PerformQuery{
		Application: performApp, Name: "Open Dad S Settings",
	})
	if err != nil {
		t.Fatalf("performing: %v", err)
	}
	if v.Refusal == "not_learned" {
		t.Fatal("a play with no subject sidecar could not be found by the name it was " +
			"learned under, so every play saved before the sidecar existed is now unrunnable")
	}
	if v.Goal != "Open Dad S Settings" {
		t.Errorf("the name join found %q", v.Goal)
	}
}

// ── stopping ──────────────────────────────────────────────────────────────────

// A PERFORMANCE THAT WAS STOPPED SAYS SO BEFORE IT MOVES A WINDOW.
//
// The registry cancels the context; the first thing after the goal is found is to ask. Without
// this a "stop" that arrived while the request queued would still bring an application forward
// and take a look, which is the desktop changing after somebody said not to.
//
// Deleting the ctx check before bringForward must fail this: the refusal becomes
// application_not_available, which is a fact about the desktop rather than about the Audience.
func TestAStoppedPerformanceNeverForegroundsAnything(t *testing.T) {
	rt, _, real := twoOutcomes(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	v, err := rt.PerformGoal(ctx, service.PerformQuery{
		Application: performApp, Name: "Open Dad's Settings", Subject: real,
	})
	if err != nil {
		t.Fatalf("performing: %v", err)
	}
	if v.Refusal != cancelledWord {
		t.Fatalf("a cancelled performance refused with %q, want %q — a caller cannot tell "+
			"\"you stopped it\" from \"it failed\"", v.Refusal, cancelledWord)
	}
	if len(v.Steps) != 0 {
		t.Errorf("%d step(s) ran after the request had been cancelled", len(v.Steps))
	}
	if !strings.Contains(strings.ToLower(v.Say), "stopped it") {
		t.Errorf("the sentence is %q; it should say the Audience stopped it", v.Say)
	}
}

// STOPPING BETWEEN EDGES ENDS THE WALK.
//
// The walker checks the context before every step of ONE edge. Between edges there is a fresh
// look, a re-acquisition and a memory write; a cancellation arriving in that window would
// otherwise begin the next edge anyway.
//
// Deleting the ctx check at the top of performPlan's loop must fail this.
func TestStoppingBetweenEdgesEndsTheWalk(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	steps := []observe.RelationshipRef{
		{From: "subj_a", To: "subj_b"},
		{From: "subj_b", To: "subj_c"},
	}
	var out service.PerformView
	if rt.performPlan(ctx, performApp, observe.Topology{}, steps, &out) {
		t.Fatal("a cancelled walk reported that the whole route worked")
	}
	if len(out.Steps) != 0 {
		t.Errorf("%d edge(s) were attempted after the walk had been cancelled", len(out.Steps))
	}
	if out.Refusal != cancelledWord {
		t.Errorf("a cancelled walk refused with %q, want %q", out.Refusal, cancelledWord)
	}
}

// THE CANCELLED WORD IS THE WALKER'S WORD.
//
// Two layers name the same event: the walker, when it stops between steps, and this file, when it
// stops between edges. A caller reads one field. If the two ever spelled it differently, half of
// the stops would be reported as failures — and the difference would be invisible until somebody
// stopped a play at the wrong moment.
func TestTheCancelledWordIsTheWalkersWord(t *testing.T) {
	if cancelledWord != string(rehearse.CancelledAttempt) {
		t.Errorf("this file says %q and the walker's terminal is %q",
			cancelledWord, rehearse.CancelledAttempt)
	}
	if cancelledWord != string(rehearse.RefusalCancelled) {
		t.Errorf("this file says %q and the walker's refusal is %q",
			cancelledWord, rehearse.RefusalCancelled)
	}
}

// NOTHING IN A PERFORMANCE INVENTS ITS OWN CONTEXT.
//
// # Why the source, and not a behaviour
//
// The defect was not that a step ignored cancellation — the walker honours it, and
// internal/director/rehearse tests that thoroughly. The defect was a `context.Background()`
// written at the boundary, which silently detaches everything below it. That is a property of the
// TEXT, and it came back once already by being easy to type.
//
// Every branch of a performance must descend from the registry's context, so this refuses any
// freshly-minted one anywhere in perform.go. The precedent is
// internal/platform/navsource/pump_test.go, which walks the tree rather than naming call sites.
func TestNothingInAPerformanceInventsItsOwnContext(t *testing.T) {
	const file = "perform.go"
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "context" {
			return true
		}
		if sel.Sel.Name == "Background" || sel.Sel.Name == "TODO" {
			t.Errorf("%s:%d makes a context.%s(). Everything a performance does must "+
				"descend from the registry command's context, or that branch of the "+
				"walk cannot be stopped — which is how `director stop` came to answer "+
				"\"nothing is running\" while a play was typing.",
				file, fset.Position(call.Pos()).Line, sel.Sel.Name)
		}
		return true
	})
}
