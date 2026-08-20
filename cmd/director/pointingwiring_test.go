package main

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The two joins pointing depends on, entered the way production enters them.
//
// Both of these are wires rather than algorithms, and a wire is exactly the kind of thing this
// repository has three recorded cases of building correctly and never connecting. A test that
// called `recordFrame` itself would pass while nothing called it.

// THE frame test. Sampling records the rectangle its geometry is a proportion of.
//
// Without this, every region downstream is a fraction of an unknown rectangle. `pkg/referent`
// refuses rather than guessing, so the visible symptom is not a wrong highlight — it is no
// highlight, ever, with a reason that points at the window rather than at the missing wire.
func TestPointingUsesTheFrameTheSampleWasNormalisedAgainst(t *testing.T) {
	rt := &Runtime{collector: &providers.Collector{}, engine: labelledEngine{}}
	sampler := rt.newObservationSampler(sessionClock).(*liveSampler)

	if _, ok := sampler.LastFrame(); ok {
		t.Fatal("a sampler that has taken no sample reported a frame. Pointing would then " +
			"map against a rectangle nothing measured")
	}

	req := sampleRequest()
	req.Window.Bounds = directorapi.Rect{X: -1928, Y: 40, Width: 1600, Height: 980}
	if _, err := sampler.Sample(context.Background(), req); err != nil {
		t.Fatalf("Sample: %v", err)
	}

	got, ok := sampler.LastFrame()
	if !ok {
		t.Fatal("sampling recorded no frame, so nothing downstream can be placed on the " +
			"screen. Deleting the recordFrame call in Sample is what this catches")
	}
	if got.Bounds != req.Window.Bounds {
		t.Fatalf("recorded %v, want the rectangle the runner validated, %v. The regions in "+
			"this sample are proportions of THAT rectangle and of nothing else",
			got.Bounds, req.Window.Bounds)
	}
	// The negative origin survives. A left-hand monitor is where this goes wrong silently:
	// clamping it to zero produces a plausible rectangle on the wrong screen.
	if got.Bounds.X >= 0 {
		t.Error("a negative desktop origin was not preserved")
	}
	if got.Window != req.Window.ID || got.Generation != req.Window.Generation ||
		got.Sequence != req.Sequence {
		t.Errorf("the frame does not identify the window and sample it came from: %+v", got)
	}
}

// The registry reaches that frame through its own sampler, not through a second path.
func TestTheRegistryReadsTheFrameFromTheSamplerThatTookIt(t *testing.T) {
	rt := &Runtime{collector: &providers.Collector{}, engine: labelledEngine{}}
	sampler := rt.newObservationSampler(sessionClock).(*liveSampler)

	g := newObservationRegistry()
	if _, ok := g.lastSampledFrame(); ok {
		t.Fatal("a registry with no sampler reported a frame")
	}
	g.lastSampler = sampler
	if _, ok := g.lastSampledFrame(); ok {
		t.Fatal("a sampler that has taken no sample reported a frame through the registry")
	}

	if _, err := sampler.Sample(context.Background(), sampleRequest()); err != nil {
		t.Fatalf("Sample: %v", err)
	}
	got, ok := g.lastSampledFrame()
	if !ok {
		t.Fatal("the registry could not reach the frame its own sampler recorded")
	}
	if got.Bounds != sampleRequest().Window.Bounds {
		t.Errorf("the registry read %v", got.Bounds)
	}
}

// A frame with no extent is a refusal, not a zero rectangle.
//
// Everything downstream divides by the width and the height. Reporting 0x0 as a usable frame
// would turn a missing measurement into an arithmetic accident somewhere further away.
func TestAFrameWithNoExtentIsRefused(t *testing.T) {
	rt := &Runtime{collector: &providers.Collector{}, engine: labelledEngine{}}
	sampler := rt.newObservationSampler(sessionClock).(*liveSampler)

	req := sampleRequest()
	req.Window.Bounds = directorapi.Rect{}
	if _, err := sampler.Sample(context.Background(), req); err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if f, ok := sampler.LastFrame(); ok {
		t.Fatalf("a %dx%d frame was offered as usable", f.Bounds.Width, f.Bounds.Height)
	}
}

// THE targeting test. Marco chooses the window, and then stops choosing.
//
// Two halves, and the second is the one that is easy to lose: resolving the foreground answers
// "what am I looking at", and adopting it as an ephemeral id is what stops the session re-asking
// that question on every validation and drifting onto whatever came forward. Handing the session a
// live foreground selector would pass any test that only checked the first half.
func TestTheChosenContextIsPinnedAndDoesNotFollowFocus(t *testing.T) {
	desk := &focusDesktop{
		windows: []windowref.Candidate{
			{ID: "hwnd:11", Handle: 11, ProcessID: 3, Application: "code.exe",
				Title: "marco", Bounds: rect(0, 0, 1600, 900), Visible: true,
				OnScreen: true, Foreground: true},
			{ID: "hwnd:22", Handle: 22, ProcessID: 4, Application: "chrome.exe",
				Title: "docs", Bounds: rect(100, 100, 800, 600), Visible: true,
				OnScreen: true},
		},
	}
	rt := &Runtime{winPlatform: desk, winDirectory: windowref.NewDirectory()}

	sel, err := rt.currentContext(context.Background())
	if err != nil {
		t.Fatalf("currentContext: %v", err)
	}
	if sel.EphemeralID == "" {
		t.Fatal("the chosen window was not pinned to an id. A selector that keeps asking " +
			"which window is in front is a session that follows focus, which is the exact " +
			"failure windowref exists to prevent")
	}
	if sel.Application != "" || sel.Title != "" || sel.ProcessID != 0 {
		t.Errorf("the selector carries a second, weaker way to match: %+v", sel)
	}

	// It resolves to the window that WAS in front.
	c, res, why := windowref.Resolve(context.Background(), desk, rt.winDirectory, sel)
	if !res.OK() {
		t.Fatalf("the pinned selector no longer resolves: %s (%s)", why, res)
	}
	if c.Handle != 11 {
		t.Fatalf("resolved to handle %d, want the window that was in front", c.Handle)
	}

	// Now somebody else comes forward. The pinned reference does NOT move.
	desk.windows[0].Foreground = false
	desk.windows[1].Foreground = true
	c, res, why = windowref.Resolve(context.Background(), desk, rt.winDirectory, sel)
	if !res.OK() {
		t.Fatalf("the pinned selector stopped resolving when focus moved: %s (%s)", why, res)
	}
	if c.Handle != 11 {
		t.Fatal("the session's target followed focus to another window. Everything it " +
			"observed after that would be attributed to the application the user chose")
	}
}

// Nothing pointable in front is said plainly rather than substituted for.
func TestNoForegroundIsARefusalAndNotAGuess(t *testing.T) {
	desk := &focusDesktop{windows: []windowref.Candidate{
		{ID: "hwnd:22", Handle: 22, ProcessID: 4, Application: "chrome.exe",
			Bounds: rect(100, 100, 800, 600), Visible: true, OnScreen: true},
	}}
	rt := &Runtime{winPlatform: desk, winDirectory: windowref.NewDirectory()}

	if sel, err := rt.currentContext(context.Background()); err == nil {
		t.Fatalf("with nothing in the foreground, Marco chose %s anyway", sel.Describe())
	}
}

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

// focusDesktop is a desktop whose foreground can be moved between assertions.
type focusDesktop struct{ windows []windowref.Candidate }

func (d *focusDesktop) AllCandidates(context.Context) []windowref.Candidate { return d.windows }

func (d *focusDesktop) Candidates(_ context.Context, app string) []windowref.Candidate {
	var out []windowref.Candidate
	for _, c := range d.windows {
		if c.Application == app {
			out = append(out, c)
		}
	}
	return out
}

func (d *focusDesktop) Live(_ context.Context, handle uintptr) (windowref.Candidate, bool) {
	for _, c := range d.windows {
		if c.Handle == handle {
			return c, true
		}
	}
	return windowref.Candidate{}, false
}

func (d *focusDesktop) ProcessAlive(_ context.Context, pid uint32) bool {
	for _, c := range d.windows {
		if c.ProcessID == pid {
			return true
		}
	}
	return false
}

var _ observesession.Sampler = (*liveSampler)(nil)
