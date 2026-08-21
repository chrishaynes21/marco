package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The proof moves forward with the walk.
//
// A two-edge play used to establish where Marco was standing FOUR times: once to plan, once before
// edge one, once again before edge two, and once more to confirm arrival — for three screens, two
// of which had just been positively verified moments earlier by the thing that would have changed
// them.
//
// This file holds the two places that redundancy was removed, and holds them by COUNTING the
// epistemic work rather than by reading a field. A handoff that is threaded but never consulted
// would satisfy any assertion about the value; only the count can tell whether the second look
// actually stopped happening.

// ── a desktop with three screens and a press between each ─────────────────────

// walkDesktop is the window, the eyes and the hands for a two-edge play.
//
// One object plays all three parts on purpose: what is on screen depends on what has been
// emitted, which is the only causal link a real desktop has and the one a scripted sampler cannot
// express. It also counts what it was asked, which is what the tests below are actually about.
type walkDesktop struct {
	mu       sync.Mutex
	emitted  int  // programs the runner accepted
	acquires int  // times the window was looked up
	samples  int  // times the screen was read
	stall    bool // the screen never changes, so nothing can verify
}

// screens is the desktop this play crosses, in order.
var walkScreens = []string{"a", "b", "c"}

func (w *walkDesktop) Run(context.Context, string, string) (directorapi.MarcoResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emitted++
	return directorapi.MarcoResult{}, nil
}

func (w *walkDesktop) Acquire(context.Context, windowref.Selector) (windowref.Ref, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.acquires++
	return windowref.Ref{
		ID: "hwnd:100", Handle: 100, ProcessID: 7, Application: "testgame", Generation: 1,
	}, nil
}

func (w *walkDesktop) Sample(context.Context, observesession.SampleRequest) (
	observe.Sample, error) {

	w.mu.Lock()
	w.samples++
	at := w.emitted
	if w.stall {
		at = 0
	}
	w.mu.Unlock()
	if at >= len(walkScreens) {
		at = len(walkScreens) - 1
	}
	return walkSample(walkScreens[at]), nil
}

func (w *walkDesktop) counts() (emitted, acquires, samples int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.emitted, w.acquires, w.samples
}

// walkTerms is what distinguishes the three screens. The roles are identical — four buttons and
// an icon — so the interface terms are the whole of the identity, exactly as they are for the
// dry fixtures this borrows its shape from.
func walkTerms(screen string) []observe.InterfaceTerm {
	switch screen {
	case "a":
		return []observe.InterfaceTerm{observe.TermControls, observe.TermSettings}
	case "b":
		return []observe.InterfaceTerm{observe.TermAudio, observe.TermDisplay}
	}
	return []observe.InterfaceTerm{observe.TermInvite, observe.TermSocial}
}

func walkSample(screen string) observe.Sample {
	sh := &observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 800,
	}
	regions := []observe.ShadowRegion{{
		Role: "icon", Confidence: 0.5,
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}}
	regions = append(regions, dryRegions(0.414, 0.06)...)
	sh.Semantic = observe.SemanticEvidence{Observed: true, Terms: walkTerms(screen)}
	sh.Regions = regions
	sh.Detections = len(regions)
	sh.Roles = map[string]int{}
	for _, r := range regions {
		sh.Roles[r.Role]++
		if r.Nameable {
			sh.Nameable++
		}
	}
	return observe.Sample{
		WindowGeneration: 1,
		Frame:            observe.FrameSummary{Application: "testgame", Width: 1920, Height: 1080},
		Shadow:           sh,
	}
}

func walkSignature(screen string) observe.StructureSignature {
	return observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4, "icon": 1},
		Terms: walkTerms(screen), TermsKnown: true,
	}
}

// walkRuntime is a Director that can carry out a two-edge play against walkDesktop.
//
// It returns the three subject ids in route order, so a test names screens the way memory does
// rather than by guessing at an id.
func walkRuntime(t *testing.T, w *walkDesktop) (*Runtime, []string) {
	t.Helper()
	restoreClock := sessionClock
	sessionClock = newDryClock()
	t.Cleanup(func() { sessionClock = restoreClock })

	restoreLeads := windowLeads
	windowLeads = func(windowref.Ref) bool { return true }
	t.Cleanup(func() { windowLeads = restoreLeads })

	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	for _, screen := range walkScreens {
		if err := store.Remember("testgame", walkSignature(screen),
			observe.SemanticKnowledge{
				Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
			}); err != nil {
			t.Fatalf("seeding %s: %v", screen, err)
		}
	}
	ids := make([]string, len(walkScreens))
	for _, s := range store.Subjects() {
		for i, screen := range walkScreens {
			if len(s.Structure.Terms) > 0 &&
				s.Structure.Terms[0] == walkTerms(screen)[0] {
				ids[i] = s.ID
			}
		}
	}
	for i, id := range ids {
		if id == "" {
			t.Fatalf("screen %q was not remembered: %+v", walkScreens[i], store.Subjects())
		}
	}
	ev := observe.RelationshipEvidence{
		Observations: 2,
		Preceded:     map[observe.NavIntent]int{observe.NavConfirm: 2},
		Sequences: []observe.NavSequence{{
			Intents: []observe.NavIntent{observe.NavConfirm}, Count: 2,
		}},
	}
	for i := 0; i < 3; i++ {
		if _, err := store.RememberRelationships("testgame",
			[]observe.RelationshipObservation{
				{From: ids[0], To: ids[1], Evidence: ev},
				{From: ids[1], To: ids[2], Evidence: ev},
			}); err != nil {
			t.Fatalf("seeding the route: %v", err)
		}
	}
	// AND THE DEMONSTRATIONS THEMSELVES, which is what a rehearsal judgement is computed
	// from. A relationship says two screens are connected; a candidate says how somebody
	// crossed it, and `judgeNow` reads the second.
	for _, edge := range [][2]string{{ids[0], ids[1]}, {ids[1], ids[2]}} {
		if err := store.RememberCandidate("testgame", observe.ProcedureCandidate{
			Relationship: observe.RelationshipRef{From: edge[0], To: edge[1]},
			Application:  "testgame",
			Start:        observe.Checkpoint{Subject: edge[0], Verdict: observe.MatchSame},
			Steps: []observe.DemonstrationStep{{
				Intents: []observe.NavIntent{observe.NavConfirm},
				Arrived: observe.Checkpoint{
					Subject: edge[1], Verdict: observe.MatchSame},
			}},
			Complete: true, Reason: observe.ReasonArrived,
			Checkpoints: 2, Events: 1, Sequence: 1,
		}); err != nil {
			t.Fatalf("seeding the demonstration of %s: %v", edge[0], err)
		}
	}

	g := newObservationRegistry()
	g.memory = store
	g.lastTarget, g.lastSampler = w, w
	return &Runtime{observations: g, liveMarco: w}, ids
}

// walkPlan carries out the whole two-edge route and returns what the Director produced.
func walkPlan(t *testing.T, rt *Runtime, ids []string) (
	rehearse.StageEvidence, bool, service.PerformView) {

	t.Helper()
	steps := []observe.RelationshipRef{
		{From: ids[0], To: ids[1]},
		{From: ids[1], To: ids[2]},
	}
	var out service.PerformView
	final, ok := rt.performPlan(context.Background(), "testgame",
		rt.observations.memory.Topology("testgame"), steps, nil, &out)
	return final, ok, out
}

// ── the handoff ───────────────────────────────────────────────────────────────

// A VERIFIED OUTCOME BECOMES THE NEXT EDGE'S SOURCE.
//
// # Why this counts rather than reads
//
// Edge one ends by observing the screen after its action, resolving a Place from it, and checking
// that Place against what the edge said should happen. That IS edge two's source — established
// after the only thing that could have changed it, and checked. Looking again asks the identical
// question of the identical screen.
//
// Asserting that the value was passed would pass for a walker that accepts the argument and
// establishes anyway, which is the whole failure mode. So this counts window lookups, which are
// exact and deterministic: one per establishment, one per step. A two-edge, one-step-each route
// costs THREE with the handoff and FOUR without it, and the fourth is the second establishment.
//
// Deleting the handoff in performPlan must fail this. So must ignoring `carried` in Live.Perform.
func TestAVerifiedOutcomeBecomesTheNextEdgesSource(t *testing.T) {
	w := &walkDesktop{}
	rt, ids := walkRuntime(t, w)

	final, ok, out := walkPlan(t, rt, ids)
	if !ok {
		t.Fatalf("the two-edge route did not complete: refusal=%q say=%q steps=%+v",
			out.Refusal, out.Say, out.Steps)
	}
	if len(out.Steps) != 2 || !out.Steps[0].Verified || !out.Steps[1].Verified {
		t.Fatalf("both edges must be verified for this to be about anything: %+v", out.Steps)
	}

	// THE PROOF THE WALK ENDS HOLDING is the last edge's, not the first's.
	if final.Subject != ids[2] {
		t.Fatalf("the walk handed back %q as where it ended; the route finishes at %q",
			final.Subject, ids[2])
	}
	if final.From != rehearse.EvidenceVerifiedOutcome {
		t.Errorf("the proof is labelled %q", final.From)
	}

	emitted, acquires, samples := w.counts()
	if emitted != 2 {
		t.Fatalf("%d program(s) reached the desktop for a two-edge route", emitted)
	}

	// THE CALIBRATION: the same two edges, each establishing for itself.
	//
	// Not a number written down here. `performEdge` with nothing carried IS what the loop
	// did before the handoff, so this measures the old behaviour with the production code
	// rather than against a constant that would go stale the moment sampling changed.
	alone := &walkDesktop{}
	rtAlone, idsAlone := walkRuntime(t, alone)
	for _, edge := range []observe.RelationshipRef{
		{From: idsAlone[0], To: idsAlone[1]},
		{From: idsAlone[1], To: idsAlone[2]},
	} {
		step, _, err := rtAlone.performEdge(context.Background(), "testgame", edge, nil)
		if err != nil || !step.Verified {
			t.Fatalf("the calibration walk did not verify %+v: refusal=%q err=%v",
				edge, step.Refusal, err)
		}
	}
	_, wasAcquires, wasSamples := alone.counts()

	if samples >= wasSamples || acquires >= wasAcquires {
		t.Fatalf("carrying the proof forward cost %d screen reads and %d window lookups; "+
			"establishing separately cost %d and %d.\n"+
			"Edge two re-established a screen edge one had just positively verified — "+
			"the same question about the same screen, which is the redundant work "+
			"this handoff exists to remove.",
			samples, acquires, wasSamples, wasAcquires)
	}
}

// AND NOTHING IS CARRIED OUT OF A WALK THAT PROVED NOTHING.
//
// The negative control, and the one that matters: a proof handed forward from an edge that did not
// arrive would put the next edge to work on a screen nobody checked. Here the desktop never
// changes, so edge one cannot verify — and the walk must stop with no proof at all rather than
// offer where it was MEANT to be.
func TestAWalkThatProvedNothingCarriesNothing(t *testing.T) {
	w := &walkDesktop{stall: true}
	rt, ids := walkRuntime(t, w)

	final, ok, out := walkPlan(t, rt, ids)
	if ok {
		t.Fatal("a route whose first edge never verified reported that the whole thing worked")
	}
	if final.Subject != "" {
		t.Fatalf("an unverified walk handed back %q as proof of where Marco is standing",
			final.Subject)
	}
	if len(out.Steps) != 1 {
		t.Errorf("%d step(s) were attempted past an edge that was never verified",
			len(out.Steps))
	}
	// PREMISE: the edge was ATTEMPTED and did not arrive. Refused for want of evidence or
	// authority it would never have reached the walker, and this would be a test about a
	// fixture rather than about the handoff.
	if emitted, _, _ := w.counts(); emitted != 1 {
		t.Fatalf("%d program(s) reached the desktop; this case is about a step that was "+
			"taken and did not land, not one that was refused (%q)",
			emitted, out.Refusal)
	}
}

// ── arrival ───────────────────────────────────────────────────────────────────

// THE LAST EDGE'S PROOF ANSWERS THE ARRIVAL QUESTION.
//
// `confirmArrival` exists because a plan that ran is not a goal that was reached, and the only
// honest answer is a look. When the final edge already TOOK that look — after its action, and
// checked against what it expected — the extra one is the same question about the same screen.
//
// The negative half of this, and the serious one, is
// TestArrivalIsConfirmedByLookingNotByFinishing: without proof, arrival is decided by looking, and
// a look that cannot answer is not arrival.
func TestArrivalReusesTheProofTheLastEdgeProduced(t *testing.T) {
	w := &walkDesktop{}
	rt, ids := walkRuntime(t, w)

	final, ok, out := walkPlan(t, rt, ids)
	if !ok {
		t.Fatalf("the route did not complete: %q", out.Refusal)
	}
	_, before, _ := w.counts()

	rt.confirmArrival(context.Background(), "testgame", ids[2], final, &out)
	if !out.Arrived || out.To != ids[2] {
		t.Fatalf("the goal was reached and confirmArrival reports %+v", out)
	}
	if out.Say != "Done." {
		t.Errorf("it says %q", out.Say)
	}
	if _, after, _ := w.counts(); after != before {
		t.Errorf("confirming arrival cost %d further window lookup(s) on a screen the "+
			"last edge had just positively verified", after-before)
	}

	// AND A GOAL THE WALK DID NOT PROVE FALLS THROUGH TO LOOKING. Without this half the
	// test above passes for a function that agrees with whatever it is handed.
	var elsewhere service.PerformView
	rt.confirmArrival(context.Background(), "testgame", ids[0], final, &elsewhere)
	if elsewhere.Arrived {
		t.Fatalf("a walk that ended at %q reported arriving at %q because it was holding "+
			"a proof about somewhere else", ids[2], ids[0])
	}
}

// ── the planning look ─────────────────────────────────────────────────────────

// withHistory gives a runtime a window to find when the live desktop cannot answer.
//
// `performSelector` prefers the foreground and falls back to a window this Director has referred
// to before. A test has no real desktop, so the fallback is the reachable half — and it is the
// same selector the walk itself will use, which is the point.
func withHistory(t *testing.T, rt *Runtime) {
	t.Helper()
	g := rt.observations
	g.mu.Lock()
	g.finished = append(g.finished, observesession.Result{
		Session: observe.Session{
			Application: "testgame",
			Selector:    windowref.Selector{Application: "testgame"},
		},
	})
	g.mu.Unlock()
}

// THE LOOK THAT PLANNED THE ROUTE IS EDGE ONE'S SOURCE.
//
// # What used to happen, and why it was never wrong
//
// `PerformGoal` brings the application forward, takes a fresh look, resolves a Place, and plans
// from it. Edge one then established that same Place again from nothing — six readings and the
// gaps between them, for a fact that had had no opportunity to change: building a plan touches no
// desktop.
//
// It was redundant rather than incorrect, which is why it survived. Removing it needs the one
// thing the look does not produce: a WINDOW the proof can be bound to, without which nothing can
// be checked against the foreground.
//
// This holds all three halves — the proof is made, it is usable, and it removes the establishment
// — and says plainly what it cannot reach.
func TestThePlanningLookIsEdgeOnesSource(t *testing.T) {
	t.Run("the look becomes a proof bound to a window", func(t *testing.T) {
		w := &walkDesktop{}
		rt, ids := walkRuntime(t, w)
		withHistory(t, rt)

		proof := rt.planningProof(context.Background(), "testgame", ids[0])
		if proof == nil {
			t.Fatal("the planning look produced no proof at all")
		}
		if proof.Subject != ids[0] {
			t.Fatalf("the proof is about %q, not the Place that was just established",
				proof.Subject)
		}
		if proof.Ref.ID == "" {
			t.Fatal("the proof names no window, so nothing can check it against the " +
				"foreground — which is the whole reason it has to be acquired")
		}
		if proof.From != rehearse.EvidencePlanning {
			t.Errorf("the proof is labelled %q; a planning look is not an outcome",
				proof.From)
		}
		if !proof.Justifies(sessionClock.Now(), "testgame", ids[0], windowLeads) {
			t.Fatalf("the proof cannot justify acting on the Place it was just taken "+
				"of: %+v", *proof)
		}
	})

	t.Run("and there is nothing to make a proof of when nothing was seen", func(t *testing.T) {
		w := &walkDesktop{}
		rt, _ := walkRuntime(t, w)
		withHistory(t, rt)

		if got := rt.planningProof(context.Background(), "testgame", ""); got != nil {
			t.Fatalf("a look that resolved nothing produced %+v — two absences are not "+
				"a place", *got)
		}
	})

	t.Run("it removes edge one's establishment", func(t *testing.T) {
		// WITH the proof.
		fast := &walkDesktop{}
		rtFast, idsFast := walkRuntime(t, fast)
		withHistory(t, rtFast)
		var out service.PerformView
		_, ok := rtFast.performPlan(context.Background(), "testgame",
			rtFast.observations.memory.Topology("testgame"),
			[]observe.RelationshipRef{{From: idsFast[0], To: idsFast[1]}},
			rtFast.planningProof(context.Background(), "testgame", idsFast[0]), &out)
		if !ok {
			t.Fatalf("the walk did not complete: %q", out.Refusal)
		}
		_, withProof, _ := fast.counts()

		// WITHOUT it — the same production code, establishing for itself.
		plain := &walkDesktop{}
		rtPlain, idsPlain := walkRuntime(t, plain)
		var also service.PerformView
		if _, ok := rtPlain.performPlan(context.Background(), "testgame",
			rtPlain.observations.memory.Topology("testgame"),
			[]observe.RelationshipRef{{From: idsPlain[0], To: idsPlain[1]}},
			nil, &also); !ok {
			t.Fatalf("the calibration walk did not complete: %q", also.Refusal)
		}
		_, withoutProof, _ := plain.counts()

		if withProof >= withoutProof {
			t.Fatalf("edge one cost %d window lookups with the planning proof and %d "+
				"without it — the proof was passed and the establishment happened "+
				"anyway", withProof, withoutProof)
		}
	})
}

// AND `PerformGoal` ACTUALLY PASSES IT.
//
// # What this can see, and what it cannot
//
// `PerformGoal` cannot be entered from a test: it goes through `winctx` to bring a window
// forward, which moves the real desktop or fails. Everything below that line is reachable and is
// gated behind ordinary tests; the CALL that connects the planning look to the walk is not.
//
// So this reads the source, exactly as TestEveryLiveWalkerChecksTheForeground does, and it is
// honest about being weaker: it sees that the call is present and cannot see what it computes.
// `planningProof` returning nil for every input would satisfy it — which is why the subtests
// above assert the proof is made and that it removes work.
func TestPerformGoalHandsThePlanningLookToTheWalk(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "perform.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing perform.go: %v", err)
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name.Name != "PerformGoal" {
			continue
		}
		if !callsAnyMethodNamed(fn.Body, "planningProof") {
			t.Fatal("PerformGoal never calls planningProof. It establishes a Place, " +
				"plans from it, and then lets edge one establish the same Place " +
				"again from nothing.")
		}
		return
	}
	t.Fatal("PerformGoal is not in perform.go any more; this test has lost its subject")
}

// ── the deterministic acceptance ──────────────────────────────────────────────

// wholeRoute runs everything `PerformGoal` does below the foreground: the planning proof, both
// edges, and the arrival confirmation.
//
// `PerformGoal` itself cannot be entered — `bringForward` goes through `winctx` and moves the real
// desktop or fails — so this is as close to the product path as a test can stand, and every piece
// of it is the production function.
func wholeRoute(t *testing.T, w *walkDesktop) (*Runtime, []string, service.PerformView) {
	t.Helper()
	rt, ids := walkRuntime(t, w)
	withHistory(t, rt)

	ctx := context.Background()
	var out service.PerformView
	final, ok := rt.performPlan(ctx, "testgame",
		rt.observations.memory.Topology("testgame"),
		[]observe.RelationshipRef{{From: ids[0], To: ids[1]}, {From: ids[1], To: ids[2]}},
		rt.planningProof(ctx, "testgame", ids[0]), &out)
	if ok {
		rt.confirmArrival(ctx, "testgame", ids[2], final, &out)
	}
	return rt, ids, out
}

// A THREE-SCREEN ROUTE ESTABLISHES NOTHING TWICE.
//
// # The shape this roadmap set out to produce
//
//	look                    →  Home
//	edge 1: confirm Home, act, verify Bluetooth & devices
//	edge 2: confirm Bluetooth & devices, act, verify Mouse
//	arrival: Mouse is already proved
//
// and not:
//
//	look, prove Home, prove Home AGAIN, act,
//	prove Bluetooth, prove Bluetooth AGAIN, act,
//	prove Mouse, prove Mouse AGAIN
//
// The counts are what says which of those happened. This asserts them absolutely — two shortened
// confirmations, both accepted, and NO full establishment anywhere in the route — because the
// point of the acceptance is the shape, not a relative improvement.
func TestOneRouteProvesEachScreenOnce(t *testing.T) {
	w := &walkDesktop{}
	_, ids, out := wholeRoute(t, w)

	if out.Refusal != "" || len(out.Steps) != 2 {
		t.Fatalf("the route did not run: refusal=%q steps=%+v", out.Refusal, out.Steps)
	}
	if !out.Arrived || out.To != ids[2] {
		t.Fatalf("the route ran and arrival reports %v at %q", out.Arrived, out.To)
	}
	if out.Cost.Establishments != 0 {
		t.Errorf("the route established where it was %d time(s) from nothing. Every screen "+
			"it stood on had just been proved — by the planning look, or by the edge "+
			"before it verifying its own outcome.", out.Cost.Establishments)
	}
	if out.Cost.Confirmations != 2 || out.Cost.Reused != 2 {
		t.Errorf("%d shortened confirmation(s), %d accepted; two edges means two, and both "+
			"should agree on a desktop nobody touched",
			out.Cost.Confirmations, out.Cost.Reused)
	}

	// AND THE SAME ROUTE WITHOUT THE PLANNING PROOF, for the size of what it buys.
	//
	// This isolates ONE of the two handoffs. Edge two still receives edge one's verified
	// outcome — that is inside `performPlan` and cannot be switched off from here — so the
	// calibration establishes exactly once, for edge one, and the optimized route establishes
	// not at all. The other handoff has its own calibration in
	// TestAVerifiedOutcomeBecomesTheNextEdgesSource.
	plain := &walkDesktop{}
	rtPlain, idsPlain := walkRuntime(t, plain)
	var before service.PerformView
	if _, ok := rtPlain.performPlan(context.Background(), "testgame",
		rtPlain.observations.memory.Topology("testgame"),
		[]observe.RelationshipRef{
			{From: idsPlain[0], To: idsPlain[1]}, {From: idsPlain[1], To: idsPlain[2]}},
		nil, &before); !ok {
		t.Fatalf("the calibration route did not run: %q", before.Refusal)
	}
	if before.Cost.Establishments != 1 {
		t.Fatalf("the calibration route established %d time(s); the premise of this test "+
			"is that edge one, holding no proof, establishes for itself exactly once",
			before.Cost.Establishments)
	}
	if out.Cost.Samples >= before.Cost.Samples {
		t.Fatalf("carrying proof forward read the screen %d times against %d without it",
			out.Cost.Samples, before.Cost.Samples)
	}
	t.Logf("screen readings: %d carried, %d with edge one establishing (%d fewer)",
		out.Cost.Samples, before.Cost.Samples, before.Cost.Samples-out.Cost.Samples)
}

// AND THE FULL LOOK COMES BACK THE MOMENT THE PROOF CANNOT BE USED.
//
// The optimization is only allowed to exist because it fails closed. Here the desktop refuses to
// change under the first action, so edge one cannot verify — and edge two is never reached, which
// is the existing stopping rule doing its job. What must be true is that nothing was carried past
// the failure and nothing claimed arrival.
func TestAnInvalidatedRouteFallsBackToLooking(t *testing.T) {
	w := &walkDesktop{stall: true}
	_, _, out := wholeRoute(t, w)

	if out.Arrived {
		t.Fatal("a route whose first edge never verified reported arrival")
	}
	if len(out.Steps) != 1 {
		t.Fatalf("%d step(s) were attempted past an edge that was never verified",
			len(out.Steps))
	}
	// The proof from the planning look was still USED — that is the point of it — and what
	// it bought was a confirmation rather than an establishment.
	if out.Cost.Confirmations != 1 {
		t.Errorf("%d confirmation(s); edge one held a planning proof and should have "+
			"checked it", out.Cost.Confirmations)
	}
	if out.Cost.Establishments != 0 {
		t.Errorf("%d establishment(s) on an edge that had a good proof to check",
			out.Cost.Establishments)
	}
}

// ── what a performance adds to what was watched ───────────────────────────────

// executionProven says whether memory now claims MARCO walked this edge and checked.
//
// The same assessment `plannableEdges` builds, asked of one edge. Not a re-derivation: it goes
// through `AssessCandidate` and `WithRehearsal` exactly as planning does, so the two cannot come
// to different answers about the same demonstration.
func executionProven(t *testing.T, rt *Runtime, edge observe.RelationshipRef) bool {
	t.Helper()
	memory := rt.observations.memory
	store := memory.(observe.CandidateStore)
	rehearsals := memory.(observe.RehearsalStore)
	top := memory.Topology("testgame")
	for _, c := range store.Candidates("testgame") {
		if c.Relationship != edge {
			continue
		}
		j, known := rt.observations.judgeNow("testgame", c.Relationship)
		if !known {
			continue
		}
		a := observe.AssessCandidate(c, top, observe.DefaultCaptureBounds(),
			corroborationFor(store, "testgame", c))
		if a.WithRehearsal(c, j.Digest, top, rehearsals.Rehearsals("testgame")).Verified {
			return true
		}
	}
	return false
}

// PERFORMING AN OBSERVED EDGE ADDS EXECUTION EVIDENCE. IT DOES NOT REPLACE THE OBSERVATION.
//
// # The seam 35B left open
//
// ADR-089 split what an edge can claim: OBSERVED is "a person demonstrated this and the evidence
// was clean"; VERIFIED is "Marco performed it and positively verified where it ended up". A
// Fast-Learned route is plannable on the first, and the second could not yet be earned because
// nothing wrote it back when a real run succeeded.
//
// The history has to be able to say BOTH: the human demonstrated it, and later Marco performed and
// verified it. Overwriting the first with the second would lose the provenance the whole
// distinction exists for; writing a second graph edge would give one route two identities.
func TestPerformingAnObservedEdgeAddsExecutionEvidence(t *testing.T) {
	w := &walkDesktop{}
	rt, ids := walkRuntime(t, w)
	withHistory(t, rt)
	edge := observe.RelationshipRef{From: ids[0], To: ids[1]}

	store := rt.observations.memory.(observe.CandidateStore)
	candidatesBefore := len(store.Candidates("testgame"))
	edgesBefore := len(rt.observations.memory.Topology("testgame").Relationships)

	// PREMISE: watched, never walked. Without this the test would pass on a fixture that
	// arrived already execution-proven.
	if executionProven(t, rt, edge) {
		t.Fatal("the fixture already claims Marco walked this edge; there is nothing to earn")
	}

	ctx := context.Background()
	var out service.PerformView
	if _, ok := rt.performPlan(ctx, "testgame",
		rt.observations.memory.Topology("testgame"),
		[]observe.RelationshipRef{edge},
		rt.planningProof(ctx, "testgame", ids[0]), &out); !ok {
		t.Fatalf("the edge did not verify: %q", out.Refusal)
	}

	if !executionProven(t, rt, edge) {
		t.Fatal("Marco performed the edge and positively verified where it ended up, and " +
			"memory still says only that somebody demonstrated it. The run proved " +
			"something and nothing wrote it down.")
	}

	// AND THE DEMONSTRATION IS STILL THERE. Execution evidence is ADDED beside what was
	// watched — a history that can only say the last thing that happened is not a history.
	if got := len(store.Candidates("testgame")); got != candidatesBefore {
		t.Errorf("%d demonstration(s) after the run, %d before; performing an edge "+
			"rewrote what the person showed", got, candidatesBefore)
	}
	// AND THERE IS STILL ONE EDGE. One relationship carrying two kinds of evidence, never
	// an observed edge beside a verified one — that would give the planner two answers.
	if got := len(rt.observations.memory.Topology("testgame").Relationships); got != edgesBefore {
		t.Errorf("%d relationship(s) after the run, %d before; verification created a "+
			"second semantic edge for one route", got, edgesBefore)
	}
}

// AND A FAILED PERFORMANCE ERASES NOTHING.
//
// Marco trying and failing is a fact about Marco. It is not evidence that the person did not
// demonstrate the route, and the demonstration is often the only record of how the route goes.
// Deleting it on a failure would mean one bad run — a dialog in the way, a slow screen — costing
// somebody everything they showed.
func TestAFailedPerformancePreservesWhatWasDemonstrated(t *testing.T) {
	w := &walkDesktop{stall: true} // the screen never changes, so nothing can verify
	rt, ids := walkRuntime(t, w)
	withHistory(t, rt)
	edge := observe.RelationshipRef{From: ids[0], To: ids[1]}

	store := rt.observations.memory.(observe.CandidateStore)
	before := len(store.Candidates("testgame"))
	rehearsals := rt.observations.memory.(observe.RehearsalStore)
	proofsBefore := len(rehearsals.Rehearsals("testgame"))

	ctx := context.Background()
	var out service.PerformView
	if _, ok := rt.performPlan(ctx, "testgame",
		rt.observations.memory.Topology("testgame"),
		[]observe.RelationshipRef{edge},
		rt.planningProof(ctx, "testgame", ids[0]), &out); ok {
		t.Fatal("the premise of this test is a performance that does NOT verify")
	}

	if got := len(store.Candidates("testgame")); got != before {
		t.Fatalf("%d demonstration(s) survived a failed run, %d before it. Marco failing "+
			"is a fact about Marco, not about what the person showed it.", got, before)
	}
	if got := len(rehearsals.Rehearsals("testgame")); got != proofsBefore {
		t.Errorf("%d execution proof(s) after a run that verified nothing, %d before",
			got, proofsBefore)
	}
	if executionProven(t, rt, edge) {
		t.Error("a failed run left the edge claiming Marco had walked it and checked")
	}
}

// ── the look's own lifetime ───────────────────────────────────────────────────

// A LOOK ENDS WHEN IT HAS ITS ANSWER.
//
// # Six seconds of somebody else's screen reading
//
// `freshPlace` starts an observation session to answer one question — which screen is in front —
// and that session is bounded by `freshLookWatch`, eight seconds. The question is ordinarily
// answered in one or two. Nothing ended the session, so the remaining six were spent SAMPLING THE
// SCREEN at `freshLookInterval`, concurrently with the walk the look existed to begin, contending
// for the one accessibility provider with every reading the route took.
//
// It was never a leak: the session is bounded and retires on its own. It was a session doing work
// nobody wanted, at exactly the moment this roadmap is trying to make the walk cheap.
//
// # What can be reached from a test, and what cannot
//
// A look that STARTS a session goes through `StartObservation`, which builds a live Windows target
// and sampler; a test has no desktop for those. So this holds the claim in three pieces and says
// which one is the weak one: `endLook` really retires a session, `freshPlace` really calls it, and
// a look that reused somebody else's session leaves it alone.
func TestALookEndsWhenItHasItsAnswer(t *testing.T) {
	t.Run("ending a look retires the session it names", func(t *testing.T) {
		g, store := watchedRegistry(t)
		watchNow(t, g, store, "settings")
		rt := &Runtime{observations: g}
		id := g.ActiveID()
		if id == "" {
			t.Fatal("nothing is watching; there is no session to end")
		}

		// NO POLLING AFTER THIS LINE, and that is the whole assertion.
		//
		// `Cancel` sets a context and returns; the runner notices at the end of the sample
		// it is taking. A test that polls for twenty seconds passes whether or not anything
		// waited, and it was written that way first.
		//
		// What breaks when nothing waits is not slowness. `lookNow` returns early while a
		// session is running, so the NEXT look starts none of its own and polls a retiring
		// session until `freshLookTimeout` expires — a whole performance refusing
		// `place_unknown`, in 6.7 seconds, without ever reading the screen. Measured live.
		rt.endLook(context.Background(), id)
		if g.ActiveID() != "" {
			t.Fatal("endLook returned while the session was still running. Cancelling is " +
				"a signal, not an event: the next look will see something active, " +
				"decline to start its own, and poll a corpse until it times out.")
		}
	})

	t.Run("and freshLook ends the one it started", func(t *testing.T) {
		// THE WEAK PIECE, and it is weak in a known way: the AST sees the call and
		// cannot see what it is passed. `endLook("")` would satisfy it, which is why
		// the id contract has a test of its own above.
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, "perform.go", nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parsing perform.go: %v", err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			// `freshLook`, not `freshPlace`. The latter is a thin wrapper on it now,
			// and the session belongs to whichever function STARTS one — naming the
			// wrapper would pass for a body that had quietly stopped ending anything.
			if !ok || fn.Body == nil || fn.Name.Name != "freshLook" {
				continue
			}
			if !callsAnyMethodNamed(fn.Body, "endLook") {
				t.Fatal("freshLook starts an observation session and never ends " +
					"it. It is bounded, so nothing leaks — it simply goes " +
					"on reading the screen for the rest of its eight " +
					"seconds, alongside the walk it existed to start.")
			}
			return
		}
		t.Fatal("freshLook is not in perform.go any more")
	})

	t.Run("somebody else's session is left alone", func(t *testing.T) {
		g, store := watchedRegistry(t)
		here := watchNow(t, g, store, "settings")
		rt := &Runtime{observations: g}

		if got, _ := rt.freshPlace(context.Background(), "settings"); got != here {
			t.Fatalf("the look answered %q, want %q", got, here)
		}
		if g.ActiveID() == "" {
			t.Fatal("a look answered from a session it did not start and then " +
				"cancelled it; that session belongs to whoever started it")
		}
	})
}

// AND THE SESSION IT DOES START IS THE ONE IT ENDS.
//
// The half above proves a look leaves other people's sessions alone, and would pass for a function
// that never cancels anything. This one enters `lookNow` directly — the only way to see the id it
// hands back — and requires the pair to be honest: a started session names itself, a reused one
// names nothing.
func TestOnlyTheSessionALookStartedIsItsToEnd(t *testing.T) {
	g, store := watchedRegistry(t)
	watchNow(t, g, store, "settings")
	rt := &Runtime{observations: g}

	// Something is already watching this application, so this look starts nothing — and it
	// must say so, because an id here would be a licence to cancel somebody else's session.
	started, err := rt.lookNow(context.Background(), "settings")
	if err != nil {
		t.Fatalf("the look failed: %v", err)
	}
	if started != "" {
		t.Fatalf("a look that reused a running session reported starting %q. `endLook` "+
			"would then cancel a session this look does not own.", started)
	}
	if g.ActiveID() == "" {
		t.Fatal("the session went away")
	}
}

// AND EVERY LOOK THAT CONCLUDED A PLACE IS COUNTED.
//
// # Why an instrument needs its own gate
//
// The counters are what the deterministic acceptance asserts and what a live harness reports, so
// a look that resolves a Place without being counted makes the optimization look better than it
// is — in exactly the direction nobody would question. `Live.placeNow` exists to make that one
// call site; calling `observe.PlaceNow` directly compiles, works, and under-reports.
//
// The relation is the check rather than a written-down number: every shortened confirmation
// resolves a Place, every full establishment resolves a Place, and every step that ran resolves
// the Place it landed on. Nothing else in a walk resolves one.
//
// Bypassing the tally in confirmCarried or establish must fail this.
func TestEveryLookThatConcludedAPlaceIsCounted(t *testing.T) {
	w := &walkDesktop{}
	_, _, out := wholeRoute(t, w)
	if out.Refusal != "" {
		t.Fatalf("the route did not run: %q", out.Refusal)
	}

	want := out.Cost.Confirmations + out.Cost.Establishments + len(out.Steps)
	if out.Cost.Resolutions != want {
		t.Fatalf("%d Place resolution(s) counted; %d looks concluded one "+
			"(%d confirmation(s) + %d establishment(s) + %d step outcome(s)).\n"+
			"A look that resolves a Place without being counted makes the saving read "+
			"larger than it is, which is the one direction nobody checks.",
			out.Cost.Resolutions, want, out.Cost.Confirmations,
			out.Cost.Establishments, len(out.Steps))
	}
	// AND THE READINGS ARE COUNTED TOO. Samples is what the duration is nearly proportional
	// to, so an uncounted reading is an uncounted millisecond.
	if out.Cost.Samples < out.Cost.Resolutions {
		t.Errorf("%d screen reading(s) produced %d Place resolution(s); a Place cannot be "+
			"resolved from a reading that was never taken",
			out.Cost.Samples, out.Cost.Resolutions)
	}
}

// A REFUSED EDGE REPORTS WHAT IT SPENT.
//
// # An instrument that under-reports its own worst case
//
// A refusal produces no `RehearsalResult` — deliberately, and for a good reason: "Marco declined
// to try" and "Marco tried and it went wrong" are different facts. While the cost came off that
// result, a refused edge therefore reported NOTHING.
//
// And the refusal path is where a walk looks most. An edge whose carried proof is contradicted
// runs the shortened confirmation AND then the full establishment — seven readings, against five
// for an edge that simply worked. So every route total was missing its most expensive edges,
// in the direction that makes the optimization look better than it is.
//
// Found live, not here: a real run of Home -> Bluetooth & devices -> Mouse, interrupted by a
// person clicking mid-way, reported five readings for the edge that succeeded and no cost at all
// for the edge that refused.
//
// Reading the tally off the walker instead covers both paths with one snapshot pair.
func TestARefusedEdgeReportsWhatItSpent(t *testing.T) {
	w := &walkDesktop{}
	rt, ids := walkRuntime(t, w)
	withHistory(t, rt)

	// THE LIVE SHAPE, reproduced. The route's SECOND edge, holding a proof of the second
	// screen, while the desktop still shows the FIRST — because nothing has been performed
	// yet. The proof is perfectly justifiable and simply wrong, which is exactly what a
	// person clicking somewhere else produces.
	//
	// So the edge looks twice: the shortened confirmation disagrees, and the establishment
	// that follows finds a screen the grant does not authorise. It refuses, having done the
	// most looking any walk does.
	ctx := context.Background()
	proof := rt.planningProof(ctx, "testgame", ids[1])
	if proof == nil {
		t.Fatal("no proof was made; this case needs one to be contradicted")
	}
	var out service.PerformView
	if _, ok := rt.performPlan(ctx, "testgame",
		rt.observations.memory.Topology("testgame"),
		[]observe.RelationshipRef{{From: ids[1], To: ids[2]}},
		proof, &out); ok {
		t.Fatal("the premise of this test is an edge that REFUSES")
	}
	if len(out.Steps) != 1 {
		t.Fatalf("%d step(s) recorded", len(out.Steps))
	}
	if out.Steps[0].Verified {
		t.Fatal("the edge verified; there is nothing to be about")
	}
	// PREMISE: it refused for the reason this test is about — it looked and disagreed —
	// rather than being turned away before any looking happened.
	if r := out.Steps[0].Refusal; r == "no_evidence" || r == "not_eligible" || r == "no_authority" {
		t.Fatalf("the edge was refused with %q, before it ever looked at anything. This "+
			"case is about a walk that DID look and then refused.", r)
	}

	spent := out.Steps[0].Cost
	if spent.Samples == 0 {
		t.Fatal("a refused edge reported reading the screen ZERO times. It established " +
			"where it was and found somewhere else — that is what the refusal MEANS, " +
			"and it is the most expensive thing a walk does. A total built from this " +
			"understates the work in the direction that flatters the optimization.")
	}
	if spent.Establishments == 0 {
		t.Errorf("a refused edge reported no establishment; it refused BECAUSE of one")
	}
	if spent.Confirmations == 0 {
		t.Errorf("a refused edge reported no confirmation; it held a proof and checked it")
	}
	if spent.Reused != 0 {
		t.Errorf("%d proof(s) reused on an edge whose proof was contradicted",
			spent.Reused)
	}
	// AND THE ROUTE'S TOTAL CARRIES IT. Counting per step and dropping it from the sum
	// would be the same defect one layer up.
	if out.Cost.Samples < spent.Samples {
		t.Errorf("the route totalled %d reading(s) while its only step reported %d",
			out.Cost.Samples, spent.Samples)
	}
}

// A LOOK THAT RAN OUT SAYS WHICH LOOK IT WAS.
//
// # Two very different problems, one sentence
//
// `place_unknown` reaches the Audience as "I can't tell which screen is in front right now",
// followed by whatever reason the look supplied. The look supplied nothing on a timeout, so the
// same sentence covered:
//
//	the screen is one Marco genuinely does not recognise      -> put it on a screen it knows
//	the look never started, because something else held the   -> a fault, and a different fix
//	  registry and had nothing to say
//
// Measured live: a performance refused this way in 6.7 seconds and left a record that could not
// say which had happened. The reason existed in `freshPlace` the whole time and was thrown away.
func TestALookThatRanOutSaysWhichLookItWas(t *testing.T) {
	// EACH OF THE THREE, AT THE FUNCTION THAT DECIDES. `freshLook` cannot be entered from a
	// test without a live desktop, so the wording lives in `lookRanOutWhy` and is held here
	// directly — the mutation that made every timeout claim the window was unreadable
	// survived while this was only ever reached through the walk.
	t.Run("three problems, three sentences", func(t *testing.T) {
		unread := observe.Place{Placed: true, Reach: observe.ReachShell}
		read := observe.Place{Placed: true, Reach: observe.ReachContent}

		borrowed := lookRanOutWhy("settings", false, read)
		unreadable := lookRanOutWhy("settings", true, unread)
		unknown := lookRanOutWhy("settings", true, read)

		if borrowed == unreadable || unreadable == unknown || borrowed == unknown {
			t.Fatalf("two of these say the same thing:\n  %q\n  %q\n  %q",
				borrowed, unreadable, unknown)
		}
		if !strings.Contains(unreadable, "read the page") {
			t.Errorf("an unreadable window says %q", unreadable)
		}
		if !strings.Contains(unknown, "recognise") {
			t.Errorf("an unrecognised page says %q", unknown)
		}
		for _, s := range []string{borrowed, unreadable, unknown} {
			if !strings.Contains(s, "settings") {
				t.Errorf("%q does not name the application", s)
			}
		}
	})

	t.Run("a look of its own that recognised nothing", func(t *testing.T) {
		rt, _, _, _ := namingRuntime(t)
		standingOn(rt, observe.TermAudio) // history, and nothing watching

		subject, why := rt.freshPlace(context.Background(), "settings")
		if subject != "" {
			t.Fatalf("it answered %q", subject)
		}
		if why == "" {
			t.Fatal("a look that ran out gave no reason at all. `place_unknown` then " +
				"reads the same whether the screen was unrecognised or the look " +
				"never happened, and those need different things done about them.")
		}
	})

	t.Run("and a look that could not be taken says something else entirely", func(t *testing.T) {
		// Somebody is demonstrating in another application, so no look can be taken here.
		g, store := watchedRegistry(t)
		watchNow(t, g, store, "testgame")
		rt := &Runtime{observations: g}

		subject, why := rt.freshPlace(context.Background(), "settings")
		if subject != "" {
			t.Fatalf("it answered %q about an application nothing is watching", subject)
		}
		if !strings.Contains(why, "testgame") {
			t.Fatalf("the reason reads %q, which does not name what is in the way", why)
		}
	})
}

// ── the window, and the page in it ────────────────────────────────────────────

// A LOOK SAYS WHETHER IT COULD READ THE WINDOW.
//
// `freshPlace` answers "which screen", which is all any caller wanted until a live run refused
// `place_unknown` about a window that had never been read. A subject and a sentence cannot tell
// "I looked and don't know this page" from "I could see the window and not the page", and those
// need opposite things done about them.
//
// `freshLook` keeps the whole Place. This holds that it does — and that the wrapper still answers
// the narrow question, so the callers that only want a subject are unchanged.
func TestALookSaysWhetherItCouldReadTheWindow(t *testing.T) {
	g, store := watchedRegistry(t)
	here := watchNow(t, g, store, "settings")
	rt := &Runtime{observations: g}

	seen, why := rt.freshLook(context.Background(), "settings")
	if seen.Subject != here {
		t.Fatalf("the look answered %q, want %q (%s)", seen.Subject, here, why)
	}
	if !seen.Readable() {
		t.Fatal("a look that RECOGNISED a screen reported that it could not read the window")
	}
	if seen.Reach != observe.ReachContent {
		t.Errorf("reach = %q", seen.Reach)
	}
	// AND THE NARROW ANSWER IS THE SAME ANSWER. Two functions, one fact.
	if subject, _ := rt.freshPlace(context.Background(), "settings"); subject != seen.Subject {
		t.Errorf("freshPlace says %q and freshLook says %q", subject, seen.Subject)
	}
}

// AN UNREADABLE WINDOW IS NOT AN UNKNOWN PLACE, AND THE AUDIENCE IS TOLD SO.
//
// # Two sentences, and only one of them is actionable
//
//	place_unknown          "I can't tell which screen is in front right now"
//	perception_incomplete  "I can see Settings, but I can't read the page right now."
//
// The first sends a person to open a different page. Measured live, that was the wrong
// instruction three times over: the page was fine and the window was not being read, so changing
// the page produced the identical refusal.
//
// A separate machine-readable word so a client, Activity and a future observer can tell without
// parsing prose — and the sentence names the application rather than a control count, because the
// evidence belongs in diagnostics and this is what somebody reads.
func TestAnUnreadableWindowIsNotAnUnknownPlace(t *testing.T) {
	say := func(t *testing.T, reach observe.Reach) service.PerformView {
		t.Helper()
		var out service.PerformView
		seen := observe.Place{Placed: true, Reach: reach}
		refuseTheLook(&out, "settings", ": nothing matched", seen)
		return out
	}

	unread := say(t, observe.ReachShell)
	if unread.Refusal != "perception_incomplete" {
		t.Fatalf("a window that could not be read refused with %q. `place_unknown` tells "+
			"somebody to open a different page, and the page was never the problem.",
			unread.Refusal)
	}
	if !strings.Contains(unread.Say, "settings") ||
		!strings.Contains(strings.ToLower(unread.Say), "read") {
		t.Errorf("it says %q, which does not tell the person what is actually wrong",
			unread.Say)
	}
	for _, leak := range []string{"control", "%", "subj_", "accessibility", "0."} {
		if strings.Contains(strings.ToLower(unread.Say), leak) {
			t.Errorf("the sentence leaks %q: %q", leak, unread.Say)
		}
	}

	// AND A PAGE THAT REALLY IS UNKNOWN KEEPS ITS OWN WORD. Without this the test above
	// passes for a version that calls everything a perception failure.
	unknown := say(t, observe.ReachContent)
	if unknown.Refusal != "place_unknown" {
		t.Fatalf("a screen that was read and not recognised refused with %q", unknown.Refusal)
	}
}

// AN EDGE THAT WALKED REPORTS HOW LONG IT TOOK.
//
// # A missing measurement rendered as a hard zero
//
// The duration used to ride on the walker's tally, and `Cost.Since` — which subtracts one reading
// of a running count from another — left it at zero. Every edge therefore reported taking no time
// at all, and a live run of a route that took three and a half seconds said it had spent 0 ms
// inside the walk. Nothing failed; the number was simply absent, wearing the shape of a fact.
//
// That is the third time this instrument has failed in the direction that flatters — a blank
// column for a hard zero, a refused edge reporting no cost, and now a duration silently dropped.
// The pattern is worth the third comment: an instrument's failures are not symmetrical, and
// nobody questions a flattering number.
//
// A tally is counted; a duration is timed. The stopwatch belongs to the caller and is now its
// argument, so there is no duration on the tally to forget to carry.
func TestAWalkedEdgeReportsHowLongItTook(t *testing.T) {
	w := &walkDesktop{}
	_, _, out := wholeRoute(t, w)
	if out.Refusal != "" || len(out.Steps) != 2 {
		t.Fatalf("the route did not run: %q", out.Refusal)
	}
	for i, s := range out.Steps {
		if s.Cost.TotalMS <= 0 {
			t.Errorf("edge %d reports %d ms. It read the screen %d times and settled "+
				"between them; no time at all is not a measurement, it is a missing "+
				"one wearing the shape of a fact.", i+1, s.Cost.TotalMS, s.Cost.Samples)
		}
	}
	if out.Cost.TotalMS <= 0 {
		t.Errorf("the route totals %d ms across %d edges", out.Cost.TotalMS, len(out.Steps))
	}
	// AND IT IS AT LEAST WHAT IT SPENT LOOKING. A total smaller than its own part would
	// mean the two are measuring different things.
	if out.Cost.TotalMS < out.Cost.LookingMS {
		t.Errorf("the route took %d ms and spent %d of them looking",
			out.Cost.TotalMS, out.Cost.LookingMS)
	}
}
