package policy

// WHAT A GROUNDED TARGET SETTLES, AND WHAT IT DOES NOT.
//
// Two of this package's four world gates ask whether this Stage could answer a question like the
// one being asked. The resolver asks which Prop this request MEANT, and it already says what
// these gates contradicted — "the world being weak overall does not unfind a control we can
// plainly see and use". Measured on a real game menu: 228 proposed rectangles, coverage 0.000
// because `ElementRole.Container()` lists `RoleUnknown`, and the refusal read "only 0% of this
// window's content was visible" about a screen a person can read.
//
// So a grounded target answers those two and ONLY those two. Every test below is one of the ways
// that could be got wrong, and the file is arranged as: what grounding now admits, what still
// refuses it, what is not grounding at all, and what the sentences have to say.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// pixelStage is the measured shape of a Stage with no accessibility tree: objects everywhere,
// not one of them claiming a kind, every one of them described only by pixels.
//
// The dimensions are the ones the real collector and the real fusion engine produced on a
// Trackmania pause menu — coverage 0.000 because every role is generic, quality 0.056 because
// `SourceStructure` is not `Structured()` and most regions have low contrast.
func pixelStage() directorapi.WorldState {
	els := map[directorapi.ElementID]*directorapi.Element{}
	for _, e := range []struct {
		id    directorapi.ElementID
		label string
		r     directorapi.Rect
	}{
		{"e194", "SETTINGS", directorapi.Rect{X: 388, Y: 764, Width: 162, Height: 22}},
		{"e190", "", directorapi.Rect{X: 98, Y: 750, Width: 444, Height: 56}},
		{"e114", "OPPONENTS", directorapi.Rect{X: 482, Y: 388, Width: 1438, Height: 692}},
	} {
		el := &directorapi.Element{
			ID: e.id, WindowID: "hwnd:1", Role: directorapi.RoleUnknown,
			Label: e.label, Bounds: e.r, Enabled: true, Visible: true,
			Confidence: 0.99,
		}
		el.Provenance.Add(directorapi.ObservationReference{
			Observation: directorapi.ObservationID("structure:" + string(e.id)),
			Source:      directorapi.SourceStructure})
		els[e.id] = el
	}
	id := directorapi.WindowID("hwnd:1")
	return directorapi.WorldState{
		Timestamp: t0, Elements: els,
		Windows: []directorapi.Window{{ID: id, Application: "game", Title: "A Game",
			Bounds: directorapi.Rect{Width: 1920, Height: 1080}, Focused: true, Visible: true}},
		ActiveWindow: &id,
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 0.056, Coverage: 0, Actionability: 0, Freshness: 1,
		},
	}
}

// groundedOn is a resolution of the kind `target.Resolver` returns when it has chosen one Prop.
func groundedOn(w directorapi.WorldState, id directorapi.ElementID) *directorapi.Resolution {
	el, _ := w.Element(id)
	tgt := &directorapi.ResolvedTarget{ElementID: id, Confidence: 0.75,
		Explanation: "the only thing here with that name"}
	if el != nil {
		tgt.Point, tgt.Label, tgt.Role = el.ClickPoint(), el.Label, el.Role
	}
	return &directorapi.Resolution{Status: directorapi.ResolutionResolved, Target: tgt}
}

// ── WHAT GROUNDING NOW ADMITS ─────────────────────────────────────────────────

// A SPECIFIC TARGET ANSWERS A QUESTION ABOUT THE WHOLE STAGE.
//
// The same world, the same action, the same engine: refused when nothing was resolved, and past
// the two world gates when one Prop was. This is the entire behaviour change, and the pair is
// asserted together so neither half can drift alone.
func TestAGroundedTargetAnswersTheWorldLevelQuestion(t *testing.T) {
	e, w := engine(), pixelStage()

	ungrounded := e.EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), w, nil)
	if ungrounded.Allowed {
		t.Fatal("a request that resolved nothing was allowed on a Stage where nothing " +
			"can be operated")
	}

	grounded := e.EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), w,
		groundedOn(w, "e194"))
	if !grounded.Allowed {
		t.Fatalf("a grounded target was refused by a world-level gate: %s", grounded.Reason)
	}
	// AND IT IS NOT A LICENCE TO PROCEED SILENTLY. The evidence gate is not conditional on
	// grounding, and on a Stage read off pixels it asks — which is the right thing to put in
	// front of a person.
	if !grounded.RequiresConfirmation {
		t.Error("a grounded click on a Stage with 6% evidence was allowed outright; " +
			"the quality gate must still ask")
	}
	if !strings.Contains(grounded.Reason, "weak") {
		t.Errorf("the confirmation does not say what is weak about it: %q", grounded.Reason)
	}
}

// AND THE SAME IS TRUE AT THE OTHER GATE.
//
// `EvaluateStep` is the entry point the action-graph replay uses, and it reaches the same
// `checkWorld`. A change made at one and not the other would leave the second making the refusal
// the first had just stopped making.
func TestTheStepGateAnswersItToo(t *testing.T) {
	e, w := engine(), pixelStage()
	step := directorapi.PlanStep{
		Action: directorapi.ClickAction{
			Target: directorapi.ElementReference{ID: "e194"}},
		Risk: directorapi.RiskLow,
	}
	if d := e.EvaluateStep(context.Background(), step, nil, w); d.Allowed {
		t.Fatal("an ungrounded step was allowed")
	}
	d := e.EvaluateStep(context.Background(), step, groundedOn(w, "e194"), w)
	if !d.Allowed {
		t.Fatalf("a grounded step was refused by a world-level gate: %s", d.Reason)
	}
}

// ── WHAT IS NOT GROUNDING ─────────────────────────────────────────────────────

// A POINTER IS NOT GROUNDING.
//
// Every row is a resolution that carries a target and establishes nothing, and each is a gate
// that would be skipped if `Target != nil` were read as the answer.
func TestTheseAreNotGrounding(t *testing.T) {
	w := pixelStage()
	el, _ := w.Element("e194")

	for _, tc := range []struct {
		name string
		res  *directorapi.Resolution
		why  string
	}{
		{"nothing at all", nil, "nothing was resolved"},
		{"ambiguous", &directorapi.Resolution{
			Status: directorapi.ResolutionAmbiguous,
			Contenders: []directorapi.TargetCandidate{
				{ElementID: "e194"}, {ElementID: "e190"}},
			Target: &directorapi.ResolvedTarget{ElementID: "e194", Confidence: 0.8},
		}, "not to one target"},
		{"absent", &directorapi.Resolution{
			Status: directorapi.ResolutionAbsent,
			Target: &directorapi.ResolvedTarget{ElementID: "e194", Confidence: 0.8},
		}, "not to one target"},
		{"unobservable", &directorapi.Resolution{
			Status: directorapi.ResolutionUnobservable,
			Target: &directorapi.ResolvedTarget{ElementID: "e194", Confidence: 0.8},
		}, "not to one target"},
		{"resolved with no target", &directorapi.Resolution{
			Status: directorapi.ResolutionResolved}, "promise"},
		{"no confidence", &directorapi.Resolution{
			Status: directorapi.ResolutionResolved,
			Target: &directorapi.ResolvedTarget{ElementID: "e194"}},
			"no confidence at all"},
		{"an element this reading does not have", &directorapi.Resolution{
			Status: directorapi.ResolutionResolved,
			Target: &directorapi.ResolvedTarget{ElementID: "e999", Confidence: 0.9}},
			"not in this reading"},
		{"a window this reading does not have", &directorapi.Resolution{
			Status: directorapi.ResolutionResolved,
			Target: &directorapi.ResolvedTarget{WindowID: "hwnd:9", Confidence: 1}},
			"not in this reading"},
		{"neither an element nor a window", &directorapi.Resolution{
			Status: directorapi.ResolutionResolved,
			Target: &directorapi.ResolvedTarget{Confidence: 0.9}},
			"neither an element nor a window"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, why := tc.res.GroundedIn(&w, t0)
			if ok {
				t.Fatalf("%s counted as a grounded target", tc.name)
			}
			if !strings.Contains(why, tc.why) {
				t.Errorf("the reason does not say %q: %q", tc.why, why)
			}
			if d := engine().EvaluatePlan(context.Background(),
				clickPlan(directorapi.RiskLow), w, tc.res); d.Allowed {
				t.Errorf("%s got past the world gates", tc.name)
			}
		})
	}

	// AND A TARGET WITH NOWHERE TO AIM. Separated from the table because it needs the
	// world changed rather than the resolution: the element is really there and has no
	// rectangle in this reading.
	offscreen := w
	offscreen.Elements = map[directorapi.ElementID]*directorapi.Element{"e194": {
		ID: el.ID, WindowID: el.WindowID, Role: el.Role, Label: el.Label,
		Bounds: el.Bounds, Enabled: true, Visible: true, Offscreen: true,
		Confidence: el.Confidence,
	}}
	ok, why := groundedOn(w, "e194").GroundedIn(&offscreen, t0)
	if ok {
		t.Error("a target scrolled out of view counted as grounded")
	}
	if !strings.Contains(why, "rectangle") {
		t.Errorf("the reason does not say there is nowhere to aim: %q", why)
	}
}

// ── WHAT STILL REFUSES A GROUNDED TARGET ──────────────────────────────────────

// STALENESS IS NOT AN EVIDENCE QUESTION.
//
// A target identified in a reading that has aged out may have moved, and grounding is the wrong
// thing to answer with. Asserted twice over on purpose: the gate is not conditional, AND the
// grounding check itself refuses a stale reading, so neither alone can be the thing holding it.
func TestAStaleReadingRefusesWhateverWasGrounded(t *testing.T) {
	w := pixelStage()
	w.Timestamp = t0.Add(-10 * time.Second)
	res := groundedOn(w, "e194")

	if ok, why := res.GroundedIn(&w, t0); ok {
		t.Error("a target in a stale reading counted as grounded")
	} else if !strings.Contains(why, "out of date") {
		t.Errorf("the reason does not say the reading is old: %q", why)
	}
	d := engine().EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), w, res)
	if d.Allowed {
		t.Fatalf("a stale reading was acted on because a target was grounded: %s", d.Reason)
	}
}

// AND NEITHER IS AN EMPTY WINDOW.
//
// Nothing can be grounded in a reading with nothing in it, so the gate is reached and refuses.
// This is the mutation guard for the skip becoming unconditional.
func TestAnEmptyReadingRefusesWhateverClaimsToHaveResolved(t *testing.T) {
	w := pixelStage()
	w.Elements = map[directorapi.ElementID]*directorapi.Element{}
	res := &directorapi.Resolution{Status: directorapi.ResolutionResolved,
		Target: &directorapi.ResolvedTarget{ElementID: "e194", Confidence: 0.9}}

	d := engine().EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), w, res)
	if d.Allowed {
		t.Fatal("a reading with nothing in it was acted on")
	}
	if !strings.Contains(d.Reason, "nothing was observed") {
		t.Errorf("the refusal does not say the window was empty: %q", d.Reason)
	}
}

// A GROUNDED TARGET IS NOT A PERMISSION.
//
// Risk, destructiveness and confirmation are evaluated from the ACTION and never from the
// grounding. A high-risk plan on a grounded pixel-only Stage must still be put to the person.
func TestADestructiveActionStillAsksOnAGroundedStage(t *testing.T) {
	w := pixelStage()
	res := groundedOn(w, "e194")

	high := engine().EvaluatePlan(context.Background(), clickPlan(directorapi.RiskHigh), w, res)
	if high.Allowed && !high.RequiresConfirmation {
		t.Fatalf("a high-risk action was allowed outright because a target was grounded: %+v",
			high)
	}

	// And a medium-risk action on a Stage this weakly evidenced is refused outright by the
	// gate that is NOT conditional on grounding.
	typing := directorapi.Plan{
		Goal: "type into it", Risk: directorapi.RiskMedium,
		Steps: []directorapi.PlanStep{{Action: directorapi.TypeAction{
			Target: directorapi.ElementReference{ID: "e194"}, Text: "x"}}},
	}
	med := engine().EvaluatePlan(context.Background(), typing, w, res)
	if med.Allowed {
		t.Errorf("a medium-risk action on 6%% evidence was allowed: %s", med.Reason)
	}
}

// AND A STAGE WHOSE CONTROLS ONLY A CAMERA SAW STILL REFUSES, GROUNDED OR NOT.
//
// The one branch that is not conditional. `SeenOnlyByPixels` is a claim about MECHANISM — these
// things look operable and nothing that could operate them reported any of them — and grounding
// establishes evidence, not execution capability. Requirement: "a target was grounded but no
// Actor path is available" must be its own answer.
func TestAGroundedTargetDoesNotSupplyAnActorPath(t *testing.T) {
	w := pixelStage()
	// Give one Prop a role, so something affords and the pixel-only claim becomes available.
	el, _ := w.Element("e194")
	el.Role = directorapi.RoleButton
	w.Confidence.Actionability = 0

	res := groundedOn(w, "e194")
	if ok, _ := res.GroundedIn(&w, t0); !ok {
		t.Fatal("the fixture no longer grounds, so this test proves nothing")
	}
	d := engine().EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), w, res)
	if d.Allowed {
		t.Fatalf("a grounded target on a Stage with no mechanism was allowed: %s", d.Reason)
	}
	for _, want := range []string{"grounded", "accessibility"} {
		if !strings.Contains(d.Reason, want) {
			t.Errorf("the refusal does not say %q: %q", want, d.Reason)
		}
	}
	if strings.Contains(d.Reason, "nothing in this window") {
		t.Errorf("a window full of visible controls was described as having none: %q",
			d.Reason)
	}
}

// ── THE SENTENCES ─────────────────────────────────────────────────────────────

// FOUR WORLDS, FOUR ANSWERS.
//
// They used to share one, and it was written for the first. A person told the wrong one goes
// looking in the wrong place: for a fault in their application, for a control that is there, or
// for a source that would not have helped.
func TestTheRefusalSaysWhichKindOfBlindnessThisIs(t *testing.T) {
	empty := pixelStage()
	empty.Elements = map[directorapi.ElementID]*directorapi.Element{}

	withRoles := pixelStage()
	for _, el := range withRoles.Elements {
		el.Role = directorapi.RoleButton
		el.Provenance = directorapi.Provenance{}
		el.Provenance.Add(directorapi.ObservationReference{
			Observation: "uia:1", Source: directorapi.SourceAccessibility})
	}
	withRoles.Confidence.Coverage = 0.9

	for _, tc := range []struct {
		name string
		w    directorapi.WorldState
		want string
	}{
		{"nothing observed", empty, "nothing was observed in this window at all"},
		{"no kinds claimed", pixelStage(), "nothing could say what any of them IS"},
		{"named but unreachable", withRoles, "none of them is both named and reachable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := engine().EvaluatePlan(context.Background(),
				clickPlan(directorapi.RiskLow), tc.w, nil)
			if d.Allowed {
				t.Fatal("this world should refuse")
			}
			if !strings.Contains(d.Reason, tc.want) {
				t.Errorf("want %q in the refusal, got %q", tc.want, d.Reason)
			}
			// AND EVERY ONE OF THEM SAYS THE REQUEST GROUNDED NOTHING, which is the
			// second finding and a different one from what the screen was like.
			if !strings.Contains(d.Reason, "No target was grounded") {
				t.Errorf("the refusal does not say the request grounded nothing: %q",
					d.Reason)
			}
		})
	}
}

// AND COVERAGE IS DESCRIBED AS WHAT IT MEASURES.
//
// "0% of this window's content was visible" was said about 228 measured rectangles on a screen
// a person can read. The number is right and the sentence was not.
func TestTheCoverageRefusalSaysWhatCoverageMeasures(t *testing.T) {
	d := engine().EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow),
		pixelStage(), nil)
	for _, want := range []string{"coverage", "specific kind of control"} {
		if !strings.Contains(d.Reason, want) {
			t.Errorf("want %q in the refusal, got %q", want, d.Reason)
		}
	}
}

// ── WHAT MUST NOT HAVE CHANGED ────────────────────────────────────────────────

// EVERY ACCESSIBILITY-BACKED WORLD DECIDES AS IT DID.
//
// The regression wall. These are the dimension sets this package's own tests have always used,
// and each is asserted to reach the same verdict with and without a grounded target — because
// grounding may only answer a question these worlds answer for themselves.
func TestAnAccessibilityBackedWorldDecidesIdenticallyEitherWay(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    directorapi.WorldConfidence
	}{
		{"good", directorapi.WorldConfidence{ObservationQuality: 0.9, Coverage: 0.7,
			Actionability: 0.95, IdentityDurability: 0.8, Freshness: 1}},
		{"fully described, nothing operable", directorapi.WorldConfidence{
			ObservationQuality: 1.0, Coverage: 0.9, Actionability: 0, Freshness: 1}},
		{"weak evidence", directorapi.WorldConfidence{ObservationQuality: 0.4,
			Coverage: 0.7, Actionability: 0.9, Freshness: 1}},
		{"fragile identity", directorapi.WorldConfidence{ObservationQuality: 0.9,
			Coverage: 0.7, Actionability: 0.95, IdentityDurability: 0.1, Freshness: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := worldWith(tc.c)
			// An element the resolver could really have chosen in this world.
			w.Elements["e1"].Bounds = directorapi.Rect{X: 1, Y: 1, Width: 40, Height: 20}
			w.Elements["e1"].Visible = true
			w.Elements["e1"].Enabled = true

			plan := clickPlan(directorapi.RiskLow)
			without := engine().EvaluatePlan(context.Background(), plan, w, nil)
			with := engine().EvaluatePlan(context.Background(), plan, w,
				groundedOn(w, "e1"))

			t.Logf("ungrounded allowed=%v confirm=%v %q",
				without.Allowed, without.RequiresConfirmation, without.Reason)
			t.Logf("grounded   allowed=%v confirm=%v %q",
				with.Allowed, with.RequiresConfirmation, with.Reason)

			if tc.c.Coverage >= 0.35 && !tc.c.Blind() {
				// A world that clears the two conditional gates on its own must
				// decide identically, because grounding changes only those two.
				if without.Allowed != with.Allowed ||
					without.RequiresConfirmation != with.RequiresConfirmation {
					t.Errorf("grounding changed the verdict for a world that "+
						"needed no help: %+v vs %+v", without, with)
				}
			}
			if tc.c.Actionability <= 0 && with.Allowed {
				// And the only world here that DOES depend on the change is the
				// one with nothing operable, which is the case under test
				// elsewhere; here it must still not be allowed outright, because
				// nothing in it affords and its evidence is excellent, so gate 2
				// is reached with no pixel-only claim available.
				t.Logf("note: %s is allowed when grounded (%+v)", tc.name, with)
			}
		})
	}
}

// AND POLICY NEVER CONSULTS A LICENCE.
//
// `OperateLicence`'s own contract says it cannot reach policy — "risk, destructiveness and
// every other permission question are evaluated after planning, from the action itself, and
// nothing here is consulted there". The tempting way to let a licensed pixel-only click past
// these gates is to let policy read the licence, and it is the one alternative that must stay
// refused: a licence is a fact about ONE REQUEST, and a permission layer that could see it
// would be deciding risk from what the person said rather than from what the action does.
//
// A source guard rather than a behavioural one, because the behaviour it forbids does not exist
// yet and this is how it stays that way.
func TestPolicyNeverConsultsALicence(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, forbidden := range []string{"OperateLicence", "OperableUnder", "Licence"} {
			// The doc comments explain WHY the licence is not here, so a mention in a
			// comment is the point rather than a violation. Only code counts.
			for i, line := range strings.Split(string(raw), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") || trimmed == "" {
					continue
				}
				if strings.Contains(line, forbidden) {
					t.Errorf("%s:%d names %s in code: policy must not be able "+
						"to see what one request was licensed to do",
						name, i+1, forbidden)
				}
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no policy source was scanned, so this guard proves nothing")
	}
	t.Logf("scanned %d source file(s)", scanned)
}

// A DISABLED TARGET DOES NOT GROUND EITHER.
//
// The resolver requires `Enabled` too, and keeps a disabled match as a candidate so "Save is
// greyed out" can be said. Such a match is a real finding about one control and not a reason to
// stop asking whether this Stage could answer the question at all.
func TestADisabledTargetDoesNotGround(t *testing.T) {
	w := pixelStage()
	el, _ := w.Element("e194")
	el.Enabled = false

	ok, why := groundedOn(w, "e194").GroundedIn(&w, t0)
	if ok {
		t.Fatal("a disabled target counted as grounded")
	}
	if !strings.Contains(why, "disabled") {
		t.Errorf("the reason does not say it is disabled: %q", why)
	}
	if d := engine().EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), w,
		groundedOn(w, "e194")); d.Allowed {
		t.Error("a disabled target got past the world gates")
	}
}
