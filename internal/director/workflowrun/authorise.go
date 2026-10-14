package workflowrun

// The right to perform ONE action.
//
// [[ADR-183-remembering-recognising-and-being-allowed-to-act]] gives the three objects of this
// subsystem three different lifetimes, and this is the shortest of them:
//
//	Workflow        lives durably     "this is what you showed me"
//	Binding         ONE SESSION       "this is where I think I am"
//	Authorisation   ONE ACTION        "this press, now, here"
//
// A session flag would be the wrong shape for the last line, and the wrongness is not theoretical:
// a flag granted while looking at the Play menu is still set four screens later, against a
// re-acquired control nobody has seen, in a window that something else has covered. So this names
// the step, names who granted it, carries the observation it was granted in, and expires.
//
// None of it can be constructed from inside the run. `Granter` is an interface the composition
// root supplies, for the same reason `StepReadinessAt` never infers `Authorised`: a planner that
// could grant its own permission is the thing every boundary in this subsystem exists to prevent.

import (
	"context"
	"fmt"
	"time"
)

// Authorisation is somebody's permission to perform one step, once.
type Authorisation struct {
	// Step is the one step this permits. Not "the workflow" and not "the next thing".
	Step int
	// By is who granted it. An unattributed grant is refused, for the reason every other
	// provenance in this subsystem is: it is indistinguishable from a default.
	By string
	// Epoch and Generation are the observation the grant was given in.
	//
	// Here because an authorisation is given while LOOKING at something. The screen the
	// person agreed about is the screen the point was found on, and both stop being true at
	// the same moment — so a grant that outlived its observation would be permission to
	// click a remembered coordinate on a screen that has since changed.
	Epoch      uint64
	Generation uint64
	// Expires is when it lapses regardless of anything else. A grant with no expiry is a
	// session flag with extra words.
	Expires time.Time
}

// Spendable reports whether this authorisation permits acting on `step` in `at`, and why not.
//
// Four separate questions, each with its own answer, because a caller that gets one "no" cannot
// tell a person whether to look again, agree again, or stop.
func (a Authorisation) Spendable(step int, at Sighting, now time.Time) (bool, string) {
	switch {
	case a.By == "":
		return false, "the authorisation names nobody who granted it"
	case a.Step != step:
		return false, fmt.Sprintf("the authorisation is for step %d and this is step %d",
			a.Step, step)
	case a.Epoch != at.Epoch || a.Generation != at.Generation:
		return false, fmt.Sprintf("the authorisation was given in observation %d/%d and "+
			"this is %d/%d; the screen it was granted about is not the screen "+
			"standing now", a.Epoch, a.Generation, at.Epoch, at.Generation)
	case a.Expires.IsZero():
		return false, "the authorisation has no expiry, which would make it a session flag"
	case !now.Before(a.Expires):
		return false, fmt.Sprintf("the authorisation lapsed at %s",
			a.Expires.UTC().Format(time.RFC3339))
	}
	return true, ""
}

// Intent is what a person is being asked to authorise, in terms they can check against the screen.
//
// Deliberately not the Marco source. The source is `OS's Click` at two integers, which answers
// "what will be sent" and not "what am I agreeing to"; a person looking at a menu needs the
// control's name, where it is, and where the workflow thinks it goes.
type Intent struct {
	Step int
	// Epoch and Generation are the observation this question is being asked about.
	//
	// Carried so the grant can be stamped with it. The stamp does NOT prove the world
	// stood still — a granter that simply copies these numbers satisfies it — and it is
	// not there for that. It catches a granter that hands back a CACHED permission from
	// an earlier screen, which is the shape a "remember my answer" convenience takes. The
	// world standing still is a different question, asked by re-observing it at the
	// instant of the press; see the runner's WithConfirm.
	Epoch      uint64
	Generation uint64
	// Control is the artifact's own label for the thing that would be pressed.
	Control string
	// Screen is the label of the screen the step starts on, as established in this
	// observation.
	Screen string
	// Expect is the label of the screen the workflow says follows.
	Expect string
	// At is where the press would land, in desktop coordinates. The real number, because a
	// person authorising a click is entitled to know where it goes.
	At Point
	// Because is how the control was re-acquired, in the matcher's own words.
	Because string
}

// Describe is the one-line form a prompt shows.
func (i Intent) Describe() string {
	return fmt.Sprintf("step %d: press %s on %s at (%d,%d), expecting %s",
		i.Step, i.Control, i.Screen, i.At.X, i.At.Y, i.Expect)
}

// Granter is asked for the right to perform one action.
//
// An interface rather than a flag, so the answer comes from outside the run every time it is
// needed. Returning false is a complete answer and stops the walk; it is not an error.
type Granter interface {
	Authorise(ctx context.Context, in Intent) (Authorisation, bool)
}
