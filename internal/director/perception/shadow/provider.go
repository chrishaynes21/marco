package shadow

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Running an expensive experimental detector beside live perception, without slowing it down.
//
// # Why cadence lives here and not in the scheduler
//
// The authoritative observation loop samples at its own rate, chosen for what authoritative
// perception costs. ScreenParser costs roughly 0.9s per frame and 1.25 GB resident; forcing the
// whole loop to accommodate that would make the experiment change the thing it is measuring.
//
// So the gate is inside the provider. The collector calls it every cycle as normal; most of
// those calls return immediately having done nothing. From the outside it is an ordinary
// provider that is usually empty, which is a state the pipeline already understands.
//
// # Skip, never queue
//
// If an inference is still running when the next slot arrives, the slot is SKIPPED. Queueing
// would build a backlog of pictures of a game that has moved on — and a detection compared
// against belief from a different moment is not evidence about a disagreement, it is evidence
// about latency. Skipped slots are counted, because an experiment that quietly ran a third as
// often as it claimed would report a third of the information gain and call it a finding.

// Provider wraps an expensive provider so it runs on its own cadence and never blocks.
//
// It implements ShadowOnly, so the collector routes its evidence to Cycle.Shadow and fusion
// never sees it. That is the only thing standing between an experiment and belief, and it is
// declared by the type rather than configured.
type Provider struct {
	inner   observation.TargetedProvider
	cadence time.Duration
	now     func() time.Time
	// worth is asked before an inference starts: is more evidence worth buying right now?
	//
	// Nil means yes, which is what keeps every existing caller and every test unchanged.
	// See WhenWorthIt.
	worth func() bool

	mu       sync.Mutex
	inFlight bool
	lastRun  time.Time

	stats Stats

	latMu     sync.Mutex
	latencies []time.Duration
}

// Stats is what a shadow session can report about itself.
type Stats struct {
	Inferences  int64 `json:"inferences"`
	SkippedBusy int64 `json:"skipped_busy"`
	SkippedRate int64 `json:"skipped_cadence"`
	// SkippedUnneeded is inferences declined because the primary reading already answered
	// the question. Counted rather than silent: an experiment that stopped running must
	// never look like one that ran and found nothing.
	SkippedUnneeded int64 `json:"skipped_unneeded"`
	Failures        int64 `json:"failures"`

	// FirstLatency is the cold pass: model load plus session creation plus one inference.
	// Held separately because averaging it into the median would misreport steady cost by
	// a wide margin, and the cold number is the one that decides whether a session can
	// start without the user noticing.
	FirstLatency time.Duration `json:"first_latency"`
}

// percentile of the WARM passes.
//
// The samples live on the Provider rather than on Stats so that Stats stays a plain,
// copyable value. A metrics struct carrying a mutex cannot be returned from a snapshot
// without copying the lock, which `go vet` correctly refuses.
func (p *Provider) percentile(pct int) time.Duration {
	p.latMu.Lock()
	defer p.latMu.Unlock()
	if len(p.latencies) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), p.latencies...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	i := (len(sorted) * pct) / 100
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i]
}

func (p *Provider) recordLatency(d time.Duration) {
	p.latMu.Lock()
	defer p.latMu.Unlock()
	p.latencies = append(p.latencies, d)
}

// Snapshot copies the counters for a diagnostic.
func (p *Provider) Snapshot() Stats {
	return Stats{
		Inferences:      atomic.LoadInt64(&p.stats.Inferences),
		SkippedBusy:     atomic.LoadInt64(&p.stats.SkippedBusy),
		SkippedRate:     atomic.LoadInt64(&p.stats.SkippedRate),
		SkippedUnneeded: atomic.LoadInt64(&p.stats.SkippedUnneeded),
		Failures:        atomic.LoadInt64(&p.stats.Failures),
		FirstLatency:    p.stats.FirstLatency,
	}
}

// Latencies reports the warm median and p95.
func (p *Provider) Latencies() (median, p95 time.Duration) {
	return p.percentile(50), p.percentile(95)
}

// DefaultCadence is how often a shadow inference may start.
//
// Two seconds, from measurement rather than taste: ~0.9s median CPU inference means a 1s
// cadence spends most of its time inside the detector, and 200ms or 500ms would skip far more
// slots than it ran. Two seconds leaves the machine to the game, which is the whole point of
// an experiment a player is not supposed to notice.
const DefaultCadence = 2 * time.Second

// NewProvider wraps an expensive provider as a shadow one.
func NewProvider(inner observation.TargetedProvider, cadence time.Duration) *Provider {
	return NewProviderWithClock(inner, cadence, time.Now)
}

// NewProviderWithClock is NewProvider with the clock supplied.
//
// Exported because a cadence gate is defined entirely in terms of time, and the only honest
// way to test one is to control it. The alternative is a test that sleeps and hopes — which on
// Windows, whose clock advances in ~15ms steps, does not even reliably reproduce the case it
// claims to cover.
func NewProviderWithClock(inner observation.TargetedProvider, cadence time.Duration,
	now func() time.Time) *Provider {

	if cadence <= 0 {
		cadence = DefaultCadence
	}
	if now == nil {
		now = time.Now
	}
	return &Provider{inner: inner, cadence: cadence, now: now}
}

// ShadowOnly marks this provider's evidence as never believable.
func (p *Provider) ShadowOnly() {}

var (
	_ observation.ShadowProvider   = (*Provider)(nil)
	_ observation.TargetedProvider = (*Provider)(nil)
)

func (p *Provider) Name() string { return "shadow:" + p.inner.Name() }

// Inner is the provider running on this cadence.
//
// Exposed so a diagnostic can ask it what its last pass actually did — how many regions it
// read, how many it skipped for being unsayable, how many it stopped reading at. A cadence
// gate has nothing to say about any of that, and a cap whose effects are invisible reads as
// "there was nothing to find".
func (p *Provider) Inner() observation.TargetedProvider { return p.inner }

func (p *Provider) Sources() []observation.Source { return p.inner.Sources() }

func (p *Provider) Observe(ctx context.Context,
	req observation.Request) ([]observation.Observation, error) {

	return p.ObserveTargeted(ctx, req).Observations, nil
}

// ObserveTargeted runs the inner provider if a slot is due and none is in flight.
//
// The inner provider is a full TargetedProvider and its outcome is returned UNCHANGED apart
// from timing. Being experimental is not a reason to weaken provenance: an experiment that
// cannot say which window generation it looked at cannot be compared with belief at all, which
// would make the whole session worthless.
func (p *Provider) ObserveTargeted(ctx context.Context,
	req observation.Request) observation.ProviderOutcome {

	// IS MORE EVIDENCE WORTH BUYING? Asked BEFORE the cadence slot is claimed, so a
	// declined inference does not consume the slot a later, useful one would have used.
	//
	// 37C measured what this saves: over six coherent desktop moments, ScreenParser added no
	// actionable semantic item to a reading that was already sufficient, and took 645–1379ms
	// per frame to add it. Running it against a healthy accessibility reading is a second of
	// work for a result the World already contains.
	//
	// The policy is observe.EscalationOf and it names no sensor — it answers "is more
	// evidence worth buying", and this provider decides that it is the one holding the
	// expensive sensor. Nothing here re-derives sufficiency; deciding that is 37D's job and
	// there is exactly one place it happens.
	//
	// Deleting this must fail TestASufficientReadingDoesNotBuyAnInference.
	if p.worth != nil && !p.worth() {
		atomic.AddInt64(&p.stats.SkippedUnneeded, 1)
		return p.skipped(req)
	}
	if !p.claim() {
		return p.skipped(req)
	}
	defer p.release()

	started := p.now()
	out := p.inner.ObserveTargeted(ctx, req)
	elapsed := p.now().Sub(started)

	n := atomic.AddInt64(&p.stats.Inferences, 1)
	if n == 1 {
		p.stats.FirstLatency = elapsed
	} else {
		p.recordLatency(elapsed)
	}
	if out.State == observation.StateFailed || out.State == observation.StateUnavailable {
		atomic.AddInt64(&p.stats.Failures, 1)
	}
	return out
}

// claim reserves the slot, reporting whether this call may run.
func (p *Provider) claim() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inFlight {
		atomic.AddInt64(&p.stats.SkippedBusy, 1)
		return false
	}
	if !p.lastRun.IsZero() && p.now().Sub(p.lastRun) < p.cadence {
		atomic.AddInt64(&p.stats.SkippedRate, 1)
		return false
	}
	p.inFlight = true
	p.lastRun = p.now()
	return true
}

func (p *Provider) release() {
	p.mu.Lock()
	p.inFlight = false
	p.mu.Unlock()
}

// skipped is the outcome for a cycle this provider deliberately sat out.
//
// StateEmpty with a reason, not a failure: it did not fail, it was not asked. Reporting a skip
// as a failure would put the experiment's own cadence into a diagnostic as though something
// were wrong, and would make the real failures impossible to find.
func (p *Provider) skipped(req observation.Request) observation.ProviderOutcome {
	out := observation.ProviderOutcome{
		Source:    directorapi.SourceVision,
		Scope:     directorapi.ScopeTarget,
		State:     observation.StateEmpty,
		Reason:    "shadow slot skipped: cadence " + p.cadence.String(),
		StartedAt: p.now(),
	}
	out.FinishedAt = out.StartedAt
	if req.Target != nil {
		out.ExpectedTarget = *req.Target
	}
	return out
}

// WhenWorthIt gates inference on whether more evidence is worth buying.
//
// The hook is a function rather than a value because the answer changes with every reading,
// and a provider holding a stale boolean would keep spending on a screen that recovered — or
// stop spending on one that broke.
//
// Nil, or never set, means always worth it. That is the pre-37E behaviour and it is the
// default on purpose: a gate that defaulted to closed would silently end the experiment.
func (p *Provider) WhenWorthIt(worth func() bool) *Provider {
	p.worth = worth
	return p
}
