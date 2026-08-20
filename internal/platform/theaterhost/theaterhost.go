// Package theaterhost puts on a production: it takes the semantic thing a play asked for and
// makes it happen with whatever the machine can currently do.
//
// # Director decides what should happen; Theater makes it happen
//
// A play says `do Theater's Activate with target1`, where target1 is a thing called "Mouse". It
// does not say how. This package answers how, on the stage it finds tonight:
//
//	Stage         what is live: which window, which place, what can be reached
//	Resolution    which live thing IS that target — zero, one, or several
//	Casting       which Actor can perform this part on this stage
//	Production    the Actor performs it
//	Verification  did the world change the way the play said it would
//
// # Why the play must not choose
//
// Because a play that named its provider would require that provider forever. The demonstration
// was watched through accessibility; that is provenance, recorded on the durable target. Tonight
// the accessibility bridge might be missing and sight might be available, and the same play
// should still go on. See [[ADR-068-the-theater-is-the-durable-semantic-world]].
//
// # What this package may not do
//
// It performs nothing itself. Every effect goes through an Actor, and an Actor is supplied from
// outside — so a Theater with no Actors is inert by construction rather than by intention, and
// there is no path from a resolution to an effect that does not pass through casting.
package theaterhost

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/activate"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Target is the semantic thing a play asked for.
//
// The whole of what a play may say about it: what it is called, and what sort of thing it is.
// There is deliberately no field for a runtime id, an element handle, a rectangle or a
// coordinate — those are how an Actor finds it, and they belong to the Actor.
type Target struct {
	// Name is what the target is called. A reference into what Marco already knows, not a
	// search string the play invented.
	Name string
	// Kind narrows what sort of thing to look for. Empty means "whatever it is", which is
	// treated as saying nothing rather than as excluding anything.
	Kind string
	// Window scopes the search, empty meaning whatever is in front.
	//
	// A SCOPE, not a handle. Which window a production belongs to is knowledge the caller
	// has and the Theater cannot guess — and a search without it finds a control of the
	// right name in whatever else happens to be open, which on a desktop is a coin toss
	// that looks like working.
	Window string
}

// Candidate is one live thing an Actor believes is the target, right now.
//
// Deliberately opaque above the Actor that produced it: `Handle` means whatever that Actor needs
// it to mean and is never durable, never compared across Actors, and never written anywhere. It
// exists so a resolution can be handed back to the Actor that made it.
type Candidate struct {
	// Handle is the Actor's own way of naming what it found. Ephemeral by definition.
	Handle string
	// Describes is a short human phrase for diagnostics — never durable, never a name.
	Describes string
	// Window is the scope this candidate was found in, carried so the cast program acts in
	// the same place the search did. Empty is "whatever is in front".
	Window string
}

// Actor is something that can find and perform on a target.
//
// # Why one interface and not two
//
// Because resolving and performing are the same competence seen twice. An Actor that can find the
// Mouse button through the accessibility tree is the Actor that can press it there; one that finds
// it by sight presses it by pointing. Splitting them would let Marco resolve with one and act with
// another, which is a resolution about a different thing.
//
// An Actor is a CLASS of capability, not an instance of a control — the same sense in which a
// Marco play has actors. Marco's `actor` is a thing in the play; a Theater Actor is a thing that
// can perform a part. Same metaphor, opposite sides of the curtain.
type Actor interface {
	// Name is what this Actor is called, for diagnostics and for casting notes.
	Name() string
	// Available reports whether this Actor can do anything at all right now.
	//
	// Asked before casting, so a missing bridge is "not available tonight" rather than an
	// error in the middle of a production.
	Available(ctx context.Context) bool
	// Find is every live candidate this Actor believes is the target.
	//
	// Zero, one or several. It must NOT choose among several — see Activate for why that
	// refusal belongs to the Theater rather than to whoever happened to look.
	//
	// A READ. An Actor may look however it likes; only ACTING is constrained below.
	Find(ctx context.Context, t Target) ([]Candidate, error)
	// Cast is the legal Marco this Actor would run to activate one candidate, one way.
	//
	// # It performs NOTHING
	//
	// An Actor expresses its part; the Theater's Production boundary runs it, through an
	// injected MarcoRunner. That is not ceremony — it is the invariant that every real input
	// passes through Marco compilation, which `MarcoRunner.Run` guarantees returns a compile
	// error "BEFORE any desktop mutation".
	//
	// An Actor that called a host directly would be a second route from the Director to an
	// effect: no compile gate, and nothing for a dry run to record. The live rehearsal path
	// has always compiled; this is how the Theater does too, rather than the two of them
	// having separate answers again.
	//
	// `ok` false means this Actor cannot express that way at all — distinct from the control
	// not implementing it, which only running can discover.
	//
	// Deleting the compile route — performing here — must fail
	// TestAnActorNeverReachesAHostDirectly.
	Cast(c Candidate, w activate.Way) (program string, ok bool)
}

// Refusal is the CLOSED vocabulary of why a production did not go on.
//
// Silence is the hard case here as everywhere else: "nothing happened" has four explanations and
// they call for four different responses — the person is on the wrong screen, the name has
// changed, the screen has two of them, or this machine cannot act at all.
type Refusal string

const (
	// TargetNotFound is a target nothing on the current stage matches. The ordinary honest
	// answer when a play is run somewhere it was not meant for.
	TargetNotFound Refusal = "target_not_found"
	// TargetAmbiguous is several matches and no way to tell which was meant.
	//
	// A REFUSAL, never a choice. Pressing the first of several controls sharing a name is a
	// coin toss performed on somebody's computer, and it would be indistinguishable from
	// working — which is worse than failing, because nobody would look.
	TargetAmbiguous Refusal = "target_ambiguous"
	// NoActorAvailable is a stage with nothing on it that can act. Distinct from not finding
	// the target: Marco may know perfectly well what was meant and have no way to do it.
	NoActorAvailable Refusal = "no_actor_available"
	// PerformFailed is an Actor that tried and could not.
	PerformFailed Refusal = "perform_failed"
	// NotVerified is an action that was performed and could not be shown to have worked.
	//
	// Kept apart from PerformFailed deliberately. "I pressed it and the world did not change"
	// is a different fact from "I could not press it", and only one of them means the play
	// should stop trusting what it thinks it knows.
	NotVerified Refusal = "not_verified"
)

// Production is what happened when the Theater tried to put a target activation on.
type Production struct {
	// Performed says an Actor actually acted. False with a Refusal is the Theater declining
	// BEFORE anything reached the machine, which is a different fact from trying and failing.
	Performed bool
	// Cast is which Actor performed it, for the record. Provenance about this attempt, never
	// a requirement on the next one.
	Cast string
	// Refused is the closed reason, empty when it went on.
	Refused Refusal
	// Detail is a human sentence about the refusal. Never a durable claim.
	Detail string
	// Program is the Marco that was run, empty when nothing was. The Actor writes it and
	// the boundary runs it, so this is the only place a caller can see what was sent.
	Program string
}

// Theater casts an available Actor and puts the production on.
//
// # Where verification is, and is not
//
// Not here. `Activate` resolves, casts and runs; whether the world then became what somebody
// expected is asked at `Perform`, of a verifier the CALLER brings. There were briefly two ideas
// of verification in this file — a `Changed(ctx) bool` for the play path and a
// `production.Verifier` for the Director's — and two answers to "did that work" is exactly the
// drift Roadmap 34E exists to end. A standalone runtime brings nothing, which is honest, and
// gets `not_verified` rather than a refusal it cannot act on.
type Theater struct {
	actors []Actor
	// runner is how a cast action reaches a machine: compiled Marco, never a direct host
	// call. Injected, so cmd/director supplies its real or recording runner and cmd/marco
	// supplies its own. See run.go.
	runner directorapi.MarcoRunner
}

// New builds a Theater over the Actors it may cast.
//
// Order is the CASTING ORDER and is the caller's to decide: the first available Actor that finds
// exactly one candidate performs. It is not a preference between providers so much as a statement
// about which is cheapest to ask.
func New(actors ...Actor) *Theater {
	return &Theater{actors: actors}
}

// Player is one Actor's readiness tonight. Diagnostics only — nothing decides anything on it.
type Player struct {
	// Name is what the Actor calls itself.
	Name string `json:"name"`
	// Available is the answer to the SAME question casting asks, at the moment it was asked.
	Available bool `json:"available"`
}

// Roster is who is in the Theater and who can act, in casting order.
//
// # Why this exists
//
// Because "nothing happened" is the hard silence. `no_actor_available` is the honest refusal for
// a machine that cannot act, but it is only reachable by RUNNING a play — so the first time
// anybody discovers that the Theater is empty is when a learned play they just saved does
// nothing. A person should be able to ask before that.
//
// # Why it must not be a second opinion
//
// It calls a.Available(ctx) on t.actors in order — the same predicate, on the same actors, in the
// same sequence Activate walks. A roster that computed readiness its own way would be a
// diagnostic that agrees with the product right up until the moment somebody needs it to
// disagree, which is the one moment it is consulted.
//
// Reporting `available: false` rather than omitting the Actor on purpose: "accessibility is here
// and cannot act" and "there is no accessibility actor at all" are different machines with
// different fixes.
func (t *Theater) Roster(ctx context.Context) []Player {
	if t == nil {
		return nil
	}
	out := make([]Player, 0, len(t.actors))
	for _, a := range t.actors {
		out = append(out, Player{Name: a.Name(), Available: a.Available(ctx)})
	}
	return out
}

// Activate is the whole production: resolve, cast, perform, verify.
//
// # Why resolution happens per Actor and refusal happens here
//
// Because "several things are called Mouse" is a fact about the stage, not about whoever looked.
// An Actor that picked one would be making a semantic decision inside a lookup, where nothing
// could see it. So an Actor reports what it found and the Theater decides — and decides to stop.
func (t *Theater) Activate(ctx context.Context, target Target) Production {
	if strings.TrimSpace(target.Name) == "" {
		return Production{Refused: TargetNotFound, Detail: "a target needs a name"}
	}
	available := 0
	for _, a := range t.actors {
		if !a.Available(ctx) {
			continue
		}
		available++
		found, err := a.Find(ctx, target)
		if err != nil {
			// This Actor could not look. Another might; a failure to search is not a
			// finding, and treating it as "not there" would hide a broken bridge behind
			// a sentence about the screen.
			continue
		}
		switch len(found) {
		case 0:
			continue
		case 1:
			// The candidate carries the scope it was found in, so the cast program acts
			// in the same window the search did.
			c := found[0]
			c.Window = target.Window
			program, err := t.run(ctx, a, c)
			if err != nil {
				return Production{
					Cast: a.Name(), Refused: PerformFailed,
					Detail: err.Error(), Program: program,
				}
			}
			// PERFORMED, which is not the same as verified — an Actor sending something
			// is not the application having done anything. That question is asked at
			// Perform, of a verifier the caller brings. See NotVerified.
			return Production{Performed: true, Cast: a.Name(), Program: program}
		default:
			return Production{
				Cast:    a.Name(),
				Refused: TargetAmbiguous,
				Detail: fmt.Sprintf("%d things here are called %q, so I cannot tell "+
					"which one you meant: %s", len(found), target.Name,
					describe(found)),
			}
		}
	}
	if available == 0 {
		return Production{
			Refused: NoActorAvailable,
			Detail: "nothing on this machine can act on a target right now, so this play " +
				"cannot go on here",
		}
	}
	return Production{
		Refused: TargetNotFound,
		Detail:  fmt.Sprintf("nothing here is called %q", target.Name),
	}
}

// describe lists what was found, for an ambiguity a person has to resolve.
//
// Sorted, so the same stage produces the same sentence twice.
func describe(found []Candidate) string {
	out := make([]string, 0, len(found))
	for _, c := range found {
		if c.Describes != "" {
			out = append(out, c.Describes)
		}
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// ── the Marco boundary ────────────────────────────────────────────────────────

// Host fulfils the Theater act.
//
// One capability, and the whole of what a play may ask: activate this semantic target. Anything
// else is a refusal rather than a silent ok — the act declares what exists, and a capability
// nobody declared is a mistake somebody should see.
type Host struct {
	theater *Theater
	// last is why the most recent production did not go on, for diagnostics only. It never
	// changes what a play is told: the play gets ok or failed, and nothing else.
	last string
}

// NewHost builds the Theater act's implementation.
//
// A nil Theater is a real answer: a Marco with nothing that can act on a target refuses rather
// than degrading into a click at a remembered coordinate.
func NewHost(t *Theater) *Host { return &Host{theater: t} }

// Last is why the most recent production did not go on, for diagnostics.
func (h *Host) Last() string { return h.last }

// Roster is who this host could cast, in casting order. Nil when there is no Theater at all,
// empty when the Theater has nobody in it — which are different machines.
func (h *Host) Roster(ctx context.Context) []Player {
	if h == nil {
		return nil
	}
	return h.theater.Roster(ctx)
}

func (h *Host) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	switch strings.ToLower(c.Action) {
	case "activate":
		return h.activate(c)
	default:
		return failed(fmt.Sprintf("the Theater act has no %q", c.Action))
	}
}

// activate puts on one target activation.
func (h *Host) activate(c runtime.HostCall) (string, runtime.Value, error) {
	set := c.Input.AsSet()
	if set == nil {
		return h.refuse("activate needs a Target")
	}
	target := Target{
		Name: strings.TrimSpace(fieldText(set, "Name")),
		Kind: strings.TrimSpace(fieldText(set, "Kind")),
	}
	if target.Name == "" {
		return h.refuse("a target needs a name")
	}
	if h.theater == nil {
		return h.refuse("nothing here can act on a target")
	}
	p := h.theater.Activate(c.Ctx, target)
	if p.Refused != "" {
		// The closed reason travels with the sentence. A play sees `failed` either way;
		// a person reading the diagnostics needs to know whether the screen was wrong,
		// the name was, or the machine simply cannot act.
		return h.refuse(string(p.Refused) + ": " + p.Detail)
	}
	return "ok", runtime.Absent(), nil
}

func (h *Host) refuse(why string) (string, runtime.Value, error) {
	h.last = why
	return failed(why)
}

func failed(msg string) (string, runtime.Value, error) {
	return "failed", runtime.ErrVal(&runtime.Err{Message: msg}), nil
}

// fieldText reads one text field out of a set, empty when it is absent.
//
// An absent field is not an error here: Kind is optional by design, and a Target that gave one
// and a Target that did not are both legal sentences.
func fieldText(s *runtime.Set, name string) string {
	if s == nil {
		return ""
	}
	v, _ := s.Get(name)
	return v.AsText()
}
