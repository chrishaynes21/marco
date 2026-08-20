package main

import (
	"context"
	"strings"
	"testing"

	"path/filepath"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// Telling the truth about what Marco can see with, and changing nothing by looking.

// A source that is not running is never displayed as running.
//
// The one failure a status display must not have. Somebody told "Vision: ON" will trust a highlight
// for a reason that is not true — they will believe Marco looked at the screen when it read an
// accessibility tree — and they will trust it hardest on the applications where the tree is worst.
func TestADisabledPerceptionSourceIsNeverShownAsOn(t *testing.T) {
	// Nothing wired at all. Every source must say so, and each must say WHY.
	bare := (&Runtime{}).perceptionSources()
	if len(bare) == 0 {
		t.Fatal("a Director with nothing wired reported no sources at all, so a surface " +
			"has nothing to show and will show nothing rather than 'off'")
	}
	for _, s := range bare {
		if s.On {
			t.Errorf("%s reported ON with nothing wired", s.Name)
		}
		if s.Reason == "" {
			t.Errorf("%s is off with no reason, so a person cannot tell whether it is "+
				"broken, missing, or switched off on purpose", s.Name)
		}
	}

	// OCR present, vision absent: the mixed case, which is this machine's actual state.
	mixed := (&Runtime{collector: &providers.Collector{}}).perceptionSources()
	by := map[string]sourceStatus{}
	for _, s := range mixed {
		by[s.Name] = s
	}
	if !by["accessibility"].On {
		t.Error("accessibility is off with a collector wired")
	}
	if by["vision"].On {
		t.Error("vision reported ON with no detector running")
	}
	if by["accessibility"].Reason != "" {
		t.Error("a source that is ON carried an excuse for being off")
	}

	// And an explicit unavailability reason reaches the surface rather than being swallowed.
	blocked := (&Runtime{collector: &providers.Collector{},
		ocrUnavailable: "tesseract is not installed"}).perceptionSources()
	for _, s := range blocked {
		if s.Name != "ocr" {
			continue
		}
		if s.On {
			t.Fatal("OCR reported ON while unavailable")
		}
		if s.Reason != "tesseract is not installed" {
			t.Errorf("the real reason was replaced with %q", s.Reason)
		}
	}
}

// Looking at what Marco believes does not change what Marco believes.
//
// Sight is a read. If inspecting started a session, answered a proposal, or wrote anything to
// memory, the inspection would be measuring its own side effects — and the person using it would
// have no way to know.
func TestLookingAtSightChangesNothing(t *testing.T) {
	g := newObservationRegistry()
	rt := &Runtime{observations: g}

	before := struct {
		active   string
		finished int
		known    int
	}{string(g.ActiveID()), len(g.finished), len(g.Known())}

	for range 3 {
		if _, err := rt.PointAt(context.Background(), service.ObservePoint{}); err != nil {
			t.Fatalf("PointAt: %v", err)
		}
		if _, err := rt.PointAt(context.Background(),
			service.ObservePoint{Subject: "subj_anything"}); err != nil {
			t.Fatalf("PointAt(subject): %v", err)
		}
	}

	if got := string(g.ActiveID()); got != before.active {
		t.Errorf("looking started or stopped a session: %q -> %q", before.active, got)
	}
	if len(g.finished) != before.finished {
		t.Errorf("looking added %d finished session(s)", len(g.finished)-before.finished)
	}
	if len(g.Known()) != before.known {
		t.Error("looking changed what Marco has been told")
	}
}

// Pointing at nothing never claims a highlight.
//
// The one wording failure the whole mechanism exists to prevent: "the highlighted controls" in
// front of somebody with nothing highlighted.
func TestAnUnavailableReferentNeverClaimsAHighlight(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry()}
	got, err := rt.PointAt(context.Background(), service.ObservePoint{})
	if err != nil {
		t.Fatalf("PointAt: %v", err)
	}
	if got.CanPoint() {
		t.Fatal("a Director that has never observed anything offered rectangles to draw")
	}
	if len(got.Boxes) != 0 || len(got.Regions) != 0 {
		t.Errorf("%d box(es) and %d region(s) came back with nothing observed",
			len(got.Boxes), len(got.Regions))
	}
	if got.Say == "" {
		t.Error("nothing to point at, and nothing to say about it either")
	}
}

// THE subject-routing test. Asking for one remembered judgement does not get you whatever Marco
// happens to be talking about.
//
// A wiring test, and it exists because the observe-level rules can all be correct while nothing
// calls them: deleting the branch that routes a named subject leaves every unit test passing and
// makes every "show me what this refers to" button point at the wrong thing.
func TestAskingAboutOneJudgementDoesNotGetWhateverMarcoIsTalkingAbout(t *testing.T) {
	g := pointableRegistry(t)
	rt := &Runtime{observations: g}

	// With no subject, Marco points at what it is currently referring to.
	current, err := rt.PointAt(context.Background(), service.ObservePoint{})
	if err != nil {
		t.Fatalf("PointAt: %v", err)
	}
	if len(current.Regions) == 0 {
		t.Fatal("the fixture produced nothing to point at, so this test proves nothing")
	}

	// Naming a subject nothing recognised must NOT hand back that same referent.
	named, err := rt.PointAt(context.Background(),
		service.ObservePoint{Subject: "subj_nothing_recognised_this"})
	if err != nil {
		t.Fatalf("PointAt(subject): %v", err)
	}
	if len(named.Regions) > 0 {
		t.Fatalf("asking about an unrecognised judgement returned %d region(s) — the ones "+
			"Marco is currently referring to. Every \"show me what this refers to\" would "+
			"point at the wrong thing, and look right doing it", len(named.Regions))
	}
	if named.Refusal != string(observe.NotRecognisedHere) {
		t.Errorf("refused with %q, want %q", named.Refusal, observe.NotRecognisedHere)
	}
	if named.Subject != "subj_nothing_recognised_this" {
		t.Error("the answer does not say which subject it is about")
	}
}

// pointableRegistry is a registry holding one finished session with something pointable in it.
func pointableRegistry(t *testing.T) *observationRegistry {
	t.Helper()
	const state = observe.ScreenStateID("state_1")
	track := func(id string, r observe.Region) observe.ShadowTrack {
		return observe.ShadowTrack{ID: id, Present: true, Reference: r,
			Seen: 10, Eligible: 10,
			States: []observe.TrackState{{State: state, Seen: 10, Eligible: 10}}}
	}
	shadow := observe.ShadowTotals{
		Structure: observe.StructureFused, Inferences: 10,
		States: []observe.ScreenState{{ID: state, Inferences: 10, Episodes: 1}},
		Tracks: []observe.ShadowTrack{
			track("a", observe.Region{X: 0.02, Y: 0.01, Width: 0.03, Height: 0.03}),
			track("b", observe.Region{X: 0.06, Y: 0.01, Width: 0.03, Height: 0.03}),
		},
	}
	groups := observe.Groups(shadow.Tracks, shadow.States)
	if len(groups) == 0 {
		t.Skip("the fixture tracks did not form a group")
	}

	g := newObservationRegistry()
	g.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_1", Application: "testapp"},
		Stats:   observesession.Stats{Shadow: shadow},
		Proposals: observe.ProposalLedger{Proposals: []observe.Proposal{{
			ID: "q_1", Status: observe.ProposalOpen, Question: "Are these one set?",
			Subject: observe.Subject{Kind: observe.SubjectGroup, Ref: groups[0].ID},
		}}},
	}}
	rt := &Runtime{collector: &providers.Collector{}, engine: labelledEngine{}}
	sampler := rt.newObservationSampler(sessionClock).(*liveSampler)
	if _, err := sampler.Sample(context.Background(), sampleRequest()); err != nil {
		t.Fatalf("Sample: %v", err)
	}
	g.lastSampler = sampler
	return g
}

// TestAskingAboutOneQuestionDoesNotGetWhateverMarcoIsTalkingAbout enters at Runtime.PointAt.
//
// # The live failure
//
// Windows Settings. The open question's subject was a screen state, which is deliberately not
// pointable; the general path skips such a subject so it does not consume the choice, and fell
// through to an interpretation about a seventeen-control group. A "show me what this refers to"
// button under that question would have drawn the group beneath the screen question's sentence.
//
// The observe package holds the rule. Nothing entered through PointAt with a question, so deleting
// the routing branch there left every one of those tests green while every question button in the
// browser pointed at the wrong subject.
func TestAskingAboutOneQuestionDoesNotGetWhateverMarcoIsTalkingAbout(t *testing.T) {
	g := pointableRegistry(t)
	// A screen question, alongside the pointable group question the fixture already holds.
	r := g.finished[0]
	r.Proposals.Proposals = append(r.Proposals.Proposals, observe.Proposal{
		ID: "q_screen", Status: observe.ProposalOpen,
		Question: "I keep seeing a screen that looks like a menu. Is that what it is?",
		Subject:  observe.Subject{Kind: observe.SubjectState, Ref: "state_1"},
	})
	g.finished[0] = r
	rt := &Runtime{observations: g}

	current, err := rt.PointAt(context.Background(), service.ObservePoint{})
	if err != nil {
		t.Fatalf("PointAt: %v", err)
	}
	if len(current.Regions) == 0 {
		t.Fatal("the fixture produced nothing to point at, so this test proves nothing")
	}

	named, err := rt.PointAt(context.Background(),
		service.ObservePoint{Question: "q_screen"})
	if err != nil {
		t.Fatalf("PointAt(question): %v", err)
	}
	if len(named.Regions) > 0 {
		t.Fatalf("asking about a question whose subject is a whole screen returned %d "+
			"region(s) — the ones Marco is currently referring to. Every question button "+
			"would point at a different subject and look right doing it", len(named.Regions))
	}
	if named.Unavailable != string(observe.ReferentNotAPart) {
		t.Errorf("refused with %q, want %q", named.Unavailable, observe.ReferentNotAPart)
	}
	if named.Question == "" {
		t.Error("the refusal does not carry the question it is about, so a surface cannot " +
			"show which one it failed to point at")
	}
}

// ── what Marco can act on, and what it last did ───────────────────────────────

// Sight says what Marco can act on here and what it last did, in words.
//
// # Why these two belong on this surface
//
// "What are you seeing?" is asked by somebody who has just watched Marco do something, in a place
// Marco may or may not have learned. Perception answers what is on screen; neither half of what
// they actually want to know is a perception fact. What Marco can ACT ON is Theater knowledge —
// durable targets grounded in this place — and what it LAST DID is history.
//
// Mutation: stop populating either. This fails naming the one that went.
func TestSightSaysWhatItCanActOnAndWhatItLastDid(t *testing.T) {
	g := pointableRegistry(t)

	// The reading is composed the way the surface composes it.
	v := pointView{
		Say:        "the Mouse button",
		Place:      "Bluetooth Settings — a screen you've named",
		Targets:    []string{"Mouse (button)", "Devices (item)"},
		LastAction: "click Mouse", LastActionWhen: "just now",
	}
	out := renderSight(v, false)
	for _, want := range []string{"I can act on", "Mouse (button)", "Devices (item)",
		"I last did", "click Mouse", "just now"} {
		if !strings.Contains(out, want) {
			t.Errorf("the reading does not say %q:\n%s", want, out)
		}
	}
	// And neither line appears when there is nothing true to put in it. A surface that
	// printed "I can act on nothing" every time trains a person to stop reading it.
	quiet := renderSight(pointView{Say: "nothing in particular"}, false)
	for _, never := range []string{"I can act on", "I last did"} {
		if strings.Contains(quiet, never) {
			t.Errorf("the reading says %q with nothing to say:\n%s", never, quiet)
		}
	}
	_ = g
}

// A target is described with its KIND, never with a subject id.
//
// "Mouse" alone does not say whether Marco thinks that is a button or a heading, and that
// difference decides whether asking for it does anything. And an identifier is not an answer to
// any question a person asks Sight — see [[ADR-069-a-name-is-authored-and-can-be-taken-back]].
func TestATargetIsReadAsALabelAndAKind(t *testing.T) {
	got := describeTarget(observe.RememberedSubject{
		ID: "subj_deadbeef",
		Structure: observe.StructureSignature{
			Subject: observe.SubjectTarget, Label: "Mouse", Kind: string(observe.KindButton),
		},
	})
	if got != "Mouse (button)" {
		t.Errorf("a target reads as %q, want its label and its kind", got)
	}
	if strings.Contains(got, "subj_") {
		t.Error("a target is described to a person with an identifier")
	}
	// A target Marco could not read a word off is said to be unnamed rather than blank.
	blank := describeTarget(observe.RememberedSubject{
		Structure: observe.StructureSignature{Subject: observe.SubjectTarget},
	})
	if strings.TrimSpace(blank) == "" {
		t.Error("a target with no label renders as nothing at all")
	}
}

// Targets are only claimed for a place Marco has actually settled on.
//
// A candidate match is not enough. Attributing one screen's targets to another that merely
// resembles it is the same error as naming the wrong screen, one layer down — and it would be
// read as "Marco knows this place", which is precisely what it does not know.
//
// # Why this asserts a structural property rather than killing a guard
//
// There is nothing to kill. `Place.Subject` is filled only when the verdict is `MatchSame`
// (`Verdict.Established`), and every durable target is grounded in a real place id — so an
// unsettled place asks for targets grounded in `""` and there are none by construction. Writing
// an extra `p.Verdict != MatchSame` check would be an unfalsifiable restatement of that. What can
// be asserted, and is, is the property both halves rest on.
func TestTargetsAreNotClaimedForAPlaceMarcoHasNotSettledOn(t *testing.T) {
	g := pointableRegistry(t)
	// No memory at all: nothing can be settled, so nothing may be claimed.
	if got := g.targetsHere(observe.ShadowTotals{}, "testgame"); len(got) != 0 {
		t.Errorf("Marco claims it can act on %v with no durable memory to know it from", got)
	}
	// And with no application, there is no namespace for a target to be grounded in.
	if got := g.targetsHere(observe.ShadowTotals{}, ""); len(got) != 0 {
		t.Errorf("Marco claims it can act on %v in no application", got)
	}

	// THE STRUCTURAL PROPERTY, stated so a change to either half fails here.
	//
	// (a) only a settled place carries a subject.
	for _, v := range []observe.MatchVerdict{
		observe.MatchCandidate, observe.MatchDifferent, observe.MatchInsufficient, "",
	} {
		if v.Established() {
			t.Errorf("%q counts as established, so an unsettled place would carry a "+
				"subject and its targets would be claimed for whatever resembles it", v)
		}
	}
	if !observe.MatchSame.Established() {
		t.Error("a same-screen match is not established, so nothing is ever claimed")
	}

	// (b) a durable target is grounded in a real place, never in the empty one.
	store, note := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if note != "" {
		t.Fatalf("memory: %s", note)
	}
	place, err := store.EstablishPlace("testapp", observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 5},
		Terms: []observe.InterfaceTerm{observe.TermAudio}, TermsKnown: true,
	})
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	if _, err := store.RememberTarget("testapp",
		observe.TargetSignature(place, "Mouse", observe.KindButton),
		observe.FromAccessible); err != nil {
		t.Fatalf("remembering: %v", err)
	}
	if got := store.TargetsIn("testapp", ""); len(got) != 0 {
		t.Errorf("%d target(s) are grounded in no place at all, so an unsettled screen "+
			"would be told it can act on them", len(got))
	}
	if got := store.TargetsIn("testapp", place); len(got) != 1 {
		t.Errorf("the target is not grounded in the place it was demonstrated on (%d found)",
			len(got))
	}
}

// PointAt itself fills in what Marco can act on and what it last did.
//
// # Why this exists beside the two above
//
// Because renderSight prints whatever it is handed and targetsHere answers whatever it is asked.
// Neither says the production path ever CALLS them. Three recorded cases in this repository are
// complete, tested code that was never invoked; a wiring test has to enter where the product
// enters. See the memory note "prove wiring by deleting it".
//
// Deleting either assignment in PointAt must fail this.
func TestPointAtFillsInWhatMarcoCanActOnAndLastDid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_HOME", dir)

	// A history with one thing in it, written through the production graph.
	g, err := openGraph()
	if err != nil {
		t.Fatalf("opening the graph: %v", err)
	}
	if err := g.Add(actiongraph.ActionNode{
		ID: "n_1", Timestamp: time.Now(), Goal: "click Mouse",
	}); err != nil {
		t.Fatalf("recording an action: %v", err)
	}

	// A place Marco has settled on, with a durable target grounded in it.
	store, note := semanticmemory.Open(filepath.Join(dir, "semantic-memory.json"))
	if note != "" {
		t.Fatalf("memory: %s", note)
	}
	reg := pointableRegistry(t)
	reg.memory = store
	// The place is established from the SIGNATURE THE FIXTURE'S OWN SHADOW PRODUCES, so
	// PlaceNow genuinely settles on it. Inventing a plausible-looking signature instead
	// would leave the target list empty and the assertion below vacuous — which is how S1
	// survived the first time this test was written.
	shadow := reg.finished[0].Stats.Shadow
	// The fixture never says which state is CURRENT, and its state carries no interface
	// terms, because pointing needs neither. Settling on a place needs both: a place with no
	// discriminator could never be recognised again and the store refuses to keep it.
	shadow.CurrentState = shadow.States[0].ID
	shadow.States[0].Roles = map[string]int{"button": 5}
	shadow.States[0].Terms = map[observe.InterfaceTerm]int{observe.TermAudio: 10}
	shadow.States[0].TermObservations = 10
	reg.finished[0].Stats.Shadow = shadow
	sig, ok := observe.SignatureOfState(shadow, shadow.CurrentState,
		observe.DefaultHypothesisThresholds())
	if !ok {
		t.Skip("the fixture shadow produces no state signature, so nothing can be settled")
	}
	place, err := store.EstablishPlace("testapp", sig)
	if err != nil {
		t.Fatalf("establishing the place: %v", err)
	}
	if _, err := store.RememberTarget("testapp",
		observe.TargetSignature(place, "Mouse", observe.KindButton),
		observe.FromAccessible); err != nil {
		t.Fatalf("remembering the target: %v", err)
	}

	rt := &Runtime{observations: reg}
	v, err := rt.PointAt(context.Background(), service.ObservePoint{})
	if err != nil {
		t.Fatalf("PointAt: %v", err)
	}
	if v.LastAction != "click Mouse" {
		t.Errorf("Sight says it last did %q, want what the action graph records. The panel "+
			"and the CLI both read this field; nothing else would say it was empty.",
			v.LastAction)
	}
	if v.LastActionWhen == "" {
		t.Error("Sight says what it did and not when, so a person cannot tell an action " +
			"they just watched from one from yesterday")
	}
	// And the Theater is actually asked. Not "the field exists" — the durable target the
	// store holds reaches the surface.
	if len(v.Targets) != 1 || v.Targets[0] != "Mouse (button)" {
		t.Errorf("Sight says it can act on %v, want the durable target grounded in this "+
			"place. The Theater knows about it and the surface is not asking.", v.Targets)
	}
}

// Sight says what Marco last did even when it has observed nothing.
//
// # Found live, not in a test
//
// A freshly started Director with 183 actions behind it said "I haven't watched anything yet" and
// nothing else. The last-action line was absent — not because the graph was empty, but because
// PointAt's nothing-observed branch returned before reaching it. History does not depend on
// whether a session has run, and a person who just watched Marco do something and asks what it is
// seeing is owed that answer.
//
// The earlier wiring test could not catch it: its fixture has a finished session, so it never
// took this branch.
func TestSightSaysWhatItLastDidEvenWithNothingObserved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_HOME", dir)

	g, err := openGraph()
	if err != nil {
		t.Fatalf("opening the graph: %v", err)
	}
	if err := g.Add(actiongraph.ActionNode{
		ID: "n_1", Timestamp: time.Now(), Goal: `focus "Untitled. Unmodified."`,
	}); err != nil {
		t.Fatalf("recording an action: %v", err)
	}

	// A Director that has observed NOTHING. The registry is empty on purpose.
	rt := &Runtime{observations: newObservationRegistry()}
	v, err := rt.PointAt(context.Background(), service.ObservePoint{})
	if err != nil {
		t.Fatalf("PointAt: %v", err)
	}
	if v.Refusal != string(observe.NothingObserved) {
		t.Fatalf("the fixture observed something after all (%q), so this proves nothing",
			v.Refusal)
	}
	if v.LastAction != `focus "Untitled. Unmodified."` {
		t.Errorf("Sight says it last did %q with a full action graph behind it. Nothing "+
			"observed is not nothing to say.", v.LastAction)
	}
	if v.LastActionWhen == "" {
		t.Error("Sight says what it did and not when")
	}
	// And it still claims nothing it cannot back: no place, no targets, no rectangles.
	if v.Place != "" || len(v.Targets) != 0 || v.CanPoint() {
		t.Errorf("a Director that has observed nothing claims place=%q targets=%v points=%v",
			v.Place, v.Targets, v.CanPoint())
	}
}
