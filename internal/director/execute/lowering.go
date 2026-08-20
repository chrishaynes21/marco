package execute

import (
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Lowerings is how the executor learns what Marco its actions became.
//
// The executor calls the Actuator through directorapi interfaces and never sees an
// Operation, so the generated source has to come back some other way. This is that
// way: the composition root hands the executor a look-up over the same recorder the
// lowering history uses.
//
// Optional. Without it the trace simply lacks the Marco, which is a poorer diagnostic
// rather than a broken one.
type Lowerings interface {
	// Last returns the most recently lowered operation, false if there is none.
	Last() (marcoexec.Result, bool)
}

// attachLowering copies the compile and runtime detail of the Marco that just ran onto
// an execution outcome.
//
// Called immediately after the Actuator returns, so "most recent" is unambiguous: the
// pipeline runs one action at a time under a lock, and the lowering that just happened
// is the one this action caused.
func (e *Executor) attachLowering(out *directorapi.ExecutionOutcome) {
	if e.Lowerings == nil {
		return
	}
	res, ok := e.Lowerings.Last()
	if !ok {
		return
	}
	// Already redacted by marcoexec before it was recorded, so a credential name
	// cannot reach an action record through here.
	out.Marco = res.Source
	out.Capabilities = res.Capabilities

	switch res.Status {
	case marcoexec.StatusCompileFailed:
		out.CompileStatus, out.RuntimeStatus = "failed", "not reached"
	case marcoexec.StatusUnsupported:
		out.CompileStatus, out.RuntimeStatus = "not attempted", "not reached"
	case marcoexec.StatusContextChanged:
		out.CompileStatus, out.RuntimeStatus = "not attempted", "refused: the target context changed"
	case marcoexec.StatusCompleted:
		out.CompileStatus, out.RuntimeStatus = "ok", "ok"
	default:
		out.CompileStatus, out.RuntimeStatus = "ok", string(res.Status)
	}
}

// loweringTrace is the one-line summary the execution trace shows.
func loweringTrace(out directorapi.ExecutionOutcome) string {
	if len(out.Capabilities) == 0 {
		return ""
	}
	parts := []string{"marco " + strings.Join(out.Capabilities, ", ")}
	if out.CompileStatus != "" {
		parts = append(parts, "compile "+out.CompileStatus)
	}
	if out.RuntimeStatus != "" {
		parts = append(parts, "runtime "+out.RuntimeStatus)
	}
	return strings.Join(parts, " · ")
}
