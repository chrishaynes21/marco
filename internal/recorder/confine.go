package recorder

import "time"

// ── RECORD ONLY THE CHOSEN WINDOW ────────────────────────────────────────────
//
// What a recording admits, decided at the instant each event happens rather than afterwards.
//
// # Why admission and not a filter at the end
//
// Because the thing being protected is the buffer. A recording made while somebody answered a
// message and looked something up holds their message and what they looked up: keystrokes with
// the letters in them, and clicks with the coordinates of a window that is not the one they were
// demonstrating in. Deciding at Finish that those steps are not wanted leaves them recorded, in
// `Draft.Events`, published to whoever was listening, and written beside the routine — which is
// the shape of defect this repository has a rule about for passwords. So an event that did not
// happen in the chosen window never enters the buffer and never reaches the channel.
//
// # What the chosen window IS
//
// A window handle, with the process that owns it. NOT the process alone: two windows of one
// application satisfy a comparison of process ids and are two different windows, which is the
// same reason `RecordedEvent.HWnd` exists. Attribution that cannot be read is UNKNOWN and
// therefore not a member — an event whose window could not be determined is not admitted.
//
// # The boundary is the FOREGROUND, not the rectangle
//
// Membership asks whether the chosen window is the foreground window at that instant, not
// whether the pointer is inside its bounds. That is what makes a drag out of the window and
// back one gesture rather than three: a drag holds the foreground for its whole life, so every
// sample of it belongs to the window it started in, wherever the pointer went.

// Target is the one window a recording admits input from.
//
// The zero value is "nothing chosen yet" — see Watching.OneWindow, where the window is latched
// on the first foreground that is not the one the person started from.
type Target struct {
	// HWnd is the window itself, and the whole of membership.
	HWnd uintptr
	// PID is the process owning HWnd when it was chosen. It is identity, not membership: a
	// handle Windows has reused belongs to a different window, and a target whose process
	// no longer matches is a target that is GONE rather than one to keep recording.
	PID uint32
}

// Chosen reports whether a window has been settled on yet.
func (t Target) Chosen() bool { return t.HWnd != 0 }

// Watching is which windows a recording admits input from. It is STATED by the caller that
// starts the recording, before the hooks go on, exactly as recorder.Mode is — see Confined.
type Watching int

const (
	// WholeDesktop admits everything, and is the ZERO VALUE: a recorder nobody has told
	// anything records what every recording recorded before this.
	//
	// The default that CHANGED is the product's, and it belongs to the callers that state
	// it — see orchestrator.Watching, whose zero value is one window. Putting it here
	// instead would make a bare `recorder.New()` confine itself, which is a decision about
	// what a demonstration is being taken by the thing that only knows how to listen.
	WholeDesktop Watching = iota
	// OneWindow admits only the chosen window, and is what Studio, Learn and LearnAuto
	// now ask for: a demonstration is of one application, and the time somebody spends
	// away from it is not part of what they were showing.
	OneWindow
)

// Confined is a recorder that can be told which window to record.
//
// An optional interface, for the reason `Stated` is one: the rule has to be exercisable against
// a recorder that is not the real one, and a backend with no notion of windows (the stub) must
// not have to grow one. A recorder that cannot be told records what it always did.
type Confined interface {
	// Only states what the recording `Start` begins next admits.
	//
	// Before Start, because the hooks read it: a scope stated afterwards would be stated
	// after off-target input could already have been recorded.
	//
	// `t` is the window somebody CHOSE, handle and owner both. Not inferred here and not
	// inferred by the hooks: the first version latched the first foreground window that
	// was not Marco's, which is whatever came forward first rather than anything a person
	// picked. A caller asking for `OneWindow` with no target is refused by
	// `orchestrator.Begin` before the hooks go on, rather than recording the desktop.
	Only(w Watching, t Target)
}

// Pausable is a recorder that can say whether it is currently admitting nothing.
//
// For a surface that is LOOKED AT rather than read. `orchestrator.waitForStop` prints the
// transitions as they happen, which suits a terminal and the HUD that streams it; a window with
// a recording card redraws on a poll and needs to ask. Optional for the reason `Confined` is:
// a backend with no notion of windows has nothing to answer.
type Pausable interface {
	// Paused is whether the person is outside the chosen window right now, and how long
	// they have been.
	Paused() (bool, time.Duration)
}

// ── WHAT HAPPENS TO ONE OBSERVED EVENT ───────────────────────────────────────

// Admission is what the recorder does with one event the hooks saw.
type Admission int

const (
	// Drop is neither recorded nor published: it did not happen in the chosen window.
	Drop Admission = iota
	// Admit is recorded and published.
	Admit
	// StopOnly is PUBLISHED BUT NOT RECORDED, and it exists for exactly one thing: the
	// stop gesture must work from anywhere.
	//
	// The stop is detected by a consumer of the live channel (`orchestrator.waitForStop`,
	// `cmd/marco` panic-stop), from `StopKey.Triggered`, which reads nothing but the key
	// name of an EvKey and ignores every key outside the gesture. So the whole of what a
	// stop needs is the stop keys themselves, and that is the whole of what an off-target
	// event may put on the channel. Nothing else off-target is published, because a
	// consumer of that channel is as entitled to not hold somebody's message as the
	// buffer is.
	StopOnly
)

// ── THE CROSSING RULES ───────────────────────────────────────────────────────

// Boundary tracks what is held across the edge of the chosen window, and decides each event.
//
// # A gesture that crosses the boundary is UNREPRESENTABLE, not trimmed
//
// A gesture starting inside the window cannot authorize recording its outside portion, and the
// inside half on its own is not what the person did. Dropping a modifier and keeping the clicks
// it modified writes down a different demonstration — `ctrl+click` becomes `click` — and saving
// that silently is worse than refusing: the person is the only one who can perform it again, so
// what they get is the steps, the reason, and a refusal at Save.
//
// # Every key, not only the modifiers
//
// `simplify.modifierTrouble` refuses a modifier released with no press because a modifier
// changes what the keys under it mean. A boundary is not that question. Q held across the edge
// is a held key whose press is not in the recording, and the recording cannot say what the
// release meant; a game where Q is "use" is the ordinary case, not the exotic one. So the state
// here is every key and every button.
//
// # And in the other direction
//
// The case the first draft of this missed: entering the window with Ctrl ALREADY DOWN. The
// press is outside and excluded, so a ctrl+click in the window would be recorded as an ordinary
// click — a correct admission decision producing a wrong demonstration. So an admitted action
// taken while anything is held from outside is a crossing, which is the same rule read the
// other way round, and it is also what bounds the activation exception below.
type Boundary struct {
	// inside is whether the chosen window was the foreground at the last event seen.
	inside bool
	// heldInside is what was pressed while admitted and is still down. Transient, and
	// never written anywhere: held state is metadata about a gesture, not content.
	heldInside map[string]bool
	// heldOutside is what was pressed while NOT admitted and is still down. It holds the
	// IDENTITY of a foreign key, which is the minimum a crossing can be detected with,
	// and it is in memory for as long as the key is down and nowhere else.
	heldOutside map[string]bool
	// crossing is why this recording cannot be written down, or "" while it still can.
	// FIRST REASON WINS: the first thing that crossed is the one the person can act on.
	crossing string
	// away is the running total of time spent outside the window, and absences is how
	// many times. Both are for the sentence review shows; the clock transformation reads
	// the markers themselves.
	away     time.Duration
	absences int
	leftAt   time.Time
}

// NewBoundary starts a boundary outside the window, which is where every recording starts: the
// person has chosen a window and has not gone to it yet.
func NewBoundary() *Boundary {
	return &Boundary{heldInside: map[string]bool{}, heldOutside: map[string]bool{}}
}

// HeldBefore seeds what was already down when the recording began.
//
// # Why it is held FROM OUTSIDE and not simply known about
//
// Because it is exactly that: a press this recording did not witness, whose release has nothing
// to pair with. Treating it as held-from-outside makes one rule cover two facts that are the
// same fact at different times — Ctrl pressed in another window and still down, and Ctrl
// pressed before Record and still down. An admitted action while either is held writes down a
// gesture the person did not perform, and the recording says so instead of saving it.
//
// The names are transient metadata and go nowhere: they live for as long as the key is down, and
// the only one that is ever said out loud is the one named in a refusal.
func (b *Boundary) HeldBefore(ids []string) {
	for _, id := range ids {
		b.heldOutside[id] = true
	}
}

// Crossing is why the recording cannot be saved, or "" when it can.
func (b *Boundary) Crossing() string { return b.crossing }

// Away is how long was spent outside the chosen window, and how many separate absences.
func (b *Boundary) Away() (time.Duration, int) { return b.away, b.absences }

// Outside is whether the foreground is elsewhere right now, and since when.
//
// The INSTANT it left, not the running total: what a surface shows while a recording is paused
// is how long this absence has lasted, and the total is for review afterwards.
func (b *Boundary) Outside() (bool, time.Time) { return !b.inside, b.leftAt }

// Arrived tells the boundary where the foreground is now, and reports the marker to record when
// that is a change.
//
// A LEAVING WITH SOMETHING STILL HELD IS A CROSSING, decided here because this is the instant
// it stops being recoverable: whatever happens to that key next happens somewhere else.
func (b *Boundary) Arrived(inside bool, at time.Time) (EventKind, bool) {
	if inside == b.inside {
		return 0, false
	}
	b.inside = inside
	if !inside {
		for id := range b.heldInside {
			b.cross(id + " was still held when the window was left, so the " +
				"recording has half of a gesture in it")
			break
		}
		b.leftAt = at
		return EvLeft, true
	}
	if !b.leftAt.IsZero() {
		b.away += at.Sub(b.leftAt)
		b.absences++
		b.leftAt = time.Time{}
	}
	return EvEntered, true
}

// Left reports whether the foreground is currently somewhere other than the chosen window.
func (b *Boundary) Left() bool { return !b.inside }

// Pressed decides one press or release of a key or a button, and updates the held state. `id`
// identifies the thing held — a key name or a button name — and only has to be stable between
// the press and the release.
func (b *Boundary) Pressed(id string, down, admitted, isStop bool) Admission {
	if down {
		if !admitted {
			b.heldOutside[id] = true
			return b.unadmitted(isStop)
		}
		// AN ACTION THAT DEPENDS ON AN EXCLUDED GESTURE. Asked before the press is
		// remembered, so the press that trips it is itself reported.
		b.tainted(id)
		b.heldInside[id] = true
		return Admit
	}
	// ── THE RELEASE OF A PRESS WE DID NOT RECORD ──
	//
	// Dropped, and NOT a crossing: this is the activation click. Somebody clicks on the
	// window to bring it to the front, and the press of that click lands while the window
	// is still behind — so the press is outside and the release is inside, and what is
	// left on its own is a release of a button nothing pressed. It is how you get INTO
	// the window, not part of the demonstration, so it goes.
	//
	// The exception is bounded by `tainted` above rather than by anything here: the press
	// is forgotten only when its release arrives, so an admitted action performed while it
	// is STILL HELD has already made the recording unrepresentable. Click-to-return is
	// dropped; entering with Ctrl down and clicking is refused.
	if b.heldOutside[id] {
		delete(b.heldOutside, id)
		return Drop
	}
	if b.heldInside[id] {
		delete(b.heldInside, id)
		if !admitted {
			b.cross(id + " was pressed in the window and released outside it, so " +
				"the recording has half of a gesture in it")
			return Drop
		}
		b.tainted(id)
		return Admit
	}
	// A RELEASE OF SOMETHING THIS RECORDING NEVER SAW GO DOWN.
	//
	// It used to be ADMITTED here, on the reasoning that a recording may begin with
	// somebody resting a finger on a key and `simplify` refuses the cases it can name.
	// That was the hole: `simplify.modifierTrouble` names modifiers, so a release of
	// anything else was recorded as a keystroke nobody made, and the PRESS it belonged to
	// had already modified an admitted action without appearing anywhere.
	//
	// `HeldBefore` closes it at the source — what was down at Start is seeded, so its
	// release takes the branch above and is dropped. What reaches here now is a release
	// of something that went down between the scan and the first hook callback, which is
	// the same thing one instant later: not ours, and not written down.
	if !admitted {
		return b.unadmitted(isStop)
	}
	b.tainted(id)
	return Drop
}

// Moved decides one mouse-move sample.
//
// A MOVE NEVER TRIPS THE CROSSING RULE, deliberately. A move is a sample of where the pointer
// was, not something the person did — the throttle in the hook already means most of them are
// not recorded — and a bare move over the window while a key is held somewhere else expresses
// no intent that could be written down wrongly. Whatever the person then DOES trips it, because
// a press and a release are intents and a move is not.
func (b *Boundary) Moved(admitted bool) Admission {
	if admitted {
		return Admit
	}
	return Drop
}

// Admits decides an event that is neither a press nor a move — an app switch, say — where
// there is no held state to maintain.
func (b *Boundary) Admits(admitted bool) Admission {
	if admitted {
		return Admit
	}
	return Drop
}

func (b *Boundary) unadmitted(isStop bool) Admission {
	if isStop {
		return StopOnly
	}
	return Drop
}

// tainted records a crossing when an admitted action is performed while something is held from
// outside the window. See the type comment: the press that is missing changes what this action
// means, and the recording cannot say so.
func (b *Boundary) tainted(id string) {
	for held := range b.heldOutside {
		b.cross(id + " was used in the window while " + held + " was held down from " +
			"outside it, so what the recording has is not what was done")
		return
	}
}

func (b *Boundary) cross(why string) {
	if b.crossing == "" {
		b.crossing = why
	}
}
