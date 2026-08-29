package shadow_test

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// AN EXPENSIVE SENSOR DOES NOT RUN AGAINST A READING THAT ALREADY ANSWERS THE QUESTION.
//
// # The measurement behind the gate
//
// 37C ran ScreenParser over six coherent desktop moments and compared it against what
// production perception already believed. Of 473 detections, every one of the 302 that matched
// no accessibility element at IoU still had its centre inside an element production already
// perceived. Nothing actionable was added. It cost 645–1379ms a frame to add nothing.
//
// So the rule is narrow and evidence-backed: when the primary reading is sufficient, do not
// spend. It is not a heuristic about how busy the machine is or how recently the model ran —
// the cadence gate beside it already answers those.
//
// # And it declines only on a positive answer
//
// The dangerous version of this gate is one that defaults closed. No session, no memory,
// nothing settled yet — all of those are Marco not knowing whether the reading suffices, and a
// gate that read them as "no need" would silently end the experiment it gates while looking
// like an optimisation. Same shape as Provenance.OnlyDescribesPixels: deny on evidence, never
// on absence.

// countingInner records how many inferences actually started.
type countingInner struct{ ran int }

func (c *countingInner) Name() string { return "screenparser" }
func (c *countingInner) Sources() []observation.Source {
	return []observation.Source{directorapi.SourceVision}
}
func (c *countingInner) Observe(context.Context, observation.Request) (
	[]observation.Observation, error) {
	return nil, nil
}
func (c *countingInner) ObserveTargeted(context.Context,
	observation.Request) observation.ProviderOutcome {
	c.ran++
	return observation.ProviderOutcome{}
}

func TestASufficientReadingDoesNotBuyAnInference(t *testing.T) {
	clock := time.Now()
	inner := &countingInner{}
	worth := true
	p := shadow.NewProviderWithClock(inner, time.Millisecond, func() time.Time {
		clock = clock.Add(time.Second) // never rate-limited, so only the gate can decline
		return clock
	}).WhenWorthIt(func() bool { return worth })

	// Worth buying: it runs.
	p.ObserveTargeted(context.Background(), observation.Request{})
	if inner.ran != 1 {
		t.Fatalf("the detector ran %d times when more evidence was worth buying, want 1",
			inner.ran)
	}

	// Not worth buying: it does not.
	worth = false
	for i := 0; i < 5; i++ {
		p.ObserveTargeted(context.Background(), observation.Request{})
	}
	if inner.ran != 1 {
		t.Errorf("the detector ran %d times against a reading that already answered the "+
			"question, want 1.\n37C measured what each of those costs: 645–1379ms to add "+
			"no actionable semantic item.", inner.ran)
	}

	// And the declines are COUNTED. An experiment that stopped running must never look
	// like one that ran and found nothing — the icon_detect 0% taught this project that.
	if got := p.Snapshot().SkippedUnneeded; got != 5 {
		t.Errorf("%d declines recorded, want 5. A silent gate is indistinguishable from "+
			"a detector that found nothing.", got)
	}

	// Worth again: it resumes. A gate that latched would be a different thing entirely.
	worth = true
	p.ObserveTargeted(context.Background(), observation.Request{})
	if inner.ran != 2 {
		t.Errorf("the detector ran %d times after evidence became worth buying again, "+
			"want 2 — the gate latched", inner.ran)
	}
}

// With no gate wired, everything runs. This is the pre-37E behaviour and the default.
func TestAnUngatedDetectorStillRuns(t *testing.T) {
	clock := time.Now()
	inner := &countingInner{}
	p := shadow.NewProviderWithClock(inner, time.Millisecond, func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	})
	for i := 0; i < 3; i++ {
		p.ObserveTargeted(context.Background(), observation.Request{})
	}
	if inner.ran != 3 {
		t.Errorf("an ungated detector ran %d of 3 times.\nA nil gate must mean `always "+
			"worth it`, or wiring one becomes a silent behaviour change everywhere it "+
			"is not set.", inner.ran)
	}
}

// A declined inference does not consume the cadence slot.
//
// The gate is asked BEFORE the slot is claimed. If it were asked after, a run of sufficient
// readings would keep re-basing the rate limiter, and the first reading that genuinely needed
// an inference would be told to wait for a cadence it never actually used.
func TestADeclinedInferenceDoesNotSpendTheSlot(t *testing.T) {
	clock := time.Now()
	inner := &countingInner{}
	worth := false
	p := shadow.NewProviderWithClock(inner, time.Hour, func() time.Time { return clock }).
		WhenWorthIt(func() bool { return worth })

	for i := 0; i < 4; i++ {
		p.ObserveTargeted(context.Background(), observation.Request{})
	}
	worth = true
	p.ObserveTargeted(context.Background(), observation.Request{})
	if inner.ran != 1 {
		t.Errorf("after four declines the first needed inference ran %d times, want 1.\n"+
			"Declining must not claim the cadence slot, or refusing to spend becomes a "+
			"reason not to spend later.", inner.ran)
	}
	if got := p.Snapshot().SkippedRate; got != 0 {
		t.Errorf("%d rate skips recorded; a declined inference was counted as one that "+
			"was too soon", got)
	}
}
