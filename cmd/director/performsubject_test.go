package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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
	if _, ok := rt.performPlan(ctx, performApp, observe.Topology{}, steps, nil, &out); ok {
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

// NOTHING THAT CAN REACH THE WALKER INVENTS ITS OWN CONTEXT.
//
// # Why the source, and not a behaviour
//
// The defect was not that a step ignored cancellation — the walker honours it, and
// internal/director/rehearse tests that thoroughly. The defect was a `context.Background()`
// written at the boundary, which silently detaches everything below it. That is a property of the
// TEXT, and it came back once already by being easy to type.
//
// # Why this walks the directory instead of naming a file
//
// It used to say `const file = "perform.go"`, and that is exactly how the same defect survived one
// file over: `rehearserun.go` handed `context.Background()` to the identical walker for a LIVE
// rehearsal — "want me to try it once?" — so the Audience could not stop something typing on their
// real desktop, and `director stop` answered "nothing is running" while it did.
//
// So the subject is every production file in this directory that imports
// internal/director/rehearse. A new composition root for the walker is caught the day it is
// written rather than the day somebody happens to widen a test. The precedent is
// internal/platform/navsource/pump_test.go, which walks the tree rather than naming call sites —
// the house pattern for a defect whose shape repeats across sites.
func TestNothingThatCanReachTheWalkerInventsItsOwnContext(t *testing.T) {
	for _, file := range filesImporting(t, "internal/director/rehearse") {
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
				t.Errorf("%s:%d makes a context.%s(). Everything that can reach the "+
					"walker must descend from the caller's context, or that branch "+
					"cannot be stopped — which is how `director stop` came to answer "+
					"\"nothing is running\" while a play was typing.",
					file, fset.Position(call.Pos()).Line, sel.Sel.Name)
			}
			return true
		})
	}
}

// EVERY LIVE WALKER CHECKS THE FOREGROUND.
//
// `rehearse.Live.behind` returns false when `inFront` is nil, so the "input would land somewhere
// else" refusal — the one guard between a real keystroke and somebody else's window — is not
// merely weakened by a missing `WithForeground`, it is switched off entirely and silently.
//
// There were two composition roots for one walker and only one of them installed it, so the gate
// was live for a rehearsal Marco asked permission for and dead for a play the Audience asked for
// by name. This holds the property at the source, for whatever roots exist: any function that
// builds a `rehearse.NewLive` must also install the foreground answer.
func TestEveryLiveWalkerChecksTheForeground(t *testing.T) {
	for _, file := range filesImporting(t, "internal/director/rehearse") {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parsing %s: %v", file, err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if !callsMethod(fn.Body, "rehearse", "NewLive") {
				continue
			}
			if !callsAnyMethodNamed(fn.Body, "WithForeground") {
				t.Errorf("%s: %s builds a rehearse.Live and never calls WithForeground. "+
					"Live.behind reports false when inFront is nil, so the "+
					"window_not_in_front refusal can never fire and real input goes "+
					"to whatever the person happens to be looking at.",
					file, fn.Name.Name)
			}
		}
	}
}

// filesImporting lists this directory's PRODUCTION files that import one package.
//
// Tests are excluded deliberately: a test may legitimately mint a context to prove what a
// cancelled one does, and a fixture that builds a walker without a foreground answer is
// describing a dry run.
func filesImporting(t *testing.T, importPath string) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading this package: %v", err)
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, imp := range parsed.Imports {
			if strings.Trim(imp.Path.Value, `"`) == "github.com/chaynes-simpleclouds/marco/"+importPath {
				out = append(out, name)
				break
			}
		}
	}
	if len(out) == 0 {
		// A guard that silently examines nothing is worse than no guard: it passes
		// forever. If the import moves, this must be rewritten, not quietly retired.
		t.Fatalf("no production file in this package imports %s any more", importPath)
	}
	return out
}

// callsMethod reports whether the body contains pkg.name(...).
func callsMethod(body *ast.BlockStmt, pkg, name string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if ok && ident.Name == pkg && sel.Sel.Name == name {
			found = true
		}
		return true
	})
	return found
}

// callsAnyMethodNamed reports whether the body calls a method of this name on anything.
func callsAnyMethodNamed(body *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
			found = true
		}
		return true
	})
	return found
}

// A CONTEXT THIS DIRECTOR ACCEPTS IS A CONTEXT IT NAMES.
//
// # The defect had no call site to point at
//
// `learnTail.Rehearse` was spelled `func (t *learnTail) Rehearse(context.Context)` — the type
// without a name, which in Go is the way to say "I am obliged to accept this and I intend to
// ignore it". The Learn coordinator handed it the episode's context on the one path that types on
// somebody's real desktop, and it went into the bin one line before `Runtime.Rehearse` minted a
// `context.Background()` of its own.
//
// The Background half is caught by TestNothingThatCanReachTheWalkerInventsItsOwnContext. This
// catches the half that leaves no trace: a parameter that was never named cannot be found by
// searching for what it was used for, because it was used for nothing.
//
// A discarded context is always a decision worth writing down. If a method genuinely has no use
// for one, naming it `ctx` and leaving it unused costs nothing — Go permits an unused parameter —
// and the next reader can see that the choice was made rather than defaulted into.
func TestAContextThisDirectorAcceptsIsAContextItNames(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading this package: %v", err)
	}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Type.Params == nil {
				continue
			}
			for _, field := range fn.Type.Params.List {
				if !isContextType(field.Type) {
					continue
				}
				checked++
				if len(field.Names) == 0 {
					t.Errorf("%s:%d — %s accepts a context.Context and does not name it, "+
						"which throws away everything the caller was holding. Name it.",
						name, fset.Position(field.Pos()).Line, fn.Name.Name)
					continue
				}
				for _, id := range field.Names {
					if id.Name == "_" {
						t.Errorf("%s:%d — %s discards its context.Context into `_`. "+
							"Cancellation reaches this Director through that value "+
							"and nowhere else.",
							name, fset.Position(id.Pos()).Line, fn.Name.Name)
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no production function in this package takes a context.Context. That cannot " +
			"be right, so this guard is examining nothing and would pass forever.")
	}
}

// isContextType reports whether an expression is `context.Context`.
func isContextType(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Context" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "context"
}
