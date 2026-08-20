package values

import "fmt"

// Capture is one program step that stores a fact.
//
//	A capture is not a desktop mutation. Nothing is clicked, nothing is typed,
//	nothing changes. It observes and remembers.
//
// That is why a capture creates no Action Graph node: a node claims the computer was
// touched, and every later count of "what did the Director do" would be wrong. It is
// also why a failed capture is safe — it stops the program having changed nothing.
//
// A CLOSED typed model rather than a parameter map. The fields a capture needs depend
// entirely on its kind, and a map would let a literal capture carry a target, or a
// clipboard capture carry a literal, with nothing to notice until execution.
type Capture struct {
	Kind CaptureKind `json:"kind"`
	// Name is what the value will be bound as, already normalised.
	Name string `json:"name"`
	// Literal is the text, for CaptureLiteral only.
	Literal *string `json:"literal,omitempty"`
}

// CaptureKind is where a value comes from.
//
// The source is not an implementation detail: it decides what evidence proves the
// capture, how sensitive the result is, and whether a desktop read is needed at all.
type CaptureKind string

const (
	// CaptureSelectedText reads what the user has selected.
	CaptureSelectedText CaptureKind = "selected_text"
	// CaptureControlValue reads a control's own value.
	CaptureControlValue CaptureKind = "control_value"
	// CaptureClipboard reads the clipboard WITHOUT changing it.
	CaptureClipboard CaptureKind = "clipboard"
	// CaptureWindowTitle reads a window title from the world already observed.
	CaptureWindowTitle CaptureKind = "window_title"
	// CaptureLiteral binds text the user wrote. No observation, no desktop read.
	CaptureLiteral CaptureKind = "literal"
)

// Produces is the kind of value this capture yields.
func (k CaptureKind) Produces() Kind {
	switch k {
	case CaptureSelectedText, CaptureLiteral:
		return KindText
	case CaptureControlValue:
		return KindControlValue
	case CaptureClipboard:
		return KindClipboard
	case CaptureWindowTitle:
		return KindWindowTitle
	}
	return ""
}

// NeedsTarget reports whether a control has to be resolved first.
//
// Only the two that read a specific control. The clipboard is not a control, a window
// title comes from the world snapshot, and a literal needs no world at all — running
// an accessibility walk for "remember \"John Smith\" as customer" would be pure cost.
func (k CaptureKind) NeedsTarget() bool {
	return k == CaptureSelectedText || k == CaptureControlValue
}

// NeedsWorld reports whether the capture must observe before it can run.
func (k CaptureKind) NeedsWorld() bool { return k != CaptureLiteral }

// Describe names the capture for a person.
func (k CaptureKind) Describe() string {
	switch k {
	case CaptureSelectedText:
		return "the selected text"
	case CaptureControlValue:
		return "the control's value"
	case CaptureClipboard:
		return "the clipboard"
	case CaptureWindowTitle:
		return "the window title"
	case CaptureLiteral:
		return "the text you gave"
	}
	return string(k)
}

// Validate checks that exactly the fields this kind needs are present.
//
// Both directions. A missing literal would capture nothing; a literal on a clipboard
// capture means the parser produced something incoherent, and letting it through would
// bind whichever field the executor happened to read.
func (c Capture) Validate() error {
	if _, err := NormalizeName(c.Name); err != nil {
		return err
	}
	if c.Kind.Produces() == "" {
		return fmt.Errorf("values: %q is not a kind of capture this Director performs", c.Kind)
	}
	if c.Kind == CaptureLiteral {
		if c.Literal == nil {
			return fmt.Errorf("values: capturing a literal needs the text to capture")
		}
		if len(*c.Literal) > MaxTextLength {
			return fmt.Errorf("values: the literal is %d characters and the limit is %d",
				len(*c.Literal), MaxTextLength)
		}
		return nil
	}
	if c.Literal != nil {
		return fmt.Errorf("values: a %s capture reads from the desktop and cannot carry "+
			"literal text", c.Kind)
	}
	return nil
}

// CaptureFailure is why a capture produced no value.
//
// The reasons stay DISTINCT because the user's next move differs for each: nothing was
// selected is a thing they can fix, the control has no readable value is not, and a
// refusal on a password field is deliberate and should not look like a malfunction.
type CaptureFailure string

const (
	// FailureAbsent: the source was observed and holds nothing to read.
	FailureAbsent CaptureFailure = "absent"
	// FailureUnreadable: the source exists but its value could not be read.
	FailureUnreadable CaptureFailure = "unreadable"
	// FailureUnknown: no provider could give an honest answer.
	FailureUnknown CaptureFailure = "unknown"
	// FailureAmbiguous: several things matched and none was chosen.
	FailureAmbiguous CaptureFailure = "ambiguous"
	// FailureUnsafe: reading it is refused — a password, a protected control.
	FailureUnsafe CaptureFailure = "unsafe"
	// FailureUnsupported: the source holds something this Director cannot represent,
	// such as a picture on the clipboard.
	FailureUnsupported CaptureFailure = "unsupported"
	// FailureTimedOut: the read did not finish in time.
	FailureTimedOut CaptureFailure = "timed_out"
	// FailureCancelled: the user stopped it.
	FailureCancelled CaptureFailure = "cancelled"
)
