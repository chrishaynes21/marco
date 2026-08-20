package program

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Static binding analysis.
//
//	The validator proves the binding WILL exist if the earlier steps succeed.
//	Runtime execution still proves the capture actually did.
//
// Two different claims, and keeping them apart is what makes this useful. Planning
// cannot know whether a field will turn out to be readable — that is a fact about the
// screen at a moment that has not happened yet. What it CAN know is whether the
// program even makes sense: whether a step reads a name some earlier step will bind.
//
// That check has to happen before the first step, because the alternative is
// discovering at step 3 that ${email} was never captured, having already typed into two
// controls. There is no undo for that.

// Binds reports the value a step captures, empty if it captures none.
func Binds(in directorapi.Intent) (values.Capture, bool) {
	c, ok := in.Parameters[values.ParamCapture].(values.Capture)
	return c, ok
}

// Reads reports the value a step consumes, if it consumes one.
func Reads(in directorapi.Intent) (values.Reference, bool) {
	input, ok := in.Parameters[values.ParamInput].(values.Input)
	if !ok || input.Reference == nil {
		return values.Reference{}, false
	}
	return *input.Reference, true
}

// ErrUseBeforeCapture names a value used before the program captures it.
//
// Its own type so the message is exact and testable. "Unknown value" would be wrong
// here: the name IS captured, just not yet, and telling the user it is unknown would
// send them off to check a spelling that is correct.
type ErrUseBeforeCapture struct {
	Name string
	Step int // 1-based
}

func (e *ErrUseBeforeCapture) Error() string {
	return fmt.Sprintf("Value %q is used before it is captured.", e.Name)
}

// ValidateDataFlow checks that every value is captured before it is used.
//
// Walks the steps in order against an empty namespace, exactly as execution will. The
// result is a proof about the PROGRAM rather than about the world, which is why it can
// run before anything is observed.
func ValidateDataFlow(p Program) error {
	// bound maps a name to the KIND the capture will produce. The kind is known
	// statically — a clipboard capture yields a clipboard value whatever the clipboard
	// turns out to hold — so type compatibility is provable here too.
	bound := map[string]values.Kind{}

	for i, s := range p.Steps {
		where := fmt.Sprintf("step %d (%q)", i+1, s.Phrase)

		// READS are checked BEFORE the step's own binding is added. A step cannot
		// consume the value it captures: "remember the clipboard as clip" does not also
		// type it, and a step that appeared to do both would have to decide whether the
		// read happened before or after the write.
		if ref, reads := Reads(s.Operation); reads {
			kind, ok := bound[ref.Name]
			if !ok {
				return &ErrUseBeforeCapture{Name: ref.Name, Step: i + 1}
			}
			// Compatibility, statically. A future kind with no text rule is caught here
			// rather than after two controls have already been edited.
			probe, err := values.New(kind, "", "static check", values.VisibilityNormal)
			if err == nil {
				if err := probe.TextCompatible(); err != nil {
					return fmt.Errorf("%s uses %s, but %w", where, ref, err)
				}
			}
		}

		if c, binds := Binds(s.Operation); binds {
			if err := c.Validate(); err != nil {
				return fmt.Errorf("%s: %w", where, err)
			}
			if _, taken := bound[c.Name]; taken {
				// Values are immutable, so a second capture under the same name could
				// only mean one of them is silently discarded.
				return fmt.Errorf("%s captures %q, which an earlier step already captured; "+
					"values are immutable, so use a different name", where, c.Name)
			}
			if len(bound) >= values.MaxValues {
				return fmt.Errorf("%s would capture more than %d values in one program",
					where, values.MaxValues)
			}
			bound[c.Name] = c.Kind.Produces()
		}
	}
	return nil
}
