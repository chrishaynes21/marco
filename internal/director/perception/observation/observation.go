// Package observation is the Director's evidence layer.
//
// The architectural rule this package exists to enforce:
//
//	Observations are EVIDENCE. World State is BELIEF.
//	The Director reasons only over belief.
//	Providers contribute only evidence.
//	Fusion is the sole component allowed to convert one into the other.
//
// Before this existed, the accessibility provider effectively WAS the world model:
// what it reported became elements, and every downstream component silently inherited
// its assumptions. That is fine while there is one source and fatal the moment there
// are two, because there is nowhere for a second account of the same button to go
// except into a duplicate of it.
//
// So perception is split. A provider's job ends at "here is what I saw"; it never
// assigns an element id, never decides two reports are the same object, and never
// constructs a WorldState. Everything downstream of fusion — the planner, the target
// resolver, policy, verification, replay, the action graph — sees only elements, and
// none of them can tell whether the evidence came from accessibility, OCR, a browser's
// DOM, a vision model or a skill. That is the property that lets OCR be added later
// without touching any of them.
package observation

import (
	"strings"
	"time"
	"unicode"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Kind and the kind constants are re-exported from directorapi, where they live
// because an Element's provenance records them and the contract package cannot import
// this one.
type Kind = directorapi.ObservationKind

const (
	ElementObservation     = directorapi.ObservationElement
	TextObservation        = directorapi.ObservationText
	WindowObservation      = directorapi.ObservationWindow
	ApplicationObservation = directorapi.ObservationApplication
	VisualObservation      = directorapi.ObservationVisual
)

// Source is where evidence came from. The priority ladder lives in directorapi.
type Source = directorapi.ObservationSource

// Observation is one piece of evidence: one source's report about one thing at one
// instant.
//
// An interface rather than a struct because the kinds are genuinely different shapes.
// OCR reports a string and a box with no claim about what it labels; accessibility
// reports a control with a role and a state; the window system reports a window. What
// they have in common is exactly what this interface names — where, when, from whom,
// how sure, and about what — and fusion needs no more than that to decide which
// evidence concerns which object.
//
// Deliberately read-only. An observation is a record of something that was true a
// moment ago; nothing downstream may edit it, because provenance that could be
// rewritten after the fact would explain nothing.
type Observation interface {
	ID() directorapi.ObservationID
	Source() Source
	Timestamp() time.Time
	Bounds() directorapi.Rect
	Confidence() float64
	Kind() Kind
}

// Reference is the provenance entry for obs within the given cycle.
func Reference(obs Observation, cycle CycleID) directorapi.ObservationReference {
	return directorapi.ObservationReference{
		Observation: obs.ID(),
		Source:      obs.Source(),
		Kind:        obs.Kind(),
		Cycle:       string(cycle),
		At:          obs.Timestamp(),
	}
}

// ── element evidence ──────────────────────────────────────────────────────────

// Element is evidence about a UI control.
//
// It wraps the wire-format observation rather than restating its fields: that struct
// is what fixtures are recorded in and what providers already produce, and having two
// descriptions of the same thing is how they drift apart. The wrapper exists to give
// it the interface, not to replace it.
type Element struct {
	Raw directorapi.Observation
}

// NewElement wraps a raw observation as element evidence.
func NewElement(o directorapi.Observation) Element {
	o.Kind = directorapi.ObservationElement
	return Element{Raw: o}
}

func (e Element) ID() directorapi.ObservationID { return e.Raw.ID }
func (e Element) Source() Source                { return e.Raw.Source }
func (e Element) Timestamp() time.Time          { return e.Raw.Timestamp }
func (e Element) Bounds() directorapi.Rect      { return e.Raw.Bounds }
func (e Element) Kind() Kind                    { return ElementObservation }

// Confidence is the source's own certainty. A source that reports none is not thereby
// uncertain — accessibility never scores its own nodes — so zero reads as one.
func (e Element) Confidence() float64 {
	if e.Raw.Confidence <= 0 {
		return 1
	}
	return e.Raw.Confidence
}

// ── text evidence ─────────────────────────────────────────────────────────────

// Text is evidence that a string was rendered somewhere.
//
// Defined now and emitted by nobody, which is the point: OCR arrives as a provider
// that emits these, and fusion already knows what to do with them. Nothing about the
// engine, the planner or the world model changes on that day.
//
// The distinction from Element is not pedantry. Text is a claim about pixels, not
// about objects: reading "Save" somewhere is not evidence that a Save BUTTON exists,
// only that the word is on screen. Fusion may use it to reinforce a control an
// accessibility source already found, and must not invent a control from it alone.
type Text struct {
	ObservationID directorapi.ObservationID
	CycleID       CycleID
	// ProviderID names which provider produced this, for diagnostics — one machine may
	// eventually run more than one OCR engine, and "OCR said so" would not say which.
	ProviderID string

	From  Source
	At    time.Time
	Box   directorapi.Rect
	Score float64

	// Content is the recognised string, RAW and comparable. Both are kept because
	// they answer different questions: the raw form is what a person sees on screen
	// and what an explanation must quote, the comparable form is what fusion matches
	// on. Collapsing them would mean either matching on whitespace noise or showing
	// the user a string their screen does not contain.
	Content NormalizedText

	// WindowID and ApplicationID scope the text. Both are gates in text fusion: the
	// same word in two windows is two different words.
	WindowID      directorapi.WindowID
	ApplicationID string

	// Language is the recognition language, when the engine reports one.
	Language string
	// LineID groups words the engine split out of one rendered line, and WordIndex
	// orders them within it. Engine granularity is PRESERVED rather than pre-joined:
	// a provider that concatenated words would be making a grouping decision that
	// belongs to fusion, where it can be explained and refused.
	LineID    string
	WordIndex int

	Metadata map[string]string
}

func (t Text) ID() directorapi.ObservationID { return t.ObservationID }
func (t Text) Source() Source                { return t.From }
func (t Text) Timestamp() time.Time          { return t.At }
func (t Text) Bounds() directorapi.Rect      { return t.Box }
func (t Text) Confidence() float64           { return t.Score }
func (t Text) Kind() Kind                    { return TextObservation }

// Reference is the provenance entry for this text within a cycle.
func (t Text) Reference(cycle CycleID) directorapi.ObservationReference {
	return Reference(t, cycle)
}

// ── text normalisation ────────────────────────────────────────────────────────

// NormalizedText is a string as it appears and as it is compared.
type NormalizedText struct {
	// Raw is exactly what the source reported. Never rewritten: an explanation that
	// quoted a cleaned-up string would be quoting something the screen does not say.
	Raw string `json:"raw"`
	// Comparable is the form fusion matches on.
	Comparable string `json:"comparable,omitempty"`
}

// String is the raw form — what a person reads.
func (n NormalizedText) String() string { return n.Raw }

// Empty reports whether there is no text at all.
func (n NormalizedText) Empty() bool { return n.Comparable == "" }

// Differs reports whether normalisation changed anything, which is when an
// explanation should show both.
func (n NormalizedText) Differs() bool { return n.Raw != n.Comparable }

// NewText normalises s and keeps both forms.
func NewText(s string) NormalizedText {
	return NormalizedText{Raw: s, Comparable: Normalize(s)}
}

// Normalize reduces a string to the form fusion compares on.
//
// Deliberately conservative. It removes DECORATION that toolkits and OCR engines add
// and remove inconsistently — an accelerator marker, a trailing ellipsis, a label
// colon, case, whitespace runs — and nothing else.
//
// It does NOT spell-correct, infer missing words, expand abbreviations or drop
// punctuation from the raw value. Every one of those would let fusion match text the
// screen does not contain, and a match on invented text is a click on the wrong
// control with full confidence.
func Normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	space := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			// Runs of any whitespace become one space, and leading whitespace is
			// dropped. OCR splits and joins whitespace unpredictably at glyph
			// boundaries; a label differing only in spacing is the same label.
			space = b.Len() > 0
			continue
		case r == '&':
			// A keyboard accelerator marker. "&Save" and "Save" are one label.
			continue
		}
		if space {
			b.WriteRune(' ')
			space = false
		}
		b.WriteRune(unicode.ToLower(canonicalPunctuation(r)))
	}

	out := b.String()
	// Trailing decoration, innermost first: "Save As…:" → "save as".
	for {
		trimmed := strings.TrimSuffix(out, "...")
		trimmed = strings.TrimSuffix(trimmed, "…")
		trimmed = strings.TrimSuffix(trimmed, ":")
		trimmed = strings.TrimRight(trimmed, " ")
		if trimmed == out {
			return out
		}
		out = trimmed
	}
}

// canonicalPunctuation folds the punctuation variants that differ between what a
// toolkit renders and what an OCR engine reads back — smart quotes, dashes, the
// non-breaking space's visible cousins. A curly apostrophe read as a straight one is
// not a different label.
func canonicalPunctuation(r rune) rune {
	switch r {
	case '‘', '’', '‛', 'ʼ': // ‘ ’ ‛ ʼ
		return '\''
	case '“', '”', '‟': // “ ” ‟
		return '"'
	case '‐', '‑', '‒', '–', '—', '−': // ‐ ‑ ‒ – — −
		return '-'
	case ' ', ' ': // non-breaking spaces that survived IsSpace on some engines
		return ' '
	}
	return r
}

// ── window and application evidence ───────────────────────────────────────────

// Window is evidence that a top-level window exists.
//
// Windows are evidence too. Treating them as ambient truth rather than as something
// a source REPORTED is how the window system quietly became a second, unaccountable
// perception path — one whose failures nothing recorded and whose contribution no
// element could cite.
type Window struct {
	ObservationID directorapi.ObservationID
	From          Source
	At            time.Time

	Detail directorapi.Window
}

func (w Window) ID() directorapi.ObservationID { return w.ObservationID }
func (w Window) Source() Source                { return w.From }
func (w Window) Timestamp() time.Time          { return w.At }
func (w Window) Bounds() directorapi.Rect      { return w.Detail.Bounds }
func (w Window) Confidence() float64           { return 1 }
func (w Window) Kind() Kind                    { return WindowObservation }

// Application is evidence that an application is running and, usually, in front.
type Application struct {
	ObservationID directorapi.ObservationID
	From          Source
	At            time.Time

	Detail directorapi.Application
	// Active is whether this application was the foreground one.
	Active bool
	// WindowID is its window, when the source knew which.
	WindowID directorapi.WindowID
}

func (a Application) ID() directorapi.ObservationID { return a.ObservationID }
func (a Application) Source() Source                { return a.From }
func (a Application) Timestamp() time.Time          { return a.At }
func (a Application) Bounds() directorapi.Rect      { return directorapi.Rect{} }
func (a Application) Confidence() float64           { return 1 }
func (a Application) Kind() Kind                    { return ApplicationObservation }

// ── narrowing a mixed set ─────────────────────────────────────────────────────

// Elements returns the element evidence in obs, in order.
func Elements(obs []Observation) []directorapi.Observation {
	out := make([]directorapi.Observation, 0, len(obs))
	for _, o := range obs {
		if e, ok := o.(Element); ok {
			out = append(out, e.Raw)
		}
	}
	return out
}

// Texts returns the text evidence in obs, in order.
func Texts(obs []Observation) []Text {
	var out []Text
	for _, o := range obs {
		if t, ok := o.(Text); ok {
			out = append(out, t)
		}
	}
	return out
}

// Windows returns the window evidence in obs, in order.
func Windows(obs []Observation) []directorapi.Window {
	var out []directorapi.Window
	for _, o := range obs {
		if w, ok := o.(Window); ok {
			out = append(out, w.Detail)
		}
	}
	return out
}

// ActiveApplication returns the application evidence marked active, if any.
func ActiveApplication(obs []Observation) (Application, bool) {
	for _, o := range obs {
		if a, ok := o.(Application); ok && a.Active {
			return a, true
		}
	}
	return Application{}, false
}

// CountByKind tallies obs by kind, for diagnostics.
func CountByKind(obs []Observation) map[Kind]int {
	out := map[Kind]int{}
	for _, o := range obs {
		out[o.Kind()]++
	}
	return out
}

// CountBySource tallies obs by source, for diagnostics.
func CountBySource(obs []Observation) map[Source]int {
	out := map[Source]int{}
	for _, o := range obs {
		out[o.Source()]++
	}
	return out
}

// ── visual state evidence ─────────────────────────────────────────────────────

// VisualStateKind is what a visual observation claims about appearance.
//
// Every one of these describes how something LOOKS or whether it CHANGED. None of them
// says what a thing is, and that omission is the whole design: there is deliberately no
// "button", "link", "menu item" or "actionable" kind, so a visual provider has no way
// to express the claim that would be dangerous. Pixels may report state; only structure
// may report that something is a control.
type VisualStateKind string

const (
	// Appearance states, attachable to an element whose ROLE already permits them.
	VisualSelected           VisualStateKind = "selected"
	VisualChecked            VisualStateKind = "checked"
	VisualPressed            VisualStateKind = "pressed"
	VisualExpanded           VisualStateKind = "expanded"
	VisualCollapsed          VisualStateKind = "collapsed"
	VisualDisabledAppearance VisualStateKind = "disabled_appearance"
	VisualLoading            VisualStateKind = "loading"
	VisualProgress           VisualStateKind = "progress"

	// Change states, which are about a REGION rather than about a control. These are
	// what verification consumes: they answer "did anything happen there", which is a
	// different question from "what is there".
	VisualRegionChanged       VisualStateKind = "region_changed"
	VisualRegionUnchanged     VisualStateKind = "region_unchanged"
	VisualRegionStillChanging VisualStateKind = "region_still_changing"
	VisualOverlayAppeared     VisualStateKind = "overlay_appeared"
	VisualOverlayDisappeared  VisualStateKind = "overlay_disappeared"
)

// StateFlag maps a visual kind onto the state flag it would fill, and whether the
// observation asserts it true or false. Change kinds map to nothing: a region that
// changed is evidence about an event, not about a control's state.
func (k VisualStateKind) StateFlag() (flag string, value bool, ok bool) {
	switch k {
	case VisualSelected:
		return directorapi.StateSelected, true, true
	case VisualChecked:
		return directorapi.StateChecked, true, true
	case VisualPressed:
		return directorapi.StatePressed, true, true
	case VisualExpanded:
		return directorapi.StateExpanded, true, true
	case VisualCollapsed:
		return directorapi.StateExpanded, false, true
	case VisualDisabledAppearance:
		return directorapi.StateEnabled, false, true
	case VisualLoading:
		return directorapi.StateBusy, true, true
	}
	return "", false, false
}

// AboutChange reports whether the kind describes a region's change rather than a
// control's state.
func (k VisualStateKind) AboutChange() bool {
	switch k {
	case VisualRegionChanged, VisualRegionUnchanged, VisualRegionStillChanging,
		VisualOverlayAppeared, VisualOverlayDisappeared:
		return true
	}
	return false
}

// VisualState is evidence about how a region of the screen looks, or that it changed.
//
// The third kind of evidence, after structure and text. Like text it can never create
// an element; unlike text it does not even carry a string a planner could match on.
// What it carries is a claim about appearance and a bounded region it applies to.
type VisualState struct {
	ObservationID directorapi.ObservationID
	CycleID       CycleID
	ProviderID    string

	VisualKind VisualStateKind
	From       Source
	At         time.Time
	Box        directorapi.Rect
	Score      float64

	WindowID      directorapi.WindowID
	ApplicationID string

	// TargetHint is the element the provider was LOOKING AT when it produced this.
	// A hint and not an assignment: whether the evidence actually belongs to that
	// element is fusion's decision, and a provider that could assign it would be
	// deciding what its own pixels mean.
	TargetHint *directorapi.ElementID

	Metadata map[string]string
}

func (v VisualState) ID() directorapi.ObservationID { return v.ObservationID }
func (v VisualState) Source() Source                { return v.From }
func (v VisualState) Timestamp() time.Time          { return v.At }
func (v VisualState) Bounds() directorapi.Rect      { return v.Box }
func (v VisualState) Confidence() float64           { return v.Score }
func (v VisualState) Kind() Kind                    { return VisualObservation }

// Reference is the provenance entry for this observation within a cycle.
func (v VisualState) Reference(cycle CycleID) directorapi.ObservationReference {
	return Reference(v, cycle)
}

// Visuals returns the visual evidence in obs, in order.
func Visuals(obs []Observation) []VisualState {
	var out []VisualState
	for _, o := range obs {
		if v, ok := o.(VisualState); ok {
			out = append(out, v)
		}
	}
	return out
}
