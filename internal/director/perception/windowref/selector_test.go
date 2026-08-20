package windowref_test

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// Choosing a window on purpose, and keeping it.
//
// The gap these close was found the hard way: three live attempts at Rocket League all
// captured VS Code or the terminal, because running a diagnostic gives the terminal focus
// and the Director looked at whatever was in front.

// AllCandidates makes the fake desktop searchable by title and process, as the real
// platform is.
func (d *desktop) AllCandidates(_ context.Context) []windowref.Candidate {
	var out []windowref.Candidate
	for _, h := range sortedHandles(d.windows) {
		out = append(out, d.windows[h])
	}
	return out
}

func TestExactlyOnePrimarySelectorIsRequired(t *testing.T) {
	if err := (windowref.Selector{}).Validate(); err == nil {
		t.Error("an empty selector validated")
	}
	if err := (windowref.Selector{Application: "app", Title: "x"}).Validate(); err == nil {
		t.Error("two primaries validated; there would be nothing to disambiguate them")
	}
	for _, s := range []windowref.Selector{
		{Application: "app"}, {Title: "x"}, {EphemeralID: "window_1"}, {ProcessID: 7},
	} {
		if err := s.Validate(); err != nil {
			t.Errorf("%s did not validate: %v", s.Describe(), err)
		}
	}
}

func TestSelectingByApplication(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "rocketleague", rect(0, 0, 1920, 1080))
	d.open(200, 9, "code", rect(0, 0, 800, 600))
	d.foreground(200) // the terminal is in front, as it always is

	got, res, why := windowref.Resolve(context.Background(), d, nil,
		windowref.Selector{Application: "rocketleague"})
	if !res.OK() {
		t.Fatalf("resolve = %v (%s)", res, why)
	}
	if got.Application != "rocketleague" {
		t.Fatalf("selected %q; focus must not decide this", got.Application)
	}
}

func TestSelectingByTitle(t *testing.T) {
	d := newDesktop()
	c := d.open(100, 7, "rocketleague", rect(0, 0, 1920, 1080))
	c.Title = "Rocket League (64-bit, DX11, Cooked)"
	d.windows[100] = c
	d.open(200, 9, "code", rect(0, 0, 800, 600))

	got, res, why := windowref.Resolve(context.Background(), d, nil,
		windowref.Selector{Title: "rocket league"})
	if !res.OK() {
		t.Fatalf("resolve = %v (%s)", res, why)
	}
	if got.Handle != 100 {
		t.Fatalf("selected handle %d, want 100", got.Handle)
	}
}

func TestSelectingByProcess(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	d.open(200, 9, "app", rect(0, 0, 400, 300))

	got, res, why := windowref.Resolve(context.Background(), d, nil,
		windowref.Selector{ProcessID: 9})
	if !res.OK() {
		t.Fatalf("resolve = %v (%s)", res, why)
	}
	if got.ProcessID != 9 {
		t.Fatalf("selected process %d, want 9", got.ProcessID)
	}
}

func TestAnAmbiguousSelectorAsksForABetterOne(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	d.open(200, 7, "app", rect(100, 100, 800, 600))

	_, res, why := windowref.Resolve(context.Background(), d, nil,
		windowref.Selector{Application: "app"})
	if res.OK() {
		t.Fatal("one of two matches was chosen by enumeration order")
	}
	if res != windowref.AmbiguousSelector {
		t.Errorf("resolution = %q, want ambiguous", res)
	}
	if !strings.Contains(why, "window-id") {
		t.Errorf("the explanation %q does not say how to be more specific", why)
	}
}

func TestNothingMatchingIsNotFound(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))

	_, res, _ := windowref.Resolve(context.Background(), d, nil,
		windowref.Selector{Application: "ghost"})
	if res != windowref.NotFound {
		t.Errorf("resolution = %q, want not_found", res)
	}
}

func TestAnEphemeralIDResolvesWhileItsGenerationLasts(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	dir := windowref.NewDirectory()

	listed := dir.List(context.Background(), d, "")
	if len(listed) != 1 {
		t.Fatalf("listed %d windows, want 1", len(listed))
	}
	if listed[0].ID == "" {
		t.Fatal("no ephemeral id was issued")
	}

	got, res, why := windowref.Resolve(context.Background(), d, dir,
		windowref.Selector{EphemeralID: listed[0].ID})
	if !res.OK() {
		t.Fatalf("resolve = %v (%s)", res, why)
	}
	if got.Handle != 100 {
		t.Errorf("handle = %d, want 100", got.Handle)
	}
}

func TestAnEphemeralIDGoesStaleWithItsWindow(t *testing.T) {
	// An id names one generation. Silently attaching to whatever replaced it would make
	// an explicitly chosen window mean "whatever came next" — the durable-identity
	// mistake in a new costume.
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	dir := windowref.NewDirectory()
	listed := dir.List(context.Background(), d, "")

	d.close(100)
	d.open(200, 7, "app", rect(0, 0, 800, 600)) // a replacement, same application

	_, res, why := windowref.Resolve(context.Background(), d, dir,
		windowref.Selector{EphemeralID: listed[0].ID})
	if res.OK() {
		t.Fatal("a stale id silently attached to the replacement window")
	}
	if res != windowref.StaleEphemeralID {
		t.Errorf("resolution = %q, want stale_id", res)
	}
	if !strings.Contains(why, "director windows") {
		t.Errorf("the explanation %q does not say how to get a fresh id", why)
	}
}

func TestAListingDoesNotLeakHandles(t *testing.T) {
	d := newDesktop()
	d.open(661516, 38060, "rocketleague", rect(0, 0, 1920, 1080))
	dir := windowref.NewDirectory()

	for _, l := range dir.List(context.Background(), d, "") {
		if strings.Contains(l.ID, "661516") {
			t.Errorf("the ephemeral id %q contains the raw handle", l.ID)
		}
		if strings.Contains(l.ID, "hwnd") {
			t.Errorf("the ephemeral id %q exposes the platform reference", l.ID)
		}
	}
}

// ── focus independence, the point of all of this ──────────────────────────────

func TestFocusChangesDoNotRetargetASelectedWindow(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "rocketleague", rect(0, 0, 1920, 1080))
	d.open(200, 9, "code", rect(0, 0, 1200, 800))
	d.foreground(100) // the game is in front to begin with
	tr := tracker(d)

	first := tr.AcquireBy(context.Background(), d, nil,
		windowref.Selector{Application: "rocketleague"})
	if !first.State.OK() {
		t.Fatalf("selecting the game failed: %v (%s)", first.State, first.Reason)
	}

	// The person alt-tabs to the terminal — which is what running any diagnostic does.
	d.foreground(200)

	second := tr.AcquireBy(context.Background(), d, nil,
		windowref.Selector{Application: "rocketleague"})
	if !second.State.OK() {
		t.Fatalf("the game stopped resolving once the terminal took focus: %v (%s)",
			second.State, second.Reason)
	}
	if second.Ref.Application != "rocketleague" {
		t.Fatalf("observation followed focus to %q; this is the defect the selector "+
			"exists to remove", second.Ref.Application)
	}
	if second.Ref.Generation != first.Ref.Generation {
		t.Errorf("the generation changed (%d → %d) although the window did not",
			first.Ref.Generation, second.Ref.Generation)
	}
}

func TestSelectingByApplicationFollowsARestart(t *testing.T) {
	// The executable name is exactly the identity that survives a restart, so following
	// one is what the user asked for.
	d := newDesktop()
	d.open(100, 7, "rocketleague", rect(-1920, 0, 1920, 1080))
	tr := tracker(d)
	first := tr.AcquireBy(context.Background(), d, nil,
		windowref.Selector{Application: "rocketleague"})
	if !first.State.OK() {
		t.Fatalf("setup: %v", first.State)
	}

	d.exit(7)
	d.open(200, 42, "rocketleague", rect(0, 0, 1920, 1080))

	got := tr.AcquireBy(context.Background(), d, nil,
		windowref.Selector{Application: "rocketleague"})
	if !got.State.OK() {
		t.Fatalf("a restarted application did not resolve: %v (%s)", got.State, got.Reason)
	}
	if got.Ref.Bounds == (rect(-1920, 0, 1920, 1080)) {
		t.Fatal("the old bounds came back")
	}
	if !got.ProcessChanged {
		t.Error("a restart onto a new process is not reported as one")
	}
	if got.Ref.Generation == first.Ref.Generation {
		t.Error("the generation did not change across a restart")
	}
}

func TestSelectingByProcessCannotOutliveIt(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	tr := tracker(d)
	if v := tr.AcquireBy(context.Background(), d, nil,
		windowref.Selector{ProcessID: 7}); !v.State.OK() {
		t.Fatalf("setup: %v", v.State)
	}

	d.exit(7)
	d.open(200, 42, "app", rect(0, 0, 800, 600)) // same program, new process

	got := tr.AcquireBy(context.Background(), d, nil, windowref.Selector{ProcessID: 7})
	if got.State.OK() {
		t.Fatal("a process selector attached to a different process")
	}
	if held, ok := tr.Current(); ok {
		t.Fatalf("the tracker still holds %+v", held)
	}
}
