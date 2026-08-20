package execute

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The persistence audit.
//
//	Sensitive data may exist in memory long enough to execute the requested
//	effect, but it may not become history.
//
// The marker below is deliberately unlike anything that occurs naturally, so a hit
// anywhere in a serialised artifact is a leak and never a coincidence.

const marker = "ZZQX-SENSITIVE-MARKER-7f31d9-XYZZY"

// sensitiveProgram runs a capture-then-consume program whose captured value is the
// marker, and returns everything durable or diagnostic that came out of it.
func sensitiveProgram(t *testing.T) (*harness, ProgramOutcome) {
	t.Helper()
	h, _, vals := captureHarness(t, fieldScenes(8)...)
	// A control value is classified sensitive, which is what puts the redaction rule
	// in play. The marker is the field's contents.
	vals.value, vals.known = marker, true
	if x, ok := h.pipeline.Executor.(*Executor); ok {
		x.Editor = &recordingEditor{}
	}

	prog, err := program.Decompose(
		"remember this field's value as email and then type ${email}", testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	out := h.pipeline.RunProgram(context.Background(), prog, program.Context{}, 0)

	// The mutation RAN — the editor received the value and a node was recorded. Whether
	// the scripted world then reflected the change is beside the point here, and testing
	// redaction on a recorded FAILURE is if anything the stronger case: failures are
	// recorded too, so they are just as durable and just as capable of leaking.
	if len(out.Steps) < 2 {
		t.Fatalf("the consuming step never ran: %s (%s)", out.Status, out.Message)
	}
	if h.graph.Len() == 0 {
		t.Fatalf("no node was recorded, so there is nothing to audit")
	}
	return h, out
}

func TestASensitiveCapturedValueAppearsInNoDurableArtifact(t *testing.T) {
	h, out := sensitiveProgram(t)

	// Every artifact the spec names, serialised the way each is actually stored or
	// sent. Marshalling is the right check rather than reading fields: it is exactly
	// what a file, a trace or a service event does with these values, so a field added
	// later is covered without anyone remembering to extend the test.
	artifacts := map[string]any{
		"program result": out,
		"command result": Collapse("the request", out),
	}
	nodes, err := h.graph.Recent(50)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	artifacts["action graph"] = nodes
	for i, n := range nodes {
		artifacts["graph node "+string(n.ID)] = n
		artifacts["graph node plan "+string(rune('a'+i))] = n.Plan
	}
	for i, s := range out.Steps {
		artifacts["step outcome "+string(rune('1'+i))] = s
		if s.Record != nil {
			artifacts["step record "+string(rune('1'+i))] = s.Record
		}
		if s.Node != nil {
			artifacts["step node "+string(rune('1'+i))] = s.Node
		}
		if s.Plan != nil {
			artifacts["step plan "+string(rune('1'+i))] = s.Plan
		}
		artifacts["step intent "+string(rune('1'+i))] = s.Intent
		artifacts["step stages "+string(rune('1'+i))] = s.Stages
	}

	for name, artifact := range artifacts {
		raw, err := json.Marshal(artifact)
		if err != nil {
			t.Errorf("%s: marshal: %v", name, err)
			continue
		}
		if strings.Contains(string(raw), marker) {
			t.Errorf("%s leaked the sensitive value:\n%s", name, raw)
		}
	}
}

func TestTheMutationStillHappenedWithTheRealValue(t *testing.T) {
	// The other half, and the one that makes the test above meaningful. Redaction that
	// also broke the edit would pass every leak check and be useless.
	h, _ := sensitiveProgram(t)
	ed, ok := h.pipeline.Executor.(*Executor).Editor.(*recordingEditor)
	if !ok {
		t.Fatal("the recording editor was replaced")
	}
	if len(ed.text) == 0 {
		t.Fatal("nothing was delivered to the editor")
	}
	if ed.text[len(ed.text)-1] != marker {
		t.Fatalf("the editor received %q, want the real captured value", ed.text[len(ed.text)-1])
	}
}

func TestTheConsumingNodeRecordsTheReferenceWithoutTheValue(t *testing.T) {
	h, _ := sensitiveProgram(t)
	nodes, err := h.graph.Recent(50)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}

	var consuming *actiongraph.ActionNode
	for i := range nodes {
		if _, dep := DependedOnProgramValue(nodes[i]); dep {
			consuming = &nodes[i]
		}
	}
	if consuming == nil {
		t.Fatal("no node recorded that it consumed a program-local value")
	}
	for key, want := range map[string]string{
		MetaInputKind:       MetaInputProgramValue,
		MetaValueName:       "email",
		MetaValueKind:       string(values.KindControlValue),
		MetaValueVisibility: string(values.VisibilitySensitive),
	} {
		if got, _ := consuming.Metadata[key].(string); got != want {
			t.Errorf("metadata %s = %q, want %q", key, got, want)
		}
	}
}

func TestACaptureStepCreatesNoNodeSoHistoryIsNotFabricated(t *testing.T) {
	h, out := sensitiveProgram(t)
	nodes, _ := h.graph.Recent(50)
	// Two steps ran; only the mutation is a desktop action.
	if len(nodes) != 1 {
		t.Fatalf("the program produced %d nodes, want 1 — the capture must not create one",
			len(nodes))
	}
	if out.Steps[0].Node != nil {
		t.Fatal("the capture step produced a node")
	}
}

func TestAPublicValueIsStillDebuggable(t *testing.T) {
	// Redaction is about content that was private when read, not about where it came
	// from. Blanking a window title would make history useless while protecting nothing.
	h, _, _ := captureHarness(t, fieldScenes(8)...)
	if x, ok := h.pipeline.Executor.(*Executor); ok {
		x.Editor = &recordingEditor{}
	}
	prog, err := program.Decompose(
		"remember the window title as title and then type ${title}", testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	out := h.pipeline.RunProgram(context.Background(), prog, program.Context{}, 0)
	if len(out.Steps) < 2 {
		t.Fatalf("the consuming step never ran: %s (%s)", out.Status, out.Message)
	}
	nodes, _ := h.graph.Recent(50)
	raw, _ := json.Marshal(nodes)
	if !strings.Contains(string(raw), "Untitled - Notepad") {
		t.Fatalf("a public value was redacted out of history:\n%s", raw)
	}
	// It is still marked as a program value, so replay still refuses it.
	if _, dep := DependedOnProgramValue(nodes[0]); !dep {
		t.Fatal("a public consumed value was not recorded as one")
	}
}

// ── replay ────────────────────────────────────────────────────────────────────

func TestAnActionThatUsedAProgramValueIsNotReplayable(t *testing.T) {
	h, _ := sensitiveProgram(t)
	nodes, _ := h.graph.Recent(50)

	out := h.pipeline.Replay(context.Background(), ReplaySpec{Node: nodes[0], Count: 1})
	if out.Status != directorapi.ResultBlocked {
		t.Fatalf("status = %s, want blocked", out.Status)
	}
	if out.StoppedBecause != "not_replayable" {
		t.Fatalf("stopped because %q", out.StoppedBecause)
	}
	// The refusal NAMES the value without revealing it — the name is what the user
	// called it, and is the whole content of an honest explanation.
	if !strings.Contains(out.Message, `program-local value "email"`) {
		t.Fatalf("message = %q", out.Message)
	}
	if strings.Contains(out.Message, marker) {
		t.Fatalf("the refusal leaked the value: %q", out.Message)
	}
	if out.Completed != 0 {
		t.Fatalf("%d iterations ran despite the refusal", out.Completed)
	}
}

func TestOrdinaryReplayIsUnaffected(t *testing.T) {
	// The rule is narrow. An action that depended on nothing program-local replays
	// exactly as it always did.
	h := newHarness(menuFlow()...)
	first := h.pipeline.Handle(context.Background(), "click File")
	if first.Node == nil {
		t.Fatalf("no node: %s", first.Message)
	}
	if _, dep := DependedOnProgramValue(*first.Node); dep {
		t.Fatal("an ordinary click was marked as depending on a program value")
	}
	out := h.pipeline.Replay(context.Background(), ReplaySpec{Node: *first.Node, Count: 1})
	if out.StoppedBecause == "not_replayable" {
		t.Fatalf("an ordinary action was refused: %s", out.Message)
	}
}
