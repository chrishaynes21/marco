package goal

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Expansion: a goal becomes an ordinary program.
//
//	Expansion must produce ordinary Director Programs. No special execution engine.
//	Everything downstream stays unchanged.
//
// That rule is why this file is short and why nothing below it needed changing.
// Variables, collections, control flow, clarification, replay and verification keep
// working because what comes out of here is the same program.Program that a
// multi-clause request has always produced — the only difference is where the steps
// came from.
//
// The steps carry INTENTS, not resolved targets, exactly like every other program: a
// rename's fourth step resolves its editable field when it runs, not when the goal was
// understood, because the field does not exist until the third step has run.

// Expansion is the full account of turning one goal into one program.
//
// It carries the procedure and the reasoning alongside the program, because `director
// explain goal` has to answer "why these steps?" and reconstructing that from the
// program alone is impossible — the steps are the answer, not the working.
type Expansion struct {
	Goal      Goal            `json:"goal"`
	Procedure string          `json:"procedure"`
	Why       string          `json:"why,omitempty"`
	Safety    Safety          `json:"safety"`
	Program   program.Program `json:"program"`
	// Generic records whether the chosen procedure was the generic one or an
	// application override, which is the first thing to check when an expansion looks
	// wrong for the application it ran in.
	Generic     bool   `json:"generic"`
	Application string `json:"application,omitempty"`

	// Bindings are the deictic references this expansion resolved, in step order.
	//
	// The EXPANSION's copy is a record of what was bound; the live one lives in the
	// request's binding store and is what revalidation refreshes. Kept here so
	// diagnostics and a confirmation prompt can describe the concrete object without
	// reaching into the store.
	Bindings []*binding.Binding `json:"bindings,omitempty"`
	// Deictic reports whether any directive pointed rather than named. A planning-only
	// expansion of a deictic goal is unrunnable by construction — see Plan.
	Deictic bool `json:"deictic,omitempty"`
	// Planned marks an expansion made WITHOUT a binder, for display only. Such an
	// expansion never reaches execution: its deictic references carry RequiresBinding
	// and no BindingID, which program.ValidateBound refuses.
	Planned bool `json:"planned,omitempty"`
}

// Expand turns a goal into a validated program.
//
// The order of the checks is the whole of it:
//
//  1. An unknown goal is refused before anything is looked up.
//  2. A goal with no procedure for this application is refused — NOT approximated with
//     a procedure for a different one, which is how "rename" in an editor would rename
//     a file instead of a symbol.
//  3. Missing requirements are refused as a QUESTION, before expansion, so the user is
//     asked "what should it be called?" rather than watched while the Director focuses
//     a file and then discovers it has no name to type.
//  4. Only then does the procedure run, and what it produces is validated by the
//     ordinary program validator — the same one every other request goes through.
//
// A deictic directive is RESOLVED here, through the supplied binder, and the resulting
// binding is attached to the step's referring expression. A nil binder produces a
// PLANNING-ONLY expansion whose deictic references are marked as needing a binding and
// carry none — runnable nowhere, which is what `director explain goal` wants and what
// execution refuses.
func Expand(r *Registry, g Goal, b Binder) (Expansion, error) {
	if !g.Kind.Known() {
		return Expansion{}, Refusal{
			Goal: g.Kind,
			Reason: fmt.Sprintf("%q is not a goal the Director knows how to carry out",
				g.Kind),
		}
	}
	proc, ok := r.Select(g)
	if !ok {
		return Expansion{}, Refusal{
			Goal:   g.Kind,
			Reason: fmt.Sprintf("the Director has no procedure for %s", g.Kind.Describe()),
		}
	}
	if missing := proc.Missing(g); len(missing) > 0 {
		return Expansion{}, Refusal{
			Goal: g.Kind, Missing: missing,
			Reason: fmt.Sprintf("%s needs more than the request gave it", g.Kind.Describe()),
		}
	}

	directives, err := proc.Steps(g)
	if err != nil {
		return Expansion{}, err
	}
	if len(directives) == 0 {
		return Expansion{}, fmt.Errorf("goal: %s produced no steps", proc.Name)
	}

	ex := Expansion{
		Goal: g, Procedure: proc.Name, Why: proc.Why, Safety: proc.Safety,
		Generic: proc.Generic(), Application: g.Context.Application, Planned: b == nil,
	}

	prog := program.Program{Goal: g.Describe(), Status: program.StatusPlanned}
	// ONE binding for the whole expansion, not one per step. "This file" is a single
	// reference in the user's sentence, and a procedure whose steps each bound
	// independently could have its select land on one object and its context menu on
	// another — which is the exact divergence the binding layer exists to prevent.
	var bound *binding.Binding
	for i, d := range directives {
		// A deictic directive is bound BEFORE its intent is built, so the intent it
		// produces carries a concrete identity rather than "whatever is focused".
		if d.TargetDeictic {
			ex.Deictic = true
			if bound == nil {
				resolved, berr := bindDirective(b, proc, g, d, i)
				if berr != nil {
					return Expansion{}, berr
				}
				if resolved != nil {
					bound = resolved
					ex.Bindings = append(ex.Bindings, resolved)
				}
			}
		}
		// A non-deictic directive gets no binding, whatever the expansion has bound
		// elsewhere: a step naming the Rename control by role is not aimed at the file.
		step := bound
		if !d.TargetDeictic {
			step = nil
		}
		op, err := directiveIntent(d, proc.Expect, step)
		if err != nil {
			return Expansion{}, fmt.Errorf("goal: %s step %d: %w", proc.Name, i+1, err)
		}
		verification := program.VerifyRequired
		if d.BestEffort {
			verification = program.VerifyBestEffort
		}
		prog.Steps = append(prog.Steps, program.Step{
			ID:            program.StepID(fmt.Sprintf("s%d", i+1)),
			Operation:     op,
			Phrase:        d.Phrase,
			Preconditions: d.Preconditions,
			Verification:  verification,
			FailurePolicy: program.Stop,
		})
	}

	// The ordinary validator, not a private one. A procedure that produced something
	// the program layer would refuse must fail HERE, before anything runs, rather than
	// at step four with three steps already performed.
	if err := program.Validate(prog); err != nil {
		return Expansion{}, fmt.Errorf("goal: %s expanded into a program the Director "+
			"cannot run: %w", proc.Name, err)
	}
	// And, when a binder was supplied, the stronger one: every deictic reference must
	// carry the binding it declared it needs. This is what makes the old untyped focus
	// fallback unreachable — a bound expansion that somehow produced a bare focused
	// query fails here rather than at the executor.
	if b != nil {
		if err := program.ValidateBound(prog); err != nil {
			return Expansion{}, fmt.Errorf("goal: %s expanded into a program the Director "+
				"cannot run: %w", proc.Name, err)
		}
	}

	ex.Program = prog
	return ex, nil
}

// Plan expands a goal for DISPLAY, without resolving anything.
//
// The expansion `director explain goal` and `director dry-run` want: it shows the steps a
// request would become without observing the screen, without moving focus and without
// binding anything. Its deictic references say what they would need and carry nothing, so
// the program it produces is refused by ValidateBound and can never be run by accident.
func Plan(r *Registry, g Goal) (Expansion, error) { return Expand(r, g, nil) }

// bindDirective resolves one directive's deictic target, or refuses.
//
// Returns (nil, nil) for every directive that is not deictic, which is most of them: a
// procedure's own controls are named by ROLE and resolved by label at run time, and
// binding them would be answering a question nobody asked.
func bindDirective(b Binder, proc Procedure, g Goal, d Directive, i int) (*binding.Binding, error) {
	if !d.TargetDeictic {
		return nil, nil
	}
	// A procedure that points at something and will not say what kind of thing it is
	// cannot be bound, and must not fall back to "anything focused".
	if proc.Expect == "" {
		return nil, fmt.Errorf("goal: %s step %d points at something without declaring "+
			"what kind of object it must be, so it cannot be resolved safely", proc.Name, i+1)
	}
	if b == nil {
		// Planning only. The reference is still marked as needing a binding, so the
		// program is unrunnable rather than quietly aimed at the focused control.
		return nil, nil
	}

	phrase := d.Target
	if phrase == "" {
		phrase = "this"
	}
	bound, prob := b.Bind(BindRequest{
		Phrase: phrase, Expected: proc.Expect,
		Application: g.Context.Application,
		Semantic:    d.Semantic, Focus: d.Focus,
		Origin: binding.Origin{
			Goal: string(g.Kind), Procedure: proc.Name,
			StepID: fmt.Sprintf("s%d", i+1), StepIndex: i + 1, Request: g.Phrase,
		},
	})
	if prob != nil {
		return nil, BindingFailure{
			Goal: g.Kind, Procedure: proc.Name, StepIndex: i + 1,
			Phrase: phrase, Problem: prob,
		}
	}
	if bound == nil {
		return nil, fmt.Errorf("goal: %s step %d could not be resolved and gave no reason, "+
			"so nothing was done", proc.Name, i+1)
	}
	return bound, nil
}

// directiveIntent turns one directive into the Intent a program step carries.
//
// Constructed directly rather than by writing a phrase and parsing it back. A procedure
// that emitted English would be able to fail on its own output, and its correctness
// would depend on the parser rather than on what it says.
func directiveIntent(d Directive, expect binding.ObjectKind, bound *binding.Binding) (
	directorapi.Intent, error) {

	// A step whose subject the user POINTED at rather than named. It carries the
	// binding that says WHICH object that is — never a bare "whatever holds focus".
	ref, hasRef := reference(d, expect, bound)

	switch {
	case d.Focus:
		if !hasRef {
			return directorapi.Intent{}, fmt.Errorf("a focus step names no control")
		}
		return directorapi.Intent{
			Kind: directorapi.IntentAct, Verb: "focus", Confidence: 1,
			Raw:     d.Phrase,
			Targets: []directorapi.ReferenceExpression{ref},
		}, nil

	case d.SetText:
		// An edit step, shaped exactly as the editing parser shapes one — including the
		// typed value input, so a procedure can set a field to a captured value and the
		// data-flow validator sees it like any other.
		input, err := values.ParseInput(d.Text)
		if err != nil {
			return directorapi.Intent{}, fmt.Errorf("the text to set is not usable: %w", err)
		}
		in := directorapi.Intent{
			Kind: directorapi.IntentAct, Verb: "edit", Confidence: 1,
			Raw:  d.Phrase,
			Text: d.Text,
			Parameters: map[string]any{
				intent.EditOperation: "set_text",
				values.ParamInput:    input,
			},
		}
		if input.IsReference() {
			// The literal is cleared for the same reason the parser clears it: nothing
			// downstream may mistake the reference's own text for the value.
			in.Text = ""
		}
		if hasRef {
			in.Targets = []directorapi.ReferenceExpression{ref}
		} else {
			// An edit that names no control means THE FIELD THE PREVIOUS STEP OPENED —
			// a rename box, a new-folder label, a Save As filename. Every such step
			// carries a precondition that waits for an editable control to hold focus,
			// so by the time this runs the field exists and is focused.
			//
			// Found live: without it the step reached the resolver with an empty query
			// and was refused as "not specific enough to look for", three steps into a
			// rename that had already opened the box.
			//
			// ANAPHORIC, not deictic: it points backwards at what this procedure just
			// produced, not at something the user indicated. It needs no binding for the
			// same reason — the object it names did not exist when the user spoke.
			in.Targets = []directorapi.ReferenceExpression{{
				Phrase: "the field that just opened",
				Kind:   directorapi.ReferenceAnaphoric,
				Query:  &directorapi.ElementQuery{Focused: boolPtr(true)},
			}}
		}
		return in, nil

	case d.Semantic != "":
		if !d.Semantic.Known() {
			return directorapi.Intent{}, fmt.Errorf("%q is not a semantic action", d.Semantic)
		}
		in := directorapi.Intent{
			Kind: directorapi.IntentAct, Verb: intent.SemanticVerb, Confidence: 1,
			Raw:        d.Phrase,
			Parameters: map[string]any{intent.SemanticKindParam: string(d.Semantic)},
		}
		if d.Semantic.NeedsTarget() && !hasRef {
			return directorapi.Intent{}, fmt.Errorf("%s needs a control to act on", d.Semantic)
		}
		if hasRef {
			in.Targets = []directorapi.ReferenceExpression{ref}
		} else {
			// A verb that names no control — confirm, undo, refresh — is addressed to
			// the FOCUSED CONTEXT. Aimed explicitly rather than left empty, because the
			// pipeline resolves a target for every action and an empty query is refused
			// as "not specific enough to look for".
			//
			// Found live: a rename got as far as typing the new name into the box and
			// then failed to confirm it, three verified steps in.
			//
			// Anaphoric for the same reason the edit above is: it points at what this
			// procedure has just produced, not at something the user indicated, so it
			// needs no binding.
			in.Targets = []directorapi.ReferenceExpression{{
				Phrase: "the focused control",
				Kind:   directorapi.ReferenceAnaphoric,
				Query:  &directorapi.ElementQuery{Focused: boolPtr(true)},
			}}
		}
		return in, nil
	}
	return directorapi.Intent{}, fmt.Errorf("a directive says nothing to do")
}

// reference builds the referring expression for a directive's target.
//
// Two shapes, and the difference is what makes "rename this file" work: a NAMED target
// is a label to search for, and a POINTED one is resolved against what holds focus —
// the same reading the semantic layer gives "expand this".
func reference(d Directive, expect binding.ObjectKind, bound *binding.Binding) (
	directorapi.ReferenceExpression, bool) {

	// The inline editor a previous step opened. Declared, not queried: a procedure says
	// "the editor for the thing I bound", and which control that is on this machine is
	// the platform's answer — resolved immediately before the step runs.
	if d.TargetEditor {
		return directorapi.ReferenceExpression{
			Phrase: "the rename editor", Kind: directorapi.ReferenceAnaphoric,
			RequiresEditor: true,
		}, true
	}

	// A deictic target FIRST, before the named forms. "Rename this file" carries both
	// the user's phrase in Target and the fact that they pointed, and reading Target as
	// a label would search the desktop for a control called "this file".
	if d.TargetDeictic {
		phrase := d.Target
		if phrase == "" {
			phrase = "this"
		}
		ref := directorapi.ReferenceExpression{
			Phrase: phrase, Kind: directorapi.ReferenceDeictic,
			RequiresBinding: true, ExpectedKind: string(expect),
		}
		if bound.Bound() {
			// The query is built from the BOUND object's identity, not from "focused":
			// the whole point is that the step acts on the thing the user pointed at
			// even if focus has since moved somewhere it could be re-read from.
			ref.BindingID = string(bound.ID)
			ref.Query = queryFor(bound)
		}
		return ref, true
	}

	if d.Role != "" {
		// A control named by MEANING. The query carries every alias, so it resolves on a
		// machine in any of the languages the role's table covers, and the canonical
		// name stays in Label so anything that only knows about Label still reads
		// sensibly.
		names := Aliases(d.Role)
		if len(names) == 0 {
			return directorapi.ReferenceExpression{}, false
		}
		return directorapi.ReferenceExpression{
			Phrase: d.Role.Describe(), Kind: directorapi.ReferenceLiteral,
			Query: &directorapi.ElementQuery{
				Label: names[0], AnyLabel: names,
				// A destructive role demands an EXACT label. "Save" is a substring of
				// "Don't Save", so a near match on the discard role is a confident wrong
				// answer that loses the user's work. Refusing is recoverable.
				ExactLabel: d.Role.Destructive(),
			},
		}, true
	}
	if d.Target != "" {
		return directorapi.ReferenceExpression{
			Phrase: d.Target, Kind: directorapi.ReferenceLiteral,
			Query: &directorapi.ElementQuery{Label: d.Target},
		}, true
	}
	return directorapi.ReferenceExpression{}, false
}

// queryFor builds the element query that finds a bound object again.
//
// By LABEL and WINDOW, scoped to the application — the durable, semantic description of
// the object, exactly as every other reference in the Director is expressed. The
// binding's element id is deliberately NOT used as a handle: it is evidence of what was
// bound, and the revalidation step is what proves the object is still that one.
func queryFor(b *binding.Binding) *directorapi.ElementQuery {
	q := &directorapi.ElementQuery{Label: b.Label}
	if b.WindowID != "" {
		w := directorapi.WindowID(b.WindowID)
		q.Window = &w
	}
	if b.Application != "" {
		q.Application = b.Application
	}
	if b.Label == "" {
		// Nothing to search by. A focused query is the honest fallback HERE and only
		// here, because the binding has already established which object this is and
		// the revalidation ahead re-checks it — this is a way of reaching a known
		// object, not a way of choosing one.
		focused := true
		q.Focused = &focused
	}
	return q
}

// boolPtr is the one-line helper a pointer-valued query field needs.
func boolPtr(b bool) *bool { return &b }
