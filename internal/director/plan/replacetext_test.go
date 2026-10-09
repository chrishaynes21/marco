package plan

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The text a replace looks FOR has to survive the whole way to the action.
//
// A wiring test rather than a parse test: the planner already proves "change milk to oat milk"
// reads as a search, and the verifier already proves a computed replacement is checkable. What
// nothing proved is the passage BETWEEN them — three hand-offs, each of which has dropped the
// word milk at some point in this feature's short life:
//
//	planner  →  Plan.Operation.(edit.ReplaceText).Old
//	intent   →  Parameters[EditFind]
//	plan     →  EditAction.Find
//	executor →  edit.ReplaceText{Old: …}
//
// It enters through intent.New() and Planner.Build, the two production constructors, because a
// test that filled in Parameters itself would pass with the intent hand-off deleted — which is
// exactly the shape of the three recorded cases of complete code that was never invoked.
// [[wiring-tests-mutation-gate]].
func TestAReplacesSearchTextReachesTheAction(t *testing.T) {
	for _, phrase := range []string{
		"change milk to oat milk",
		"replace milk with oat milk",
	} {
		t.Run(phrase, func(t *testing.T) {
			in := Input{
				Intent: intent.New().Parse(phrase),
				Resolution: directorapi.Resolution{
					Status: directorapi.ResolutionResolved,
					Target: &directorapi.ResolvedTarget{
						ElementID: "e1", Label: "Text editor",
						Role:  directorapi.RoleTextField,
						Query: &directorapi.ElementQuery{Label: "Text editor"},
					},
				},
			}
			built, err := New().Build(in)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if len(built.Steps) != 1 {
				t.Fatalf("got %d steps, want 1", len(built.Steps))
			}
			a, isEdit := built.Steps[0].Action.(directorapi.EditAction)
			if !isEdit {
				t.Fatalf("action = %T, want EditAction", built.Steps[0].Action)
			}
			if a.Operation != "replace_text" {
				t.Fatalf("operation = %q, want replace_text", a.Operation)
			}
			if a.Find != "milk" {
				t.Fatalf("the action looks for %q, want %q.\n\nWithout it the executor has "+
					"nothing to search for, and the replace that reaches the user's "+
					"document is a guess about the selection.", a.Find, "milk")
			}
			if a.Text != "oat milk" {
				t.Errorf("the action puts in %q, want %q", a.Text, "oat milk")
			}

			// AND THE EXPECTATION NAMES BOTH HALVES. "The control's text changed" is true
			// of the document that merely had "oat milk" prepended to it.
			d := built.Steps[0].Expect[0].Description
			if !strings.Contains(d, "milk") || !strings.Contains(d, "oat milk") {
				t.Errorf("the expectation reads %q, which does not say what becomes what", d)
			}
		})
	}
}

// TestReplacingTextIsNotALowRiskEdit — it overwrites the person's own words, and the old ones
// are gone unless the application has an undo the Director may not assume it has.
func TestReplacingTextIsNotALowRiskEdit(t *testing.T) {
	if got := editRisk("replace_text"); got == directorapi.RiskLow {
		t.Errorf("replace_text is graded %s, the same as a copy or a select-all", got)
	}
}
