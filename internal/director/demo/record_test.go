package demo_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Recording a demonstration.
//
//	Record semantics, never mechanics.
//	Recording never bypasses verification.
//
// The recorder is fed the Outcomes the pipeline produces. These build them by hand —
// including the mechanical parts, deliberately, so that "none of it is kept" is a claim
// about what the recorder does rather than about what the fixture happened to omit.

// outcome builds a completed request as the pipeline reports one.
func outcome(request string, in directorapi.Intent, target directorapi.ResolvedTarget,
	node actiongraph.NodeID, verified bool) execute.Outcome {

	status := directorapi.ActionSucceeded
	if !verified {
		status = directorapi.ActionFailed
	}
	rec := &directorapi.ActionRecord{
		ID: directorapi.ActionID(node), RequestedIntent: request,
		Target: target, Success: verified, Status: status,
		Verification: directorapi.VerificationResult{
			Success: verified,
			Evidence: []directorapi.Evidence{
				{Kind: "focus_changed", Observed: true, Detail: "focus moved to " + target.Label},
			},
		},
	}
	n := &actiongraph.ActionNode{ID: node}
	n.ResolvedTarget.App = "explorer"
	n.ResolvedTarget.Label = target.Label
	n.ResolvedTarget.Identity.NativeID = target.NativeID

	out := execute.Outcome{
		Request: request, Intent: in, Record: rec, Node: n,
		Status: directorapi.ResultDone,
	}
	if !verified {
		out.Status = directorapi.ResultFailed
	}
	return out
}

// mechanical is a resolved target with every handle a real one carries.
func mechanical(label, native string) directorapi.ResolvedTarget {
	return directorapi.ResolvedTarget{
		ElementID: "e42", WindowID: "hwnd:1", NativeID: native,
		Point: directorapi.Point{X: 921, Y: 381},
		Role:  directorapi.RoleButton, Label: label, Confidence: 0.9,
	}
}

func semanticIntent(verb directorapi.SemanticActionKind, phrase string) directorapi.Intent {
	return directorapi.Intent{
		Kind: directorapi.IntentAct, Verb: intent.SemanticVerb, Raw: phrase,
		Parameters: map[string]any{intent.SemanticKindParam: string(verb)},
		Targets: []directorapi.ReferenceExpression{{
			Phrase: phrase, Kind: directorapi.ReferenceLiteral,
		}},
	}
}

func editIntent(text string) directorapi.Intent {
	return directorapi.Intent{
		Kind: directorapi.IntentAct, Verb: "edit", Raw: "set the name", Text: text,
		Parameters: map[string]any{intent.EditOperation: "set_text"},
		Targets: []directorapi.ReferenceExpression{{
			Phrase: "the rename editor", Kind: directorapi.ReferenceAnaphoric,
			RequiresEditor: true,
		}},
	}
}

// ── what is kept ──────────────────────────────────────────────────────────────

// TestTheRecorderKeepsSemanticsAndNoMechanics.
func TestTheRecorderKeepsSemanticsAndNoMechanics(t *testing.T) {
	r := demo.NewRecorder()
	r.Now = func() time.Time { return t0 }
	if _, err := r.Start("demo-1"); err != nil {
		t.Fatalf("start: %v", err)
	}

	r.Observe(outcome("click Rename",
		semanticIntent(directorapi.SemanticInvoke, "Rename"),
		mechanical("Rename", "uia:7"), "action_1", true))

	d, err := r.Stop()
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(d.Steps) != 1 {
		t.Fatalf("%d steps recorded", len(d.Steps))
	}
	s := d.Steps[0]
	if s.Semantic != directorapi.SemanticInvoke {
		t.Errorf("verb = %q", s.Semantic)
	}
	// The semantic ROLE, recovered from the label — which is what makes the learned
	// procedure work on a machine in another language.
	if s.Target.Role == "" {
		t.Errorf("the control's semantic role was not recovered: %+v", s.Target)
	}
	if s.Node != "action_1" {
		t.Errorf("the step does not reference its action-graph node: %q", s.Node)
	}

	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"uia:7", "hwnd:1", "e42", "921", "381"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("the demonstration kept %q, which is a mechanism:\n%s", forbidden, raw)
		}
	}
}

// TestADemonstrationReferencesNodesRatherThanCopyingThem.
//
//	Demonstrations reference Action Graph nodes. They do not duplicate them.
func TestADemonstrationReferencesNodesRatherThanCopyingThem(t *testing.T) {
	r := demo.NewRecorder()
	_, _ = r.Start("demo-2")
	r.Observe(outcome("click Rename", semanticIntent(directorapi.SemanticInvoke, "Rename"),
		mechanical("Rename", "uia:7"), "action_1", true))
	r.Observe(outcome("set the name", editIntent("Budget"),
		mechanical("Alpha.txt", "uia:9"), "action_2", true))
	d, _ := r.Stop()

	if len(d.Nodes) != 2 || d.Nodes[0] != "action_1" || d.Nodes[1] != "action_2" {
		t.Fatalf("lineage = %v, want both nodes in order", d.Nodes)
	}
	raw, _ := json.Marshal(d)
	// A copied node would bring its verification result, its plan and its resolved
	// target with it. None of that belongs here: the graph is where execution history
	// lives, and there is one of it.
	for _, forbidden := range []string{"\"plan\"", "\"resolved_target\"", "\"outcome\""} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("the demonstration duplicated graph content (%s):\n%s", forbidden, raw)
		}
	}
}

// TestAnUnverifiedActionIsNotSilentlyKeptAsAStep — recording cannot make an action into
// something it was not.
func TestAnUnverifiedActionIsNotSilentlyKeptAsAStep(t *testing.T) {
	r := demo.NewRecorder()
	_, _ = r.Start("demo-3")
	r.Observe(outcome("click Rename", semanticIntent(directorapi.SemanticInvoke, "Rename"),
		mechanical("Rename", "uia:7"), "action_1", false))
	d, _ := r.Stop()

	if len(d.Steps) == 1 && d.Steps[0].Verified {
		t.Fatal("a failed action was recorded as a verified step")
	}
	if d.Status != demo.Refused {
		t.Errorf("status = %s; a demonstration containing an unverified action cannot be "+
			"learned from", d.Status)
	}
}

// TestNothingIsRecordedOutsideASession.
func TestNothingIsRecordedOutsideASession(t *testing.T) {
	r := demo.NewRecorder()
	r.Observe(outcome("click Rename", semanticIntent(directorapi.SemanticInvoke, "Rename"),
		mechanical("Rename", "uia:7"), "action_1", true))

	if r.Recording() {
		t.Fatal("the recorder believes a session is open")
	}
	if _, err := r.Stop(); err == nil {
		t.Error("stopping with no session open did not report it")
	}
}

// TestASecondSessionIsRefused — two demonstrations of one desktop cannot be told apart.
func TestASecondSessionIsRefused(t *testing.T) {
	r := demo.NewRecorder()
	if _, err := r.Start("demo-a"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := r.Start("demo-b"); err == nil {
		t.Fatal("a second demonstration was opened over an open one")
	}
}

// TestAnApplicationChangeIsRecordedAndRefusesTheProcedure — a procedure is registered for
// an application, and one that spanned two is not one procedure.
func TestAnApplicationChangeIsRecordedAndRefusesTheProcedure(t *testing.T) {
	r := demo.NewRecorder()
	_, _ = r.Start("demo-4")

	first := outcome("click Rename", semanticIntent(directorapi.SemanticInvoke, "Rename"),
		mechanical("Rename", "uia:7"), "action_1", true)
	r.Observe(first)

	second := outcome("click Save", semanticIntent(directorapi.SemanticInvoke, "Save"),
		mechanical("Save", "uia:8"), "action_2", true)
	second.Node.ResolvedTarget.App = "notepad"
	r.Observe(second)

	d, _ := r.Stop()
	if d.Application != "" {
		t.Errorf("application = %q; the session crossed two", d.Application)
	}
	if len(d.Notes) == 0 {
		t.Error("the change of application was not recorded")
	}
}

// ── safety (Part 15) ──────────────────────────────────────────────────────────

// TestACredentialDemonstrationIsRefusedAndItsValueNeverStored.
func TestACredentialDemonstrationIsRefusedAndItsValueNeverStored(t *testing.T) {
	r := demo.NewRecorder()
	_, _ = r.Start("demo-login")
	r.Observe(outcome("focus the password box",
		semanticIntent(directorapi.SemanticSelect, "Password"),
		mechanical("Password", "uia:1"), "action_1", true))
	r.Observe(outcome("type it", editIntent("hunter2"),
		mechanical("Password", "uia:1"), "action_2", true))

	d, _ := r.Stop()
	if d.Status != demo.Refused {
		t.Fatalf("status = %s, want refused", d.Status)
	}
	if !strings.HasPrefix(d.Refusal, demo.RefusalMessage) {
		t.Errorf("the refusal does not lead with the fixed message: %s", d.Refusal)
	}
	raw, _ := json.Marshal(d)
	if strings.Contains(string(raw), "hunter2") {
		t.Fatalf("the password was written into the demonstration:\n%s", raw)
	}
	if out := demo.Extract(d); out.OK() {
		t.Fatal("a refused demonstration was extracted from anyway")
	}
}

// TestAnAuthenticationFlowIsRefused.
func TestAnAuthenticationFlowIsRefused(t *testing.T) {
	r := demo.NewRecorder()
	_, _ = r.Start("demo-auth")
	r.Observe(outcome("click Sign in", semanticIntent(directorapi.SemanticInvoke, "Sign in"),
		mechanical("Sign in", "uia:1"), "action_1", true))
	r.Observe(outcome("click Continue", semanticIntent(directorapi.SemanticInvoke, "Continue"),
		mechanical("Continue", "uia:2"), "action_2", true))

	d, _ := r.Stop()
	if d.Status != demo.Refused || !strings.Contains(d.Refusal, "authentication") {
		t.Fatalf("status = %s, refusal = %q", d.Status, d.Refusal)
	}
}

// TestAPaymentFlowIsRefused.
func TestAPaymentFlowIsRefused(t *testing.T) {
	r := demo.NewRecorder()
	_, _ = r.Start("demo-pay")
	r.Observe(outcome("click Checkout", semanticIntent(directorapi.SemanticInvoke, "Checkout"),
		mechanical("Checkout", "uia:1"), "action_1", true))
	r.Observe(outcome("click Place order",
		semanticIntent(directorapi.SemanticConfirm, "Place order"),
		mechanical("Place order", "uia:2"), "action_2", true))

	d, _ := r.Stop()
	if d.Status != demo.Refused || !strings.Contains(d.Refusal, "payment") {
		t.Fatalf("status = %s, refusal = %q", d.Status, d.Refusal)
	}
}

// TestADestructiveDemonstrationIsRefused.
func TestADestructiveDemonstrationIsRefused(t *testing.T) {
	r := demo.NewRecorder()
	_, _ = r.Start("demo-delete")
	r.Observe(outcome("select this file", semanticIntent(directorapi.SemanticSelect, "Alpha.txt"),
		mechanical("Alpha.txt", "uia:1"), "action_1", true))
	r.Observe(outcome("click Delete", semanticIntent(directorapi.SemanticInvoke, "Delete"),
		mechanical("Delete", "uia:2"), "action_2", true))

	d, _ := r.Stop()
	if d.Status != demo.Refused {
		t.Fatalf("status = %s, want refused: %q", d.Status, d.Refusal)
	}
	if !strings.Contains(d.Refusal, "removes or discards") {
		t.Errorf("the refusal does not say why: %s", d.Refusal)
	}
}

// TestAnActionThatNeededConfirmationIsRefused.
//
// Agreeing to something once is not agreeing to a procedure that does it on demand.
func TestAnActionThatNeededConfirmationIsRefused(t *testing.T) {
	r := demo.NewRecorder()
	_, _ = r.Start("demo-confirmed")

	first := outcome("click Discard", semanticIntent(directorapi.SemanticInvoke, "Discard changes"),
		mechanical("Discard changes", "uia:1"), "action_1", true)
	first.Binding = &execute.BindingDiagnostics{Confirmation: execute.ConfirmationAccepted}
	r.Observe(first)
	r.Observe(outcome("click Close", semanticIntent(directorapi.SemanticClose, "Close"),
		mechanical("Close", "uia:2"), "action_2", true))

	d, _ := r.Stop()
	if d.Status != demo.Refused {
		t.Fatalf("status = %s, want refused", d.Status)
	}
	if len(d.Confirmed) == 0 {
		t.Error("the confirmation was not recorded as the reason it can be refused for")
	}
}

// TestAnAbandonedSessionKeepsWhatItSaw — for the user to look at, never to learn from.
func TestAnAbandonedSessionKeepsWhatItSaw(t *testing.T) {
	r := demo.NewRecorder()
	_, _ = r.Start("demo-abandon")
	r.Observe(outcome("click Rename", semanticIntent(directorapi.SemanticInvoke, "Rename"),
		mechanical("Rename", "uia:7"), "action_1", true))

	d, err := r.Abandon("the user changed their mind")
	if err != nil {
		t.Fatalf("abandon: %v", err)
	}
	if d.Status != demo.Abandoned || len(d.Steps) != 1 {
		t.Fatalf("status = %s with %d step(s)", d.Status, len(d.Steps))
	}
	if out := demo.Extract(d); out.OK() {
		t.Fatal("an abandoned session became a procedure")
	}
}

// TestStoppingReportsTheFinishedSessionToItsOwner — the seam the service persists through.
func TestStoppingReportsTheFinishedSessionToItsOwner(t *testing.T) {
	var got *demo.Demonstration
	r := demo.NewRecorder()
	r.OnComplete = func(d *demo.Demonstration) { got = d }

	_, _ = r.Start("demo-5")
	r.Observe(outcome("click Rename", semanticIntent(directorapi.SemanticInvoke, "Rename"),
		mechanical("Rename", "uia:7"), "action_1", true))
	if _, err := r.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got == nil || got.ID != "demo-5" {
		t.Fatalf("the finished session was not reported: %+v", got)
	}
}
