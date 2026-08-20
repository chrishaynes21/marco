// Package production is the contract between the Director and the Theater.
//
// # Why it exists, and why it is empty of machinery
//
// Because the Director may not import the Theater. `internal/director/rehearse` holds no host,
// opens no window and imports nothing that could — a tested boundary, and the reason a rehearsal
// cannot act by itself. `theaterhost` is a platform adapter and lives on the other side of that
// line.
//
// So the two meet here: the Director names what production it wants and what permits it, the
// Theater implements the port, and whoever constructs the Director decides which Theater is at
// the other end. The same shape as `directorapi.MarcoRunner`, for the same reason.
//
// # What this package may never grow
//
// Machinery. It is types and one interface. A helper here would be a third place that knows how
// productions work, and the whole point of Roadmap 34E is that there is one body.
package production

import (
	"context"
	"fmt"
	"strings"
)

// Target is a semantic thing that can be acted upon.
//
// A REFERENCE into what Marco already knows, never a search string somebody invented. Name and
// Kind and nothing else: no runtime id, no element handle, no provider. A Director that handed
// over a resolved accessibility element would have decided how to act, which is the Theater's
// business and the migration this contract exists to complete.
type Target struct {
	// Name is what the target is called.
	Name string
	// Kind narrows what sort of thing to look for. Empty says nothing rather than
	// excluding anything.
	Kind string
}

// Request is one production the Director wants put on.
//
// Deliberately narrow. Everything here is semantic: what to act on, and what the Director expects
// to be true afterwards. Nothing about HOW — no provider, no capability, no handle, no window
// internals — because the Theater decides how the current stage can realise it.
type Request struct {
	// Target is what to act on.
	Target Target
	// Window scopes the search, empty meaning whatever is in front.
	//
	// A scope, not a handle: it says which window this production belongs to, which the
	// Director does know and the Theater cannot guess.
	Window string
	// Expect is the durable place this production should reach, empty when the Director has
	// no expectation. Local verification checks against it.
	Expect string
}

// Authority is proof that ONE production is permitted.
//
// # The invariant this type exists to hold
//
// Theater consumes authority. It never creates, widens or infers it. The Audience gives
// permission, the Director mints and holds it, and the Theater is handed something it can spend
// exactly once.
//
// Claim is called before anything reaches a machine and refuses if the permission does not cover
// this production or has already been spent — so a Theater that tried twice would be refused by
// the authority rather than trusted not to.
type Authority interface {
	// Claim spends the permission for this production, or says why it may not.
	Claim(Request) error
}

// Producer is the Theater, as the Director may ask for it.
//
// One method. The Director says what it wants, what permits it and what will check it; everything
// else — which actors exist, which can reach the target, how to express the action — is the
// Theater's.
//
// # Why the verifier arrives with the request rather than with the Theater
//
// Because it belongs to the CALLER, not to the stage. One Theater serves a saved play run from a
// standalone runtime, which has no observation stack and honestly brings nothing, and a live
// rehearsal, which brings the Director's own settled observation. A verifier installed at
// construction would be one caller's answer applied to another's production — and, with both
// callers live in one process, a race over a shared field.
//
// Nil is a real answer and produces `not_verified`. It must never produce success.
type Producer interface {
	Perform(ctx context.Context, r Request, a Authority, v Verifier) Report
}

// Refusal is the CLOSED vocabulary of why a production did not happen, or could not be shown to
// have happened.
//
// One vocabulary for both callers. A rehearsal and a saved play meeting the same wall must say
// the same word, or a person debugging one learns nothing about the other — and two vocabularies
// is how the two bodies drifted in the first place.
type Refusal string

const (
	// NotPermitted is authority refusing: no grant, spent, expired, or for another
	// production. Never a statement about the stage.
	NotPermitted Refusal = "not_permitted"
	// TargetNotFound is nothing on the current stage answering to that name.
	TargetNotFound Refusal = "target_not_found"
	// TargetAmbiguous is several things answering to it. The Theater does not choose.
	TargetAmbiguous Refusal = "target_ambiguous"
	// NoActorAvailable is nothing on this machine able to act right now.
	NoActorAvailable Refusal = "no_actor_available"
	// PerformFailed is an actor that was cast, tried, and could not.
	PerformFailed Refusal = "perform_failed"
	// NotVerified is a production that happened and could not be shown to have worked.
	//
	// Never success. An actor reporting that it sent something is not the application
	// having done anything, and a Theater with nothing to check with says so.
	NotVerified Refusal = "not_verified"
)

// Report is what the Theater tells the Director about one production.
//
// Facts, not policy. Whether this satisfies the Audience's goal, whether to try something else,
// whether to ask — all of that is the Director's, and nothing here decides it.
type Report struct {
	// Attempted says a request reached the Theater and authority permitted it.
	Attempted bool
	// Performed says an Actor actually acted. False with a Refusal is the Theater
	// declining BEFORE anything reached the machine, which is a different fact from
	// trying and failing.
	Performed bool
	// Cast is which Actor performed it. Provenance about this production, never a
	// requirement on the next one.
	Cast string
	// Program is the Marco that was run, empty when nothing was.
	//
	// Carried back because the Director is the caller that SHOWS it: a dry run prints the
	// program it would have sent, and that display predates the Theater. Now that the Actor
	// writes the program rather than the Director, the only way the report can still say
	// what was sent is for it to come back over the boundary.
	Program string
	// Verified says the world afterwards was what the Director expected.
	Verified bool
	// Observed is where the stage ended up, when verification could tell. Empty when it
	// could not.
	Observed string
	// Refused is the closed reason, empty when the production went on and verified.
	Refused Refusal
	// Detail is the human sentence behind the refusal — which target, which actor, what
	// the host said. Never a durable claim, and never parsed.
	Detail string
}

// Failed reports whether anything is wrong with this production.
func (r Report) Failed() bool { return r.Refused != "" }

// Verifier decides whether the production actually changed the world as expected.
//
// # Why it is injected, and why nil is honest
//
// Because verification is the Director's existing machinery — the settled observation, the
// recall against durable memory, the closed outcome vocabulary — and a Theater that grew its own
// would be a second answer to the one question every other part of this system asks carefully.
//
// A standalone runtime running a saved play has no observation stack at all. Nil is the truthful
// answer there, and it produces `not_verified` rather than a claim nobody can support. What must
// never happen is nil producing success.
type Verifier interface {
	// Verify reports what the world looks like after a production, and whether it is what
	// the request expected.
	Verify(ctx context.Context, r Request) (observed string, verified bool)
}

// Refuse builds a refused report.
func Refuse(reason Refusal, format string, args ...any) Report {
	return Report{Refused: reason, Detail: fmt.Sprintf(format, args...)}
}

// Describe is a one-line reading of a report, for a log or a panel.
func Describe(r Report) string {
	var b strings.Builder
	switch {
	case !r.Attempted:
		b.WriteString("not attempted")
	case !r.Performed:
		b.WriteString("nothing was sent")
	case r.Verified:
		b.WriteString("performed and verified")
	default:
		b.WriteString("performed")
	}
	if r.Cast != "" {
		b.WriteString(" by " + r.Cast)
	}
	if r.Refused != "" {
		b.WriteString(" — " + string(r.Refused))
		if r.Detail != "" {
			b.WriteString(": " + r.Detail)
		}
	}
	return b.String()
}
