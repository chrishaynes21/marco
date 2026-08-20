// Package vision turns what a screen LOOKS LIKE into observations.
//
// The safety rule this package exists to enforce, stated once so nothing below has to
// re-argue it:
//
//	A detected box is evidence that something SHAPED LIKE a control is at a location.
//	It is not, by itself, evidence that an INTERACTIVE CONTROL is there.
//
// This is the OCR package's rule one level up, and it has to be stated separately because
// the temptation here is much stronger. OCR reports glyphs, and nobody is tempted to click
// glyphs. A UI-element detector reports something it CALLS a button, and the whole point of
// the model is that it is usually right — which makes it exactly the kind of evidence that
// gets promoted to fact by accident.
//
// So:
//
//   - A detection never asserts ACTIONABILITY. Enabled, Visible and Focused are left
//     unknown, because a picture cannot establish any of them: a greyed-out button and a
//     live one differ by a colour a model was not trained to judge, and the difference
//     between them is a click that does nothing and a click that deletes something.
//   - A detection whose class is TEXT stays text. Reading the word "Delete" in a
//     detected label does not produce a Delete button, for the same reason it does not in
//     OCR — it might be a heading, a log line, or a document someone is writing.
//   - A class this build does not recognise is REFUSED rather than passed through with a
//     guessed role. An unmapped class arriving as an element is how a model's private
//     vocabulary becomes the Director's.
//
// What is left after those three is genuinely useful: a full-screen game with no
// accessibility tree goes from "the Director cannot see into this" to "the Director can
// see boxes here, weighed as boxes". Fusion decides whether a box beside a piece of OCR
// text becomes a belief about a control, and the policy engine still refuses to do
// anything destructive on unstructured evidence alone.
//
// # What this package is not
//
// It is not a model. There is no inference here, no weights, no image preprocessing beyond
// cropping, and no dependency that could bring any of those in — the Director's packages
// import the standard library and nothing else. A Detector is an interface, satisfied
// today by a bridge to plugins/vision and tomorrow by whatever else, and the backend never
// sees a desktop coordinate or an observation.
package vision

import (
	"context"
	"errors"
	"image"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ── the detector boundary ─────────────────────────────────────────────────────

// Detector finds things in an image. That is its entire responsibility.
//
// An interface so the Director is never bound to one vision implementation: OpenCV today,
// a YOLO-style detector tomorrow, something else later. None of that may reach fusion,
// which is why a Detector returns plain results rather than observations — turning a
// result into evidence involves coordinate conversion, window scoping, class mapping and
// filtering, and every one of those is the provider's job.
//
// A Detector sees image-local pixels and returns image-local boxes. It has no way to place
// anything on a desktop, deliberately: a backend cannot get placement wrong because a
// backend is never told where the picture came from.
type Detector interface {
	// Detect finds the elements in one image.
	Detect(ctx context.Context, in Input) ([]Detection, error)
	// Model names what produced the results — the model file, the algorithm, the
	// version. Carried onto every observation, so a reader can tell which detector
	// believed something.
	Model() string
}

// Input is what a detector is asked to look at.
type Input struct {
	// Image is in image-local pixels. The detector never sees desktop coordinates.
	Image image.Image
	// Classes, when non-empty, restricts what to look for. A hint: a detector free to
	// ignore it is behaving correctly, and the provider filters afterwards regardless.
	Classes []string
}

// Detection is one thing a detector found, in IMAGE-LOCAL pixels.
type Detection struct {
	// Class is the detector's own word for what this is ("button", "icon",
	// "text", "checkbox"). Mapped to the Director's vocabulary by the provider —
	// see classMap — and REFUSED when it maps to nothing.
	Class string
	// Bounds is where it is, in image-local pixels.
	Bounds image.Rectangle
	// Confidence is 0..1. Detectors that score differently normalise before
	// returning; a mixed convention here would silently reject or accept everything.
	Confidence float64
	// Text is what the detector read inside the box, when it reads text at all. Most
	// do not, and empty is the ordinary answer. It is treated as a LABEL HINT and
	// never as a reason to make something actionable.
	Text string
	// Attributes carry anything else the backend wants to say, opaque to the Director
	// and recorded as metadata. Never interpreted: a backend cannot smuggle semantics
	// through a map that nothing reads.
	Attributes map[string]string
}

// Unavailable reports that no vision backend could be reached.
//
// A distinct error type for the reason OCR has one: "no detector configured" and "the
// detector ran and found nothing" must never look alike. The first is a capability gap the
// user can fix and the Director should say so; the second is a fact about the screen.
// Returning an empty success for the first would make a window look featureless.
type Unavailable struct {
	Backend string
	Reason  string
}

func (u *Unavailable) Error() string {
	return "vision: " + u.Backend + " is unavailable: " + u.Reason
}

// IsUnavailable reports whether err is an unavailability rather than a failure.
func IsUnavailable(err error) bool {
	var u *Unavailable
	return errors.As(err, &u)
}

// ── the class vocabulary ──────────────────────────────────────────────────────

// Class is what a detection is, in the Director's own words.
//
// A CLOSED vocabulary, and the closure is the safety property. A detector's classes are
// its own — one model says "btn", another "Button", a third "clickable_region" — and
// passing those through would put a model's private taxonomy into the world model, where
// nothing downstream could reason about it. So each is mapped here, by a table a person
// can read, and anything unmapped is refused.
type Class string

const (
	// ClassButton is something shaped like a pressable control.
	ClassButton Class = "button"
	// ClassIcon is a small pictorial control or indicator.
	ClassIcon Class = "icon"
	// ClassText is rendered text. Stays TEXT — see the package doc.
	ClassText Class = "text"
	// ClassField is something shaped like an input.
	ClassField Class = "field"
	// ClassCheckbox and ClassRadio are the two-state shapes.
	ClassCheckbox Class = "checkbox"
	ClassRadio    Class = "radio"
	// ClassSlot is a cell of a regular grid — an inventory square, a palette swatch,
	// a calendar day. Named for the SHAPE rather than for anything a game calls it.
	ClassSlot Class = "slot"
	// ClassBar is a filled proportion — a progress bar, a health bar, a meter.
	ClassBar Class = "bar"
	// ClassPanel is a bounded region containing other things — a dialog, a card, a
	// grouping box.
	ClassPanel Class = "panel"
	// ClassMenu is a list of choices.
	ClassMenu Class = "menu"
	// ClassImage is pictorial content that is not a control.
	ClassImage Class = "image"
)

// classes is every class, in a stable order.
var classes = []Class{
	ClassButton, ClassIcon, ClassText, ClassField, ClassCheckbox, ClassRadio,
	ClassSlot, ClassBar, ClassPanel, ClassMenu, ClassImage,
}

// Classes returns the vocabulary.
func Classes() []Class { return append([]Class{}, classes...) }

// Known reports whether a class is one this build can reason about.
func (c Class) Known() bool {
	for _, k := range classes {
		if k == c {
			return true
		}
	}
	return false
}

// Structural reports whether a class describes a THING rather than rendered content.
//
// The line the safety rule is drawn on. A structural class may become an element
// observation — one the Director can eventually act on, if fusion and policy agree. A
// non-structural one may only become text, whatever the detector called it.
func (c Class) Structural() bool {
	switch c {
	case ClassText, ClassImage:
		return false
	}
	return c.Known()
}

// Nameable reports whether a class earns the right to carry readable text.
//
// STRUCTURAL and NAMEABLE are different properties and this build needs both separately. A
// `panel` is a real thing at a real place and reading its contents would read whatever the
// panel contains; an `icon` is a real thing whose "label" on a desktop is a document title or
// a contact. Both are structural. Neither may be named.
//
// The allowlist itself is not decided here — it is `directorapi.ElementRole.NameablePlaintext`,
// the same property the session's privacy classifier applies. This method exists so the reader
// can spend its budget on boxes whose text will survive, rather than reading text the
// classifier is about to withhold.
func (c Class) Nameable() bool { return c.Structural() && c.Role().NameablePlaintext() }

// Role is the element role a structural class maps to.
//
// RoleUnknown for the non-structural ones, and for anything outside the vocabulary. A
// detector's box is a claim about SHAPE, and these are the shapes the Director's own role
// vocabulary has words for.
func (c Class) Role() directorapi.ElementRole {
	switch c {
	case ClassButton:
		return directorapi.RoleButton
	case ClassIcon:
		return directorapi.RoleIcon
	case ClassField:
		return directorapi.RoleTextField
	case ClassCheckbox:
		return directorapi.RoleCheckbox
	case ClassRadio:
		return directorapi.RoleRadio
	case ClassSlot:
		// A grid cell is a list item: it is one of many similar things arranged
		// together, which is what the role means and what a collection iterates.
		return directorapi.RoleListItem
	case ClassBar:
		return directorapi.RoleProgressBar
	case ClassPanel:
		return directorapi.RoleGroup
	case ClassMenu:
		return directorapi.RoleMenu
	}
	return directorapi.RoleUnknown
}

// classMap translates a detector's own word into this vocabulary.
//
// A reviewable table, and deliberately not a fuzzy match. A model that says "clickable"
// means something its authors decided; whether that is a button here is a judgement, and
// judgements belong in a table somebody read rather than in a string-distance function.
//
// Anything absent is REFUSED. That is the point: a new model's vocabulary is a change to
// this table, reviewed, rather than a silent widening of what the Director believes.
var classMap = map[string]Class{
	"button": ClassButton, "btn": ClassButton, "push_button": ClassButton,
	"pushbutton": ClassButton, "clickable": ClassButton,

	"icon": ClassIcon, "image_icon": ClassIcon, "symbol": ClassIcon,

	"text": ClassText, "label": ClassText, "textblock": ClassText,
	"static_text": ClassText, "caption": ClassText, "title": ClassText,

	"field": ClassField, "input": ClassField, "textbox": ClassField,
	"text_field": ClassField, "edit": ClassField, "textinput": ClassField,

	"checkbox": ClassCheckbox, "check_box": ClassCheckbox, "toggle": ClassCheckbox,
	"radio": ClassRadio, "radio_button": ClassRadio,

	"slot": ClassSlot, "cell": ClassSlot, "grid_cell": ClassSlot,
	"item_slot": ClassSlot, "tile": ClassSlot,

	"bar": ClassBar, "progress": ClassBar, "progressbar": ClassBar,
	"progress_bar": ClassBar, "meter": ClassBar, "gauge": ClassBar,

	"panel": ClassPanel, "dialog": ClassPanel, "group": ClassPanel,
	"container": ClassPanel, "card": ClassPanel, "window": ClassPanel,

	"menu": ClassMenu, "menu_list": ClassMenu, "dropdown": ClassMenu,

	"image": ClassImage, "picture": ClassImage, "graphic": ClassImage,
}

// ClassOf maps a detector's class name, reporting false for anything unmapped.
//
// Case- and separator-insensitive, because "Push Button", "push_button" and "pushButton"
// are the same claim written three ways — and that is normalisation of SPELLING, not of
// meaning. A word absent from the table stays absent however it is spelled.
func ClassOf(name string) (Class, bool) {
	key := normalizeClass(name)
	if key == "" {
		return "", false
	}
	if c, ok := classMap[key]; ok {
		return c, true
	}
	// A detector that already speaks the Director's vocabulary.
	if c := Class(key); c.Known() {
		return c, true
	}
	return "", false
}

// normalizeClass lowercases and collapses separators.
func normalizeClass(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return strings.Trim(s, "_")
}
