package values

import (
	"fmt"
	"strings"
)

// ParamCapture and ParamInput are the Intent.Parameters keys that carry the typed
// data-flow structures.
//
// Declared HERE rather than in the intent package because both the parser that writes
// them and the program validator that reads them import this package, and neither
// imports the other. A key defined in one of them would have to be duplicated as a
// string literal in the other, and the two copies would drift the first time one was
// renamed.
//
// The values behind these keys are the typed structs below — a values.Capture and a
// values.Input — never loose strings.
const (
	ParamCapture = "capture"
	ParamInput   = "value_input"
)

// Input is the text an operation will use: either a LITERAL the user wrote or a
// REFERENCE to something the program captured.
//
//	Exactly one is populated. Never both, never neither.
//
// This type is the reason `type ${customer}` is not implemented by string replacement.
// A replacement has to decide, at planning time, what the text will be — and at
// planning time the capture has not happened yet, so the only thing it can substitute
// is a guess or an empty string. Carrying the reference through to execution means the
// value is read at the moment it is used, from the environment of the program that
// captured it, and a missing one stops the program instead of typing nothing.
type Input struct {
	// Literal is the text itself, when the user wrote it out.
	Literal *string `json:"literal,omitempty"`
	// Reference names a program-local captured value, when they wrote ${name}.
	Reference *Reference `json:"reference,omitempty"`
}

// LiteralInput is text the user supplied directly.
func LiteralInput(text string) Input { return Input{Literal: &text} }

// ReferenceInput names a captured value.
func ReferenceInput(name string) Input { return Input{Reference: &Reference{Name: name}} }

// IsReference reports whether this input must be looked up before it can be used.
func (i Input) IsReference() bool { return i.Reference != nil }

// Validate enforces exactly-one.
//
// Checked rather than assumed, because both-populated and neither-populated are both
// reachable by a caller building the struct directly, and both are silent: neither
// would fail until the text was already going into a control.
func (i Input) Validate() error {
	switch {
	case i.Literal == nil && i.Reference == nil:
		return fmt.Errorf("values: an input must be either literal text or a ${value} reference")
	case i.Literal != nil && i.Reference != nil:
		return fmt.Errorf("values: an input cannot be both literal text and a ${value} reference")
	}
	if i.Reference != nil {
		if _, err := NormalizeName(i.Reference.Name); err != nil {
			return err
		}
	}
	return nil
}

// ErrTransformation reports a reference the Director will not evaluate.
//
// Concatenation, formatting and defaults are all deliberately absent, and this is what
// says so out loud. Interpolating "Dear ${name}," would be easy and would immediately
// raise the question of what happens when the capture is empty, or unknown, or secret —
// three answers this milestone does not have. Refusing is the honest position.
type ErrTransformation struct {
	Phrase string
	Reason string
}

func (e *ErrTransformation) Error() string {
	return fmt.Sprintf("%q asks for %s, which this Director does not do; "+
		"a value may be used whole (\"type ${customer}\") but not combined or reshaped",
		e.Phrase, e.Reason)
}

// ParseInput reads text that is either a whole ${reference} or a literal.
//
// The whole-token rule is the point. "${customer}" is a reference; "Dear ${customer}"
// is a concatenation and is REFUSED rather than interpolated; "$5.00" and "$save" are
// ordinary text and stay ordinary text, because the value namespace is `${}` and the
// object namespace is `$`, and neither may fall back to the other.
func ParseInput(text string) (Input, error) {
	trimmed := strings.TrimSpace(text)
	if ref, ok := ParseReference(trimmed); ok {
		return Input{Reference: &ref}, nil
	}
	if strings.Contains(trimmed, "${") {
		// It reaches for a value but is not one. Named explicitly: silently treating it
		// as literal text would type the braces into the user's document, and silently
		// interpolating it would implement a feature that does not exist.
		return Input{}, &ErrTransformation{
			Phrase: trimmed,
			Reason: "a value combined with other text",
		}
	}
	return Input{Literal: &text}, nil
}

// Resolve produces the concrete value this input stands for.
//
// Called at STEP EXECUTION TIME, never at planning time. A literal resolves to itself;
// a reference is looked up in the environment of the program running right now. The
// lookup can fail, and when it does the caller stops — which is the behaviour that
// distinguishes a value that could not be captured from a value that was empty.
func (i Input) Resolve(env *Environment) (Value, error) {
	if err := i.Validate(); err != nil {
		return Value{}, err
	}
	if i.Literal != nil {
		// A literal is already verified: the user wrote it, so there is nothing to read
		// and nothing that could have failed to read.
		return New(KindText, *i.Literal, "the request", Classify(KindText, *i.Literal))
	}
	v, err := env.Resolve(i.Reference.Name)
	if err != nil {
		return Value{}, err
	}
	if err := v.TextCompatible(); err != nil {
		return Value{}, fmt.Errorf("%s cannot be used here: %w", i.Reference, err)
	}
	return v, nil
}

// Describe renders the input for a plan preview, without resolving it.
//
// A reference shows as the reference. Showing a resolved value in a preview would be a
// lie twice over: the value does not exist yet, and printing it would leak whatever it
// turns out to be.
func (i Input) Describe() string {
	switch {
	case i.Reference != nil:
		return i.Reference.String()
	case i.Literal != nil:
		return fmt.Sprintf("%q", truncate(*i.Literal, 40))
	}
	return "(nothing)"
}
