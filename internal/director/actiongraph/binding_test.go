package actiongraph_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Binding metadata in durable history.
//
//	Old graphs without binding metadata still decode. Re-encoding old graphs does not
//	invent a binding. Binding metadata is audit and targeting information, not
//	authorization. Avoid persisting ephemeral pointers or implementation objects.

var bindingAt = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

func liveBinding() *binding.Binding {
	return &binding.Binding{
		ID: "b1", Phrase: "this file",
		Expected: binding.KindFile, Resolved: binding.KindFile,
		ElementID: "e2", NativeID: "uia:2", Resource: `C:\tmp\Report.txt`,
		Application: "explorer", WindowID: "hwnd:1", WindowTitle: "tmp",
		Label: "Report.txt",
		Evidence: []binding.Evidence{
			{Kind: "focus", Detail: `"Report.txt" holds focus`},
			{Kind: "backing_path", Detail: `C:\tmp\Report.txt`, Decisive: true},
		},
		Stability: "e2|file|C:\\tmp\\Report.txt|hwnd:1|true;",
		Sequence:  1, ResolvedAt: bindingAt, Confidence: 1,
		Refreshed: []string{"the same file, re-observed"},
		Origin: binding.Origin{
			Goal: "rename", Procedure: "explorer rename", StepID: "s1", StepIndex: 1,
			Request: "rename this file to Budget",
		},
	}
}

// ── round trip ────────────────────────────────────────────────────────────────

// TestABoundNodeSurvivesSerialisation.
func TestABoundNodeSurvivesSerialisation(t *testing.T) {
	node := actiongraph.ActionNode{
		ID: "n1", Timestamp: bindingAt, Goal: "rename this file to Budget",
		Binding: liveBinding().Snapshot(), Confirmation: "accepted",
	}

	raw, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back actiongraph.ActionNode
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !back.Binding.Bound() {
		t.Fatal("the binding did not survive the round trip")
	}
	got, want := back.Binding, node.Binding
	if got.Resource != want.Resource || got.NativeID != want.NativeID ||
		got.Expected != want.Expected || got.Resolved != want.Resolved ||
		got.Application != want.Application || got.WindowID != want.WindowID ||
		got.Label != want.Label || got.Confidence != want.Confidence {
		t.Errorf("identity changed across the round trip:\n got %+v\nwant %+v", got, want)
	}
	if len(got.Evidence) != len(want.Evidence) || !got.Evidence[1].Decisive {
		t.Errorf("the evidence did not survive: %+v", got.Evidence)
	}
	if len(got.Refreshed) != 1 {
		t.Errorf("the refresh history did not survive: %+v", got.Refreshed)
	}
	if got.Origin.Procedure != "explorer rename" || got.Origin.StepIndex != 1 {
		t.Errorf("the provenance did not survive: %+v", got.Origin)
	}
	if back.Confirmation != "accepted" {
		t.Errorf("confirmation = %q, want the recorded outcome", back.Confirmation)
	}
}

// TestTheStabilityTokenIsStoredAsADigestNotAsItself.
//
//	Avoid persisting ephemeral pointers or implementation objects.
//
// The token is a summary of everything focusable at the time — large, and a description of
// the user's screen. Durable history keeps a digest, which answers the only question a
// replay asks of it.
func TestTheStabilityTokenIsStoredAsADigestNotAsItself(t *testing.T) {
	live := liveBinding()
	snap := live.Snapshot()

	if snap.StabilityDigest == "" {
		t.Fatal("no digest was recorded, so a reader cannot tell one world from another")
	}
	if strings.Contains(snap.StabilityDigest, "Report.txt") ||
		strings.Contains(snap.StabilityDigest, "hwnd:1") {
		t.Errorf("the raw token was stored: %q", snap.StabilityDigest)
	}
	raw, _ := json.Marshal(snap)
	if strings.Contains(string(raw), live.Stability) {
		t.Error("the serialised snapshot contains the stability token verbatim")
	}
}

// TestASnapshotCarriesNoCoordinates.
func TestASnapshotCarriesNoCoordinates(t *testing.T) {
	raw, err := json.Marshal(liveBinding().Snapshot())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{`"x"`, `"y"`, "bounds", "point", "click_point"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("the snapshot contains %s: %s", forbidden, raw)
		}
	}
}

// TestASnapshotDoesNotAliasTheLiveBinding — durable history may not change when the
// binding it came from is refreshed.
func TestASnapshotDoesNotAliasTheLiveBinding(t *testing.T) {
	live := liveBinding()
	snap := live.Snapshot()

	live.Refreshed = append(live.Refreshed, "the window changed")
	live.Evidence = append(live.Evidence, binding.Evidence{Kind: "later", Detail: "x"})

	if len(snap.Refreshed) != 1 || len(snap.Evidence) != 2 {
		t.Fatalf("the snapshot changed when the live binding did: %+v", snap)
	}
}

// ── compatibility ─────────────────────────────────────────────────────────────

// oldNode is a node exactly as a build before bindings would have written it.
const oldNode = `{
  "id": "n7",
  "timestamp": "2026-07-01T09:15:00Z",
  "intent": {"kind": "act", "verb": "click", "confidence": 1, "raw": "click Save"},
  "goal": "click Save",
  "plan": {"goal": "click Save", "risk": "low", "steps": [
    {"index": 0, "action": {"type": "click", "query": {"label": "Save"}, "description": "Save"}}
  ]},
  "requested_target": {"phrase": "Save", "kind": "literal"},
  "resolved_target": {"element_id": "e4", "label": "Save", "role": "button",
    "bounds": {"x": 10, "y": 10, "width": 60, "height": 24},
    "identity": {"native_id": "uia:4", "label_unique": true, "durable": true},
    "confidence": 1},
  "verification": {"success": true},
  "outcome": {"success": true, "status": "succeeded",
    "before": {"fingerprint": "a"}, "after": {"fingerprint": "b"}}
}`

// TestAnOldGraphWithoutBindingsStillDecodes.
func TestAnOldGraphWithoutBindingsStillDecodes(t *testing.T) {
	var node actiongraph.ActionNode
	if err := json.Unmarshal([]byte(oldNode), &node); err != nil {
		t.Fatalf("a node written before bindings existed no longer decodes: %v", err)
	}
	if node.ID != "n7" || node.Goal != "click Save" {
		t.Fatalf("the node decoded wrongly: %+v", node)
	}
	if node.Binding.Bound() {
		t.Fatal("a binding appeared on a node that never had one")
	}
	// And it is still replayable: a non-deictic action needs no binding.
	spec, ok := node.Action()
	if !ok {
		t.Fatal("the old node lost its action")
	}
	if _, err := spec.Rebuild(); err != nil {
		t.Fatalf("an old non-deictic node is no longer replayable: %v", err)
	}
}

// TestReEncodingAnOldGraphDoesNotInventABinding.
func TestReEncodingAnOldGraphDoesNotInventABinding(t *testing.T) {
	var node actiongraph.ActionNode
	if err := json.Unmarshal([]byte(oldNode), &node); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	raw, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), `"binding"`) {
		t.Errorf("re-encoding invented a binding field: %s", raw)
	}
	if strings.Contains(string(raw), `"confirmation"`) {
		t.Errorf("re-encoding invented a confirmation record: %s", raw)
	}
}

// TestAnOldDeicticNodeWithoutIdentityIsRefusedRatherThanGuessed.
//
//	An old graph representing a deictic action without sufficient target identity must
//	be refused at execution rather than guessed.
//
// Asserted here at the level the graph owns: a snapshot known only by a caption reports
// that it cannot be re-identified, which is what the replay path refuses on.
func TestAnOldDeicticNodeWithoutIdentityIsRefusedRatherThanGuessed(t *testing.T) {
	labelOnly := &binding.Snapshot{
		Phrase: "this file", Expected: binding.KindFile, Resolved: binding.KindFile,
		Label: "Report.txt",
	}
	if !labelOnly.Bound() {
		t.Fatal("the fixture is not even a binding")
	}
	if labelOnly.Identified() {
		t.Fatal("a caption was accepted as identity; two files in different folders " +
			"share a name, and re-finding by caption renames the wrong one")
	}

	withPath := &binding.Snapshot{
		Phrase: "this file", Expected: binding.KindFile, Resolved: binding.KindFile,
		Label: "Report.txt", Resource: `C:\tmp\Report.txt`,
	}
	if !withPath.Identified() {
		t.Fatal("a snapshot with a path cannot be re-identified")
	}
}

// TestRestoreDropsTheStabilityTokenSoAReplayAlwaysRechecks.
func TestRestoreDropsTheStabilityTokenSoAReplayAlwaysRechecks(t *testing.T) {
	restored := liveBinding().Snapshot().Restore()
	if restored.Stability != "" {
		t.Fatalf("a restored binding carries a stability token %q, so a replay could "+
			"short-circuit the re-identification it must always perform", restored.Stability)
	}
	if restored.Resource != `C:\tmp\Report.txt` || restored.NativeID != "uia:2" {
		t.Errorf("the restored binding lost the identity it needs: %+v", restored)
	}
}

// TestSameResourceAnswersUnknownRatherThanTrueWhenEitherSideHasNoPath.
func TestSameResourceAnswersUnknownRatherThanTrueWhenEitherSideHasNoPath(t *testing.T) {
	snap := liveBinding().Snapshot()
	if _, known := binding.SameResource(snap, &binding.Binding{Resolved: binding.KindFile}); known {
		t.Fatal("a missing path was treated as evidence about sameness")
	}
	same, known := binding.SameResource(snap, &binding.Binding{
		Resolved: binding.KindFile, Resource: `c:\TMP\report.TXT`,
	})
	if !known || !same {
		t.Error("the same Windows path in different case was not recognised")
	}
}

// TestBindingMetadataIsNotAuthorization — the field records what happened; nothing on the
// node grants permission.
func TestBindingMetadataIsNotAuthorization(t *testing.T) {
	node := actiongraph.ActionNode{
		ID: "n1", Binding: liveBinding().Snapshot(), Confirmation: "accepted",
	}
	// The confirmation is a STRING, not a decision type: there is nothing here for a
	// caller to treat as a live outcome, and reading it back yields a record.
	if node.Confirmation != "accepted" {
		t.Fatalf("confirmation = %q", node.Confirmation)
	}
	raw, _ := json.Marshal(node)
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := generic["authorized"]; present {
		t.Error("the node carries an authorization field")
	}
	if _, present := generic["permitted"]; present {
		t.Error("the node carries a permission field")
	}
}

var _ = directorapi.ActionSemantic
