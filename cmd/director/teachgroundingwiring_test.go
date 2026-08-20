package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The teaching side of pointing, entered where production enters it.
//
// The rule this repository pays for: a wiring test goes in through the production constructor. The
// coordinator's own tests prove that a grounding is asked the right question and that a failed one
// changes nothing — but they inject the grounding themselves, so they would all still pass if the
// production start never installed one and no teach session could ever point at anything.

// TestTeachingInstallsGroundingThroughTheProductionStart enters at teaching.start.
//
// Deleting `.WithGrounding(ground)` must fail this.
func TestTeachingInstallsGroundingThroughTheProductionStart(t *testing.T) {
	tc := &teaching{}
	g := &countingGround{}
	p := &oneTeachPass{}

	if _, err := tc.start("open downloads", p, &recognisingMemory{}, nil, g,
		windowref.Selector{EphemeralID: "window_1"}, "Downloads", "Open"); err != nil {
		t.Fatalf("starting: %v", err)
	}
	defer func() { _, _ = tc.stop() }()

	waitFor(t, func() bool { return g.count() > 0 })

	s, _ := tc.read()
	if s.StartReferent == nil {
		t.Fatal("the start was established and nothing was grounded. Every coordinator test " +
			"injects its own grounding, so this is the only place that notices when the " +
			"production start stops installing one")
	}
	if s.StartState == "" {
		t.Error("the start was grounded with no screen pinned")
	}
}

// TestAGroundedTeachViewCarriesDesktopRectangles holds the conversion.
//
// Regions are window-relative all the way through Director. A view that handed a surface those
// numbers would draw a highlight in the top-left corner of the display, and it would do it
// confidently. The conversion happens once, in pkg/referent, against the frame the measurement was
// taken with — and this is the test that the teach path actually goes through it.
func TestAGroundedTeachViewCarriesDesktopRectangles(t *testing.T) {
	desk := &focusDesktop{windows: []windowref.Candidate{
		{ID: "hwnd:11", Handle: 11, ProcessID: 3, Application: "code.exe",
			Bounds: rect(200, 100, 1000, 800), Visible: true, OnScreen: true,
			Foreground: true},
	}}
	dir := windowref.NewDirectory()
	sel := windowref.Selector{EphemeralID: dir.Adopt(desk.windows[0])}
	rt := &Runtime{
		observations: newObservationRegistry(), teach: &teaching{},
		winPlatform: desk, winDirectory: dir,
	}
	rt.teachGround = newTeachGrounding(rt)
	rt.teachGround.frames[observe.ReferentTeachStart] = sampledFrame{
		Sequence: 4, Bounds: directorapi.Rect{X: 200, Y: 100, Width: 1000, Height: 800},
	}

	s := teach.Session{
		Start: "subj_start", StartState: "state_1",
		StartReferent: &observe.VisualReferent{
			Role: observe.ReferentTeachStart, About: "2 controls I recognise this screen by",
			Regions: []observe.Region{
				{X: 0.1, Y: 0.2, Width: 0.1, Height: 0.05},
				{X: 0.5, Y: 0.6, Width: 0.1, Height: 0.05},
			},
		},
	}
	views := rt.groundingViews(context.Background(), s, sel)
	if len(views) != 1 {
		t.Fatalf("%d endpoint view(s), want one for the start", len(views))
	}
	v := views[0]
	if !v.CanShow() {
		t.Fatalf("the start could not be shown: unavailable=%q unmappable=%q",
			v.Unavailable, v.Unmappable)
	}
	if len(v.Boxes) != len(s.StartReferent.Regions) {
		t.Fatalf("%d box(es) for %d region(s). One box per member is the whole point; a "+
			"single bounding box would claim everything between them",
			len(v.Boxes), len(s.StartReferent.Regions))
	}
	// The window sits at x=200,y=100. A region 10% across it must land there, not at 10% of
	// the display — the failure a missing conversion produces, which looks plausible on a
	// maximised window and is obviously wrong on this one.
	if v.Boxes[0].X < 200 || v.Boxes[0].Y < 100 {
		t.Errorf("the first box is at (%d,%d), which is outside the window it belongs to. "+
			"These are window-relative numbers being drawn as desktop ones",
			v.Boxes[0].X, v.Boxes[0].Y)
	}
	if !strings.HasPrefix(v.Say, "START") {
		t.Errorf("the sentence reads %q; it should name the endpoint", v.Say)
	}
}

// TestAWindowThatMovedSinceGroundingIsNotDrawnAtTheOldPlace holds the freshness refusal.
//
// The measurement and the current position are read from two different places on purpose. If a
// person drags their window between "hold still a moment" and the highlight, every rectangle is
// stale by exactly that much — and a highlight that is nearly right is read as exactly right.
func TestAWindowThatMovedSinceGroundingIsNotDrawnAtTheOldPlace(t *testing.T) {
	desk := &focusDesktop{windows: []windowref.Candidate{
		{ID: "hwnd:11", Handle: 11, ProcessID: 3, Application: "code.exe",
			Bounds: rect(200, 100, 1000, 800), Visible: true, OnScreen: true,
			Foreground: true},
	}}
	dir := windowref.NewDirectory()
	sel := windowref.Selector{EphemeralID: dir.Adopt(desk.windows[0])}
	rt := &Runtime{
		observations: newObservationRegistry(), teach: &teaching{},
		winPlatform: desk, winDirectory: dir,
	}
	rt.teachGround = newTeachGrounding(rt)
	rt.teachGround.frames[observe.ReferentTeachStart] = sampledFrame{
		Sequence: 4, Bounds: directorapi.Rect{X: 200, Y: 100, Width: 1000, Height: 800},
	}
	s := teach.Session{
		Start: "subj_start", StartState: "state_1",
		StartReferent: &observe.VisualReferent{
			Role:    observe.ReferentTeachStart,
			Regions: []observe.Region{{X: 0.1, Y: 0.2, Width: 0.1, Height: 0.05}},
		},
	}

	// The user drags the window.
	desk.windows[0].Bounds = rect(600, 400, 1000, 800)

	views := rt.groundingViews(context.Background(), s, sel)
	if len(views) != 1 {
		t.Fatalf("%d endpoint view(s), want one", len(views))
	}
	if views[0].CanShow() {
		t.Fatalf("drew %d box(es) against a window that has moved since the measurement",
			len(views[0].Boxes))
	}
	if views[0].Unmappable == "" {
		t.Error("the refusal carries no reason, so nothing can tell a moved window from a " +
			"start that was never grounded")
	}
}

// ── doubles ───────────────────────────────────────────────────────────────────

// recognisingMemory recognises wherever it is asked about, so the start is established and the
// session gets as far as grounding it. Everything else is the empty stub's behaviour.
type recognisingMemory struct{ stubTeachMemory }

func (m *recognisingMemory) Recall(string, observe.StructureSignature) observe.Recollection {
	return observe.Recollection{
		Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{ID: "subj_start"},
	}
}

// countingGround is a grounding that always points, and counts.
type countingGround struct {
	mu sync.Mutex
	n  int
}

func (g *countingGround) Ground(_ observe.ShadowTotals, application string,
	_ observe.ScreenStateID, role observe.ReferentRole) observe.VisualReferent {

	g.mu.Lock()
	g.n++
	g.mu.Unlock()
	return observe.VisualReferent{
		Role: role, Application: application, About: "3 controls",
		Regions: []observe.Region{{X: 0.1, Y: 0.1, Width: 0.1, Height: 0.1}},
	}
}

func (g *countingGround) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.n
}

// oneTeachPass returns a single placed session and then blocks, so the start is established and
// the session sits still while the test reads it.
type oneTeachPass struct {
	mu   sync.Mutex
	done int
}

func (p *oneTeachPass) Observe(ctx context.Context, _ time.Duration) (
	observesession.Result, error) {

	p.mu.Lock()
	n := p.done
	p.done++
	p.mu.Unlock()
	if n > 0 {
		<-ctx.Done()
		return observesession.Result{}, ctx.Err()
	}
	return observesession.Result{
		Session: observe.Session{ID: "observe_1", Application: "code.exe"},
		Stats: observesession.Stats{
			SamplesTaken: 12,
			Shadow: observe.ShadowTotals{
				CurrentState: "state_1",
				States: []observe.ScreenState{{
					ID: "state_1", Episodes: 3, TermObservations: 2,
					Roles: map[string]int{"button": 4},
				}},
			},
		},
	}, nil
}

func (p *oneTeachPass) Finish() {}

func (p *oneTeachPass) AwaitSubject(context.Context) error { return nil }
