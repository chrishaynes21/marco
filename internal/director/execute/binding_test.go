package execute

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Binding enforcement on the execution path.
//
//	The binding is revalidated immediately before the first externally observable
//	action that depends on it.
//	A changed or uncertain target causes clarification, refusal, or blocking — not
//	rebinding to the newly focused object.
//
// These drive the real pipeline, not a stand-in for it: every one calls Handle and then
// asserts on what the ACTUATOR and the operations runner recorded, because "no external
// effect" is a claim about what reached the host, not about what a status string said.

// fileItem builds a list item with a real path behind it.
func fileItem(id, label, path string) directorapi.Observation {
	o := obs(id, directorapi.RoleListItem, label, rect(10, 10, 200, 24))
	o.Attributes = map[string]any{"path": path}
	return o
}

// selectedObs marks an observation as highlighted.
func selectedObs(o directorapi.Observation) directorapi.Observation {
	yes := true
	o.Selected = &yes
	return o
}

// boundIntent is what goal expansion produces for "rename this file": a deictic
// reference that declares it needs a binding and carries one.
func boundIntent(id binding.ID, label string) directorapi.Intent {
	return directorapi.Intent{
		Kind: directorapi.IntentAct, Verb: "focus", Confidence: 1, Raw: "focus this file",
		Targets: []directorapi.ReferenceExpression{{
			Phrase: "this file", Kind: directorapi.ReferenceDeictic,
			RequiresBinding: true, ExpectedKind: string(binding.KindFile),
			BindingID: string(id),
			Query:     &directorapi.ElementQuery{Label: label},
		}},
	}
}

// bindingFixture files one binding in a fresh request store and returns both.
func bindingFixture(t *testing.T, w directorapi.WorldState, phrase string) (
	context.Context, binding.ID, *binding.Binding) {

	t.Helper()
	ctx := ensureBindings(context.Background())
	r := binding.NewResolver()
	r.Now = func() time.Time { return t0 }
	b, prob := r.Resolve(phrase, binding.KindFile, &w)
	if prob != nil {
		t.Fatalf("the fixture world does not bind: %s", prob.Message)
	}
	id := binding.StoreFrom(ctx).Put(b)
	return ctx, id, b
}

// deicticWorld is a file manager with one focused file and two distractors.
func deicticWorld(at time.Time, focusedIndex int) directorapi.WorldState {
	items := []directorapi.Observation{
		obs("uia:1", directorapi.RoleWindow, "tmp", rect(0, 0, 800, 600)),
		fileItem("uia:2", "Report.txt", `C:\tmp\Report.txt`),
		fileItem("uia:3", "Report2.txt", `C:\tmp\Report2.txt`),
	}
	if focusedIndex > 0 && focusedIndex < len(items) {
		items[focusedIndex] = focused(items[focusedIndex])
	}
	return scene(at, []directorapi.Window{{
		ID: "hwnd:1", Application: "explorer", Title: "tmp",
		Bounds: rect(0, 0, 800, 600), Focused: true, Visible: true, MonitorID: "monitor:1",
	}}, items...)
}

// ── the guard ─────────────────────────────────────────────────────────────────

// TestADeicticActionWithoutABindingIsRefused — the last of the three guards, and the one
// that catches an intent assembled outside goal expansion.
func TestADeicticActionWithoutABindingIsRefused(t *testing.T) {
	h := newHarness(deicticWorld(t0, 1))
	in := boundIntent("", "Report.txt")

	out := h.pipeline.handleParsed(context.Background(), "focus this file", in, progContext())

	if out.Status != directorapi.ResultFailed {
		t.Fatalf("status = %s; an unresolved deictic action must be refused", out.Status)
	}
	if !strings.Contains(out.Message, "never resolved") {
		t.Errorf("the refusal does not say why: %s", out.Message)
	}
	if len(h.focuser.targets) != 0 || len(h.actuator.clicks) != 0 {
		t.Fatal("something reached the host despite an unresolved deictic target")
	}
}

// TestAnUnknownExpectedKindIsRefused — a build that cannot check a kind must not pretend.
func TestAnUnknownExpectedKindIsRefused(t *testing.T) {
	h := newHarness(deicticWorld(t0, 1))
	in := boundIntent("b1", "Report.txt")
	in.Targets[0].ExpectedKind = "spreadsheet_row"

	out := h.pipeline.handleParsed(context.Background(), "focus this file", in, progContext())
	if out.Status != directorapi.ResultFailed {
		t.Fatalf("status = %s; an uncheckable kind must be refused", out.Status)
	}
	if len(h.focuser.targets) != 0 {
		t.Fatal("focus was requested for an object of an uncheckable kind")
	}
}

// ── revalidation ──────────────────────────────────────────────────────────────

// TestAnUnchangedBindingExecutes — the ordinary case, and the control for the rest.
func TestAnUnchangedBindingExecutes(t *testing.T) {
	w := deicticWorld(t0, 1)
	h := newHarness(w, w)
	ctx, id, _ := bindingFixture(t, w, "this file")

	out := h.pipeline.handleParsed(ctx, "focus this file", boundIntent(id, "Report.txt"),
		progContext())

	if out.Status == directorapi.ResultFailed || out.Status == directorapi.ResultBlocked {
		t.Fatalf("status = %s (%s); an unchanged binding must execute", out.Status, out.Message)
	}
	if len(h.focuser.targets) == 0 {
		t.Fatal("nothing was focused, so the action did not reach the host at all")
	}
	if !bindingStage(out, "unchanged") {
		t.Errorf("the trace does not record the binding as unchanged: %+v", out.Stages)
	}
}

// TestARefreshedBindingExecutesAndIsRecorded — the same object, re-observed after the
// tree was rebuilt.
func TestARefreshedBindingExecutesAndIsRecorded(t *testing.T) {
	win := []directorapi.Window{{
		ID: "hwnd:1", Application: "explorer", Title: "tmp",
		Bounds: rect(0, 0, 800, 600), Focused: true, Visible: true, MonitorID: "monitor:1",
	}}
	// Report.txt is focused, and a second item is also highlighted.
	before := scene(t0, win,
		obs("uia:1", directorapi.RoleWindow, "tmp", rect(0, 0, 800, 600)),
		focused(fileItem("uia:2", "Report.txt", `C:\tmp\Report.txt`)),
		selectedObs(fileItem("uia:3", "Report2.txt", `C:\tmp\Report2.txt`)),
	)
	// The highlight is gone. The world moved — so the binding must be re-established
	// rather than assumed — but it moved around the SAME file, which is still focused.
	after := scene(t0.Add(time.Second), win,
		obs("uia:1", directorapi.RoleWindow, "tmp", rect(0, 0, 800, 600)),
		focused(fileItem("uia:2", "Report.txt", `C:\tmp\Report.txt`)),
		fileItem("uia:3", "Report2.txt", `C:\tmp\Report2.txt`),
	)
	h := newHarness(after, after)
	ctx, id, _ := bindingFixture(t, before, "this file")

	out := h.pipeline.handleParsed(ctx, "focus this file", boundIntent(id, "Report.txt"),
		progContext())

	if out.Status == directorapi.ResultFailed || out.Status == directorapi.ResultBlocked {
		t.Fatalf("status = %s (%s); the same file was re-identified and must execute",
			out.Status, out.Message)
	}
	if len(h.focuser.targets) == 0 {
		t.Fatal("nothing was focused after a successful refresh")
	}
	if !bindingStage(out, "still the same object") {
		t.Errorf("the trace does not record the refresh: %+v", out.Stages)
	}
	// And the store now holds the refreshed binding, so everything downstream sees it.
	refreshed, ok := binding.StoreFrom(ctx).Get(id)
	if !ok {
		t.Fatal("the binding vanished from the store")
	}
	if len(refreshed.Refreshed) == 0 {
		t.Error("the refresh was not recorded on the binding, so a reader cannot tell " +
			"the world moved under it")
	}
	if refreshed.Resource != `C:\tmp\Report.txt` {
		t.Errorf("the refreshed binding points at %q, not the file it was made for",
			refreshed.Resource)
	}
}

// TestFocusMovingToAnotherFileStopsBeforeAnyExternalEffect is the case the whole
// milestone is about.
func TestFocusMovingToAnotherFileStopsBeforeAnyExternalEffect(t *testing.T) {
	before := deicticWorld(t0, 1)                 // Report.txt focused
	after := deicticWorld(t0.Add(time.Second), 2) // the user clicked Report2.txt
	h := newHarness(after, after)
	ctx, id, _ := bindingFixture(t, before, "this file")

	out := h.pipeline.handleParsed(ctx, "focus this file", boundIntent(id, "Report.txt"),
		progContext())

	if out.Status == directorapi.ResultDone || out.Status == directorapi.ResultPartial {
		t.Fatalf("status = %s; the focus moved and the action ran anyway", out.Status)
	}
	if len(h.focuser.targets) != 0 || len(h.actuator.clicks) != 0 ||
		len(h.operations.ops) != 0 {
		t.Fatal("something reached the host after the target changed; this is the " +
			"failure that renames the wrong file")
	}
	if !strings.Contains(out.Message, "focus moved") {
		t.Errorf("the refusal does not name the cause: %s", out.Message)
	}
	// And it did NOT quietly rebind to what is focused now.
	still, _ := binding.StoreFrom(ctx).Get(id)
	if still.Resource != `C:\tmp\Report.txt` {
		t.Errorf("the binding was re-pointed at %q; a stale binding must be refused, "+
			"never re-resolved", still.Resource)
	}
}

// TestADeletedObjectStopsBeforeAnyExternalEffect.
func TestADeletedObjectStopsBeforeAnyExternalEffect(t *testing.T) {
	before := deicticWorld(t0, 1)
	gone := scene(t0.Add(time.Second), []directorapi.Window{{
		ID: "hwnd:1", Application: "explorer", Title: "tmp",
		Bounds: rect(0, 0, 800, 600), Focused: true, Visible: true, MonitorID: "monitor:1",
	}},
		obs("uia:1", directorapi.RoleWindow, "tmp", rect(0, 0, 800, 600)),
		focused(fileItem("uia:3", "Report2.txt", `C:\tmp\Report2.txt`)),
	)
	h := newHarness(gone, gone)
	ctx, id, _ := bindingFixture(t, before, "this file")

	out := h.pipeline.handleParsed(ctx, "focus this file", boundIntent(id, "Report.txt"),
		progContext())

	if out.Status == directorapi.ResultDone {
		t.Fatalf("status = %s; the object is gone and the action ran anyway", out.Status)
	}
	if len(h.focuser.targets) != 0 {
		t.Fatal("focus was requested for an object that is no longer there")
	}
}

// TestALookalikeInAnotherFolderDoesNotSatisfyABinding — a label is not identity.
func TestALookalikeInAnotherFolderDoesNotSatisfyABinding(t *testing.T) {
	before := deicticWorld(t0, 1) // C:\tmp\Report.txt
	elsewhere := scene(t0.Add(time.Second), []directorapi.Window{{
		ID: "hwnd:1", Application: "explorer", Title: "archive",
		Bounds: rect(0, 0, 800, 600), Focused: true, Visible: true, MonitorID: "monitor:1",
	}},
		obs("uia:1", directorapi.RoleWindow, "archive", rect(0, 0, 800, 600)),
		// Same NAME, different folder. The classic wrong-file rename.
		focused(fileItem("uia:7", "Report.txt", `C:\archive\Report.txt`)),
	)
	h := newHarness(elsewhere, elsewhere)
	ctx, id, _ := bindingFixture(t, before, "this file")

	out := h.pipeline.handleParsed(ctx, "focus this file", boundIntent(id, "Report.txt"),
		progContext())

	if out.Status == directorapi.ResultDone {
		t.Fatal("a file with the same name in a different folder satisfied the binding")
	}
	if len(h.focuser.targets) != 0 {
		t.Fatal("the host was asked to act on a lookalike")
	}
}

// TestNoCapabilityCallHappensAfterAFailedRevalidation is the same claim as the tests
// above, stated once as the general property.
func TestNoCapabilityCallHappensAfterAFailedRevalidation(t *testing.T) {
	before := deicticWorld(t0, 1)
	moved := deicticWorld(t0.Add(time.Second), 2)
	h := newHarness(moved, moved)
	ctx, id, _ := bindingFixture(t, before, "this file")

	in := boundIntent(id, "Report.txt")
	in.Verb = intent.SemanticVerb
	in.Parameters = map[string]any{intent.SemanticKindParam: string(directorapi.SemanticSelect)}

	_ = h.pipeline.handleParsed(ctx, "select this file", in, progContext())

	if len(h.operations.ops) != 0 {
		t.Fatalf("%d operation(s) were lowered and run after a failed revalidation: %v",
			len(h.operations.ops), h.operations.kinds())
	}
	if h.graph.Len() != 0 {
		t.Error("an action was recorded although none was performed")
	}
}

// ── request scope ─────────────────────────────────────────────────────────────

// TestBindingsDoNotLeakBetweenRequests.
func TestBindingsDoNotLeakBetweenRequests(t *testing.T) {
	w := deicticWorld(t0, 1)
	first := ensureBindings(context.Background())
	r := binding.NewResolver()
	b, prob := r.Resolve("this file", binding.KindFile, &w)
	if prob != nil {
		t.Fatalf("fixture: %s", prob.Message)
	}
	id := binding.StoreFrom(first).Put(b)

	second := ensureBindings(context.Background())
	if _, found := binding.StoreFrom(second).Get(id); found {
		t.Fatal("a binding made in one request was visible in the next; a binding is " +
			"trustworthy for exactly one request")
	}
	if binding.StoreFrom(second).Len() != 0 {
		t.Error("the second request started with bindings it did not make")
	}
}

// TestOneRequestSharesOneStoreAcrossItsSteps — the counterpart: step 4 must act on the
// object step 1 bound.
func TestOneRequestSharesOneStoreAcrossItsSteps(t *testing.T) {
	ctx := ensureBindings(context.Background())
	again := ensureBindings(ctx)
	if binding.StoreFrom(ctx) != binding.StoreFrom(again) {
		t.Fatal("a nested call made a second store, so a later step would not find what " +
			"an earlier one bound")
	}
}

// progContext is the per-step context a program supplies, with its value and collection
// environments made — the shape handleParsed expects when a step runs.
func progContext() program.Context {
	var pctx program.Context
	pctx.EnsureValues()
	pctx.EnsureCollections()
	return pctx
}

// bindingStage reports whether the trace has a binding stage containing a phrase.
func bindingStage(out Outcome, contains string) bool {
	for _, s := range out.Stages {
		if s.Name == "binding" && strings.Contains(s.Detail, contains) {
			return true
		}
	}
	return false
}
