package observesession_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// Watching one bounded demonstration, because the user said "watch me".
//
// Entered through the real path every time: a store on disk holding a corroborated edge, the real
// learning question, a real yes through `Respond`, the real runner arming itself from the pending
// request, and the real per-sample feed. Nothing here constructs a `Capture` or hands a
// `ProcedureCandidate` to anything.
//
// The two invariants every test below defends:
//
//	nothing observed WITHOUT an approved request becomes a candidate; and
//	nothing a candidate contains can be executed.

// ── the fixture ───────────────────────────────────────────────────────────────

// demoSampler scripts a scene the runner can walk: a start screen, an intermediate one, and a
// destination, with scripted navigation between them.
//
// Each element of `script` is one sample: which screen is up, and what the player did just
// before it appeared. That is where a real keypress lands — the subscription is drained at the
// start of a sample, so a press made between two observations arrives on the observation that
// first shows its effect.
type demoSampler struct {
	script []demoFrame
	calls  int
	// onFrame fires after frame n has been produced, so a test can act mid-demonstration
	// without racing the runner.
	onFrame func(n int)
}

type demoFrame struct {
	// screen is which fixture screen is showing: "a", "b", "c", "x" (an unrecognised one),
	// or "" for gameplay.
	screen string
	// inputs is the navigation observed since the previous sample.
	inputs []observe.NavIntent
	// targets is aligned with inputs: what each event resolved to, zero when nothing did.
	targets []observe.SemanticTarget
	// conditional marks that navigation as context-admitted.
	conditional bool
	// skipped models an observation slot the detector sat out. The navigation still arrives.
	skipped bool
	// editable models a screen offering somewhere to type.
	editable int
}

func (s *demoSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	// After the script runs out the scene holds still on its last screen, so a session
	// longer than the script does not invent transitions.
	f := s.script[len(s.script)-1]
	if s.calls <= len(s.script) {
		f = s.script[s.calls-1]
	} else {
		f.inputs = nil
	}

	sh := &observe.ShadowSample{
		Detector: "screenparser", Ran: !f.skipped, TargetProven: !f.skipped, LatencyMS: 800,
	}
	for i, intent := range f.inputs {
		ev := observe.InputEvent{
			Intent: intent, AtMS: int64(s.calls)*100 + int64(i), Conditional: f.conditional,
		}
		if i < len(f.targets) && f.targets[i] != (observe.SemanticTarget{}) {
			target := f.targets[i]
			ev.Target = &target
		}
		sh.Inputs = append(sh.Inputs, ev)
	}
	if !f.skipped {
		regions := []observe.ShadowRegion{{
			Role: "icon", Confidence: 0.5,
			Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
		}}
		switch f.screen {
		case "a":
			regions = append(regions, screenRegions(0.414, 0.06)...)
			sh.Semantic = terms(observe.TermControls, observe.TermSettings)
		case "b":
			regions = append(regions, screenRegions(0.414, 0.70)...)
			sh.Semantic = terms(observe.TermAudio, observe.TermDisplay)
		case "c":
			regions = append(regions, screenRegions(0.700, 0.36)...)
			sh.Semantic = terms(observe.TermInvite, observe.TermSocial)
		case "x":
			// A screen with no remembered subject behind it.
			regions = append(regions, screenRegions(0.100, 0.36)...)
			sh.Semantic = terms(observe.TermHelp, observe.TermLanguage)
		}
		sh.Regions = regions
		sh.Detections = len(regions)
		sh.Roles = map[string]int{}
		for _, r := range regions {
			sh.Roles[r.Role]++
			if r.Nameable {
				sh.Nameable++
			}
		}
		if f.editable > 0 {
			sh.Semantic.EditableFields = f.editable
		}
	}
	if s.onFrame != nil {
		defer s.onFrame(s.calls)
	}
	return observe.Sample{
		WindowGeneration: 1,
		Frame:            observe.FrameSummary{Application: "testgame", Width: 1920, Height: 1080},
		Shadow:           sh,
	}, nil
}

func terms(t ...observe.InterfaceTerm) observe.SemanticEvidence {
	return observe.SemanticEvidence{Terms: t, Observed: true}
}

// hold repeats one frame, so a screen recurs often enough to become a state.
func hold(screen string, n int) []demoFrame {
	out := make([]demoFrame, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, demoFrame{screen: screen})
	}
	return out
}

func press(screen string, intents ...observe.NavIntent) demoFrame {
	return demoFrame{screen: screen, inputs: intents}
}

// approvedRun seeds a corroborated A→B edge, answers the learning question yes, and returns the
// store and the two subject ids.
//
// ONE store handle throughout, as a Director has: two handles over one file let the last
// whole-file write clobber the other.
func approvedRun(t *testing.T, dir string) (*semanticmemory.Store, string, string) {
	t.Helper()
	store := memoryAt(t, dir)
	from, to := seedRelationshipIn(t, store, 3, strongEvidence())

	runner, _ := runOver(t, store)
	p, ok := learningQuestion(runner)
	if !ok {
		t.Fatal("the fixture produced no learning question, so nothing could be approved")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the approval was not recorded")
	}
	return store, from, to
}

// demonstrate runs one session over a scripted scene, with the capture armed from the store.
func demonstrate(t *testing.T, store *semanticmemory.Store,
	script []demoFrame) (*observesession.Runner, observesession.Result) {

	t.Helper()
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: script}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)
	got, err := r.Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return r, got
}

// happyScript is A, navigate, an intermediate screen, navigate, B.
func happyScript() []demoFrame {
	var out []demoFrame
	out = append(out, hold("a", 4)...)
	out = append(out, press("x", observe.NavDown, observe.NavDown, observe.NavConfirm))
	out = append(out, hold("x", 3)...)
	out = append(out, press("b", observe.NavConfirm))
	out = append(out, hold("b", 4)...)
	return out
}

func intents(s observe.DemonstrationStep) []observe.NavIntent { return s.Intents }

// ── PART 27 / PART 35: the production path ────────────────────────────────────

// THE end-to-end test: an approved request produces a complete candidate.
//
// Deleting the arm call, the feed, or the finish must fail this.
func TestAnApprovedDemonstrationIsCapturedEndToEnd(t *testing.T) {
	dir := t.TempDir()
	store, from, to := approvedRun(t, dir)

	_, res := demonstrate(t, store, happyScript())
	if res.Demonstration == nil {
		t.Fatal("an approved request produced no demonstration record at all; nothing was " +
			"watching after the user said yes")
	}
	c := *res.Demonstration
	if !c.Complete {
		t.Fatalf("the demonstration did not complete: %s (%+v)", c.Reason, c)
	}
	if c.Reason != observe.ReasonArrived {
		t.Errorf("reason = %q, want arrival at the destination", c.Reason)
	}
	// AUTHORISATION provenance: it traces to the approved edge and to nothing else.
	if c.Relationship.From != from || c.Relationship.To != to {
		t.Fatalf("the candidate names %+v, not the approved edge %s → %s",
			c.Relationship, from, to)
	}
	// START established from CURRENT evidence, not assumed from the request.
	if c.Start.Subject != from || c.Start.Transient {
		t.Errorf("start is %+v, want the remembered subject %s", c.Start, from)
	}
	if !c.Start.Verdict.Established() {
		t.Errorf("the start was accepted on a %q verdict", c.Start.Verdict)
	}
	// TWO legs, in order, with the intermediate screen preserved between them.
	if len(c.Steps) != 2 {
		t.Fatalf("%d step(s); the demonstration was flattened and the intermediate screen "+
			"is gone: %+v", len(c.Steps), c.Steps)
	}
	if got := intents(c.Steps[0]); !reflect.DeepEqual(got,
		[]observe.NavIntent{observe.NavDown, observe.NavDown, observe.NavConfirm}) {
		t.Errorf("first leg = %v, want down, down, confirm — in that order", got)
	}
	if got := intents(c.Steps[1]); !reflect.DeepEqual(got,
		[]observe.NavIntent{observe.NavConfirm}) {
		t.Errorf("second leg = %v, want confirm", got)
	}
	if !c.Steps[0].Arrived.Transient {
		t.Errorf("the intermediate screen was recorded as a remembered subject (%+v); it is "+
			"not one", c.Steps[0].Arrived)
	}
	if c.Steps[1].Arrived.Subject != to {
		t.Errorf("the demonstration ended at %+v, not at the approved destination",
			c.Steps[1].Arrived)
	}
	// And it says what it is NOT.
	if c.Verified {
		t.Error("one watched example marked itself verified")
	}
	joined := strings.Join(c.Describe(), "\n")
	if !strings.Contains(joined, "verified: no") {
		t.Errorf("the description does not disclaim verification:\n%s", joined)
	}

	// It survives, because an approved demonstration is meaningful evidence.
	kept := memoryAt(t, dir).Candidates("testgame")
	if len(kept) != 1 {
		t.Fatalf("%d candidate(s) persisted, want 1", len(kept))
	}
	if !kept[0].Complete || kept[0].Relationship.From != from {
		t.Errorf("what was kept is not what was demonstrated: %+v", kept[0])
	}
}

// ── PART 28: no approval, no candidate ────────────────────────────────────────

// The same transition, observed passively, produces nothing.
//
// The load-bearing authorisation test. A demonstration is something a person agreed to give, and
// an observation that happened to look like one is not it.
func TestWithoutAnApprovedRequestNothingIsCaptured(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	// A corroborated edge exists and the user has NOT been asked, or has not said yes.
	seedRelationshipIn(t, store, 3, strongEvidence())

	_, res := demonstrate(t, store, happyScript())
	if res.Demonstration != nil {
		t.Fatalf("an unapproved transition produced a demonstration: %+v", res.Demonstration)
	}
	if got := len(memoryAt(t, dir).Candidates("testgame")); got != 0 {
		t.Fatalf("%d candidate(s) were persisted with no approval behind them", got)
	}
}

// A refusal is not an approval.
func TestARefusedRequestArmsNothing(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	seedRelationshipIn(t, store, 3, strongEvidence())
	runner, _ := runOver(t, store)
	p, ok := learningQuestion(runner)
	if !ok {
		t.Fatal("no question")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseContradicted); !ok {
		t.Fatal("the refusal was not recorded")
	}

	_, res := demonstrate(t, store, happyScript())
	if res.Demonstration != nil {
		t.Fatalf("a refused request armed a capture: %+v", res.Demonstration)
	}
}

// ── PART 29: the wrong start ──────────────────────────────────────────────────

// A demonstration that never begins at the approved start produces no candidate.
//
// Current evidence wins. The request says the transition is A → B; whether the user is standing
// on A is a question only the current cycle can answer.
func TestADemonstrationThatNeverStartsAtTheApprovedSubject(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)
	// Add the third screen so C is a remembered subject the session can sit on.
	if err := store.Remember("testgame", cSignature(), observe.SemanticKnowledge{
		Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeConfirmed,
	}); err != nil {
		t.Fatalf("Remember: %v", err)
	}

	var script []demoFrame
	script = append(script, hold("c", 6)...)
	script = append(script, press("b", observe.NavConfirm))
	script = append(script, hold("b", 5)...)

	_, res := demonstrate(t, store, script)
	if res.Demonstration == nil {
		t.Fatal("no record at all; a capture was armed and must report what became of it")
	}
	c := *res.Demonstration
	if c.Complete {
		t.Fatalf("a demonstration that began somewhere else completed: %+v", c)
	}
	if c.Start.Subject != "" {
		t.Errorf("a start was recorded from a screen that is not the approved one: %+v", c.Start)
	}
	if got := len(memoryAt(t, dir).Candidates("testgame")); got != 0 {
		t.Errorf("%d candidate(s) persisted from a demonstration that never started", got)
	}
}

// ── PART 30: the wrong destination ────────────────────────────────────────────

// Ending somewhere else is a mismatch, never a re-interpreted request.
func TestADemonstrationEndingElsewhereIsAMismatch(t *testing.T) {
	dir := t.TempDir()
	store, from, _ := approvedRun(t, dir)
	if err := store.Remember("testgame", cSignature(), observe.SemanticKnowledge{
		Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeConfirmed,
	}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	var third string
	for _, s := range store.Subjects() {
		if len(s.Structure.Terms) > 0 && s.Structure.Terms[0] == observe.TermInvite {
			third = s.ID
		}
	}

	var script []demoFrame
	script = append(script, hold("a", 4)...)
	script = append(script, press("c", observe.NavConfirm))
	script = append(script, hold("c", 5)...)

	_, res := demonstrate(t, store, script)
	if res.Demonstration == nil {
		t.Fatal("no record")
	}
	c := *res.Demonstration
	if c.Complete {
		t.Fatalf("a demonstration that ended somewhere else completed: %+v", c)
	}
	if c.Reason != observe.ReasonDestinationMismatch {
		t.Errorf("reason = %q, want destination_mismatch", c.Reason)
	}
	// The approved request is untouched, and no A→C candidate appeared.
	for _, kept := range memoryAt(t, dir).Candidates("testgame") {
		if kept.Relationship.To == third {
			t.Errorf("a candidate was created for a transition nobody approved: %+v",
				kept.Relationship)
		}
	}
	if got := len(memoryAt(t, dir).Candidates("testgame")); got != 0 {
		t.Errorf("%d candidate(s) persisted from a mismatched demonstration", got)
	}
	_ = from
}

// ── PART 32: a skipped observation slot ───────────────────────────────────────

// Navigation that arrives during a skipped inference is still navigation.
//
// Screen evidence and input evidence fail independently. A slot the detector sat out says nothing
// about where the user is and must not erase what they did, nor invent a checkpoint.
func TestNavigationSurvivesASkippedInferenceSlot(t *testing.T) {
	dir := t.TempDir()
	store, _, to := approvedRun(t, dir)

	var script []demoFrame
	script = append(script, hold("a", 4)...)
	script = append(script, demoFrame{skipped: true, inputs: []observe.NavIntent{observe.NavDown}})
	script = append(script, demoFrame{skipped: true})
	script = append(script, press("b", observe.NavConfirm))
	script = append(script, hold("b", 4)...)

	_, res := demonstrate(t, store, script)
	if res.Demonstration == nil {
		t.Fatal("no record")
	}
	c := *res.Demonstration
	if !c.Complete {
		t.Fatalf("the demonstration did not complete across skipped slots: %s", c.Reason)
	}
	if len(c.Steps) != 1 {
		t.Fatalf("%d step(s); a skipped slot fabricated a checkpoint: %+v", len(c.Steps), c.Steps)
	}
	if got := intents(c.Steps[0]); !reflect.DeepEqual(got,
		[]observe.NavIntent{observe.NavDown, observe.NavConfirm}) {
		t.Errorf("leg = %v, want down, confirm — the press made during the skipped slot is "+
			"still a press", got)
	}
	if c.Skipped == 0 {
		t.Error("the skipped slots were not counted, so a reader cannot tell that screens " +
			"in between were never seen")
	}
	if c.Steps[0].Arrived.Subject != to {
		t.Errorf("arrived at %+v", c.Steps[0].Arrived)
	}
}

// ── PART 31: an unknown intermediate screen ───────────────────────────────────

// An unrecognised screen is kept as a transient checkpoint and never promoted.
func TestAnUnknownIntermediateScreenIsTransientAndNotRemembered(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)
	before := len(store.Subjects())

	_, res := demonstrate(t, store, happyScript())
	if res.Demonstration == nil || !res.Demonstration.Complete {
		t.Fatalf("the demonstration did not complete: %+v", res.Demonstration)
	}
	mid := res.Demonstration.Steps[0].Arrived
	if !mid.Transient || mid.Subject != "" {
		t.Errorf("the unrecognised screen was recorded as a subject: %+v", mid)
	}
	if len(mid.Structure.Roles) == 0 {
		t.Error("the transient checkpoint kept no structure, so a future validation has " +
			"nothing to check progress against")
	}
	// It did NOT become durable semantic memory. Promotion involves a person.
	if after := len(memoryAt(t, dir).Subjects()); after != before {
		t.Errorf("memory grew from %d to %d subjects during a demonstration; a screen "+
			"observed while learning was promoted without anybody agreeing", before, after)
	}
}

// ── PART 16: cancel ───────────────────────────────────────────────────────────

// A cancelled demonstration produces no partial candidate.
func TestACancelledDemonstrationProducesNoCandidate(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)

	sampler := &demoSampler{script: happyScript()}
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, sampler,
		&recordingEvents{}).WithMemory(store).WithCandidates(store)
	// Cancelled MID-DEMONSTRATION, deterministically: after the start subject has been
	// recognised and some navigation recorded, but before the destination. That is the
	// situation the rule is about — a partial demonstration must not become a candidate.
	// Through CancelCapture, which is the entry point the CLI uses.
	sampler.onFrame = func(n int) {
		if n == 6 {
			_ = r.CancelCapture()
		}
	}
	res, err := r.Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Demonstration != nil && res.Demonstration.Complete {
		t.Fatal("a cancelled demonstration completed")
	}
	// Cancelling before arming leaves nothing; cancelling after leaves an incomplete record.
	// Neither may persist a candidate.
	for _, c := range memoryAt(t, dir).Candidates("testgame") {
		if !c.Complete {
			t.Errorf("an incomplete demonstration was persisted: %+v", c)
		}
	}
}

// ── PART 34: bounds ───────────────────────────────────────────────────────────

// A demonstration that outgrows a bound stops, and says which one.
func TestADemonstrationStopsAtItsBoundAndSaysWhich(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)

	var script []demoFrame
	script = append(script, hold("a", 4)...)
	// Endless navigation that never arrives anywhere.
	for i := 0; i < 40; i++ {
		script = append(script, press("a", observe.NavDown, observe.NavUp))
	}

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: script}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store).
		WithCaptureBounds(observe.CaptureBounds{
			MaxEvents: 6, MaxCheckpoints: 8, MaxRunLength: 8,
			MaxInferences: 500, MaxRestarts: 2,
		})
	res, err := r.Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Demonstration == nil {
		t.Fatal("no record")
	}
	c := *res.Demonstration
	if c.Complete {
		t.Fatal("a demonstration that never arrived completed")
	}
	if c.Reason != observe.ReasonTooManyEvents {
		t.Errorf("reason = %q, want the event bound", c.Reason)
	}
	if c.Events > 8 {
		t.Errorf("%d events were recorded past a bound of 6; the slice is not bounded",
			c.Events)
	}
	if got := len(memoryAt(t, dir).Candidates("testgame")); got != 0 {
		t.Errorf("%d bounded-out demonstration(s) were persisted", got)
	}
}

// ── PART 6: text entry ────────────────────────────────────────────────────────

// A screen you type on is marked as a boundary, and nothing typed is observed.
func TestAScreenThatTakesTypingIsMarkedNotRecorded(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)

	var script []demoFrame
	script = append(script, hold("a", 4)...)
	script = append(script, demoFrame{screen: "x", editable: 2,
		inputs: []observe.NavIntent{observe.NavConfirm}})
	for i := 0; i < 3; i++ {
		script = append(script, demoFrame{screen: "x", editable: 2})
	}
	script = append(script, press("b", observe.NavConfirm))
	script = append(script, hold("b", 4)...)

	_, res := demonstrate(t, store, script)
	if res.Demonstration == nil {
		t.Fatal("no record")
	}
	c := *res.Demonstration
	var marked bool
	for _, s := range c.Steps {
		if s.RequiresTextEntry {
			marked = true
		}
	}
	if !marked {
		t.Fatalf("a demonstration through a screen with editable fields was not marked as "+
			"needing typed input: %+v", c.Steps)
	}
	joined := strings.Join(c.Describe(), "\n")
	if !strings.Contains(joined, "not observed") {
		t.Errorf("the description does not say the typing was not watched:\n%s", joined)
	}
}

// ── PARTS 25/33: authority and privacy ────────────────────────────────────────

// A candidate cannot be executed, and does not pretend it could.
//
// Asserted on the SHAPE of the type rather than on the absence of a call somebody might add:
// there is no method on it that names running anything.
func TestAProcedureCandidateHasNoWayToBeRun(t *testing.T) {
	rt := reflect.TypeOf(observe.ProcedureCandidate{})
	for _, forbidden := range []string{
		"Execute", "Run", "Replay", "Perform", "Apply", "Compile", "Invoke", "Do",
	} {
		if _, ok := rt.MethodByName(forbidden); ok {
			t.Errorf("ProcedureCandidate has a %s method; this milestone creates evidence, "+
				"not something that can act", forbidden)
		}
		if _, ok := reflect.PointerTo(rt).MethodByName(forbidden); ok {
			t.Errorf("*ProcedureCandidate has a %s method", forbidden)
		}
	}
	// And the one field a consumer must read before believing anything.
	if _, ok := rt.FieldByName("Verified"); !ok {
		t.Error("a candidate cannot say whether it has been verified, so a consumer has no " +
			"way to ask")
	}
}

// Nothing captured can reach the durable candidate, in the type graph or on disk.
func TestNoRawInputCanReachADemonstration(t *testing.T) {
	forbidden := []string{
		"keycode", "scancode", "rawkey", "rune", "character", "deviceid", "vkey",
		"screenshot", "pixels", "image", "frame", "title", "label", "text",
		"username", "account", "path", "filename", "window", "process", "pid",
		"generation", "coordinate",
	}
	seen := map[reflect.Type]bool{}
	var walk func(rt reflect.Type, path string)
	walk = func(rt reflect.Type, path string) {
		for rt.Kind() == reflect.Ptr || rt.Kind() == reflect.Slice ||
			rt.Kind() == reflect.Array || rt.Kind() == reflect.Map {
			if rt.Kind() == reflect.Map {
				walk(rt.Key(), path+"[key]")
			}
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		seen[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			name := strings.ToLower(f.Name)
			// RequiresTextEntry is the one deliberate exception, and it is the OPPOSITE of a
			// leak: a bool saying "this demonstration crossed a screen you type on, and
			// what you typed was not watched". Named here rather than pattern-matched away,
			// so widening the exception is a visible edit — the same discipline
			// TestRawKeyIdentityCannotCrossTheBoundary applies to `Conditional`.
			if f.Name == "RequiresTextEntry" && f.Type.Kind() == reflect.Bool {
				continue
			}
			// SemanticTarget.Label is the SECOND deliberate exception, made by the
			// goal-centric milestone and named here so widening it further stays a
			// visible edit. It is not captured input: it is the admitted name of the one
			// control the person's own event landed on, gated twice before it could
			// exist — the canonical plaintext role allowlist or the explicit Learn
			// demonstration licence, and the standing shape filter either way
			// (observe.AdmittedTargetLabel). A candidate exists only because a person
			// asked Marco to learn and then acted; the label of what they aimed at is
			// the meaning of their demonstration, not a recording of their keyboard.
			// The durable TOPOLOGY still carries no labels — TargetedSequence is
			// stripped to NavSequence at that boundary, structurally.
			if f.Name == "Label" && rt == reflect.TypeOf(observe.SemanticTarget{}) {
				continue
			}
			// StructureSignature.Label is the THIRD, and it is reachable here without ever
			// being populated. It is a durable TARGET's identity — the word on a control —
			// and every signature a candidate carries describes a SCREEN, which has no
			// label. Exempted structurally because the type is shared, and then proved
			// empty by value: see TestACandidateCarriesNoTargetLabelInItsCheckpoints,
			// which is the assertion that actually holds the line.
			// [[ADR-068-the-theater-is-the-durable-semantic-world]].
			if f.Name == "Label" && rt == reflect.TypeOf(observe.StructureSignature{}) {
				continue
			}
			for _, bad := range forbidden {
				if strings.Contains(name, bad) {
					t.Errorf("%s.%s (%s) could hold captured input. A person saying "+
						"'watch me' did not agree to have their keystrokes recorded",
						path, f.Name, f.Type)
				}
			}
			walk(f.Type, path+"."+f.Name)
		}
	}
	walk(reflect.TypeOf(observe.ProcedureCandidate{}), "ProcedureCandidate")
	walk(reflect.TypeOf(observe.CaptureView{}), "CaptureView")
}

// And on the bytes, which is where a creative marshaller would show up.
func TestTheStoredDemonstrationContainsNothingCaptured(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)
	if _, res := demonstrate(t, store, happyScript()); res.Demonstration == nil ||
		!res.Demonstration.Complete {
		t.Fatalf("the demonstration did not complete: %+v", res.Demonstration)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "memory.json"))
	if err != nil {
		t.Fatalf("reading the store: %v", err)
	}
	body := strings.ToLower(string(raw))
	for _, leak := range []string{
		"keycode", "scancode", "vk_", "\"key\"", "hwnd", "generation", "process",
		"state_", "shadow_", "\"s\"", "enter", "screenshot", "pixel",
	} {
		if strings.Contains(body, leak) {
			t.Errorf("the stored demonstration contains %q", leak)
		}
	}
	for _, want := range []string{"candidates", "\"confirm\"", "\"down\"", "steps"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the stored demonstration does not contain %q, so what it claims to "+
				"hold is not readable", want)
		}
	}
	// It is valid JSON and holds one candidate.
	var f map[string]any
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("the store is not readable JSON: %v", err)
	}
	if c, ok := f["candidates"].([]any); !ok || len(c) != 1 {
		t.Errorf("candidates = %v", f["candidates"])
	}
}

// ── PART 24: lifecycle across a restart ───────────────────────────────────────

// A completed candidate survives; an unfinished capture is never resumed.
func TestACompletedCandidateSurvivesAndAnUnfinishedOneDoesNot(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)

	// An unfinished demonstration: the user starts and never arrives.
	var stalled []demoFrame
	stalled = append(stalled, hold("a", 8)...)
	if _, res := demonstrate(t, store, stalled); res.Demonstration == nil ||
		res.Demonstration.Complete {
		t.Fatalf("the stalled demonstration was not reported as unfinished: %+v",
			res.Demonstration)
	}
	if got := len(memoryAt(t, dir).Candidates("testgame")); got != 0 {
		t.Fatalf("%d unfinished demonstration(s) persisted", got)
	}

	// A new Director. The pending request is still pending, so it arms again — and this
	// time the user completes it.
	reopened := memoryAt(t, dir)
	if _, res := demonstrate(t, reopened, happyScript()); res.Demonstration == nil ||
		!res.Demonstration.Complete {
		t.Fatalf("a later session could not complete the same approved demonstration: %+v",
			res.Demonstration)
	}
	kept := memoryAt(t, dir).Candidates("testgame")
	if len(kept) != 1 || !kept[0].Complete {
		t.Fatalf("the completed demonstration did not survive: %+v", kept)
	}
	// And it is not resumed: the later session started from scratch rather than continuing
	// the stalled one, which would have carried navigation nobody agreed to give twice.
	if kept[0].Events != 4 {
		t.Errorf("the kept demonstration has %d events; a stalled capture was resumed rather "+
			"than restarted", kept[0].Events)
	}
}

// ── PART 22: status while capturing ───────────────────────────────────────────

// A running capture can be described, safely and without raw keys.
func TestARunningCaptureCanBeDescribed(t *testing.T) {
	dir := t.TempDir()
	store, from, to := approvedRun(t, dir)

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: happyScript()}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)
	if _, err := r.Run(context.Background(), config()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	v := r.Capture()
	if v == nil {
		t.Fatal("a session that watched a demonstration cannot describe it")
	}
	if v.Relationship.From != from || v.Relationship.To != to {
		t.Errorf("the view names %+v", v.Relationship)
	}
	if v.Events == 0 || v.Checkpoints == 0 {
		t.Errorf("the view carries no counts: %+v", v)
	}
	if !v.State.Settled() {
		t.Errorf("state = %q after the session ended", v.State)
	}
}

// ── PART 21: one at a time ────────────────────────────────────────────────────

// Two approved requests do not produce two simultaneous captures.
func TestOnlyOneDemonstrationIsWatchedAtATime(t *testing.T) {
	dir := t.TempDir()
	store, from, _ := approvedRun(t, dir)
	// A second approved edge.
	if err := store.Remember("testgame", cSignature(), observe.SemanticKnowledge{
		Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeConfirmed,
	}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	var third string
	for _, s := range store.Subjects() {
		if len(s.Structure.Terms) > 0 && s.Structure.Terms[0] == observe.TermInvite {
			third = s.ID
		}
	}
	for i := 0; i < 3; i++ {
		if _, err := store.RememberRelationships("testgame",
			[]observe.RelationshipObservation{{
				From: from, To: third, Evidence: strongEvidence(),
			}}); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}
	if err := store.RememberLearning("testgame",
		observe.RelationshipRef{From: from, To: third},
		observe.LearningRequest{Status: observe.LearningPending}); err != nil {
		t.Fatalf("approving the second: %v", err)
	}
	if n := len(observe.PendingLearning(store.Topology("testgame"))); n != 2 {
		t.Fatalf("%d pending request(s), want 2", n)
	}

	r, res := demonstrate(t, store, happyScript())
	if res.Demonstration == nil {
		t.Fatal("no record")
	}
	v := r.Capture()
	if v == nil {
		t.Fatal("no capture view")
	}
	// Exactly one, and deterministically the same one every run.
	if res.Demonstration.Relationship != v.Relationship {
		t.Errorf("the record (%+v) and the view (%+v) describe different demonstrations",
			res.Demonstration.Relationship, v.Relationship)
	}
	if got := len(memoryAt(t, dir).Candidates("testgame")); got > 1 {
		t.Errorf("%d demonstrations were captured at once", got)
	}
}

// A demonstration's checkpoints carry no target label — they describe SCREENS.
//
// The value-level half of the structural exemption above. `StructureSignature` is shared by every
// durable subject, so a target's `Label` field is reachable from a candidate by type; what makes
// the exemption honest is that nothing ever puts a value there. A checkpoint is a place, and a
// place has no label.
//
// Worth asserting rather than assuming: the type is shared, so a future change that started
// stamping a label onto a checkpoint's signature would compile, pass the structural sweep through
// its own exemption, and quietly put a control's name into every demonstration.
func TestACandidateCarriesNoTargetLabelInItsCheckpoints(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)
	_, got := demonstrate(t, store, happyScript())

	seen := 0
	check := func(where string, sig observe.StructureSignature) {
		seen++
		if sig.Label != "" {
			t.Errorf("%s carries the target label %q. A checkpoint describes a screen; a "+
				"label belongs to a target, and putting one here would write a control's "+
				"name into every demonstration that passed it.", where, sig.Label)
		}
		if sig.Subject == observe.SubjectTarget {
			t.Errorf("%s is a target subject; a checkpoint is a place", where)
		}
	}
	_ = got
	for _, c := range store.Candidates("testgame") {
		check("the start", c.Start.Structure)
		for i, s := range c.Steps {
			check("step "+itoa(i+1), s.Arrived.Structure)
		}
	}
	if seen == 0 {
		t.Skip("this walk produced no candidate, so there was nothing to check")
	}
}

// itoa keeps the message above free of an fmt import for one number.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
