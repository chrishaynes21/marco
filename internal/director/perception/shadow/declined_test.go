package shadow_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
)

// A slot the purchase policy declined must not describe itself as a cadence skip.
//
// It did, for as long as there was one skip reason. The two have opposite remedies — one is
// the gate pacing itself and is working, the other is the experiment being switched off by a
// claim about a different kind of evidence — and they were indistinguishable downstream, which
// is how a live Trackmania session came to report 77 cadence skips for a detector the policy
// had declined.

// The fake inner provider is countingInner, from worth_test.go — the same one the gate
// tests use, so there is one account of what "ran" means in this package.

func TestADeclinedSlotSaysItWasDeclinedAndNotPaced(t *testing.T) {
	inner := &countingInner{}
	p := shadow.NewProviderWithClock(inner, time.Second, time.Now).
		WhenWorthIt(func() bool { return false })

	out := p.ObserveTargeted(context.Background(), observation.Request{})
	if inner.ran != 0 {
		t.Fatalf("precondition: the inner provider ran %d time(s) behind a closed gate", inner.ran)
	}
	if !strings.Contains(out.Reason, shadow.DeclinedReason) {
		t.Errorf("a declined slot reports %q, which does not say it was declined", out.Reason)
	}
	if strings.Contains(out.Reason, "cadence") {
		t.Errorf("a declined slot describes itself as a cadence skip: %q", out.Reason)
	}
	// Still a skipped slot, so everything that already counted skips keeps working.
	if !strings.Contains(out.Reason, "shadow slot skipped") {
		t.Errorf("a declined slot is no longer recognisable as a skipped one: %q", out.Reason)
	}
}

func TestACadenceSkipStillSaysCadence(t *testing.T) {
	inner := &countingInner{}
	now := time.Now()
	p := shadow.NewProviderWithClock(inner, time.Hour, func() time.Time { return now })

	if first := p.ObserveTargeted(context.Background(), observation.Request{}); first.Reason ==
		"" && inner.ran == 0 {
		t.Fatal("precondition: the first slot did not run")
	}
	out := p.ObserveTargeted(context.Background(), observation.Request{})
	if !strings.Contains(out.Reason, "cadence") {
		t.Errorf("a paced slot reports %q", out.Reason)
	}
	if strings.Contains(out.Reason, shadow.DeclinedReason) {
		t.Errorf("a paced slot claims the reading declined it: %q", out.Reason)
	}
}
