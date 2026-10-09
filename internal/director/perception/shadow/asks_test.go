package shadow_test

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// THE EXPERIMENT'S ACTIVATION IS NOT THE AUTHORITATIVE CYCLE'S PURCHASE DECISION.
//
// # The live failure
//
// The inner provider is opt-in on its own source: vision looks at nothing unless the request
// NAMES vision. The request this provider is handed is the authoritative cycle's, and the
// Include list on it is the authoritative purchase decision — so on a surface where the primary
// reading claims sufficiency, the request carried no pixel source and the inner provider
// returned instantly having looked at nothing.
//
// That is a second gate, sharing one channel with the first, and it is the one
// $MARCO_SHADOW_UNGATED does not remove. Measured live on Trackmania with the switch set: 122
// counted inferences, of which 116 were a provider nobody asked — 0ms each, no detections, and
// each one reported as an inference that could not prove which window it observed. The report
// said the experiment ran 122 times. It ran six.
//
// It is the worst shape a configuration switch can have: it said UNGATED and the detector was
// still gated, by a different line, in a different package.

// recordingInner remembers the request it was handed.
type recordingInner struct {
	ran  int
	last observation.Request
}

func (r *recordingInner) Name() string { return "screenparser" }
func (r *recordingInner) Sources() []observation.Source {
	return []observation.Source{directorapi.SourceVision}
}
func (r *recordingInner) Observe(context.Context, observation.Request) (
	[]observation.Observation, error) {
	return nil, nil
}
func (r *recordingInner) ObserveTargeted(_ context.Context,
	req observation.Request) observation.ProviderOutcome {
	r.ran++
	r.last = req
	return observation.ProviderOutcome{}
}

func TestTheExperimentAsksItsOwnDetectorForPixels(t *testing.T) {
	inner := &recordingInner{}
	clock := time.Now()
	p := shadow.NewProviderWithClock(inner, time.Millisecond, func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	})

	// The authoritative request a sufficient reading produces: no pixel source at all.
	p.ObserveTargeted(context.Background(), observation.Request{})

	if inner.ran != 1 {
		t.Fatalf("the inner provider was called %d times, want 1", inner.ran)
	}
	if !inner.last.Includes(directorapi.SourceVision) {
		t.Errorf("the experiment handed its own detector a request that does not ask for "+
			"%s.\nVision is opt-in on its own source, so the detector returns "+
			"StateNotRequested without looking at anything — and this provider counts "+
			"that as an inference.\nMeasured live: 116 of 122 reported inferences were "+
			"this, at 0ms each.\nInclude was %v.", directorapi.SourceVision,
			inner.last.Include)
	}
}

// And it does NOT widen the cycle fusion reads.
//
// The whole point of a shadow provider is that its evidence cannot reach belief. If asking for
// its own sources mutated the caller's request, the authoritative vision provider on the same
// Director would start running too — and its detections DO reach fusion. That would turn a
// diagnostic switch into a change in what Marco believes, which is the one thing this type
// exists to make impossible.
func TestAskingForPixelsDoesNotWidenTheAuthoritativeRequest(t *testing.T) {
	inner := &recordingInner{}
	clock := time.Now()
	p := shadow.NewProviderWithClock(inner, time.Millisecond, func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	})

	// A request with its own Include list, shared with the caller the way a real cycle
	// shares it — and with SPARE CAPACITY, which is the only shape in which the mistake is
	// observable. `append` to a full slice allocates and the aliasing bug hides; append to
	// one with room writes into the caller's array, and every later reader of that array
	// sees a source the caller never asked for.
	backing := make([]observation.Source, 4)
	backing[0] = directorapi.SourceOCR
	req := observation.Request{Include: backing[:1]}
	p.ObserveTargeted(context.Background(), req)

	if len(req.Include) != 1 || req.Include[0] != directorapi.SourceOCR {
		t.Fatalf("the caller's request was modified: Include is now %v.\nThe shadow "+
			"provider must not be able to add a source to the cycle fusion reads — the "+
			"authoritative vision provider shares that source name and its evidence is "+
			"believable.", req.Include)
	}
	if backing[1] != "" {
		t.Errorf("the experiment wrote %q into the caller's backing array past its "+
			"length.\nThe request must be widened on a COPY: the authoritative vision "+
			"provider answers to the same source name, and its detections reach fusion.",
			backing[1])
	}
	if !inner.last.Includes(directorapi.SourceOCR) {
		t.Errorf("the experiment dropped the caller's own %s request; it must ADD to the "+
			"request, not replace it", directorapi.SourceOCR)
	}
	if !inner.last.Includes(directorapi.SourceVision) {
		t.Errorf("the experiment did not ask for %s", directorapi.SourceVision)
	}
}

// A declined slot still never reaches the detector.
//
// The fix above must not become "the experiment always runs": the purchase policy and the
// cadence are this provider's own decisions and they stay in force. Only the authoritative
// request's source set stopped being a third, invisible one.
func TestAskingForPixelsDoesNotDefeatTheGate(t *testing.T) {
	inner := &recordingInner{}
	clock := time.Now()
	p := shadow.NewProviderWithClock(inner, time.Millisecond, func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	}).WhenWorthIt(func() bool { return false })

	for i := 0; i < 4; i++ {
		p.ObserveTargeted(context.Background(), observation.Request{})
	}
	if inner.ran != 0 {
		t.Errorf("the detector ran %d times behind a policy that declined every slot, "+
			"want 0", inner.ran)
	}
}
