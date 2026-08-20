package goal_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Expansion binds what the user pointed at, or refuses.
//
//	A request such as "rename this file to Budget" cannot execute unless "this file"
//	has resolved to a typed, evidenced binding.
//	Do not silently fall back to the old focus behavior.

var bt0 = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

// element builds one object as a provider would report it.
func element(id string, role directorapi.ElementRole, label, path string) *directorapi.Element {
	attrs := map[string]any{"native_id": "uia:" + id}
	if path != "" {
		attrs["path"] = path
	}
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Role: role, Label: label, WindowID: "hwnd:1",
		Enabled: true, Visible: true, Confidence: 1, Attributes: attrs,
	}
}

func withFocus(el *directorapi.Element) *directorapi.Element  { el.Focused = true; return el }
func withSelect(el *directorapi.Element) *directorapi.Element { el.Selected = true; return el }

func screen(els ...*directorapi.Element) *directorapi.WorldState {
	m := map[directorapi.ElementID]*directorapi.Element{}
	for _, e := range els {
		m[e.ID] = e
	}
	return &directorapi.WorldState{
		Timestamp: bt0, Elements: m,
		Windows: []directorapi.Window{{
			ID: "hwnd:1", Application: "explorer", Title: "tmp", Focused: true, Visible: true,
		}},
	}
}

// testBinder is the pipeline's binder, without the pipeline: it resolves against one
// world and files what it resolved.
type testBinder struct {
	world    *directorapi.WorldState
	store    *binding.Store
	requests []goal.BindRequest
}

func newTestBinder(w *directorapi.WorldState) *testBinder {
	return &testBinder{world: w, store: binding.NewStore()}
}

func (b *testBinder) Bind(req goal.BindRequest) (*binding.Binding, *binding.Problem) {
	b.requests = append(b.requests, req)
	r := binding.NewResolver()
	r.Now = func() time.Time { return bt0 }
	out, prob := r.Resolve(req.Phrase, req.Expected, b.world)
	if prob != nil {
		return nil, prob
	}
	out.Origin = req.Origin
	b.store.Put(out)
	return out, nil
}

func renameGoal() goal.Goal {
	g, ok := goal.Parse("rename this file to Budget")
	if !ok {
		panic("the fixture request is no longer parsed as a goal")
	}
	g.Context.Application = "explorer"
	return g
}

// ── the worked example ────────────────────────────────────────────────────────

// TestRenameThisFileCreatesAFileBinding is the milestone's headline case.
func TestRenameThisFileCreatesAFileBinding(t *testing.T) {
	b := newTestBinder(screen(
		withFocus(element("e1", directorapi.RoleListItem, "Report.txt", `C:\tmp\Report.txt`)),
	))

	ex, err := goal.Expand(goal.NewRegistry(), renameGoal(), b)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(ex.Bindings) != 1 {
		t.Fatalf("the expansion recorded %d bindings, want one", len(ex.Bindings))
	}
	bound := ex.Bindings[0]
	if bound.Resolved != binding.KindFile {
		t.Errorf("resolved kind = %s, want file", bound.Resolved)
	}
	if bound.Resource != `C:\tmp\Report.txt` {
		t.Errorf("resource = %q, want the path behind the item", bound.Resource)
	}
	if !bound.Decisive() {
		t.Error("the binding has no decisive evidence, so it is a guess")
	}
	// The procedure and step it came from, so a later mismatch can name them.
	if bound.Origin.Procedure == "" || bound.Origin.StepIndex != 1 {
		t.Errorf("the binding carries no usable provenance: %+v", bound.Origin)
	}
	if len(b.requests) != 1 || b.requests[0].Expected != binding.KindFile {
		t.Errorf("the binder was asked for %+v, want one request expecting a file", b.requests)
	}
}

// TestTheExpandedActionCarriesTheBinding — the binding must reach the step, not merely
// exist alongside it.
func TestTheExpandedActionCarriesTheBinding(t *testing.T) {
	b := newTestBinder(screen(
		withFocus(element("e1", directorapi.RoleListItem, "Report.txt", `C:\tmp\Report.txt`)),
	))

	ex, err := goal.Expand(goal.NewRegistry(), renameGoal(), b)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	ref := ex.Program.Steps[0].Operation.Targets[0]
	if !ref.RequiresBinding {
		t.Fatal("the step's reference does not declare that it needs a binding")
	}
	if ref.BindingID == "" {
		t.Fatal("the step's reference carries no binding id, so nothing downstream can " +
			"find out what it points at")
	}
	if ref.BindingID != string(ex.Bindings[0].ID) {
		t.Errorf("the step points at binding %q and the expansion recorded %q",
			ref.BindingID, ex.Bindings[0].ID)
	}
	if ref.ExpectedKind != string(binding.KindFile) {
		t.Errorf("expected kind = %q, want file", ref.ExpectedKind)
	}
	// The semantic target survives: this is still a reference to a named thing, not a
	// point on the screen.
	if ref.Query == nil || ref.Query.Label != "Report.txt" {
		t.Errorf("the reference lost its semantic query: %+v", ref.Query)
	}
	// And the program is runnable, which the unbound one is not.
	if err := program.ValidateBound(ex.Program); err != nil {
		t.Fatalf("a bound expansion was refused by the execution validator: %v", err)
	}
}

// ── the refusals ──────────────────────────────────────────────────────────────

func expectRefusal(t *testing.T, w *directorapi.WorldState, want binding.ProblemReason) goal.BindingFailure {
	t.Helper()
	_, err := goal.Expand(goal.NewRegistry(), renameGoal(), newTestBinder(w))
	if err == nil {
		t.Fatal("the expansion succeeded; it should have refused")
	}
	var failure goal.BindingFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error is %T (%v), want a BindingFailure", err, err)
	}
	if failure.Problem.Reason != want {
		t.Errorf("reason = %s, want %s (%s)", failure.Problem.Reason, want,
			failure.Problem.Message)
	}
	return failure
}

// TestAFocusedFolderCannotProduceARename — the mistake the binding layer exists for.
func TestAFocusedFolderCannotProduceARename(t *testing.T) {
	f := expectRefusal(t, screen(
		withFocus(element("e1", directorapi.RoleListItem, "Projects", `C:\tmp\Projects`)),
	), binding.ReasonWrongKind)

	if f.Clarifiable() {
		t.Error("a wrong-kind focus was offered as a question; there is nothing to ask, " +
			"and asking would invite the same folder to be selected again")
	}
	if !strings.Contains(f.Error(), "folder") {
		t.Errorf("the refusal does not say what was focused: %s", f.Error())
	}
}

// TestAnUnsavedBufferCannotProduceARename — a tab labelled like a file is not a file.
func TestAnUnsavedBufferCannotProduceARename(t *testing.T) {
	expectRefusal(t, screen(
		withFocus(element("e1", directorapi.RoleTab, "Budget.txt", "")),
	), binding.ReasonWrongKind)
}

// TestABareControlCannotProduceARename.
func TestABareControlCannotProduceARename(t *testing.T) {
	expectRefusal(t, screen(
		withFocus(element("e1", directorapi.RoleButton, "Rename", "")),
	), binding.ReasonWrongKind)
}

// TestNothingFocusedCannotProduceARename.
func TestNothingFocusedCannotProduceARename(t *testing.T) {
	f := expectRefusal(t, screen(
		element("e1", directorapi.RoleListItem, "Report.txt", `C:\tmp\Report.txt`),
	), binding.ReasonNoFocus)

	if !f.Clarifiable() {
		t.Error("nothing focused is answerable by the user and was not offered as a question")
	}
}

// TestAmbiguousCandidatesStopBeforeRunProgram — several equally plausible files is a
// question, and the program never comes into existence to be run.
func TestAmbiguousCandidatesStopBeforeRunProgram(t *testing.T) {
	f := expectRefusal(t, screen(
		withSelect(element("e1", directorapi.RoleListItem, "Report.txt", `C:\tmp\Report.txt`)),
		withSelect(element("e2", directorapi.RoleListItem, "Report2.txt", `C:\tmp\Report2.txt`)),
	), binding.ReasonAmbiguous)

	if !f.Clarifiable() {
		t.Fatal("an ambiguity is exactly what the user can settle, and it was not " +
			"offered as a question")
	}
	q := f.Question()
	if !strings.Contains(q, "Report.txt") || !strings.Contains(q, "Report2.txt") {
		t.Errorf("the question does not name the candidates: %s", q)
	}
}

// TestTheUntypedFocusFallbackIsUnreachable walks every procedure in the library and
// proves that no deictic step anywhere can produce a bare focused-element query.
//
// A sweep rather than a single case, because the fallback was a shared helper: removing it
// from the rename path and leaving it in the delete path would be worse than not removing
// it at all.
func TestTheUntypedFocusFallbackIsUnreachable(t *testing.T) {
	r := goal.NewRegistry()
	for _, kind := range goal.Vocabulary {
		g := satisfied(kind)
		g.Context.TargetIsImplicit = true
		g.Context.Target = "this"

		ex, err := goal.Plan(r, g)
		if err != nil {
			continue // refused before expansion; nothing to check
		}
		for i, s := range ex.Program.Steps {
			for _, ref := range s.Operation.Targets {
				if !ref.RequiresBinding {
					continue
				}
				if ref.Query != nil && ref.Query.Focused != nil && *ref.Query.Focused {
					t.Errorf("%s step %d aims a deictic reference at whatever holds focus",
						kind, i+1)
				}
				if ref.BindingID != "" {
					t.Errorf("%s step %d has a binding id without anything having "+
						"observed the screen", kind, i+1)
				}
			}
		}
		if ex.Deictic {
			if err := program.ValidateBound(ex.Program); err == nil {
				t.Errorf("%s expanded into an unbound deictic program the execution "+
					"validator accepted", kind)
			}
		}
	}
}

// TestAProcedureThatPointsWithoutDeclaringAKindIsRefused — silence is not "anything will
// do".
func TestAProcedureThatPointsWithoutDeclaringAKindIsRefused(t *testing.T) {
	r := goal.NewRegistry()
	r.Register(goal.Procedure{
		Name: "careless", Goal: goal.Rename,
		Applications: []string{"careless.exe"},
		Requires:     []goal.Requirement{goal.RequiresTarget, goal.RequiresName},
		Safety:       goal.Safety{Mutations: 1, Risk: directorapi.RiskMedium},
		Steps: func(g goal.Goal) ([]goal.Directive, error) {
			return []goal.Directive{{
				Semantic: directorapi.SemanticSelect, TargetDeictic: true,
				Target: "this", Phrase: "select what was pointed at",
			}}, nil
		},
	})

	g := renameGoal()
	g.Context.Application = "careless.exe"
	if _, err := goal.Expand(r, g, newTestBinder(screen(
		withFocus(element("e1", directorapi.RoleListItem, "Report.txt", `C:\tmp\Report.txt`)),
	))); err == nil {
		t.Fatal("a procedure that points at something without saying what kind of thing " +
			"it must be was allowed to expand")
	}
}
