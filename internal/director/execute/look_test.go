package execute

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/plan"
	"github.com/chaynes-simpleclouds/marco/internal/director/search"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Does the Director GO AND LOOK for a target the reading does not contain?
//
// The resolve stage has four answers and now treats two of them as recoverable:
//
//	unobservable   the reading did not represent the window → look again, SAME place
//	absent         the window was read and the thing is not in it → look SOMEWHERE ELSE
//
// The first was already here. The second is what this file is about, and the difference between
// them is the difference between re-reading a page and turning one.
//
// # Why this is a pipeline test and not a search test
//
// internal/director/search proves the loop: every outcome, the budget, the gate, the restore.
// What it cannot prove is that anything asks it. This repository has recorded six occasions on
// which a complete, correct mechanism was never invoked, and two of them are in this exact area
// — `directorapi.DiscoveryPlan` is built by the resolver, printed by `director inspect` and
// executed by nothing, and `RecoveryScrollIntoView` is declared and referenced nowhere at all.
//
// Everything here is NON-ACTUATING. The worlds are written down and the scroll moves a counter.

// scrollingDesktop is a window whose visible content depends on where the view is.
//
// Deliberately unlike the shared harness, whose observer advances on every call: the question
// here is whether a reading changes when and only when something scrolls, so the position has to
// be the thing that drives it.
type scrollingDesktop struct {
	screens []directorapi.WorldState
	at      int
	scrolls []directorapi.ScrollWhere
	stuck   bool
	failAt  int
}

func (d *scrollingDesktop) observe(context.Context) (directorapi.WorldState, error) {
	if d.at >= len(d.screens) {
		return d.screens[len(d.screens)-1], nil
	}
	return d.screens[d.at], nil
}

func (d *scrollingDesktop) move(_ context.Context, m search.Move) error {
	sem, ok := m.Action.(directorapi.SemanticAction)
	if !ok {
		return errors.New("not a semantic action")
	}
	d.scrolls = append(d.scrolls, sem.Scroll)
	if d.failAt > 0 && len(d.scrolls) >= d.failAt {
		return errors.New("the region does not expose scrolling")
	}
	switch sem.Scroll {
	case directorapi.ScrollTop:
		d.at = 0
	default:
		if d.stuck {
			return nil
		}
		if d.at < len(d.screens)-1 {
			d.at++
		}
	}
	return nil
}

// screen is a readable window holding the named buttons, plus one scrollable pane.
//
// The pane is load-bearing. The search's move names no control — it names the CAPABILITY
// `Scrollable`, as ADR-140 built it to — so a window with nothing scrollable in it is a window
// where the move itself cannot resolve.
func screen(labels ...string) directorapi.WorldState {
	els := map[directorapi.ElementID]*directorapi.Element{
		"pane": {
			ID: "pane", Role: directorapi.RolePane, WindowID: "hwnd:1",
			Enabled: true, Visible: true, Confidence: 1,
			Bounds:     directorapi.Rect{X: 0, Y: 0, Width: 400, Height: 300},
			Attributes: map[string]any{"patterns": "scroll", "native_id": "pane"},
		},
	}
	for i, l := range labels {
		id := directorapi.ElementID("b" + strings.ToLower(l))
		els[id] = &directorapi.Element{
			ID: id, Label: l, Role: directorapi.RoleButton, WindowID: "hwnd:1",
			Enabled: true, Visible: true, Confidence: 1,
			Bounds:     directorapi.Rect{X: 10, Y: 10 + 30*i, Width: 80, Height: 24},
			Attributes: map[string]any{"patterns": "invoke", "native_id": string(id)},
		}
	}
	w := directorapi.WorldState{
		Timestamp: t0,
		Elements:  els,
		Windows:   []directorapi.Window{{ID: "hwnd:1", Application: "fixture", Focused: true}},
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 1, Coverage: 1, Actionability: 1, Freshness: 1,
		},
	}
	active := directorapi.WindowID("hwnd:1")
	w.ActiveWindow = &active
	return w
}

// lookRig is a pipeline over a scrolling desktop, with a searcher wired the way the Director
// wires one: observing through the same observer, moving through the same executor.
type lookRig struct {
	desktop  *scrollingDesktop
	acted    []directorapi.Action
	pipeline *Pipeline
}

func newLookRig(d *scrollingDesktop) *lookRig {
	r := &lookRig{desktop: d}
	ops := &fakeOperations{}
	exec := &Executor{
		Actuator:   &fakeActuator{},
		Focus:      &fakeFocuser{},
		Operations: ops,
		Resolve: func(ref directorapi.ElementReference) (directorapi.ResolvedTarget, error) {
			w, _ := d.observe(context.Background())
			if el, ok := w.Element(ref.ID); ok {
				native, _ := el.Attributes["native_id"].(string)
				return directorapi.ResolvedTarget{
					ElementID: el.ID, WindowID: el.WindowID, Point: el.ClickPoint(),
					Role: el.Role, Label: el.Label, NativeID: native,
					Control: directorapi.FactsOf(el),
				}, nil
			}
			// A reference carrying a capability query rather than an id — which is what the
			// search's move carries. Resolved against the reading in front of Marco now.
			if ref.Query != nil {
				res := target.NewResolver().Resolve(&w, *ref.Query)
				if res.Status == directorapi.ResolutionResolved {
					return *res.Target, nil
				}
			}
			return directorapi.ResolvedTarget{}, errors.New("the target is no longer on screen")
		},
	}
	tick := 0
	r.pipeline = &Pipeline{
		Observe:     d.observe,
		Intent:      intent.New().Parse,
		Resolver:    target.NewResolver(),
		Planner:     plan.New(),
		Policy:      testPolicy(),
		Verifier:    verify.New(),
		Graph:       actiongraph.NewMemory(),
		SettleDelay: time.Millisecond,
		Now: func() time.Time {
			tick++
			return t0.Add(time.Duration(tick) * time.Second)
		},
		// Recording what reaches the desktop, so a test can say what was performed without
		// anything being performed.
		Executor: &recordingExecutor{inner: exec, acted: &r.acted},
		// THE SEARCHER, wired as the Director wires it: the same observer a command reads
		// through, and the same executor the plan acts through. A search with its own route
		// to either would be a second Director with weaker rules.
		Searcher: search.New(d.observe, func(ctx context.Context, m search.Move) error {
			return d.move(ctx, m)
		}),
		// A click on an unfamiliar control needs confirming, and these tests are not about
		// the confirmation gate. Answering yes leaves the search as the only thing being
		// measured.
		Confirmer: &recordingConfirmer{answer: true},
	}
	return r
}

// recordingExecutor passes everything through and keeps a list.
type recordingExecutor struct {
	inner directorapi.DirectorExecutor
	acted *[]directorapi.Action
}

func (e *recordingExecutor) Execute(ctx context.Context, a directorapi.Action) (
	directorapi.ExecutionOutcome, error) {
	*e.acted = append(*e.acted, a)
	return e.inner.Execute(ctx, a)
}

// TestTheDirectorLooksForATargetThatIsNotInTheReading is the capability.
//
// "Click Publish" against a window whose first screen has no Publish. The resolver answers
// `absent` — read, and not here — and the request scrolls, re-reads, resolves and carries on.
// Nobody had to break it into "scroll down" and then "click Publish", which is the whole of what
// the continuation is for.
//
// Deleting the lookElsewhere call from Pipeline.handleParsed must fail this.
func TestTheDirectorLooksForATargetThatIsNotInTheReading(t *testing.T) {
	d := &scrollingDesktop{screens: []directorapi.WorldState{
		screen("Save", "Cancel"),
		screen("Details", "Help"),
		screen("Publish", "Discard"),
	}}
	r := newLookRig(d)

	// A FIRST READING THAT HONESTLY DOES NOT CONTAIN IT, asserted rather than assumed. If the
	// resolver ever answered something else for this fixture, everything below would be
	// measuring a different recovery.
	first := target.NewResolver().Resolve(&d.screens[0], directorapi.ElementQuery{Label: "Publish"})
	if first.Status != directorapi.ResolutionAbsent {
		t.Fatalf("the first screen resolves Publish as %s; this test needs `absent`, which is "+
			"the one status that means read-and-not-here", first.Status)
	}

	out := r.pipeline.Handle(context.Background(), "click Publish")

	if len(d.scrolls) == 0 {
		t.Fatal("THE DIRECTOR DID NOT GO AND LOOK.\n\n" +
			"A target absent from the first reading was refused without the searcher being " +
			"consulted. A searcher that is built and never asked is a seventh recorded case " +
			"of production never invoking a working mechanism.")
	}
	if out.Resolution == nil || out.Resolution.Status != directorapi.ResolutionResolved {
		t.Fatalf("after searching, the target resolves as %v (%q) — the search found the "+
			"screen and the pipeline did not carry on from it", out.Resolution, out.Message)
	}
	if out.Resolution.Target.Label != "Publish" {
		t.Fatalf("resolved to %q after the search", out.Resolution.Target.Label)
	}

	// AND IT CARRIED ON: one request, no instruction to scroll, and the action lands on the
	// control that was two screens away.
	var acted bool
	for _, a := range r.acted {
		if strings.Contains(a.Describe(), "Publish") {
			acted = true
		}
	}
	if !acted {
		t.Errorf("nothing was performed on Publish (%s: %q). Performed: %v",
			out.Status, out.Message, describeAll(r.acted))
	}
	// AND IT DID NOT SCROLL AWAY FROM WHAT IT FOUND.
	if d.at != 2 {
		t.Errorf("the view is at screen %d, not where the target was found", d.at)
	}
	for _, s := range d.scrolls {
		if s == directorapi.ScrollTop {
			t.Error("a successful search scrolled back to the top, away from what it found")
		}
	}
}

// TestSomethingAlreadyOnScreenIsNotSearchedFor — the search is a recovery, not a preamble.
//
// A target that resolves in the first reading must never cost a scroll. A Director that scrolled
// before acting would move the view on every single request.
func TestSomethingAlreadyOnScreenIsNotSearchedFor(t *testing.T) {
	d := &scrollingDesktop{screens: []directorapi.WorldState{
		screen("Save", "Cancel"), screen("Publish"),
	}}
	r := newLookRig(d)

	r.pipeline.Handle(context.Background(), "click Save")
	if len(d.scrolls) != 0 {
		t.Errorf("a target that was already on screen cost %d scroll(s): %v",
			len(d.scrolls), d.scrolls)
	}
}

// TestAnUnreadableWindowIsNotSearched is the line between the two recoveries.
//
// `unobservable` means Marco could not read the place it is in. Moving the view would be looking
// harder at something it cannot see: it would spend the budget producing more unreadable places
// and then report `unreadable`, which is what it already knew. The existing recovery for that
// status is to read the SAME place again, and it stays the only one.
func TestAnUnreadableWindowIsNotSearched(t *testing.T) {
	// A window of containers and no content — the reading that produced `unobservable` when
	// it was measured, and the fixture ADR-137 is about.
	thin := directorapi.WorldState{
		Timestamp: t0,
		Elements: map[directorapi.ElementID]*directorapi.Element{
			"p1": {
				ID: "p1", Role: directorapi.RolePane, WindowID: "hwnd:1",
				Enabled: true, Visible: true, Confidence: 1,
				Bounds: directorapi.Rect{X: 0, Y: 0, Width: 400, Height: 300},
			},
		},
		Windows: []directorapi.Window{{ID: "hwnd:1", Application: "fixture", Focused: true}},
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 0.2, Coverage: 0.1, Actionability: 0, Freshness: 1,
		},
	}
	active := directorapi.WindowID("hwnd:1")
	thin.ActiveWindow = &active

	d := &scrollingDesktop{screens: []directorapi.WorldState{thin, screen("Publish")}}
	r := newLookRig(d)

	res := target.NewResolver().Resolve(&thin, directorapi.ElementQuery{Label: "Publish"})
	if res.Status != directorapi.ResolutionUnobservable {
		t.Skipf("this fixture now resolves as %s; the test needs `unobservable`", res.Status)
	}

	r.pipeline.Handle(context.Background(), "click Publish")
	if len(d.scrolls) != 0 {
		t.Errorf("an unreadable window was searched (%d scrolls). Scrolling a window Marco "+
			"cannot read produces more places it cannot read, and then claims to have "+
			"looked.", len(d.scrolls))
	}
}

// TestASearchThatFoundNothingSaysSoMoreStronglyThanOneLookCould.
//
// An exhausted search is a far stronger claim than a failed reading, and the sentence has to be
// the stronger one — otherwise the search is work the person's screen paid for and they were
// never told about. "Nothing matching Publish is present in the observed window" is true of one
// reading; what was established is that it is not anywhere scrolling could reach.
func TestASearchThatFoundNothingSaysSoMoreStronglyThanOneLookCould(t *testing.T) {
	d := &scrollingDesktop{screens: []directorapi.WorldState{
		screen("Save", "Cancel"), screen("Details", "Help"),
	}}
	r := newLookRig(d)

	out := r.pipeline.Handle(context.Background(), "click Publish")
	if out.Status == directorapi.ResultDone {
		t.Fatalf("a target that is nowhere was reported as %s", out.Status)
	}
	if !strings.Contains(out.Message, "scrolling") {
		t.Errorf("the refusal does not account for the search: %q\n\nThe person's screen "+
			"scrolled and came back. A sentence that does not mention it describes a "+
			"Director that did less than it did.", out.Message)
	}
	// AND THE VIEW WENT BACK, which is the part the person can see.
	if d.at != 0 {
		t.Errorf("the view was left at screen %d after an unsuccessful search", d.at)
	}
	if len(d.scrolls) == 0 || d.scrolls[len(d.scrolls)-1] != directorapi.ScrollTop {
		t.Errorf("the search did not scroll back to the top: %v", d.scrolls)
	}
}

// TestAnUnreadablePlaceStopsTheSearchFromClaimingAbsence is [[ADR-006-unknown-is-not-false]] at
// the level of a whole request.
//
// One place that could not be read is one place the thing could have been. The sentence has to
// leave that open, because the remedy differs: "it is not there" sends somebody to look
// elsewhere, and "I could not read part of it" sends them to bring the window forward.
func TestAnUnreadablePlaceStopsTheSearchFromClaimingAbsence(t *testing.T) {
	blank := directorapi.WorldState{
		Timestamp: t0,
		Windows:   []directorapi.Window{{ID: "hwnd:1", Application: "fixture", Focused: true}},
	}
	active := directorapi.WindowID("hwnd:1")
	blank.ActiveWindow = &active

	d := &scrollingDesktop{screens: []directorapi.WorldState{
		screen("Save", "Cancel"), blank, blank, screen("Details"),
	}}
	r := newLookRig(d)

	out := r.pipeline.Handle(context.Background(), "click Publish")
	if strings.Contains(out.Message, "not anywhere scrolling could reach") {
		t.Errorf("a search that could not read part of what it looked at claimed absence: %q",
			out.Message)
	}
}

// TestAMoveThatFailedDoesNotBecomeAFindingAboutTheScreen.
//
// The region refused to scroll, so the places beyond it were never looked at, and any sentence
// about them would be about somewhere nobody went.
func TestAMoveThatFailedDoesNotBecomeAFindingAboutTheScreen(t *testing.T) {
	d := &scrollingDesktop{
		screens: []directorapi.WorldState{screen("Save"), screen("Publish")},
		failAt:  1,
	}
	r := newLookRig(d)

	out := r.pipeline.Handle(context.Background(), "click Publish")
	if strings.Contains(out.Message, "not anywhere scrolling could reach") {
		t.Errorf("a search whose first move failed claimed to have looked everywhere: %q",
			out.Message)
	}
}

func describeAll(as []directorapi.Action) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Describe())
	}
	return out
}
