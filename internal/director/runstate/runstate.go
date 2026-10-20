// Package runstate is a supervised run's own account of itself, as events rather than as prose.
//
// # Why this exists
//
// A front end needs to know what is happening: which stage the run is in, what it is asking the
// person right now, how long preparation has left, and how it ended. `perform-workflow` prints
// all of that for somebody at a terminal, and the only machine-readable thing it emitted was the
// run citation — so Studio READ THE PROSE, matching on sentences like "May Marco" and "Is that
// the screen you are on?".
//
// That is two defects wearing one coat. A window whose behaviour depends on the exact wording of
// a consent prompt breaks when somebody improves the wording, silently and in the direction of
// showing a person the wrong thing about a press they are about to authorise. And prose is not
// authoritative: the sentence is printed NEAR the decision, not BY it, so a reader of prose is
// always inferring a state that something else already knew for certain.
//
// So the decisions emit. Every event here is written at the point the runner decides something —
// the start wait beginning and ENDING, a question being put, a question being answered or
// withdrawn, an action going out, the evidence for it being collected, a verdict arriving, the
// run ending — and the words a person reads are carried INSIDE the event, for the transcript,
// never parsed back out of it.
//
// EVERY TRANSITION, which is the part that is easy to get wrong while the transport is right. A
// snapshot stream makes a prompt disappear when the next snapshot arrives; it does nothing at all
// about a transition nobody emitted. The run's own preparation used to end without a word, so the
// latest snapshot stayed a deadline and an open question through acquisition and provider
// start-up; and the interval between a press and its verdict was emitted at neither end, so a
// front end showed `acting` for the whole of the longest part of an interaction. Both looked
// exactly like a working front end, which is why they are named here.
//
// # Snapshots, not deltas
//
// Each event is the whole current state. A front end renders the latest one and is correct by
// construction: a question that has been answered is absent from the next snapshot, so a stale
// prompt cannot survive on screen. Deltas would require the reader to track resolution, and a
// reader that missed one would leave a person looking at a question nobody is waiting on — which
// is the likeliest thing to be answered twice.
//
// `Seq` is there so a reader can tell it has fallen behind, and `Final` so it can tell a run that
// ENDED from a child that merely stopped: a run whose last event is not final is incomplete,
// whatever the process did, and saying so is the difference between "it failed" and "nobody
// knows".
//
// # Why these phases are not `pkg/playbill`'s
//
// `playbill.Phase` is the account the SERVICE gives of a command it is carrying out, and this is
// the account a SUPERVISED RUN gives of an Act. The stages are genuinely different: playbill has
// no "which screen are you on", and a supervised run's permission is asked once per press rather
// than once per command. Reusing that type would have meant adding words to it the service never
// emits, and then two readers of one vocabulary would disagree about what half of it means.
//
// What IS reused is the outcome: `runref.Outcome`, the seven words both binaries already share.
package runstate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
)

// Version is the event format. A reader that cannot vouch for a version ignores the event rather
// than half-understanding it: a front end that rendered a field it did not know the meaning of
// would be showing somebody a guess about a press.
const Version = 1

// Phase is the stage a supervised run is in.
type Phase string

const (
	// Preparing is before anything has been asked: the delay, and waiting for the
	// person to say they are ready. The only phase with a deadline of its own.
	Preparing Phase = "preparing"
	// Starting is preparation OVER and nothing being asked yet: the window is being
	// acquired, the perception providers are coming up, the first look has not landed.
	//
	// A phase of its own because the alternative was leaving `Preparing` standing. The
	// run's preparation ends the instant somebody presses start — and acquisition and
	// provider start-up take seconds after it, during which a front end still showing
	// `preparing` is showing a countdown to a deadline that has already been spent, next
	// to a question nobody is waiting on an answer to.
	Starting Phase = "starting"
	// Naming is asking which of the saved screens the person is on.
	Naming Phase = "naming"
	// Agreeing is asking whether a step belongs in this run at all, which is a
	// different question from whether it may happen now.
	Agreeing Phase = "agreeing"
	// Asking is asking permission for one press, on one control, now.
	Asking Phase = "asking"
	// Acting is a press that has gone out.
	Acting Phase = "acting"
	// Verifying is the post-action evidence: being collected, and then answered.
	//
	// `Interaction.Verdict` is which of the two. Empty means the press has gone out and
	// the run is still waiting for evidence — the interval a front end used to show as
	// `acting`, because nothing was emitted between the press and the answer. Set means
	// the coordinator has answered, and the next event says what follows from it.
	Verifying Phase = "verifying"
	// Ended is over. Carried only on a final event.
	Ended Phase = "ended"
)

// Key is one answer a question accepts, and what answering it means.
//
// Carried rather than assumed, so a front end shows the keys the runner is actually listening for.
// A window that hardcoded F7/F8/F9 would keep showing them after the runner stopped accepting one.
type Key struct {
	Key   string `json:"key"`
	Means string `json:"means"`
}

// Question is what the run is waiting for a person to answer.
type Question struct {
	// ID identifies this asking of it. A second question about the same step is a
	// different ID, so an answer cannot be attributed to the wrong one.
	ID string `json:"id"`
	// Asks is the words the person is reading, for the transcript and the window.
	//
	// NEVER PARSED. It is here so a front end can show the question verbatim — putting
	// its own words on a consent prompt would be writing somebody else's question — and
	// everything a reader needs to DECIDE with is a field beside it.
	Asks string `json:"asks"`
	// Keys is what it accepts, and Until when it stops accepting them.
	Keys  []Key     `json:"keys"`
	Until time.Time `json:"until,omitzero"`
}

// Interaction is which of an Act's interactions is in hand.
//
// # Two numberings, and only one of them is the Act's
//
// An Act is composed BY PICKING: it names a subset of a demonstration's interactions, in an
// order of its own. So the position of an interaction in the ACT and the position of the step it
// was demonstrated at are different numbers, and the first draft of this published the second
// one under the name of the first — an Act naming interaction 2 of a three-step routine said
// `index 2` about its FIRST interaction, left `of` unset, and put the control identifier where
// the name the person gave it belongs. A window showing "2" about the first of one is telling
// somebody something false about a press they are about to authorise.
//
// So they are separate fields, filled by the thing that SELECTED the interaction — which is the
// only party that knows which Scene a sentence meant. See cmd/director/interactionidentity.go.
type Interaction struct {
	// Index is its position in the performance, counting from 1, and Of how many that
	// performance has. Zero means nobody could say.
	//
	// "The performance" is the Act when an Act is being performed and the artifact walk
	// otherwise, because those are the two things a run can be: a run of three sentences
	// is on 2 of 3 whatever the demonstration underneath it is numbered.
	Index int `json:"index"`
	Of    int `json:"of"`
	// Name is what the PERSON called it — the Scene's name in the play. Empty for a
	// workflow walk, which has no named scenes.
	Name string `json:"name"`
	// Step is the position in the underlying demonstration, and Control the identifier
	// the demonstration recorded for the thing being pressed.
	//
	// Both are evidence about where this came from and neither is a name: a window that
	// showed `Control` as the interaction's name was showing a person the machine's word
	// for a thing they had given a word of their own.
	Step    int    `json:"step"`
	Control string `json:"control,omitempty"`
	// Verdict is what the coordinator's post-action evidence said, once it has said
	// anything: `verified`, `contradicted`, `unconfirmed`, `unasked`.
	//
	// A field and not part of the name, which is what it was. Empty means the question is
	// still open — which, during `Verifying`, is the difference between evidence being
	// collected and an answer having arrived.
	Verdict string `json:"verdict,omitempty"`
}

// Event is the whole state of a run at one moment.
type Event struct {
	Version int       `json:"version"`
	Seq     int       `json:"seq"`
	At      time.Time `json:"at"`
	// Run is the canonical reference, the same one both binaries parse.
	Run string `json:"run"`
	// Act and Routine are what is being performed and what it was composed from. Act is
	// empty for an ordinary workflow run.
	Act     string `json:"act,omitempty"`
	Routine string `json:"routine,omitempty"`
	// Live is whether this run REALLY PRESSES, as the run itself knows it — not as a
	// front end's checkbox last reported it.
	Live  bool  `json:"live"`
	Phase Phase `json:"phase"`
	// Interaction and Question are what is in hand, when anything is. A nil Question
	// means nothing is being asked, which is how a stale prompt disappears.
	Interaction *Interaction `json:"interaction,omitempty"`
	Question    *Question    `json:"question,omitempty"`
	// Preparation is when preparation ends, zero at every other time.
	//
	// ONLY THE RUNNER DECIDES WHEN IT ENDS. A front end may count down to this instant
	// and show what it computes; the run is not over because a window's arithmetic said
	// so, and a window that acted on its own clock would be deciding something it is not
	// in a position to know.
	Preparation time.Time `json:"preparation,omitzero"`
	// Outcome and Reason are how it ended, on the final event only.
	Outcome runref.Outcome `json:"outcome,omitempty"`
	Reason  string         `json:"reason,omitempty"`
	// Final marks the last event of a run. A stream without one describes a run nobody
	// can speak for; see the package comment.
	Final bool `json:"final,omitempty"`
}

// Says is the question's words, or empty. For a transcript, never for a decision.
func (e Event) Says() string {
	if e.Question == nil {
		return ""
	}
	return e.Question.Asks
}

// Writer emits a run's events, one JSON object per line.
//
// Newline-delimited because a reader may be tailing the file while it is being written, and a
// line is the smallest thing that is either there or not. A JSON array would be unreadable until
// the run ended, which is exactly when a front end no longer needs it.
type Writer struct {
	mu  sync.Mutex
	w   io.WriteCloser
	seq int
	// run, act, routine and live are on every event, so a reader of any single line
	// knows which run it is about.
	run, act, routine string
	live              bool
}

// NewWriter opens a stream at a path. A nil Writer is valid and does nothing, so no caller has to
// check whether events were asked for.
func NewWriter(path, run, act, routine string, live bool) (*Writer, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("runstate: opening %s: %w", path, err)
	}
	return &Writer{w: f, run: run, act: act, routine: routine, live: live}, nil
}

// Say emits one snapshot. Safe on a nil Writer.
//
// The caller fills in what has changed; the run's identity is added here so it cannot be
// forgotten at a call site.
func (w *Writer) Say(e Event) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seq++
	e.Version, e.Seq, e.At = Version, w.seq, time.Now()
	e.Run, e.Act, e.Routine, e.Live = w.run, w.act, w.routine, w.live
	raw, err := json.Marshal(e)
	if err != nil {
		return
	}
	// FLUSHED BY BEING UNBUFFERED. A front end polling a file needs the line to be
	// there when it looks, and a buffered writer would hold a question back until
	// something else was said.
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
// A truncated last line is ignored rather than refused: a reader tailing a live run will
// sometimes see half a line, and a front end that errored on it would blink.
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

// Latest is the last event of a stream and whether the run ENDED.
//
// Two answers, because they are different: a run whose last event is not final is incomplete, and
// a reader that returned only the event would have no way to tell that from a run that ended.
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
