package binding

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Resolving and revalidating a deictic reference.

// Resolver turns "this" into a Binding against an observed world.
//
// Injectable clock and sequence so a test produces a deterministic binding; nothing here
// observes on its own, because the world it must bind against is the one the caller
// already has.
type Resolver struct {
	// Now supplies the resolution timestamp. Defaults to time.Now.
	Now func() time.Time
	// seq is the monotonic counter behind Binding.Sequence.
	seq int64
}

// NewResolver returns a resolver with the default clock.
func NewResolver() *Resolver { return &Resolver{Now: time.Now} }

func (r *Resolver) now() time.Time {
	if r == nil || r.Now == nil {
		return time.Now()
	}
	return r.Now()
}

func (r *Resolver) next() int64 {
	r.seq++
	return r.seq
}

// Resolve binds a deictic phrase to a concrete object of the expected kind.
//
// The order of the checks is the safety argument:
//
//  1. An unknown expected kind is refused before anything is inspected — a procedure
//     asking for something this build cannot check must not be answered by pretending.
//  2. Nothing focused means "this" names nothing. Clarification, not a guess.
//  3. The focused object is classified, and a WRONG KIND is refused outright. This is
//     the case the whole package exists for: a folder, an unsaved tab, a selection or a
//     bare control is not what "this file" meant, and acting on it renames the wrong
//     thing.
//  4. A file-backed kind with no backing resource is refused: without a path there is
//     nothing to revalidate against later, and a tab label alone is not identity.
//  5. Several equally plausible objects is an ambiguity, which the user can settle.
func (r *Resolver) Resolve(phrase string, expected ObjectKind, w *directorapi.WorldState) (
	*Binding, *Problem) {

	if !expected.Known() {
		return nil, &Problem{
			Reason: ReasonUnknownKind,
			Message: fmt.Sprintf("%q asks for %q, which is not a kind of object this "+
				"Director knows how to check", phrase, expected),
		}
	}
	if w == nil {
		return nil, &Problem{
			Reason:  ReasonNoFocus,
			Message: fmt.Sprintf("nothing has been observed, so %q names nothing", phrase),
		}
	}

	focused := focusedElement(w)
	selected := selectedElements(w)

	// The focused object is what "this" means first: a user looking at a focused item
	// means that one even when something else is still highlighted elsewhere.
	primary := focused
	via := "focus"

	// Unless what is focused is not the KIND being asked for and exactly one selected
	// object is. In a file manager the list holds the selection while the window, the
	// address bar or a toolbar button holds keyboard focus — so "rename this file" with
	// Alpha.txt highlighted means Alpha.txt, and reading it as the toolbar refuses a
	// request that was perfectly clear.
	//
	// Found live: the Director selected a file, verified the selection, and then refused
	// to rename it because focus sat on the folder's own container.
	//
	// EXACTLY ONE, and only of the right kind. Several selected files is still an
	// ambiguity the user can settle, and nothing selected is still a refusal — this
	// widens which object can be chosen, never how confidently.
	if kind, _ := classify(primary); primary == nil || !kind.Satisfies(expected) {
		if plausible := sameKind(selected, expected); len(plausible) == 1 {
			primary, via = plausible[0], "selection"
		} else if len(plausible) > 1 {
			return nil, &Problem{
				Reason: ReasonAmbiguous,
				Message: fmt.Sprintf("%q could mean any of %d selected items",
					phrase, len(plausible)),
				Candidates: candidatesOf(plausible),
			}
		}
	}
	if primary == nil && len(selected) == 1 {
		primary, via = selected[0], "selection"
	}
	if primary == nil {
		if len(selected) > 1 {
			return nil, &Problem{
				Reason:     ReasonAmbiguous,
				Message:    fmt.Sprintf("%q could mean any of %d selected items", phrase, len(selected)),
				Candidates: candidatesOf(selected),
			}
		}
		return nil, &Problem{
			Reason: ReasonNoFocus,
			Message: fmt.Sprintf("nothing is focused or selected, so %q names nothing. "+
				"Select the item first, or name it.", phrase),
		}
	}

	kind, resource := classify(primary)
	if !kind.Satisfies(expected) {
		return nil, &Problem{
			Reason: ReasonWrongKind,
			Message: fmt.Sprintf(
				"%q needs %s, and what is focused is %s (%q). Refusing rather than acting "+
					"on the wrong object.",
				phrase, expected.Describe(), kind.Describe(), primary.Label),
			Candidates: candidatesOf([]*directorapi.Element{primary}),
		}
	}
	if expected.FileBacked() && resource == "" {
		return nil, &Problem{
			Reason: ReasonNoResource,
			Message: fmt.Sprintf(
				"%q needs %s, and %q has no file behind it that this Director can see. "+
					"A label alone is not enough to be sure which object to act on.",
				phrase, expected.Describe(), primary.Label),
		}
	}

	// More than one focused-or-selected object of the RIGHT kind is a genuine ambiguity.
	if plausible := sameKind(selected, expected); len(plausible) > 1 && via == "selection" {
		return nil, &Problem{
			Reason:     ReasonAmbiguous,
			Message:    fmt.Sprintf("%q could mean any of %d selected items", phrase, len(plausible)),
			Candidates: candidatesOf(plausible),
		}
	}

	b := &Binding{
		Phrase: phrase, Expected: expected, Resolved: kind,
		ElementID: string(primary.ID), NativeID: nativeIDOf(primary),
		WindowID: string(primary.WindowID), WindowTitle: windowTitle(w, primary.WindowID),
		Application: applicationOf(w, primary.WindowID),
		Label:       primary.Label, Resource: resource,
		Stability:  StabilityToken(w),
		Sequence:   r.next(),
		ResolvedAt: r.now(),
		Confidence: primary.Confidence,
	}
	b.Evidence = append(b.Evidence, Evidence{
		Kind: via, Detail: fmt.Sprintf("%q holds %s", primary.Label, via),
		Source: directorapi.SourceAccessibility,
	})
	if resource != "" {
		// The decisive one. A path is identity; everything else is corroboration.
		b.Evidence = append(b.Evidence, Evidence{
			Kind: "backing_path", Detail: resource,
			Source: directorapi.SourceAccessibility, Decisive: true,
		})
	}
	// How the source established it, when it said. Carried through so a person reading a
	// binding can see that the path came from the shell's account of its own selection
	// rather than from a caption and a folder — which is the distinction the whole
	// resource-identity path exists to preserve.
	if r := primary.Resource; r.Known() {
		b.Identity = r.Clone()
		b.Evidence = append(b.Evidence, Evidence{
			Kind: "resource_" + r.Source, Detail: r.Describe(),
			Source: directorapi.SourceAccessibility, Decisive: true,
		})
		for _, why := range r.Evidence {
			b.Evidence = append(b.Evidence, Evidence{
				Kind: "shell", Detail: why, Source: directorapi.SourceAccessibility,
			})
		}
	}
	if b.NativeID != "" {
		b.Evidence = append(b.Evidence, Evidence{
			Kind: "native_id", Detail: b.NativeID, Source: directorapi.SourceAccessibility,
		})
	}
	b.Candidates = candidatesOf(sameKind(selected, expected))
	return b, nil
}

// Revalidation is what a re-check concluded.
type Revalidation struct {
	// OK reports whether the action may proceed.
	OK bool `json:"ok"`
	// Binding is the binding to use — the original, or a refreshed one.
	Binding *Binding `json:"binding,omitempty"`
	// Refreshed reports that the same object was re-established after a harmless
	// change, and Changes says what moved.
	Refreshed bool     `json:"refreshed,omitempty"`
	Changes   []string `json:"changes,omitempty"`
	// Problem is why the action must not proceed.
	Problem *Problem `json:"problem,omitempty"`
}

// Revalidate re-checks a binding against a freshly observed world.
//
//	A binding created during expansion or planning must not be trusted indefinitely.
//
// The algorithm:
//
//  1. If the stability token is unchanged, the world has not moved and the binding
//     stands — no re-identification needed.
//  2. Otherwise look for the SAME object, by its decisive identity: the backing
//     resource first, then the source's native id. Found, still the right kind, and
//     still the focused-or-selected one → REFRESH: the binding is renewed with the new
//     element identity and the change recorded.
//  3. Found but no longer focused or selected → the user moved on. Refused, never
//     silently re-bound to whatever is focused now.
//  4. Not found at all → gone.
//
// The third case is the one that matters most. Binding to the newly focused object is
// exactly what a careless implementation does, and it is how "rename this file" renames
// whatever the user happened to click while the Director was thinking.
func (r *Resolver) Revalidate(b *Binding, w *directorapi.WorldState) Revalidation {
	if !b.Bound() {
		return Revalidation{OK: false, Problem: &Problem{
			Reason: ReasonNoFocus, Message: "there is no binding to revalidate",
		}}
	}
	if w == nil {
		return Revalidation{OK: false, Problem: &Problem{
			Reason:  ReasonGone,
			Message: "nothing could be observed, so the binding could not be re-checked",
		}}
	}

	if token := StabilityToken(w); token == b.Stability {
		// The world is the one the binding was made against. Nothing to re-establish.
		return Revalidation{OK: true, Binding: b}
	}

	match, changes := r.reidentify(b, w)
	if match == nil {
		return Revalidation{OK: false, Problem: &Problem{
			Reason: ReasonGone,
			Message: fmt.Sprintf("%s is no longer there, so the action was not performed",
				b.Describe()),
		}}
	}

	kind, resource := classify(match)
	if !kind.Satisfies(b.Expected) {
		return Revalidation{OK: false, Problem: &Problem{
			Reason: ReasonWrongKind,
			Message: fmt.Sprintf("%s is now %s rather than %s, so the action was not performed",
				b.Describe(), kind.Describe(), b.Expected.Describe()),
		}}
	}

	// Still the object the user was pointing at?
	if !isCurrent(match, w) {
		return Revalidation{OK: false, Problem: &Problem{
			Reason: ReasonChanged,
			Message: fmt.Sprintf(
				"the focus moved away from %s before the action ran. Refusing rather than "+
					"acting on whatever is focused now.", b.Describe()),
			Candidates: candidatesOf(currentObjects(w)),
		}}
	}

	refreshed := *b
	refreshed.ElementID = string(match.ID)
	refreshed.NativeID = nativeIDOf(match)
	refreshed.WindowID = string(match.WindowID)
	refreshed.WindowTitle = windowTitle(w, match.WindowID)
	refreshed.Label = match.Label
	if resource != "" {
		refreshed.Resource = resource
	}
	refreshed.Stability = StabilityToken(w)
	refreshed.Sequence = r.next()
	refreshed.ResolvedAt = r.now()
	refreshed.Refreshed = append(append([]string{}, b.Refreshed...), changes...)

	return Revalidation{OK: true, Binding: &refreshed, Refreshed: true, Changes: changes}
}

// reidentify finds the bound object in a new world, by identity rather than by position.
func (r *Resolver) reidentify(b *Binding, w *directorapi.WorldState) (*directorapi.Element, []string) {
	var byResource, byNative, byLabel *directorapi.Element
	for _, el := range w.Elements {
		if b.Resource != "" && strings.EqualFold(resourcePath(el), b.Resource) {
			byResource = el
			break
		}
		if b.NativeID != "" && nativeIDOf(el) == b.NativeID {
			byNative = el
		}
		if byLabel == nil && b.Label != "" && el.Label == b.Label {
			byLabel = el
		}
	}

	switch {
	case byResource != nil:
		return byResource, changesBetween(b, byResource, "the same file, re-observed")
	case byNative != nil:
		// The element id survived — but if BOTH sides name a backing resource and the
		// resources differ, this is a different object wearing a reused id. A view that
		// navigated to another folder does exactly that: the list item at the same
		// position keeps its runtime id and now stands for a different file.
		//
		// Refused rather than accepted, which is the whole reason a path outranks an id.
		if b.Resource != "" {
			if got := resourcePath(byNative); got != "" && !strings.EqualFold(got, b.Resource) {
				return nil, nil
			}
		}
		return byNative, changesBetween(b, byNative, "the same element id, re-observed")
	}
	// A label alone is NOT identity. Two files can share a name in different folders,
	// and matching on it would re-bind to a different object with the same caption.
	return nil, nil
}

// changesBetween describes what moved between a binding and its re-observation.
func changesBetween(b *Binding, el *directorapi.Element, how string) []string {
	changes := []string{how}
	if string(el.ID) != b.ElementID {
		changes = append(changes, "the element id changed (the tree was rebuilt)")
	}
	if string(el.WindowID) != b.WindowID {
		changes = append(changes, "the window changed")
	}
	return changes
}

// isCurrent reports whether an element is the focused or selected one.
func isCurrent(el *directorapi.Element, w *directorapi.WorldState) bool {
	if el.Focused {
		return true
	}
	if el.Selected {
		// Selected AND nothing else focused: the user's pointer is still on it.
		return focusedElement(w) == nil || focusedElement(w).ID == el.ID
	}
	return false
}

// StabilityToken summarises the world for change detection.
//
// Built from the identity and state of the objects a deictic reference can name — which
// is exactly the question being asked ("could 'this' now mean something else?") and
// nothing more. A token over the whole element set would change whenever a clock ticked
// somewhere, forcing a re-identification on every action for no reason.
//
// A world that cannot be summarised falls back to its timestamp, which always looks
// changed. That errs toward re-checking, which is the safe direction.
func StabilityToken(w *directorapi.WorldState) string {
	if w == nil {
		return ""
	}
	current := currentObjects(w)
	if len(current) == 0 {
		return "none@" + w.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	var b strings.Builder
	for _, el := range current {
		kind, resource := classify(el)
		fmt.Fprintf(&b, "%s|%s|%s|%s|%v;", el.ID, kind, resource, el.WindowID, el.Focused)
	}
	return b.String()
}

// ── world helpers ─────────────────────────────────────────────────────────────

func focusedElement(w *directorapi.WorldState) *directorapi.Element {
	for _, el := range w.Elements {
		if el.Focused {
			return el
		}
	}
	return nil
}

func selectedElements(w *directorapi.WorldState) []*directorapi.Element {
	var out []*directorapi.Element
	for _, el := range w.Elements {
		if el.Selected {
			out = append(out, el)
		}
	}
	// Stable order, so an ambiguity lists its candidates the same way every run.
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// currentObjects is everything that could be "this" right now.
func currentObjects(w *directorapi.WorldState) []*directorapi.Element {
	out := selectedElements(w)
	if f := focusedElement(w); f != nil {
		out = append([]*directorapi.Element{f}, out...)
	}
	return out
}

func sameKind(els []*directorapi.Element, want ObjectKind) []*directorapi.Element {
	var out []*directorapi.Element
	for _, el := range els {
		if kind, _ := classify(el); kind.Satisfies(want) {
			out = append(out, el)
		}
	}
	return out
}

func candidatesOf(els []*directorapi.Element) []Candidate {
	var out []Candidate
	for _, el := range els {
		kind, resource := classify(el)
		out = append(out, Candidate{
			ElementID: string(el.ID), Kind: kind, Label: el.Label, Resource: resource,
			Why: fmt.Sprintf("%s, %s", kind.Describe(), stateWord(el)),
		})
	}
	return out
}

func stateWord(el *directorapi.Element) string {
	switch {
	case el.Focused:
		return "focused"
	case el.Selected:
		return "selected"
	}
	return "present"
}

func nativeIDOf(el *directorapi.Element) string {
	if v, ok := el.Attributes["native_id"].(string); ok {
		return v
	}
	return ""
}

func windowTitle(w *directorapi.WorldState, id directorapi.WindowID) string {
	for i := range w.Windows {
		if w.Windows[i].ID == id {
			return w.Windows[i].Title
		}
	}
	return ""
}

func applicationOf(w *directorapi.WorldState, id directorapi.WindowID) string {
	for i := range w.Windows {
		if w.Windows[i].ID == id {
			return w.Windows[i].Application
		}
	}
	return ""
}
