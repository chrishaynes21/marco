package main

// ONE GENUINE PRESS, ONE ATTRIBUTED CROSSING, ONE DURABLE FACT.
//
// Every confirmation below is built by the real `observe.TransitionAttributed` from a witness the
// real correlator produced: the presses go through `coalesce` and `attribute`, the acts come back
// out of `takeForCrossing`, and the provenance question is the real one. No test here states a
// confirmation level, which is the owner's requirement and the point of the type:
//
//	Add integration tests using real observation and attribution components rather than
//	test-authored ConfirmationAttributed values. Include negative cases where a click occurs but
//	no attributable transition follows.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The measured Trackmania geometry, and a window to state it in.
var (
	learnWindow = directorapi.Rect{Width: 1920, Height: 1080}
	learnWords  = directorapi.Rect{X: 388, Y: 764, Width: 162, Height: 22}
	learnRow    = directorapi.Rect{X: 98, Y: 750, Width: 444, Height: 56}
)

// menuPair is the Stage a surface question is asked about: the words, and the row around them.
func menuPair() directorapi.WorldState {
	shape := func(id string, at directorapi.Rect, label string,
		sources ...directorapi.ObservationSource) *directorapi.Element {

		el := &directorapi.Element{
			ID: directorapi.ElementID(id), Role: directorapi.RoleUnknown,
			Label: label, WindowID: "hwnd:1", Enabled: true, Visible: true,
			Confidence: 0.99, Bounds: at,
		}
		for i, s := range sources {
			el.Provenance.Add(directorapi.ObservationReference{
				Observation: directorapi.ObservationID(id + string(rune('a'+i))),
				Source:      s,
			})
		}
		return el
	}
	words := shape("word", learnWords, "SETTINGS",
		directorapi.SourceStructure, directorapi.SourceOCR)
	row := shape("row", learnRow, "", directorapi.SourceStructure)
	win := directorapi.WindowID("hwnd:1")
	w := directorapi.WorldState{
		Timestamp: time.Now(),
		Elements: map[directorapi.ElementID]*directorapi.Element{
			words.ID: words, row.ID: row},
		Windows: []directorapi.Window{{ID: win, Application: "trackmania",
			Bounds: learnWindow, Focused: true, Visible: true}},
		ActiveWindow: &win,
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 1, Coverage: 1, Actionability: 1, Freshness: 1},
	}
	return w
}

// aQuestion is the real question the real evaluator asks about that Stage.
func aQuestion(t *testing.T) surface.Question {
	t.Helper()
	w := menuPair()
	h := surface.New().Evaluate(&w, "click Settings", "click",
		directorapi.ElementQuery{Text: "settings"})
	q, ok := h.Question()
	if !ok {
		t.Fatalf("the fixture leaves no question: %s — %s", h.Decision, h.Explanation)
	}
	return q
}

// asked opens the question on a learner backed by a real durable store, and returns both.
func asked(t *testing.T, place string) (*surfaceLearner, *semanticmemory.Store) {
	t.Helper()
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	l := newSurfaceLearner(&semanticStores{
		targets: store, surfaces: store, recall: store.Recall,
	}, time.Now)
	l.Asked(execute.Outcome{
		Request: "click Settings", Status: directorapi.ResultNeedsClarification,
		SurfaceQuestion: ptrQuestion(aQuestion(t)),
	}, "trackmania", place, "observe_1", "state_menu", "observe_1")
	if l.Open() == nil {
		t.Fatal("no question was opened")
	}
	return l, store
}

func ptrQuestion(q surface.Question) *surface.Question { return &q }

// pressedAt is a press as the navigation producer emits one: window-relative, 0..1, stamped with
// the screen state it was performed ON.
func pressedAt(x, y int, state string, atMS int64) observe.AttributedInput {
	return observe.AttributedInput{
		Event: observe.InputEvent{
			Intent: observe.NavPoint, AtMS: atMS, Device: observe.DeviceMouse,
			Where: observe.PointerAt{
				X: float64(x) / float64(learnWindow.Width),
				Y: float64(y) / float64(learnWindow.Height),
			},
		},
		State: observe.ScreenStateID(state),
	}
}

// onlyTheRow is a press inside the row and well clear of the words.
func onlyTheRow(state string) observe.AttributedInput {
	return pressedAt(learnRow.X+20, learnRow.Y+learnRow.Height/2, state, 1000)
}

// onTheWords is a press in the middle of the words, which is inside the row as well.
func onTheWords(state string) observe.AttributedInput {
	return pressedAt(learnWords.X+learnWords.Width/2, learnWords.Y+learnWords.Height/2,
		state, 1000)
}

// look is one ambient reading carrying an input log.
func look(session string, in ...observe.AttributedInput) ambientLook {
	return ambientLook{OK: true, Session: observe.SessionID(session),
		Application: "trackmania", Inputs: in}
}

// lastOutcome is what became of the most recent question.
func lastOutcome(l *surfaceLearner) (string, string) {
	got := l.Lessons()
	if len(got) == 0 {
		return "", ""
	}
	return got[len(got)-1].Outcome, got[len(got)-1].Detail
}

// ── THE PRESS ─────────────────────────────────────────────────────────────────

// A GENUINE PRESS SETTLES AN OPEN QUESTION, AND ONLY WHEN IT TELLS THE SURFACES APART.
//
// Through `ambientObserver.drain`, which is the production reader of the admitted input log — so
// the press travels the path a real one does and nothing in this test hands the learner an event
// directly.
func TestAGenuinePressSettlesAnOpenQuestion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      observe.AttributedInput
		settled bool
		outcome string
	}{
		{"on a part only the row covers", onlyTheRow("state_menu"), true, "settled"},
		{"on the words, which are inside both", onTheWords("state_menu"), false,
			"undiscriminating"},
		{"on neither", pressedAt(20, 20, "state_menu", 1000), false, "elsewhere"},
		// THE SCREEN HAD ALREADY MOVED. The correlator stamps every press with the state
		// it was performed on, and a press performed somewhere else is not an answer to
		// a question about this screen.
		{"on a different screen", onlyTheRow("state_elsewhere"), false, "asked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := testRuntime(t)
			l, _ := asked(t, "subj_menu")
			rt.surfaces = l
			rt.surfacesOnce.Do(func() {})

			rt.ambient().drain(look("observe_1", tc.in))

			settled := l.Open() != nil && l.Open().Settled() != nil
			if settled != tc.settled {
				t.Fatalf("settled = %v, want %v", settled, tc.settled)
			}
			out, detail := lastOutcome(l)
			if out != tc.outcome {
				t.Errorf("outcome = %q, want %q (%s)", out, tc.outcome, detail)
			}
			t.Logf("%s: %s", out, detail)
		})
	}
}

// AND A PRESS FROM ANOTHER OBSERVATION SESSION IS NOT AN ANSWER.
//
// Screen state ids are session-local counters, so `state_menu` in two sessions are unrelated
// screens — the fact `drain` drops its whole cursor over.
func TestAPressFromAnotherSessionDoesNotAnswer(t *testing.T) {
	rt := testRuntime(t)
	l, _ := asked(t, "subj_menu")
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	rt.ambient().drain(look("observe_9", onlyTheRow("state_menu")))

	if l.Open().Settled() != nil {
		t.Error("a press from another session settled the question")
	}
}

// ── THE CROSSING ──────────────────────────────────────────────────────────────

// A PRESS IS LEARNED ONLY AFTER AN ATTRIBUTED CROSSING, AND THE NEGATIVE CASES ARE THE POINT.
//
// The witness is built from acts the real correlator filed and handed back, so what is being
// tested is the connection rather than a hand-made value.
func TestAPressIsLearnedOnlyAfterAnAttributedCrossing(t *testing.T) {
	for _, tc := range []struct {
		name string
		// by is who the crossing was attributed to.
		by ambient.Source
		// also adds a second human action to the crossing.
		also bool
		// sameScreen makes the crossing go nowhere.
		sameScreen bool
		// none means no crossing arrives at all.
		none    bool
		learned bool
	}{
		{name: "a human crossing after the press", by: ambient.ByHuman, learned: true},
		{name: "no crossing at all", none: true},
		{name: "a crossing Marco caused", by: ambient.ByMarco},
		{name: "a crossing attributed to two actions", by: ambient.ByHuman, also: true},
		{name: "the screen did not change", by: ambient.ByHuman, sameScreen: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := testRuntime(t)
			l, store := asked(t, "subj_menu")
			rt.surfaces = l
			rt.surfacesOnce.Do(func() {})
			a := rt.ambient()

			// THE PRESS, through the real reader and the real coalescer.
			//
			// ONE LOOK CARRYING THE SESSION'S WHOLE LOG, which is what a look is:
			// `drain` keeps an ABSOLUTE cursor into the session stream, so feeding
			// it a delta makes the second reading appear empty. Found by this very
			// test passing for the wrong reason — the two-action case reported one.
			in := []observe.AttributedInput{onlyTheRow("state_menu")}
			if tc.also {
				// A SECOND PRESS, eight seconds later and elsewhere in the same
				// row: far outside the coalescing window, so two actions.
				in = append(in, pressedAt(learnRow.X+40, learnRow.Y+40,
					"state_menu", 9000))
			}
			a.attribute(a.drain(look("observe_1", in...)))
			if l.Open() == nil || l.Open().Settled() == nil {
				t.Fatal("the press did not settle the question, so this proves nothing")
			}

			if !tc.none {
				// THE CROSSING, with the acts the correlator actually filed.
				did := a.takeForCrossing("state_menu", "state_settings")
				if len(did) == 0 {
					t.Fatal("the correlator filed no action against the screen " +
						"being left, so the witness would be empty")
				}
				to := "subj_settings"
				if tc.sameScreen {
					to = "subj_menu"
				}
				l.offerCrossing(ambient.Step{
					From: "subj_menu", To: to, Application: "trackmania",
					Did: did, By: tc.by, At: time.Now(),
				})
			}

			stored := store.DemonstratedSurfaces("trackmania")
			usable := 0
			for _, d := range stored {
				if ok, _ := d.Usable(); ok {
					usable++
				}
			}
			out, detail := lastOutcome(l)
			if tc.learned {
				if usable != 1 {
					t.Fatalf("%d usable record(s), want 1 — %s: %s", usable,
						out, detail)
				}
				if out != "learned" {
					t.Errorf("outcome = %q, want learned (%s)", out, detail)
				}
				// AND IT SAYS WHAT CONFIRMED IT, which no caller stated.
				if !strings.Contains(stored[0].Confirmed.Why(), "attributed") {
					t.Errorf("the confirmation does not name its evidence: %q",
						stored[0].Confirmed.Why())
				}
				t.Logf("learned: %s — %s", detail, stored[0].Confirmed.Why())
				return
			}
			if usable != 0 {
				t.Fatalf("%d usable record(s) with no attributable transition: %+v",
					usable, stored)
			}
			t.Logf("%s: %s", out, detail)
		})
	}
}

// AND A PRESS WITH NO CROSSING AT ALL IS KEPT AS WHAT IT WAS.
//
// The owner's rule at the far end: a click occurring is not evidence that it did anything, so the
// demonstration is retained as unconfirmed rather than promoted. Nothing can aim with it.
func TestAPressNothingConfirmedIsKeptAndCannotAim(t *testing.T) {
	rt := testRuntime(t)
	l, store := asked(t, "subj_menu")
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})
	a := rt.ambient()
	a.attribute(a.drain(look("observe_1", onlyTheRow("state_menu"))))

	// A CROSSING MARCO CAUSED. The press was real; this is not evidence about it.
	did := a.takeForCrossing("state_menu", "state_settings")
	l.offerCrossing(ambient.Step{From: "subj_menu", To: "subj_settings",
		Application: "trackmania", Did: did, By: ambient.ByMarco, At: time.Now()})

	stored := store.DemonstratedSurfaces("trackmania")
	if len(stored) != 1 {
		t.Fatalf("%d record(s), want the demonstration kept", len(stored))
	}
	if ok, why := stored[0].Usable(); ok {
		t.Error("a demonstration nothing confirmed can aim")
	} else if !strings.Contains(why, "observed") {
		t.Errorf("the refusal does not name what was missing: %s", why)
	}
	out, detail := lastOutcome(l)
	t.Logf("%s: %s", out, detail)
}

// ── THE LIFECYCLE ─────────────────────────────────────────────────────────────

// THE DAEMON OPENS A QUESTION AND CANNOT SETTLE IT ITSELF.
//
// Three things in one: the one door opens a question from a finished request, a later request
// closes it, and nothing in this package can produce a usable confirmation without a crossing.
func TestTheDaemonOpensAQuestionItCannotSettleItself(t *testing.T) {
	l, store := asked(t, "subj_menu")
	if out, _ := lastOutcome(l); out != "asked" {
		t.Fatalf("the question was not recorded as asked: %q", out)
	}

	// A SETTLED PRESS ALONE STORES NOTHING. There is no door from here to a sufficient
	// confirmation: `observe.TransitionAttributed` needs a crossing, and this has none.
	if _, settled, _ := l.Open().Press(onlyTheRow("state_menu"), time.Now()); !settled {
		t.Fatal("the press did not settle the question")
	}
	if n := len(store.DemonstratedSurfaces("trackmania")); n != 0 {
		t.Errorf("%d record(s) were written on the strength of a press alone", n)
	}

	// A LATER REQUEST CLOSES IT. The person asked for something else, and the press they
	// make next belongs to that.
	l.Asked(execute.Outcome{Request: "click Resume", Status: directorapi.ResultDone},
		"trackmania", "subj_menu", "observe_1", "state_menu", "observe_1")
	if l.Open() != nil {
		t.Error("a later request left the old question open")
	}
	if out, detail := lastOutcome(l); out != "replaced" {
		t.Errorf("outcome = %q, want replaced (%s)", out, detail)
	}
}

// A QUESTION ASKED ON AN UNRECOGNISED SCREEN IS ASKED, AND ITS ANSWER CANNOT BE STORED.
//
// A target is `{label, kind, place}` and one with no place could never be found again. The person
// is not at fault for Marco not recognising the screen, so the question is still put — and the
// sentence says the answer cannot be kept.
func TestAQuestionWithNoDurablePlaceCannotBeLearned(t *testing.T) {
	l, store := asked(t, "")
	_, detail := lastOutcome(l)
	if !strings.Contains(detail, "no durable identity") {
		t.Errorf("the question does not say the answer cannot be remembered: %s", detail)
	}
	if _, settled, _ := l.Open().Press(onlyTheRow("state_menu"), time.Now()); !settled {
		t.Fatal("the press did not settle the question")
	}
	l.offerCrossing(ambient.Step{From: "subj_menu", To: "subj_settings",
		Application: "trackmania", Did: []ambient.Act{{Kind: ambient.Activate}},
		By: ambient.ByHuman, At: time.Now()})

	if n := len(store.DemonstratedSurfaces("trackmania")); n != 0 {
		t.Errorf("%d record(s) stored against a screen with no durable identity", n)
	}
	out, why := lastOutcome(l)
	if out != "unconfirmed" {
		t.Errorf("outcome = %q: %s", out, why)
	}
	t.Logf("%s: %s", out, why)
}

// AND A LAPSED QUESTION IS NOT ANSWERABLE.
func TestALapsedQuestionIsNotAnswerable(t *testing.T) {
	l, _ := asked(t, "subj_menu")
	late := time.Now().Add(surface.HowLongAQuestionStaysOpen + time.Second)
	if ok, why := l.Open().Open(late); ok {
		t.Error("a question asked 90 seconds ago is still open")
	} else if !strings.Contains(why, "lapsed") {
		t.Errorf("the refusal does not say it lapsed: %s", why)
	}
	if _, settled, why := l.Open().Press(onlyTheRow("state_menu"), late); settled {
		t.Errorf("a lapsed question was settled: %s", why)
	}
}

// ── THE DAEMON'S OWN SEAMS ────────────────────────────────────────────────────

// THE DAEMON WIRES BOTH SEAMS, AND THE GROUNDER IT WIRES GROUNDS A STORED DEMONSTRATION.
//
// ADR-206 shipped with `Surfaces` and `Place` nil, so no production path grounded anything. This
// is the assertion that they are no longer nil and that the objects behind them work together:
// the grounder is the production one, the store is the production one, and the place comes from
// the resolver ambient watching already runs.
func TestTheDaemonGroundsADemonstratedSurface(t *testing.T) {
	rt := testRuntime(t)
	if rt.pipeline.Surfaces == nil {
		t.Fatal("the pipeline has no surface grounder, so a demonstrated surface can " +
			"never reach a press and the whole milestone is inert")
	}
	if rt.pipeline.Place == nil {
		t.Fatal("the pipeline has no way to know which screen it is on, so a target " +
			"has no durable identity and nothing could be looked up")
	}
	if rt.semanticMemory == nil {
		t.Skip("this Director has no durable memory")
	}
	// THE PLACE SEAM READS AMBIENT WATCHING'S OWN ANSWER. Nothing is in front during a
	// test, so it is empty — and the assertion that matters is that it is the ambient
	// observer being asked rather than something new.
	rt.ambient().mu.Lock()
	rt.ambient().lastSubject = "subj_menu"
	rt.ambient().mu.Unlock()
	if got := rt.pipeline.Place(menuPair()); got != "subj_menu" {
		t.Fatalf("the place seam answered %q, not the standing place", got)
	}

	// A DEMONSTRATION IN THE PRODUCTION STORE, written the only way one can be.
	place, err := rt.semanticMemory.EstablishPlace("trackmania", observe.StructureSignature{
		Subject: observe.SubjectState, Members: 2, TermsKnown: true,
		Terms: []observe.InterfaceTerm{observe.TermSettings},
	})
	if err != nil {
		t.Fatalf("establishing a place: %v", err)
	}
	rt.ambient().mu.Lock()
	rt.ambient().lastSubject = place
	rt.ambient().mu.Unlock()

	l := newSurfaceLearner(&semanticStores{targets: rt.semanticMemory,
		surfaces: rt.semanticMemory, recall: rt.semanticMemory.Recall}, time.Now)
	l.Asked(execute.Outcome{Request: "click Settings",
		Status:          directorapi.ResultNeedsClarification,
		SurfaceQuestion: ptrQuestion(aQuestion(t))},
		"trackmania", place, "observe_1", "state_menu", "observe_1")
	if _, settled, _ := l.Open().Press(onlyTheRow("state_menu"), time.Now()); !settled {
		t.Fatal("the press did not settle the question")
	}
	l.offerCrossing(ambient.Step{From: place, To: "subj_settings",
		Application: "trackmania", Did: []ambient.Act{{Kind: ambient.Activate}},
		By: ambient.ByHuman, At: time.Now()})
	if out, detail := lastOutcome(l); out != "learned" {
		t.Fatalf("the demonstration was not learned: %s — %s", out, detail)
	}

	// AND THE PRODUCTION GROUNDER FINDS IT, on a freshly built Stage.
	w := menuPair()
	g := rt.pipeline.Surfaces.Ground(&w, "trackmania", rt.pipeline.Place(w),
		"click Settings", "click", directorapi.ElementQuery{Text: "settings"})
	if !g.Shown.Given() {
		t.Fatalf("the daemon did not ground the demonstration: %s", g.Because)
	}
	if g.Shown.ElementID != "word" || g.Shown.Surface != "row" {
		t.Errorf("it vouched for %s and aimed at %s", g.Shown.ElementID, g.Shown.Surface)
	}
	if g.Question != nil {
		t.Error("and it asked the question again anyway")
	}
	t.Logf("grounded: %s", g.Because)
}

// AND THE WHOLE THING THROUGH THE PRODUCTION SAMPLE PATH.
//
// Every test above calls `offerCrossing` directly, and the mutation gate caught what that leaves
// unproven: deleting the production call site breaks nothing they assert. So this drives
// `ambientObserver.record` — the function the supervisor's own `sample` calls — twice, with the
// press drained in between, in the order production uses:
//
//	drain + attribute    the press, filed against the screen it was performed on
//	record               the crossing, which is where the confirmation is offered
//
// Deleting either hook must fail this.
func TestTheProductionSamplePathLearnsADemonstration(t *testing.T) {
	rt := testRuntime(t)
	l, store := asked(t, "subj_menu")
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})
	a := rt.ambient()
	t0 := time.Now()

	// STANDING ON THE MENU, which is what makes the next reading a crossing rather than a
	// first sighting.
	a.record("trackmania", ambientLook{OK: true, Session: "observe_1",
		Application: "trackmania", Place: knownPlace("subj_menu"),
		State: "state_menu"}, t0)

	// THE PRESS, through the production reader.
	a.attribute(a.drain(look("observe_1", onlyTheRow("state_menu"))))
	if l.Open() == nil || l.Open().Settled() == nil {
		t.Fatal("the press did not settle the question through the production reader")
	}

	// AND THE SCREEN CHANGES, which is where the crossing is concluded and offered.
	a.record("trackmania", ambientLook{OK: true, Session: "observe_1",
		Application: "trackmania", Place: knownPlace("subj_settings"),
		State: "state_settings"}, t0.Add(2*time.Second))

	out, detail := lastOutcome(l)
	if out != "learned" {
		t.Fatalf("the production path did not learn the demonstration: %s — %s",
			out, detail)
	}
	stored := store.DemonstratedSurfaces("trackmania")
	if len(stored) != 1 {
		t.Fatalf("%d record(s) stored, want 1", len(stored))
	}
	if ok, why := stored[0].Usable(); !ok {
		t.Errorf("the stored record cannot aim: %s", why)
	}
	t.Logf("learned: %s", detail)
}
