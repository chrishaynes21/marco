package main

import (
	"go/build"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// The visibility path from the Director's own state.
//
// Everything here drives the REAL observation registry through the REAL sessions the
// product runs, and then asks the REAL Runtime.Playbill what a person would see. Nothing
// constructs a playbill.View by hand: a test that did would prove the renderer and say
// nothing about whether the Director's belief ever arrives, which is the only interesting
// question and the one this repository has now got wrong five times.

// watchOf runs the production visibility read and returns the account and its rendering.
func watchOf(t *testing.T, g *observationRegistry) (playbill.View, string) {
	t.Helper()
	rt := testRuntime(t)
	rt.observations = g
	v := rt.Playbill(service.PlaybillPayload{}).Normalise()
	if err := v.Admit(); err != nil {
		t.Fatalf("the Director produced an account its own guard refuses: %v", err)
	}
	return v, renderWatch(v.Watch())
}

func renderWatch(lines []playbill.Line) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(strings.Repeat("  ", l.Indent))
		b.WriteString(l.Text)
		b.WriteString("\n")
	}
	return b.String()
}

// ── A: the Director's own state reaches the account ───────────────────────────

// THE wiring test. A real session's belief reaches the visibility surface.
//
// Mutations that must fail this:
//   - deleting the `r.observations.Snapshot("")` call in Runtime.Playbill;
//   - deleting `currentFrom`, `seeingFrom` or `thinkingFrom` from the assembly;
//   - returning an empty View when a session exists.
//
// Each of those leaves a Director that answers, renders, and quietly claims to be
// watching nothing — which reads as a working panel and a broken Marco.
func TestAWatchedSessionsBeliefReachesTheAccount(t *testing.T) {
	g := observedRegistryFor(t)

	v, text := watchOf(t, g)

	if v.Current.Application != "testgame" {
		t.Fatalf("the account does not know what was being watched: %q — "+
			"a surface would show a person an anonymous Marco watching nothing",
			v.Current.Application)
	}
	if v.Current.Samples == 0 {
		t.Error("the account carries no sample count, so nobody can tell Marco looked")
	}
	if v.Seeing.Structure == 0 {
		t.Errorf("nothing reached SEEING from a session that detected things:\n%s", text)
	}
	if v.Seeing.Looks == 0 {
		t.Errorf("the account cannot say how often it looked at this screen:\n%s", text)
	}
	if len(v.Thinking.Readings) == 0 {
		t.Errorf("the session's interpretations did not reach THINKING:\n%s", text)
	}
	if !strings.Contains(text, "testgame") {
		t.Errorf("the rendering never says what was being watched:\n%s", text)
	}
	// The session has ENDED, and the account says so rather than claiming to be live.
	if v.Current.Watching {
		t.Error("a finished session reported itself as still watching")
	}
}

// A screen that memory established is RECOGNISED, and it is called what the user calls it.
//
// The two halves of ADR-016 and ADR-031 arriving on a screen: the durable match, and the
// person's own word for it. A surface showing `subject_a1b2` instead is the exact failure
// this milestone exists to prevent, and it is worse than showing nothing because it looks
// like information.
func TestARecognisedScreenIsCalledWhatTheUserCallsIt(t *testing.T) {
	g := observedRegistryFor(t)
	nameEverySubject(t, g, "the pause menu")

	v, text := watchOf(t, g)

	if v.Current.Recognition != playbill.Recognised {
		t.Fatalf("a screen memory had confirmed was not recognised: %q\n%s",
			v.Current.Recognition, text)
	}
	if v.Current.Screen != "the pause menu" {
		t.Fatalf("the account named the screen %q rather than the user's word", v.Current.Screen)
	}
	if !strings.Contains(text, "“the pause menu”") {
		t.Errorf("the rendering did not use the user's word:\n%s", text)
	}
	// The LIVE phrasing is the one that claims recognition of what is on screen now, and
	// this session has ended — so the account reports rather than claims.
	if strings.Contains(text, "I recognise this as") {
		t.Errorf("a finished session claimed to recognise what is on screen NOW:\n%s", text)
	}
	// And no internal identifier came with it.
	for _, bad := range []string{"state_", "subject_", "q_", "d_"} {
		if strings.Contains(text, bad) {
			t.Errorf("the rendering leaked an internal identifier %q:\n%s", bad, text)
		}
	}
}

// An unnamed screen says so in ordinary words, and never prints its durable id.
func TestAnUnnamedScreenIsDescribedRatherThanIdentified(t *testing.T) {
	g := observedRegistryFor(t)

	_, text := watchOf(t, g)
	if strings.Contains(text, "subject_") {
		t.Errorf("a durable subject id reached a person:\n%s", text)
	}
}

// ── C: the presentation does not recompute anything ───────────────────────────

// C. The account's readings ARE the session's, in the session's own order.
//
// Not "similar to". If the visibility layer re-derived interpretations it would
// eventually rank, threshold or filter them differently from the session that acted on
// them, and a person debugging would be comparing Marco against a second opinion nobody
// wrote down.
func TestTheAccountReportsTheSessionsOwnInterpretations(t *testing.T) {
	g := observedRegistryFor(t)

	view, ok := g.Snapshot("")
	if !ok {
		t.Fatal("the fixture produced no session")
	}
	v, _ := watchOf(t, g)

	if v.Thinking.Total != len(view.Hypotheses) {
		t.Fatalf("the account reports %d interpretations, the session holds %d",
			v.Thinking.Total, len(view.Hypotheses))
	}
	for i, r := range v.Thinking.Readings {
		want := standingOf(view.Hypotheses[i])
		if r.Standing != want {
			t.Errorf("reading %d stands as %q; the session says %q", i, r.Standing, want)
		}
	}
}

// C, structurally. The shared representation cannot reach the Director's analysis at all.
//
// The overlay renders `playbill.View` and nothing else. If this package could import the
// observation core then a future convenience — "just recompute the hypotheses here" —
// would compile, and the presentation would quietly become a second analyser.
func TestTheVisibilityRepresentationImportsNoAnalysis(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "pkg", "playbill"))
	if err != nil {
		t.Fatalf("locating the package: %v", err)
	}
	pkg, err := build.ImportDir(root, 0)
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}
	for _, imp := range pkg.Imports {
		if strings.Contains(imp, "/internal/") {
			t.Errorf("pkg/playbill imports %s. The representation a presentation renders "+
				"must not be able to reach the analysis it describes — that is how a "+
				"surface becomes a second opinion", imp)
		}
	}
}

// ── H: an unchanged Director produces an unchanged account ────────────────────

// H. Polling a still Director twice produces the same digest, so nothing redraws.
func TestPollingTheSameSessionTwiceProducesTheSameDigest(t *testing.T) {
	g := observedRegistryFor(t)
	rt := testRuntime(t)
	rt.observations = g

	first := rt.Playbill(service.PlaybillPayload{}).Normalise().WithDigest()
	second := rt.Playbill(service.PlaybillPayload{}).Normalise().WithDigest()

	if first.Digest != second.Digest {
		t.Fatalf("two reads of a finished session disagreed: %s vs %s\n"+
			"a surface coalescing on this would churn while nothing happened",
			first.Digest, second.Digest)
	}
}

// ── the timeline ──────────────────────────────────────────────────────────────

// The timeline is the session's OWN event log, translated — not a second recorder.
//
// A finished session has no live recorder, so it carries no moments. That is honest: its
// findings are in the account, and inventing a timeline for it would mean building the
// second recorder this milestone exists to avoid.
func TestAFinishedSessionCarriesNoInventedTimeline(t *testing.T) {
	g := observedRegistryFor(t)
	v, _ := watchOf(t, g)
	if len(v.Recent) > 0 {
		t.Errorf("a finished session produced %d moments from somewhere. The live event "+
			"log retires with its session; anything here was invented", len(v.Recent))
	}
}

// ── E: the account cannot authorise anything ──────────────────────────────────

// E. An outstanding rehearsal grant is REPORTED as a state and never handed over.
//
// A view is serialised to JSON and passed around. Authority that can be marshalled is
// authority that can be replayed, so the account carries the word "issued" and nothing
// that could be turned back into permission.
func TestAnOutstandingGrantIsReportedAndNeverHandedOver(t *testing.T) {
	g := authorizedRegistry(t)

	rt := testRuntime(t)
	rt.observations = g
	v := rt.Playbill(service.PlaybillPayload{Diagnostics: true}).Normalise()

	if v.Learning.Stage != playbill.RehearsalOffered {
		t.Fatalf("an issued grant did not reach the account: stage %q", v.Learning.Stage)
	}
	if v.Diagnostics == nil || v.Diagnostics.Authority != string(observe.GrantIssued) {
		t.Fatalf("the grant's state did not reach diagnostics: %+v", v.Diagnostics)
	}
	// Reading it changed nothing: the grant is still unused, so the one attempt it
	// permits has not been quietly spent by somebody opening a panel.
	if gr := g.last.Grant(); gr == nil || gr.State() != observe.GrantIssued {
		t.Fatal("reading the account consumed or revoked the authorization")
	}
	// And the rendering says it in words that promise nothing.
	text := renderWatch(v.Watch())
	if !strings.Contains(text, "I can try this once") {
		t.Errorf("the offer did not read as an offer:\n%s", text)
	}
}

// Reading the account cannot ask a question either.
//
// The lowering judgement is a read everywhere EXCEPT that the production LearnedPlay call
// may put a naming question. A visibility surface polling that would interrogate somebody
// about screen names twice a second, which is why the account uses the pure judgement.
func TestReadingTheAccountDoesNotAskTheUserAnything(t *testing.T) {
	g, _ := unnamedRegistry(t)

	before := openQuestionCount(g)
	rt := testRuntime(t)
	rt.observations = g
	for i := 0; i < 5; i++ {
		rt.Playbill(service.PlaybillPayload{Diagnostics: true})
	}
	if after := openQuestionCount(g); after != before {
		t.Fatalf("polling the visibility surface created %d question(s). A panel left "+
			"open would interrogate somebody about screen names forever",
			after-before)
	}
}

// ── fixtures ──────────────────────────────────────────────────────────────────

// observedRegistryFor is a registry that has watched one real session of a known screen.
func observedRegistryFor(t *testing.T) *observationRegistry {
	t.Helper()
	restore := sessionClock
	sessionClock = newDryClock()
	t.Cleanup(func() { sessionClock = restore })

	store, _ := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	g := newObservationRegistry()
	g.memory = store
	seedDryRoute(t, store)
	observeOnce(t, g, dryHold("a", 10))
	return g
}

// nameEverySubject puts the user's word next to each remembered screen.
//
// Through the store's own naming write, which is where AnswerName lands. The naming
// CONVERSATION — propose, ask, bind the answer to the right subject — has its own wiring
// tests in screennamewiring_test.go; what is under test here is whether the visibility
// account reads the word once it exists, and driving the proposal policy would only make
// this test depend on how many other questions happened to be open.
func nameEverySubject(t *testing.T, g *observationRegistry, name string) {
	t.Helper()
	store, ok := g.memory.(*semanticmemory.Store)
	if !ok {
		t.Fatal("the fixture has no store")
	}
	subjects := store.Subjects()
	if len(subjects) == 0 {
		t.Fatal("the fixture remembered no screens, so there is nothing to name")
	}
	// Distinct words per screen: memory refuses two screens the same name, which is
	// right — two things a person calls the same thing are one thing to them.
	for i, s := range subjects {
		word := name
		if i > 0 {
			word = name + " " + string(rune('a'+i))
		}
		if err := store.NameSubject("testgame", s.ID, observe.ScreenName(word)); err != nil {
			t.Fatalf("naming %s: %v", s.ID, err)
		}
	}
}

func openQuestionCount(g *observationRegistry) int {
	view, ok := g.Snapshot("")
	if !ok {
		return 0
	}
	n := 0
	for _, p := range view.Proposals {
		if p.Status == observe.ProposalOpen {
			n++
		}
	}
	return n
}
