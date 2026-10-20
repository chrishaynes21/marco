package demonstrated

// THE PLAY, WRITTEN FROM THE DEMONSTRATION, WITH THE GATE IN IT.
//
// # The measurement this file's shape rests on
//
// A FLAT play is a play that lies. With the two interactions as consecutive sentences —
//
//	do Demonstrated's Press with "the graphics tab".
//	do Demonstrated's Press with "apply changes".
//
// — a failure of the first one does not stop the second. Measured: the host was asked for both,
// and the script logged `done`. An unhandled failure does not abort a body, so the second
// control is pressed after the first one failed and the run reports success.
//
// So the nesting is not a style. It is the gate, and it is the only gate:
//
//	do GraphicsTab's Press...
//	    when ok?
//	        do ApplyChanges's Press...
//	            ...
//	    or?
//	        this is that!
//
// Which is why this generator exists at all rather than a template in a doc comment: the one
// shape that is correct is the one a person would not naturally type, and a generated flat
// sequence would be indistinguishable from a working one until an action failed on somebody's
// real machine. `TestTheGeneratedPlayIsNested` holds it.

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Named is one interaction of a saved routine with the name a person gave it.
//
// The name is REQUIRED and it is the person's. A generated name would be the artifact's own
// `control_1` — a label scoped to a file, meaningless tomorrow, and exactly the perception
// internal that must not reach a play. Studio's recording flow is where the name comes from.
type Named struct {
	Position int
	Name     string
}

// Compose writes the play and the bindings for one saved routine, together.
//
// Together on purpose: the digest in the record is of the source this returns, so the two cannot
// be written from different inputs. A caller that generated source and then built a record from
// somewhere else would be able to produce a pair that passes preflight and performs something
// else.
func Compose(a workflowstore.Artifact, act string, picked []Named, at time.Time,
	check Plays) (string, Bindings, error) {
	if strings.TrimSpace(act) == "" {
		return "", Bindings{}, fmt.Errorf("an Act needs a name of its own; it is what " +
			"somebody will run it by")
	}
	if len(picked) == 0 {
		return "", Bindings{}, fmt.Errorf("a play needs at least one interaction")
	}
	rev, err := RevisionOf(a)
	if err != nil {
		return "", Bindings{}, err
	}
	scenes := make([]Binding, 0, len(picked))
	seen := map[string]bool{}
	locals := make([]string, 0, len(picked))
	for _, p := range picked {
		if strings.TrimSpace(p.Name) == "" {
			return "", Bindings{}, fmt.Errorf("interaction %d has no name; a play refers "+
				"to an interaction by the name a person gave it", p.Position)
		}
		if p.Position < 0 || p.Position >= len(a.Steps) {
			return "", Bindings{}, fmt.Errorf("%q names interaction %d and %q has %d",
				p.Name, p.Position, a.Name, len(a.Steps))
		}
		if seen[p.Name] {
			return "", Bindings{}, fmt.Errorf("two interactions are both called %q; "+
				"a name in a play has to mean one thing", p.Name)
		}
		seen[p.Name] = true
		scenes = append(scenes, Binding{Name: p.Name, Routine: a.Name,
			Revision: rev, Position: p.Position})
		locals = append(locals, sceneName(p.Name))
	}
	src := write(a, act, picked, locals)
	// AND IT COMPILES, HERE, BEFORE IT IS OFFERED TO ANYBODY. The production checker, the
	// same resolver a run uses — see [[ADR-005-legal-marco-only]] and the two-lists-one-fact
	// defect that came of asking a different question from the one the runtime asks.
	if check == nil {
		return "", Bindings{}, fmt.Errorf("a play is not saved without being checked")
	}
	if err := check.Check(src); err != nil {
		return "", Bindings{}, fmt.Errorf("the play this would save does not compile: %w", err)
	}
	return src, Bindings{
		Version: BindingsVersion, Act: act, Source: routes.DigestOf(src),
		Application: string(a.Application), Scenes: scenes, SavedAt: at.UTC(),
	}, nil
}

// write renders the play. One interaction per scene, and the gate between them is the nesting.
func write(a workflowstore.Artifact, act string, picked []Named, locals []string) string {
	var b strings.Builder
	// THE ACT'S OWN NAME IN ITS OWN FILE, with the demonstration named as where the
	// interactions came from.
	//
	// The first version used the DEMONSTRATION's name throughout, so an Act called "ark
	// audio by name" opened with a comment about "ark graphics live" and ended by logging
	// that "ark graphics live" had verified. Both true, and neither about the file
	// somebody is reading. A generated program that reads like a data dump has failed
	// even if it runs, and this one was failing at its first line.
	fmt.Fprintf(&b, "// %s — an Act: the interactions below, in this order.\n", act)
	fmt.Fprintf(&b, "//\n// Composed from what somebody demonstrated as %q.\n", a.Name)
	b.WriteString("//\n")
	b.WriteString("// Each interaction is asked for by the name the person gave it. Every one " +
		"is found again in\n")
	b.WriteString("// a fresh look, agreed to at the moment it happens, and checked " +
		"afterwards — and the next\n")
	b.WriteString("// one is inside the last one's `when ok?`, so nothing follows an " +
		"action that did not work.\n")
	b.WriteString("use demonstrated.\n\n")

	for i, p := range picked {
		fmt.Fprintf(&b, "the %s is a scene.\n", locals[i])
		b.WriteString("this can Press.\n")
		b.WriteString("this's Press does...\n")
		fmt.Fprintf(&b, "    do Demonstrated's Press with %s.\n", quote(p.Name))
		b.WriteString("    this is that!\n\n")
	}

	actor := sceneName(act)
	fmt.Fprintf(&b, "the %s is an actor.\n", actor)
	b.WriteString("this can Run.\n")
	b.WriteString("this's Run does...\n")
	b.WriteString(nest(locals, 1))
	b.WriteString("\nthe App is a script.\n\n")
	fmt.Fprintf(&b, "do %s's Run...\n", actor)
	b.WriteString("    when ok?\n")
	// THE ACT'S NAME HERE TOO, which is the line a person actually sees when it works.
	//
	// It was the demonstration's, so an Act called "ark audio by name" ended by logging
	// that "ark graphics live" had verified. I reported this fixed when the header and the
	// actor were — a `sed` whose `\n` matched a newline rather than the two characters in
	// the Go string had silently left this one line behind, and the Architect found it.
	fmt.Fprintf(&b, "        log %s.\n", quote(act+": every action verified"))
	b.WriteString("    or?\n")
	b.WriteString("        log that's error.\n")
	return b.String()
}

// nest writes the interactions as a chain, each inside the previous one's success.
//
// The innermost `when ok?` is the only place the whole run succeeds, and every `or?` adopts the
// outcome verbatim with `this is that!` — so the sentence a person is shown at the end is the one
// the Director wrote about the action that stopped it, not a summary this generator invented.
func nest(locals []string, depth int) string {
	pad := strings.Repeat("    ", depth)
	if len(locals) == 0 {
		return pad + "this is ok!\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%sdo %s's Press...\n", pad, locals[0])
	fmt.Fprintf(&b, "%s    when ok?\n", pad)
	b.WriteString(nest(locals[1:], depth+2))
	fmt.Fprintf(&b, "%s    or?\n", pad)
	fmt.Fprintf(&b, "%s        this is that!\n", pad)
	return b.String()
}

// sceneName turns a person's words into a Marco declaration name: capitalised, letters only.
//
// A LEADING ARTICLE IS DROPPED, because `the TheGraphicsTab is a scene.` is what keeping it
// produces, and a generated play that stutters is one a person stops reading. Only the
// declaration name is affected: the sentence that performs the interaction carries the person's
// own words, exactly as they typed them.
func sceneName(s string) string {
	for _, a := range []string{"the ", "The ", "a ", "A ", "an ", "An "} {
		if strings.HasPrefix(s, a) {
			s = s[len(a):]
			break
		}
	}
	var b strings.Builder
	up := true
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if up {
				b.WriteRune(unicode.ToUpper(r))
				up = false
			} else {
				b.WriteRune(r)
			}
		default:
			up = true
		}
	}
	out := b.String()
	if out == "" || unicode.IsDigit(rune(out[0])) {
		// A declaration has to start with a letter, and a name of only digits or
		// punctuation is not one a person typed on purpose.
		out = "Interaction" + out
	}
	return out
}

// quote renders a Marco text literal. Through the encoder the language already has, because the
// escape rules belong to it and an interpolated quote would change what a play says.
func quote(s string) string {
	q, err := marcoexec.Quote(s)
	if err != nil {
		// Unquotable text cannot reach here: Compose refuses an empty name, and the
		// encoder's other refusals are about control characters a person cannot type
		// into a name field. Falling back to an escaped form rather than panicking,
		// because a play that cannot be written is a saved routine somebody loses.
		return `"` + strings.NewReplacer(`\`, `\`, `"`, `\"`).Replace(s) + `"`
	}
	return q
}
