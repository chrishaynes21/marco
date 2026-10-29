package main

// PRESSING PLAY IS THE AUTHORISATION, AND IT HAS AN EDGE.
//
// # What changed, and what this file holds
//
// The contract. A supervised run used to ask three questions per press — are you ready, which
// screen is this, do you agree to the step, may I press it — and a person had answered all of
// them by composing the Act and choosing to run it. Now the decision is made once, in front of
// the Act, and carried as a grant.
//
// A contract with no edge is not a contract, so these are the edges:
//
//	the grant is for the Act that is about to run, at the revision somebody saw
//	it names the interactions that Act performs, in the order it performs them
//	and a press of anything else is REFUSED, mid-run, with nothing sent
//
// And the thing that is gone is gone: a granted run puts no question to anybody. That is
// asserted through the production command rather than inferred, because "it should not ask" is
// the kind of claim that stays true in a unit test and false in the product.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// agGrantGiven is a grant as a surface writes one, over the interactions named.
//
// Complete, because an incomplete one is refused by `readActGrant` — it has to pin which version
// of the Act was authorised, which is a revision, a play digest and what each name means. The
// placeholders here are for the tests that drive `agrees` directly; a test going through the
// command wants `agGrantFor`, which reads the real ones off the plan.
func agGrantGiven(t *testing.T, act string, names ...string) actGrant {
	t.Helper()
	g := actGrant{Version: actGrantVersion, Act: act, Interactions: names,
		Revision: "rev-1", Source: "the-play-digest",
		By: "the person who pressed Play in Studio", At: dpLooked}
	for i, n := range names {
		g.Bound = append(g.Bound, grantBound{Name: n, Position: i})
	}
	return g
}

// agGrantFor is the grant Studio would write for a saved Act, read off its own plan.
//
// The revision, the digest and the mapping all come from the preflighted plan rather than being
// invented, which is what the surface does: it reads the bindings it is offering and hands back
// what it was shown.
func agGrantFor(t *testing.T, play string) actGrant {
	t.Helper()
	plan, code := planToPerform(play)
	if code != 0 {
		t.Fatalf("%q does not preflight, so there is nothing to authorise", play)
	}
	return agGrantForPlan(t, play, plan)
}

// agGrantForPlan is the same, for a plan the caller already has.
func agGrantForPlan(t *testing.T, play string, p demonstrated.Plan) actGrant {
	t.Helper()
	g := actGrant{Version: actGrantVersion, Act: play, Interactions: p.Order,
		Revision: p.Revision, Source: p.Source,
		By: "the person who pressed Play in Studio", At: dpLooked}
	for _, name := range p.Order {
		g.Bound = append(g.Bound, grantBound{Name: name, Position: p.Steps[name]})
	}
	return g
}

// agWritten is a grant on disk, where the run reads it from.
func agWritten(t *testing.T, g any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "granted.json")
	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A GRANT THAT DOES NOT SAY ENOUGH TO BE A DECISION IS NOT ONE.
//
// Every case here is a surface that meant to hand over somebody's decision and handed over
// something else. None of them falls back to asking: the person is in their game, and performing
// under a guess about what they meant is the single thing this type exists to prevent.
func TestAGrantThatIsNotADecisionIsRefused(t *testing.T) {
	for _, c := range []struct {
		name  string
		grant map[string]any
		says  string
	}{
		{"no version", map[string]any{"act": "a", "interactions": []string{"x"},
			"by": "somebody", "at": dpLooked}, "version 0"},
		{"a version this Marco does not perform", map[string]any{"version": 99,
			"act": "a", "interactions": []string{"x"}, "by": "somebody",
			"at": dpLooked}, "version 99"},
		{"no Act", map[string]any{"version": 1, "interactions": []string{"x"},
			"by": "somebody", "at": dpLooked}, "names no Act"},
		{"no interactions", map[string]any{"version": 1, "act": "a",
			"by": "somebody", "at": dpLooked}, "names no interactions"},
		{"nobody gave it", map[string]any{"version": 1, "act": "a",
			"interactions": []string{"x"}, "at": dpLooked},
			"does not say who gave it"},
		{"no instant", map[string]any{"version": 1, "act": "a",
			"interactions": []string{"x"}, "by": "somebody"},
			"does not say when"},
		// ── AND THE THREE THAT PIN WHICH ACT ──
		//
		// Required, not optional, because an optional check is one that can be
		// skipped by leaving a field out — and the last of these is the only term
		// that sees the same names bound to different interactions.
		{"no revision", map[string]any{"version": 1, "act": "a",
			"interactions": []string{"x"}, "by": "somebody", "at": dpLooked,
			"source": "d", "bound": []grantBound{{Name: "x"}}},
			"names no demonstration revision"},
		{"no play digest", map[string]any{"version": 1, "act": "a",
			"interactions": []string{"x"}, "by": "somebody", "at": dpLooked,
			"revision": "r", "bound": []grantBound{{Name: "x"}}},
			"names no play digest"},
		{"no mapping for its names", map[string]any{"version": 1, "act": "a",
			"interactions": []string{"x"}, "by": "somebody", "at": dpLooked,
			"revision": "r", "source": "d"},
			"says what 0 of them mean"},
		{"a mapping for other names", map[string]any{"version": 1, "act": "a",
			"interactions": []string{"x"}, "by": "somebody", "at": dpLooked,
			"revision": "r", "source": "d",
			"bound": []grantBound{{Name: "y"}}},
			"one of the two lists is wrong"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := readActGrant(agWritten(t, c.grant))
			if err == nil {
				t.Fatalf("%s was accepted as an authorisation", c.name)
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the refusal does not say which half is missing "+
					"(%q): %v", c.says, err)
			}
		})
	}
	// AND A FILE THAT IS NOT THERE AT ALL.
	if _, err := readActGrant(filepath.Join(t.TempDir(), "nothing.json")); err == nil {
		t.Error("a missing authorisation was accepted")
	}
}

// AND A GRANT IS FOR THE THING SOMEBODY WAS SHOWN.
//
// Checked against the PREFLIGHTED plan, before a window is acquired. The revision case is the
// one worth the words: an Act composed again between the press and the run is a different set of
// interactions under the same name, and spending somebody's decision on it would be spending it
// on something they never saw.
func TestAGrantIsForTheActSomebodyWasShown(t *testing.T) {
	art := dpATwoTapPanel()
	art.Application = "arkascended"
	plan := demonstrated.Plan{
		Name: "two-taps", Revision: "rev-1", Source: "the-play-digest",
		Order:   []string{"the graphics tab", "apply changes"},
		Steps:   map[string]int{"the graphics tab": 0, "apply changes": 1},
		Routine: art,
	}
	given := func(mut func(g *actGrant)) actGrant {
		g := agGrantGiven(t, "two-taps", "the graphics tab", "apply changes")
		g.Revision = "rev-1"
		mut(&g)
		return g
	}

	// THE ONE THAT AGREES, first, so the cases below are failures of one term each.
	if err := given(func(*actGrant) {}).agrees("two-taps", plan); err != nil {
		t.Fatalf("a grant for exactly this Act was refused: %v", err)
	}
	// AND THE NAME IS READ AS A PERSON SAYS IT, which is how every other name in this
	// product is read.
	if err := given(func(g *actGrant) { g.Act = "Two-Taps" }).agrees("two-taps",
		plan); err != nil {
		t.Errorf("the same Act under a different capitalisation was refused: %v", err)
	}

	for _, c := range []struct {
		name string
		mut  func(g *actGrant)
		says string
	}{
		{"for another Act", func(g *actGrant) { g.Act = "something else" },
			"performs"},
		{"for a revision that has been composed again since",
			func(g *actGrant) { g.Revision = "rev-0" }, "composed again since"},
		{"for another application",
			func(g *actGrant) { g.Application = "notepad.exe" }, "pinned to"},
		{"for fewer interactions than the Act performs",
			func(g *actGrant) { g.Interactions = []string{"the graphics tab"} },
			"names 1 interaction(s)"},
		{"for the same interactions the other way round",
			func(g *actGrant) {
				g.Interactions = []string{"apply changes",
					"the graphics tab"}
			}, "the order somebody authorised is not the order"},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := given(c.mut).agrees("two-taps", plan)
			if err == nil {
				t.Fatalf("a grant %s was accepted", c.name)
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the refusal does not say %q: %v", c.says, err)
			}
		})
	}
}

// AND THE SAME NAMES BOUND TO DIFFERENT INTERACTIONS ARE A DIFFERENT ACT.
//
// # The hole this closes, and why the other four terms cannot
//
// An Act can be composed again over the SAME demonstration with the SAME names in the SAME
// order, bound to DIFFERENT positions. Then:
//
//	the revision is the demonstration's, and the demonstration did not move
//	the names and their order are what the person typed
//	the application is the demonstration's
//	and the play's own digest is unchanged, because its sentences ARE those names
//
// Everything a person can see is identical and every control that would be pressed is different.
// The premise is asserted below by composing it both ways through the production composer, rather
// than asserted here in prose.
func TestTheSameNamesBoundToDifferentInteractionsAreRefused(t *testing.T) {
	// THE ACT AS IT IS NOW: the two names the other way round.
	plan := demonstrated.Plan{
		Name: "two-taps", Revision: "rev-1", Source: "the-same-digest",
		Order:   []string{"the graphics tab", "apply changes"},
		Steps:   map[string]int{"the graphics tab": 1, "apply changes": 0},
		Routine: dpATwoTapPanel(),
	}
	// AND THE AUTHORISATION, GIVEN WHEN THEY MEANT THE OTHER WAY ROUND.
	g := agGrantGiven(t, "two-taps", "the graphics tab", "apply changes")
	g.Revision, g.Source = "rev-1", "the-same-digest"
	g.Bound = []grantBound{{Name: "the graphics tab", Position: 0},
		{Name: "apply changes", Position: 1}}

	err := g.agrees("two-taps", plan)
	if err == nil {
		t.Fatal("an Act recomposed over the same names would have been performed " +
			"under an authorisation given for different interactions")
	}
	for _, says := range []string{"the graphics tab", "interaction 0", "interaction 1",
		"composed again since"} {
		if !strings.Contains(err.Error(), says) {
			t.Errorf("the refusal does not say %q: %v", says, err)
		}
	}
	// ── AND THE FOUR OLDER TERMS ALL PASS, which is the point: a grant with no
	//    mapping in it is accepted here, so this term is the one doing the work and
	//    not a second copy of one that already held.
	was := g
	was.Bound = nil
	if err := was.agrees("two-taps", plan); err != nil {
		t.Errorf("the revision, names, order and digest terms refuse this case "+
			"after all, so the mapping is not what closes it: %v", err)
	}
}

// AND THE PREMISE, THROUGH THE PRODUCTION COMPOSER.
//
// Not a hypothetical. Composing one demonstration's two interactions under two names, and then
// composing it again with the same names swapped onto the other positions, produces bindings
// whose revision, names, order AND play digest are all identical — so every term the grant
// carried before this change is blind to it. If this test ever fails, the hole closed itself and
// the mapping check above is belt and braces; until then it is the only term that sees this.
func TestRecomposingTheSameNamesChangesNothingTheOldTermsCouldSee(t *testing.T) {
	art := dpATwoTapPanel()
	one := []demonstrated.Named{
		{Position: 0, Name: "the graphics tab"},
		{Position: 1, Name: "apply changes"},
	}
	other := []demonstrated.Named{
		{Position: 1, Name: "the graphics tab"},
		{Position: 0, Name: "apply changes"},
	}
	srcA, bindsA, err := demonstrated.Compose(art, "two-taps", one, dpLooked,
		marcoChecks{})
	if err != nil {
		t.Fatal(err)
	}
	srcB, bindsB, err := demonstrated.Compose(art, "two-taps", other, dpLooked,
		marcoChecks{})
	if err != nil {
		t.Fatal(err)
	}

	if srcA != srcB {
		t.Fatalf("the two plays differ, so the digest alone would have caught "+
			"this:\n--- one ---\n%s\n--- the other ---\n%s", srcA, srcB)
	}
	if bindsA.Source != bindsB.Source {
		t.Errorf("the bindings record different digests (%s, %s)", bindsA.Source,
			bindsB.Source)
	}
	// AND THE NAMES, THE ORDER AND THE REVISION ARE THE SAME.
	for i := range bindsA.Scenes {
		if bindsA.Scenes[i].Name != bindsB.Scenes[i].Name {
			t.Errorf("scene %d is named %q and %q", i, bindsA.Scenes[i].Name,
				bindsB.Scenes[i].Name)
		}
		if bindsA.Scenes[i].Revision != bindsB.Scenes[i].Revision {
			t.Errorf("scene %d pins revisions %q and %q", i,
				bindsA.Scenes[i].Revision, bindsB.Scenes[i].Revision)
		}
		// AND THE POSITIONS ARE WHAT DIFFER, which is the whole finding.
		if bindsA.Scenes[i].Position == bindsB.Scenes[i].Position {
			t.Errorf("scene %d means interaction %d both ways round", i,
				bindsA.Scenes[i].Position)
		}
	}
}

// AND THE APPLICATION IT NAMES IS THE ONE THE ACT IS PINNED TO.
//
// Its own test because the Act in the table above carries no application, so the case there
// proves the mismatch and not the match.
func TestAGrantAgreesWithTheApplicationTheActIsPinnedTo(t *testing.T) {
	art := dpATwoTapPanel()
	art.Application = "arkascended"
	plan := demonstrated.Plan{Name: "two-taps", Revision: "rev-1",
		Source:  "the-play-digest",
		Order:   []string{"the graphics tab", "apply changes"},
		Steps:   map[string]int{"the graphics tab": 0, "apply changes": 1},
		Routine: art}

	g := agGrantGiven(t, "two-taps", "the graphics tab", "apply changes")
	g.Revision, g.Application = "rev-1", "arkascended"
	if err := g.agrees("two-taps", plan); err != nil {
		t.Fatalf("a grant naming the Act's own application was refused: %v", err)
	}
}

// ── THE EDGE AT THE PRESS ───────────────────────────────────────────────────

// A PRESS THE GRANT DOES NOT NAME IS REFUSED, AND NOTHING IS SENT.
//
// # Why this exists although preflight already checked the names
//
// Because they check different things. `agrees` compares the grant with the PLAN; this compares
// it with what the play actually aimed at, which is runtime behaviour. Defence in depth, and
// said plainly: while preflight holds, the name case here cannot fire in production. The
// index-zero case can — an aim that outlived its interaction names nothing — and both are the
// same refusal, because "Marco is about to press something nobody authorised" is one event
// however it came about.
func TestAPressTheGrantDoesNotNameIsRefused(t *testing.T) {
	out := &strings.Builder{}
	scenes := actInteractions(dpATwoTapPanel(), 2)
	g := &grantedGranter{
		grant:  agGrantGiven(t, "two-taps", "the graphics tab"),
		out:    out,
		live:   true,
		life:   10 * time.Second,
		now:    func() time.Time { return dpLooked },
		scenes: scenes,
	}

	// ── THE INTERACTION IT WAS GIVEN FOR: permitted, and stamped with the
	//    observation it was asked in.
	scenes.aim(1, "the graphics tab", 0)
	a, ok := g.Authorise(context.Background(),
		workflowrun.Intent{Step: 0, Epoch: 4, Generation: 1, Window: "w"})
	if !ok {
		t.Fatalf("the interaction the grant names was refused:\n%s", out)
	}
	if a.Epoch != 4 || a.Generation != 1 || a.Window != "w" {
		t.Errorf("the authorisation is not stamped with what it was asked about: %+v", a)
	}
	if !strings.Contains(a.By, "pressed Play") {
		t.Errorf("the authorisation does not say what permitted it: %q", a.By)
	}
	// AND IT EXPIRES. A grant given before the run does not become a standing
	// permission inside it.
	if !a.Expires.After(dpLooked) || a.Expires.After(dpLooked.Add(time.Minute)) {
		t.Errorf("the authorisation expires at %s, which is not shortly after the "+
			"instant it was given", a.Expires)
	}
	g.scenes.over()

	// ── AN INTERACTION IT WAS NOT GIVEN FOR: refused.
	scenes.aim(2, "apply changes", 1)
	if _, ok := g.Authorise(context.Background(),
		workflowrun.Intent{Step: 1, Epoch: 4, Generation: 1}); ok {
		t.Error("a press nobody authorised was permitted")
	}
	g.scenes.over()

	// ── AND A PRESS NOTHING AIMED AT ALL: refused, which is the case that can
	//    actually arise.
	if _, ok := g.Authorise(context.Background(),
		workflowrun.Intent{Step: 1, Epoch: 4, Generation: 1}); ok {
		t.Error("a press of an interaction the play did not select was permitted")
	}
	if g.granted != 1 || g.refused != 2 {
		t.Errorf("the granter permitted %d and refused %d", g.granted, g.refused)
	}
	// AND IT SAYS SO WHERE A PERSON READS IT. This is the loudest line in the product:
	// Marco tried to do something nobody agreed to.
	if n := strings.Count(out.String(), "REFUSED"); n != 2 {
		t.Errorf("the transcript says REFUSED %d time(s):\n%s", n, out)
	}
	if !strings.Contains(out.String(), "Nothing was sent") {
		t.Errorf("the refusal does not say that nothing was sent:\n%s", out)
	}
}

// AND A GRANT FOR SOMETHING ELSE STOPS THE COMMAND BEFORE IT OBSERVES ANYTHING.
//
// `agrees` is tested on its own above; this is the call site. Without it a run would perform an
// Act under an authorisation given for a different one — the exact failure the type exists to
// prevent — and every other test in this file would still pass, because they all hand over a
// grant that matches.
//
// Exit 1 and not 2: the command was used correctly and the thing it was pointed at cannot be
// performed. And nothing is observed — there is no stream at all, because the refusal happens
// before the run's own account is opened.
func TestAGrantForAnotherActStopsTheCommand(t *testing.T) {
	name := anActOverAStatedWindow(t)
	g := agGrantGiven(t, "an Act nobody composed", "the control in front")
	path := filepath.Join(t.TempDir(), "events.jsonl")

	out := &liveOut{}
	code := 0
	captureStdoutInto(t, out, func() int {
		code = runPerformWorkflow([]string{
			"--play", name, "--granted", agWritten(t, g), "--foreground",
			"--seconds", "3", "--events", path})
		return code
	})
	if code != 1 {
		t.Errorf("a grant for another Act exited %d", code)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("the run opened its account before checking what authorised it")
	}
}

// AND AN ACT REPLACED BETWEEN PLAY AND STARTUP IS REFUSED BEFORE ANY INPUT.
//
// The whole window, end to end through the command: an Act composed, an authorisation written
// for it, the Act composed AGAIN over the same demonstration with the same names on the other
// interactions, and then the run started with the authorisation that was given. Nothing is
// observed and nothing is pressed — the refusal happens before the run opens its own account.
//
// This is the case the Architect's finding named, and before the mapping was pinned it ran: the
// revision, the names, the order and the play's digest were all unchanged by the recompose.
func TestAnActReplacedBetweenPlayAndStartupIsRefused(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_ROUTES", filepath.Join(home, "routes"))
	art := dpATwoTapPanel()
	store, err := demo.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}
	// AND HOW AN ACT GETS REPLACED, which is worth being exact about.
	//
	// `demonstrated.Save` REFUSES to write over a play that exists — "give this one a
	// different name rather than replacing it" — so an Act cannot be recomposed in
	// place, and that narrows this window a long way. What it does not do is close it:
	// the pair can be removed and composed again, which is two commands, and the
	// bindings are a file somebody can edit while the play's digest stays what it was.
	// So this does it the way a person would.
	rt := routes.Route{Slug: routes.Slug("two-taps")}
	compose := func(picked []demonstrated.Named) {
		_ = os.Remove(routesHere().Path(rt))
		_ = os.Remove(demonstrated.BindingsPath(routesHere(), rt))
		src, binds, cerr := demonstrated.Compose(art, "two-taps", picked, dpLooked,
			marcoChecks{})
		if cerr != nil {
			t.Fatal(cerr)
		}
		if serr := demonstrated.Save(routesHere(), rt, src, binds,
			dpLooked); serr != nil {
			t.Fatal(serr)
		}
	}
	// ── AS IT WAS WHEN SOMEBODY PRESSED PLAY.
	compose([]demonstrated.Named{
		{Position: 0, Name: "the graphics tab"},
		{Position: 1, Name: "apply changes"},
	})
	plan, perr := planToPerform("two-taps")
	if perr != 0 {
		t.Fatalf("the Act as composed does not preflight (exit %d)", perr)
	}
	g := agGrantForPlan(t, "two-taps", plan)
	granted := agWritten(t, g)

	// ── AND AS IT IS NOW: the same names, the other interactions.
	compose([]demonstrated.Named{
		{Position: 1, Name: "the graphics tab"},
		{Position: 0, Name: "apply changes"},
	})

	path := filepath.Join(t.TempDir(), "events.jsonl")
	out := &liveOut{}
	code := 0
	captureStdoutInto(t, out, func() int {
		code = runPerformWorkflow([]string{
			"--play", "two-taps", "--granted", granted, "--foreground",
			"--seconds", "3", "--events", path})
		return code
	})
	if code != 1 {
		t.Errorf("an Act replaced since it was authorised exited %d:\n%s", code, out)
	}
	if _, serr := os.Stat(path); serr == nil {
		t.Error("the run began observing before checking what it was authorised to do")
	}
	// AND NOTHING WAS PERFORMED AND NOTHING KEPT. A refusal this early has no record
	// either: the reference is minted after the authorisation is checked, which is the
	// order that makes "no input was sent" a claim about a run that never started.
	if runs, _ := os.ReadDir(filepath.Join(home, "runs")); len(runs) != 0 {
		t.Errorf("%d run record(s) were written for a run that was refused before "+
			"it began", len(runs))
	}
	// THE SENTENCE ITSELF goes to stderr, which this harness does not capture; it is
	// asserted word for word against the production `agrees` in
	// TestTheSameNamesBoundToDifferentInteractionsAreRefused.
}

// ── AND THE GATES BETWEEN INTERACTIONS ARE UNTOUCHED ────────────────────────

// A GRANTED RUN STILL WAITS FOR EACH INTERACTION'S OUTCOME BEFORE THE NEXT.
//
// # Why this is the test that says the contract is safe
//
// Because the fear about removing the questions is that they were load-bearing. They were not:
// the thing that stops Marco pressing the second control before the first one has been shown to
// have worked is ADR-242's outcome gate, in the coordinator, which asks nobody anything. So the
// same two-interaction Act is driven through the production coordinator, verifier and runtime
// with scripted looks — the real everything except the pixels — and permitted by the real
// `grantedGranter` reading a real grant.
//
// What must hold: both presses happen, in the play's order, each authorised by the grant, with
// the first interaction's VERDICT arriving before the second is ever authorised.
func TestAGrantedRunStillWaitsForEachOutcome(t *testing.T) {
	art := dpATwoTapPanel()
	now := dpLooked
	inOrder := []demonstrated.Named{
		{Position: 0, Name: "the graphics tab"},
		{Position: 1, Name: "apply changes"},
	}
	saved := dpSave(t, art, inOrder)
	src, binds := saved.dpReload(t)
	plan, err := demonstrated.Preflight(src, binds, dpAStore{art.Name: art},
		marcoChecks{})
	if err != nil {
		t.Fatalf("the saved pair did not preflight: %v", err)
	}
	// THE GRANT, OVER EXACTLY WHAT THE PLAY PERFORMS — and checked, as the command
	// checks it, rather than assumed to match.
	g := agGrantForPlan(t, "two-taps", plan)
	if aerr := g.agrees("two-taps", plan); aerr != nil {
		t.Fatalf("the grant this test gives does not match the play: %v", aerr)
	}

	path := filepath.Join(t.TempDir(), "events.jsonl")
	say, werr := runstate.NewWriter(path, dpTheRun, "two-taps", art.Name, false)
	if werr != nil {
		t.Fatal(werr)
	}
	scenes := actInteractions(art, len(plan.Order))
	out := &strings.Builder{}
	byGrant := &grantedGranter{grant: g, out: out, live: false,
		life: 10 * time.Second, now: func() time.Time { return now },
		events: say, scenes: scenes}

	st := dpAPanel(dpPanelWords(dpFirstOutcome()...), dpPanelWords(dpSecondOutcome()...))
	run, bind, _, book := dpARunUnder(t, art, &now, byGrant)
	st.book, st.now = book, &now
	sup := demonstrated.NewSupervised(run, bind, st.look, 120).
		WithWatch(func(c context.Context, a workflowrun.Attempt) (bool, string) {
			sayWhatTheCoordinatorDecided(say, scenes, a)
			return true, ""
		})
	acc, perr := performDemonstratedPlay(context.Background(), dpTheRun, plan, sup,
		scenes, out)
	say.Close()
	if perr != nil {
		t.Fatalf("the play did not run: %v\n%s", perr, out)
	}
	if got := acc.Outcome(); got != runref.Verified {
		t.Fatalf("a granted two-interaction run ended %q: %+v\n%s", got, acc.Asked(),
			out)
	}

	// ── BOTH PRESSES PERMITTED BY THE GRANT, AND NONE REFUSED.
	if byGrant.granted != 2 || byGrant.refused != 0 {
		t.Fatalf("the authorisation permitted %d press(es) and refused %d:\n%s",
			byGrant.granted, byGrant.refused, out)
	}
	events, rerr := runstate.Read(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	// ── AND NOTHING WAS ASKED ALONG THE WAY.
	for i, e := range events {
		if e.Question != nil {
			t.Fatalf("event %d asks %q", i, e.Question.ID)
		}
	}
	// ── AND THE SECOND WAS NOT AUTHORISED UNTIL THE FIRST HAD A VERDICT.
	//
	// Read off the stream in order: the first `acting` naming interaction 2 must come
	// after a `verifying` that names interaction 1 WITH a verdict. This is ADR-242,
	// restated for the contract that no longer asks — the gate is the coordinator's and
	// it never depended on a keypress.
	verdictOnFirst, actedOnSecond := -1, -1
	for i, e := range events {
		at := e.Interaction
		if at == nil {
			continue
		}
		if at.Index == 1 && e.Phase == runstate.Verifying && at.Verdict != "" &&
			verdictOnFirst < 0 {
			verdictOnFirst = i
		}
		if at.Index == 2 && e.Phase == runstate.Acting && actedOnSecond < 0 {
			actedOnSecond = i
		}
	}
	if verdictOnFirst < 0 {
		t.Fatalf("the first interaction never got a verdict: %v", phasesIn(events))
	}
	if actedOnSecond < 0 {
		t.Fatalf("the second interaction was never pressed: %v", phasesIn(events))
	}
	if actedOnSecond < verdictOnFirst {
		t.Errorf("the second interaction was pressed at %d, before the first one's "+
			"verdict at %d — the outcome gate is not holding under a grant",
			actedOnSecond, verdictOnFirst)
	}
	// AND THE TRANSCRIPT SAYS WHAT PERMITTED EACH OF THEM, in the person's own words
	// for the interaction and the grant's for the authority.
	for _, says := range []string{"1 of 2 · the graphics tab",
		"2 of 2 · apply changes", "pressed Play"} {
		if !strings.Contains(out.String(), says) {
			t.Errorf("the transcript does not say %q:\n%s", says, out)
		}
	}
}

// ── AND THROUGH THE PRODUCTION COMMAND, THE QUESTIONS ARE GONE ──────────────

// A GRANTED RUN PUTS NO QUESTION TO ANYBODY.
//
// The whole claim, through `runPerformWorkflow` over a real Act, with a key hook nobody touches.
// Four things must hold at once, and the first two are what the owner was handed a checklist for:
//
//	nothing is asked — no start prompt, no screen question, no press authorisation
//	the run says `starting` straight away, because preparation was pressing Play
//	nothing on the terminal invites a keypress
//	and when it cannot establish the screen it PAUSES, saying what it is looking at
//
// The last one is the honest half of removing the questions. This Act's demonstration was
// recorded from one look at the window in front and its screens carry no words, so the evidence
// `workflowstore.Standing` needs is absent — which is exactly a case a person must be told
// about rather than asked about.
func TestAGrantedRunAsksNobodyAnything(t *testing.T) {
	name := anActOverAStatedWindow(t)

	// A KEY HOOK THAT NOBODY PRESSES. Installed because Studio passes --hotkeys so
	// that F9 still stops a run; what it must NOT do is answer anything.
	src, _ := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	was := newKeySource
	t.Cleanup(func() { newKeySource = was })
	newKeySource = func() (*navsource.Source, string) { return src, "" }

	g := agGrantFor(t, name)
	path := filepath.Join(t.TempDir(), "events.jsonl")
	out := &liveOut{}
	captureStdoutInto(t, out, func() int {
		return runPerformWorkflow([]string{
			"--play", name, "--granted", agWritten(t, g), "--foreground",
			"--hotkeys", "--seconds", "3", "--patience", "40",
			"--dwell", "0s", "--events", path})
	})
	events, err := runstate.Read(path)
	if err != nil || len(events) == 0 {
		t.Fatalf("no events were written: %v", err)
	}

	// ── 1. NOTHING WAS ASKED.
	for i, e := range events {
		if e.Question != nil {
			t.Fatalf("event %d of a granted run asks %q (%s)", i,
				e.Question.ID, e.Question.Asks)
		}
		if e.Phase == runstate.Asking || e.Phase == runstate.Naming ||
			e.Phase == runstate.Agreeing || e.Phase == runstate.Preparing {
			t.Errorf("event %d of a granted run is in phase %q", i, e.Phase)
		}
	}
	// ── 2. AND IT STARTED, rather than preparing.
	if events[0].Phase != runstate.Starting {
		t.Errorf("a granted run begins in phase %q, not `starting`: %v",
			events[0].Phase, phasesIn(events))
	}
	// ── 3. AND THE TERMINAL INVITED NO KEYPRESS.
	said := out.String()
	for _, gone := range []string{"press F7", "Prepare the application",
		"Which saved screen are you looking at", "May Marco",
		"Do you agree this is a step"} {
		if strings.Contains(said, gone) {
			t.Errorf("a granted run still says %q:\n%s", gone, said)
		}
	}
	// AND IT SAYS WHAT IT IS PERFORMING AND UNDER WHOSE AUTHORITY.
	if !strings.Contains(said, "pressed Play") {
		t.Errorf("the run does not say what authorised it:\n%s", said)
	}
	// ── 4. AND THE RECORD SAYS WHAT AUTHORISED IT.
	//
	// The first question anybody asks about a press that should not have happened, and
	// the record is what a bug report cites. A run under the keyed contract leaves this
	// empty and its attempts say who pressed what; a granted run has one answer for the
	// whole run and this is where it lives.
	if rec := theOnlyRecord(t); !strings.Contains(rec.Authority, "pressed Play") {
		t.Errorf("the record does not say what authorised the run: %q", rec.Authority)
	}
	// ── 5. AND IT PAUSED, saying what it could not establish.
	paused := ""
	for _, e := range events {
		if e.Phase == runstate.Paused {
			paused = e.Reason
		}
	}
	if paused == "" {
		t.Fatalf("the run could not establish its screen and never said so: %v",
			phasesIn(events))
	}
	if !strings.Contains(paused, "screen") {
		t.Errorf("the pause does not say what is wrong in words a person could act "+
			"on: %q", paused)
	}
}
