// Package screenhost answers one question and can do nothing else.
//
//	do Screen's Showing with "the pause menu"
//	→ ok      the screen in front IS the one the user named
//	→ failed  anything else at all
//
// # Why this exists
//
// A learned play says where it begins — see [[ADR-030-a-play-says-where-it-begins]] — and until
// something could answer that sentence, every guarded play refused. This is the something.
//
// # What it may do
//
// Look, and compare. The interface it is given has three methods and every one of them is a read:
// which application is in front, which screen is in front, and what a name the user gave refers
// to. Nothing else. It cannot press a key, focus a window, run a route, write to memory, grant authority or
// register anything — and `TestTheScreenHostCannotAct` holds that structurally.
//
// **Perception, not action.** A play asking where it is must not thereby be doing something.
//
// # Silence is never yes
//
// Every way of failing to establish equality returns `failed`: an unknown name, an ambiguous name,
// an ambiguous screen, a screen nobody can see, a missing recogniser, unreadable memory. The play's
// `or?` arm then returns before its first effect. There is no path through this file that answers
// `ok` without having positively identified the named subject.
package screenhost

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// Recognition is the smallest read-only view of the world this host needs.
//
// Deliberately three methods and not an interface to Director. A host handed the Director runtime
// could do anything Director can do; this one can look at two things and read a name.
type Recognition interface {
	// Application is which program is in front, empty when that cannot be established.
	Application() string
	// CurrentSubject identifies the screen in front RIGHT NOW, freshly.
	//
	// Returns the durable subject id and how the recognition went. It must not write to
	// memory: answering a question about the world is not learning about it.
	CurrentSubject(application string) (id string, outcome Outcome)
	// SubjectNamed resolves a user-given screen name inside one application.
	//
	// Exact and scoped. Ambiguity is reported as not-found rather than resolved.
	SubjectNamed(application, name string) (id string, ok bool)
}

// Outcome is how a recognition attempt went.
//
// All of these become `failed` at the language boundary, and they are kept apart anyway: turning
// *"I could not look"* into *"I looked and it was different"* would be a lie told to whoever reads
// the diagnostics, and the two call for opposite fixes.
type Outcome string

const (
	// Recognised is a screen positively identified as one durable subject.
	Recognised Outcome = "recognised"
	// Ambiguous is a screen that could be more than one remembered subject. Marco does not
	// pick the nearest.
	Ambiguous Outcome = "ambiguous"
	// Unobservable is perception failing to see the screen at all.
	Unobservable Outcome = "unobservable"
	// Unavailable is the recogniser or memory not being there.
	Unavailable Outcome = "unavailable"
	// Unknown is a screen observed cleanly and matching nothing remembered.
	Unknown Outcome = "unrecognised"
)

// Host fulfils the Screen act.
type Host struct {
	world Recognition
	// last is why the most recent question failed, for diagnostics only. It never changes
	// what a play is told: the play gets ok or failed, and nothing else.
	last string
}

// New builds a Screen host over a read-only view of the world.
func New(world Recognition) *Host { return &Host{world: world} }

// Why is the reason the last question failed, for `marco` diagnostics.
func (h *Host) Why() string { return h.last }

// Invoke answers the Screen act's capabilities.
func (h *Host) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	switch strings.ToLower(c.Action) {
	case "showing":
		return h.showing(c)
	default:
		// A capability this host does not know is a refusal, never a silent ok. The act
		// declares what exists; anything else is a mistake somebody should see.
		return fail(fmt.Sprintf("the Screen act has no %q", c.Action))
	}
}

// showing reports whether the screen in front is the one the user named.
func (h *Host) showing(c runtime.HostCall) (string, runtime.Value, error) {
	name := strings.TrimSpace(c.Input.AsText())
	if name == "" {
		return h.refuse("a screen name is needed")
	}
	if h.world == nil {
		// No recogniser wired. A standalone Marco cannot tell where it is, so a guarded
		// play refuses — it does not degrade into blind replay.
		return h.refuse("nothing here can recognise a screen")
	}
	app := h.world.Application()
	if app == "" {
		return h.refuse("no application is in front")
	}
	// The name is resolved INSIDE the current application. `settings` in one program is not
	// `settings` in another, and there is no fallback that would let it be.
	want, ok := h.world.SubjectNamed(app, name)
	if !ok {
		return h.refuse(fmt.Sprintf("nothing in %s is called %q", app, name))
	}
	got, outcome := h.world.CurrentSubject(app)
	switch outcome {
	case Recognised:
		if got == want {
			h.last = ""
			return ok2()
		}
		return h.refuse("this is a different screen")
	case Ambiguous:
		return h.refuse("this screen could be more than one Marco remembers")
	case Unobservable:
		return h.refuse("Marco could not see the screen")
	case Unavailable:
		return h.refuse("Marco could not check")
	default:
		return h.refuse("Marco does not recognise this screen")
	}
}

func (h *Host) refuse(why string) (string, runtime.Value, error) {
	h.last = why
	return fail(why)
}

func ok2() (string, runtime.Value, error) { return "ok", runtime.Absent(), nil }

func fail(msg string) (string, runtime.Value, error) {
	return "failed", runtime.ErrVal(&runtime.Err{Message: msg}), nil
}
