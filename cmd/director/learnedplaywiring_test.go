package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// Director watched and verified the behaviour. What it learned is now ordinary readable Marco.
//
// And nothing else. The play is not saved, not registered, not resolvable and not runnable — this
// milestone is about representation, and the tests below spend as much effort on what did NOT
// happen as on what did.

// verifiedRegistry drives the whole chain and stops with one route Marco has actually rehearsed.
//
// The rehearsal is completed through the production path and then stored the way the production
// path stores it, so the derived verification this milestone gates on is the real one.
func verifiedRegistry(t *testing.T) *observationRegistry {
	t.Helper()
	g := authorizedRegistry(t)
	grant := g.last.Grant()
	if grant == nil {
		t.Fatal("the fixture holds no authorization")
	}
	// The user names the starting screen. Without it the route cannot be written down at
	// all, which is the point of the gate rather than an inconvenience of the fixture.
	if store, ok := g.memory.(*semanticmemory.Store); ok {
		name, err := observe.UserSuppliedScreenName("the pause menu")
		if err != nil {
			t.Fatalf("naming: %v", err)
		}
		if err := store.NameSubject("testgame", grant.Source, name); err != nil {
			t.Fatalf("naming: %v", err)
		}
		// And where it is expected to finish. Both are the users words; neither is
		// something Director may invent.
		end, err := observe.UserSuppliedScreenName("the audio page")
		if err != nil {
			t.Fatalf("naming: %v", err)
		}
		if err := store.NameSubject("testgame", grant.Destination, end); err != nil {
			t.Fatalf("naming: %v", err)
		}
	}
	j, ok := g.judgeNow("testgame", grant.Relationship)
	if !ok {
		t.Fatal("no judgement for the authorized route")
	}
	// A completed attempt, stored by the production writer. The dry host cannot produce one —
	// nothing reached a computer — so the evidence is written here as `rememberRehearsal`
	// writes it, which is what a live `--live` run would have done.
	g.rememberRehearsal("testgame", j, rehearse.RehearsalResult{
		Relationship: grant.Relationship, Source: grant.Source,
		Destination: grant.Destination, Evidence: j.Digest, Live: true,
		Terminal: rehearse.CompletedRoute, StepsTaken: 1, Inputs: 1,
		Steps: []rehearse.StepRecord{{Outcome: rehearse.DirectlyVerified}},
	})
	return g
}

func learned(t *testing.T, g *observationRegistry) service.LearnedView {
	t.Helper()
	rt := &Runtime{observations: g}
	out, err := rt.LearnedPlay(service.LearnedQuery{Application: "testgame"})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	return out
}

func onlyPlay(t *testing.T, v service.LearnedView) service.LearnedPlayView {
	t.Helper()
	for _, p := range v.Plays {
		if p.Eligible {
			return p
		}
	}
	var refusals []string
	for _, p := range v.Plays {
		refusals = append(refusals, strings.Join(p.Refusals, ","))
	}
	t.Fatalf("no route could be written down (refusals: %v)", refusals)
	return service.LearnedPlayView{}
}

// ── THE headline ──────────────────────────────────────────────────────────────

// A verified route becomes ordinary Marco, through the real protocol, compiled before it is shown.
//
// Nothing here constructs a judgement, a play or a program. Deleting the lowering call, the
// compile check or the eligibility gate must fail this.
func TestAVerifiedRouteBecomesOrdinaryMarco(t *testing.T) {
	g := verifiedRegistry(t)
	play := onlyPlay(t, learned(t, g))

	// It is a play, and it says what the demonstration did.
	for _, want := range []string{
		"use os.",
		"is an actor.",
		"this can Run.",
		"this's Run does...",
		`do OS's Navigate with "confirm".`,
		"this is ok!",
		"the App is a script.",
	} {
		if !strings.Contains(play.Source, want) {
			t.Errorf("the play does not contain %q:\n%s", want, play.Source)
		}
	}

	// And it says NOTHING about how Director came to believe it.
	lower := strings.ToLower(play.Source)
	for _, leak := range []string{
		"subj_", "state_", "shadow", "hwnd", "confidence", "candidate", "evidence",
		"assessment", "rehears", "digest", "grant", "attempt", "session", "verdict",
		"checkpoint", "observ", "verified", "hypothes", "fingerprint",
	} {
		if strings.Contains(lower, leak) {
			t.Errorf("the play mentions %q. Director may know WHY it learned this; the play "+
				"says WHAT it does:\n%s", leak, play.Source)
		}
	}
	// No numbers at all: a play made of navigation meanings has nothing to count.
	if strings.ContainsAny(play.Source, "0123456789") {
		t.Errorf("the play contains a digit:\n%s", play.Source)
	}
}

// ── what did NOT happen ───────────────────────────────────────────────────────

// Writing a play down saves nothing, registers nothing, and makes nothing invokable.
//
// The most important test in the milestone. A generated route that quietly landed in `routes/`
// would become resolvable by a later natural-language request — which is a different permission
// from the one anybody gave, and nobody was asked for it.
func TestWritingAPlayDownRegistersNothing(t *testing.T) {
	dir := t.TempDir()
	before := treeOf(t, dir)
	// The whole working tree, not a guessed path. A test that watched only `routes/` would
	// miss a play written beside the binary, and "it was not saved" has to mean nowhere.
	cwdBefore := treeOf(t, ".")

	g := verifiedRegistry(t)
	play := onlyPlay(t, learned(t, g))
	if play.Source == "" {
		t.Fatal("nothing was generated")
	}

	if after := treeOf(t, dir); !reflect.DeepEqual(before, after) {
		t.Fatalf("lowering wrote files: %v", after)
	}
	// And the working tree is byte-unchanged. Not "contains no suspicious name" — UNCHANGED,
	// because a play written anywhere a registry might later scan becomes resolvable by a
	// natural-language request nobody made, whatever somebody called the file.
	if got := treeOf(t, "."); !reflect.DeepEqual(cwdBefore, got) {
		t.Fatalf("lowering wrote to disk:\nbefore %v\nafter  %v", cwdBefore, got)
	}
	// And the view carries no handle by which anything could ask for it later.
	rt := reflect.TypeOf(service.LearnedPlayView{})
	for _, forbidden := range []string{"ID", "Slug", "Path", "File", "Route", "Command",
		"Phrase", "Trigger", "Register"} {
		if _, ok := rt.FieldByName(forbidden); ok {
			t.Errorf("LearnedPlayView carries %s; a play with a name somebody can say is a "+
				"skill, and nobody has been asked", forbidden)
		}
	}
}

// treeOf lists a directory tree, or nothing if it does not exist.
func treeOf(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			out = append(out, filepath.ToSlash(path))
		}
		return nil
	})
	return out
}

// A route Marco has only WATCHED cannot be written down.
//
// The gate is the derived verification, not consistency and not corroboration. Two agreeing
// demonstrations are still only two things Marco saw somebody else do.
func TestAnUnrehearsedRouteIsNotWrittenDown(t *testing.T) {
	g := authorizedRegistry(t) // demonstrated twice, never rehearsed
	v := learned(t, g)
	if len(v.Plays) == 0 {
		t.Fatal("the fixture produced no routes at all")
	}
	for _, p := range v.Plays {
		if p.Eligible || p.Source != "" {
			t.Fatalf("a route nobody has tried was written down:\n%s", p.Source)
		}
		var sawNotVerified bool
		for _, r := range p.Refusals {
			if r == string(observe.RefusalNotVerified) {
				sawNotVerified = true
			}
		}
		if !sawNotVerified {
			t.Errorf("refusals %v do not say it has never been verified", p.Refusals)
		}
	}
}

// Evidence that stops matching stops the lowering, without anybody editing it.
func TestARevisedDemonstrationCanNoLongerBeWrittenDown(t *testing.T) {
	g := verifiedRegistry(t)
	if onlyPlay(t, learned(t, g)).Source == "" {
		t.Fatal("the fixture does not lower")
	}

	// The demonstration is revised. The rehearsal really happened; it is now about
	// something else.
	store := g.memory.(*semanticmemory.Store)
	grant := g.last.Grant()
	for _, c := range store.Candidates("testgame") {
		if c.Relationship == grant.Relationship && c.Sequence == 1 {
			c.Events++
			if err := store.RememberCandidate("testgame", c); err != nil {
				t.Fatalf("revising: %v", err)
			}
		}
	}
	for _, p := range learned(t, g).Plays {
		if p.Eligible {
			t.Fatalf("a revised demonstration was still written down:\n%s", p.Source)
		}
	}
}

// The same verified procedure produces byte-identical Marco across sessions.
//
// Session numbering, screen-state numbering, track ids, window generation and map iteration all
// change between runs of the fixture. None of them reaches the generator, and this is where that
// stops being an intention.
func TestTheSameProcedureAlwaysWritesTheSameMarco(t *testing.T) {
	first := onlyPlay(t, learned(t, verifiedRegistry(t))).Source
	if first == "" {
		t.Fatal("nothing was generated")
	}
	for i := 0; i < 3; i++ {
		again := onlyPlay(t, learned(t, verifiedRegistry(t))).Source
		if again != first {
			t.Fatalf("run %d generated different source:\n--- first ---\n%s\n--- again ---\n%s",
				i+2, first, again)
		}
	}
}

// THE PRE-FLIGHT ACCEPTS A PLAY THAT PRESSES A CONTROL BY NAME.
//
// # The live failure this exists for
//
// A route the Audience demonstrated, named at both ends, rehearsed and verified 1/1 — and then:
//
//	not_lowerable: core_cannot_express
//	the generated play does not compile: 101:13: unknown type "Target"
//
// Nothing was wrong with the play or with Marco. `compileGenerated` assembled its own world out of
// two module sources and two `use` lines, and a play that presses a control by name imports a
// third act. The spec suite asserted this exact play compiles and was green, because it kept its
// own longer list of modules. Two hand-maintained copies of one fact, and only one of them was on
// the path a person actually walks.
//
// So this enters through the PRODUCTION function, with the play the production lowerer writes.
// Deleting the `driver.CheckSource` call — or narrowing what it resolves — must fail here.
func TestThePreflightAcceptsAPlayThatPressesAControlByName(t *testing.T) {
	src, err := marcoexec.LowerActionsBetween("MouseSettings", "Open", "Home", "Mouse Settings",
		[][]marcoexec.PlayAction{{marcoexec.Press("Mouse", "button")}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if !strings.Contains(src, "use theater.") {
		t.Fatalf("the fixture no longer imports the Theater act, so it cannot catch the "+
			"failure it exists for:\n\n%s", src)
	}
	if err := compileGenerated(src); err != nil {
		t.Fatalf("the Director refuses its own play as inexpressible: %v\n\n%s", err, src)
	}
}

// And every act a learned play may import resolves through the same door.
//
// The bug was one missing module, so the guard is not "Theater works now" — it is that the
// pre-flight resolves whatever the generator emits. A play with an entry condition, a postcondition
// and a named press imports OS, Screen and Theater at once.
func TestThePreflightResolvesEveryActALearnedPlayImports(t *testing.T) {
	src, err := marcoexec.LowerActionsBetween("MouseSettings", "Open", "Home", "Mouse Settings",
		[][]marcoexec.PlayAction{
			{marcoexec.Press("Bluetooth & devices", "button")},
			{marcoexec.Press("Mouse", "button")},
		})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	for _, use := range []string{"use os.", "use screen.", "use theater."} {
		if !strings.Contains(src, use) {
			t.Fatalf("the play does not %s, so this proves less than it claims:\n\n%s", use, src)
		}
	}
	if err := compileGenerated(src); err != nil {
		t.Fatalf("a play importing all three acts is refused: %v\n\n%s", err, src)
	}
}

// A WHOLE ROUTE LOWERS EVERY EDGE, IN ORDER, EXACTLY ONCE.
//
// # The live failure
//
// A demonstration of Home → Bluetooth & devices → Mouse verified both edges and saved a play
// containing only the second:
//
//	do Screen's Showing with "Bluetooth & devices"…   ← begins in the middle
//	the target1 is a Target with Name "Mouse"…
//
// Asked from Home it refuses its own entry condition. Each edge lowers to its own play — right for
// reuse — and nothing had ever asked for the ROUTE.
//
// Three edges, not two: a rule that happens to work for a pair is not a rule.
func TestAWholeRouteLowersEveryEdgeInOrder(t *testing.T) {
	edge := func(from, to, press string) lowered {
		return lowered{
			view:     service.LearnedPlayView{From: from, To: to, Eligible: true},
			startsOn: from, endsOn: to,
			steps: [][]marcoexec.PlayAction{{marcoexec.Press(press, "item")}},
		}
	}
	plays := []lowered{
		edge("subj_c", "subj_d", "D"), // store order is not walk order
		edge("subj_a", "subj_b", "B"),
		edge("subj_b", "subj_c", "C"),
	}
	walk := []service.LearnedStep{
		{From: "subj_a", To: "subj_b"},
		{From: "subj_b", To: "subj_c"},
		{From: "subj_c", To: "subj_d"},
	}

	got, ok := joinWalk(plays, walk)
	if !ok {
		t.Fatal("a route whose every edge is eligible could not be written down")
	}
	if got.startsOn != "subj_a" {
		t.Errorf("the play starts on %q; the Audience began at subj_a", got.startsOn)
	}
	if got.endsOn != "subj_d" {
		t.Errorf("the play finishes on %q, want subj_d", got.endsOn)
	}
	if len(got.steps) != 3 {
		t.Fatalf("%d step run(s) for a three-edge route: %+v", len(got.steps), got.steps)
	}
	for i, want := range []string{"B", "C", "D"} {
		if len(got.steps[i]) != 1 || got.steps[i][0].Called != want {
			t.Errorf("step %d presses %+v, want %q — walk order, not store order",
				i+1, got.steps[i], want)
		}
	}
}

// An incomplete route is refused rather than shortened.
//
// Half a procedure is a different procedure. An edge missing from the walk means the play cannot
// represent what was taught, and a shorter play claiming to be it is worse than no play.
func TestARouteMissingAnEdgeIsNotWrittenDown(t *testing.T) {
	plays := []lowered{{
		view:     service.LearnedPlayView{From: "subj_a", To: "subj_b", Eligible: true},
		startsOn: "subj_a", endsOn: "subj_b",
		steps: [][]marcoexec.PlayAction{{marcoexec.Press("B", "item")}},
	}}
	walk := []service.LearnedStep{
		{From: "subj_a", To: "subj_b"},
		{From: "subj_b", To: "subj_c"}, // never lowered
	}
	if _, ok := joinWalk(plays, walk); ok {
		t.Error("a route with a missing edge was written down as though it were complete")
	}
}

// An ineligible edge blocks the route too.
func TestAnIneligibleEdgeBlocksTheRoute(t *testing.T) {
	plays := []lowered{
		{view: service.LearnedPlayView{From: "subj_a", To: "subj_b", Eligible: true},
			startsOn: "subj_a", endsOn: "subj_b",
			steps: [][]marcoexec.PlayAction{{marcoexec.Press("B", "item")}}},
		{view: service.LearnedPlayView{From: "subj_b", To: "subj_c", Eligible: false},
			startsOn: "subj_b", endsOn: "subj_c"},
	}
	walk := []service.LearnedStep{
		{From: "subj_a", To: "subj_b"}, {From: "subj_b", To: "subj_c"},
	}
	if _, ok := joinWalk(plays, walk); ok {
		t.Error("a route containing an edge that cannot be written down was written down")
	}
}
