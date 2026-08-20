package values

import (
	"fmt"
	"time"
)

// Provenance and consumption: how a value came to exist, and where it went.
//
//	Diagnostics may explain a value without exposing its contents.
//
// Everything in this file is DESIGNED to be safe rather than redacted to be safe, and
// the difference matters. A struct with a plaintext field has to be scrubbed at every
// boundary and will eventually be missed at one of them; a struct with no such field
// cannot leak whatever anyone does with it. So none of the types here has anywhere to
// put the content — not a Text, not a Value, not an any.

// SourceKind names WHERE a value came from, as a closed vocabulary.
//
// Distinct from the free-text Source, which reads well in a sentence ("the text_field
// \"Customer\"") but cannot be switched on. An explanation needs both: the vocabulary to
// decide what to say, and the prose to say it.
type SourceKind string

const (
	SourceLiteral      SourceKind = "literal"
	SourceControlValue SourceKind = "control_value"
	SourceSelectedText SourceKind = "selected_text"
	SourceClipboard    SourceKind = "clipboard"
	SourceWindowTitle  SourceKind = "window_title"
)

// Describe renders the source for a person.
func (s SourceKind) Describe() string {
	switch s {
	case SourceLiteral:
		return "the text you gave"
	case SourceControlValue:
		return "editable control value"
	case SourceSelectedText:
		return "selected text"
	case SourceClipboard:
		return "the clipboard"
	case SourceWindowTitle:
		return "active window title"
	}
	return string(s)
}

// Provenance is the recorded account of one capture.
//
// Every field is filled from what actually happened. An explanation that invented a
// detail would be worse than one that omitted it, because it would be believed — so a
// field the capture could not establish stays empty and the renderer says nothing
// rather than guessing.
type Provenance struct {
	// SourceKind is the closed vocabulary; Source is the readable phrase.
	SourceKind SourceKind `json:"source_kind"`
	Source     string     `json:"source,omitempty"`
	// Application and Role describe the control a value was read from, where there
	// was one. Empty for the clipboard, a window title or a literal.
	Application string `json:"application,omitempty"`
	Role        string `json:"role,omitempty"`
	// Method names how the read was performed — "the control's value API", "clipboard
	// probe with restoration". This is where the strategy ladder becomes explainable.
	Method string `json:"method,omitempty"`
	// Provider is the component that answered, when one can be named.
	Provider string `json:"provider,omitempty"`
	// ClipboardRestored is set only for a clipboard-assisted read. Three states, and
	// the third is the point: nil means the clipboard was never borrowed at all, which
	// is different from borrowed-and-restored and from borrowed-and-lost.
	ClipboardRestored *bool `json:"clipboard_restored,omitempty"`
	// StepID and StepIndex locate the capture in its program.
	StepID    string `json:"step_id,omitempty"`
	StepIndex int    `json:"step_index,omitempty"`
	// Confidence is the resolver's confidence in the control that was read, 0 when
	// no control was involved.
	Confidence float64 `json:"confidence,omitempty"`
}

// Consumption is one step's use of a value.
//
// Recorded only for a use that actually reached execution. A failed lookup is traced —
// it is a real event and worth seeing — but recording it here would make the history
// claim the value was used when it was not, and "which steps consumed it" would stop
// being answerable.
type Consumption struct {
	StepID    string `json:"step_id,omitempty"`
	StepIndex int    `json:"step_index,omitempty"`
	// Operation is the semantic operation that received it ("set_text").
	Operation  string    `json:"operation"`
	ConsumedAt time.Time `json:"consumed_at"`
	// Outcome is how the consuming step ended.
	Outcome string `json:"outcome,omitempty"`
}

// ValueSnapshot is one value, described safely.
//
// A COPY, always. The active environment is live — a program is still running against
// it — and handing out anything that aliased it would let a diagnostic reader mutate
// execution state, or read a value mid-write.
type ValueSnapshot struct {
	Name       string        `json:"name"`
	Kind       Kind          `json:"kind"`
	Visibility Visibility    `json:"visibility"`
	ByteLength int           `json:"byte_length"`
	CapturedAt time.Time     `json:"captured_at"`
	Verified   bool          `json:"verified"`
	Provenance Provenance    `json:"provenance"`
	ConsumedBy []Consumption `json:"consumed_by,omitempty"`
	// Preview is the content for PUBLIC values only, bounded and escaped. For anything
	// else it is the redaction marker.
	//
	// Produced by String(), never by Plaintext(): the redaction is the type's, so this
	// field cannot become a leak by someone editing this file alone.
	Preview string `json:"preview"`
}

// EnvironmentSnapshot is the whole active environment, described safely.
type EnvironmentSnapshot struct {
	ProgramID string          `json:"program_id,omitempty"`
	TakenAt   time.Time       `json:"taken_at"`
	Values    []ValueSnapshot `json:"values"`
	// Cleared reports that the program has ended and the values are gone. A snapshot
	// of a finished environment is empty AND says why it is empty.
	Cleared bool `json:"cleared"`
}

// Find returns one value's snapshot by name.
func (s EnvironmentSnapshot) Find(name string) (ValueSnapshot, bool) {
	normalised, err := NormalizeName(name)
	if err != nil {
		return ValueSnapshot{}, false
	}
	for _, v := range s.Values {
		if v.Name == normalised {
			return v, true
		}
	}
	return ValueSnapshot{}, false
}

// previewLimit bounds a public preview.
//
// Bounded because a diagnostic must not be able to dump 64 KiB into a terminal or a
// JSON payload, and because a preview is for recognising a value rather than for
// reading it.
const previewLimit = 60

// preview renders a value's content for diagnostics, honouring its visibility.
//
// Goes through String(), which redacts anything not public. Calling Plaintext() here
// and deciding for ourselves would put a second copy of the redaction rule in the one
// place most likely to be read by a person.
func (v Value) preview() string {
	s := v.String()
	if v.visibility != VisibilityNormal {
		return s
	}
	if v.Empty() {
		return `"" (empty, and verified as empty)`
	}
	return fmt.Sprintf("%q", truncate(s, previewLimit))
}
