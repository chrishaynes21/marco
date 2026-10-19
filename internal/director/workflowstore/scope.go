package workflowstore

// WHICH APPLICATION A WORKFLOW BELONGS TO.
//
// # The defect this closes
//
// `Artifact` recorded no application at all. Not a weak one, not a mutable one — none. So a
// workflow demonstrated against Ark, handed to `perform-workflow --application solasta`, would
// acquire Solasta's window and start resolving Ark's controls against it. The only thing in the
// way was a person answering *is this screen_a?* with no.
//
// A person catching it is not scope retention. The owner's call:
//
//	Marco's executable workflow doesn't know which application it belongs to. That means an Ark
//	workflow could potentially be directed at Solasta, with the user left to catch the mistake.
//	That's an architectural safety defect, not a perception problem.
//
// # What the identity is, and what it deliberately is not
//
// The executable IMAGE NAME, lowercased, as `winctx.LiveWindow.Image` reports it and as
// `windowref.Ref.Application` already carries it — "the semantic identity that DOES survive a
// restart, and the thing reacquisition searches by".
//
// NOT a PID: a relaunched game is the same application and a different process. NOT a window
// handle, for the same reason and because handles are recycled. NOT a title: titles change
// while a window lives, two applications can share one, and a game window's title is the
// commonest place an account name appears — which is why `capture-game-session` refuses to
// record one and why this does too.
//
// # Legacy artifacts are UNSCOPED, which is not the same as universal
//
//	Existing artifacts without application scope must remain readable, but they must not
//	silently become valid for every application.
//
// So an empty scope answers `false` to every `Allows`, with a sentence saying it was saved
// before scope existed. Reading one is fine; running one live needs it bound first, from
// observed provenance, which is a deliberate act and not a side effect of whatever happens to
// be in front.

import (
	"fmt"
	"strings"
)

// Scope is the application a workflow was demonstrated against.
//
// A string rather than a struct, because there is exactly one durable fact here and a struct
// with one field invites a second that should not exist. The empty value is a legacy artifact.
type Scope string

// NormaliseScope is how an application name becomes a scope.
//
// One place, so a scope written at teaching time and a scope compared at replay cannot differ
// by case or whitespace. The platform already lowercases `Image`; this does not trust that.
func NormaliseScope(application string) Scope {
	return Scope(strings.ToLower(strings.TrimSpace(application)))
}

// Known reports whether this artifact says which application it belongs to.
func (s Scope) Known() bool { return s != "" }

// Allows reports whether this scope permits acting on an application, and why not.
//
// The refusal carries the sentence because every refusal in this subsystem does: "the evidence
// does not agree" sends somebody nowhere, and this one has to name both applications so a
// person can see at a glance that they pointed the wrong file at the wrong game.
func (s Scope) Allows(application string) (bool, string) {
	got := NormaliseScope(application)
	switch {
	case !s.Known():
		return false, "this workflow was saved before workflows recorded which " +
			"application they belong to, so it is UNSCOPED — it is readable, and " +
			"it is not valid for any application until it is bound to one from " +
			"observed provenance"
	case !got.Known():
		return false, fmt.Sprintf("this workflow belongs to %q and the application "+
			"in front could not be identified, so there is nothing to agree with",
			string(s))
	case got != s:
		return false, fmt.Sprintf("this workflow was demonstrated against %q and the "+
			"application resolved now is %q; a workflow may only act on the "+
			"application it was shown in", string(s), string(got))
	default:
		return true, fmt.Sprintf("this workflow was demonstrated against %q and that "+
			"is the application resolved now", string(s))
	}
}

// Scoped reports whether this artifact knows which application it belongs to.
func (a Artifact) Scoped() bool { return a.Application.Known() }

// ScopeAllows is the artifact's own answer to "may I act on this application".
//
// Asked of the artifact rather than of the scope so a caller cannot forget that an unscoped
// artifact exists: there is one call, it returns a sentence, and the sentence differs for the
// three ways this can be no.
func (a Artifact) ScopeAllows(application string) (bool, string) {
	ok, why := a.Application.Allows(application)
	if !ok {
		return false, fmt.Sprintf("%q: %s", a.Name, why)
	}
	return true, why
}

// ScopedTo returns a copy of this artifact bound to an application.
//
// # Why this takes OBSERVED provenance and not a name somebody typed
//
//	Never blindly trust an application name supplied by the caller if it disagrees with
//	observed provenance.
//
// The caller passes what the window layer reported about the window it actually watched —
// `windowref.Ref.Application` — and never the `--application` argument. A selector is what
// somebody asked to watch; the image name is what they watched. Those are usually the same and
// the one time they differ is the one time it matters.
//
// Refuses to change a scope that is already set. Rebinding is how a workflow would quietly
// become valid for a second application, and a workflow that needs a different scope is a
// workflow that needs demonstrating there.
func (a Artifact) ScopedTo(observed string) (Artifact, error) {
	got := NormaliseScope(observed)
	switch {
	case !got.Known():
		return a, fmt.Errorf("workflowstore: no application was observed, so %q "+
			"cannot be bound to one", a.Name)
	case a.Application.Known() && a.Application != got:
		return a, fmt.Errorf("workflowstore: %q is already scoped to %q and cannot be "+
			"rebound to %q; a workflow for another application is another "+
			"demonstration", a.Name, string(a.Application), string(got))
	}
	out := a
	out.Application = got
	return out, nil
}
