// Package navsource turns physical input into navigation MEANING, and destroys the physical
// part before it can travel.
//
// # The governing rule
//
// Marco should remember what the user meant to do, not which plastic button they happened to
// press. A raw key code may exist inside this package for as long as it takes to classify it.
// It may not leave. The only value that crosses out is observe.InputEvent, whose closed intent
// vocabulary cannot express "the user pressed S".
//
// # Why the boundary is here and not further in
//
// Because a boundary that depends on every caller's diligence is not a boundary. rawEvent is
// unexported, has no String method, is never logged, and never appears in a struct that leaves
// the package — so a future edit that wanted to leak a key code would have to export a new type
// to do it, which is a visible act rather than a silent one.
//
// TestRawKeyIdentityCannotCrossTheBoundary asserts the shape of what escapes, by field name and
// by type, so adding `RawKey int` fails even though an int is not text.
//
// # What is deliberately NOT captured
//
// Character keys. Not letters, not digits, not punctuation — not even as opaque codes. Passive
// discovery must be structurally incapable of reconstructing typed content, which means the
// classifier has no branch that could admit one. Text-entry learning, when it comes, is a
// separate explicit privilege with its own lifecycle events; see the note at the bottom of this
// file.
//
// # Threading
//
// The OS hook callback is the most latency-critical code in the product. Windows silently drops
// a low-level hook that overruns LowLevelHooksTimeout, and dropping it takes the overlay's F12
// stop key with it — see CLAUDE.md. So the callback does exactly one thing: a non-blocking
// offer into a bounded queue. Classification, repeat suppression, session gating and timestamps
// all happen on a worker goroutine.
package navsource

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// QueueDepth bounds the hook-to-worker handoff.
//
// Generous enough that ordinary play never reaches it, and bounded because the alternative is
// unbounded memory driven by how fast somebody can type. When it is full the offer FAILS and
// the event is dropped: input observation is advisory evidence, and losing one navigation
// event is strictly better than stalling a system-wide keyboard hook.
const QueueDepth = 256

// The reason vocabulary is the CONSUMER's, not this package's.
//
// observe.IgnoreReason is what crosses the boundary into a report, so it is defined where it
// lands. A producer that wanted to say something new has to add a constant to the Director's
// closed set to do it, rather than inventing a diagnostic string on this side.
type IgnoreReason = observe.IgnoreReason

const (
	// ReasonUnsupported is a key with no navigation meaning — the overwhelming majority,
	// including every character key.
	ReasonUnsupported = observe.IgnoreUnsupported
	// ReasonAmbiguous is a key that navigates in menus and moves the player in gameplay.
	// Recognised so the count is visible, and refused because the same physical input
	// means different things in different states and this layer cannot tell which.
	ReasonAmbiguous = observe.IgnoreAmbiguous
	// ReasonRepeat is auto-repeat while a key is held.
	ReasonRepeat = observe.IgnoreRepeat
	// ReasonNoSession is navigation observed while nothing was watching.
	ReasonNoSession = observe.IgnoreNoSession
	// ReasonUnplaceablePointer is a pointer press with no window to place it in.
	ReasonUnplaceablePointer = observe.IgnoreUnplaceablePointer
	// ReasonMarcoOwned is input aimed at Marco's own control surface.
	ReasonMarcoOwned = observe.IgnoreMarcoOwned
)

// Stats is what the producer did, in semantic terms only.
type Stats = observe.InputStats

// rawEvent is a physical event, and never leaves this package.
//
// No String method, deliberately. A type with one invites a log line, and a log line containing
// a key code is the failure this whole design is arranged to prevent.
type rawEvent struct {
	code uint16
	down bool
	at   time.Time
	// pointer marks a pointer press rather than a key. When set, `code` is meaningless and
	// x/y carry the screen position the press happened at.
	//
	// # Why the raw position is allowed to exist here and nowhere else
	//
	// A pointer press is only navigation if it can be placed inside the window Marco is
	// watching, and placing it needs the position it happened at. That position is an
	// ABSOLUTE desktop coordinate — the one thing this package exists to make sure never
	// leaves it. So it lives on the type that never leaves, is normalised on the worker
	// against the target's bounds, and the absolute pair is dropped on the floor.
	//
	// Nothing here observes movement. Only presses, and only while a hook is installed.
	pointer bool
	x, y    int32
}

// backend is the platform hook. Injectable so the queue, classifier and lifecycle are all
// testable without installing a system-wide hook.
type backend interface {
	// start begins delivering events to offer, which MUST be safe to call from an OS
	// callback and must never block. It returns false when the queue is full.
	start(offer func(rawEvent) bool) error
	stop()
	// unavailable explains why this platform produces nothing, empty when it works.
	unavailable() string
}

// Source is the live producer of navigation intents.
type Source struct {
	backend backend
	raw     chan rawEvent
	// why is why this platform produces nothing, empty when it works. Carried into Stats
	// so a report can tell "nobody pressed anything" from "nothing was listening".
	why string
	// drops counts offers the bounded queue refused.
	//
	// An atomic rather than a counter under s.mu, because it is incremented BY THE HOOK
	// THREAD inside offer. A lock there is the one thing this design is arranged to avoid:
	// Windows silently unhooks a low-level hook that overruns its timeout, and it takes the
	// overlay's F12 stop key with it.
	drops atomic.Int64

	mu    sync.Mutex
	sub   *Subscription
	stats Stats
	held  map[uint16]bool
	// screen is what the last observation said the screen looked like, and when.
	//
	// Guarded by mu and read ONLY from the classifier worker. The hook callback never
	// touches it — a context lookup on that thread is exactly the kind of work that gets a
	// low-level hook silently unhooked, and there is nothing the hook could do with it
	// anyway.
	screen screenContext
	// window is where the watched window was, and when that was confirmed.
	//
	// The frame a pointer press is made relative to. Guarded by mu and read only from the
	// worker, for the same reason as `screen`: asking the OS where a window is, on a
	// low-level hook callback, is how a hook gets silently dropped.
	window windowBounds
	// controls is what the watched window OFFERED, as of the last valid inference — the
	// actionable elements a press can be resolved against. Guarded by mu and read only from
	// the worker, like everything the hook must never touch. See SetActionables.
	controls actionables
	// owned is whether Marco's own surface held the foreground, as last confirmed.
	owned ownerContext

	done   chan struct{}
	closed bool
	wg     sync.WaitGroup
}

// New starts a navigation source, or returns one that explains why it cannot run.
func New() (*Source, string) { return newWith(newBackend()) }

// NewSynthetic returns a source driven by the returned function instead of by a hook.
//
// The press function takes a VIRTUAL-KEY CODE, because the thing worth testing is the whole
// path — admission, repeat suppression, session gating, classification — and a seam that
// accepted a NavIntent would skip exactly the code that enforces the privacy boundary. A test
// that fed intents in would prove the plumbing and nothing about the guarantee.
//
// The synthetic source installs no hook and observes no real keyboard.
func NewSynthetic() (*Source, func(code uint16, down bool)) {
	b := &syntheticBackend{}
	s, _ := newWith(b)
	return s, func(code uint16, down bool) {
		if b.offer != nil {
			b.offer(rawEvent{code: code, down: down, at: time.Now()})
		}
	}
}

// NewSyntheticPointer returns a source driven by pointer presses at ABSOLUTE positions.
//
// Absolute, deliberately, and for the same reason NewSynthetic takes a virtual-key code: the
// thing worth testing is the whole path, and a seam that accepted an already-normalised position
// would skip the placement rule that decides whether a press belongs to the watched window at
// all. A test feeding in window-relative coordinates would prove the plumbing and nothing about
// the guarantee.
//
// It installs no hook and observes no real mouse.
func NewSyntheticPointer() (*Source, func(x, y int32)) {
	b := &syntheticBackend{}
	s, _ := newWith(b)
	return s, func(x, y int32) {
		if b.offer != nil {
			b.offer(rawEvent{pointer: true, down: true, x: x, y: y, at: time.Now()})
		}
	}
}

type syntheticBackend struct{ offer func(rawEvent) bool }

func (syntheticBackend) unavailable() string { return "" }
func (b *syntheticBackend) start(o func(rawEvent) bool) error {
	b.offer = o
	return nil
}
func (b *syntheticBackend) stop() { b.offer = nil }

// VK codes for the admitted navigation keys, so a caller outside this package can drive the
// synthetic source without restating Windows constants.
const (
	KeyEscape = vkEscape
	KeyReturn = vkReturn
	KeyBack   = vkBack
	KeyUp     = vkUp
	KeyDown   = vkDown
	KeyLeft   = vkLeft
	KeyRight  = vkRight

	// The context-conditional keys, exported for the same reason: a test that restated
	// 0x57 would be asserting against a magic number rather than against the table.
	KeyW     = vkW
	KeyA     = vkA
	KeyS     = vkS
	KeyD     = vkD
	KeySpace = vkSpace
)

// newWith builds a source over a given backend, so the queue, classifier and lifecycle are
// exercised without installing a system-wide hook.
func newWith(b backend) (*Source, string) {
	if why := b.unavailable(); why != "" {
		return &Source{backend: b, done: make(chan struct{}), closed: true, why: why}, why
	}
	s := &Source{
		backend: b,
		raw:     make(chan rawEvent, QueueDepth),
		held:    map[uint16]bool{},
		done:    make(chan struct{}),
	}
	s.wg.Add(1)
	go s.classify()
	if err := b.start(s.offer); err != nil {
		s.Close()
		s.why = err.Error()
		return s, s.why
	}
	return s, ""
}

// offer is called FROM THE OS HOOK CALLBACK. It must never block and must do nothing else.
//
// A select with a default is the whole implementation on purpose. Anything more — a lock, a
// map lookup, a timestamp conversion — is work on a thread that Windows will unhook if it is
// slow, and every one of those is available on the other side of this channel.
func (s *Source) offer(e rawEvent) bool {
	select {
	case s.raw <- e:
		return true
	default:
		// One lock-free add, and the event is gone. Losing a navigation event is strictly
		// better than stalling a system-wide keyboard hook — but it must be COUNTED, or a
		// correlation thinned by backpressure reads as a player who pressed nothing.
		s.drops.Add(1)
		return false
	}
}

// classify is the worker. Everything that is not "put it in the queue" happens here.
func (s *Source) classify() {
	defer s.wg.Done()
	for {
		select {
		case <-s.done:
			return
		case e := <-s.raw:
			s.handle(e)
		}
	}
}

func (s *Source) handle(e rawEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.Received++

	if e.pointer {
		s.handlePointer(e)
		return
	}

	// Key-up only clears the held set. It is not navigation, and emitting on both edges
	// would double every intent.
	if !e.down {
		delete(s.held, e.code)
		return
	}
	// Auto-repeat. A low-level hook reports a held key as a stream of key-downs with no
	// repeat flag, so held state is the only way to tell one press from forty.
	if s.held[e.code] {
		s.ignore(ReasonRepeat)
		return
	}
	s.held[e.code] = true

	// The screen context is read HERE, on the worker, under the lock the classifier already
	// holds. Freshness is judged against the event's own timestamp rather than against now,
	// so a keypress that sat in the queue during a burst is admitted on the context that was
	// current when it happened.
	intent, conditional, reason := classifyKey(e.code, s.screen.fresh(e.at))
	if intent == "" {
		s.ignore(reason)
		return
	}
	if s.sub == nil {
		// Navigation observed while nothing was watching is discarded here rather than
		// buffered. Retention is session-bounded: a Director that accumulated intents
		// whenever it happened to be running would be keeping a record of the user's
		// keyboard for no session's benefit.
		s.ignore(ReasonNoSession)
		return
	}
	if s.owned.marco(e.at) {
		// Typing INTO Marco. See ReasonMarcoOwned: operating the thing that is watching
		// is not something it is being shown.
		s.ignore(ReasonMarcoOwned)
		return
	}
	s.stats.Classified++
	if conditional {
		s.stats.Conditional++
	}
	// A CONFIRM lands on whatever holds the keyboard's attention, and the last inference
	// may know which control that was. The same enrichment rule as a pointer press: the
	// keypress is admitted whatever this resolves to.
	var target *observe.SemanticTarget
	if intent == observe.NavConfirm {
		target = s.controls.focused(e.at)
	}
	s.sub.add(intent, e.at, conditional, observe.PointerAt{}, target)
}

// handlePointer turns a pointer press into a placed navigation, or refuses it.
//
// # Why a press is not automatically navigation
//
// Because "the pointer went down somewhere on the desktop" is not an observation about the
// application Marco is watching. It becomes one only when the press can be placed INSIDE that
// window — and placing it needs bounds recent enough to still describe where the window is.
// Without them the honest answer is that Marco saw a click and cannot say where, which is
// counted rather than admitted with a zero position.
//
// There is no held/repeat handling here on purpose. A held mouse button does not auto-repeat;
// the hook delivers one button-down per press, and treating a press like a key would introduce
// state that nothing sets.
func (s *Source) handlePointer(e rawEvent) {
	// SESSION FIRST, before placement. Nothing is watching, so nothing about where this
	// press landed is a fact worth reporting — and the frame is only pushed while a session
	// runs, so asking about placement first reports every press made before one started as
	// `unplaceable_pointer`.
	//
	// That misreported a live diagnosis: six clicks a person made around a 22-second
	// observation came back "unplaceable", which reads as "Marco could not place your click
	// inside the window" — a perception failure — when the truth was "you clicked before I
	// was watching". Two very different problems, and only one of them is Marco's.
	//
	// The keyboard path has always checked session membership at this point, for the same
	// reason; the pointer path did it last only because placement needed doing anyway.
	if s.sub == nil {
		s.ignore(ReasonNoSession)
		return
	}
	// OWNERSHIP BEFORE PLACEMENT, and for the same reason session membership comes first: a
	// press aimed at Marco's own surface is not a press this session could place OR admit,
	// and calling it unplaceable would report operating Marco as a perception failure. It
	// also has to come before the geometry, because Marco's overlay lies OVER the watched
	// window — a press on it IS inside the rectangle.
	if s.owned.marco(e.at) {
		s.ignore(ReasonMarcoOwned)
		return
	}
	if !s.window.fresh(e.at) {
		s.ignore(ReasonUnplaceablePointer)
		return
	}
	where, inside := s.window.place(e.x, e.y)
	if !inside {
		// A press somewhere else on the desktop. Not the watched application's business,
		// and emitting it would attribute the user's other windows to this session.
		s.ignore(ReasonUnplaceablePointer)
		return
	}
	s.stats.Classified++
	// Placed, and then one of THREE outcomes — kept apart because they send a reader to
	// three different places, and collapsing them cost a live diagnosis:
	//
	//	resolved   a control contained the press and its name was admitted
	//	unnamed    a control contained it and the name was WITHHELD — the label gate
	//	            doing its job, not a failure to find anything
	//	unresolved nothing in the index contained the press at all
	//
	// The middle one is the ordinary case for passive observation: a list item's text is not
	// on the plaintext allowlist, so only an explicit Learn licence admits it. Reporting that
	// as "landed on nothing" reads as a geometry or coordinate-space fault and sends the
	// reader hunting for one.
	target := s.controls.under(e.x, e.y, e.at)
	switch {
	case target == nil:
		s.stats.PointerUnresolved++
	case !target.Named():
		s.stats.PointerUnnamed++
	default:
		s.stats.PointerResolved++
	}
	// Never conditional. A pointer press means the same thing whatever the screen looks
	// like — the ambiguity that flag exists for is a KEY meaning two things, and a click
	// has only ever meant one.
	//
	// The RAW press is admitted first and unconditionally; the semantic target is resolved
	// beside it from evidence that was already on this side of the boundary. Resolution
	// failing — a stale index, a press between controls, no index at all — costs the
	// enrichment and never the event. Interpretation failure is not capture failure.
	s.sub.add(observe.NavPoint, e.at, false, where, target)
}

func (s *Source) ignore(r IgnoreReason) {
	if s.stats.Ignored == nil {
		s.stats.Ignored = map[IgnoreReason]int{}
	}
	s.stats.Ignored[r]++
}

// Stats reports what the producer did. Semantic counts only.
func (s *Source) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.stats
	if len(s.stats.Ignored) > 0 {
		out.Ignored = make(map[IgnoreReason]int, len(s.stats.Ignored))
		for k, v := range s.stats.Ignored {
			out.Ignored[k] = v
		}
	}
	out.Dropped = int(s.drops.Load())
	out.Unavailable = s.why
	// What the producer currently has to resolve against. A snapshot rather than a tally:
	// the question a reader asks is "was anything on offer", and the newest answer is the
	// one that matters.
	out.ControlsOffered = len(s.controls.items)
	return out
}

// windowBounds is where the watched window was, and when that was confirmed.
//
// Absolute desktop coordinates, held for as long as it takes to turn one pointer press into a
// normalised position and never longer. Nothing reads these but `place`, and what `place`
// returns is window-relative.
type windowBounds struct {
	x, y, w, h int
	at         time.Time
}

// fresh reports whether these bounds may be used to place an event at t.
//
// The same two bounds `screenContext.fresh` applies, for the same reasons: bounds must not be
// NEWER than the press — a window that moved after a click must not be used to decide where that
// click landed — and they go stale when observations stop.
//
// A window with no area is not a frame. It is what an unvalidated or minimised target reports,
// and normalising against it would divide by zero and call the result a corner.
func (b windowBounds) fresh(t time.Time) bool {
	if b.w <= 0 || b.h <= 0 || b.at.IsZero() || t.Before(b.at) {
		return false
	}
	return t.Sub(b.at) <= ScreenContextTTL
}

// place turns an absolute desktop position into a normalised window-relative one.
//
// Returns false for anything outside the window, INCLUSIVE of the edges being outside: a press
// at exactly the right or bottom edge belongs to whatever is beyond it.
//
// This is the only function in the product that reads an absolute pointer coordinate, and the
// value it returns is the only thing that survives it.
func (b windowBounds) place(x, y int32) (observe.PointerAt, bool) {
	dx, dy := int(x)-b.x, int(y)-b.y
	if dx < 0 || dy < 0 || dx >= b.w || dy >= b.h {
		return observe.PointerAt{}, false
	}
	return observe.PointerAt{
		X: float64(dx) / float64(b.w),
		Y: float64(dy) / float64(b.h),
	}, true
}

// Actionable is one control the watched window offered, as of one valid inference.
//
// # Why absolute pixels are allowed to exist here
//
// The same reason windowBounds holds them: a press arrives as an absolute desktop
// coordinate, and deciding what it landed on needs the controls in the same space. They live
// on a type that never leaves the package, are consulted on the worker, and what crosses out
// is a role and an ADMITTED label — never a rectangle.
//
// The Label arrives already gated: the composition root applies the canonical role
// allowlist, the demonstration licence and the shape filter before pushing, because those
// policies live beside perception and this package must not grow a second copy of them. What
// this side enforces is the boundary's own rule — the consumer re-checks shape and bounds in
// admissibleInputs, so a pusher that ignored its half cannot smuggle a paragraph through.
type Actionable struct {
	// X, Y, W, H are absolute desktop pixels, like windowBounds.
	X, Y, W, H int
	// Role is the control's kind; Label its admitted name, empty when withheld.
	Role, Label string
	// Focused says the keyboard's attention was on this control at the inference.
	Focused bool
}

// actionables is the worker-side index of what a press can be resolved against.
type actionables struct {
	items []Actionable
	at    time.Time
}

// fresh applies the SAME two-sided rule windowBounds does: evidence must not be newer than
// the press it explains, and it goes stale when observations stop.
func (a actionables) fresh(t time.Time) bool {
	if len(a.items) == 0 || a.at.IsZero() || t.Before(a.at) {
		return false
	}
	return t.Sub(a.at) <= ScreenContextTTL
}

// under resolves the smallest actionable containing an absolute desktop point, at event time.
func (a actionables) under(x, y int32, at time.Time) *observe.SemanticTarget {
	if !a.fresh(at) {
		return nil
	}
	best := -1
	for i, c := range a.items {
		if c.W <= 0 || c.H <= 0 {
			continue
		}
		if int(x) < c.X || int(y) < c.Y || int(x) >= c.X+c.W || int(y) >= c.Y+c.H {
			continue
		}
		if best == -1 || c.W*c.H < a.items[best].W*a.items[best].H {
			best = i
		}
	}
	if best == -1 {
		return nil
	}
	return &observe.SemanticTarget{Role: a.items[best].Role, Label: a.items[best].Label}
}

// focused is the control holding the keyboard's attention, when the last inference knew.
func (a actionables) focused(at time.Time) *observe.SemanticTarget {
	if !a.fresh(at) {
		return nil
	}
	for _, c := range a.items {
		if c.Focused {
			return &observe.SemanticTarget{Role: c.Role, Label: c.Label}
		}
	}
	return nil
}

// SetActionables records what the watched window offered, and when that was confirmed.
//
// Pushed by the composition root on every valid inference, beside SetWindowBounds, from the
// same validated cycle — so a press is resolved against what was actually on screen when it
// happened rather than whatever appeared afterwards.
func (s *Source) SetActionables(items []Actionable, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.controls = actionables{items: items, at: at}
}

// ownerContext is whether MARCO'S OWN surface held the foreground, and when that was confirmed.
//
// Pushed like every other scoped fact rather than asked at capture time, for the reason the whole
// hook path is arranged this way: answering "whose window is in front" means a syscall, and a
// low-level hook callback that makes one is a hook Windows will drop.
type ownerContext struct {
	marcoOwned bool
	at         time.Time
}

// marco reports whether Marco owned the foreground when an event at t happened.
//
// CONSERVATIVE in the direction that protects the demonstration, not in the direction that
// protects the count: with no recent answer this returns false, so input is admitted and the
// person's evidence survives. Silently discarding a real demonstration because ownership could
// not be confirmed would be the worse failure by far — the whole point of capture-first is that
// what the person DID is the ground truth.
//
// The staleness bound is the ordinary one, and the same reasoning as screenContext.fresh: an
// answer newer than the press says nothing about the press.
func (c ownerContext) marco(t time.Time) bool {
	if !c.marcoOwned || c.at.IsZero() || t.Before(c.at) {
		return false
	}
	return t.Sub(c.at) <= ScreenContextTTL
}

// SetSurfaceOwner records whether Marco's own control surface is in front.
//
// Pushed by the composition root on every valid inference, beside SetWindowBounds and
// SetActionables, from the same validated cycle — so a press is classified against who owned the
// screen when it happened rather than against whoever owns it by the time it is read.
func (s *Source) SetSurfaceOwner(marcoOwned bool, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.owned = ownerContext{marcoOwned: marcoOwned, at: at}
}

// screenContext is the producer's picture of what is on screen, and how old it is.
type screenContext struct {
	menuLike bool
	at       time.Time
}

// fresh reports whether the context may be relied on for an event at t.
//
// Two bounds, and the second is easy to miss. The assessment must not be NEWER than the press:
// a keypress made during gameplay, immediately before a menu appeared for some other reason,
// must not be admitted as navigation because the next observation happened to look menu-like.
// The question is always what Marco had last seen when the key went down.
func (c screenContext) fresh(t time.Time) bool {
	if !c.menuLike || c.at.IsZero() || t.Before(c.at) {
		return false
	}
	return t.Sub(c.at) <= ScreenContextTTL
}

// SetWindowBounds records where the watched window was, and when that was confirmed.
//
// Pushed by the composition root from the same validated reference every other piece of evidence
// is scoped to, so a pointer press is placed against the window perception actually looked at
// rather than against whatever is in front now.
//
// Called on every valid inference, like SetScreenContext, so the position a press is normalised
// against goes stale within one observation gap of the window moving.
func (s *Source) SetWindowBounds(x, y, w, h int, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.window = windowBounds{x: x, y: y, w: w, h: h, at: at}
}

// ScreenContextTTL is how long a screen assessment may be relied on with no newer one.
//
// # This is a backstop, not the freshness mechanism
//
// The real bound is FLIP-OFF: the composition root sets the context on every valid inference,
// including the ones that do not look like choices, so the moment the player leaves a menu the
// next observation turns admission off. The error window is therefore one inference gap, which
// was measured at a 3807ms median in [[Experiment-008-unknown-game-discovery]] — not this
// constant.
//
// What the TTL guards is inferences STOPPING: a run of skipped slots, a detector failure, a
// session whose sampler stalls. Without it a "menu-like" flag set once would persist for the
// rest of the session and admit gameplay movement as navigation for minutes — manufacturing
// exactly the false evidence this whole design refuses.
//
// Ten seconds is deliberately generous against a ~3.8s cadence: it is meant to catch a stall,
// not to second-guess the flip-off it sits behind. Tightening it below the observation gap would
// refuse nearly everything and gain nothing, because a keypress is almost always older than the
// last inference.
const ScreenContextTTL = 10 * time.Second

// SetScreenContext records what the last observation said the screen looks like.
//
// Called from the composition root once per valid inference, with menuLike computed from that
// inference's RAW detections — never from tracks, states or hypotheses. See observe.MenuLike for
// why the predicate is shaped the way it is, and why it deliberately does not measure spacing.
//
// Calling it with false is as important as calling it with true: that is the flip-off the
// freshness guarantee actually rests on.
func (s *Source) SetScreenContext(menuLike bool, at time.Time) {
	s.mu.Lock()
	s.screen = screenContext{menuLike: menuLike, at: at}
	s.mu.Unlock()
}

// Open begins a session-bounded subscription, stamped against t0.
//
// At most one at a time. A second Open replaces the first, and the replaced subscription stops
// accumulating — two overlapping sessions correlating the same keypress would be two different
// claims about one action.
func (s *Source) Open(t0 time.Time) *Subscription {
	sub := &Subscription{t0: t0}
	s.mu.Lock()
	if s.sub != nil {
		s.sub.close()
	}
	s.sub = sub
	// Held state is cleared so a key already down when the session began does not have its
	// release misread, and so nothing from before the session can survive into it.
	s.held = map[uint16]bool{}
	s.mu.Unlock()
	return sub
}

// CloseSession detaches the current subscription.
func (s *Source) CloseSession(sub *Subscription) {
	s.mu.Lock()
	if s.sub == sub {
		s.sub = nil
	}
	s.mu.Unlock()
	sub.close()
}

// Close stops the backend and the worker.
func (s *Source) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.sub = nil
	s.mu.Unlock()

	if s.backend != nil {
		s.backend.stop()
	}
	close(s.done)
	s.wg.Wait()
	return nil
}

// Subscription accumulates one session's navigation.
type Subscription struct {
	mu     sync.Mutex
	t0     time.Time
	buf    []observe.InputEvent
	closed bool
}

// MaxPending bounds one drain interval's retention.
const MaxPending = 256

func (u *Subscription) add(intent observe.NavIntent, at time.Time, conditional bool,
	where observe.PointerAt, target *observe.SemanticTarget) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed || len(u.buf) >= MaxPending {
		return
	}
	// Session-relative, from the same monotonic basis the session's own clock uses. A
	// wall-clock stamp would make correlation depend on two independent clocks agreeing.
	u.buf = append(u.buf, observe.InputEvent{
		Intent:      intent,
		AtMS:        at.Sub(u.t0).Milliseconds(),
		Conditional: conditional,
		Where:       where,
		Target:      target,
	})
}

// Peek is how many events are pending, without consuming them.
func (u *Subscription) Peek() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.buf)
}

// Drain returns everything since the last drain and clears the buffer.
func (u *Subscription) Drain() []observe.InputEvent {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.buf) == 0 {
		return nil
	}
	out := u.buf
	u.buf = nil
	sort.SliceStable(out, func(a, b int) bool { return out[a].AtMS < out[b].AtMS })
	return out
}

func (u *Subscription) close() {
	u.mu.Lock()
	u.closed, u.buf = true, nil
	u.mu.Unlock()
}

// Future privacy extension, documented and deliberately not built.
//
// Passive navigation never retains character identity, and nothing above can be configured to.
// Learning a command like "invite user xyz" needs the typed value, which is a different
// privilege with a different lifecycle: an explicit Learn/Text mode emitting
// text_entry_started / text_entry_committed / parameter_candidate, entered by the user on
// purpose and visible while it runs. Keeping that on its own channel is what lets passive
// observation stay structurally incapable of reconstructing typed content — a mode flag on
// THIS producer would mean the capability existed and was merely switched off.

// Actionables is what the last reading offered a press to resolve against.
//
// A READ, for diagnostics and for the gate that holds the offering deterministic. It returns a
// copy: the slice is shared with the classifier goroutine, and handing a caller the live one is
// how a diagnostic comes to race the thing it describes.
//
// Deleting the copy must fail TestReadingTheActionablesCannotRaceTheClassifier.
func (s *Source) Actionables() []Actionable {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Actionable, len(s.controls.items))
	copy(out, s.controls.items)
	return out
}
