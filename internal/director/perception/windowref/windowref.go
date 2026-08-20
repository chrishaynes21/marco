// Package windowref keeps the Director's idea of "the window" honest.
//
// A window handle is an EXECUTION REFERENCE, not identity. It is valid for as long as the
// window exists and not one moment longer, the operating system is free to hand the same
// number to something else afterwards, and nothing about it survives the application being
// closed and reopened. Treating one as durable is how a Director ends up describing the
// wrong screen with complete confidence.
//
// # The incident this exists for
//
// Rocket League was closed and relaunched. The Director went on holding the destroyed
// handle, and because the live-bounds lookup for a dead window simply fails, the capture
// path fell back to the bounds it had cached — a rectangle on a different monitor. The
// detector then found 169 real UI elements there, and every diagnostic attributed them to
// Rocket League. Nothing errored. Nothing looked wrong. The evidence was real pixels,
// correctly detected, from a window that had not existed for minutes.
//
// The lesson is in the governing rule: valid pixels from the wrong window are invalid
// evidence, and no pixels at all is the better answer.
//
// # What this package does
//
//	validate  →  is the reference I hold still the thing I think it is?
//	invalidate → forget it completely, atomically
//	reacquire →  find the application's CURRENT window, by identity, never by old geometry
//	epoch     →  number each distinct window so evidence cannot silently span two of them
//
// It contains no Windows code. Platform is the seam, so the whole lifecycle is testable
// without a desktop — which matters, because the interesting states (destroyed, recycled
// handle, ambiguous candidates) are all but impossible to stage live on demand.
package windowref

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// State is the outcome of validating a reference.
//
// Distinct values rather than one "capture failed", because they call for different
// answers: a destroyed window should be reacquired, an ambiguous one should be reported to
// a person, and a recycled handle is a near-miss worth being loud about.
type State string

const (
	// Valid: the handle still names the expected window of the expected process.
	Valid State = "valid"
	// Destroyed: the handle no longer names a window at all.
	Destroyed State = "destroyed"
	// OwnershipChanged: the handle names a window belonging to a different process now.
	// The operating system recycles handle numbers; this is that, caught.
	OwnershipChanged State = "ownership_changed"
	// ProcessExited: the window's process is gone.
	ProcessExited State = "process_exited"
	// BoundsUnavailable: the window exists but will not report a usable rectangle.
	BoundsUnavailable State = "bounds_unavailable"
	// OffScreen: the bounds fall outside every current monitor. A window parked at
	// (-32000,-32000) is Windows' way of saying "minimized", and capturing there returns
	// whatever happens to be nowhere.
	OffScreen State = "off_screen"
	// Replaced: the application has a window, but not this one — it closed this and
	// opened another.
	Replaced State = "replaced"
	// Ambiguous: several equally plausible windows. Refusing to choose is the honest
	// answer; picking one arbitrarily would be a coin toss presented as a fact.
	Ambiguous State = "ambiguous"
	// Unavailable: the application has no window that can be captured.
	Unavailable State = "unavailable"
	// Unknown: the platform could not answer. Never treated as valid.
	Unknown State = "unknown"
)

// OK reports whether a frame may be captured under this state.
//
// Exactly one state passes. Written as an allow-list rather than a list of failures so a
// state added later is refused until somebody decides otherwise, which is the safe
// direction for a predicate that gates screen capture.
func (s State) OK() bool { return s == Valid }

// Ref is a live reference to one window.
//
// EPHEMERAL. Never serialise it as identity, never persist it, never compare two of them
// across an application restart and conclude anything. Handle is a number the OS may reuse;
// what makes this trustworthy is that it is validated immediately before use and carries
// the ProcessID that validation checked it against.
type Ref struct {
	// ID is the Director's window identifier, e.g. "hwnd:661516".
	ID directorapi.WindowID
	// Handle is the raw platform handle. Diagnostics must not show this by default —
	// see Describe.
	Handle uintptr
	// ProcessID owns the window. The field that makes handle recycling detectable.
	ProcessID uint32
	// Application is the semantic identity that DOES survive a restart, and the thing
	// reacquisition searches by.
	Application string
	// Title as last read, for ranking and diagnostics only. Never for identity: titles
	// change while a window lives.
	Title string
	// Bounds as validation last confirmed them. Refreshed by every validation; never
	// carried across one that failed.
	Bounds directorapi.Rect
	// Generation is the epoch this reference belongs to — see Tracker.
	Generation uint64
	// ValidatedAt is when the platform last confirmed all of the above.
	ValidatedAt time.Time
}

// Zero reports whether the reference names nothing.
func (r Ref) Zero() bool { return r.Handle == 0 && r.ID == "" }

// Describe renders a reference for a person.
//
// No raw handle. A handle in a diagnostic invites a reader to treat it as identity and
// compare it with one from five minutes ago, which is the whole mistake this package
// exists to prevent. The generation is the number that IS meaningful across time.
func (r Ref) Describe() string {
	if r.Zero() {
		return "no window"
	}
	app := r.Application
	if app == "" {
		app = "unknown application"
	}
	return fmt.Sprintf("%s window, generation %d, at %dx%d+%d+%d",
		app, r.Generation, r.Bounds.Width, r.Bounds.Height, r.Bounds.X, r.Bounds.Y)
}

// Validation is one validation, with the evidence for its verdict.
type Validation struct {
	State State
	// Ref is the reference as validation found it — bounds refreshed, process
	// confirmed. Zero unless State is Valid.
	Ref Ref
	// Reason is a sentence a person can act on. Always set when State is not Valid.
	Reason string
	// Previous is the generation that was abandoned, when this validation invalidated
	// one. Zero otherwise.
	Previous uint64
	// Reacquired is true when this Ref came from searching for the application rather
	// than from confirming the reference passed in.
	Reacquired bool
	// ProcessChanged is true when reacquisition landed on a different process — an
	// application restart rather than a new window in the same one.
	ProcessChanged bool
}

// Candidate is a window the platform is offering as the application's.
type Candidate struct {
	ID          directorapi.WindowID
	Handle      uintptr
	ProcessID   uint32
	Application string
	Title       string
	Bounds      directorapi.Rect
	Foreground  bool
	Visible     bool
	Minimized   bool
	// OnScreen is whether the bounds intersect any current monitor.
	OnScreen bool
}

// Platform is the live desktop, as this package needs it.
//
// Every method must answer from CURRENT platform state. An implementation that answered
// from a cache would defeat the entire package, which is why each is documented as a
// question about right now.
type Platform interface {
	// Live reports what the handle currently is: whether a window exists, which
	// process owns it, and where it is at this instant. ok is false when no window
	// has that handle.
	Live(ctx context.Context, handle uintptr) (Candidate, bool)
	// ProcessAlive reports whether the process currently exists.
	ProcessAlive(ctx context.Context, pid uint32) bool
	// Candidates lists the current top-level windows belonging to an application.
	// Empty is a legitimate answer: an application can be running with no window.
	Candidates(ctx context.Context, application string) []Candidate
}

// Tracker owns the Director's current window reference and its epoch.
//
// One per Director. Safe for concurrent use: diagnostics, the observe cycle and a running
// command all ask it questions, and the answer must never be half-updated — a caller that
// saw a new generation with the old bounds would capture exactly the wrong rectangle.
type Tracker struct {
	platform Platform
	now      func() time.Time

	mu sync.RWMutex
	// current is the last reference that PASSED validation, and the only one anything
	// may capture from.
	current Ref
	// proposed is an unvalidated suggestion from the last observed world. Kept apart
	// from current so a suggestion can never be mistaken for a validated fact — see
	// Propose.
	proposed   Ref
	generation uint64
	// lastState is what the most recent validation concluded, for diagnostics.
	lastState State
	lastWhy   string
}

// NewTracker returns a tracker over a platform.
func NewTracker(p Platform) *Tracker {
	return &Tracker{platform: p, now: time.Now, lastState: Unknown}
}

// WithClock replaces the clock, for tests.
func (t *Tracker) WithClock(now func() time.Time) *Tracker {
	t.now = now
	return t
}

// Current returns the reference as last validated, and whether one is held.
//
// NOT proof of anything. It is what was true at ValidatedAt, which is why nothing captures
// from it without calling Acquire first.
func (t *Tracker) Current() (Ref, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.current, !t.current.Zero()
}

// Generation is the current epoch.
func (t *Tracker) Generation() uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.generation
}

// LastState is what the most recent validation concluded, and why.
func (t *Tracker) LastState() (State, string) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastState, t.lastWhy
}

// Invalidate forgets the current reference completely.
//
// ATOMIC from a consumer's point of view: after this returns, no caller can obtain the old
// handle or the old bounds from this tracker. That is the property that makes the fallback
// which caused the incident impossible — there is nothing left to fall back TO.
func (t *Tracker) Invalidate(why string) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.invalidateLocked(why)
}

func (t *Tracker) invalidateLocked(why string) uint64 {
	previous := t.current.Generation
	t.current = Ref{}
	t.lastWhy = why
	return previous
}

// Propose offers an unvalidated candidate — typically whatever the last observed world
// says is in front.
//
// It never makes anything valid. It only changes which reference the next Acquire will
// CHECK, and Acquire refuses or reacquires exactly as it would otherwise. That is what
// makes it safe to feed a snapshot in here: a snapshot is a description of the past, and
// the only thing done with it is to ask the platform whether it is still true.
//
// A proposal is kept SEPARATE from the validated reference, and that separation is
// load-bearing. An earlier version overwrote the current reference with the proposal, which
// made the two indistinguishable — so a proposal that then validated was treated as the
// reference already held, adopt was never reached, and every window in the running Director
// reported generation 0 forever. The epoch is only meaningful if it is assigned exclusively
// on the path where something passed validation.
func (t *Tracker) Propose(r Ref) {
	if r.Handle == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.proposed = Ref{
		ID: r.ID, Handle: r.Handle, ProcessID: r.ProcessID,
		Application: r.Application, Title: r.Title, Bounds: r.Bounds,
	}
}

// checkFirst is the reference Acquire should validate: the proposal when there is a
// relevant one, otherwise whatever was last validated.
//
// A proposal wins because it is the fresher observation — the person switched windows and
// the world noticed. It still proves nothing; it only decides what gets checked.
func (t *Tracker) checkFirst(application string) Ref {
	p := t.proposed
	if p.Handle == 0 {
		return t.current
	}
	if application != "" && p.Application != "" && !strings.EqualFold(p.Application, application) {
		return t.current
	}
	if t.current.Handle == p.Handle {
		// Same window, already validated: keep the validated copy so its generation
		// survives.
		return t.current
	}
	return p
}

// Acquire returns a reference that is valid RIGHT NOW for the given application.
//
// The one entry point anything about to capture should use. It validates what is held,
// reacquires when that fails, and reports honestly when it cannot. It never returns a
// reference it has not just confirmed against the platform.
//
// An empty application means "whatever is held", which only works when something is held —
// there is nothing to search for otherwise.
func (t *Tracker) Acquire(ctx context.Context, application string) Validation {
	t.mu.Lock()
	defer t.mu.Unlock()

	held := t.checkFirst(application)
	if !held.Zero() && (application == "" || strings.EqualFold(held.Application, application)) {
		if v, ok := t.confirm(ctx, held); ok {
			// Through adopt, always: it is the single place an epoch is assigned,
			// and a validated window that skipped it would report generation 0.
			t.adopt(v.Ref)
			v.Ref = t.current
			t.lastState, t.lastWhy = Valid, ""
			return v
		} else {
			// The held reference is gone. Forget it BEFORE searching, so a failed
			// reacquisition cannot leave the old one reachable.
			previous := t.invalidateLocked(v.Reason)
			t.lastState = v.State
			if application == "" {
				application = held.Application
			}
			re := t.reacquire(ctx, application, held)
			re.Previous = previous
			if !re.State.OK() {
				t.lastState, t.lastWhy = re.State, re.Reason
				return re
			}
			t.adopt(re.Ref)
			re.Ref = t.current
			t.lastState, t.lastWhy = Valid, ""
			return re
		}
	}

	if application == "" {
		t.lastState, t.lastWhy = Unavailable, "no window is being tracked and no application was named"
		return Validation{State: Unavailable, Reason: t.lastWhy}
	}

	previous := t.invalidateLocked("")
	re := t.reacquire(ctx, application, held)
	re.Previous = previous
	if !re.State.OK() {
		t.lastState, t.lastWhy = re.State, re.Reason
		return re
	}
	t.adopt(re.Ref)
	re.Ref = t.current
	t.lastState, t.lastWhy = Valid, ""
	return re
}

// Confirm re-checks the held reference against the live desktop and changes nothing.
//
// # Why this is separate from Acquire
//
// Acquire is what you call before ACTING: it validates, and where validation fails it
// reacquires and adopts, because a caller about to capture needs a window. This is what you
// call to ask a question about evidence already in hand — "is the window I was told this
// came from still the window I hold?" — and adopting there would be wrong twice over. It
// would change the Director's target as a side effect of checking a provider's homework,
// and, worse, it would advance the generation and so make the very staleness being tested
// for disappear at the moment of testing.
//
// So: no adopt, no reacquire, no epoch change, no lastState update. It reads the platform
// now and reports what it found.
//
// ok is false when nothing is held or the held reference no longer passes — which is
// precisely the condition that makes evidence about it unattributable.
func (t *Tracker) Confirm(ctx context.Context) (Ref, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.current.Zero() {
		return Ref{}, false
	}
	v, ok := t.confirm(ctx, t.current)
	if !ok {
		return Ref{}, false
	}
	// The epoch comes from what is HELD, not from the live read: confirm refreshes bounds
	// and title, and the generation is the one thing about a reference that a
	// re-validation must never invent.
	v.Ref.Generation = t.current.Generation
	return v.Ref, true
}

// adopt installs a freshly validated reference, starting a new epoch when the window
// actually changed.
//
// The generation increments on a different HANDLE or a different PROCESS, and not on a
// window that merely moved. Getting that distinction right is what makes the number worth
// carrying: a generation that changed every time a window was dragged would tell nobody
// anything, and one that survived a restart would be a lie.
func (t *Tracker) adopt(r Ref) {
	same := !t.current.Zero() &&
		t.current.Handle == r.Handle &&
		t.current.ProcessID == r.ProcessID
	if !same {
		t.generation++
	}
	r.Generation = t.generation
	t.current = r
}

// confirm checks a held reference against the live desktop.
//
// Every check is a question about NOW. Cached bounds are never evidence of anything: a
// destroyed window's last known rectangle is exactly the thing that must not be trusted.
func (t *Tracker) confirm(ctx context.Context, held Ref) (Validation, bool) {
	live, exists := t.platform.Live(ctx, held.Handle)
	if !exists {
		return Validation{
			State:  Destroyed,
			Reason: fmt.Sprintf("the %s window that was being tracked no longer exists", label(held.Application)),
		}, false
	}
	if held.ProcessID != 0 && live.ProcessID != held.ProcessID {
		return Validation{
			State: OwnershipChanged,
			Reason: fmt.Sprintf(
				"that window handle now belongs to another process (was %d, is %d) — "+
					"the system reuses handle numbers", held.ProcessID, live.ProcessID),
		}, false
	}
	if live.ProcessID != 0 && !t.platform.ProcessAlive(ctx, live.ProcessID) {
		return Validation{
			State:  ProcessExited,
			Reason: fmt.Sprintf("the process behind the %s window has exited", label(held.Application)),
		}, false
	}
	if held.Application != "" && live.Application != "" &&
		!strings.EqualFold(live.Application, held.Application) {
		return Validation{
			State: OwnershipChanged,
			Reason: fmt.Sprintf("that window belongs to %s now, not %s",
				live.Application, held.Application),
		}, false
	}
	if live.Bounds.Width <= 0 || live.Bounds.Height <= 0 {
		return Validation{
			State:  BoundsUnavailable,
			Reason: fmt.Sprintf("the %s window reports no usable size", label(held.Application)),
		}, false
	}
	if !live.OnScreen {
		return Validation{
			State: OffScreen,
			Reason: fmt.Sprintf(
				"the %s window is not on any monitor (it is at %d,%d — minimized windows park off-screen)",
				label(held.Application), live.Bounds.X, live.Bounds.Y),
		}, false
	}

	// Bounds refreshed from the platform, never carried over.
	return Validation{State: Valid, Ref: Ref{
		ID: live.ID, Handle: live.Handle, ProcessID: live.ProcessID,
		Application: firstNonEmpty(live.Application, held.Application),
		Title:       live.Title, Bounds: live.Bounds,
		Generation: held.Generation, ValidatedAt: t.now(),
	}}, true
}

// reacquire finds the application's current window by identity.
//
// Never by the old geometry, and never assuming the handle is unchanged. Those are the two
// shortcuts that would reintroduce the bug in a new place.
func (t *Tracker) reacquire(ctx context.Context, application string, previous Ref) Validation {
	if application == "" {
		return Validation{State: Unavailable, Reason: "no application to search for"}
	}
	candidates := usable(t.platform.Candidates(ctx, application))
	if len(candidates) == 0 {
		return Validation{
			State: Unavailable,
			Reason: fmt.Sprintf("no window belonging to %s is currently available to capture",
				application),
		}
	}

	best, tied := rank(candidates)
	if tied {
		return Validation{
			State: Ambiguous,
			Reason: fmt.Sprintf(
				"%s has %d windows and none is clearly the one meant; refusing to guess",
				application, len(candidates)),
		}
	}

	// The verdict a caller acts on is Valid: a reacquired window is as capturable as one
	// that never went away. That the old one was Replaced is carried by Reacquired and
	// ProcessChanged, which is what diagnostics report and what a freshness check reads.
	return Validation{
		State:          Valid,
		Reacquired:     true,
		ProcessChanged: previous.ProcessID != 0 && previous.ProcessID != best.ProcessID,
		Ref: Ref{
			ID: best.ID, Handle: best.Handle, ProcessID: best.ProcessID,
			Application: firstNonEmpty(best.Application, application),
			Title:       best.Title, Bounds: best.Bounds, ValidatedAt: t.now(),
		},
	}
}

// usable drops candidates nothing could capture from.
func usable(in []Candidate) []Candidate {
	out := make([]Candidate, 0, len(in))
	for _, c := range in {
		if c.Handle == 0 || c.Minimized || !c.OnScreen {
			continue
		}
		if c.Bounds.Width <= 0 || c.Bounds.Height <= 0 {
			continue
		}
		out = append(out, c)
	}
	return out
}

// rank picks the window most likely to be the one meant, and reports a tie.
//
// Deterministic and explicit, in this order:
//
//  1. the foreground window, if it belongs to the application — it is what the person is
//     looking at, and no other signal beats that;
//  2. the largest visible window — an application's main window is almost always its
//     biggest, and splash screens and tool palettes are not;
//  3. nothing, when the top two cannot be told apart.
//
// A tie is reported rather than broken. Two windows of identical area with no foreground
// among them is a genuine question about which the person meant, and answering it by
// enumeration order would be a coin toss dressed as a decision.
func rank(candidates []Candidate) (Candidate, bool) {
	if len(candidates) == 1 {
		return candidates[0], false
	}
	var foreground []Candidate
	for _, c := range candidates {
		if c.Foreground {
			foreground = append(foreground, c)
		}
	}
	if len(foreground) == 1 {
		return foreground[0], false
	}
	pool := candidates
	if len(foreground) > 1 {
		// More than one claims foreground: the platform is confused, so fall through to
		// area among those rather than trusting the flag.
		pool = foreground
	}

	sorted := make([]Candidate, len(pool))
	copy(sorted, pool)
	sort.SliceStable(sorted, func(i, j int) bool {
		return area(sorted[i].Bounds) > area(sorted[j].Bounds)
	})
	if len(sorted) > 1 && area(sorted[0].Bounds) == area(sorted[1].Bounds) {
		return Candidate{}, true
	}
	return sorted[0], false
}

func area(r directorapi.Rect) int { return r.Width * r.Height }

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func label(app string) string {
	if app == "" {
		return "tracked"
	}
	return app
}
