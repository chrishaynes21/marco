package variables

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Capture and resolution.
//
//	The remembered object is evidence, never authority.
//
// Capture turns a RESOLVED target back into a QUERY — it deliberately throws the answer
// away and keeps the question. Resolution then hands that query to the ordinary
// resolver, which observes, ranks, and can come back ambiguous or absent exactly as it
// would for a freshly typed phrase.
//
// Nothing here resolves anything itself. That is the point: a variable that had its own
// lookup path would be a second definition of "which control did they mean", and the
// two would drift until one of them clicked the wrong thing.

// Capture builds a variable from a resolution the Director just performed.
//
// The resolved target is the INPUT, not the output. What is stored is the query that
// found it plus the semantic facts that make the query specific — label, role,
// application. The ElementID, the window handle and the click point are all discarded
// here, permanently and on purpose.
func Capture(name string, kind Kind, res directorapi.Resolution, world *directorapi.WorldState,
	phrase string) (Variable, error) {

	normalised, err := NormalizeName(name)
	if err != nil {
		return Variable{}, err
	}
	if res.Target == nil {
		return Variable{}, fmt.Errorf("variables: there is nothing resolved to remember as %q", normalised)
	}
	t := res.Target

	v := Variable{
		Name:  normalised,
		Kind:  kind,
		Label: t.Label,
		Role:  t.Role,
		Provenance: Provenance{
			CapturedAt:  now(),
			Phrase:      phrase,
			Explanation: firstNonEmpty(t.Explanation, res.Explanation),
			Confidence:  t.Confidence,
			Source:      "resolver",
		},
	}

	// The query that found it. Copied rather than referenced: the resolution belongs
	// to one command and will be reused and mutated by the next.
	if t.Query != nil {
		q := *t.Query
		// Window and ordinal are the two fields that are TRUE ONLY OF THAT INSTANT. A
		// window id is a handle, and an ordinal ("the second one") describes a
		// candidate list that will be different next time. Keeping either would make
		// this a coordinate in disguise.
		q.Window = nil
		q.Ordinal = 0
		v.Query = &q
	}
	if v.Query == nil || !v.Query.Constrained() {
		// Fall back to the semantic facts. A target with no query behind it can still
		// be remembered by what it IS.
		v.Query = &directorapi.ElementQuery{Label: t.Label, Role: t.Role}
	}

	if world != nil {
		if w, ok := world.FocusedWindow(); ok {
			v.Application, v.WindowTitle = w.Application, w.Title
			// Scope the query to the application, not the window. An application is
			// still the same application after a restart; a window is not.
			v.Query.Application = w.Application
		}
	}
	if !v.Resolvable() {
		return Variable{}, fmt.Errorf(
			"variables: %q could not be described well enough to find again", normalised)
	}
	return v, nil
}

// CaptureText remembers a value rather than a control.
func CaptureText(name, text, source, phrase string) (Variable, error) {
	normalised, err := NormalizeName(name)
	if err != nil {
		return Variable{}, err
	}
	return Variable{
		Name: normalised, Kind: KindText, Text: text,
		Provenance: Provenance{
			CapturedAt: now(), Phrase: phrase, Source: source, Confidence: 1,
		},
	}, nil
}

// CaptureWindow remembers a window semantically.
func CaptureWindow(name string, w directorapi.Window, phrase string) (Variable, error) {
	normalised, err := NormalizeName(name)
	if err != nil {
		return Variable{}, err
	}
	return Variable{
		Name: normalised, Kind: KindWindow,
		Application: w.Application, WindowTitle: w.Title,
		Provenance: Provenance{
			CapturedAt: now(), Phrase: phrase, Source: "window_system", Confidence: 1,
		},
	}, nil
}

// QueryFor returns the query a variable should be resolved with.
//
// This is the ONLY thing a variable contributes to a lookup. It goes to the ordinary
// resolver, against a world observed a moment ago, and comes back resolved, ambiguous,
// absent or unobservable exactly as any other query would — which is what keeps a
// remembered target no more trusted than a freshly described one.
func QueryFor(v Variable) (directorapi.ElementQuery, error) {
	switch v.Kind {
	case KindText:
		return directorapi.ElementQuery{}, fmt.Errorf(
			"%q is remembered text, not a control, so it cannot be clicked", v.Name)
	case KindWindow:
		return directorapi.ElementQuery{}, fmt.Errorf(
			"%q is remembered as a window, not a control", v.Name)
	}
	if v.Query == nil {
		return directorapi.ElementQuery{}, &ErrStale{
			Name:   v.Name,
			Detail: "it was stored without a way to describe what to look for.",
		}
	}
	return *v.Query, nil
}

// StaleFor explains a resolution that did not find the remembered object.
//
// The wording distinguishes the two cases a user needs to tell apart: the thing is not
// there at all, versus the Director could not see well enough to say. Reporting the
// second as the first would tell someone their button was gone when the application had
// merely not exposed its interior.
func StaleFor(v Variable, res directorapi.Resolution) error {
	detail := ""
	switch res.Status {
	case directorapi.ResolutionAbsent:
		detail = fmt.Sprintf("Nothing matching %s is in the observed window.", v.Describe())
	case directorapi.ResolutionUnobservable:
		detail = "The application could not be observed well enough to say, " +
			"which is not evidence that it is gone."
	default:
		detail = res.Explanation
	}
	return &ErrStale{Name: v.Name, Detail: detail}
}

// now is injectable so captures are deterministic under test.
var now = time.Now
