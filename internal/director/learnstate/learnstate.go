// Package learnstate is a recording's own account of itself, as events rather than as prose.
//
// # Why this exists
//
// `demonstrate-visual` says everything it has to say at the two ends: a block of instructions
// before the delay, and a report after the person presses ENTER. In between — which is the whole
// of the demonstration — it is silent. A terminal is fine with that, because the person watching
// it is the person doing the demonstrating and they can see their own screen.
//
// A window cannot. Studio's first version of the recording card could say two things: that a
// recording was running, and how many of its seconds were left. So the only guidance it could
// give about the part that decides whether anything useful was learned was a sentence of advice
// — rest before you click, wait after it — and the owner was left estimating how long seven OCR
// readings take. The project's own instruction about that:
//
//	The recorder knows whether it obtained evidence; the person should not have to estimate
//	seven OCR readings.
//
// So the recorder emits what it knows, every time it changes: whether the screen it started on
// has been read yet, whether a press has been seen, whether Marco could place that press on
// anything, whether it has a verified local outcome, and when it has not, WHY — in
// `observe.LocalOutcome.Why`'s own words, which is the same evidence `workflow.Learn` will use
// to decide whether the interaction can be checked at all.
//
// # Snapshots, not deltas
//
// Each event is the whole current state, exactly as `runstate` is and for the same reason: a
// front end renders the latest one and is correct by construction. `Seq` lets a reader tell it
// has fallen behind, and `Final` lets it tell a recording that ENDED from a child that merely
// stopped.
//
// # Why this is not runstate
//
// Because a recording is not a run and shares none of its stages. Nothing is authorised, nothing
// is pressed, no question is put to anybody, and there is no outcome in `runref`'s vocabulary to
// report. The two streams have one word in common — `Ended` — and giving `runstate.Phase` a
// second meaning would leave two readers of one vocabulary disagreeing about what half of it
// means. That is the mistake `runstate` itself names about `playbill.Phase`.
package learnstate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Version is the stream format. A reader skips anything it cannot vouch for.
const Version = 1

// Phase is where a recording is, in the recorder's own terms.
//
// They are the stages a person doing a demonstration is actually in. `Preparing` is the delay
// before watching starts; the rest are stages of the demonstration itself, and a recording moves
// between them as freely as a person clicking two controls does.
type Phase string

const (
	// Preparing is the delay: the window has been brought forward and nothing is being
	// watched yet.
	Preparing Phase = "preparing"
	// Reading is watching, with no press seen yet and the screen in front not read yet.
	//
	// The half of a demonstration people skip. Marco identifies the screen an Act starts
	// on by the words read off it, so a demonstration whose first press happened before
	// anything was read produces an Act that can only ever wait.
	Reading Phase = "reading"
	// Ready is watching, with the screen in front read and no press seen yet.
	Ready Phase = "ready"
	// Checking is watching after a press Marco placed, with no verified outcome yet.
	Checking Phase = "checking"
	// Blind is watching after a press that landed on nothing the proposer found.
	//
	// Its own phase because waiting cannot fix it and `Checking` would say waiting is all
	// there is to do. A press with no region under it can never become a step — it is kept
	// as evidence of what the person did and nothing can be replayed from it — so the
	// thing to do is click the control itself rather than beside it.
	Blind Phase = "blind"
	// Captured is watching after a press that HAS a verified local outcome.
	Captured Phase = "captured"
	// Ended is the recording over.
	Ended Phase = "ended"
)

// Event is one snapshot of a recording.
type Event struct {
	Version int       `json:"version"`
	Seq     int       `json:"seq"`
	At      time.Time `json:"at"`
	// Name is what the person is teaching, and Application the window being watched.
	Name        string `json:"name,omitempty"`
	Application string `json:"application,omitempty"`
	Phase       Phase  `json:"phase"`
	// Presses is how many presses the correlator has been given.
	//
	// Presses, not steps: what became of each one is the end report's business, and a
	// window that reported steps would say nothing had happened while somebody was
	// clicking.
	Presses int `json:"presses,omitempty"`
	// Watched is how many of those presses Marco could place on something, so an outcome
	// is being measured for them at all.
	//
	// `Presses - Watched` is presses that landed on nothing the proposer found. Carried
	// separately rather than folded into the phase, because a session can have one of
	// each and the phase is about the press in flight.
	Watched int `json:"watched,omitempty"`
	// Captured is how many presses have a verified local outcome.
	Captured int `json:"captured,omitempty"`
	// Missing is why the last press has no verified outcome yet, in the words of the thing
	// that decided — `observe.LocalOutcome.Why`. Empty when nothing is missing, and empty
	// before the first press, where there is nothing to miss.
	Missing string `json:"missing,omitempty"`
	// Left is how many seconds of the ceiling remain.
	Left int `json:"left,omitempty"`
	// Final marks the last event of a recording that ended of its own accord, and Reason
	// says why it ended.
	Final  bool   `json:"final,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Writer is a recording's stream. A nil Writer is valid and does nothing.
type Writer struct {
	mu  sync.Mutex
	w   *os.File
	seq int

	name, application string
	// last is the JSON of the last event said, so an unchanged state is not written
	// again. A recorder is consulted ten times a second and the answer is the same ten
	// times; a file a person reads afterwards should not say so a hundred times.
	last string
}

// NewWriter opens a stream at a path. An empty path is a nil Writer, so no caller has to check
// whether progress was asked for.
func NewWriter(path, name, application string) (*Writer, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("learnstate: opening %s: %w", path, err)
	}
	return &Writer{w: f, name: name, application: application}, nil
}

// Say emits one snapshot, unless it says exactly what the last one did.
//
// Emitting on change is a correctness choice and not an optimisation: this is called from the
// recorder's own loop, which runs beside a low-level input hook, and a write per look would put
// file I/O on the cadence that hook shares.
func (w *Writer) Say(e Event) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	e.Version, e.Name, e.Application = Version, w.name, w.application
	// THE SECONDS LEFT ARE NOT A CHANGE. They tick every second by construction, so
	// comparing them would emit every event there is.
	left := e.Left
	e.Left, e.Seq, e.At = 0, 0, time.Time{}
	same, err := json.Marshal(e)
	if err != nil {
		return
	}
	if string(same) == w.last {
		return
	}
	w.last = string(same)
	w.seq++
	e.Left, e.Seq, e.At = left, w.seq, time.Now()
	raw, err := json.Marshal(e)
	if err != nil {
		return
	}
	// UNBUFFERED, so a window polling the file sees the line when it looks.
	_, _ = w.w.Write(append(raw, '\n'))
}

// Close ends the stream.
func (w *Writer) Close() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.w.Close()
}

// Read is every event in a stream, in order, skipping any this build cannot vouch for.
//
// A truncated last line is ignored rather than refused: a reader tailing a live recording will
// sometimes see half a line, and a window that errored on it would blink.
func Read(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var e Event
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if e.Version != Version {
			continue
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

// Latest is the last event of a stream and whether the recording ENDED.
func Latest(path string) (Event, bool, error) {
	all, err := Read(path)
	if err != nil {
		return Event{}, false, err
	}
	if len(all) == 0 {
		return Event{}, false, nil
	}
	last := all[len(all)-1]
	return last, last.Final, nil
}

// PhaseOf is where a recording is, from what the recorder knows.
//
// The one place that decides, so the recorder, the window and a test cannot drift. `startRead`
// is whether anything has been read on the screen the session began on; `presses` how many
// presses the correlator has been given; `watched` how many of those it could place on a region,
// so an outcome is being measured for them; and `captured` how many have a verified one.
//
// # Why a press with no outcome yet is not a failure
//
// Because it is the ordinary case for most of a second. The outcome is words read somewhere
// other than the control, which APPEARED when it was pressed and STAYED — four terms, two of
// which need readings on both sides of the press, from a source that answers about twice a
// second. So `Checking` is where a demonstration spends the moment after every click, and the
// only thing that makes it alarming is a person finishing while it is still true.
//
// # Why an outcome in flight outranks a press that landed on nothing
//
// Because the phase is about the press being measured NOW, and a session can have one of each: a
// first click beside a control and a second one on it. Reporting the blind press as the phase
// would leave the window saying "click the control itself" while the control it wants had
// already been clicked and its outcome was arriving. The blind presses are still reported —
// `Presses - Watched` — and the end report is where they are accounted for.
func PhaseOf(startRead bool, presses, watched, captured int) Phase {
	switch {
	case presses == 0 && !startRead:
		return Reading
	case presses == 0:
		return Ready
	case captured < watched:
		return Checking
	case watched == 0:
		// EVERY PRESS LANDED ON NOTHING. Waiting will not change that, so this
		// does not say to wait.
		return Blind
	}
	return Captured
}

// Doing is the one sentence a window shows for a phase, in the words of what the person is doing.
//
// Not the phase word: `checking` is the recorder's vocabulary and right for a recorder, and a
// person who has just clicked something is owed a sentence about the thing they did. Empty for a
// recording that has ended, where what it LEARNED is the headline.
func Doing(p Phase, application string) string {
	switch p {
	case Preparing:
		if application == "" {
			return "Bringing the window forward…"
		}
		return "Bringing " + application + " forward…"
	case Reading:
		// AND WHY TO WAIT, because this is the stage whose absence produces an Act
		// that can only ever wait. Said as what Marco is doing rather than as an
		// instruction to hold still, which is the same request without the blame.
		return "Reading the starting screen… rest where you are for a moment."
	case Ready:
		return "Click the control once, with the mouse."
	case Checking:
		return "Checking what changed… wait for the result to settle."
	case Blind:
		return "That press landed on nothing Marco could see. Click the control " +
			"itself — on its label — rather than beside it."
	case Captured:
		return "Result captured. Click the next control, or finish."
	}
	return ""
}
