package orchestrator

// A RE-RENDER KEEPS THE CREDENTIAL IT WAS READING.
//
// # The defect
//
// Four operations in this package re-render a SAVED routine: a step edit, a rebuild under
// today's rules, a re-simplification, and a move. Every one of them hands `codegen` the set of
// argument names the route declares, and `codegen` writes a DECLARED secret argument under a
// ROUTE-QUALIFIED name — `sign-in-to-the-portal:password` — while an undeclared one becomes the
// bare `password`.
//
// The declared set came from `routes.SplitArgs` of the routine's NAME. A saved routine's header
// does not carry the `with …` clause: `Save` writes `type my address`, and the declaration lives
// in the STEPS, as `{{password}}` in the text of a step that types. So the set was empty for
// every routine in the library, and every re-render of one that reads a credential quietly
// rewrote the identifier.
//
// Those are two different entries in somebody's credential store. The review shows the same step
// list, the same sentence shape and the same word `Secret`; what changed is the one thing that
// decides which password comes back, and nothing comes back.
//
// # Two halves, and the second is not a belt for the first
//
// `secretArgsFor` PRESERVES: it reads the source being replaced and declares exactly the names
// that source reads under their qualified form, so the render reproduces them. That is enough
// for the operations whose placeholder survives in the steps and whose route name does not
// change — a step edit, a rebuild from the stored steps, a move between scopes.
//
// `secretsKept` REFUSES, and covers the ones where preservation is not possible at all:
//
//   - a RENAME changes the route name, so the qualified identifier changes with it by
//     construction;
//   - a re-simplification renders the DEMONSTRATION, which never contained the placeholder — a
//     secret's value never enters a recording — so there is nothing to reproduce it from;
//   - and a source somebody edited by hand can read a credential under a name no render of its
//     steps would ever produce.
//
// The alternative to refusing is moving the credential, and this code does not touch a
// credential store: it reads two pieces of generated text and compares names. See the secrets
// invariant in CLAUDE.md.
//
// See [[ADR-287-a-re-render-keeps-the-credential-it-was-reading]].

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// secretArgsFor is the secret-meaning argument names a re-render of THIS SOURCE must declare, so
// that the credentials it reads keep the names it reads them under.
//
// `base` is the routine's name without its arguments — what `codegen` makes the route's file base
// out of.
//
// # Why it reads the source rather than declaring every placeholder it finds
//
// Because the qualification is not a property of the name, it is a property of what the file
// SAYS. A routine can hold a bare `{{token}}` that its source reads as the global `token`, and
// declaring it would qualify it — the same defect pointing the other way. So each name is
// declared only when the source already reads it under the qualified form, and a name the source
// reads under neither is left undeclared and caught by `secretsKept`.
//
// NON-SECRET placeholders are not this function's business: `declaredFor` already declares them,
// and must, or `codegen` turns a value somebody types into a credential-store read.
func secretArgsFor(base, src string, steps []macroir.Step) []string {
	reads := map[string]bool{}
	for _, n := range codegen.SecretsIn(src) {
		reads[n] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range steps {
		if s.Kind != macroir.StepType {
			continue
		}
		for _, a := range routes.Arguments(s.Text) {
			name := strings.ToLower(a.Name)
			if seen[name] || !codegen.MeansASecret(name) {
				continue
			}
			seen[name] = true
			if reads[codegen.SecretNameFor(base, name)] {
				out = append(out, name)
			}
		}
	}
	return out
}

// renderArgsFor is everything a re-render of a SAVED routine must declare: the names the routine
// itself declares, and the credentials this source already reads under their qualified name.
//
// ONE FUNCTION BECAUSE THERE ARE THREE CALLERS, and three places assembling the same list is
// three places for one of them to miss half of it — which is how this defect reached three
// surfaces at once. See `declaredFor`, which answers the first half and is also the draft path's
// whole answer: a DRAFT carries its arguments in `Draft.Args` from the phrase somebody named it
// with, so its secrets were never unqualified.
func renderArgsFor(name, src string, steps []macroir.Step) (string, []string) {
	base, args := declaredFor(name, steps)
	return base, append(args, secretArgsFor(base, src, steps)...)
}

// secretsKept is "" when every credential the routine reads is still read under the same name,
// and the sentence to refuse with otherwise.
//
// `doing` names the operation, because the same refusal reaches four surfaces and "this" is not
// something a person can act on.
//
// A name present before and absent after is the defect. A name identical in both is preserved by
// construction, which is the ordinary case once `renderArgsFor` has done its work.
func secretsKept(was, now, doing string) string {
	after := map[string]bool{}
	for _, n := range codegen.SecretsIn(now) {
		after[n] = true
	}
	var lost []string
	for _, n := range codegen.SecretsIn(was) {
		if !after[n] {
			lost = append(lost, n)
		}
	}
	if len(lost) == 0 {
		return ""
	}
	instead := "would leave it reading no credential at all"
	if gained := newNames(was, now); len(gained) > 0 {
		instead = "would leave it reading " + quotedList(gained) + " instead"
	}
	return fmt.Sprintf("it reads %s from your credential store, and %s %s. Those are "+
		"different entries, so nothing would be found under the new name. "+
		"Nothing has been changed and nothing was read from the store",
		quotedList(lost), doing, instead)
}

// newNames is the credentials the new source reads that the old one did not.
func newNames(was, now string) []string {
	before := map[string]bool{}
	for _, n := range codegen.SecretsIn(was) {
		before[n] = true
	}
	var out []string
	for _, n := range codegen.SecretsIn(now) {
		if !before[n] {
			out = append(out, n)
		}
	}
	return out
}

// quotedList is the names in a sentence, each in quotes.
func quotedList(names []string) string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, fmt.Sprintf("%q", n))
	}
	switch len(out) {
	case 0:
		return "nothing"
	case 1:
		return out[0]
	case 2:
		return out[0] + " and " + out[1]
	}
	return strings.Join(out[:len(out)-1], ", ") + " and " + out[len(out)-1]
}
