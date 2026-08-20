package main

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
	"path/filepath"
	"time"
)

// Part 9: can a person watching tell the transition states apart?
//
// Six states the visibility layer has to distinguish, and they are the ones somebody debugging
// a live session will be trying to read off the panel:
//
//	same screen · screen changed · transition observed
//	transition unattributed · transition attributed · relationship evidence updated
//
// Nothing here redesigns Watch. The account already carried relationships; what it could not
// say was that the screen had changed at all, or whether anything was seen before the change —
// which is the difference between an application somebody is driving and one that redraws
// itself.

// ── the fixture: a session of two real-shaped screens ─────────────────────────

// watchSampler plays a script of compositions with optional navigation and terms, exactly as
// the live sampler produces them.
type watchSampler struct {
	frames []watchFrame
	calls  int
}

type watchFrame struct {
	regions []observe.ShadowRegion
	inputs  []observe.InputEvent
	terms   []observe.InterfaceTerm
}

func (s *watchSampler) Sample(_ context.Context, _ observesession.SampleRequest) (observe.Sample, error) {
	f := s.frames[len(s.frames)-1]
	if s.calls < len(s.frames) {
		f = s.frames[s.calls]
	} else {
		f.inputs = nil
	}
	s.calls++

	out := observe.Sample{Structure: observe.StructuralView{
		Source: observe.StructureFused, Regions: f.regions}}
	if len(f.inputs) > 0 || len(f.terms) > 0 {
		sh := &observe.ShadowSample{Inputs: f.inputs}
		if len(f.terms) > 0 {
			sh.Semantic = observe.SemanticEvidence{Observed: true, Terms: f.terms}
		}
		out.Shadow = sh
	}
	return out, nil
}

func watchFrames(regions []observe.ShadowRegion, n int,
	intents []observe.NavIntent, terms ...observe.InterfaceTerm) []watchFrame {

	out := make([]watchFrame, 0, n)
	for i := range n {
		f := watchFrame{regions: regions, terms: terms}
		if i == 0 {
			for j, in := range intents {
				f.inputs = append(f.inputs,
					observe.InputEvent{Intent: in, AtMS: int64(j) * 40})
			}
		}
		out = append(out, f)
	}
	return out
}

// watchedSession runs one whole session through the real registry and returns the account.
func watchedSession(t *testing.T, g *observationRegistry, frames []watchFrame) playbill.View {
	t.Helper()
	if _, err := g.Start(dryTarget{}, &watchSampler{frames: frames},
		observesession.NopEvents{}, windowref.Selector{Application: "testgame"},
		dryBounds()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for g.ActiveID() != "" {
		if time.Now().After(deadline) {
			t.Fatal("the session never finished")
		}
		time.Sleep(time.Millisecond)
	}
	rt := testRuntime(t)
	rt.observations = g
	v := rt.Playbill(service.PlaybillPayload{Diagnostics: true}).Normalise()
	if err := v.Admit(); err != nil {
		t.Fatalf("the account failed its own guard: %v", err)
	}
	return v
}

func newWatchRegistry(t *testing.T) *observationRegistry {
	t.Helper()
	restore := sessionClock
	sessionClock = newDryClock()
	t.Cleanup(func() { sessionClock = restore })

	store, _ := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	g := newObservationRegistry()
	g.memory = store
	return g
}

// ── the six states ────────────────────────────────────────────────────────────

// An application sitting still says so, and says nothing about changes.
func TestWatchSaysNothingChangedWhenNothingDid(t *testing.T) {
	g := newWatchRegistry(t)
	frames := watchFrames(screenfixture.Editor(), 24, nil, observe.TermSearch)

	v := watchedSession(t, g, frames)
	text := renderWatch(v.Watch())

	if v.Thinking.Changes != 0 {
		t.Errorf("an application that did not change reported %d change(s)", v.Thinking.Changes)
	}
	if strings.Contains(text, "The screen changed") {
		t.Errorf("Watch announced a change that did not happen:\n%s", text)
	}
	if v.Diagnostics.Screens != 1 {
		t.Errorf("diagnostics reports %d screens", v.Diagnostics.Screens)
	}
}

// A change with something observed before it reads as a correlation, never as a cause.
func TestWatchDistinguishesAnAttributedChange(t *testing.T) {
	g := newWatchRegistry(t)
	var frames []watchFrame
	frames = append(frames, watchFrames(screenfixture.Editor(), 6, nil, observe.TermSearch)...)
	frames = append(frames, watchFrames(screenfixture.Settings(), 8,
		[]observe.NavIntent{observe.NavConfirm}, observe.TermSettings)...)

	v := watchedSession(t, g, frames)
	text := renderWatch(v.Watch())

	if v.Thinking.Changes == 0 {
		t.Fatalf("a session that changed screen reported no change:\n%s", text)
	}
	if v.Thinking.Caused == 0 {
		t.Errorf("a change with navigation before it was not counted: caused=%d of %d",
			v.Thinking.Caused, v.Thinking.Changes)
	}
	if !strings.Contains(text, "The screen changed") {
		t.Errorf("Watch did not say the screen changed:\n%s", text)
	}
	// The hedge is the point. "I saw you do something" is a correlation; the sentence must
	// never promote it.
	if !strings.Contains(text, "I can't say it caused") {
		t.Errorf("Watch reported a correlation without its hedge:\n%s", text)
	}
	for _, bad := range []string{"because you", "caused by", "you pressed"} {
		if strings.Contains(text, bad) {
			t.Errorf("Watch claimed causation with %q:\n%s", bad, text)
		}
	}
	if v.Diagnostics.Attributed == 0 {
		t.Error("diagnostics does not carry the attributed count")
	}

	// Watch's numbers ARE production's numbers.
	//
	// Not "consistent with" — equal. A surface that computed its own count could say the
	// screen changed when the session recorded nothing, which is the one thing an
	// observability surface must never do. Found by mutation: an off-by-one in the
	// assembly survived every other assertion here.
	view, ok := g.Snapshot("")
	if !ok {
		t.Fatal("the session vanished")
	}
	var changes, attributed int
	for _, tr := range view.Stats.Shadow.Transitions {
		changes += tr.Count
		attributed += tr.Attributed()
	}
	if v.Thinking.Changes != changes {
		t.Errorf("Watch says the screen changed %d times; the session recorded %d",
			v.Thinking.Changes, changes)
	}
	if v.Thinking.Caused != attributed {
		t.Errorf("Watch says %d changes had something before them; the session recorded %d",
			v.Thinking.Caused, attributed)
	}
}

// A change nobody caused reads as one nobody caused.
func TestWatchDistinguishesAnUnattributedChange(t *testing.T) {
	g := newWatchRegistry(t)
	var frames []watchFrame
	frames = append(frames, watchFrames(screenfixture.Editor(), 6, nil, observe.TermSearch)...)
	frames = append(frames, watchFrames(screenfixture.Settings(), 8, nil, observe.TermSettings)...)

	v := watchedSession(t, g, frames)
	text := renderWatch(v.Watch())

	if v.Thinking.Changes == 0 {
		t.Fatalf("no change was reported:\n%s", text)
	}
	if v.Thinking.Caused != 0 {
		t.Errorf("a change with nothing observed before it was credited to something: %d",
			v.Thinking.Caused)
	}
	if !strings.Contains(text, "I didn't see you do anything before any of them") {
		t.Errorf("Watch did not say the change had no observed cause:\n%s", text)
	}
	if v.Diagnostics.Unattributed == 0 {
		t.Error("diagnostics does not carry the unattributed count")
	}
}

// Relationship evidence, once it exists, reads as a relationship — hedged until it is durable.
func TestWatchDistinguishesRelationshipEvidence(t *testing.T) {
	g := newWatchRegistry(t)
	round := func() []watchFrame {
		var f []watchFrame
		for range 2 {
			f = append(f, watchFrames(screenfixture.Editor(), 5, nil,
				observe.TermSearch, observe.TermNotifications)...)
			f = append(f, watchFrames(screenfixture.Settings(), 10,
				[]observe.NavIntent{observe.NavConfirm},
				observe.TermSettings, observe.TermAudio)...)
		}
		return f
	}

	// Four sittings, answering between them.
	//
	// The production policy allows ONE open question at a time — the cost of asking is the
	// cost of breaking somebody's attention, not the cost of a dialog — so two screens take
	// two sittings to settle, and only after both are durable subjects can the edge between
	// them be written. That is the real shape of this interaction and the fixture models it
	// rather than raising the policy to get there in one.
	for range 4 {
		watchedSession(t, g, round())
		answerEverySemanticQuestion(t, g)
	}

	rt := testRuntime(t)
	rt.observations = g
	v := rt.Playbill(service.PlaybillPayload{Diagnostics: true}).Normalise()
	text := renderWatch(v.Watch())

	if len(v.Thinking.Links) == 0 {
		t.Fatalf("two sittings of the same route produced no relationship on the surface:\n%s",
			text)
	}
	if !strings.Contains(text, "leads to") {
		t.Errorf("Watch did not describe the relationship:\n%s", text)
	}
	// And no internal identifier came with it.
	for _, bad := range []string{"subj_", "state_", "q_", "shadow_", "group_"} {
		if strings.Contains(text, bad) {
			t.Errorf("Watch leaked %q:\n%s", bad, text)
		}
	}
	t.Logf("%s", text)
}

// answerEverySemanticQuestion confirms every open interpretation, through the production path.
func answerEverySemanticQuestion(t *testing.T, g *observationRegistry) {
	t.Helper()
	view, ok := g.Snapshot("")
	if !ok {
		return
	}
	for _, p := range view.Proposals {
		if p.Status != observe.ProposalOpen {
			continue
		}
		if p.Ask == observe.AskSemantic || p.Ask == "" {
			if _, ok := g.Answer("", p.ID, observe.ResponseConfirmed); !ok {
				t.Logf("the answer to %s was not accepted", p.ID)
			}
		}
	}
}
