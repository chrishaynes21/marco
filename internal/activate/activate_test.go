package activate

import (
	"context"
	"errors"
	"testing"
)

// The one answer to "press this control".
//
// # Why the ladder's CONTENT is asserted here by name
//
// Because every other test compares one caller against `Ladder` itself, which proves the two
// paths agree and nothing about what they agree on. A mutation that deletes a rung shrinks the
// expectation with it and every such test still passes — verified: removing Expand survived the
// whole suite.
//
// So this file is the fixed point. Each rung is named, in order, with the reason it is there.

// Every way of pressing is present, in order.
//
// Deleting a rung must fail here, whatever else agrees.
func TestTheLadderHasEveryWayInOrder(t *testing.T) {
	want := []Way{Invoke, Select, Expand, Toggle}
	if len(Ladder) != len(want) {
		t.Fatalf("the ladder has %d rung(s): %v", len(Ladder), Ladder)
	}
	for i, w := range want {
		if Ladder[i] != w {
			t.Errorf("rung %d is %q, want %q", i+1, Ladder[i], w)
		}
	}
	// And each by name, with why, so a deletion fails against a reason rather than a count.
	present := map[Way]bool{}
	for _, w := range Ladder {
		present[w] = true
	}
	for w, why := range map[Way]string{
		Invoke: "a button doing its one thing",
		Select: "a navigation or list item becoming the chosen one — Windows Settings is " +
			"built almost entirely from these, and a ladder without it fails every " +
			"navigation step on the most obvious application to teach Marco with",
		Expand: "a disclosure opening",
		Toggle: "a checkbox or switch changing state",
	} {
		if !present[w] {
			t.Errorf("%q is missing from the ladder: it is %s", w, why)
		}
	}
}

// Toggle is last.
//
// It is the only way whose effect depends on what the control was already doing. Trying it before
// the others could switch something off that somebody meant to press, and a demonstration would
// be reproduced backwards.
func TestToggleIsLast(t *testing.T) {
	if Ladder[len(Ladder)-1] != Toggle {
		t.Errorf("the last rung is %q, want toggle", Ladder[len(Ladder)-1])
	}
	if Ladder[0] != Invoke {
		t.Errorf("the first rung is %q, want invoke", Ladder[0])
	}
}

// tried records the ways an attempt was asked for.
type tried struct {
	ways        []Way
	unsupported map[Way]bool
	broken      map[Way]bool
}

func (r *tried) attempt(_ context.Context, w Way) (bool, error) {
	r.ways = append(r.ways, w)
	switch {
	case r.unsupported[w]:
		return true, errors.New("unsupported: no such pattern")
	case r.broken[w]:
		return false, errors.New("the bridge went away")
	}
	return false, nil
}

// An unsupported way licenses the next one.
func TestAnUnsupportedWayFallsThrough(t *testing.T) {
	r := &tried{unsupported: map[Way]bool{Invoke: true, Select: true}}
	got, err := Activate(context.Background(), r.attempt)
	if err != nil {
		t.Fatalf("activation failed: %v", err)
	}
	if got != Expand {
		t.Errorf("activated by %q, want expand", got)
	}
	if len(r.ways) != 3 {
		t.Errorf("tried %v, want three ways", r.ways)
	}
}

// A real failure stops the ladder.
//
// THE load-bearing distinction. "This control has no such pattern" invites another way; "this
// control has the pattern and it went wrong" does not. Retrying that under a different name is
// Marco pressing a control repeatedly in different ways until something happens, which is the one
// thing an authorised attempt must never do.
//
// Deleting the unsupported check must fail this.
func TestARealFailureStopsTheLadder(t *testing.T) {
	r := &tried{broken: map[Way]bool{Invoke: true}}
	if _, err := Activate(context.Background(), r.attempt); err == nil {
		t.Fatal("a real failure reported success")
	}
	if len(r.ways) != 1 {
		t.Errorf("a control whose Invoke FAILED was pressed %d more way(s): %v",
			len(r.ways)-1, r.ways)
	}
}

// The first way that works ends it.
func TestTheFirstWayThatWorksWins(t *testing.T) {
	r := &tried{}
	got, err := Activate(context.Background(), r.attempt)
	if err != nil || got != Invoke {
		t.Fatalf("activated by %q (%v), want invoke", got, err)
	}
	if len(r.ways) != 1 {
		t.Errorf("an ordinary button was pressed %d way(s): %v", len(r.ways), r.ways)
	}
}

// A control implementing nothing is refused, having tried each way once.
func TestAControlThatImplementsNothingIsRefused(t *testing.T) {
	r := &tried{unsupported: map[Way]bool{
		Invoke: true, Select: true, Expand: true, Toggle: true,
	}}
	if _, err := Activate(context.Background(), r.attempt); err == nil {
		t.Fatal("a control implementing nothing reported success")
	}
	if len(r.ways) != len(Ladder) {
		t.Errorf("tried %d way(s), want each rung exactly once: %v", len(r.ways), r.ways)
	}
}

// The host's refusal contract is recognised in one place.
//
// The `unsupported:` prefix is the accessibility host's own, and its comment says it exists "so
// the Director can fall back and RECORD that it did". Both callers must read it the same way, so
// only one of them reads it.
func TestTheUnsupportedContractIsRecognised(t *testing.T) {
	if !Unsupported("unsupported: the control does not implement InvokePattern") {
		t.Error("the host's own refusal was not recognised as a missing pattern")
	}
	if Unsupported("the bridge went away") {
		t.Error("an ordinary failure was read as a missing pattern, which would make " +
			"Marco press the control three more ways after something went wrong")
	}
}
