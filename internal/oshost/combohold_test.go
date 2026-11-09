package oshost

import (
	"context"
	"strings"
	"testing"
)

// A COMBO IS HELD KEY BY KEY AND LET GO OF IN REVERSE: `shift+w` is Shift down, W down, and then
// W up, Shift up. Before ADR-340 it was one key name the machine could not resolve.
func TestAComboIsHeldKeyByKeyAndReleasedInReverse(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	ctx := context.Background()
	holding(t, h, ctx, "shift+w")
	if status := releasingKey(t, h, ctx, "shift+w"); status != "ok" {
		t.Fatalf("the release answered %q", status)
	}
	if got := strings.Join(rec.calls, ","); got != "hold shift,hold w,release w,release shift" {
		t.Fatalf("the machine was asked for %q", got)
	}
	h.ReleaseHeld()
	if len(rec.keyUps) != 2 {
		t.Errorf("the sweep released keys a combo had already let go of: %v", rec.keyUps)
	}
}

// A COMBO REFUSED PART WAY LETS GO OF WHAT WENT DOWN, before it reports. Leaving Shift down
// because W was refused is every keystroke after it arriving shifted.
func TestAComboRefusedPartWayLetsGoOfWhatWentDown(t *testing.T) {
	rec := &recBackend{refuseHolds: map[string]bool{"w": true}}
	h := &Host{b: rec}
	if st := holding(t, h, context.Background(), "shift+w"); st == "ok" {
		t.Fatal("a combo whose W was refused reported ok")
	}
	if got := strings.Join(rec.calls, ","); got != "hold shift,hold w,release shift" {
		t.Fatalf("the machine was asked for %q", got)
	}
	h.ReleaseHeld()
	if len(rec.keyUps) != 1 {
		t.Errorf("the sweep found keys still held: %v", rec.keyUps)
	}
}

// AND A COMBO'S KEY THE MACHINE WILL NOT RELEASE IS STILL TRIED BY THE SWEEP, while the rest of
// the combo comes up.
func TestAComboKeyThatWillNotComeUpIsLeftForTheSweep(t *testing.T) {
	rec := &recBackend{refuseReleases: map[string]int{"w": 1}}
	h := &Host{b: rec}
	ctx := context.Background()
	holding(t, h, ctx, "shift+w")
	if status := releasingKey(t, h, ctx, "shift+w"); status == "ok" {
		t.Fatal("a release whose W was refused reported ok")
	}
	h.ReleaseHeld()
	if got := strings.Join(rec.keyUps, ","); got != "w,shift,w" {
		t.Fatalf("the releases asked for were %q, want w,shift then the sweep's w", got)
	}
}
