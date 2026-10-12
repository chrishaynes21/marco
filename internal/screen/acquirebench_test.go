package screen_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// NOTHING IN THIS FILE CALLS BenchmarkAcquire.
//
// It reads the screen, and the authorisation it has is for one bounded run against one nominated
// window, launched by the owner. A unit test that called it would photograph whatever happened
// to be on the desktop, on every `go test ./...`, for the rest of the repository's life — which
// is not a thing anybody agreed to and is exactly the kind of quiet widening this subsystem's
// whole discipline exists to prevent.
//
// So what is tested here is everything around it: the arithmetic, the configurations, and the
// privacy boundary as a property of the source.

// A stage is reported as percentiles, and the order samples arrived in must not matter.
//
// A cadence has to survive its p95. An average hides the frame that arrived late, and a frame
// that arrives late is the one that makes a press land in the wrong slot.
func TestAnAcquireStageIsReportedAsPercentiles(t *testing.T) {
	var s screen.AcquireStage
	// Nine fast samples and one slow one, slow one FIRST so an unsorted read finds the
	// wrong value at both ends.
	cost := []time.Duration{90, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	for _, d := range cost {
		screen.AddAcquireSampleForTest(&s, d*time.Millisecond)
	}
	if m := s.At(50); m < 1.9 || m > 2.1 {
		t.Errorf("the median is %.2f, want 2. A mean would be 10.8 and would describe "+
			"no capture that happened", m)
	}
	if w := s.At(100); w < 89 || w > 91 {
		t.Errorf("the worst sample reads %.2f, want 90: taking a percentile without "+
			"sorting reports whatever arrived last", w)
	}
	// An unmeasured stage is zero and not a panic. A configuration whose every capture
	// failed measures nothing, and must not take the report down with it.
	var none screen.AcquireStage
	if none.At(50) != 0 || none.At(100) != 0 {
		t.Error("an unmeasured stage did not report zero")
	}
	if none.Line() == "" {
		t.Error("an unmeasured stage printed nothing at all")
	}
}

// Throughput is captures per second at the median total, and zero when there is no total.
func TestThroughputIsTakenFromTheWholeAcquisition(t *testing.T) {
	var total screen.AcquireStage
	total.Name = "total acquisition"
	for range 5 {
		screen.AddAcquireSampleForTest(&total, 50*time.Millisecond)
	}
	r := screen.AcquireResult{Stages: []screen.AcquireStage{total}}
	if got := r.Throughput(); got < 19.9 || got > 20.1 {
		t.Errorf("a 50ms median is %.2f captures a second, want 20", got)
	}
	// A result with stages but no TOTAL reports nothing rather than guessing from a
	// part: the sum of the stages is not the whole, because the loop does work between
	// them.
	var blit screen.AcquireStage
	blit.Name = "blit"
	screen.AddAcquireSampleForTest(&blit, 10*time.Millisecond)
	if got := (screen.AcquireResult{Stages: []screen.AcquireStage{blit}}).Throughput(); got != 0 {
		t.Errorf("a result with no total reports %.2f captures a second", got)
	}
	if got := (screen.AcquireResult{}).Throughput(); got != 0 {
		t.Errorf("an empty result reports %.2f", got)
	}
}

// THE FOUR CONFIGURATIONS DIFFER BY ONE THING EACH, which is what makes the table readable.
//
//	Compare: 1. Existing per-frame resource allocation at full resolution. 2. Reused
//	compatible DC/bitmap resources at full resolution. 3. Reduced-resolution acquisition
//	using StretchBlt, at least at half resolution. 4. Reused resources with reduced-resolution
//	acquisition.
//
// A row that differed from its predecessor by two things could not say which one mattered. And
// the first row has to be TODAY'S path, or the table has no baseline.
func TestTheFourConfigurationsDifferByOneThingEach(t *testing.T) {
	cfgs := screen.DefaultAcquireConfigs()
	if len(cfgs) != 4 {
		t.Fatalf("%d configuration(s), want 4", len(cfgs))
	}
	// Row one is what production does now: handles per frame, full size.
	if cfgs[0].Reuse || cfgs[0].Divide != 1 {
		t.Errorf("the first configuration is %+v; it must be today's path, or the "+
			"table has no baseline", cfgs[0])
	}
	// The whole 2x2 is covered exactly once.
	seen := map[[2]int]bool{}
	for _, c := range cfgs {
		r := 0
		if c.Reuse {
			r = 1
		}
		k := [2]int{r, c.Divide}
		if seen[k] {
			t.Errorf("configuration %+v repeats a combination already measured", c)
		}
		seen[k] = true
		if c.Divide < 1 {
			t.Errorf("configuration %+v divides by %d, which is not a destination",
				c.Name, c.Divide)
		}
		if c.Name == "" {
			t.Errorf("a configuration has no name, so its row cannot be read: %+v", c)
		}
	}
	// At least one reduced configuration, at least half, as required.
	reduced := false
	for _, c := range cfgs {
		if c.Divide >= 2 {
			reduced = true
		}
	}
	if !reduced {
		t.Error("no configuration reduces during the blit, which is the only reduction " +
			"that makes acquisition itself cheaper")
	}
}

// THE PRIVACY BOUNDARY, AS A PROPERTY OF THE SOURCE.
//
// The authorisation for this benchmark is explicit about what it may keep:
//
//	Timing aggregates only; no captured pixels, screenshots, window content, or input events
//	written to disk.
//
// A promise kept by discipline lasts until somebody adds a convenient debug line. So this reads
// the file: the benchmark may not write, encode, hash, or print a buffer, and it must wipe the
// one it used. The only thing it may read out of the pixels is the one-bit emptiness check, and
// that function is named so this test can allow exactly it and nothing else.
func TestTheAcquireBenchmarkKeepsNoPixels(t *testing.T) {
	src, err := os.ReadFile("acquirebench_windows.go")
	if err != nil {
		t.Fatalf("reading the benchmark: %v", err)
	}
	body := string(src)
	for _, forbidden := range []string{
		"os.WriteFile", "os.Create", "png.Encode", "jpeg.Encode",
		"sha256", "md5", "fnv", "base64",
		"fmt.Print", "fmt.Fprint", "log.",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the benchmark names %q. It may keep timings and counts; a "+
				"captured window's pixels leave this function as one bit",
				forbidden)
		}
	}
	// AND IT WIPES THE BUFFER. A measurement has no business leaving a window's pixels
	// in a heap block somebody else will be handed.
	if !strings.Contains(body, "buf[i] = 0") {
		t.Error("the benchmark does not wipe its capture buffer before dropping it")
	}
	// The emptiness check is the ONE read, and it is one bit. If a second reader of the
	// buffer appears, this test should be the thing that asks why.
	if n := strings.Count(body, "anyNonZero(buf)"); n != 1 {
		t.Errorf("the capture buffer is read %d time(s) by anyNonZero; exactly one "+
			"one-bit read is what was authorised", n)
	}
	// Nothing in the result type carries pixels, which is the same claim from the other
	// side: a reader of AcquireResult cannot be handed window content because there is
	// nowhere in it to put any.
	shared, err := os.ReadFile("acquirebench.go")
	if err != nil {
		t.Fatalf("reading the result type: %v", err)
	}
	for _, forbidden := range []string{"[]byte", "image.", "Pix"} {
		if strings.Contains(string(shared), forbidden) {
			t.Errorf("the result type names %q, so it could carry pixels out", forbidden)
		}
	}
}
