package orchestrator

// A ROUTINE CAN BE CALLED SOMETHING ELSE.
//
// # It is a move, and the move engine was already here
//
// Nothing renamed a routine before this, and almost nothing new was needed to. `Rescope` is a
// complete move: two locks in a fixed order, both ends revalidated against the digests the review
// was built from, an EXCLUSIVE create unless a version-bound consent says otherwise, the
// destination READ BACK before the original is removed, and the recording, the step account
// (rebound to the source the move writes) and the phrases all carried across. A rename is that
// move with a destination that differs by SLUG instead of by application or scope.
//
// So this file is the three things a rename has that a rescope does not.
//
// # One: the name is where the arguments are declared
//
// `cloud pull stone with item` declares `item`, and `routes.SplitArgs` is what reads it. A
// free-text rename box is therefore a way to drop an argument without being told — the routine
// saves, and the next Play types the literal text the demonstration typed. So the caller gives
// the BASE name only and the clause is carried here, out of the routine's current name. The
// window shows it beside the box, not in it.
//
// # Two: a declared secret is qualified by the routine's own name
//
// `codegen` writes a declared secret argument under a ROUTE-QUALIFIED name — and the route is the
// routine's own name, slugged. A rename re-renders, so the identifier moves with the name, and
// the routine goes to the credential store under a name nobody has ever saved anything under.
//
// It is the one re-render that cannot be preserved, by construction: every other one keeps the
// route name and can reproduce the identifier. So a rename REFUSES that case, through the one
// check all four re-renders share — see secretkeep.go. Nothing is read from the store and
// nothing is migrated into it.
//
// # Three: a collision is refused and never replaced
//
// `Rescope` can replace what is at the destination, under a consent bound to the version
// somebody was shown. A rename offers no such thing: two routines of one name in one place is
// the collision, and the answer to it is a different name. The refusal is raised at preparation
// so there is never a press that could take one.
//
// See [[ADR-285-a-routine-can-be-called-something-else]].

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// PrepareRename renders a saved routine under another name, for review.
//
// IT WRITES NOTHING. `newBase` is the name without its `with …` clause; the clause is taken off
// the routine's current name and put back, so an argument cannot be dropped by retyping.
//
// The result is a `Rescoping` and is performed by `Rescope` — one move engine, so the read-back,
// the revalidation and the original-retention reporting are the same ones. Pass `Consent{}`: a
// rename never replaces anything, and a `Rescoping` whose destination is taken comes back
// REFUSED rather than movable.
func (d Deps) PrepareRename(rt routes.Route, newBase string) (Rescoping, error) {
	if rt.Slug == "" {
		return Rescoping{}, fmt.Errorf("a rename needs the routine it is about")
	}
	if strings.TrimSpace(newBase) == "" {
		return Rescoping{}, fmt.Errorf("say what it should be called")
	}
	// THE SCOPE IT ALREADY HAS, which is what makes this a rename and not a rescope.
	// Passed through rather than assumed: `rescopeTarget` works out the destination's
	// application from it, and the one scope that has none is the one that must not
	// acquire one on the way past.
	m, err := d.prepareMove(rt, ScopeAlreadyOf(rt), rt.App, newBase)
	if err != nil {
		return Rescoping{}, err
	}
	if m.Refused != "" {
		return m, nil
	}
	// ── THE NAME IT ALREADY HAS IS NOT A RENAME ──
	//
	// Said rather than performed, for the reason `PrepareRescope` refuses the scope a
	// routine already has: the destination identity would be the source identity, which
	// means one lock taken twice, an exclusive create onto the file being moved, and a
	// removal of what had just been written. Compared on the SLUG, because that is what
	// the identity is made of — `Cloud Pull Stone` and `cloud pull stone` are one place.
	if m.To.Slug == m.From.Slug {
		return Rescoping{}, fmt.Errorf("%q is already called that, so there is "+
			"nothing to rename", m.Name)
	}
	// ── AND A COLLISION IS REFUSED, WITH NOTHING TO AGREE TO ──
	if m.Taken {
		m.Refused = fmt.Sprintf("there is already a routine at %s. Nothing has been "+
			"renamed — choose another name, or delete that one first",
			m.TakenID)
		m.Taken, m.TakenSays = false, ""
		return m.sealed(), nil
	}
	return m, nil
}

// renamedTo is the full new name: the base somebody typed, with the clause that declares this
// routine's arguments put back on the end.
//
// `now` is the name the routine has, which is where the clause comes from. A base that carries
// its own `with …` is REFUSED rather than merged — somebody typing one means to change the
// arguments, and a rename is not the operation that does that; merging would silently produce
// either a duplicate clause or a quiet disagreement with the placeholders in the source.
func renamedTo(now, base string, rt routes.Route) (string, string) {
	base = strings.TrimSpace(base)
	if _, typed := routes.SplitArgs(base); len(typed) > 0 {
		return "", "a rename changes what a routine is CALLED, and “with …” is how " +
			"it says what it asks for — leave that part off and the values it " +
			"already asks for are kept"
	}
	if routes.Slug(base) == "" {
		return "", fmt.Sprintf("%q has no letters or digits in it, so there would be "+
			"nothing to file the routine under", base)
	}
	_, args := routes.SplitArgs(now)
	if len(args) == 0 {
		return base, ""
	}
	return base + " with " + strings.Join(args, ", "), ""
}
