package screen

// WHAT A CAPTURE'S ACQUISITION ACTUALLY COSTS, STAGE BY STAGE.
//
// # Why this is here and not in a command
//
// The GDI handles and the proc table live in this package, and a benchmark that reached for them
// from outside would need them exported — which would make the one path that can read the screen
// reachable from anywhere. So the measurement lives beside the thing it measures, uses the SAME
// procs `capture` uses, and what crosses out is a table of numbers.
//
// # The privacy boundary, which is the whole reason this is allowed to exist
//
// Nothing here retains, returns, writes, hashes or reports a pixel. A capture's buffer is reused
// and overwritten; the only thing read OUT of it is a single boolean — whether any byte in it was
// non-zero — because a blit that silently failed returns black, and a faster capture that
// returned nothing would otherwise look like an improvement.
//
// One bit per frame, and the bit is "did this contain anything at all". No histogram, no sample,
// no hash, no region. [[ADR-177-what-an-acquisition-costs]] states the same boundary from the
// outside.
//
// # What makes the comparison fair
//
// Every configuration blits from the SAME source rectangle. Only the destination size changes,
// and both are reported, because a reduced capture that also sampled less of the window would
// look faster for a reason that has nothing to do with the thing under test.
//
// It injects nothing, clicks nothing, and activates no window. See acquirebench_other.go for the
// platforms where it does nothing at all.

import (
	"fmt"
	"sort"
	"time"
)

// AcquireConfig is one way of taking a capture.
type AcquireConfig struct {
	Name string
	// Reuse hoists the device context and the bitmap across frames instead of creating
	// and destroying them per capture, which is what `capture` does today.
	Reuse bool
	// Divide reduces the destination during the blit, with StretchBlt. 1 is a full-size
	// BitBlt — the same call production makes.
	//
	// Reducing DURING the blit is the only reduction that makes acquisition itself
	// cheaper, because it shrinks what GetDIBits has to move. A reduction afterwards
	// shrinks only the proposer's work, which is already measured and is not the
	// bottleneck. See [[ADR-176-the-time-is-in-the-acquisition-and-a-smaller-observation-keeps-the-clicks]].
	Divide int
}

// AcquireStage is one stage's latencies, in milliseconds.
type AcquireStage struct {
	Name    string
	Samples []float64
}

func (s *AcquireStage) add(d time.Duration) {
	s.Samples = append(s.Samples, float64(d.Microseconds())/1000)
}

// At returns the pth percentile, p in [0,100]. Zero for a stage nobody measured.
func (s AcquireStage) At(p int) float64 {
	if len(s.Samples) == 0 {
		return 0
	}
	v := make([]float64, len(s.Samples))
	copy(v, s.Samples)
	sort.Float64s(v)
	i := p * (len(v) - 1) / 100
	if i < 0 {
		i = 0
	}
	return v[i]
}

// AcquireResult is what one configuration produced. Timings and counts only.
type AcquireResult struct {
	Config AcquireConfig
	// Stages are the per-call latencies: the device context, the setup, the blit, the
	// transfer, the cleanup, and the whole acquisition.
	Stages []AcquireStage
	// Frames attempted and Failures how many returned no pixels at all.
	Frames, Failures int
	// SrcW and SrcH are the source rectangle every configuration shares; DstW and DstH
	// the destination this one produced. Reported so a reader can check that a faster
	// configuration did not simply sample less of the window.
	SrcW, SrcH, DstW, DstH int
	// NonBlank is captures whose buffer held at least one non-zero byte.
	//
	// The validity check, and it is one bit per frame. A blit that failed silently
	// returns black, and a configuration that was faster because it returned nothing
	// would otherwise read as an improvement.
	NonBlank int
	// Bytes is how much GetDIBits moved per capture, which is the number the reduction
	// is supposed to change.
	Bytes int
	// GDIBefore and GDIAfter are this process's GDI object counts around the run, for
	// the leak check: a configuration that reuses handles must not accumulate them, and
	// one that creates them per frame must give them all back.
	GDIBefore, GDIAfter uint32
}

// Throughput is captures per second at the median total.
func (r AcquireResult) Throughput() float64 {
	for _, s := range r.Stages {
		if s.Name != "total acquisition" {
			continue
		}
		if m := s.At(50); m > 0 {
			return 1000 / m
		}
	}
	return 0
}

// Line renders one stage for a report.
func (s AcquireStage) Line() string {
	return fmt.Sprintf("    %-22s n=%-5d median %7.3fms  p95 %7.3fms  max %7.3fms",
		s.Name, len(s.Samples), s.At(50), s.At(95), s.At(100))
}

// DefaultAcquireConfigs are the four configurations the measurement compares.
//
// Exactly the four the brief names, and in this order because each one differs from the previous
// by one thing: today's path, then handle reuse alone, then reduction alone, then both. A table
// whose rows differed by two things at once could not say which one mattered.
func DefaultAcquireConfigs() []AcquireConfig {
	return []AcquireConfig{
		{Name: "per-frame handles, full size", Reuse: false, Divide: 1},
		{Name: "reused handles, full size", Reuse: true, Divide: 1},
		{Name: "per-frame handles, half size", Reuse: false, Divide: 2},
		{Name: "reused handles, half size", Reuse: true, Divide: 2},
	}
}
