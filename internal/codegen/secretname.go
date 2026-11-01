package codegen

// THE NAME A ROUTINE READS A CREDENTIAL UNDER, COMPOSED IN ONE PLACE AND READ BACK IN ONE PLACE.
//
// # Why this file exists
//
// Because the identifier is the credential. `do OS's Secret with "sign-in:password"` and
// `do OS's Secret with "password"` are two different entries in somebody's credential store, and
// a routine that re-renders from one to the other looks identical in every review — same step
// list, same sentence shape, same word `Secret` — and signs in with nothing.
//
// That is not hypothetical. The declared set handed to a render came from the routine's NAME, and
// a SAVED routine's header does not carry the `with …` clause: the declaration lives in the steps,
// as `{{password}}` in the text of a step that types. So every re-render of a saved routine
// produced an UNQUALIFIED name, and the three surfaces that re-render — the step editor, the
// rebuild review and a move — each silently redirected the lookup. The step editor did not even
// get that far: it compares its own render against the file and refuses when they differ, so a
// routine that reads a password could not be edited at all.
//
// So both halves live here. `secretName` composes it and `typeSentence` is its only other caller;
// `SecretsIn` reads them back out of a generated source. A second composer or a second reader is
// a second thing to disagree with the first, and disagreeing here means looking up a secret
// nobody stored.
//
// NOTHING HERE TOUCHES A CREDENTIAL STORE. These are names, and a secret's value never enters a
// route's source at all — see the secrets invariant in CLAUDE.md.
//
// See [[ADR-287-a-re-render-keeps-the-credential-it-was-reading]].

import "regexp"

// secretName is the name a route reads a DECLARED secret argument under: route-qualified, so each
// route remembers its own. `base` is the route's file base — `fileBase(name)`.
func secretName(base, arg string) string { return base + ":" + arg }

// SecretNameFor is `secretName` for a caller outside this package, from the route's NAME.
//
// EXPORTED BECAUSE PRESERVING A LOOKUP ACROSS A RE-RENDER NEEDS THE SAME COMPOSITION. The
// orchestrator has to answer "does this source read `password` under the route-qualified name or
// the bare one", and the only honest way to ask is with the string this package would write.
func SecretNameFor(routeName, arg string) string { return secretName(fileBase(routeName), arg) }

// secretRe finds every credential name a generated source reads.
//
// The NAME, never a value: `OS's Secret` exists precisely so that a secret's value never enters a
// route's source. See `typeSentence`.
var secretRe = regexp.MustCompile(`OS's Secret with "([^"]*)"`)

// SecretsIn is every credential name a generated source reads, in order, de-duplicated.
//
// EXPORTED FOR THE SAME REASON `SecretNameFor` is. Comparing the names a routine read BEFORE an
// operation with the names it reads after is how a surface refuses to redirect a lookup it cannot
// preserve — and a reader written beside each of those surfaces would be three readers of one
// format.
func SecretsIn(src string) []string {
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
