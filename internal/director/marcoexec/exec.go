package marcoexec

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Status is what became of one lowered operation.
//
// Distinct values rather than a bool, because the caller's next move differs for each
// and collapsing them into FAILED destroys exactly the information that decides it. A
// compile failure means the Director asked for something Marco cannot express and the
// desktop was never touched; a runtime failure means the act ran and refused; a
// context change means the operation was correct and the world moved. Reporting all
// three as "failed" is how a system ends up retrying the one that must not be retried.
type Status string

const (
	// StatusCompleted: compiled, ran, resolved ok.
	StatusCompleted Status = "completed"
	// StatusUnsupported: the Director cannot express this in Marco at all. Refused
	// before generation; nothing was compiled and nothing ran.
	StatusUnsupported Status = "unsupported"
	// StatusCompileFailed: legal-looking source Marco rejected. NOTHING was executed —
	// this is the compile-before-mutation guarantee reporting itself.
	StatusCompileFailed Status = "compile_failed"
	// StatusRuntimeFailed: Marco ran it and a capability resolved failed.
	StatusRuntimeFailed Status = "runtime_failed"
	// StatusCancelled: the context ended. Distinct from a failure because nothing was
	// wrong — the user or a stop asked for it.
	StatusCancelled Status = "cancelled"
	// StatusContextChanged: the intended target was not what the foreground held when
	// it came time to act, so the operation was REFUSED rather than sent somewhere
	// else. See Guard.
	StatusContextChanged Status = "target_context_changed"
)

// Result is one lowered operation's full record.
type Result struct {
	Operation Operation `json:"operation"`
	Status    Status    `json:"status"`
	// Source is the exact Marco that was compiled, redacted when the operation
	// carried credential material. Diagnostic and export material — never the
	// semantic key, which stays the Operation.
	Source string `json:"source"`
	// Capabilities are the act-and-capability pairs the source invokes.
	Capabilities []string `json:"capabilities,omitempty"`
	// Diagnostic is the compiler's or runtime's message, empty on success.
	Diagnostic string `json:"diagnostic,omitempty"`
	// Output is what the program logged.
	Output string `json:"output,omitempty"`
	// Returned holds each capability's return value, for the reads.
	Returned map[string]directorapi.MarcoValue `json:"returned,omitempty"`
}

// Failed reports whether the operation did not complete.
func (r Result) Failed() bool { return r.Status != StatusCompleted }

// Summary is a one-line human form. Safe to log: Source is already redacted.
func (r Result) Summary() string {
	s := fmt.Sprintf("%s via %s — %s", r.Operation.Kind,
		strings.Join(r.Capabilities, ", "), r.Status)
	if r.Diagnostic != "" {
		s += ": " + r.Diagnostic
	}
	return s
}

// Guard decides whether the world is still the one an operation was planned against.
//
// Optional, and only consulted for operations that can land in the wrong application.
// It exists because of a real incident: an edit resolved against Notepad executed
// while Discord held the foreground, and the text went into a message box. A previous
// focus operation having succeeded proves nothing about the instant of execution —
// the user alt-tabs, a notification steals focus, an installer pops up.
type Guard interface {
	// Confirm reports whether the intended window is still the right one to act on.
	// The string explains a refusal, for the trace.
	Confirm(ctx context.Context, window string) (ok bool, why string, err error)
}

// Executor lowers operations and runs them through Marco.
//
// The one place a Director intention becomes an effect. Everything else in the
// Director asks this.
type Executor struct {
	runner directorapi.MarcoRunner
	guard  Guard
	// recorder receives every result, for the action graph and diagnostics.
	recorder func(Result)
}

// New builds an executor over a runner.
func New(runner directorapi.MarcoRunner) *Executor { return &Executor{runner: runner} }

// WithGuard returns an executor that confirms the target context before acting on
// operations that could land in the wrong application.
func (e *Executor) WithGuard(g Guard) *Executor {
	if e == nil {
		return nil
	}
	c := *e
	c.guard = g
	return &c
}

// WithRecorder returns an executor that reports every result.
func (e *Executor) WithRecorder(fn func(Result)) *Executor {
	if e == nil {
		return nil
	}
	c := *e
	c.recorder = fn
	return &c
}

// contextSensitive reports whether landing in the wrong application would be harmful.
//
// Typing, secrets, keys and clipboard pastes all deliver INPUT to whatever holds
// focus, so a foreground change between resolution and execution sends them
// somewhere else entirely, silently and successfully. Drag is here because it starts
// with a press wherever the pointer is. Window and clipboard-read operations name
// their target explicitly and are unaffected.
func contextSensitive(o Operation) bool {
	switch o.Kind {
	case KindType, KindTypeSecret, KindKey, KindDrag:
		return true
	}
	return false
}

// Do lowers one operation and executes it.
//
// The order is the guarantee:
//
//	validate → confirm the context → lower → lex → parse → graph → compile → run
//
// Everything before "run" can refuse, and refusing at any of them means the desktop
// was never touched. There is no partial execution: a program either compiles whole
// or does not run at all.
func (e *Executor) Do(ctx context.Context, o Operation) (Result, error) {
	res := Result{Operation: o}
	res.Capabilities, _ = o.Capabilities()

	finish := func(err error) (Result, error) {
		if e != nil && e.recorder != nil {
			e.recorder(res)
		}
		return res, err
	}

	if e == nil || e.runner == nil {
		res.Status, res.Diagnostic = StatusUnsupported, "no Marco runner is wired"
		return finish(errors.New("marcoexec: " + res.Diagnostic))
	}
	if err := ctx.Err(); err != nil {
		res.Status, res.Diagnostic = StatusCancelled, err.Error()
		return finish(err)
	}

	if err := o.Validate(); err != nil {
		// Refused before generation. Nothing compiled, nothing ran.
		res.Status, res.Diagnostic = StatusUnsupported, err.Error()
		return finish(err)
	}

	if contextSensitive(o) && e.guard != nil {
		ok, why, err := e.guard.Confirm(ctx, o.Window)
		if err != nil {
			res.Status, res.Diagnostic = StatusContextChanged,
				"the intended target could not be confirmed: "+err.Error()
			return finish(errors.New(res.Diagnostic))
		}
		if !ok {
			// The whole point. Refuse rather than deliver input to whatever happens to
			// be in front — which succeeds, verifies against nothing, and is invisible.
			res.Status, res.Diagnostic = StatusContextChanged, why
			return finish(errors.New(res.Diagnostic))
		}
	}

	source, err := Lower(o)
	if err != nil {
		res.Status, res.Diagnostic = StatusUnsupported, err.Error()
		return finish(err)
	}
	// Redacted from the moment it is stored. The unredacted source exists only as the
	// argument to Run, and only for as long as that call takes.
	res.Source = Redact(o, source)

	out, runErr := e.runner.Run(ctx, o.Describe(), source)
	res.Output, res.Returned = out.Output, out.Returned

	switch {
	case runErr == nil && len(out.Failed) == 0:
		res.Status = StatusCompleted
		return finish(nil)

	case ctx.Err() != nil:
		res.Status, res.Diagnostic = StatusCancelled, ctx.Err().Error()
		return finish(ctx.Err())

	case runErr != nil && isCompileError(runErr):
		// The guarantee, reporting itself: Marco rejected the program, so the runtime
		// was never entered and the desktop was never touched.
		res.Status, res.Diagnostic = StatusCompileFailed, redactErr(o, runErr)
		return finish(errors.New(res.Diagnostic))

	case runErr != nil:
		res.Status, res.Diagnostic = StatusRuntimeFailed, redactErr(o, runErr)
		return finish(errors.New(res.Diagnostic))

	default:
		res.Status = StatusRuntimeFailed
		res.Diagnostic = strings.Join(out.Failed, ", ") + " resolved failed"
		if v, ok := out.Returned[out.Failed[0]]; ok && v.Error != "" {
			res.Diagnostic = redactText(o, v.Error)
		}
		return finish(errors.New(res.Diagnostic))
	}
}

// Preview lowers an operation WITHOUT executing it.
//
// Backs `director lower`. It runs validation and generation and stops — no compile, no
// runtime, no host. A preview that executed would make the command unusable for the
// thing it is for, which is looking at what the Director would do before it does it.
func Preview(o Operation) (Result, error) {
	res := Result{Operation: o}
	res.Capabilities, _ = o.Capabilities()
	source, err := Lower(o)
	if err != nil {
		res.Status, res.Diagnostic = StatusUnsupported, err.Error()
		return res, err
	}
	res.Source = Redact(o, source)
	return res, nil
}

// isCompileError reports whether a run failed before execution.
//
// driver decorates lex, parse, graph and compile failures with those words. Matching
// on them is coarser than a typed error would be, but the driver returns a decorated
// error rather than a type, and the distinction is worth having even approximately:
// getting it wrong in the safe direction reports a compile failure as a runtime one,
// which is misleading but not dangerous.
func isCompileError(err error) bool {
	msg := err.Error()
	for _, marker := range []string{"lex:", "parse:", "graph:", "compile:", "has no capability", "unknown subject"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// redactErr keeps credential material out of an error message.
func redactErr(o Operation, err error) string { return redactText(o, err.Error()) }

// redactText removes a secret's NAME from text that will be logged or stored.
//
// The value never reaches this process, so there is nothing to strip; the name is
// removed because a credential name can itself be sensitive, and because an error
// quoting the program would otherwise reproduce the line the redaction just removed.
func redactText(o Operation, s string) string {
	if !o.Secret() {
		return s
	}
	if o.SecretRef != "" {
		s = strings.ReplaceAll(s, o.SecretRef, "<redacted>")
	}
	return s
}

// Guarded reports whether this executor confirms the target context before acting.
//
// Exists so a composition root can ASSERT it wired the guarded copy. WithGuard returns
// a copy, so keeping the original in scope makes it easy to pass the unguarded one by
// accident — which silently removes foreground protection from that path and looks
// exactly like working code. That happened once; this is how it gets caught.
func (e *Executor) Guarded() bool { return e != nil && e.guard != nil }
