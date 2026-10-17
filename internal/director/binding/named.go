package binding

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// BINDING AN OBJECT THE PERSON NAMED, rather than one they pointed at.
//
// # What was measured
//
// A live Explorer window on a disposable folder:
//
//	rename this file to summary.txt    →  COMPLETED, summary.txt on disk
//	rename notes.txt to summary.txt    →  FAILED: step 3 of 4 (set the name to
//	                                      "summary.txt") did not complete: this step acts
//	                                      on the editor for an object nothing bound, so
//	                                      there is nothing to look for an editor of
//
// The same rename, the same file, and the version that NAMES it cannot run. The procedure's
// third step acts on the editor for a BOUND object — which is the whole reason it works at all,
// because a details view has an Edit control per column per row and typing into the wrong one
// changes a caption and renames nothing. Pointing bound the file. Naming it did not, because
// `Resolve` is entirely deictic: it reads focus and selection and never looks at the phrase
// except to quote it in a refusal.
//
// # Why naming is not weaker evidence than pointing
//
// It is stronger. "This file" means whatever the person was looking at, and the whole deictic
// path exists to pin that down before it moves. "notes.txt" is a name they chose, and the only
// question is which observed object bears it.
//
// So the clauses are the same shape and the same vocabulary: the kind must satisfy what the
// procedure declared, exactly one object must match, several is an ambiguity the person can
// settle, and none is a refusal. What changes is how the primary object is found.
//
// # And one thing a named binding must NOT inherit
//
// Revalidation's third rule — found, but no longer focused or selected, so the person moved on
// and acting would rename whatever they happened to click. That rule is right about pointing
// and wrong about a name: nobody's attention is what identified "notes.txt". A named binding is
// re-checked by IDENTITY — its backing path, then the source's own id — and the requirement
// that it still be the current object is dropped for it and kept for everything else. See
// Binding.ByName.
//
// Enforced by TestANamedObjectBindsByItsName and
// TestANamedBindingSurvivesTheSelectionMovingAway.

// ResolveNamed binds the one observed object called `name`, of the expected kind.
func (r *Resolver) ResolveNamed(name string, expected ObjectKind,
	w *directorapi.WorldState) (*Binding, *Problem) {

	if !expected.Known() {
		return nil, &Problem{
			Reason: ReasonUnknownKind,
			Message: fmt.Sprintf("%q asks for %q, which is not a kind of object this "+
				"Director knows how to check", name, expected),
		}
	}
	wanted := strings.TrimSpace(strings.ToLower(name))
	if wanted == "" {
		return nil, &Problem{
			Reason:  ReasonNoFocus,
			Message: "no name was given, so there is nothing to look for",
		}
	}
	if w == nil {
		return nil, &Problem{
			Reason:  ReasonNoFocus,
			Message: fmt.Sprintf("nothing has been observed, so %q names nothing", name),
		}
	}

	// CONFIRMED AND UNCONFIRMED CANDIDATES, kept apart.
	//
	// A kind comes from a backing PATH, and a file manager reports the path for its
	// SELECTION. So an unselected item called "notes.txt" cannot be classified as a file
	// at all — which is a chicken and egg, because selecting it is step one of the
	// procedure that wants it.
	//
	// The resolution is an ordering rather than a relaxation: identify the object by its
	// NAME now, and confirm its KIND before anything is changed. An item whose kind is
	// INDEFINITE — no path, so nothing has said what it stands for — is eligible and
	// carries KindUnconfirmed. An item whose kind is definite and wrong is not: a details
	// view has a text_field cell bearing the same name, and a text field is a text
	// selection, which is positively not a file.
	var confirmed, unconfirmed []*directorapi.Element
	for _, el := range w.Elements {
		if !el.Visible || strings.TrimSpace(strings.ToLower(el.Label)) != wanted {
			continue
		}
		switch kind, _ := classify(el); {
		case kind.Satisfies(expected):
			confirmed = append(confirmed, el)
		case kind == KindControl:
			unconfirmed = append(unconfirmed, el)
		}
	}
	matches, pending := confirmed, false
	if len(matches) == 0 {
		matches, pending = unconfirmed, true
	}
	switch len(matches) {
	case 0:
		return nil, &Problem{
			Reason: ReasonGone,
			Message: fmt.Sprintf("nothing called %q of the right kind is on screen, so "+
				"%s could not be found", name, expected.Describe()),
		}
	case 1:
	default:
		// Several things with one name is an ambiguity the person can settle, exactly as
		// several selected objects is. Not a preference between them.
		return nil, &Problem{
			Reason:     ReasonAmbiguous,
			Message:    fmt.Sprintf("%d things on screen are called %q", len(matches), name),
			Candidates: candidatesOf(matches),
		}
	}

	primary := matches[0]
	kind, resource := classify(primary)
	b := r.bindingFor(name, expected, kind, primary, "name", resource, w)
	// BY NAME, recorded on the binding itself, because revalidation has to treat it
	// differently and must not have to guess from the evidence list.
	b.ByName = true
	b.KindUnconfirmed = pending
	return b, nil
}
