package main

// Replaying a slow source offline, at the delay it actually has.
//
// # Why the detections are recorded rather than computed here
//
// Inference costs 569-746ms a frame ([[Experiment-030-asking-the-learned-detector-for-the-tile]]). Running it inside the replay would
// turn a nine-hundred-frame measurement of the OBSERVER into a ten-minute measurement of the
// model's throughput, and would make the result depend on what else the machine was doing. So
// `vision batch` records every frame's detections once, and this reads them back.
//
// # Why a delay is simulated rather than ignored
//
// Because the delay is the entire difficulty. A detection that arrived instantly would need no
// epoch, no admission rule and no test: it would just be more geometry. What makes this worth
// measuring is that the answer describes a frame six to eight looks in the past, and the
// interface may have moved on. Feeding the detections back onto the look they came from would
// measure an integration nobody can build.
//
// So: a request is made when a screen settles, at most one is outstanding, and the answer is
// delivered `delay` looks later carrying the context the REQUEST was made in. Whether that
// context is still current is the observer's judgement, not this file's.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// detectedFrame mirrors one line of `vision batch` output.
type detectedFrame struct {
	File    string  `json:"file"`
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	MS      float64 `json:"ms"`
	Error   string  `json:"error,omitempty"`
	Element []struct {
		Label string  `json:"label"`
		Score float64 `json:"score"`
		X     int     `json:"x"`
		Y     int     `json:"y"`
		W     int     `json:"w"`
		H     int     `json:"h"`
	} `json:"elements,omitempty"`
}

// slowRequest is one outstanding ask.
type slowRequest struct {
	look       int // the look whose frame was handed to the detector
	deliverAt  int // the look its answer arrives on
	generation uint64
	epoch      uint64
}

// slowFeed turns recorded detections into delayed candidates.
type slowFeed struct {
	// byLook is detections per look, 1-based to match the replay's own numbering.
	byLook map[int][]observe.Candidate
	delay  int

	pending *slowRequest

	// The ledger. Requested and delivered differ by whatever was still outstanding at
	// the end; skipped counts settles that found a request already in flight, which is
	// the bound on outstanding work doing its job rather than an error.
	requested, delivered, skipped, empty int
	// meanMS is the recorded inference cost, carried through to the report so the
	// delay in looks can be checked against the latency it stands for.
	meanMS float64
	frames int
}

// loadSlowFeed reads `vision batch` JSONL and normalises every box to its frame.
//
// Detections are matched to looks BY POSITION, which is why `vision batch` records a line for a
// frame it could not read: a silently shorter file would shift every later frame's detections
// onto the wrong look, and the result would look like a working integration with a mysterious
// error rate.
func loadSlowFeed(path string, delay int) (*slowFeed, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fd := &slowFeed{byLook: map[int][]observe.Candidate{}, delay: delay}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)
	look := 0
	var totalMS float64
	var timed int
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var df detectedFrame
		if err := json.Unmarshal([]byte(line), &df); err != nil {
			return nil, fmt.Errorf("line %d: %w", look+1, err)
		}
		look++
		fd.frames++
		if df.MS > 0 {
			totalMS += df.MS
			timed++
		}
		if df.Error != "" || df.Width == 0 || df.Height == 0 {
			continue
		}
		bounds := image.Rect(0, 0, df.Width, df.Height)
		rects := make([]image.Rectangle, 0, len(df.Element))
		classes := make([]string, 0, len(df.Element))
		scores := make([]float64, 0, len(df.Element))
		for _, e := range df.Element {
			rects = append(rects, image.Rect(e.X, e.Y, e.X+e.W, e.Y+e.H))
			classes = append(classes, e.Label)
			scores = append(scores, e.Score)
		}
		if len(rects) == 0 {
			continue
		}
		// Normalised exactly as the proposer's own rectangles are, by the one
		// statement of pixel-to-normalised this subsystem has.
		fd.byLook[look] = observe.Detected(observe.NormaliseRects(bounds, rects),
			classes, scores, look, 0, 0)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if timed > 0 {
		fd.meanMS = totalMS / float64(timed)
	}
	return fd, nil
}

// request records an ask made on a settled look, unless one is already outstanding.
//
// BOUNDED OUTSTANDING WORK, and the bound is one. A source that queued a request per settle
// would, on a menu that settles repeatedly, build a backlog of answers about screens nobody is
// on any more — every one of which would then be refused, at the cost of having computed it.
// Refusing to ask is cheaper than refusing the answer.
func (f *slowFeed) request(look int, generation, epoch uint64) {
	if f == nil {
		return
	}
	if f.pending != nil {
		f.skipped++
		return
	}
	f.requested++
	f.pending = &slowRequest{
		look: look, deliverAt: look + f.delay,
		generation: generation, epoch: epoch,
	}
}

// due returns the detections arriving on this look, stamped with the context their request was
// made in — not the context they arrive in, which is the whole point.
func (f *slowFeed) due(look int) []observe.Candidate {
	if f == nil || f.pending == nil || look < f.pending.deliverAt {
		return nil
	}
	req := f.pending
	f.pending = nil
	f.delivered++

	found := f.byLook[req.look]
	if len(found) == 0 {
		// A LOOK THE DETECTOR FOUND NOTHING ON is an answer, and an empty one is
		// still a delivery: the request completed and the next settle may ask again.
		f.empty++
		return nil
	}
	out := make([]observe.Candidate, 0, len(found))
	for _, c := range found {
		c.Generation, c.Epoch = req.generation, req.epoch
		out = append(out, c)
	}
	return out
}

// report is the feed's own line in a measurement, so a run that admitted nothing can be told
// from one that was never asked.
func (f *slowFeed) report() string {
	if f == nil {
		return "slow source: none (pixel proposals only)"
	}
	return fmt.Sprintf("slow source: %d frame(s) of recorded detections, mean inference "+
		"%.0fms, delivered %d looks later\n  requested %d  delivered %d  "+
		"empty %d  skipped-while-busy %d",
		f.frames, f.meanMS, f.delay, f.requested, f.delivered, f.empty, f.skipped)
}

// admissionLedger is what the OBSERVER did with what the feed delivered, which is a different
// question from what the feed sent.
//
// Both are printed because only together do they say whether the integration works. A feed that
// delivered twenty answers and an observer that admitted none is a cadence problem; deliveries
// with no stale rejections at all, on a recording that visits nine screens, would mean the
// epoch was not moving and the rule was not being applied.
func admissionLedger(st fastobserve.Stats) string {
	var b strings.Builder
	fmt.Fprintf(&b, "what the observer did with them:\n")
	fmt.Fprintf(&b, "  offered %d  admitted %d  held-and-offered on %d look(s)\n",
		st.SlowOffered, st.SlowAdmitted, st.SlowOfferedLooks)
	fmt.Fprintf(&b, "  refused: stale-epoch %d  stale-window %d  look-not-evidence %d\n",
		st.SlowStaleEpoch, st.SlowStaleWindow, st.SlowUnusable)
	fmt.Fprintf(&b, "  expired while held: %d\n", st.SlowExpired)
	fmt.Fprintf(&b, "  fold latency: median %.3fms  p95 %.3fms over %d look(s)",
		st.At(50), st.At(95), len(st.LatencyMS))
	return b.String()
}
