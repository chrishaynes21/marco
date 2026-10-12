package main

// THE REAL OBSERVER, OVER THE REAL RECORDING.
//
// # What this answers
//
// The owner asked for measurement rather than arithmetic:
//
//	Measure actual sustained end-to-end throughput, latency, dropped observations, and CPU
//	cost rather than assuming the estimated ~50.7ms stage total is guaranteed.
//
// Everything but acquisition can be measured here, on 900 frames this process already has: the
// proposer, the normalisation, the fold, the publication, and whether a consumer keeps up. What
// it cannot measure is the 16.7ms capture, which needs a window —
// [[ADR-177-acquisition-is-a-frame-interval-not-a-cost]] measured that separately and it is a
// 60Hz frame interval rather than work.
//
// # And it is the comparison baseline
//
// This runs `fastobserve.Observer` — the same type the live command runs — over the recording
// every offline conclusion was drawn from. So when a live session eventually happens, "did the
// live observer see what the recording saw" is a question with a recorded answer rather than an
// impression.

import (
	"fmt"
	"image"
	"path/filepath"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// reportObserverOverRecording folds the recording through the production observer.
//
// `consume` says whether anything reads the queue. Running it both ways is the backpressure
// measurement: the same looks, once with a reader and once without, and the only thing that may
// differ is the drop count.
func reportObserverOverRecording(dir string, limit int, consume bool) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if len(frames) == 0 {
		return fmt.Errorf("%s carries no frames", dir)
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}
	inputs, _ := readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })

	// THE RECORDING'S OWN RATE. 900 frames over 89.9 seconds is 100ms a look, which is
	// the interval every offline measurement was taken at and the one the live command
	// defaults to.
	const interval = 100 * time.Millisecond
	obs, err := fastobserve.New(interval, 32)
	if err != nil {
		return err
	}

	consumed := 0
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for range obs.Updates() {
			if !consume {
				// A CONSUMER THAT DOES NOT KEEP UP — and it has to be
				// genuinely slower than the observer or the measurement is of
				// nothing. The first version slept 2ms against a 92ms look and
				// dropped zero, which read as "the consumer kept up" when it
				// meant "the consumer was never tested".
				//
				// 300ms is three looks behind per update, which is what a
				// semantic path doing real work looks like.
				time.Sleep(300 * time.Millisecond)
			}
			consumed++
		}
	}()

	opt := structure.DefaultOptions()
	base := time.Unix(1700000000, 0).UTC()
	nextInput := 0
	var proposeMS, foldMS, wholeMS float64
	var perLook []float64
	decoded := 0
	for _, fr := range frames {
		whole := time.Now()
		lk := fastobserve.Look{
			At: base.Add(time.Duration(fr.AtMS) * time.Millisecond),
			Window: fastobserve.WindowRef{
				ID: "hwnd:recorded", Generation: fr.WindowGeneration,
				Bounds: directorapi.Rect{Width: fr.Width, Height: fr.Height},
			},
			Front: true,
		}
		for nextInput < len(inputs) && inputs[nextInput].AtMS <= fr.AtMS {
			in := inputs[nextInput]
			nextInput++
			if observe.NavIntent(in.Intent) == observe.NavPoint {
				lk.Presses = append(lk.Presses, in.Where)
			}
		}
		img, err := readPNG(filepath.Join(dir, filepath.FromSlash(fr.File)))
		if err != nil {
			lk.Missing = true
		} else {
			decoded++
			t0 := time.Now()
			res := structure.Propose(img, opt)
			proposeMS += ms(time.Since(t0))
			rects := make([]image.Rectangle, 0, len(res.Candidates))
			for _, c := range res.Candidates {
				rects = append(rects, c.Bounds)
			}
			lk.Regions = observe.NormaliseRects(img.Bounds(), rects)
		}
		t1 := time.Now()
		obs.Observe(lk)
		foldMS += ms(time.Since(t1))
		d := ms(time.Since(whole))
		wholeMS += d
		perLook = append(perLook, d)
	}
	obs.Close()
	<-drained

	s := obs.Stats()
	sort.Float64s(perLook)
	at := func(p int) float64 {
		if len(perLook) == 0 {
			return 0
		}
		return perLook[p*(len(perLook)-1)/100]
	}
	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("THE PRODUCTION OBSERVER OVER %s  (consumer %s)\n",
		filepath.Base(dir), map[bool]string{true: "keeping up", false: "falling behind"}[consume])
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  %d look(s), %d frame(s) decoded, settle rule %d at %s\n",
		s.Looks, decoded, obs.SettleLooks(), interval)
	fmt.Printf("  proposer     %7.1fms total, %6.2fms a look\n",
		proposeMS, proposeMS/float64(max(1, s.Looks)))
	fmt.Printf("  observer     %7.1fms total, %6.2fms a look   <-- the fold itself\n",
		foldMS, foldMS/float64(max(1, s.Looks)))
	fmt.Printf("  per look     median %6.2fms  p95 %6.2fms  max %6.2fms  (EXCLUDING the\n",
		at(50), at(95), at(100))
	fmt.Printf("               16.7ms acquisition, which needs a window, and INCLUDING a\n")
	fmt.Printf("               %.0fms PNG decode a live observer never pays)\n",
		(wholeMS-proposeMS-foldMS)/float64(max(1, s.Looks)))
	live := at(50) - (wholeMS-proposeMS-foldMS)/float64(max(1, s.Looks)) + 16.7
	fmt.Printf("  so a live look is about %.1fms — %.1f a second\n", live, 1000/live)
	fmt.Printf("  published %d, dropped %d, consumed %d\n", s.Published, s.Dropped, consumed)
	fmt.Printf("  not evidence: %d missing, %d covered, %d window change(s)\n",
		s.Missing, s.Covered, s.Interrupted)
	fmt.Printf("\n  %d screen(s), %d transition(s)\n", len(obs.Screens()), s.Transitions)
	for _, sc := range obs.Screens() {
		fmt.Printf("      %-10s %4d look(s), settled at %d\n", sc.ID, sc.Looks, sc.First)
	}
	e := obs.Clicks()
	byCause := map[observe.TransitionCause]int{}
	for _, st := range e.Steps {
		byCause[st.Cause] += st.Count
	}
	fmt.Printf("  changes by cause: %d click, %d automatic, %d unknown\n",
		byCause[observe.CauseClick], byCause[observe.CauseAutomatic],
		byCause[observe.CauseUnknown])
	return nil
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
