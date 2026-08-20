// Package demo records what a user DEMONSTRATED and extracts a reusable procedure from
// it.
//
// The governing rules:
//
//	Record semantics, never mechanics.
//	Extract intent, not clicks.
//	Every learned procedure must still compile into legal Marco.
//	Recording never bypasses verification.
//	The learned result must be explainable.
//
// # What this changes
//
// Everything below this package expects the user to describe a procedure. The goal layer
// closed the gap between "describe every step" and "describe the outcome"; this closes the
// one between "describe the outcome" and "show me once". The user performs a task, and
// what is kept is not the coordinates, the timings, the window handles or the keystrokes —
// it is the semantic execution: which verbs, aimed at which semantic targets, with which
// waits, and what verification made of each.
//
// # What a demonstration is NOT
//
// Not a screen recording. Not a coordinate log. Nothing here captures pixels, handles,
// runtime ids, element ids, OCR text or plaintext secrets, and the tests assert that over
// the serialised form rather than trusting the code to be careful.
//
// It is also not a second observation path. A demonstration is assembled from the events
// the Director already produces while executing verified semantic actions — the same
// outcomes, the same action-graph nodes, the same verification evidence. Recording
// observes nothing on its own, which is what makes "recording never bypasses verification"
// structural: there is nothing to record until an action has been verified and recorded.
//
// # Why the extraction is deterministic
//
// A learned procedure runs on the user's desktop with the user's files. A procedure
// produced by a language model, or by probabilistic generalisation, cannot be shown to a
// person in full before it runs, and "shown in full before it runs" is the property the
// whole goal layer rests on. So extraction here is a set of stated rules over the recorded
// semantics, every one of which records WHY it fired — see Decision — and the result is
// proposed to the user rather than installed.
package demo

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ID identifies one demonstration.
type ID string

// Status is where a demonstration is in its life.
type Status string

const (
	// Recording: the session is open and steps are still arriving.
	Recording Status = "recording"
	// Completed: the user ended the session and it may be extracted from.
	Completed Status = "completed"
	// Refused: the session ended and may NEVER become a procedure. Terminal and
	// distinct from Completed on purpose — a refusal is a fact about the demonstration,
	// not a verdict the extractor should be asked to re-reach later.
	Refused Status = "refused"
	// Abandoned: the session was cancelled by the user.
	Abandoned Status = "abandoned"
)

// Terminal reports whether a status ends the session.
func (s Status) Terminal() bool { return s != Recording }

// StepKind is what one recorded step DID, in the Director's own vocabulary.
//
// A closed set, and deliberately the vocabulary the executor already speaks: a
// demonstration that invented its own action names would be a second model of what the
// Director does, and the two would drift.
type StepKind string

const (
	// StepSemantic is one semantic verb — invoke, select, expand, confirm.
	StepSemantic StepKind = "semantic"
	// StepFocus moved keyboard focus.
	StepFocus StepKind = "focus"
	// StepEdit set a control's text.
	StepEdit StepKind = "edit"
	// StepCapture bound a value or a collection. Recorded because it is part of the
	// procedure's shape, and because a procedure that consumed an unresolved value must
	// be refused rather than learned.
	StepCapture StepKind = "capture"
)

// Target is the SEMANTIC description of what a step acted on.
//
// Everything here survives the window closing. What is deliberately absent is everything
// that does not: no element id, no native id, no runtime id, no window handle, no
// coordinates, no bounds. A recorded target is a description to re-resolve, never a
// handle to reuse — the same rule the action graph follows, for the same reason.
type Target struct {
	// Phrase is how the step referred to it, in the user's or the procedure's words.
	Phrase string `json:"phrase,omitempty"`
	// Role is the control's semantic role when one was recognised — the durable way to
	// name "the control that renames" without naming it in English.
	Role goal.ControlRole `json:"role,omitempty"`
	// Label is what the control was called. Kept because a procedure that names no role
	// has nothing else, and because it is what a parameter is named after.
	Label string `json:"label,omitempty"`
	// ElementRole is the control TYPE — button, list item, text field.
	ElementRole directorapi.ElementRole `json:"element_role,omitempty"`
	// Deictic marks a target the user pointed at ("this file") rather than named.
	Deictic bool `json:"deictic,omitempty"`
	// DerivedEditor marks a target the application produced mid-procedure — the inline
	// rename box. It is derived at run time and has no durable identity by design.
	DerivedEditor bool `json:"derived_editor,omitempty"`
	// Anaphoric marks a target that points back at what the previous step produced.
	Anaphoric bool `json:"anaphoric,omitempty"`
}

// Describe renders a target in one phrase.
func (t Target) Describe() string {
	switch {
	case t.DerivedEditor:
		return "the editor the previous step opened"
	case t.Role != "":
		return t.Role.Describe()
	case t.Deictic:
		return "the object the user pointed at"
	case t.Label != "":
		return fmt.Sprintf("%q", t.Label)
	case t.Phrase != "":
		return t.Phrase
	}
	return "the focused control"
}

// Step is one verified semantic execution, as a demonstration keeps it.
type Step struct {
	// Index is the step's position, 1-based.
	Index int `json:"index"`
	// Node is the action-graph node this step WAS.
	//
	//	Demonstrations reference Action Graph nodes. They do not duplicate them.
	//
	// A reference rather than a copy: the graph is the execution history and there is
	// one of it. Everything else on this struct is the SEMANTIC summary the extractor
	// reasons over, which is a different thing from the record of what happened.
	Node actiongraph.NodeID `json:"node"`

	Kind StepKind `json:"kind"`
	// Semantic is the verb, for a semantic step.
	Semantic directorapi.SemanticActionKind `json:"semantic,omitempty"`
	// Target is what it acted on, semantically.
	Target Target `json:"target"`

	// Text is the literal a step typed. Empty for everything else, and empty for a
	// sensitive or secret value however it was typed — see Sensitive.
	Text string `json:"text,omitempty"`
	// Sensitive marks a step that handled a value the value layer classified as
	// sensitive or secret. The text is NOT stored, and the step makes the whole
	// demonstration unlearnable — see Safety.
	Sensitive bool `json:"sensitive,omitempty"`
	// ValueRef is the name of a program-local value this step consumed, if any. A
	// procedure that consumed one cannot be learned: the value belonged to a program
	// that is over, and a learned step reading it would resolve to nothing.
	ValueRef string `json:"value_ref,omitempty"`

	// Preconditions are the semantic waits the step ran under. Kept because they are
	// part of the procedure: a rename that does not wait for the editor is a rename that
	// types into the list.
	Preconditions []directorapi.Condition `json:"preconditions,omitempty"`

	// Verified is whether the step's own verification succeeded.
	Verified bool `json:"verified"`
	// Status is the action's outcome, in the executor's vocabulary.
	Status directorapi.ActionStatus `json:"status"`
	// Evidence is the KINDS of evidence verification used, never their contents.
	Evidence []string `json:"evidence,omitempty"`
	// Clarified marks a step that needed the user to disambiguate. A demonstration with
	// one is not learnable: what the procedure would do next time is exactly the
	// question that had to be asked.
	Clarified bool `json:"clarified,omitempty"`

	// Goal and Procedure are the provenance, when the step came from a goal expansion.
	// DIAGNOSTIC: goal recovery reads the ACTIONS, never this. See RecoverGoal.
	Goal      string `json:"goal,omitempty"`
	Procedure string `json:"procedure,omitempty"`

	// Application is the app the step acted in.
	Application string `json:"application,omitempty"`
	// Phrase is the request or step description, for display.
	Phrase string `json:"phrase,omitempty"`
}

// Describe renders a step in one line.
func (s Step) Describe() string {
	verb := string(s.Kind)
	if s.Semantic != "" {
		verb = string(s.Semantic)
	}
	out := fmt.Sprintf("%s %s", verb, s.Target.Describe())
	if s.Text != "" {
		out += fmt.Sprintf(" to %q", s.Text)
	}
	if s.Sensitive {
		out += " to (a sensitive value)"
	}
	return out
}

// Demonstration is one recorded session.
type Demonstration struct {
	ID        ID        `json:"id"`
	Started   time.Time `json:"started"`
	Completed time.Time `json:"completed,omitempty"`
	Status    Status    `json:"status"`

	// Application is where the demonstration took place, when it stayed in one place.
	// Empty when the user moved between applications, which is a reason to refuse: a
	// procedure is registered for an application.
	Application string `json:"application,omitempty"`

	// Requests are the phrases the user said, in order. Context for a reader, and
	// deliberately NOT the source of the recovered goal — see RecoverGoal.
	Requests []string `json:"requests,omitempty"`

	// Steps are the verified semantic steps, in order.
	Steps []Step `json:"steps,omitempty"`

	// Nodes is the action-graph lineage, in order, for everything this session did.
	// The same references the steps carry, kept whole so a reader can walk the history
	// without reconstructing it.
	Nodes []actiongraph.NodeID `json:"nodes,omitempty"`

	// Confirmed lists the actions the confirmation gate had to ask about, in the words
	// it asked them in.
	//
	// Recorded from what the gate CONCLUDED rather than inferred from the steps, because
	// the gate is the only thing that knows: policy, the goal's declared safety and the
	// coverage rules all feed it. A demonstration with any entry here cannot be learned —
	// see Unsafe.
	Confirmed []string `json:"confirmed,omitempty"`

	// Refusal is why a Refused demonstration may never become a procedure.
	Refusal string `json:"refusal,omitempty"`

	// Notes are things a reader should know that are not steps — a cancelled program, a
	// clarification, an unverified action.
	Notes []string `json:"notes,omitempty"`

	// mixedApps records that the session crossed applications, so a later step in the
	// original one does not silently re-adopt it as THE application. Unexported, and so
	// deliberately not durable: a reloaded demonstration with no Application already says
	// what this says.
	mixedApps bool
}

// Describe renders a demonstration in one line.
func (d *Demonstration) Describe() string {
	if d == nil {
		return "(no demonstration)"
	}
	app := d.Application
	if app == "" {
		app = "several applications"
	}
	return fmt.Sprintf("%s — %d step(s) in %s, %s", d.ID, len(d.Steps), app, d.Status)
}

// Learnable reports whether this demonstration is in a state extraction may look at.
//
// A refusal is terminal and a recording session is incomplete. Both are reported by the
// extractor as a refusal rather than being silently skipped, because "nothing came back"
// and "this can never come back" are different answers.
func (d *Demonstration) Learnable() (bool, string) {
	switch {
	case d == nil:
		return false, "there is no such demonstration"
	case d.Status == Recording:
		return false, "this demonstration is still being recorded, so it has no end to " +
			"extract a procedure from"
	case d.Status == Refused:
		return false, d.Refusal
	case d.Status == Abandoned:
		return false, "this demonstration was abandoned, and a procedure is not learned " +
			"from a task the user did not finish"
	case len(d.Steps) == 0:
		return false, "this demonstration recorded no semantic steps, so there is nothing " +
			"to extract"
	}
	return true, ""
}

// TextSteps returns the steps that typed something, in order.
func (d *Demonstration) TextSteps() []Step {
	var out []Step
	for _, s := range d.Steps {
		if s.Kind == StepEdit {
			out = append(out, s)
		}
	}
	return out
}

// Verbs returns the semantic verbs in order, for goal recovery and for display.
func (d *Demonstration) Verbs() []directorapi.SemanticActionKind {
	var out []directorapi.SemanticActionKind
	for _, s := range d.Steps {
		if s.Semantic != "" {
			out = append(out, s.Semantic)
		}
	}
	return out
}

// Roles returns the distinct semantic control roles the demonstration invoked.
func (d *Demonstration) Roles() []goal.ControlRole {
	seen := map[goal.ControlRole]bool{}
	var out []goal.ControlRole
	for _, s := range d.Steps {
		if r := s.Target.Role; r != "" && !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// HasRole reports whether the demonstration invoked a control of this semantic role.
func (d *Demonstration) HasRole(r goal.ControlRole) bool {
	for _, s := range d.Steps {
		if s.Target.Role == r {
			return true
		}
	}
	return false
}

// slug turns a label into an identifier-shaped name.
//
// Used for naming a parameter after the field it was typed into: "Customer name" becomes
// customer_name. Deterministic and boring on purpose — a parameter name a user cannot
// predict from the field they typed into is a parameter they will not recognise.
func slug(s string) string {
	var b strings.Builder
	lastUnderscore := true // leading underscores are dropped
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case !lastUnderscore:
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}
