// Package inline models an editor that an application opens ON an object the Director has
// already bound — File Explorer's in-place rename box being the one case it serves today.
//
// The governing rules:
//
//	The inline editor is correlated with the same bound file, not merely with a text box
//	displaying the same caption.
//	Do not treat an arbitrary focused text box as the rename editor.
//	Prefer refusal over ambiguous editor correlation.
//
// # Why this needs a model at all
//
// The live rename got three steps in and stopped, and the reason was that "the field that
// just opened" is not a thing the Director could name. Its only handle on it was "whatever
// holds focus", and in File Explorer that is dangerous in a specific way: the details view
// already contains an Edit control per column per row, and the one in the Name column of
// the selected row has the value "Alpha.txt" and a ValuePattern. It is not the rename
// editor. Typing into it changes a caption and renames nothing — which is exactly the
// failure Part 6 of the milestone names, and exactly what the previous run did.
//
// # What Windows 11 actually does, observed rather than assumed
//
// Captured from a real Explorer window before and after invoking the command-bar Rename
// button (element counts identical, 166 → 166):
//
//	ClassName    UIRenameTextElement     ← a class that exists only in rename mode
//	ControlType  ControlType.Edit
//	AutomationId ""                       ← none; the class is the marker
//	Value        "Alpha.txt"              ← the item's current display name
//	Focused      true
//	parent       UIItemsView ("Items View")
//
// and at the same moment the selected row's Name cell (AutomationId
// System.ItemNameDisplay) goes EMPTY — the editor replaces its presentation. The editor is
// a SIBLING within the items view rather than a descendant of the list item, because it
// belongs to a different UIA provider; so "is it a child of the bound item?" is not a
// correlation fact that exists here, and this package does not pretend it is.
//
// Rename mode is also fragile: it exits when the window loses focus. Observing does not
// disturb it — that was checked — but anything that steals the foreground does.
package inline

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Property is what an inline editor edits.
//
// A closed vocabulary, small on purpose. An editor whose property this build cannot name
// is not something to act on.
type Property string

const (
	// PropertyFilename is a shell item's name on disk, which is what Explorer's
	// in-place rename edits.
	PropertyFilename Property = "filename"
)

// Describe renders a property for a person.
func (p Property) Describe() string {
	if p == PropertyFilename {
		return "the file's name"
	}
	return string(p)
}

// editorClasses are the control classes that ARE an inline editor.
//
// A closed list, and the load-bearing piece of the whole correlation. Windows 11's
// Explorer gives its rename box a dedicated class, which is the difference between "a text
// box that happens to show this filename" and "the control this application opened to
// rename this item". Nothing else in the window carries it.
//
// A list rather than a constant because a future Explorer, or a second file manager, would
// add its own — and because the empty case has to stay expressible: a build that knows no
// editor class for the application in front of it must find no editor, not guess one.
var editorClasses = map[string]Property{
	"UIRenameTextElement": PropertyFilename,
}

// Editor is an inline editor, correlated with the object it edits.
//
// Request-scoped and short-lived by nature: the control exists only while the application
// is in edit mode, and a stored one is worthless the moment the mode ends. See Snapshot for
// what may outlive it.
type Editor struct {
	// ElementID and NativeID identify the control in the world it was found in.
	ElementID string `json:"element_id,omitempty"`
	NativeID  string `json:"native_id,omitempty"`
	// ClassName is the control class that identified it as an editor.
	ClassName string `json:"class_name,omitempty"`
	// WindowID is the window it belongs to.
	WindowID string `json:"window_id,omitempty"`

	// Property is what it edits, and Resource the object whose property that is.
	Property Property `json:"property"`
	Resource string   `json:"resource,omitempty"`

	// Value is what the editor contained when it was found.
	Value string `json:"value"`
	// Focused and Editable report its state.
	Focused  bool `json:"focused"`
	Editable bool `json:"editable"`

	// Evidence is why this control was accepted as the editor for that object.
	Evidence []string `json:"evidence,omitempty"`
	// Generation is the observation this was found in, so a later step can tell a
	// re-found editor from a remembered one.
	Generation string `json:"generation,omitempty"`
	// Confidence is 0..1.
	Confidence float64 `json:"confidence"`
}

// Found reports whether this is a real editor rather than a zero value.
func (e *Editor) Found() bool { return e != nil && e.ElementID != "" }

// Describe renders an editor in one line.
func (e *Editor) Describe() string {
	if !e.Found() {
		return "(no inline editor)"
	}
	return fmt.Sprintf("%s editing %s of %s, containing %q",
		e.ClassName, e.Property.Describe(), e.Resource, e.Value)
}

// Result is the closed verdict vocabulary for both verifications this package performs.
type Result string

const (
	// Verified: the editor is there, it belongs to the bound object, and it is editable.
	Verified Result = "verified"
	// Mismatched: an editor is there and it is not for the bound object.
	Mismatched Result = "mismatched"
	// Ambiguous: several controls could be the editor, so none of them is it.
	Ambiguous Result = "ambiguous"
	// Absent: no editor appeared.
	Absent Result = "absent"
	// Unverified: nothing available could settle it. NOT a pass.
	Unverified Result = "unverified"
)

// OK reports whether the step may proceed.
func (r Result) OK() bool { return r == Verified }

// Verification is the structured account of one check.
type Verification struct {
	Result Result `json:"result"`
	// Expected is the bound resource the editor had to belong to.
	Expected string `json:"expected_resource,omitempty"`
	// Editor is what was accepted, nil when nothing was.
	Editor *Editor `json:"editor,omitempty"`
	// Property is what the editor edits.
	Property Property `json:"property,omitempty"`
	// Value is what it contained.
	Value string `json:"value,omitempty"`
	// Evidence is what connected the editor to the resource; Missing is what did not.
	Evidence []string `json:"evidence,omitempty"`
	Missing  []string `json:"missing,omitempty"`
	// Candidates are the other plausible editors, for an ambiguity.
	Candidates []string `json:"candidates,omitempty"`
	Confidence float64  `json:"confidence"`
	// Reason is the verdict in one sentence.
	Reason string `json:"reason"`
}

// Describe renders a verification for a trace.
func (v Verification) Describe() string {
	out := string(v.Result)
	if v.Reason != "" {
		out += ": " + v.Reason
	}
	return out
}

// Snapshot is an editor as durable, diagnostic history.
//
//	Do not serialize ephemeral native handles as durable graph identity.
//	Replay must not assume an old inline editor still exists.
//
// So this records WHAT was edited and on what evidence, and deliberately not enough to
// find the control again. A replay re-enters rename mode from the stored file binding and
// derives a fresh editor; there is no field here that would let it skip that.
type Snapshot struct {
	// Property and Resource say what was being edited, which is the durable part.
	Property Property `json:"property"`
	Resource string   `json:"resource,omitempty"`
	// ClassName is the control class, kept because "which mechanism was this?" is a
	// real question a week later.
	ClassName string `json:"class_name,omitempty"`
	// InitialValue and FinalValue bracket the edit.
	InitialValue string `json:"initial_value,omitempty"`
	FinalValue   string `json:"final_value,omitempty"`
	// Evidence is why the editor was tied to the resource.
	Evidence []string `json:"evidence,omitempty"`
}

// Snapshot converts a live editor into durable history.
//
// The element id and the native id are dropped on purpose: they name a control that no
// longer exists by the time anything reads this, and keeping them would invite a replay to
// try.
func (e *Editor) Snapshot() *Snapshot {
	if !e.Found() {
		return nil
	}
	return &Snapshot{
		Property: e.Property, Resource: e.Resource, ClassName: e.ClassName,
		InitialValue: e.Value,
		Evidence:     append([]string{}, e.Evidence...),
	}
}

// ── finding one ───────────────────────────────────────────────────────────────

// Find looks for the inline editor an application has opened on a bound object.
//
// The correlation, and every clause is a way to refuse:
//
//  1. The control's CLASS must be one this build knows to be an inline editor. A text box
//     with the right contents is not an editor; a control the application created for
//     editing is. This is what keeps the details-view Name cell, the address bar and the
//     search box out.
//  2. It must be in the SAME WINDOW as the bound object.
//  3. Exactly one such control may exist. Two is an ambiguity and neither is chosen.
//  4. The shell must STILL report the bound object as the selected one — the editor is
//     "the box open on that item", and an item that is no longer selected is no longer
//     the thing being edited.
//  5. Its value must be the bound object's display name, allowing for a hidden extension.
//     Corroboration rather than identification: it is what distinguishes an editor opened
//     on this item from one opened on the item below.
func Find(w *directorapi.WorldState, b *binding.Binding) (*Editor, Verification) {
	return find(w, b, true)
}

// FindOpen locates the editor a previous step in the same request has already been
// editing.
//
// Identical to Find except for clause 5: it does not require the box to still contain the
// item's ORIGINAL name. By the time an edit is committed the whole point is that the value
// has changed, and re-applying the discovery rule would refuse the very editor whose
// contents are about to be committed — which is what stopped the fourth step of a rename
// after the third had typed the new name successfully.
//
// Everything else still has to hold, and it is what the correlation now rests on: the
// control CLASS, exactly one of them, the same window, and the bound item still selected.
// The value is corroboration for discovery, not identity, and this is the one moment when
// it is expected to disagree.
func FindOpen(w *directorapi.WorldState, b *binding.Binding) (*Editor, Verification) {
	return find(w, b, false)
}

// find is Find with the initial-value check made optional.
//
// The check belongs to DISCOVERING an editor and not to re-checking one already
// discovered: once the new name has been typed, the editor no longer contains the item's
// old name, and re-running the same rule would reject the very editor whose value it is
// about to confirm. Identity is carried across that boundary by the native id, which is
// stronger evidence than the contents were.
func find(w *directorapi.WorldState, b *binding.Binding, checkValue bool) (*Editor, Verification) {
	v := Verification{Result: Absent, Property: PropertyFilename}
	if b.Bound() {
		v.Expected = b.Resource
	}
	if w == nil || !b.Bound() {
		v.Result = Unverified
		v.Reason = "there is nothing to look in, or nothing bound to look for"
		v.Missing = append(v.Missing, "an observed world and a bound object")
		return nil, v
	}

	var candidates []*directorapi.Element
	for _, el := range w.Elements {
		if _, ok := editorClasses[classOf(el)]; !ok {
			continue
		}
		if b.WindowID != "" && string(el.WindowID) != b.WindowID {
			// A different window's editor is a different application's business.
			continue
		}
		candidates = append(candidates, el)
	}

	switch len(candidates) {
	case 0:
		v.Reason = fmt.Sprintf(
			"nothing in this window is an inline editor, so %s did not enter edit mode",
			describeBound(b))
		v.Missing = append(v.Missing, "a control of a known inline-editor class")
		return nil, v
	case 1:
	default:
		v.Result = Ambiguous
		for _, el := range candidates {
			v.Candidates = append(v.Candidates, fmt.Sprintf("%s %q", classOf(el), el.Label))
		}
		v.Reason = fmt.Sprintf("%d inline editors are open at once, so which one belongs "+
			"to %s cannot be established", len(candidates), describeBound(b))
		return nil, v
	}

	el := candidates[0]
	ed := &Editor{
		ElementID: string(el.ID), NativeID: nativeIDOf(el), ClassName: classOf(el),
		WindowID: string(el.WindowID), Property: editorClasses[classOf(el)],
		Resource: b.Resource, Value: valueOf(el),
		Focused: el.Focused, Editable: el.Enabled,
		Generation: w.Timestamp.UTC().Format("20060102T150405.000000000Z"),
	}
	ed.Evidence = append(ed.Evidence,
		fmt.Sprintf("%q is an inline editor by control class, not by what it contains",
			ed.ClassName),
		"it is the only one open in this window")

	// The bound object must still be the selected one. Without this the editor could be
	// the box the user opened on a different row a moment later.
	if still, ok := stillSelected(w, b); ok {
		ed.Evidence = append(ed.Evidence, still)
	} else {
		v.Result = Mismatched
		v.Editor = ed
		v.Missing = append(v.Missing, "the bound object is no longer the selected one")
		v.Reason = fmt.Sprintf("an editor is open and %s is no longer what is selected, "+
			"so the editor is not for it", describeBound(b))
		return nil, v
	}

	// Its initial value should be the item's name. Corroboration: it is what tells an
	// editor opened on Alpha.txt from one opened on Bravo.txt.
	switch {
	case !checkValue:
		ed.Evidence = append(ed.Evidence, "identity carried from the editor already found")
		ed.Confidence = 1
	case nameMatches(ed.Value, b.Label):
		ed.Evidence = append(ed.Evidence, fmt.Sprintf(
			"it opened containing %q, which is what the item is called", ed.Value))
		ed.Confidence = 1
	case ed.Value == "":
		// An editor that reports no value yet. Accepted on the class and the selection,
		// with the gap recorded rather than papered over.
		ed.Evidence = append(ed.Evidence, "it reports no value yet")
		ed.Confidence = 0.8
		v.Missing = append(v.Missing, "the editor's initial value")
	default:
		v.Result = Mismatched
		v.Editor = ed
		v.Missing = append(v.Missing, fmt.Sprintf(
			"the editor contains %q and the bound item is called %q", ed.Value, b.Label))
		v.Reason = fmt.Sprintf("the editor open in this window is for %q, not for %s",
			ed.Value, describeBound(b))
		return nil, v
	}

	if !ed.Editable {
		v.Result = Mismatched
		v.Editor = ed
		v.Missing = append(v.Missing, "the editor is not enabled")
		v.Reason = "an editor is open and it is not editable"
		return nil, v
	}

	v.Result = Verified
	v.Editor = ed
	v.Value = ed.Value
	v.Evidence = ed.Evidence
	v.Confidence = ed.Confidence
	v.Reason = fmt.Sprintf("%s is in edit mode: %s", describeBound(b), ed.Describe())
	return ed, v
}

// ── verifying the replacement ─────────────────────────────────────────────────

// VerifyValue checks that the editor now contains what was typed, and is still the same
// editor for the same object.
//
//	Do not infer successful text entry only because the input capability returned
//	success.
//
// So this re-finds the editor in a fresh world and compares. An editor that closed, moved
// to another object, or still holds the old text each produce a distinct verdict — and
// none of them is "the capability said ok".
func VerifyValue(w *directorapi.WorldState, b *binding.Binding, before *Editor,
	want string) Verification {

	// Re-found WITHOUT the initial-value check: the whole point of this call is that the
	// value has deliberately changed. What ties this editor to the previous one is the
	// native id, checked immediately below.
	ed, v := find(w, b, false)
	v.Property = PropertyFilename
	if v.Result != Verified {
		// Absent is the interesting one here: the editor closing before the value could
		// be confirmed means the edit was abandoned or committed by something else.
		if v.Result == Absent {
			v.Reason = "the editor closed before the new name could be confirmed, so " +
				"what it contained was never established"
		}
		return v
	}

	// The SAME editor, not merely an editor. A box that closed and reopened is a
	// different edit transaction, and the value in it says nothing about the first.
	if before.Found() && before.NativeID != "" && ed.NativeID != "" &&
		before.NativeID != ed.NativeID {
		v.Result = Mismatched
		v.Missing = append(v.Missing, "the editor is not the one the text was typed into")
		v.Reason = "the editor was replaced between typing and checking, so the value " +
			"that was typed cannot be confirmed"
		return v
	}

	v.Value = ed.Value
	if !valueMatches(ed.Value, want) {
		v.Result = Mismatched
		v.Missing = append(v.Missing, fmt.Sprintf(
			"the editor contains %q and %q was intended", ed.Value, want))
		v.Reason = v.Missing[len(v.Missing)-1]
		return v
	}
	v.Evidence = append(v.Evidence, fmt.Sprintf("the editor contains %q", ed.Value))
	v.Reason = fmt.Sprintf("the editor for %s contains %q, which is what was intended",
		describeBound(b), ed.Value)
	return v
}

// ── verifying the commit ──────────────────────────────────────────────────────

// VerifyClosed checks that the edit mode ended.
//
//	A closed editor alone is insufficient.
//
// Which is why this is deliberately only HALF of the commit check: it says the transaction
// ended, and says nothing about what it ended as. The other half is the filesystem
// correlation, which is where "the rename happened" is actually established.
func VerifyClosed(w *directorapi.WorldState, b *binding.Binding) Verification {
	_, v := Find(w, b)
	switch v.Result {
	case Absent:
		return Verification{
			Result: Verified, Expected: v.Expected, Property: PropertyFilename,
			Evidence: []string{"no inline editor is open any more"},
			Reason: "the edit mode ended. That alone does not mean the rename happened — " +
				"the filesystem is what settles that.",
		}
	case Verified:
		return Verification{
			Result: Mismatched, Expected: v.Expected, Property: PropertyFilename,
			Editor:  v.Editor,
			Missing: []string{"the editor is still open"},
			Reason:  "the editor is still in edit mode, so the change was not committed",
		}
	}
	// Ambiguous, mismatched or unverified: reported as it stands. An editor that is now
	// for something else is not evidence that this one committed.
	return v
}

// ── helpers ───────────────────────────────────────────────────────────────────

// classOf reads a control's class from the provider's attributes.
func classOf(el *directorapi.Element) string {
	if el == nil {
		return ""
	}
	v, _ := el.Attributes["class_name"].(string)
	return v
}

func nativeIDOf(el *directorapi.Element) string {
	v, _ := el.Attributes["native_id"].(string)
	return v
}

func valueOf(el *directorapi.Element) string {
	if el == nil {
		return ""
	}
	return el.Value
}

// stillSelected reports whether the bound object is still the selected one.
func stillSelected(w *directorapi.WorldState, b *binding.Binding) (string, bool) {
	for _, el := range w.Elements {
		if r := el.Resource; r.Known() && strings.EqualFold(r.Path, b.Resource) {
			if el.Selected || el.Focused {
				return fmt.Sprintf("the shell still reports %s as the selected item", r.Path),
					true
			}
		}
	}
	// No element carries the bound resource any more. In Explorer that is EXPECTED while
	// renaming — the row's presentation is replaced by the editor — so this is not on its
	// own a reason to refuse, and the caller's other evidence carries it.
	for _, el := range w.Elements {
		if el.Selected && strings.EqualFold(el.Label, b.Label) {
			return fmt.Sprintf("%q is still the selected item", b.Label), true
		}
	}
	return "", false
}

// nameMatches compares an editor's contents with an item's caption.
//
// Equal, or equal once an extension is dropped from either side — Explorer shows the
// extension in the rename box even when the list hides it, and the reverse happens too.
// Nothing looser: a prefix rule would match "Alpha.txt" against "Alpha (copy).txt".
func nameMatches(editorValue, itemLabel string) bool {
	a := strings.TrimSpace(strings.ToLower(editorValue))
	b := strings.TrimSpace(strings.ToLower(itemLabel))
	if a == "" || b == "" {
		return false
	}
	return a == b || stem(a) == b || a == stem(b) || stem(a) == stem(b)
}

// valueMatches compares what the editor holds with what was intended.
//
// The intended name may or may not carry an extension, and the editor may or may not show
// one; both spellings of the same intention are accepted. What is NOT accepted is a
// different name, which is the whole point.
func valueMatches(got, want string) bool {
	a := strings.TrimSpace(strings.ToLower(got))
	b := strings.TrimSpace(strings.ToLower(want))
	if a == "" || b == "" {
		return false
	}
	return a == b || stem(a) == b || a == stem(b)
}

// stem drops a trailing extension. A leading dot is not an extension: ".gitignore" is a
// name whose whole self looks like one.
func stem(s string) string {
	if i := strings.LastIndex(s, "."); i > 0 {
		return s[:i]
	}
	return s
}

func describeBound(b *binding.Binding) string {
	if !b.Bound() {
		return "the bound object"
	}
	if b.Resource != "" {
		return b.Resource
	}
	return fmt.Sprintf("%q", b.Label)
}
