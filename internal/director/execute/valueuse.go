package execute

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Consuming a captured value.
//
//	The value is read at STEP EXECUTION TIME, from the environment of the program
//	running right now.
//
// This is one function and it is deliberately tiny, because the interesting property is
// WHERE it is called rather than what it does. It runs after the intent is known and
// before the target is resolved, the plan is built or any policy is evaluated — so
// everything downstream receives concrete text and does not know a value was involved.
//
// That is what keeps `${customer}` from being a special case in the planner, the policy
// engine, the lowering and the verifier. Each of those would need its own answer to
// "what if the value is missing?", and four answers to one question is how they end up
// differing.
//
// It is emphatically NOT string substitution. The reference is carried as typed data
// all the way to here; substitution would have to decide at parse time what the text
// will be, and at parse time the capture has not happened.

// applyValue replaces a ${name} reference with the value the program captured.
//
// Returns an error the caller turns into a stopped step. A missing value STOPS: typing
// nothing into a field and reporting success is the failure this whole design exists to
// rule out.
func (p *Pipeline) applyValue(in *directorapi.Intent, env *values.Environment) (values.Value, bool, error) {
	input, ok := in.Parameters[values.ParamInput].(values.Input)
	if !ok || !input.IsReference() {
		return values.Value{}, false, nil
	}
	if env == nil || env.Cleared() {
		// A reference outside a running program. Named as a value rather than reported
		// as an empty one, so a user who typed ${save} for $save is told which namespace
		// they missed.
		return values.Value{}, true, &values.ErrUnknownValue{Name: input.Reference.Name}
	}

	v, err := input.Resolve(env)
	if err != nil {
		return values.Value{}, true, err
	}

	// The plaintext is handed over exactly once, here, to the field the executor will
	// carry to the host. Every diagnostic downstream renders the value through its own
	// String(), which redacts — see redactedIntent, which is what the trace records.
	in.Text = v.Plaintext()
	return v, true, nil
}

// describeValueUse is the safe trace line for a consumed value.
//
// Name, kind, visibility and length — every one safe at any visibility. The content is
// never here, which is what lets this be logged without a decision at the call site.
func describeValueUse(ref values.Reference, v values.Value) string {
	return fmt.Sprintf("%s → %s, %s, %d bytes", ref, v.Kind(), v.Visibility(), v.Len())
}
