package directorapi

import "context"

// ValueProvider sets and reads a control's value through its own native API.
//
// Kept separate from AccessibilityProvider because these are ACTS, not perception,
// and the split is the same one that keeps Focus honest: reading the tree is safe
// and repeatable, writing to a control is neither. A provider may serve one and not
// the other, and a caller must be able to depend on the reading half without
// acquiring the writing half by accident.
type ValueProvider interface {
	// SetValue sets the control's value directly. It returns ErrValueUnsupported
	// when the control has no such API — a normal, expected answer that means "fall
	// back", distinct from an error that means "stop".
	SetValue(ctx context.Context, window WindowID, nativeID, value string) (string, error)
	// GetValue reads the control's current value. The bool reports whether a value
	// could be read at all: a field holding "" and a field that cannot be read look
	// identical in a bare string, and only one of them proves a Clear worked.
	GetValue(ctx context.Context, window WindowID, nativeID string) (string, bool, error)
}

// ErrValueUnsupported is returned by SetValue when the control does not implement a
// value API, or implements it read-only.
//
// A sentinel rather than a string match, because the caller's next move depends on
// it: unsupported means try the next strategy, anything else means the bridge is
// broken and trying harder will only make a mess. Collapsing the two would turn
// every read-only field into an outage, and every outage into a burst of typing.
type ValueUnsupportedError struct{ Reason string }

func (e *ValueUnsupportedError) Error() string {
	if e.Reason == "" {
		return "the control has no value API"
	}
	return e.Reason
}

// Clipboard reads and writes the system clipboard.
//
// An interface rather than a direct call so the clipboard can be faked in tests
// without a real one — and, more importantly, so that BORROWING it is a thing the
// Director does through a component that can guarantee it gives it back. Destroying
// a user's clipboard as a side effect of pasting text is not an acceptable cost of
// automation.
type Clipboard interface {
	Read(ctx context.Context) (ClipboardContents, error)
	Write(ctx context.Context, text string) error
}

// ClipboardContents is what the clipboard holds.
//
// Three states, not two, because the third one changes what a borrower may do. An
// EMPTY clipboard can be borrowed and given back. A clipboard holding an IMAGE cannot
// — nothing here can save or reproduce it, so the only way to preserve it is to leave
// it alone. Through the text format alone both read as an empty string, which is why
// Empty is carried separately rather than inferred from it.
type ClipboardContents struct {
	Text   string
	IsText bool
	Empty  bool
}

// MarcoRunner executes a Marco program and reports what its capabilities returned.
//
// This is the Director's ONLY route to a desktop effect that Marco can express. It
// takes a program — source that must lex, parse, build a graph, and compile — rather
// than an act name and a payload, and that difference is the whole boundary: a
// payload sent straight to a Host skips the compiler, so a capability the language
// does not export is indistinguishable from one it does. A program cannot skip it.
//
// The Director does not import the driver, the runtime, or any host. It hands over
// source and receives results; cmd/director wires the implementation.
type MarcoRunner interface {
	// Run compiles and executes a program. A compile error is returned BEFORE any
	// desktop mutation, which is what makes an unsupported operation safe to attempt.
	Run(ctx context.Context, name, program string) (MarcoResult, error)
}

// MarcoResult is what one Marco run produced.
type MarcoResult struct {
	// Output is everything the program logged.
	Output string
	// Returned holds the data each capability returned, keyed "Act's Capability".
	//
	// Needed because some capabilities are READS — ClipboardGet, GetValue — and the
	// Director needs their values in Go. Marco has them in a frame; this carries them
	// out without the program having to log them and the Director having to parse
	// text back.
	Returned map[string]MarcoValue
	// Failed names the capabilities that returned a failed status, in order.
	Failed []string
}

// MarcoValue is one capability's return, decoded from Marco's Value.
type MarcoValue struct {
	// Text is the value when the capability returned a text.
	Text string
	// Fields are the members when it returned a set.
	Fields map[string]string
	// Status is the status the capability resolved with ("ok", "failed", …).
	Status string
	// Error is the message when Status is "failed".
	Error string
}

// Field reads one member of a returned set.
func (v MarcoValue) Field(name string) (string, bool) {
	s, ok := v.Fields[name]
	return s, ok
}
