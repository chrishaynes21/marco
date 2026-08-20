// Package observe watches one explicitly selected window while a person uses it.
//
// The Director's other perception paths answer a question somebody just asked: look now,
// tell me what is there. This one answers a question nobody can ask from a single frame —
// what STAYS, what CHANGES, and what merely flickers — by sampling the same window over a
// bounded period and comparing.
//
// # The player is in control
//
// Nothing here can act. That is not a convention or a review rule; it is a property of the
// package's dependencies, checked by boundary_test.go. There is no executor, no actuator,
// no keyboard or mouse host, no clipboard, no focus or activation call, and no path to
// lowering a Marco program. The session cannot click a button by accident because it has
// nothing to click with.
//
// The observation loop is also, deliberately, given its capture and perception through
// INTERFACES it does not construct. A package that could build its own capture could
// eventually build its own executor; one that receives everything can only do what it was
// handed.
//
// # Evidence, then hypotheses, never certainty
//
// A session produces three separate things, and the separation is the point:
//
//	observations   what was seen, how often, and with what confidence
//	transitions    what changed, from what to what, with the evidence
//	hypotheses     what it MIGHT mean — always with contradictions and a next step
//
// "Four labels appeared inside one stable panel" is an observation. "This may be a pause
// menu" is a hypothesis. "Pause menu detected" is a sentence this package will not produce,
// because a timeline can suggest meaning and cannot manufacture certainty.
package observe

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// SessionID identifies one observation session.
type SessionID string

// State is where a session is in its life.
//
// A closed vocabulary rather than free text: these are read by a status command and a
// protocol, and "finished" versus "completed" versus "done" would become three states
// nobody meant to have.
type State string

const (
	// Pending: created, not yet sampling.
	Pending State = "pending"
	// Observing: sampling.
	Observing State = "observing"
	// TargetUnavailable: the selected window went away and could not be reacquired
	// within the allowed window. Evidence collected so far is kept.
	TargetUnavailable State = "target_unavailable"
	// Completed: ran to its duration or frame bound.
	Completed State = "completed"
	// Cancelled: stopped on request.
	Cancelled State = "cancelled"
	// TimedOut: exceeded its own deadline without finishing cleanly.
	TimedOut State = "timed_out"
	// Failed: stopped by an error that was not the target going away.
	Failed State = "failed"
)

// Terminal reports whether a session in this state has stopped for good.
func (s State) Terminal() bool {
	switch s {
	case Completed, Cancelled, TimedOut, Failed, TargetUnavailable:
		return true
	}
	return false
}

// Succeeded reports whether the session ran its course.
//
// TargetUnavailable is deliberately NOT success. A session that watched for twenty seconds
// of an intended three minutes produced real evidence and an incomplete picture, and
// presenting it as a completed run would let a reader draw conclusions from a sample size
// they were never told about.
func (s State) Succeeded() bool { return s == Completed }

// Bounds are the limits every session runs under.
//
// There is no unbounded session. The loop captures a screen and runs a detector on it, so an
// unbounded one would hold a machine's GPU and disk for as long as nobody noticed.
type Bounds struct {
	// Duration is how long to watch.
	Duration time.Duration
	// Interval is the gap between samples when the scene is stable.
	Interval time.Duration
	// MaxFrames caps the samples taken however long the duration allows.
	MaxFrames int
	// MaxSnapshots caps retained samples. Older ones are dropped, and the drop is
	// counted rather than silent.
	MaxSnapshots int
	// MaxTransitions caps retained transitions.
	MaxTransitions int
	// MaxHypotheses caps generated insight candidates.
	MaxHypotheses int
	// ReacquireWindow is how long to keep trying to find the target again after it
	// disappears, when the selector permits reacquisition at all.
	ReacquireWindow time.Duration
}

// The limits a request is checked against. Named so a rejection can quote them.
const (
	DefaultDuration  = 3 * time.Minute
	MaxDuration      = 15 * time.Minute
	MinDuration      = 5 * time.Second
	DefaultInterval  = 500 * time.Millisecond
	MinInterval      = 200 * time.Millisecond
	MaxInterval      = 10 * time.Second
	DefaultMaxFrames = 600
	HardMaxFrames    = 5000

	DefaultMaxSnapshots   = 300
	DefaultMaxTransitions = 500
	DefaultMaxHypotheses  = 32

	DefaultReacquireWindow = 20 * time.Second
)

// DefaultBounds are the starting limits.
//
// Provisional, and chosen from measured cost rather than taste: a detection pass on a
// 1920x1080 frame ran 170–730ms live, and region label reading added roughly 230ms per
// control. A 500ms interval therefore samples about as fast as a frame can be understood,
// and anything faster would queue rather than observe.
func DefaultBounds() Bounds {
	return Bounds{
		Duration:        DefaultDuration,
		Interval:        DefaultInterval,
		MaxFrames:       DefaultMaxFrames,
		MaxSnapshots:    DefaultMaxSnapshots,
		MaxTransitions:  DefaultMaxTransitions,
		MaxHypotheses:   DefaultMaxHypotheses,
		ReacquireWindow: DefaultReacquireWindow,
	}
}

// Normalise fills in zero fields and reports anything out of range.
//
// Rejects rather than clamps. Silently shortening a three-hour request to fifteen minutes
// would leave somebody waiting for results from a session that ended long ago, and the
// difference between "I asked for three hours" and "it watched for fifteen minutes" is
// exactly the kind of thing a person needs told rather than discovered.
func (b Bounds) Normalise() (Bounds, error) {
	out := b
	if out.Duration == 0 {
		out.Duration = DefaultDuration
	}
	if out.Interval == 0 {
		out.Interval = DefaultInterval
	}
	if out.MaxFrames == 0 {
		out.MaxFrames = DefaultMaxFrames
	}
	if out.MaxSnapshots == 0 {
		out.MaxSnapshots = DefaultMaxSnapshots
	}
	if out.MaxTransitions == 0 {
		out.MaxTransitions = DefaultMaxTransitions
	}
	if out.MaxHypotheses == 0 {
		out.MaxHypotheses = DefaultMaxHypotheses
	}
	if out.ReacquireWindow == 0 {
		out.ReacquireWindow = DefaultReacquireWindow
	}

	switch {
	case out.Duration < MinDuration:
		return b, fmt.Errorf("a %s session is too short to see anything change; the minimum is %s",
			out.Duration, MinDuration)
	case out.Duration > MaxDuration:
		return b, fmt.Errorf("a %s session is longer than the %s maximum; "+
			"run several shorter ones rather than one that outlives your attention",
			out.Duration, MaxDuration)
	case out.Interval < MinInterval:
		return b, fmt.Errorf("a %s interval is faster than one frame can be understood "+
			"(a detection pass alone runs 170-730ms); the minimum is %s", out.Interval, MinInterval)
	case out.Interval > MaxInterval:
		return b, fmt.Errorf("a %s interval would miss almost everything; the maximum is %s",
			out.Interval, MaxInterval)
	case out.MaxFrames < 1:
		return b, fmt.Errorf("a session with no frames would observe nothing")
	case out.MaxFrames > HardMaxFrames:
		return b, fmt.Errorf("%d frames is beyond the %d maximum", out.MaxFrames, HardMaxFrames)
	}
	return out, nil
}

// Session is one bounded observation of one explicitly selected window.
type Session struct {
	ID       SessionID
	Selector windowref.Selector
	Bounds   Bounds

	StartedAt time.Time
	EndedAt   *time.Time

	State State
	// Reason explains a non-Completed state, in words a person can act on.
	Reason string

	// Application is what the target turned out to be, once resolved.
	Application string
	// Generations is every window generation the session observed, in order. More than
	// one means the window was replaced mid-session, and evidence must not be read as
	// though it came from a single continuous source.
	Generations []uint64
}

// Elapsed is how long the session has been running.
func (s *Session) Elapsed(now time.Time) time.Duration {
	if s.EndedAt != nil {
		return s.EndedAt.Sub(s.StartedAt)
	}
	if s.StartedAt.IsZero() {
		return 0
	}
	return now.Sub(s.StartedAt)
}

// Describe renders the session's target for a person, without leaking a handle.
func (s *Session) Describe() string {
	app := s.Application
	if app == "" {
		app = "unresolved"
	}
	return fmt.Sprintf("%s (%s)", app, s.Selector.Describe())
}
