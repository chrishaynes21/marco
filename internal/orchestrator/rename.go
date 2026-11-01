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
// `codegen.typeSentence` writes `do OS's Secret with "<route>:password"` for a declared secret
// argument, where `<route>` is `fileBase(name)` — the routine's name, slugged. Its own comment
// says what a disagreement about that costs: *looking up a secret nobody stored.*
//
// Renaming re-renders, so the qualifier would move, and the routine would go to the credential
// store under a name nobody has ever saved anything under. That is not a rename anybody asked
// for, and the alternative — moving the credential — is this window reaching into somebody's
// credential store, which it will not do. So the case is REFUSED, by name, and nothing is read.
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
	"regexp"
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

// secretRe finds every credential name a routine reads, as the generator writes them.
//
// `do OS's Secret with "<name>"` — the NAME, never the value: a secret's value never enters a
// route's source at all, which is the whole of why `OS's Secret` exists. See
// `codegen.typeSentence` and the secrets invariant in CLAUDE.md.
var secretRe = regexp.MustCompile(`OS's Secret with "([^"]*)"`)

// secretNames is every credential name in a source, in order, de-duplicated.
func secretNames(src string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range secretRe.FindAllStringSubmatch(src, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// secretsWouldMove is why this rename cannot be made, when it would send the routine to the
// credential store under a name nothing has been stored under.
//
// # Why it compares the two SOURCES rather than deriving the qualifier
//
// Because deriving it would be a second copy of a rule that lives in `codegen`, and the two
// would disagree the first time either changed — and disagreeing here means a rename that looks
// like it worked and a routine that cannot sign in. The question is not "how is a secret name
// built"; it is "does this routine ask for a different credential after the rename than before
// it", and both sources are right here.
//
// A name present before and absent after is the defect. A name that is the same in both is
// preserved by construction, which is the ordinary case: an UNQUALIFIED secret — a plain typed
// `{{password}}` — carries no route in it and is untouched by a rename.
//
// NOTHING IS READ FROM THE CREDENTIAL STORE and nothing is written to it. This is a comparison
// of two pieces of generated text.
func secretsWouldMove(was, now string) string {
	after := map[string]bool{}
	for _, n := range secretNames(now) {
		after[n] = true
	}
	var moved []string
	for _, n := range secretNames(was) {
		if !after[n] {
			moved = append(moved, n)
		}
	}
	if len(moved) == 0 {
		return ""
	}
	which := "a password"
	if len(moved) > 1 {
		which = "passwords"
	}
	return fmt.Sprintf("it reads %s from your credential store, under %s — and that "+
		"name is made out of the routine's own name, so renaming it would look "+
		"for something nobody stored. Nothing has been renamed and nothing was "+
		"read from the store. Rename is not available for this routine",
		which, quotedList(moved))
}

// quotedList is the names in a sentence, each in quotes.
func quotedList(names []string) string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, fmt.Sprintf("%q", n))
	}
	switch len(out) {
	case 1:
		return out[0]
	case 2:
		return out[0] + " and " + out[1]
	}
	return strings.Join(out[:len(out)-1], ", ") + " and " + out[len(out)-1]
}
