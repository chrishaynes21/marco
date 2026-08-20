package execute

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/inline"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Derived targets: the editor a previous step opened.
//
//	The inline editor is a temporary derived target created by acting on the bound file.
//	Carry it request-locally. Replay must not assume an old inline editor still exists.
//
// A binding answers "which object did the user mean?" and lives for the request. A derived
// target answers "which control is the application showing me RIGHT NOW for that object?"
// and lives for one step. Conflating them would be a category error with teeth: a
// remembered editor is a control that has since closed, and a query built from one would
// resolve to nothing or, worse, to whatever inherited its identifier.
//
// So it is derived fresh, from the observation the step itself made, immediately before
// the step runs — and if it cannot be derived, the step stops.

// deriveEditor resolves a RequiresEditor reference against the world this step observed.
//
// Returns the intent with its reference pinned to the editor's own identifier, plus the
// verification that established it. A non-verified result means the step must not run: the
// caller reports the verification's reason rather than falling back to anything.
func (p *Pipeline) deriveEditor(ctx context.Context, world directorapi.WorldState,
	in directorapi.Intent) (directorapi.Intent, inline.Verification, bool) {

	ref, ok := editorRef(in)
	if !ok {
		return in, inline.Verification{}, false
	}

	b := boundObject(ctx, in)
	if !b.Bound() {
		return in, inline.Verification{
			Result: inline.Unverified,
			Reason: "this step acts on the editor for an object nothing bound, so there " +
				"is nothing to look for an editor of",
		}, true
	}

	// A COMMIT is derived differently, and only in one clause: the editor it ends has
	// already been typed into by an earlier step of this request, so it no longer holds
	// the item's original name. Discovering it by that name would refuse it — which is
	// what stopped the previous run's fourth step after its third had succeeded. See
	// inline.FindOpen for what still has to hold.
	find := inline.Find
	if isCommit(in) {
		find = inline.FindOpen
	}
	ed, v := find(&world, b)
	if v.Result != inline.Verified {
		return in, v, true
	}

	// Pinned by the source's own identifier, which is the only thing that tells this
	// editor from the other controls in the window containing the same text. Derived a
	// moment ago from this step's own observation — see the package comment for why it
	// is never carried further than that.
	pinned := in
	pinned.Targets = append([]directorapi.ReferenceExpression{}, in.Targets...)
	for i := range pinned.Targets {
		if !pinned.Targets[i].RequiresEditor {
			continue
		}
		win := directorapi.WindowID(ed.WindowID)
		pinned.Targets[i].Query = &directorapi.ElementQuery{
			NativeID: ed.NativeID, Window: &win,
		}
		pinned.Targets[i].Phrase = fmt.Sprintf("the editor for %s", ed.Resource)
	}
	_ = ref
	return pinned, v, true
}

// editorRef returns the intent's derived-target reference, if it has one.
func editorRef(in directorapi.Intent) (directorapi.ReferenceExpression, bool) {
	for _, ref := range in.Targets {
		if ref.RequiresEditor {
			return ref, true
		}
	}
	return directorapi.ReferenceExpression{}, false
}

// boundObject is the binding a derived target belongs to.
//
// The request's ONE binding: an editor is opened on the object the user pointed at, and a
// request that bound nothing has no object for an editor to belong to. Taken from the
// store rather than from this step's own intent, because the step that opens the editor
// and the step that types into it are different steps with different references.
func boundObject(ctx context.Context, in directorapi.Intent) *binding.Binding {
	if b := bindingFor(ctx, in); b.Bound() {
		return b
	}
	store := binding.StoreFrom(ctx)
	if store == nil {
		return nil
	}
	for _, b := range store.All() {
		if b.Bound() {
			return b
		}
	}
	return nil
}

// verifyEditorValue confirms the editor holds what the step meant to put in it.
//
//	Do not infer successful text entry only because the input capability returned
//	success.
//
// Run after an edit whose target was a derived editor, against the world observed after
// acting. A mismatch FAILS the step: the capability reporting success says only that a
// call returned, and the previous live attempt had a call return successfully after
// writing into a details-view cell that renames nothing.
func (p *Pipeline) verifyEditorValue(ctx context.Context, after directorapi.WorldState,
	in directorapi.Intent, want string) (inline.Verification, bool) {

	if _, ok := editorRef(in); !ok || want == "" {
		return inline.Verification{}, false
	}
	b := boundObject(ctx, in)
	if !b.Bound() {
		return inline.Verification{}, false
	}
	return inline.VerifyValue(&after, b, nil, want), true
}

// verifyEditorClosed confirms the edit transaction ended.
//
// Half of the commit check, and it says so: an editor that closed proves the mode ended
// and nothing about what it ended as. The filesystem correlation is the other half.
func (p *Pipeline) verifyEditorClosed(ctx context.Context, after directorapi.WorldState,
	in directorapi.Intent) (inline.Verification, bool) {

	if _, ok := editorRef(in); !ok {
		return inline.Verification{}, false
	}
	b := boundObject(ctx, in)
	if !b.Bound() {
		return inline.Verification{}, false
	}
	return inline.VerifyClosed(&after, b), true
}

// editedText is the literal an edit step intends to write, empty when it writes a
// captured value instead.
func editedText(in directorapi.Intent) string { return in.Text }

// unpinEditor takes the derived editor's identifier back out of what is stored.
//
//	Do not serialize ephemeral native handles as durable graph identity.
//
// The plan had to be built against a PINNED query — that is what stopped the step acting
// on the details-view cell — and the node stores the plan. Left there, the id of a control
// that existed for one edit transaction becomes part of the durable graph: a replay looks
// for it, does not find it, and reports the action as no longer replayable rather than
// deriving the editor that is open now.
//
// So the stored form keeps what the step MEANT and drops the handle. The node's own
// account of the edit is the Snapshot — see noteEditor — which is deliberately not enough
// to find the control again either.
func unpinEditor(node *actiongraph.ActionNode) {
	if node == nil || !node.RequestedTarget.RequiresEditor {
		return
	}
	for i := range node.Plan.Steps {
		q := node.Plan.Steps[i].Action.Query
		if q == nil || q.NativeID == "" {
			continue
		}
		stripped := *q
		stripped.NativeID = ""
		node.Plan.Steps[i].Action.Query = &stripped
	}
}

// statusForEditor maps a failed editor derivation onto the request's headline status.
//
// An AMBIGUITY is a question — two editors open is something a person can resolve by
// closing one. Everything else is a refusal: an absent editor means the application did not
// enter edit mode, and a mismatched one means it is editing something else. Neither is
// answerable, and neither may proceed.
func statusForEditor(r inline.Result) directorapi.ResultStatus {
	if r == inline.Ambiguous {
		return directorapi.ResultNeedsClarification
	}
	return directorapi.ResultFailed
}

// isCommit reports whether an intent is the action that ends an edit transaction.
//
// Read from the semantic vocabulary rather than from the phrase: "confirm" is the verb
// that commits, and the ladder decides how — which on Windows is a keystroke, and remains
// a platform execution detail rather than something a procedure names.
func isCommit(in directorapi.Intent) bool {
	kind, _ := in.Parameters["semantic_kind"].(string)
	return kind == string(directorapi.SemanticConfirm) ||
		kind == string(directorapi.SemanticSubmit)
}
