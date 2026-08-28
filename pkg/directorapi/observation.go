package directorapi

import (
	"strings"
	"time"
)

// ObservationSource names where a piece of evidence came from.
//
// The order of the constants is the SOURCE PRIORITY LADDER, strongest first. It
// encodes the central rule of the Director's perception: structured knowledge beats
// pixel interpretation, always. A native integration or an accessibility tree tells
// you an element IS a button and IS disabled; OCR tells you some pixels look like
// the word "Save"; a vision model tells you something button-shaped is probably
// there. Those are not equally good reasons to click.
type ObservationSource string

const (
	// SourceNative is a first-party integration with the application itself — its
	// own API, extension or scripting interface. Ground truth where it exists.
	SourceNative ObservationSource = "native"
	// SourceDOM is a browser's document model, via an extension or devtools bridge.
	SourceDOM ObservationSource = "dom"
	// SourceAccessibility is the OS accessibility tree (UI Automation on Windows,
	// AX on macOS, AT-SPI on Linux). The workhorse: real roles, labels and states.
	SourceAccessibility ObservationSource = "accessibility"
	// SourceWindowSystem is window and process metadata — titles, bounds, PIDs,
	// monitors. Coarse, but reliable and nearly free.
	SourceWindowSystem ObservationSource = "window_system"
	// SourceOCR is recognised on-screen text with bounding boxes.
	SourceOCR ObservationSource = "ocr"
	// SourceVision is a learned UI-element detector: boxes with class labels and no
	// semantics beyond them.
	SourceVision ObservationSource = "vision"
	// SourceModel is a general vision-language model reasoning over a screenshot.
	// The last resort, used only when the structured ladder came up empty.
	SourceModel ObservationSource = "model"
	// SourcePlugin is a skill-contributed observation. Its trust is the skill's
	// trust, so it deliberately has no fixed rung on the ladder — SourceRank returns
	// a middling value and the policy engine weighs the skill itself.
	SourcePlugin ObservationSource = "plugin"
)

// SourceRank is the source's position on the priority ladder: higher is stronger
// evidence. Fusion uses it to decide which source's account of a disputed field
// wins, and ranking uses it to prefer a well-attested candidate.
//
// It is a total order over sources and NOT a probability — a rank-7 accessibility
// observation is not "7x" a rank-3 OCR one. Anything wanting a weight should map
// these to weights explicitly, so the mapping is visible and tunable rather than
// implied by the constant.
func SourceRank(s ObservationSource) int {
	switch s {
	case SourceNative:
		return 8
	case SourceDOM:
		return 7
	case SourceAccessibility:
		return 6
	case SourceWindowSystem:
		return 5
	case SourcePlugin:
		return 4
	case SourceOCR:
		return 3
	case SourceVision:
		return 2
	case SourceModel:
		return 1
	}
	return 0
}

// Structured reports whether the source describes objects rather than interpreting
// pixels. The Director prefers a structured source for any decision that matters,
// and policy requires one (or corroboration) before anything destructive.
func (s ObservationSource) Structured() bool {
	switch s {
	case SourceNative, SourceDOM, SourceAccessibility, SourceWindowSystem:
		return true
	}
	return false
}

// ObservationKind is what an observation is a report ABOUT.
//
// Sources describe different sorts of thing and should not be forced into one shape.
// Accessibility reports controls; OCR reports strings with boxes and no claim about
// what they belong to; the window system reports windows and the running application.
// Fusion needs to know which it is holding before it can decide what to do with it —
// a text observation may REINFORCE an element, where an element observation from a
// second source may MERGE with it.
type ObservationKind string

const (
	// ObservationElement is a report about a UI control: a role, usually a label,
	// often state. The only kind any provider emits today.
	ObservationElement ObservationKind = "element"
	// ObservationText is recognised text and where it is, with no claim about what it
	// labels. The shape OCR will arrive in.
	ObservationText ObservationKind = "text"
	// ObservationWindow is a report about a top-level window.
	ObservationWindow ObservationKind = "window"
	// ObservationApplication is a report about a running application.
	ObservationApplication ObservationKind = "application"
	// ObservationVisual is a report about how a region LOOKS, or that it changed. It
	// never says what a thing is.
	ObservationVisual ObservationKind = "visual"
)

// ObservationReference points back at one piece of evidence.
//
// It carries the source and the kind alongside the id because provenance is read long
// after the observations themselves have been discarded — they are ephemeral, and a
// bare id would by then name nothing. This is what lets "why did you think that was a
// button" be answered from the element alone.
type ObservationReference struct {
	Observation ObservationID     `json:"observation"`
	Source      ObservationSource `json:"source"`
	Kind        ObservationKind   `json:"kind,omitempty"`
	// Cycle is the observation cycle this evidence came from. Elements outlive
	// cycles, so an element whose provenance names an old cycle is one whose evidence
	// has not been renewed.
	Cycle string    `json:"cycle,omitempty"`
	At    time.Time `json:"at,omitempty"`
}

// Provenance is why an element is believed to exist.
//
// The distinction this type exists to protect: observations are EVIDENCE and the
// element is BELIEF. Fusion is the only thing allowed to turn one into the other, and
// this is the receipt it leaves behind. An element that cannot say why it exists is
// one whose selection cannot be explained or audited after the fact.
//
// Usually one source today. The shape is plural because the whole point of the
// observation graph is that an OCR read and a vision box will later reinforce the
// same accessibility element rather than each inventing a duplicate of it.
type Provenance struct {
	Sources []ObservationReference `json:"sources,omitempty"`
}

// Add records another piece of evidence, ignoring a duplicate observation id.
func (p *Provenance) Add(ref ObservationReference) {
	for _, got := range p.Sources {
		if got.Observation == ref.Observation {
			return
		}
	}
	p.Sources = append(p.Sources, ref)
}

// Len is how many observations contributed.
func (p Provenance) Len() int { return len(p.Sources) }

// Has reports whether s contributed.
func (p Provenance) Has(s ObservationSource) bool {
	for _, ref := range p.Sources {
		if ref.Source == s {
			return true
		}
	}
	return false
}

// IDs lists the contributing observation ids, in contribution order.
func (p Provenance) IDs() []ObservationID {
	out := make([]ObservationID, 0, len(p.Sources))
	for _, ref := range p.Sources {
		out = append(out, ref.Observation)
	}
	return out
}

// SourceList is the distinct contributing sources, strongest first — the summary
// ranking and logging want without walking every reference.
func (p Provenance) SourceList() []ObservationSource {
	var out []ObservationSource
	for _, ref := range p.Sources {
		seen := false
		for _, got := range out {
			if got == ref.Source {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, ref.Source)
		}
	}
	return out
}

// Observation is one raw report from one source at one instant, before fusion.
//
// Observations are the Director's evidence, and they are never destroyed by fusion —
// an Element points back at the observations that produced it, which is what makes
// its selection explainable after the fact ("chosen because accessibility reported a
// button labelled Save and OCR read Save at the same place").
type Observation struct {
	ID     ObservationID     `json:"id"`
	Source ObservationSource `json:"source"`
	// Kind is what this reports about. Empty means ObservationElement, so every
	// fixture recorded before kinds existed still reads correctly.
	Kind ObservationKind `json:"kind,omitempty"`
	// Timestamp is when this was observed, not when the snapshot was assembled.
	// Sources run at different speeds; a slow OCR pass can carry a stale box into an
	// otherwise fresh snapshot and fusion needs to see that.
	Timestamp time.Time `json:"timestamp"`

	// WindowID is the window this was observed in, when known.
	WindowID WindowID `json:"window_id,omitempty"`

	// Scope says whether this describes one validated window or the desktop at large.
	//
	// Typed rather than inferred from the source, because the two are not the same
	// question: the window-system provider reports monitor topology (global) and window
	// bounds (target-scoped) in one pass, and a guard that keyed on provider name would
	// have to be wrong about one of them. The zero value is target-scoped — see
	// ObservationScope.
	Scope ObservationScope `json:"scope,omitempty"`

	// Target is which validated window generation this evidence belongs to.
	//
	// Set by the ORCHESTRATION layer from the target it already validated, never
	// rediscovered by a provider. Absent on global observations, and absent on an
	// untargeted cycle where there is no target to belong to.
	Target *TargetProvenance `json:"target,omitempty"`

	Role  ElementRole `json:"role,omitempty"`
	Label string      `json:"label,omitempty"`
	Value string      `json:"value,omitempty"`
	// Description is supplementary text (an accessible description, a tooltip, help
	// text). Weaker evidence than Label for matching, but frequently the only text a
	// bare icon has — which makes it the difference between a findable control and an
	// unnameable one.
	Description string `json:"description,omitempty"`
	// Text is the raw recognised or extracted text, kept separate from Label because
	// OCR reports strings with no claim about what they label.
	Text   string `json:"text,omitempty"`
	Bounds Rect   `json:"bounds"`

	// State fields. Pointers, because "not reported" and "reported false" are
	// genuinely different: OCR cannot tell you a button is enabled, and fusion must
	// not read its silence as "disabled".
	Enabled  *bool `json:"enabled,omitempty"`
	Visible  *bool `json:"visible,omitempty"`
	Focused  *bool `json:"focused,omitempty"`
	Selected *bool `json:"selected,omitempty"`

	// Confidence is the source's own confidence in this observation, 0..1 — an OCR
	// engine's per-word score, a detector's box score. Sources that don't produce one
	// (accessibility) report 1.0; that is a statement about the source's certainty,
	// not about its rung on the ladder, which SourceRank carries separately.
	Confidence float64 `json:"confidence"`

	// NativeID is the source's own stable identifier where it has one — a UIA
	// AutomationId, a DOM id or selector. Gold for identity matching across
	// snapshots: when present and unchanged, it settles continuity outright.
	NativeID string `json:"native_id,omitempty"`

	// ParentNativeID links to the parent's NativeID, letting a tree be reconstructed
	// without the provider having to serialise nesting.
	ParentNativeID string `json:"parent_native_id,omitempty"`

	Attributes map[string]any `json:"attributes,omitempty"`

	// Resource is the object this control stands for, when the source genuinely knows.
	// Absent for almost everything — see ResourceIdentity for why an absent resource and
	// an empty one must not be the same thing.
	Resource *ResourceIdentity `json:"resource,omitempty"`

	// Entity is what this control is within the APPLICATION's own model — a slot, a
	// container, a queue, a meter. Contributed by a source that understands the
	// application, and absent for every source that does not.
	//
	// Distinct from Resource, and the pair is not redundant: a resource exists outside
	// the application and can be checked against the operating system, an entity exists
	// inside it and can be checked only against what the interface shows. See
	// EntityIdentity.
	Entity *EntityIdentity `json:"entity,omitempty"`
}

// ObservationKindOf is o's kind, defaulting to ObservationElement.
func ObservationKindOf(o Observation) ObservationKind {
	if o.Kind == "" {
		return ObservationElement
	}
	return o.Kind
}

// Reference is the provenance entry for this observation, in the given cycle.
func (o Observation) Reference(cycle string) ObservationReference {
	return ObservationReference{
		Observation: o.ID,
		Source:      o.Source,
		Kind:        ObservationKindOf(o),
		Cycle:       cycle,
		At:          o.Timestamp,
	}
}

// AccessibilitySnapshot is one accessibility provider's complete report.
type AccessibilitySnapshot struct {
	Timestamp    time.Time     `json:"timestamp"`
	Observations []Observation `json:"observations"`

	// ObservedWindow and ObservedApp are what the provider reports it ACTUALLY walked,
	// as distinct from what it was asked to walk.
	//
	// The distinction is the whole basis of the provenance guard. A request records
	// intent; these record evidence. If the requested handle died and the bridge fell
	// back to the foreground window, these name the window it really read — which is
	// the only way a caller can discover that the answer describes something else.
	//
	// Empty when the source cannot say. Empty is NOT agreement: a provider that cannot
	// name what it observed has not established provenance, and the guard treats that
	// as unproven rather than as a match.
	ObservedWindow WindowID `json:"observed_window,omitempty"`
	ObservedApp    string   `json:"observed_app,omitempty"`
	// Windows the provider was able to describe. A provider that can enumerate
	// windows saves the Director a separate window-system pass.
	Windows []Window `json:"windows,omitempty"`
	// Partial is true when the walk was cut short — a depth or node cap, a timeout,
	// an app that refused. The snapshot is still usable; it just cannot be read as
	// "the Save button does not exist".
	Partial bool   `json:"partial,omitempty"`
	Reason  string `json:"reason,omitempty"`

	// ResourceNote is why no element in this snapshot carries a backing resource, when
	// the provider tried to establish one and could not.
	//
	// A snapshot with no resources is the ordinary case and says nothing; a snapshot
	// where the provider LOOKED and refused is a different thing, and the refusal is the
	// most useful sentence a person debugging "why won't it rename this?" can read.
	// Without it the two are indistinguishable from outside the provider.
	ResourceNote string `json:"resource_note,omitempty"`
}

// TextObservation is one OCR result: a recognised string and where it is.
type TextObservation struct {
	Text       string  `json:"text"`
	Bounds     Rect    `json:"bounds"`
	Confidence float64 `json:"confidence"`
	// Line groups words that belong to the same rendered line, so a caller can
	// reassemble a phrase the engine split. -1 when the engine doesn't report it.
	Line int `json:"line,omitempty"`
}

// VisualObservation is one detector result: a box, a class and a score.
type VisualObservation struct {
	Label      string      `json:"label"`
	Role       ElementRole `json:"role,omitempty"`
	Bounds     Rect        `json:"bounds"`
	Confidence float64     `json:"confidence"`
}

// ScreenImage is a captured region of the screen. Bounds gives the capture's
// position in absolute desktop coordinates, which is what lets a provider's
// image-local box be translated back into a clickable point.
//
// PNG is the encoded image. It is passed by value through provider interfaces and
// should be treated as sensitive: screenshots are not persisted by default and are
// never sent to a remote model without explicit consent (see the privacy rules in
// the Director spec).
type ScreenImage struct {
	Bounds Rect   `json:"bounds"`
	PNG    []byte `json:"png,omitempty"`
}

// VisionRequest asks a vision provider a specific question about an image. Prompt is
// only meaningful for a general model; a fixed detector ignores it and returns
// everything it found.
type VisionRequest struct {
	Image  ScreenImage `json:"image"`
	Prompt string      `json:"prompt,omitempty"`
	// Roles narrows the request to particular kinds of element, where the provider
	// can honour it.
	Roles []ElementRole `json:"roles,omitempty"`
}

// VisionResult is a vision provider's answer.
type VisionResult struct {
	Elements []VisualObservation `json:"elements,omitempty"`
	// Text is a free-form answer from a general model, empty for a plain detector.
	Text string `json:"text,omitempty"`
}

// ObservationScope says what an observation describes.
//
// # Why the zero value is target-scoped
//
// Fail-safe. A provider added later that forgets to classify its evidence gets treated as
// describing the target, which means it must carry target provenance and will be REJECTED
// by the fusion guard if it does not. The opposite default would let a new source bypass
// the guard silently by omission, which is the failure mode the guard exists to prevent.
//
// Global evidence therefore has to say so deliberately. That is the correct direction for
// the cost to fall: declaring "this is desktop-wide" is a claim somebody makes on purpose.
type ObservationScope string

const (
	// ScopeTarget describes one validated window. The zero value.
	ScopeTarget ObservationScope = ""
	// ScopeGlobal describes the desktop at large — monitor topology and the like — and
	// legitimately carries no window generation.
	ScopeGlobal ObservationScope = "global"
)

// TargetProvenance identifies the validated window an observation belongs to.
//
// # What is deliberately absent
//
// No HWND, no native accessibility id, no bounds. A handle is not an identity — it is
// reused, and the whole reason this type exists is that "the same handle" and "the same
// window" are different claims (see the window identity ADR). Bounds are not identity
// either: a window that moved is the same window, and a different window at the same
// coordinates is not.
//
// What remains is what survives a redraw, a move and a resize, and what changes when the
// window is genuinely replaced.
type TargetProvenance struct {
	Application string `json:"application,omitempty"`
	// ProcessID is included where the platform reports one. Zero means "not known",
	// which is different from "known to be zero" — the guard only compares it when both
	// sides have it.
	ProcessID uint32 `json:"process_id,omitempty"`
	// WindowGeneration is the windowref lifecycle epoch. A recycled handle gets a new
	// generation, which is what makes stale evidence detectable at all.
	WindowGeneration uint64 `json:"window_generation,omitempty"`
}

// Known reports whether this provenance identifies anything.
//
// A zero TargetProvenance is not evidence of belonging to the current target; it is the
// absence of a claim, and the guard treats it as such.
func (t TargetProvenance) Known() bool {
	return t.WindowGeneration != 0 || t.Application != ""
}

// Matches reports whether observed provenance belongs to the expected target.
//
// Process is compared only when BOTH sides report one: a provider that cannot see a process
// id has not contradicted anything by omitting it, and rejecting on that would discard good
// evidence for a fact nobody claimed.
func (t TargetProvenance) Matches(expected TargetProvenance) bool {
	if t.WindowGeneration != expected.WindowGeneration {
		return false
	}
	if t.Application != "" && expected.Application != "" &&
		!strings.EqualFold(t.Application, expected.Application) {
		return false
	}
	if t.ProcessID != 0 && expected.ProcessID != 0 && t.ProcessID != expected.ProcessID {
		return false
	}
	return true
}

// TargetScoped reports whether this observation describes the validated target.
func (o Observation) TargetScoped() bool { return o.Scope != ScopeGlobal }

// OnlyDescribesPixels reports that every source which reported this element describes what the
// screen LOOKS like, and none of them can say how to operate it.
//
// The provenance-side half of the affordance/capability distinction — see
// Actionability.Targetable and ActuatingSource.
//
// # Why this is phrased as a denial
//
// It answers false for an element with no provenance at all, and that is deliberate rather than
// an oversight. "Nobody recorded where this came from" and "only a camera saw it" are different
// claims: the first describes a hand-built query, a capability pack's enrichment or a fixture,
// and refusing those would break every caller that constructs an element rather than observing
// one — measured, five of them, the moment it was tried the other way round.
//
// So the rule denies on POSITIVE evidence: every source is a pixel source, therefore nothing
// here has claimed a mechanism. That is the exact shape of the risk it exists for.
func (p Provenance) OnlyDescribesPixels() bool {
	if len(p.Sources) == 0 {
		return false
	}
	for _, ref := range p.Sources {
		if ActuatingSource(ref.Source) {
			return false
		}
	}
	return true
}
