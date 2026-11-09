package oshost

import (
	"slices"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// A SHIFT AND A CTRL TAPPED ON THEIR OWN BEFORE A CLICK ARE PRESSED WHEN IT IS PLAYED: the slide
// melee, through the simplifier, the generator, the compiler and the host. Before ADR-339 the
// recording had both and the machine was asked for neither.
func TestATappedShiftAndCtrlArePressedWhenPlayed(t *testing.T) {
	ev := pressesWithButtons("forward")
	k := func(name string, down bool, ms int) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: name, Down: down,
			App: "arkascended", T: when(ms)}
	}
	taps := []recorder.RecordedEvent{k("shift", true, 20), k("shift", false, 40),
		k("ctrl", true, 60), k("ctrl", false, 80)}
	ev = append(ev[:1], append(taps, ev[1:]...)...)
	src, at := savedFrom(t, orchestrator.ScopeAppAnywhere, ev)
	rec, h, out := played(t, at)
	if !slices.Equal(rec.keys, []string{"shift", "ctrl"}) || len(rec.clicks) != 1 {
		t.Fatalf("the machine was asked for keys %v and clicks %v\n%s\n%s", rec.keys, rec.clicks, src, out)
	}
	if undone := h.Unperformed(); len(undone) != 0 {
		t.Fatalf("the run reports %v", undone)
	}
}
