package windowref_test

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A fake desktop.
//
// The states that matter here — a destroyed window, a handle recycled onto another
// process, two equally plausible candidates — are all but impossible to stage on a real
// desktop on demand, which is exactly why Platform is an interface.

type desktop struct {
	windows map[uintptr]windowref.Candidate
	dead    map[uint32]bool
}

func newDesktop() *desktop {
	return &desktop{windows: map[uintptr]windowref.Candidate{}, dead: map[uint32]bool{}}
}

func (d *desktop) open(handle uintptr, pid uint32, app string, r directorapi.Rect) windowref.Candidate {
	c := windowref.Candidate{
		ID: directorapi.WindowID(id(handle)), Handle: handle, ProcessID: pid,
		Application: app, Title: app, Bounds: r,
		Visible: true, OnScreen: true,
	}
	d.windows[handle] = c
	return c
}

func (d *desktop) close(handle uintptr) { delete(d.windows, handle) }

func (d *desktop) exit(pid uint32) {
	d.dead[pid] = true
	for h, c := range d.windows {
		if c.ProcessID == pid {
			delete(d.windows, h)
		}
	}
}

func (d *desktop) foreground(handle uintptr) {
	for h, c := range d.windows {
		c.Foreground = h == handle
		d.windows[h] = c
	}
}

func (d *desktop) Live(_ context.Context, handle uintptr) (windowref.Candidate, bool) {
	c, ok := d.windows[handle]
	return c, ok
}

func (d *desktop) ProcessAlive(_ context.Context, pid uint32) bool { return !d.dead[pid] }

func (d *desktop) Candidates(_ context.Context, app string) []windowref.Candidate {
	var out []windowref.Candidate
	// Deterministic order, so a test that expects a REFUSAL cannot pass by luck.
	for _, h := range sortedHandles(d.windows) {
		if c := d.windows[h]; c.Application == app {
			out = append(out, c)
		}
	}
	return out
}

func sortedHandles(m map[uintptr]windowref.Candidate) []uintptr {
	out := make([]uintptr, 0, len(m))
	for h := range m {
		out = append(out, h)
	}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func id(h uintptr) string {
	digits := ""
	if h == 0 {
		return "hwnd:0"
	}
	for n := h; n > 0; n /= 10 {
		digits = string(rune('0'+n%10)) + digits
	}
	return "hwnd:" + digits
}

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

func tracker(d *desktop) *windowref.Tracker {
	return windowref.NewTracker(d).WithClock(func() time.Time {
		return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	})
}

// ── the incident ──────────────────────────────────────────────────────────────

// TestTheRocketLeagueRestartDefect reproduces the live failure exactly.
//
//	Rocket League at handle A, bounds X
//	→ closed; A destroyed
//	→ reopened as handle B, bounds Y, new process
//	→ capture
//
// Before this package, the Director kept A, the live-bounds lookup for a dead window
// failed, and the capture path fell back to X — a rectangle on another monitor. The
// detector found 169 real elements there and every diagnostic called them Rocket League's.
func TestTheRocketLeagueRestartDefect(t *testing.T) {
	const (
		app     = "rocketleague"
		handleA = uintptr(661516)
		handleB = uintptr(662844)
		pidA    = uint32(38060)
		pidB    = uint32(10236)
	)
	boundsX := rect(-1920, 0, 1920, 1080) // the other monitor
	boundsY := rect(0, 0, 1920, 1080)

	d := newDesktop()
	d.open(handleA, pidA, app, boundsX)
	d.foreground(handleA)
	tr := tracker(d)

	first := tr.Acquire(context.Background(), app)
	if !first.State.OK() {
		t.Fatalf("the original window would not validate: %v %s", first.State, first.Reason)
	}
	if first.Ref.Handle != handleA || first.Ref.Bounds != boundsX {
		t.Fatalf("first acquire = %+v, want handle A at bounds X", first.Ref)
	}
	genA := first.Ref.Generation

	// The game is closed and relaunched somewhere else entirely.
	d.exit(pidA)
	d.open(handleB, pidB, app, boundsY)
	d.foreground(handleB)

	got := tr.Acquire(context.Background(), app)

	if !got.State.OK() {
		t.Fatalf("after the restart the window would not validate: %v %s", got.State, got.Reason)
	}
	if got.Ref.Handle == handleA {
		t.Fatal("the destroyed handle A was returned; this is the defect")
	}
	if got.Ref.Handle != handleB {
		t.Fatalf("handle = %d, want the new window B (%d)", got.Ref.Handle, handleB)
	}
	if got.Ref.Bounds == boundsX {
		t.Fatal("bounds X were reused — capture would read the wrong monitor, " +
			"which is exactly what happened live")
	}
	if got.Ref.Bounds != boundsY {
		t.Fatalf("bounds = %v, want the new window's %v", got.Ref.Bounds, boundsY)
	}
	if got.Ref.ProcessID != pidB {
		t.Fatalf("process = %d, want the relaunched %d", got.Ref.ProcessID, pidB)
	}
	if !got.Reacquired {
		t.Error("the validation does not report that it reacquired")
	}
	if !got.ProcessChanged {
		t.Error("a restart onto a new process is not reported as a process change")
	}
	if got.Ref.Generation == genA {
		t.Fatalf("generation stayed %d across a restart; evidence from two different "+
			"windows would look continuous", genA)
	}
	if got.Previous != genA {
		t.Errorf("Previous = %d, want the abandoned generation %d", got.Previous, genA)
	}
}

// TestADestroyedWindowWhoseProcessSurvivesIsCaughtByLivenessAlone
//
// The restart test above passes even with the liveness check removed, because the
// process-exit check catches it too — defence in depth, and not a reason to trust either
// alone. This is the case only liveness can catch, and it is not exotic: a game recreates
// its window on a display-mode change, and Rocket League does exactly that when focus
// moves in exclusive fullscreen.
//
// The assertion is the incident's shape: the old rectangle must never come back.
func TestADestroyedWindowWhoseProcessSurvivesIsCaughtByLivenessAlone(t *testing.T) {
	const app, pid = "rocketleague", uint32(38060)
	oldBounds := rect(-1920, 0, 1920, 1080)
	newBounds := rect(0, 0, 1920, 1080)

	d := newDesktop()
	d.open(661516, pid, app, oldBounds)
	tr := tracker(d)
	if v := tr.Acquire(context.Background(), app); !v.State.OK() {
		t.Fatalf("setup: %v", v.State)
	}

	// The window goes; the process stays and opens another one somewhere else.
	d.close(661516)
	d.open(662844, pid, app, newBounds)

	got := tr.Acquire(context.Background(), app)
	if !got.State.OK() {
		t.Fatalf("state = %v (%s)", got.State, got.Reason)
	}
	if got.Ref.Handle == 661516 {
		t.Fatal("the destroyed handle survived; only the liveness check catches this")
	}
	if got.Ref.Bounds == oldBounds {
		t.Fatal("the old rectangle was returned — this is wrong-region capture, " +
			"the exact failure the gate exists to stop")
	}
	if got.Ref.Bounds != newBounds {
		t.Fatalf("bounds = %v, want %v", got.Ref.Bounds, newBounds)
	}
	if got.ProcessChanged {
		t.Error("the process did not change; reporting that it did would misdescribe the event")
	}
	if got.Ref.Generation < 2 {
		t.Errorf("generation = %d, want a new epoch for a different window", got.Ref.Generation)
	}
}

// TestNoWindowIsBetterThanTheWrongWindow is the governing rule, as a test.
func TestNoWindowIsBetterThanTheWrongWindow(t *testing.T) {
	const app = "rocketleague"
	d := newDesktop()
	d.open(661516, 38060, app, rect(-1920, 0, 1920, 1080))
	tr := tracker(d)
	if v := tr.Acquire(context.Background(), app); !v.State.OK() {
		t.Fatalf("setup: %v", v.State)
	}

	// Closed, and NOT relaunched.
	d.exit(38060)

	got := tr.Acquire(context.Background(), app)
	if got.State.OK() {
		t.Fatalf("a frame was authorised with no window: %+v", got.Ref)
	}
	if got.State != windowref.Unavailable {
		t.Errorf("state = %q, want %q", got.State, windowref.Unavailable)
	}
	if !got.Ref.Zero() {
		t.Fatalf("a reference survived a failed acquisition: %+v", got.Ref)
	}
	if held, ok := tr.Current(); ok {
		t.Fatalf("the tracker still holds %+v after invalidation; "+
			"no caller may be able to reach the old handle", held)
	}
}

// ── validation ────────────────────────────────────────────────────────────────

func TestAValidWindowValidates(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "notepad", rect(10, 20, 800, 600))
	tr := tracker(d)

	got := tr.Acquire(context.Background(), "notepad")
	if !got.State.OK() {
		t.Fatalf("state = %v (%s)", got.State, got.Reason)
	}
	if got.Ref.Bounds != rect(10, 20, 800, 600) {
		t.Errorf("bounds = %v", got.Ref.Bounds)
	}
	if got.Ref.ValidatedAt.IsZero() {
		t.Error("a validated reference has no validation time")
	}
}

func TestARecycledHandleIsCaught(t *testing.T) {
	// Windows reuses handle numbers. A reference whose handle now belongs to another
	// process is the most dangerous state there is: it validates as "a window exists"
	// and captures somebody else's pixels.
	d := newDesktop()
	d.open(100, 7, "notepad", rect(0, 0, 800, 600))
	tr := tracker(d)
	if v := tr.Acquire(context.Background(), "notepad"); !v.State.OK() {
		t.Fatalf("setup: %v", v.State)
	}

	// Same handle number, different process and application.
	d.close(100)
	d.open(100, 99, "calculator", rect(0, 0, 400, 300))

	got := tr.Acquire(context.Background(), "notepad")
	if got.State.OK() {
		t.Fatal("a recycled handle validated; another application's pixels would be " +
			"captured and attributed to notepad")
	}
	if got.State != windowref.OwnershipChanged && got.State != windowref.Unavailable {
		t.Errorf("state = %q, want ownership_changed (or unavailable after the search)", got.State)
	}
}

func TestAHandleRecycledWithinTheSameApplicationIsCaught(t *testing.T) {
	// The case the process check alone can catch, isolated deliberately.
	//
	// TestARecycledHandleIsCaught above also changes the application name, so the
	// application check catches it and the PID check could be deleted unnoticed. Here the
	// name is identical — a second instance of the same program, handed the number the
	// first one gave back — and only the process ID distinguishes them.
	d := newDesktop()
	first := rect(0, 0, 800, 600)
	d.open(100, 7, "app", first)
	tr := tracker(d)
	if v := tr.Acquire(context.Background(), "app"); !v.State.OK() {
		t.Fatalf("setup: %v", v.State)
	}

	// Same handle number, same application, different process — and somewhere else.
	d.close(100)
	second := rect(-1920, 0, 1024, 768)
	d.open(100, 42, "app", second)

	got := tr.Acquire(context.Background(), "app")
	if !got.State.OK() {
		t.Fatalf("state = %v (%s)", got.State, got.Reason)
	}
	if got.Ref.ProcessID != 42 {
		t.Errorf("process = %d, want the new owner 42", got.Ref.ProcessID)
	}
	if !got.ProcessChanged {
		t.Error("a handle that changed hands is not reported as a process change")
	}
	if got.Ref.Generation < 2 {
		t.Errorf("generation = %d; a different process behind the same handle number "+
			"is a different window and must start a new epoch", got.Ref.Generation)
	}
}

func TestBoundsAreRefreshedNotRemembered(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "notepad", rect(0, 0, 800, 600))
	tr := tracker(d)
	if v := tr.Acquire(context.Background(), "notepad"); !v.State.OK() {
		t.Fatalf("setup: %v", v.State)
	}

	// The window moves to another monitor.
	moved := rect(-1920, 40, 1024, 768)
	d.open(100, 7, "notepad", moved)

	got := tr.Acquire(context.Background(), "notepad")
	if got.Ref.Bounds != moved {
		t.Fatalf("bounds = %v, want the refreshed %v", got.Ref.Bounds, moved)
	}
	if got.Ref.Generation != 1 {
		t.Errorf("generation = %d; a window that merely moved is the same window",
			got.Ref.Generation)
	}
}

func TestAnOffScreenWindowIsRefused(t *testing.T) {
	// Windows parks minimized windows at (-32000,-32000). Capturing there returns
	// whatever is nowhere.
	d := newDesktop()
	c := d.open(100, 7, "rocketleague", rect(-32000, -32000, 160, 28))
	c.OnScreen = false
	d.windows[100] = c
	tr := tracker(d)

	got := tr.Acquire(context.Background(), "rocketleague")
	if got.State.OK() {
		t.Fatal("an off-screen window was authorised for capture")
	}
}

func TestEmptyBoundsAreRefused(t *testing.T) {
	d := newDesktop()
	c := d.open(100, 7, "app", rect(0, 0, 0, 0))
	d.windows[100] = c
	tr := tracker(d)

	if got := tr.Acquire(context.Background(), "app"); got.State.OK() {
		t.Fatal("a window with no size was authorised for capture")
	}
}

func TestOnlyValidPassesTheGate(t *testing.T) {
	// An allow-list, so a state added later is refused until somebody decides otherwise.
	for _, s := range []windowref.State{
		windowref.Destroyed, windowref.OwnershipChanged, windowref.ProcessExited,
		windowref.BoundsUnavailable, windowref.OffScreen, windowref.Replaced,
		windowref.Ambiguous, windowref.Unavailable, windowref.Unknown, windowref.State("brand new"),
	} {
		if s.OK() {
			t.Errorf("%q passes the capture gate", s)
		}
	}
	if !windowref.Valid.OK() {
		t.Error("valid does not pass the capture gate")
	}
}

// ── reacquisition ─────────────────────────────────────────────────────────────

func TestAReplacementWindowInTheSameProcessIsFound(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	tr := tracker(d)
	if v := tr.Acquire(context.Background(), "app"); !v.State.OK() {
		t.Fatalf("setup: %v", v.State)
	}

	d.close(100)
	d.open(200, 7, "app", rect(50, 50, 900, 700))

	got := tr.Acquire(context.Background(), "app")
	if !got.State.OK() {
		t.Fatalf("state = %v (%s)", got.State, got.Reason)
	}
	if got.Ref.Handle != 200 {
		t.Errorf("handle = %d, want the replacement 200", got.Ref.Handle)
	}
	if got.ProcessChanged {
		t.Error("a new window in the SAME process is reported as a process change")
	}
	if got.Ref.Generation != 2 {
		t.Errorf("generation = %d, want 2 — a different window is a new epoch", got.Ref.Generation)
	}
}

func TestTheForegroundWindowWins(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 1920, 1080)) // bigger
	d.open(200, 7, "app", rect(0, 0, 400, 300))
	d.foreground(200) // but not in front
	tr := tracker(d)

	got := tr.Acquire(context.Background(), "app")
	if !got.State.OK() {
		t.Fatalf("state = %v (%s)", got.State, got.Reason)
	}
	if got.Ref.Handle != 200 {
		t.Errorf("handle = %d, want the foreground window 200", got.Ref.Handle)
	}
}

func TestTheLargestWindowWinsWithoutAForeground(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 400, 300))
	d.open(200, 7, "app", rect(0, 0, 1920, 1080))
	tr := tracker(d)

	got := tr.Acquire(context.Background(), "app")
	if !got.State.OK() {
		t.Fatalf("state = %v (%s)", got.State, got.Reason)
	}
	if got.Ref.Handle != 200 {
		t.Errorf("handle = %d, want the largest window 200", got.Ref.Handle)
	}
}

func TestTwoEqualCandidatesAreAmbiguousRatherThanGuessed(t *testing.T) {
	// Answering this by enumeration order would be a coin toss presented as a fact.
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	d.open(200, 7, "app", rect(100, 100, 800, 600))
	tr := tracker(d)

	got := tr.Acquire(context.Background(), "app")
	if got.State.OK() {
		t.Fatalf("one of two equally plausible windows was chosen: %+v", got.Ref)
	}
	if got.State != windowref.Ambiguous {
		t.Errorf("state = %q, want %q", got.State, windowref.Ambiguous)
	}
	if got.Reason == "" {
		t.Error("an ambiguous result explains nothing")
	}
}

func TestMinimizedAndOffScreenCandidatesAreNotCapturable(t *testing.T) {
	d := newDesktop()
	c := d.open(100, 7, "app", rect(-32000, -32000, 160, 28))
	c.Minimized, c.OnScreen = true, false
	d.windows[100] = c
	tr := tracker(d)

	got := tr.Acquire(context.Background(), "app")
	if got.State != windowref.Unavailable {
		t.Fatalf("state = %q, want unavailable — a minimized window has no pixels", got.State)
	}
}

func TestAWindowOnANegativeCoordinateMonitorIsFine(t *testing.T) {
	// A monitor left of the primary has a negative origin, and capture there was
	// verified live. Refusing it would break a two-monitor desktop.
	d := newDesktop()
	d.open(100, 7, "app", rect(-1920, 0, 1920, 1080))
	tr := tracker(d)

	got := tr.Acquire(context.Background(), "app")
	if !got.State.OK() {
		t.Fatalf("a window on the left-hand monitor was refused: %v (%s)", got.State, got.Reason)
	}
	if got.Ref.Bounds.X != -1920 {
		t.Errorf("bounds = %v, want the negative origin preserved", got.Ref.Bounds)
	}
}

// ── epochs ────────────────────────────────────────────────────────────────────

func TestAValidatedWindowAlwaysHasAnEpoch(t *testing.T) {
	// Caught live rather than here: the running Director reported "generation 0" for a
	// perfectly valid window, because Propose overwrote the validated reference and the
	// confirm path then treated the proposal as already-adopted, skipping the one place
	// an epoch is assigned. Generation 0 means "nothing has ever been validated", so it
	// must never appear next to a window that just was.
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	tr := tracker(d)

	tr.Propose(windowref.Ref{
		ID: "hwnd:100", Handle: 100, Application: "app", Bounds: rect(0, 0, 800, 600),
	})
	got := tr.Acquire(context.Background(), "app")

	if !got.State.OK() {
		t.Fatalf("state = %v (%s)", got.State, got.Reason)
	}
	if got.Ref.Generation == 0 {
		t.Fatal("a validated window reports generation 0, which means never validated")
	}
	if tr.Generation() == 0 {
		t.Fatal("the tracker's generation stayed 0 after a successful validation")
	}
}

func TestProposingTheSameWindowRepeatedlyDoesNotChurnTheEpoch(t *testing.T) {
	// Every observation cycle proposes. If that moved the generation, the number would
	// change constantly and mean nothing.
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	tr := tracker(d)

	var generations []uint64
	for i := 0; i < 4; i++ {
		tr.Propose(windowref.Ref{
			ID: "hwnd:100", Handle: 100, Application: "app", Bounds: rect(0, 0, 800, 600),
		})
		v := tr.Acquire(context.Background(), "app")
		if !v.State.OK() {
			t.Fatalf("pass %d: %v", i, v.State)
		}
		generations = append(generations, v.Ref.Generation)
	}
	for i, g := range generations {
		if g != generations[0] {
			t.Fatalf("generation moved across identical proposals: %v (pass %d)", generations, i)
		}
	}
}

func TestAProposalIsNeverMistakenForAValidatedFact(t *testing.T) {
	// A proposal is a description of the past. Proposing a window that does not exist
	// must not make the tracker claim to hold one.
	d := newDesktop()
	tr := tracker(d)

	tr.Propose(windowref.Ref{
		ID: "hwnd:999", Handle: 999, Application: "ghost", Bounds: rect(0, 0, 800, 600),
	})

	if held, ok := tr.Current(); ok {
		t.Fatalf("a mere proposal became the held reference: %+v", held)
	}
	got := tr.Acquire(context.Background(), "ghost")
	if got.State.OK() {
		t.Fatalf("a proposal for a non-existent window validated: %+v", got.Ref)
	}
}

func TestGenerationIsStableWhileTheWindowIs(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	tr := tracker(d)

	first := tr.Acquire(context.Background(), "app")
	second := tr.Acquire(context.Background(), "app")
	third := tr.Acquire(context.Background(), "app")

	if first.Ref.Generation != second.Ref.Generation || second.Ref.Generation != third.Ref.Generation {
		t.Fatalf("generation moved without the window changing: %d, %d, %d",
			first.Ref.Generation, second.Ref.Generation, third.Ref.Generation)
	}
}

func TestInvalidateIsAtomicFromAConsumersView(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	tr := tracker(d)
	if v := tr.Acquire(context.Background(), "app"); !v.State.OK() {
		t.Fatalf("setup: %v", v.State)
	}

	tr.Invalidate("closing for a test")

	if held, ok := tr.Current(); ok {
		t.Fatalf("Current still returns %+v after Invalidate", held)
	}
}

func TestTheLastStateIsReportable(t *testing.T) {
	d := newDesktop()
	d.open(100, 7, "app", rect(0, 0, 800, 600))
	tr := tracker(d)
	_ = tr.Acquire(context.Background(), "app")
	d.exit(7)
	_ = tr.Acquire(context.Background(), "app")

	state, why := tr.LastState()
	if state.OK() {
		t.Fatalf("last state = %q after the process exited", state)
	}
	if why == "" {
		t.Error("the tracker cannot say why the window is unavailable")
	}
}

// ── diagnostics ───────────────────────────────────────────────────────────────

func TestDescribeDoesNotLeakTheHandle(t *testing.T) {
	// A handle in a diagnostic invites a reader to treat it as identity and compare it
	// with one from five minutes ago — the whole mistake this package prevents.
	r := windowref.Ref{
		ID: "hwnd:661516", Handle: 661516, ProcessID: 38060,
		Application: "rocketleague", Bounds: rect(0, 0, 1920, 1080), Generation: 4,
	}
	got := r.Describe()
	for _, leak := range []string{"661516", "hwnd"} {
		if contains(got, leak) {
			t.Errorf("Describe() = %q, which exposes %q", got, leak)
		}
	}
	if !contains(got, "generation 4") {
		t.Errorf("Describe() = %q, want the generation, which IS meaningful over time", got)
	}
}

func contains(haystack, needle string) bool {
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
