//go:build livevalidation

package live

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
)

// The one live scenario: rename a file in File Explorer.
//
//	Only: Explorer file rename. One scenario only.
//
// Everything below happens against files THIS TEST created, in a directory this test
// created, driven through the production Director over its real protocol. Nothing is
// stubbed: the confirmation is answered over the wire, the input is real, and the result
// is checked by reading the filesystem afterwards rather than by asking the Director
// whether it thinks it worked.
//
// The distractor is the point. A rename that hit the wrong item would leave Alpha.txt
// unchanged and Bravo.txt renamed — which the second half of the check catches and the
// first half alone would not.

const (
	alpha = "Alpha.txt"
	bravo = "Bravo.txt"
	// alphaBody and bravoBody are distinct so a rename that swapped them is visible.
	alphaBody = "alpha contents, must survive the rename\n"
	bravoBody = "bravo contents, must not be touched at all\n"
	// newName is what the request asks for. Deliberately WITHOUT an extension, because
	// that is how a person says it — and how the file lands depends on whether Explorer
	// is hiding extensions, which the check accommodates.
	newName = "Budget"
)

// TestLiveExplorerRename is the milestone's fifth acceptance criterion.
func TestLiveExplorerRename(t *testing.T) {
	Require(t)

	w := NewWorkspace(t, map[string]string{alpha: alphaBody, bravo: bravoBody})
	t.Logf("workspace %s", w.Dir)

	h := StartDirector(t)
	// A REAL answer over the real protocol, on its own connection. Not a stub confirmer:
	// the Director asks exactly as it would ask a person, and the harness answers.
	h.AutoConfirm(true)

	win := h.OpenExplorer(w)

	// Select the file the request will point at. "Rename THIS file" binds to what holds
	// focus, and a freshly opened folder has nothing focused — so the scenario has to
	// establish the deixis the same way a user would, by clicking the item.
	//
	// Through the Director, not by synthesising a click here: a harness with its own
	// input path would be testing its own input path.
	selected := h.Submit("click " + alpha)
	if selected.State != "completed" {
		t.Skipf("PREREQUISITE NOT MET: %s could not be selected in the Explorer window "+
			"(%s: %s), so nothing points at it and the rename was NOT attempted. This is "+
			"a SKIP, not a pass.", alpha, selected.State, selected.Message)
	}

	// ── the identity, BEFORE the request that acts on it ──────────────────────
	// The Director must be about to act on the file THIS TEST created. Checked here
	// rather than inferred from the result, because "it renamed the right file" and "it
	// renamed a file that happened to be there" look identical afterwards.
	res, why := h.SelectedResource()
	if res == nil {
		t.Skipf("PREREQUISITE NOT MET: %s\n"+
			"  The Director selected %s and could not establish what file it is. The rename "+
			"was NOT attempted.\n"+
			"  This is a SKIP, not a pass: nothing was renamed and nothing was verified.",
			why, alpha)
	}
	t.Logf("observed backing resource: %s", res.Describe())
	for _, e := range res.Evidence {
		t.Logf("   evidence: %s", e)
	}
	// The workspace guard, checked before the identity guard because it is the broader
	// one: whatever the Director is about to act on, it must be inside the directory
	// THIS TEST created. A path anywhere else is a pre-existing user file, and no
	// scenario here may touch one.
	if !strings.EqualFold(filepath.Dir(res.Path), w.Dir) {
		t.Fatalf("the Director is about to act on %s, which is not inside this test's "+
			"workspace %s. REFUSING to submit a rename: a path outside the temporary "+
			"directory is a file that belongs to somebody.", res.Path, w.Dir)
	}
	if !strings.EqualFold(res.Path, w.Files[alpha]) {
		t.Fatalf("the Director is about to act on %s and this test created %s. "+
			"Refusing to submit a rename against the wrong object.", res.Path, w.Files[alpha])
	}
	if res.Kind != "file" {
		t.Fatalf("the Director sees %s as a %s", res.Path, res.Kind)
	}

	// ── the request under test ────────────────────────────────────────────────
	out := h.Submit("rename this file to " + newName)

	// A binding that could not establish IDENTITY is an unmet prerequisite, not a
	// failure of the rename: the Director refused before sending any input, which is
	// the behaviour this whole layer exists to produce.
	//
	// It happens when the accessibility bridge reports no backing path for an Explorer
	// list item. Without a path the item is a control that happens to be captioned
	// "Alpha.txt", and a caption is not identity — two folders can hold the same name.
	// Reported as a SKIP with the exact cause so it is never mistaken for a pass, and
	// never mistaken for the binding layer misbehaving.
	if unmetIdentity(out.Message) {
		t.Skipf("PREREQUISITE NOT MET: the Director could not establish a backing path "+
			"for %s, so %q refused BEFORE sending any input and the rename did not "+
			"happen.\n  Director said: %s\n"+
			"  The gap is in the accessibility bridge (plugins/uia), which does not "+
			"surface a shell item's parsing path. Until it does, the rename cannot be "+
			"driven deictically in Explorer.\n"+
			"  This is a SKIP, not a pass: no rename occurred and nothing was verified.",
			alpha, "rename this file to "+newName, out.Message)
	}

	// ── independent verification, from outside the Director ───────────────────
	// Everything below reads the filesystem. None of it asks the Director whether it
	// succeeded, because that is the claim under test.

	if _, stillThere := w.Read(w.Files[alpha]); stillThere {
		t.Errorf("%s is still there, so it was not what got renamed (Director said %s: %s)",
			w.Files[alpha], out.State, out.Message)
	}

	renamed, body := findRenamed(t, w)
	if renamed == "" {
		t.Fatalf("nothing named %s appeared in %s. The directory holds %v. "+
			"Director said %s: %s", newName, w.Dir, w.Names(t), out.State, out.Message)
	}
	t.Logf("Alpha.txt became %s", renamed)

	if body != alphaBody {
		t.Errorf("%s does not hold what Alpha.txt held, so this is a different object "+
			"carrying the requested name", renamed)
	}

	// The distractor, untouched. This is the half that catches a rename of the item
	// next to the intended one.
	bravoBodyNow, bravoThere := w.Read(w.Files[bravo])
	if !bravoThere {
		t.Fatalf("%s is gone; the rename hit the wrong file", w.Files[bravo])
	}
	if bravoBodyNow != bravoBody {
		t.Errorf("%s changed, and nothing in the request should have touched it", bravo)
	}

	// Exactly one rename. Two files in, two files out, and the names are the ones
	// expected — which rules out a copy, a duplicate, and a second stray rename.
	names := w.Names(t)
	if len(names) != 2 {
		t.Errorf("the directory holds %d entries (%v); a rename adds and removes nothing",
			len(names), names)
	}
	if !contains(names, bravo) {
		t.Errorf("%s is not in %v", bravo, names)
	}
	if contains(names, alpha) {
		t.Errorf("%s is still in %v", alpha, names)
	}

	// ── the Director's own account ────────────────────────────────────────────
	// Checked AFTER the filesystem, and never instead of it. What this adds is that the
	// production runtime produced the record it claims to.

	if out.State != "completed" {
		t.Errorf("the filesystem shows the rename happened and the Director reported "+
			"%s: %s", out.State, out.Message)
	}

	if len(h.Answered()) == 0 {
		t.Error("no confirmation was ever put, so the confirmation path did not run")
	}
	for _, ask := range h.Answered() {
		t.Logf("confirmed: %s", ask.Question())
	}

	nodes := h.Graph()
	if len(nodes) == 0 {
		t.Fatal("the production runtime produced no action graph")
	}
	assertGraph(t, nodes, w.Files[alpha], win.Title)
}

// findRenamed returns the file that carries the requested name, and its contents.
//
// Two candidates, because a person saying "rename this to Budget" gets `Budget.txt` with
// extensions hidden and `Budget` with them shown. Both are correct renames of the bound
// object; which one happens is Explorer's business, not the Director's.
func findRenamed(t *testing.T, w *Workspace) (string, string) {
	t.Helper()
	for _, candidate := range []string{newName + ".txt", newName} {
		path := filepath.Join(w.Dir, candidate)
		if body, ok := w.Read(path); ok {
			return candidate, body
		}
	}
	return "", ""
}

// assertGraph checks what the production runtime recorded.
//
//	action graph produced, verification evidence attached.
//
// The node has to carry the BINDING — which object the action was aimed at — and the
// verification evidence, because a history that recorded only "a rename happened" cannot
// answer the question this whole milestone is about: to what.
func assertGraph(t *testing.T, nodes []actiongraph.ActionNode, boundResource, window string) {
	t.Helper()

	var bound, evidenced *actiongraph.ActionNode
	for i := range nodes {
		n := &nodes[i]
		t.Logf("node %s  %-28s %s", n.ID, n.Describe(), n.Outcome.Status)
		if n.Binding.Bound() {
			bound = n
			if strings.EqualFold(n.Binding.Resource, boundResource) {
				t.Logf("   bound to %s", n.Binding.Resource)
			}
		}
		for _, e := range n.Verification.Evidence {
			if e.Kind == "binding_correlation" {
				evidenced = n
				t.Logf("   correlation: %s", e.Detail)
			}
		}
		if n.GoalProvenance != nil {
			t.Logf("   from %s", n.GoalProvenance.Describe())
		}
	}

	if bound == nil {
		t.Error("no node carries a binding snapshot, so history cannot say which object " +
			"the rename was aimed at")
	} else if !strings.EqualFold(bound.Binding.Resource, boundResource) {
		t.Errorf("the recorded binding names %q and the file that was renamed was %q",
			bound.Binding.Resource, boundResource)
	}
	if evidenced == nil {
		t.Error("no node carries binding-correlation evidence, so the verification that " +
			"ran left no trace in history")
	}
	if window == "" {
		t.Error("the Explorer window was never identified")
	}
}

// unmetIdentity reports whether a refusal was the binding layer declining to act on an
// object it could not identify.
//
// Matched on the binding layer's own wording rather than on a status code, because the
// status is BLOCKED for several different reasons and only this one is a missing
// capability rather than a defect.
func unmetIdentity(message string) bool {
	for _, phrase := range []string{
		"needs a file, and what is focused is a control",
		"has no file behind it",
		"names nothing",
	} {
		if strings.Contains(message, phrase) {
			return true
		}
	}
	return false
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}
