// Package program is the Director's model of a bounded semantic program.
//
// The governing rule:
//
//	A program is not a batch of actions. It is a sequence of independently
//	observed, independently compiled, independently verified semantic
//	operations.
//
// The distinction is the whole package. A batch is submitted once and comes back
// once; nothing between the first action and the last can notice that the world
// moved, that a dialog appeared, or that the second target no longer exists. A
// program hands control back to the Director after every step, which is what makes
// "open File, then click Save" work at all — Save does not exist until File is open,
// so it cannot be resolved before File is clicked.
//
// What this package deliberately is NOT. There are no variables, no loops, no
// branches and no collections, and FailurePolicy has exactly one value. Those are not
// omissions to be filled in later by widening this type: each of them changes what a
// program can mean, and a sequence whose meaning is fixed at planning time can be
// shown to a person in full before it runs. `director plan` depends on that.
package program

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ProgramID identifies one program run.
type ProgramID string

// StepID identifies a step within a program.
type StepID string

// Status is where a program has got to.
type Status string

const (
	// StatusPlanned: validated, not started.
	StatusPlanned Status = "planned"
	// StatusRunning: a step is executing or being verified.
	StatusRunning Status = "running"
	// StatusCompleted: every step verified.
	StatusCompleted Status = "completed"
	// StatusFailed: a step failed and the program stopped there.
	StatusFailed Status = "failed"
	// StatusStopped: cancelled at a safe boundary between steps.
	StatusStopped Status = "stopped"
	// StatusNeedsClarification: paused mid-program with its history intact. The
	// completed steps stay completed — resuming continues, it does not restart.
	StatusNeedsClarification Status = "needs_clarification"
	// StatusRejected: validation refused the whole request before anything ran.
	StatusRejected Status = "rejected"
)

// Terminal reports whether a program has finished for good.
func (s Status) Terminal() bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusStopped, StatusRejected:
		return true
	}
	return false
}

// FailurePolicy is what to do when a step does not verify.
//
// One value, on purpose. Retry belongs to the single-step layer, which already knows
// when repeating is safe (an unchanged screen) and when it is not (an edit that may
// have landed). Continue-on-failure would mean running step 3 against a world step 2
// failed to produce. Rollback would need an inverse for every operation, and there is
// no inverse for "press Enter".
type FailurePolicy string

// Stop is the only failure policy: the program halts and later steps do not run.
const Stop FailurePolicy = "stop"

// VerificationRequirement says how strongly a step must prove itself.
type VerificationRequirement string

const (
	// VerifyRequired: the step must verify or the program stops. The default, and
	// the only value that makes the conjunction rule meaningful.
	VerifyRequired VerificationRequirement = "required"
	// VerifyBestEffort: an inconclusive verification does not stop the program.
	// Reserved for operations that genuinely cannot be proved from the World Model —
	// a selection, a copy — and never chosen to paper over a weak check.
	VerifyBestEffort VerificationRequirement = "best_effort"
)

// Step is one semantic operation in a program.
//
// Operation is an INTENT, not a plan and not a resolved target. That is the load-
// bearing choice in this file: resolution has to happen against the world as it is
// when the step runs, so a step that carried a resolved element would be carrying a
// handle from before the previous step changed the screen.
type Step struct {
	ID StepID `json:"id"`
	// Operation is the unresolved semantic operation. Resolved at execution.
	Operation directorapi.Intent `json:"operation"`
	// Phrase is the user's own words for this step, for the trace and for a
	// clarification question that has to quote them back.
	Phrase string `json:"phrase"`

	// Preconditions must hold before the step runs; Postconditions after it.
	// Evaluated as semantic waits, never as sleeps.
	Preconditions  []directorapi.Condition `json:"preconditions,omitempty"`
	Postconditions []directorapi.Condition `json:"postconditions,omitempty"`

	Verification  VerificationRequirement `json:"verification"`
	FailurePolicy FailurePolicy           `json:"failure_policy"`
}

// Program is a bounded, validated sequence.
type Program struct {
	ID     ProgramID `json:"id"`
	Goal   string    `json:"goal"`
	Steps  []Step    `json:"steps"`
	Status Status    `json:"status"`
}

// MaxSteps bounds a program.
//
// Ten because a request a person says in one breath does not decompose further than
// that, and a "plan" longer than it is far more likely to be a decomposition bug than
// a genuine intention. Exceeding it REJECTS: truncating would run a prefix of
// something the user did not ask for, which is worse than doing nothing.
const MaxSteps = 10

// controlFlow are the words that mean this request is not a sequence.
//
// Rejected rather than ignored. "Click Save if the dialog appeared" is a conditional,
// and executing it as an unconditional click is the single most dangerous thing this
// package could do: it performs an action the user explicitly gated.
var controlFlow = []string{
	"if ", "when ", "unless ", "while ", "until ", "for each ", "foreach ",
	"otherwise", " else ", "repeat until", "as long as", "in case",
}

// placeholder markers are the signs of an unfilled template.
//
// "${" was on this list and is deliberately no longer: it is now the value-reference
// token, so "type ${customer}" is a real instruction rather than a template nobody
// filled in. Everything here is still an unfilled template, and a program containing
// one is refused because running it would type the placeholder itself.
var placeholders = []string{"{{", "}}", "<placeholder>"}

// Validate checks a whole program before any of it runs.
//
// All of it, before any of it. A partially validated plan that starts executing and
// then discovers step 4 is unsupported has already done steps 1 to 3, and there is no
// undo — so the only safe place to refuse is before the first step.
func Validate(p Program) error {
	if len(p.Steps) == 0 {
		return fmt.Errorf("the request produced no steps")
	}
	if len(p.Steps) > MaxSteps {
		return fmt.Errorf("the request needs %d steps and the limit is %d; "+
			"it is rejected rather than shortened, because running part of it "+
			"would do something you did not ask for", len(p.Steps), MaxSteps)
	}
	for i, s := range p.Steps {
		if err := validateStep(i, s); err != nil {
			return err
		}
	}
	// Data flow LAST, because it is the only check that reasons about the steps
	// together rather than each on its own. A step that is individually fine can still
	// read a value nothing captures.
	return ValidateDataFlow(p)
}

func validateStep(i int, s Step) error {
	where := fmt.Sprintf("step %d (%q)", i+1, s.Phrase)

	lower := strings.ToLower(" " + s.Phrase + " ")
	for _, w := range controlFlow {
		if strings.Contains(lower, w) {
			return fmt.Errorf("%s reads as a condition (%q), and this Director executes "+
				"sequences rather than branches; it is rejected rather than run unconditionally",
				where, strings.TrimSpace(w))
		}
	}
	for _, ph := range placeholders {
		if strings.Contains(s.Phrase, ph) {
			return fmt.Errorf("%s still contains the placeholder %q", where, ph)
		}
	}
	if s.FailurePolicy != Stop {
		return fmt.Errorf("%s has failure policy %q; the only supported policy is %q",
			where, s.FailurePolicy, Stop)
	}
	if s.Operation.Kind != directorapi.IntentAct {
		return fmt.Errorf("%s is not something the Director can do: %s", where,
			firstNonEmpty(s.Operation.Ambiguity, "it was not understood"))
	}
	if !Supported(s.Operation.Verb) {
		return fmt.Errorf("%s asks for %q, which is not an operation this Director "+
			"implements", where, s.Operation.Verb)
	}
	return nil
}

// ValidateBound checks everything Validate checks, and that every reference which
// declared it needs a binding actually carries one.
//
//	A deictic action requiring a binding must fail validation if the binding is
//	absent. Do not silently fall back from typed binding to generic focus.
//
// Separate from Validate rather than folded into it, because the two answer different
// questions at different moments. Validate asks "is this a program?", which `director
// explain goal` needs to answer about a request nobody has observed the screen for.
// ValidateBound asks "may this run?", and the honest answer for an unbound deictic
// program is no.
//
// Everything on the execution path calls this. Nothing on the display path does, and a
// display-only expansion is therefore unrunnable by construction rather than by
// convention — which is the difference between a rule and a hope.
func ValidateBound(p Program) error {
	if err := Validate(p); err != nil {
		return err
	}
	for i, s := range p.Steps {
		for _, ref := range s.Operation.Targets {
			if !ref.RequiresBinding || ref.BindingID != "" {
				continue
			}
			return fmt.Errorf("step %d (%q) points at %q without having resolved what it "+
				"points at; it is refused rather than aimed at whatever holds focus, "+
				"which would act on a different object than the one you meant",
				i+1, s.Phrase, ref.Phrase)
		}
	}
	return nil
}

// supportedVerbs are the operations a program step may contain.
//
// Only what is already implemented. A verb that reached planning without an
// implementation would fail at the last possible moment — after earlier steps had
// already changed the user's screen — which is precisely what whole-program validation
// exists to prevent.
var supportedVerbs = map[string]bool{
	"click":       true,
	"focus":       true,
	"move_window": true,
	"edit":        true,
	// Capture is a program step like any other: it observes, resolves and clarifies
	// through the same path, and a variable it stores becomes available to the steps
	// that follow. It performs no desktop mutation, which is why a failed capture
	// stops the program without having changed anything.
	"remember_target": true,
	// A collection capture binds a query and touches nothing; an iteration is one step
	// containing many independently verified member actions.
	"capture_collection": true,
	"for_each":           true,
	// Capturing a VALUE is a program step for the same reasons, and one more: it is the
	// only step that produces something later steps depend on. It mutates nothing, so a
	// failed capture stops the program with the screen untouched.
	"capture_value": true,
	"wait":          true,
	// A semantic action — expand, select, submit, refresh. One verb for the whole
	// vocabulary, matching how the planner carries it: the plan's SHAPE is identical
	// for every one of them, and the particular verb travels in the intent's
	// parameters where the planner reads it back as a typed kind.
	//
	// Admitted here as a single entry rather than thirty-three because the question
	// this map answers is "can a program step do this?", and the answer is the same for
	// all of them. Which ones can be PERFORMED on a given control is the ladder's
	// question, asked at execution against the real capabilities.
	intent.SemanticVerb: true,
}

// Supported reports whether a verb may appear in a program.
func Supported(verb string) bool { return supportedVerbs[verb] }

// SupportedVerbs lists them, for diagnostics and error messages.
func SupportedVerbs() []string {
	out := make([]string, 0, len(supportedVerbs))
	for v := range supportedVerbs {
		out = append(out, v)
	}
	return out
}

// Describe renders the program for a person.
func (p Program) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d step", p.Goal, len(p.Steps))
	if len(p.Steps) != 1 {
		b.WriteString("s")
	}
	b.WriteString(")")
	return b.String()
}

// Context is what a later step may inherit from an earlier one.
//
// Deliberately narrow. "Clear it, then type Director" needs to know what "it" was; that
// is a real and narrow need. General conversational memory is a different feature with
// different failure modes — it lets a request three turns ago silently decide where
// today's text goes — and this is not it.
//
// Note what the first three fields and the fourth have in common and where they differ.
// All four are program-local. But the target fields carry IDENTITY, and identity is
// re-proved every step: the next step re-resolves rather than reusing a handle. Values
// are the opposite — they are DATA, captured once, and re-reading them would defeat the
// purpose. That is the whole distinction this milestone rests on, and the two live side
// by side here precisely so it stays visible.
type Context struct {
	// LastResolvedTarget is the element the previous step acted on.
	LastResolvedTarget *directorapi.ResolvedTarget
	// LastWindow is the window it was in.
	LastWindow *directorapi.Window
	// LastActionNode is the node the previous step produced, for parent links.
	LastActionNode string

	// Values are the facts this program has captured so far.
	//
	// One environment per program execution, created before step 1 and shared by every
	// later step. It rides on the context rather than on the pipeline because the
	// context is what already survives a clarification pause: the paused program keeps
	// it, so an answer submitted from an entirely different client process resumes with
	// the captured values still bound. A pipeline field would belong to the Director
	// instead of to the program, and would leak into whatever ran next.
	//
	// Never written to disk. There is no loader and no path — see values.Environment.
	Values *values.Environment `json:"-"`

	// Collections are the bounded semantic sets this program has captured.
	//
	// Alongside Values and for the same reasons: program-local, never persisted, and
	// carried on the context so a clarification pause keeps them. Separate from Values
	// because they answer a different question — WHICH OBJECTS, plural, rather than
	// WHAT INFORMATION — and because their members are re-resolved every iteration
	// while a value never is.
	Collections *collections.Environment `json:"-"`

	// CollectionResume is a clarification answer for a paused iteration. Set by the
	// resume path and consumed by the first member the iteration resolves, so it
	// narrows exactly one member and then stops existing.
	CollectionResume *CollectionResume `json:"-"`
}

// EnsureValues gives the context an environment, creating one on first use.
//
// Called at program start. Idempotent, which is what makes a RESUME correct: the paused
// program brings its environment back with it, and a second call must not replace it
// with an empty one — that would silently unbind everything the completed steps
// captured and turn a working program into "Unknown value".
func (c *Context) EnsureValues() *values.Environment {
	if c.Values == nil {
		c.Values = values.NewEnvironment()
	}
	return c.Values
}

// EnsureCollections gives the context a collection environment, creating one on first
// use. Idempotent for the same reason EnsureValues is: a resume must not replace what
// the completed steps captured.
func (c *Context) EnsureCollections() *collections.Environment {
	if c.Collections == nil {
		c.Collections = collections.NewEnvironment()
	}
	return c.Collections
}

// pronouns are the only back-references a step may use.
//
// A closed set, matched against the WHOLE target phrase. "the same control" refers
// back; "the control on the left" does not, and treating it as a back-reference would
// silently act on the wrong thing.
var pronouns = map[string]bool{
	"it": true, "that": true, "this": true,
	"that field": true, "that box": true, "that window": true,
	"the same control": true, "the same one": true, "the same field": true,
}

// IsBackReference reports whether a target phrase refers to the previous step's target.
func IsBackReference(phrase string) bool {
	return pronouns[strings.ToLower(strings.TrimSpace(phrase))]
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// Verb is the step's operation word, for the trace.
func (s Step) Verb() string {
	if s.Operation.Verb != "" {
		return s.Operation.Verb
	}
	return s.Phrase
}
