// Package binding turns a deictic reference — "this file", "that folder" — into a
// concrete, checkable identity BEFORE anything acts on it.
//
// The governing rules:
//
//	A deictic target must resolve to a concrete focused object of the expected kind.
//	Missing evidence requires clarification or refusal, never guessing.
//	A binding created during planning must not be trusted indefinitely.
//
// The gap this closes is the most dangerous one left in the goal layer. "Rename this
// file to Budget" resolved to whatever held keyboard focus, with no check that the thing
// focused was a FILE. A folder, an unsaved editor tab, a text selection or an unrelated
// button all satisfied that, and each of them turns a rename into something the user did
// not ask for — renaming a folder full of work, or typing a filename into a document.
//
// So a binding is created explicitly, records what it resolved and on what evidence, and
// is REVALIDATED immediately before the first action that uses it. The revalidation is
// not a formality: focus moves between planning and acting for entirely ordinary reasons
// — the user clicks something, a notification steals focus — and binding to whatever is
// focused NOW would act on a different object than the one the user was looking at when
// they spoke.
//
// What a binding is NOT: it is diagnostic and safety metadata, and it never reduces a
// semantic target to coordinates. The action still carries its semantic reference; the
// binding is what proves that reference meant something specific and still does.
package binding

import (
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ObjectKind is what sort of thing a deictic reference names.
//
// A CLOSED vocabulary, and typed rather than a free string at execution boundaries: a
// procedure that could declare it wanted a "file-ish thing" would be asking a question
// nothing could check, and the whole point of this package is that the check is real.
type ObjectKind string

const (
	// KindFile is a file-backed object: it has a path on disk.
	KindFile ObjectKind = "file"
	// KindFolder is a directory. Deliberately distinct from KindFile — "rename this
	// file" against a folder is the mistake that motivated this package.
	KindFolder ObjectKind = "folder"
	// KindDocument is an open document that has a backing file.
	KindDocument ObjectKind = "document"
	// KindEditorBuffer is an open buffer with NO backing file — an unsaved tab. A
	// legitimate object, and never a valid answer to "this file".
	KindEditorBuffer ObjectKind = "editor_buffer"
	// KindSymbol is a code symbol, for a rename-symbol refactor.
	KindSymbol ObjectKind = "symbol"
	// KindControl is an ordinary UI control.
	KindControl ObjectKind = "control"
	// KindWindow is a window.
	KindWindow ObjectKind = "window"
	// KindTextSelection is selected text. Never a valid answer to "this file".
	KindTextSelection ObjectKind = "text_selection"
)

// Vocabulary is every kind, in a stable order.
var Vocabulary = []ObjectKind{
	KindFile, KindFolder, KindDocument, KindEditorBuffer,
	KindSymbol, KindControl, KindWindow, KindTextSelection,
}

// Known reports whether a kind is in the vocabulary.
func (k ObjectKind) Known() bool {
	for _, v := range Vocabulary {
		if v == k {
			return true
		}
	}
	return false
}

// Describe renders a kind for a person.
func (k ObjectKind) Describe() string {
	switch k {
	case KindFile:
		return "a file"
	case KindFolder:
		return "a folder"
	case KindDocument:
		return "a document with a file behind it"
	case KindEditorBuffer:
		return "an unsaved editor tab"
	case KindSymbol:
		return "a code symbol"
	case KindControl:
		return "a control"
	case KindWindow:
		return "a window"
	case KindTextSelection:
		return "a text selection"
	}
	return string(k)
}

// FileBacked reports whether this kind has a resource on disk.
//
// The distinction "this file" turns on: a document is file-backed and an editor buffer
// is not, even though both are open text in the same application.
func (k ObjectKind) FileBacked() bool { return k == KindFile || k == KindDocument }

// Satisfies reports whether an object of this kind answers a request for `want`.
//
// Deliberately NOT a subtype lattice. A document satisfies "this file" because it has a
// path; nothing else does. A folder does not, an unsaved buffer does not, a selection
// does not, and a bare control does not — each of those is the specific mistake the
// milestone enumerates.
func (k ObjectKind) Satisfies(want ObjectKind) bool {
	if k == want {
		return true
	}
	if want == KindFile {
		return k == KindDocument
	}
	return false
}

// Evidence is one fact that supported a binding.
//
// Recorded rather than collapsed into a confidence number, because "the tab said
// Budget.txt" and "the application reported the path C:\tmp\Budget.txt" are different
// grounds for the same conclusion, and only the second is enough to act on.
type Evidence struct {
	// Kind names the sort of fact: "native_id", "backing_path", "role", "label",
	// "automation_id", "focus", "selection".
	Kind string `json:"kind"`
	// Detail is the fact itself, rendered for a person.
	Detail string `json:"detail"`
	// Source is where it came from.
	Source directorapi.ObservationSource `json:"source,omitempty"`
	// Decisive marks evidence that was sufficient on its own. A binding with no
	// decisive evidence is a guess, however much corroboration it has.
	Decisive bool `json:"decisive,omitempty"`
}

// Candidate is one object that could have been what "this" meant.
type Candidate struct {
	ElementID string     `json:"element_id,omitempty"`
	Kind      ObjectKind `json:"kind"`
	Label     string     `json:"label,omitempty"`
	Resource  string     `json:"resource,omitempty"`
	Why       string     `json:"why,omitempty"`
}

// Binding is a resolved deictic reference.
//
// Every field is identity or evidence. There are no coordinates here and there must not
// be: a binding proves WHICH object was meant, and a point on the screen is neither
// necessary nor sufficient for that.
type Binding struct {
	// ID is the request-scoped handle an action refers to this binding by. Assigned by
	// Store.Put; empty on a binding that has not been filed.
	ID ID `json:"id,omitempty"`
	// Phrase is what the user actually said ("this file").
	Phrase string `json:"phrase"`
	// Expected is the kind the procedure or semantic action requires.
	Expected ObjectKind `json:"expected_kind"`
	// Resolved is the kind of the object that was found.
	Resolved ObjectKind `json:"resolved_kind"`

	// ElementID is the world's identity for the object; NativeID is the accessibility
	// source's own name for it, which is what survives a re-observation.
	ElementID string `json:"element_id,omitempty"`
	NativeID  string `json:"native_id,omitempty"`
	// Application and Window locate it.
	Application string `json:"application,omitempty"`
	WindowID    string `json:"window_id,omitempty"`
	WindowTitle string `json:"window_title,omitempty"`
	// Label is what it is called on screen.
	Label string `json:"label,omitempty"`
	// Resource is the backing resource — a file path — where the object has one. The
	// field that makes "this file" checkable rather than hopeful.
	Resource string `json:"resource,omitempty"`

	// Evidence is why this was chosen.
	Evidence []Evidence `json:"evidence,omitempty"`
	// Candidates are the other plausible objects, kept so an ambiguity can be shown.
	Candidates []Candidate `json:"candidates,omitempty"`

	// Stability is the token that says whether the world has moved under the binding.
	// Compared on revalidation; a different token means re-check, not re-bind.
	Stability string `json:"stability_token,omitempty"`
	// Sequence is a monotonic counter of when this was resolved, for ordering without
	// depending on a clock.
	Sequence int64 `json:"sequence"`
	// ResolvedAt is the wall-clock time, for a person reading a trace.
	ResolvedAt time.Time `json:"resolved_at"`
	// Confidence is how sure the resolution was, 0..1.
	Confidence float64 `json:"confidence"`

	// Refreshed records that revalidation re-established the SAME object after a
	// harmless change, and what changed.
	Refreshed []string `json:"refreshed,omitempty"`

	// Origin is where the deictic phrase came from — the goal, the procedure and the
	// step. Diagnostic provenance: nothing reads it back to decide anything, and a
	// binding made outside a goal simply has none.
	Origin Origin `json:"origin,omitempty"`

	// Identity is the source.s own account of the object, when one established it: the
	// canonical path, how it was obtained, and the evidence. Absent for a binding made
	// from a path the classifier inferred from an attribute rather than from a source
	// that positively knew.
	//
	// Carried so a reader can tell those two apart. Both are paths; only one came from
	// something that could say why.
	Identity *directorapi.ResourceIdentity `json:"identity,omitempty"`
}

// Origin is the provenance of a binding request.
//
//	Construct a binding request containing … procedure and step provenance.
//
// Carried on the binding so a verification mismatch can name the goal, procedure, step
// and action it came from without a second lookup — which is the one moment a person most
// needs to know which of five steps bound the wrong thing.
type Origin struct {
	Goal      string `json:"goal,omitempty"`
	Procedure string `json:"procedure,omitempty"`
	StepID    string `json:"step_id,omitempty"`
	StepIndex int    `json:"step_index,omitempty"`
	Request   string `json:"request,omitempty"`
}

// Empty reports whether there is no provenance to show.
func (o Origin) Empty() bool { return o.Goal == "" && o.Procedure == "" && o.Request == "" }

// Bound reports whether this is a real binding rather than a zero value.
func (b *Binding) Bound() bool { return b != nil && b.Resolved != "" }

// Describe renders a binding in one line.
func (b *Binding) Describe() string {
	if !b.Bound() {
		return "(unbound)"
	}
	out := fmt.Sprintf("%q → %s", b.Phrase, b.Resolved.Describe())
	if b.Resource != "" {
		out += " " + b.Resource
	} else if b.Label != "" {
		out += " " + fmt.Sprintf("%q", b.Label)
	}
	return out
}

// Decisive reports whether any evidence was sufficient on its own.
func (b *Binding) Decisive() bool {
	for _, e := range b.Evidence {
		if e.Decisive {
			return true
		}
	}
	return false
}

// Problem is why a binding could not be made, or could not be trusted.
//
// Typed so a caller can tell a QUESTION from a REFUSAL: an ambiguity is answerable by
// the user, and a wrong-kind focus is not.
type Problem struct {
	// Reason is the machine-readable cause.
	Reason ProblemReason `json:"reason"`
	// Message is the sentence to show.
	Message string `json:"message"`
	// Candidates are the plausible objects, for an ambiguity.
	Candidates []Candidate `json:"candidates,omitempty"`
}

func (p Problem) Error() string { return p.Message }

// Clarifiable reports whether the user could resolve this by answering.
func (p Problem) Clarifiable() bool {
	return p.Reason == ReasonAmbiguous || p.Reason == ReasonNoFocus
}

// ProblemReason is the closed set of ways a binding fails.
type ProblemReason string

const (
	// ReasonNoFocus: nothing is focused, so "this" names nothing.
	ReasonNoFocus ProblemReason = "no_focus"
	// ReasonWrongKind: something is focused and it is the wrong sort of thing.
	ReasonWrongKind ProblemReason = "wrong_kind"
	// ReasonNoResource: the object looks right but has no backing resource, so its
	// identity cannot be established — an unsaved tab, or a tab whose path the
	// application does not expose.
	ReasonNoResource ProblemReason = "no_backing_resource"
	// ReasonAmbiguous: several objects are equally plausible.
	ReasonAmbiguous ProblemReason = "ambiguous"
	// ReasonGone: the bound object no longer exists.
	ReasonGone ProblemReason = "object_gone"
	// ReasonChanged: focus or window moved to a different object.
	ReasonChanged ProblemReason = "object_changed"
	// ReasonUnknownKind: the procedure asked for a kind this build does not model.
	ReasonUnknownKind ProblemReason = "unknown_kind"
)

// Describe renders a reason for a person.
func (r ProblemReason) Describe() string {
	switch r {
	case ReasonNoFocus:
		return "nothing is focused"
	case ReasonWrongKind:
		return "the focused object is the wrong kind"
	case ReasonNoResource:
		return "the object has no file behind it"
	case ReasonAmbiguous:
		return "several objects are equally plausible"
	case ReasonGone:
		return "the object is no longer there"
	case ReasonChanged:
		return "the focus moved to a different object"
	case ReasonUnknownKind:
		return "the requested kind is not one this Director models"
	}
	return string(r)
}

// classify decides what kind of object an element is.
//
// Conservative by construction. An element becomes KindFile or KindDocument ONLY when a
// backing path is positively reported; everything else that looks file-ish is an editor
// buffer or a control. Guessing upward here is the whole hazard: a tab labelled
// "Budget.txt" with no path behind it is not a file, however much it reads like one.
func classify(el *directorapi.Element) (ObjectKind, string) {
	if el == nil {
		return "", ""
	}

	// A source that POSITIVELY established what this control stands for settles both
	// questions at once, and settles them better than anything below could: the kind
	// comes from the shell's own account of the object rather than from the shape of a
	// path, so a file called "Reports" and a folder called "Reports.txt" are each what
	// they actually are.
	//
	// Checked first, and only when Known() — which requires both a path and a kind this
	// build can check. A half-established identity falls through to the weaker rules
	// below and is refused there, which is the same answer as before this existed.
	if r := el.Resource; r.Known() {
		switch {
		case r.IsFolder():
			return KindFolder, r.Path
		case r.IsFile():
			return KindFile, r.Path
		}
	}

	resource := resourceOf(el)

	switch el.Role {
	case directorapi.RoleWindow:
		return KindWindow, resource
	case directorapi.RoleTextField:
		// A focused text field with a selection is a selection, not a file.
		return KindTextSelection, ""
	}

	// A path is the decisive signal, and it decides between file and folder by its own
	// shape rather than by the control's role — a list item and a tree item are the
	// same thing here.
	if resource != "" {
		if looksLikeFolder(resource) {
			return KindFolder, resource
		}
		return KindFile, resource
	}

	// No path. A tab is an editor buffer; anything else is just a control. Neither
	// satisfies "this file", which is the point.
	if el.Role == directorapi.RoleTab {
		return KindEditorBuffer, ""
	}
	return KindControl, ""
}

// resourcePath is an element's backing path, from the strongest thing that knows it.
//
// The typed resource first: it came from a source that positively established the object,
// and it is the only one that can say a folder called "Reports.txt" is a folder. The
// attribute is the older channel and stays as a fallback so a provider that reports a path
// without the richer identity keeps working.
//
// ONE function, used by both classification and re-identification. They disagreed once —
// classification read the typed resource and re-identification read only the attribute, so
// a binding made from a shell path could never be re-found and every revalidation reported
// the file as gone.
func resourcePath(el *directorapi.Element) string {
	if el == nil {
		return ""
	}
	if r := el.Resource; r.Known() {
		return r.Path
	}
	return resourceOf(el)
}

// resourceOf reads a backing path off an element, empty when none is reported.
//
// Through Attributes, which is the documented channel for source-specific detail: an
// accessibility provider that knows an item's path says so there, and one that does not
// is not forced to invent one.
func resourceOf(el *directorapi.Element) string {
	for _, key := range []string{"path", "file_path", "resource", "uri", "full_path"} {
		if v, ok := el.Attributes[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// looksLikeFolder reports whether a path names a directory.
//
// By SHAPE, not by asking the filesystem: the Director may be looking at a remote or
// virtual location, and a stat here would be both slow and occasionally wrong. A path
// with no extension in its final segment, or one ending in a separator, is a folder.
func looksLikeFolder(path string) bool {
	trimmed := strings.TrimRight(path, `/\`)
	if trimmed != path {
		return true // ended in a separator
	}
	base := trimmed
	if i := strings.LastIndexAny(trimmed, `/\`); i >= 0 {
		base = trimmed[i+1:]
	}
	// A dot that is not the first character marks an extension. ".gitignore" is a file
	// whose whole name is an extension, which is why the index must be past zero.
	return strings.LastIndex(base, ".") <= 0
}
