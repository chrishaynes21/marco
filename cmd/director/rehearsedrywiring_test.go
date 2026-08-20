package main

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"reflect"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/platform/recordhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// One authorized step, lowered to the last thing before a computer, which is a notebook.
//
// # Why this test lives here and not beside the code
//
// Because the composition root is here. `rehearsedry.go` is the one place that chooses what sits
// at the bottom of the boundary, and everything above that choice — the registry, the runner, the
// grant, the judgement, the lowering, the encoder, Marco's compiler, Marco's runtime — is the
// production path. A test that assembled its own executor would prove that its own executor
// works.
//
// So this drives the REAL registry: a real observation session over a scripted scene, the real
// learning question, a real yes, two real demonstrations, the real rehearsal question, the real
// yes that creates authority, and then `DryRehearse`. The only thing that is not real is the
// screen the sampler describes and the host at the end.

// ── the scene ─────────────────────────────────────────────────────────────────

// dryClock is a clock that never waits.
//
// Installed over the package's `sessionClock` for the duration of a test. Sessions in this file
// would otherwise take the five-second minimum each, four times over, to prove something that has
// nothing to do with elapsed time.
type dryClock struct {
	mu  sync.Mutex
	now time.Time
}

func newDryClock() *dryClock {
	return &dryClock{now: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)}
}

func (c *dryClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *dryClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	c.mu.Unlock()
	ch := make(chan time.Time, 1)
	ch <- now
	return ch
}

// dryTarget is a window that is always there.
type dryTarget struct{}

func (dryTarget) Acquire(context.Context, windowref.Selector) (windowref.Ref, error) {
	return windowref.Ref{
		ID: "hwnd:100", Handle: 100, ProcessID: 7, Application: "testgame", Generation: 1,
	}, nil
}

// dryFrame is one sample: which screen is up, and what the player did just before it appeared.
type dryFrame struct {
	screen string
	inputs []observe.NavIntent
	// target is what a pointer press was aimed at, empty for keyboard navigation. A press
	// carries one because that is the whole of what a demonstration can honestly say about
	// where somebody clicked — see observe.SemanticTarget.
	target *observe.SemanticTarget
	// appearsCalled is what an Actor reported this screen appears to be CALLED, empty when
	// the evidence said nothing. Carried on the frame so a fixture can drive the whole chain
	// from perception to a durable name.
	appearsCalled string
}

func dryHold(screen string, n int) []dryFrame {
	out := make([]dryFrame, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, dryFrame{screen: screen})
	}
	return out
}

func dryPress(screen string, intents ...observe.NavIntent) dryFrame {
	return dryFrame{screen: screen, inputs: intents}
}

// aToB is the route this file rehearses: one press on A, arriving at B.
//
// Deliberately a single `confirm`. The happy path of this milestone is one DIRECTLY VERIFIABLE
// step, and a step that lands on a different remembered screen is what that means.
func aToB() []dryFrame {
	var out []dryFrame
	out = append(out, dryHold("a", 4)...)
	out = append(out, dryPress("b", observe.NavConfirm))
	out = append(out, dryHold("b", 5)...)
	return out
}

// dryRegions is a screen's worth of buttons, at a distinguishing position.
func dryRegions(x0, y0 float64) []observe.ShadowRegion {
	out := make([]observe.ShadowRegion, 0, 4)
	for i := 0; i < 4; i++ {
		out = append(out, observe.ShadowRegion{
			Role: "button", Nameable: true, Confidence: 0.5,
			Region: observe.Region{
				X: x0, Y: y0 + float64(i)*0.042, Width: 0.172, Height: 0.036,
			},
		})
	}
	return out
}

// drySampler walks a script.
type drySampler struct {
	script []dryFrame
	calls  int
}

func (s *drySampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	f := s.script[len(s.script)-1]
	if s.calls <= len(s.script) {
		f = s.script[s.calls-1]
	} else {
		f.inputs = nil
	}

	sh := &observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 800,
	}
	for i, intent := range f.inputs {
		sh.Inputs = append(sh.Inputs, observe.InputEvent{
			Intent: intent, AtMS: int64(s.calls)*100 + int64(i),
			Target: f.target,
		})
	}
	regions := []observe.ShadowRegion{{
		Role: "icon", Confidence: 0.5,
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}}
	switch f.screen {
	case "a":
		regions = append(regions, dryRegions(0.414, 0.06)...)
		sh.Semantic = observe.SemanticEvidence{Observed: true,
			Terms: []observe.InterfaceTerm{observe.TermControls, observe.TermSettings}}
	case "b":
		regions = append(regions, dryRegions(0.414, 0.70)...)
		sh.Semantic = observe.SemanticEvidence{Observed: true,
			Terms: []observe.InterfaceTerm{observe.TermAudio, observe.TermDisplay}}
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
	return observe.Sample{
		WindowGeneration: 1,
		Frame:            observe.FrameSummary{Application: "testgame", Width: 1920, Height: 1080},
		Shadow:           sh,
	}, nil
}

// ── driving the real registry ─────────────────────────────────────────────────

func dryBounds() observe.Bounds {
	b := observe.DefaultBounds()
	b.Duration = 10 * time.Second
	b.Interval = observe.MinInterval
	return b
}

// observeOnce runs one whole session through the registry and waits for it to retire.
func observeOnce(t *testing.T, g *observationRegistry, script []dryFrame) observe.SessionID {
	t.Helper()
	id, err := g.Start(dryTarget{}, &drySampler{script: script}, observesession.NopEvents{},
		windowref.Selector{Application: "testgame"}, dryBounds())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for g.ActiveID() != "" {
		if time.Now().After(deadline) {
			t.Fatal("the session never finished")
		}
		time.Sleep(time.Millisecond)
	}
	return id
}

// sayYes answers the newest open question of one kind, through the production answer path.
func sayYes(t *testing.T, g *observationRegistry, id observe.SessionID,
	ask observe.AskKind) observe.Proposal {

	t.Helper()
	g.mu.RLock()
	var open []observe.Proposal
	for i := len(g.finished) - 1; i >= 0; i-- {
		if g.finished[i].Session.ID == id {
			open = g.finished[i].Proposals.Open()
			break
		}
	}
	g.mu.RUnlock()
	for _, p := range open {
		if p.Ask != ask {
			continue
		}
		if _, ok := g.Answer(id, p.ID, observe.ResponseConfirmed); !ok {
			t.Fatalf("the answer to %s was not recorded", ask)
		}
		return p
	}
	t.Fatalf("no open %s question on %s (open: %+v)", ask, id, open)
	return observe.Proposal{}
}

// authorizedRegistry drives the whole chain and stops holding one live grant.
func authorizedRegistry(t *testing.T) *observationRegistry {
	t.Helper()
	restore := sessionClock
	sessionClock = newDryClock()
	t.Cleanup(func() { sessionClock = restore })

	store, _ := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	g := newObservationRegistry()
	g.memory = store
	seedDryRoute(t, store)

	// learn this? → yes
	id := observeOnce(t, g, dryHold("", 6))
	sayYes(t, g, id, observe.AskLearnRelationship)
	// the demonstration — ONE of them.
	//
	// This walk used to continue "another example? → yes → the second demonstration", and it
	// no longer does: after one clean example the follow-up is not eligible, because an
	// attempt settles what a repetition only repeats.
	// [[ADR-051-one-demonstration-and-an-attempt]]
	observeOnce(t, g, aToB())
	// may I try it? → yes, and THAT is what creates authority
	// The last session leaves the interface showing A — where the route starts — because the
	// rehearsal looks through the same eyes and has to find Marco somewhere it recognises.
	id = observeOnce(t, g, dryHold("a", 8))
	sayYes(t, g, id, observe.AskRehearse)

	if g.last.Grant() == nil {
		t.Fatal("the chain produced no authorization, so there is nothing to rehearse")
	}
	return g
}

// seedDryRoute gives memory a corroborated A → B habit, the way three sessions would.
func seedDryRoute(t *testing.T, store *semanticmemory.Store) {
	t.Helper()
	for _, sig := range []observe.StructureSignature{drySignature(0.06), drySignature(0.70)} {
		if err := store.Remember("testgame", sig, observe.SemanticKnowledge{
			Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
		}); err != nil {
			t.Fatalf("seeding a subject: %v", err)
		}
	}
	var from, to string
	for _, s := range store.Subjects() {
		if len(s.Structure.Terms) > 0 && s.Structure.Terms[0] == observe.TermAudio {
			to = s.ID
		} else {
			from = s.ID
		}
	}
	if from == "" || to == "" {
		t.Fatalf("could not identify the seeded subjects: %+v", store.Subjects())
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
			[]observe.RelationshipObservation{{From: from, To: to, Evidence: ev}}); err != nil {
			t.Fatalf("seeding the relationship: %v", err)
		}
	}
}

// drySignature is a screen as memory holds it, distinguished by where its buttons are.
func drySignature(y0 float64) observe.StructureSignature {
	terms := []observe.InterfaceTerm{observe.TermControls, observe.TermSettings}
	if y0 > 0.5 {
		terms = []observe.InterfaceTerm{observe.TermAudio, observe.TermDisplay}
	}
	return observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4, "icon": 1},
		Terms: terms, TermsKnown: true,
	}
}

// ── THE headline ──────────────────────────────────────────────────────────────

// One authorized step reaches the recording host, and exactly that step.
//
// Deleting the BeginAttempt call, the LowerStep call, or the recorder from `DryRehearse` must
// fail this. Nothing here constructs an Operation, a program, or a dry step.
func TestOneAuthorizedStepIsLoweredToARecordingHost(t *testing.T) {
	g := authorizedRegistry(t)

	step, err := rehearseOnce(t, g, false)
	if err != nil {
		t.Fatalf("the authorized step was refused: %v", err)
	}

	// WHAT WOULD BE SENT, exactly.
	if step.Status != rehearse.EmissionSent {
		t.Fatalf("status = %q, refusal %q", step.Status, step.Refusal)
	}
	if got := len(step.Emitted); got != 1 {
		t.Fatalf("%d call(s) would be sent, want 1: %v", got, step.Emitted)
	}
	if want := "OS's Navigate with confirm"; step.Emitted[0] != want {
		t.Fatalf("would send %q, want %q", step.Emitted[0], want)
	}

	// The step is a MEANING all the way down. No key appears anywhere Director can see.
	for _, forbidden := range []string{"enter", "return", "vk", "0x0d", "keycode", "scancode"} {
		if strings.Contains(strings.ToLower(step.Program), forbidden) {
			t.Errorf("the lowered program names a key (%q):\n%s", forbidden, step.Program)
		}
	}
	if !strings.Contains(step.Program, `do OS's Navigate with "confirm".`) {
		t.Errorf("the program does not lower the meaning:\n%s", step.Program)
	}

	// The classification and the expectation survived the trip.
	if step.Verification != observe.DirectlyVerifiable {
		t.Errorf("verification = %q", step.Verification)
	}
	if step.Expect == "" {
		t.Error("the expectation the next milestone will check did not survive lowering")
	}

	// AND NOTHING HAPPENED. No word in the record claims otherwise.
	joined := strings.ToLower(strings.Join(step.Describe(), "\n"))
	for _, claim := range []string{"success", "verified", "rehearsed", "worked", "reached"} {
		if strings.Contains(joined, claim) {
			t.Errorf("the dry record claims %q. Nothing was sent and nothing was observed:\n%s",
				claim, strings.Join(step.Describe(), "\n"))
		}
	}
	if !strings.Contains(joined, "real input: none") {
		t.Errorf("the record does not say that nothing was sent:\n%s",
			strings.Join(step.Describe(), "\n"))
	}
	if strings.Contains(joined, strings.ToLower(step.Attempt)) && step.Attempt != "" {
		t.Error("the readable record prints the attempt token")
	}
}

// The grant is spent by the attempt, and spent grants do not authorise a second one.
func TestAAttemptSpendsTheAuthorizationExactlyOnce(t *testing.T) {
	g := authorizedRegistry(t)
	grant := g.last.Grant()

	if _, err := rehearseOnce(t, g, false); err != nil {
		t.Fatalf("the first attempt was refused: %v", err)
	}
	if grant.State() != observe.GrantConsumed {
		t.Fatalf("the grant is %q after an attempt; a dry attempt is still an attempt",
			grant.State())
	}
	step, err := rehearseOnce(t, g, false)
	if err == nil {
		t.Fatal("a second dry attempt was authorised by a spent permission")
	}
	if step.Status != rehearse.EmissionRefused || len(step.Emitted) != 0 {
		t.Fatalf("a refused attempt produced %+v", step)
	}
	if r, _ := rehearse.RefusalOf(err); r != rehearse.RefusalGrantSpent {
		t.Errorf("refusal = %q, want %q", r, rehearse.RefusalGrantSpent)
	}
}

// A withdrawn authorization produces nothing.
func TestARevokedAuthorizationProducesNoStepEmission(t *testing.T) {
	g := authorizedRegistry(t)
	g.last.RevokeRehearsal()

	step, err := rehearseOnce(t, g, false)
	if err == nil {
		t.Fatal("a withdrawn authorization produced a dry step")
	}
	if len(step.Emitted) != 0 {
		t.Fatalf("%d call(s) reached the host after revocation", len(step.Emitted))
	}
	if r, _ := rehearse.RefusalOf(err); r != rehearse.RefusalGrantRevoked {
		t.Errorf("refusal = %q, want %q", r, rehearse.RefusalGrantRevoked)
	}
}

// Evidence that moved after the yes refuses the attempt.
func TestAAttemptRefusesEvidenceThatHasMoved(t *testing.T) {
	g := authorizedRegistry(t)

	// The first demonstration is revised — still consistent, still agreeing, and no longer
	// the thing the user was shown when they said yes.
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

	step, err := rehearseOnce(t, g, false)
	if err == nil {
		t.Fatal("an attempt was authorised against evidence that has changed since the yes")
	}
	if len(step.Emitted) != 0 {
		t.Fatalf("%d call(s) reached the host", len(step.Emitted))
	}
	if r, _ := rehearse.RefusalOf(err); r != rehearse.RefusalEvidenceMismatch {
		t.Errorf("refusal = %q, want %q", r, rehearse.RefusalEvidenceMismatch)
	}
	if grant.State() != observe.GrantIssued {
		t.Errorf("a refused claim spent the authorization (%q)", grant.State())
	}
}

// Nothing a dry attempt does becomes evidence.
func TestAAttemptChangesNothingLearned(t *testing.T) {
	g := authorizedRegistry(t)
	store := g.memory.(*semanticmemory.Store)

	before := store.Candidates("testgame")
	if _, err := rehearseOnce(t, g, false); err != nil {
		t.Fatalf("the attempt was refused: %v", err)
	}
	after := store.Candidates("testgame")

	if len(before) != len(after) {
		t.Fatalf("%d candidate(s) before, %d after", len(before), len(after))
	}
	for i := range after {
		if after[i].Verified {
			t.Fatal("a DRY step marked a candidate verified. Nothing was sent, nothing was " +
				"observed, and nothing has been tried")
		}
		if after[i].Events != before[i].Events || after[i].Sequence != before[i].Sequence {
			t.Errorf("candidate %d changed: %+v vs %+v", i, after[i], before[i])
		}
	}
	for _, rel := range store.Relationships() {
		if rel.Sessions == 0 {
			t.Error("a relationship lost its evidence")
		}
	}
}

// ── what the HOST is actually asked for ───────────────────────────────────────

// A bounded ordered run reaches the real host in exactly its own order.
//
// The Director-side tests assert on the program it produced. This one asserts on what came out
// the other end of Marco's own compiler and runtime, which is the only place that can be
// checked — and the second `down` is the point: a set, a sort or a dedup all lose it, and the
// selection stops one row short of where the demonstration went.
func TestTheRealHostReceivesTheStepsOwnOrderedIntents(t *testing.T) {
	rec := recordhost.New()
	runner := marcorunner.New(map[string]runtime.Host{"OS": rec})

	op := marcoexec.Operation{Kind: marcoexec.KindNavigate,
		Intents: []string{"down", "down", "confirm"}}
	if _, err := marcoexec.New(runner).Do(context.Background(), op); err != nil {
		t.Fatalf("running: %v", err)
	}
	want := []string{
		"OS's Navigate with down",
		"OS's Navigate with down",
		"OS's Navigate with confirm",
	}
	if got := rec.Lines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("the host was asked for %v, want %v", got, want)
	}
}

// ── zero real effects ─────────────────────────────────────────────────────────

// Nothing this milestone can do reaches a real input path.
//
// Counted rather than asserted: the recording host is the ONLY host installed, so every capability
// the program invokes lands in the notebook. A keyboard, mouse, controller or accessibility effect
// would have to come from a host that is not there.
func TestADryRehearsalProducesNoRealEffectOfAnyKind(t *testing.T) {
	g := authorizedRegistry(t)
	step, err := rehearseOnce(t, g, false)
	if err != nil {
		t.Fatalf("the attempt was refused: %v", err)
	}

	// Every call went to the recorder, and every call was a navigation meaning.
	if len(step.Emitted) == 0 {
		t.Fatal("nothing was recorded, so this proves nothing")
	}
	for _, line := range step.Emitted {
		if !strings.HasPrefix(line, "OS's Navigate ") {
			t.Errorf("a call other than a navigation meaning reached the host: %s", line)
		}
	}
	// And no capability that touches a real device, a window, a control or the clipboard.
	for _, forbidden := range []string{
		"OS's Key", "OS's Click", "OS's Move", "OS's Drag", "OS's Type", "OS's Secret",
		"OS's Activate", "OS's Launch", "OS's MoveWindow", "OS's WindowState",
		"OS's Clipboard", "OS's Spam", "OS's Repeat", "Accessibility's",
	} {
		for _, line := range step.Emitted {
			if strings.HasPrefix(line, forbidden) {
				t.Errorf("a dry rehearsal asked for %q", forbidden)
			}
		}
	}
	// Nothing was planned, executed or graphed either: this path has no ActionGraph in it,
	// which is why there is nothing to count.
	if strings.Contains(strings.ToLower(step.Program), "accessibility") {
		t.Error("the lowered program reaches the accessibility act")
	}
}

// ── the production trigger ────────────────────────────────────────────────────

// rehearseOnce drives the REAL protocol path: ObserveQuery{Rehearse} → Runtime.Observation →
// Runtime.Rehearse → rehearse.Live → one step → recorder.
//
// Nothing here constructs a Live, an Attempt, an Operation, a program or a result. Deleting any
// production call in that chain fails every test that uses this.
func rehearseOnce(t *testing.T, g *observationRegistry, live bool) (rehearse.StepEmission, error) {
	t.Helper()
	rt := &Runtime{observations: g}
	out, err := rt.Observation(service.ObserveQuery{
		Rehearse: &service.ObserveRehearse{Step: 1, Live: live},
	})
	if err != nil {
		return rehearse.StepEmission{}, err
	}
	view, ok := out.(service.RehearsalView)
	if !ok {
		t.Fatalf("the rehearsal request returned %T", out)
	}
	if !view.Attempted {
		return rehearse.StepEmission{Status: rehearse.EmissionRefused,
				Refusal: rehearse.Refusal(view.Refusal)},
			&rehearse.Error{Refusal: rehearse.Refusal(view.Refusal), Detail: view.Detail}
	}
	if len(view.Steps) == 0 {
		t.Fatalf("the attempt recorded no steps: terminal=%q", view.Terminal)
	}
	s := view.Steps[0]
	intents := make([]observe.NavIntent, 0, len(s.Intents))
	for _, in := range s.Intents {
		intents = append(intents, observe.NavIntent(in))
	}
	return rehearse.StepEmission{
		Attempt: view.Terminal, Position: s.Step, Intents: intents,
		Verification: observe.StepVerifiability(s.Verification), Expect: s.Expected,
		Status: rehearse.EmissionSent, Emitted: s.Emitted,
		Program: s.Program,
	}, nil
}

// ── nothing spends a grant except an explicit request ─────────────────────────

// Observing does not rehearse. Reviewing does not rehearse. Only asking does.
//
// The property this defends is the one a user cares about most: a Director that is watching them
// play must not, at any point, decide to take a turn. The grant exists for minutes and the only
// thing that can spend it is a request somebody made.
func TestNoSessionEverSpendsAGrantByItself(t *testing.T) {
	g := authorizedRegistry(t)
	grant := g.last.Grant()
	if grant == nil || !grant.Active() {
		t.Fatal("the fixture holds no live authorization")
	}

	// Every read a client can make, including the one that renders the whole report.
	g.Snapshot("")
	g.List()
	if grant.State() != observe.GrantIssued {
		t.Fatalf("reading the session changed the authorization to %q", grant.State())
	}

	// And it is still there to be spent deliberately, by a request and nothing else.
	if _, err := rehearseOnce(t, g, false); err != nil {
		t.Fatalf("the authorization no longer works: %v", err)
	}
	if grant.State() != observe.GrantConsumed {
		t.Fatalf("an explicit request did not claim the authorization (%q)", grant.State())
	}
}

// A later session WITHDRAWS an unspent authorization rather than leaving it lying around.
//
// The fail-closed half of the same rule. A permission given about one screen, on evidence from one
// session, has no business being spendable after the user has gone and done something else — and
// "revoked" is a state a reader can see, where "unreachable because a field was reassigned" is not.
func TestANewSessionWithdrawsAnUnspentAuthorization(t *testing.T) {
	g := authorizedRegistry(t)
	grant := g.last.Grant()
	if grant == nil || !grant.Active() {
		t.Fatal("the fixture holds no live authorization")
	}
	observeOnce(t, g, dryHold("a", 8))

	if grant.State() == observe.GrantConsumed {
		t.Fatal("a session SPENT an outstanding authorization. Watching somebody play must " +
			"never take a turn")
	}
	if grant.State() != observe.GrantRevoked {
		t.Fatalf("the superseded authorization is %q; it should be withdrawn, not left "+
			"quietly unreachable", grant.State())
	}
}

// A live rehearsal needs a real host that somebody wired, and this Director has none.
//
// `--live` is not a permission — the grant is the permission. `--live` is the composition root's
// separate question of WHAT sits under the boundary, and a Director with no real host answers it
// honestly rather than quietly running dry.
func TestALiveRehearsalRefusesWithoutARealHost(t *testing.T) {
	g := authorizedRegistry(t)
	rt := &Runtime{observations: g} // no liveMarco
	_, err := rt.Observation(service.ObserveQuery{
		Rehearse: &service.ObserveRehearse{Step: 1, Live: true},
	})
	if err == nil {
		t.Fatal("a live rehearsal was accepted by a Director with no real host wired")
	}
	if !strings.Contains(err.Error(), "real host") {
		t.Errorf("the refusal does not say why: %v", err)
	}
	if g.last.Grant().State() != observe.GrantIssued {
		t.Error("a refused live rehearsal spent the authorization")
	}
}

// Without a grant, the rehearsal request refuses and nothing is emitted.
func TestARehearsalRequestWithoutAGrantIsRefused(t *testing.T) {
	g := newObservationRegistry()
	rt := &Runtime{observations: g}
	if _, err := rt.Observation(service.ObserveQuery{
		Rehearse: &service.ObserveRehearse{Step: 1},
	}); err == nil {
		t.Fatal("a rehearsal ran with nothing authorizing it")
	}
}

// A dry rehearsal concludes nothing about the world.
//
// The trap this closes: with a recording host the application does not move, so classifying the
// post-input screen would report "the screen became A, which is not what that step was for" — which
// reads as a failed step when in fact nothing was sent. Marco does not get to say what came of an
// action it did not take.
func TestADryRehearsalDrawsNoConclusion(t *testing.T) {
	g := authorizedRegistry(t)
	rt := &Runtime{observations: g}
	out, err := rt.Observation(service.ObserveQuery{
		Rehearse: &service.ObserveRehearse{Step: 1},
	})
	if err != nil {
		t.Fatalf("the request failed: %v", err)
	}
	view := out.(service.RehearsalView)
	if !view.Attempted {
		t.Fatalf("nothing was emitted: %s", view.Detail)
	}
	if view.Live {
		t.Fatal("a request without --live installed a real host")
	}
	if view.Completed {
		t.Fatal("a dry rehearsal completed a route it never touched")
	}
	for _, s := range view.Steps {
		if s.Outcome != "" {
			t.Fatalf("a dry step concluded %q about a screen it never touched", s.Outcome)
		}
		if s.Observed != "" {
			t.Errorf("a dry step reports having observed %q", s.Observed)
		}
	}
	joined := strings.ToLower(strings.Join(view.Lines, "\n"))
	if !strings.Contains(joined, "did not try") {
		t.Errorf("the record does not say Marco did not try it:\n%s", joined)
	}
	// "reached" appears in "nothing reached the computer", which is the opposite claim.
	for _, claim := range []string{"wrong", "verified", "worked", "succeed"} {
		if strings.Contains(joined, claim) {
			t.Errorf("a dry rehearsal claims %q:\n%s", claim, joined)
		}
	}
	// And the input was still lowered and offered, exactly once.
	if len(view.Steps) != 1 || len(view.Steps[0].Emitted) != 1 ||
		view.Steps[0].Emitted[0] != "OS's Navigate with confirm" {
		t.Errorf("would send %+v", view.Steps)
	}
}

// ── verification is derived, and only from a completed live route ─────────────

// Nothing a rehearsal does marks a candidate verified. The candidate has no such flag to set.
//
// `ProcedureCandidate.Verified` is still always false and is now DELIBERATELY vestigial: whether a
// route counts as verified is recomputed from stored evidence plus what memory currently holds,
// because a stored boolean would go on saying yes after the demonstration was revised or a screen
// was contradicted. See [[ADR-026-verification-is-derived-from-a-completed-rehearsal]].
func TestNoRehearsalEverStampsACandidateVerified(t *testing.T) {
	g := authorizedRegistry(t)
	store := g.memory.(*semanticmemory.Store)
	if _, err := rehearseOnce(t, g, false); err != nil {
		t.Fatalf("the attempt was refused: %v", err)
	}
	for _, c := range store.Candidates("testgame") {
		if c.Verified {
			t.Fatal("a rehearsal stamped a demonstration verified. Verification is derived " +
				"from evidence, never written onto an observation")
		}
	}
	// And a dry attempt stored nothing at all.
	if n := len(store.Rehearsals("testgame")); n != 0 {
		t.Fatalf("a dry attempt stored %d piece(s) of rehearsal evidence", n)
	}
}

// Stored evidence stops vouching for a demonstration that has since been revised.
//
// The whole reason verification is a fold rather than a flag. The rehearsal really happened and the
// record is not edited — it simply stops being about the candidate in front of us.
func TestStoredEvidenceDoesNotVouchForARevisedDemonstration(t *testing.T) {
	g := authorizedRegistry(t)
	store := g.memory.(*semanticmemory.Store)
	grant := g.last.Grant()
	top := store.Topology("testgame")

	j, ok := g.judgeNow("testgame", grant.Relationship)
	if !ok {
		t.Fatal("no judgement for the authorized route")
	}
	var first observe.ProcedureCandidate
	for _, c := range store.Candidates("testgame") {
		if c.Relationship == grant.Relationship && c.Sequence == 1 {
			first = c
		}
	}
	evidence := observe.RehearsalEvidence{
		// Application is stamped by the store on write; set here because this record was
		// never written.
		Application:  "testgame",
		Relationship: grant.Relationship, Sequence: first.Sequence, Evidence: j.Digest,
		Source: grant.Source, Destination: grant.Destination, Completed: true,
	}
	if !evidence.VerifiedBy(first, j.Digest, top) {
		t.Fatal("evidence from a completed route does not support verifying the route it ran")
	}

	// The demonstration is revised. The rehearsal is still a fact; it is a fact about
	// something else.
	if !evidence.VerifiedBy(first, "a-different-digest", top) {
		// expected
	} else {
		t.Fatal("evidence vouched for a demonstration that has since changed")
	}

	// And a subject the route depends on is forgotten.
	forgotten := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		grant.Source: {ID: grant.Source},
	}}
	if evidence.VerifiedBy(first, j.Digest, forgotten) {
		t.Fatal("evidence vouched for a route whose destination memory no longer holds")
	}

	// A prefix, a contained ending and a dry run all fail at the same gate.
	incomplete := evidence
	incomplete.Completed = false
	if incomplete.VerifiedBy(first, j.Digest, top) {
		t.Fatal("evidence from an attempt that did not complete verified a route")
	}
}

// dryNamed is dryHold with an Actor reporting what the screen appears to be called.
func dryNamed(screen, called string, n int) []dryFrame {
	out := make([]dryFrame, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, dryFrame{screen: screen, appearsCalled: called})
	}
	return out
}
