package main

// Replaying the PIXELS, not the detector's conclusions about them.
//
// # Why this is the level that matters
//
// The L1 replay in main.go reads what the detector produced and exercises everything after it.
// It cannot test a change to the detector, to admission, or to how shading is measured, because
// those stages PRODUCED L1. Any claim that a new way of seeing is better has to be made against
// the original frames.
//
// `director capture-game-session` already writes exactly that, and has since before this: a
// frame per PNG at the window's own resolution, a measured timestamp per frame, the controller's
// observed intents on the same clock, and the window generation each frame belongs to. The
// format was there and nothing read it back through a perception path.
//
// # What it scores against
//
// A recording may carry an `annotations.json` of intervals saying which entry was selected. The
// file in hand says of itself:
//
//	MODEL-ASSISTED VISUAL ANNOTATION ... they must NOT be presented as independently
//	validated ground truth.
//
// That is honoured here: every number this prints is labelled as scored against model-assisted
// labels, and the annotation's own `y` is used only to ask "is the row the belief names the row
// the label names". Nothing in production may read that file, and nothing here feeds it to the
// perception under test — it is applied strictly after the fact, to a belief that was formed
// without it.

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
)

// frameRecord is one line of frames.jsonl, as capture-game-session writes it.
//
// The tags are the contract with that writer, the same way slot's are with the shadow trace.
// A rename there turns a replay into a report about a recording with no frames in it.
type frameRecord struct {
	Index            int    `json:"index"`
	File             string `json:"file"`
	AtMS             int64  `json:"at_ms"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	WindowGeneration uint64 `json:"window_generation"`
}

// inputRecord is one line of input.jsonl: an intent and when, with no key identity.
type inputRecord struct {
	Intent string `json:"intent"`
	AtMS   int64  `json:"at_ms"`
}

// annotationFile is a recording's labels, when it has any.
type annotationFile struct {
	Session    string `json:"session"`
	Provenance string `json:"provenance"`
	Intervals  []struct {
		From  int     `json:"from"`
		To    int     `json:"to"`
		State string  `json:"state"`
		Entry string  `json:"entry"`
		Y     float64 `json:"y"`
	} `json:"intervals"`
}

// labelAt returns the state and the selected row's normalised centre for a frame index.
func (a *annotationFile) labelAt(index int) (state string, y float64, ok bool) {
	if a == nil {
		return "", 0, false
	}
	for _, iv := range a.Intervals {
		if index >= iv.From && index <= iv.To {
			return iv.State, iv.Y, true
		}
	}
	return "", 0, false
}

// frameResult is what one recording yielded when reprocessed.
type frameResult struct {
	Name   string
	Frames int
	// Proposed is total candidates over all frames, and PerFrame the mean. The precision
	// proxy: every false rectangle is a possible list member on every frame.
	Proposed int
	// Grouped is frames whose proposals produced a stack the tracker would consider.
	Grouped int
	// CoveredSelected is frames where some proposal covered the labelled selected row,
	// and LabelledFrames how many frames carried a label to check against.
	CoveredSelected, LabelledFrames int
	// WithSelection is frames where the belief named an entry; CorrectSelection those
	// where the named row sat at the labelled row's height.
	WithSelection, CorrectSelection int
	// ProposeMS and DecodeMS are measured cost, summed.
	ProposeMS, DecodeMS int64
	Belief              observe.InterfaceBelief
	Candidates          []observe.InterfaceCandidate
	Identified          int
	Inputs              int
}

// replayFrames reprocesses a recording's pixels through the structure proposer and the
// production interface watch.
func replayFrames(dir string, stride, limit int, verbose bool) (frameResult, error) {
	r := frameResult{Name: filepath.Base(dir), Identified: -1}

	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return r, err
	}
	if len(frames) == 0 {
		return r, fmt.Errorf("%s carries no frames.jsonl entries. A recording made before "+
			"frames were captured has metadata and no pixels, and cannot be replayed at "+
			"this level", dir)
	}
	inputs, _ := readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	r.Inputs = len(inputs)

	var ann *annotationFile
	if b, err := os.ReadFile(filepath.Join(dir, "annotations.json")); err == nil {
		var a annotationFile
		if err := json.Unmarshal(b, &a); err == nil {
			ann = &a
		}
	}

	watch := observe.NewInterfaceWatch()
	// A deterministic clock seeded from the recording's own measured offsets, so a replay
	// run twice gives byte-identical belief timestamps.
	base := time.Unix(1700000000, 0).UTC()
	opt := structure.DefaultOptions()

	for i, fr := range frames {
		if stride > 1 && i%stride != 0 {
			continue
		}
		if limit > 0 && r.Frames >= limit {
			break
		}
		t0 := time.Now()
		img, err := readPNG(filepath.Join(dir, filepath.FromSlash(fr.File)))
		if err != nil {
			// A missing frame is a gap in the record, not a reason to stop: a sanitised
			// recording has frames removed on purpose.
			continue
		}
		r.DecodeMS += time.Since(t0).Milliseconds()
		r.Frames++

		t1 := time.Now()
		res := structure.Propose(img, opt)
		r.ProposeMS += time.Since(t1).Milliseconds()
		r.Proposed += len(res.Candidates)

		// THE PIXELS ARE MEASURED HERE, which is the whole point of replaying at this
		// level: shading comes from the frame through the production measurement, not
		// from whatever a previous detector happened to record.
		look := observe.InterfaceLook{
			Sequence: fr.Index,
			At:       base.Add(time.Duration(fr.AtMS) * time.Millisecond),
			Observed: true,
			Shapes:   shapesFrom(img, res.Candidates),
		}
		if sizes := observe.GroupedForLook(look); len(sizes) > 0 && sizes[0] >= minUsableStack {
			r.Grouped++
		}
		watch.Observe(look)
		b := watch.Belief()

		state, y, labelled := ann.labelAt(fr.Index)
		if labelled && state == "selected" {
			r.LabelledFrames++
			if coversHeight(res.Candidates, y, img.Bounds()) {
				r.CoveredSelected++
			}
		}
		if b.Selected >= 0 {
			r.WithSelection++
			if labelled && state == "selected" && nearHeight(b.Where, y) {
				r.CorrectSelection++
			}
		}
		if verbose {
			fmt.Printf("  frame %-5d %-12s sel %-3d  cands %-3d  label %-10s %s\n",
				fr.Index, b.Status, b.Selected, len(res.Candidates), state,
				trim(b.Because, 70))
		}
	}

	r.Belief = watch.Belief()
	r.Identified = r.Belief.After
	r.Candidates = watch.Candidates()
	reportFrames(r, ann)
	return r, nil
}

// shapesFrom turns proposals into the evidence the interface watch consumes.
//
// Shapes and not Regions, deliberately. A ShadowRegion carries a role, is counted into the role
// distribution and feeds semantic evidence that can reach cross-session identity; a shape is a
// rectangle and how it is painted, and may never become a term, a control or anything durable.
// The structure proposer names nothing, so shape is the honest channel and the belief will say
// how much of what it is about has no name.
func shapesFrom(img image.Image, cands []structure.Candidate) []observe.ShadowShape {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	if w <= 0 || h <= 0 {
		return nil
	}
	out := make([]observe.ShadowShape, 0, len(cands))
	for _, c := range cands {
		sh := selection.Measure(img, c.Bounds)
		out = append(out, observe.ShadowShape{
			Region: observe.Region{
				X:      float64(c.Bounds.Min.X-b.Min.X) / w,
				Y:      float64(c.Bounds.Min.Y-b.Min.Y) / h,
				Width:  float64(c.Bounds.Dx()) / w,
				Height: float64(c.Bounds.Dy()) / h,
			},
			Confidence: c.Contrast,
			Shade:      []float64{sh[0], sh[1]},
		})
	}
	return out
}

// coversHeight reports whether any proposal spans the labelled row's height.
//
// Height only. The label gives the selected row's vertical centre and nothing else, so a
// horizontal comparison would be inventing a width the label does not claim.
func coversHeight(cands []structure.Candidate, y float64, b image.Rectangle) bool {
	if b.Dy() <= 0 {
		return false
	}
	py := float64(b.Min.Y) + y*float64(b.Dy())
	for _, c := range cands {
		if float64(c.Bounds.Min.Y) <= py && py <= float64(c.Bounds.Max.Y) {
			return true
		}
	}
	return false
}

// nearHeight reports whether a believed region sits at the labelled row's height.
//
// The belief's region is normalised window-relative, which is the same space the label's y is
// in, so this is a direct comparison. The tolerance is the believed row's own height: naming a
// row means naming it to within its own extent, and anything tighter would be scoring the
// label's precision rather than the reader's.
func nearHeight(where observe.Region, y float64) bool {
	if where.Height <= 0 {
		return false
	}
	return where.Y-where.Height <= y && y <= where.Y+where.Height*2
}

func reportFrames(r frameResult, ann *annotationFile) {
	fmt.Printf("\n%s\n", strings.Repeat("=", 78))
	fmt.Printf("REPLAY %s   (L0: the original pixels)\n", r.Name)
	fmt.Printf("%s\n", strings.Repeat("=", 78))
	fmt.Printf("  frames reprocessed  %d   inputs in the recording %d\n", r.Frames, r.Inputs)
	if r.Frames == 0 {
		return
	}
	fmt.Printf("  cost                %dms propose, %dms decode, %.0fms/frame propose\n",
		r.ProposeMS, r.DecodeMS, float64(r.ProposeMS)/float64(r.Frames))
	fmt.Printf("  proposals           %d total, %.1f per frame  <-- the precision proxy\n",
		r.Proposed, float64(r.Proposed)/float64(r.Frames))
	fmt.Printf("  grouping            %d of %d frames produced a stack of >=%d (%.0f%%)\n",
		r.Grouped, r.Frames, minUsableStack, pct(r.Grouped, r.Frames))

	if r.LabelledFrames > 0 {
		fmt.Printf("\n  SCORED AGAINST MODEL-ASSISTED LABELS, not independently validated\n")
		if ann != nil && ann.Provenance != "" {
			fmt.Printf("  %s\n", trim(ann.Provenance, 74))
		}
		fmt.Printf("  discovery           %d of %d labelled frames had a proposal covering\n"+
			"                      the selected row (%.0f%%)\n",
			r.CoveredSelected, r.LabelledFrames, pct(r.CoveredSelected, r.LabelledFrames))
		fmt.Printf("  selection read      %d frames named an entry; %d of those matched the\n"+
			"                      labelled row's height\n", r.WithSelection, r.CorrectSelection)
	} else {
		fmt.Printf("  no annotations, so discovery and selection are unscored here\n")
	}

	fmt.Printf("\n  belief              %s, selected %d, reading %s\n",
		r.Belief.Status, r.Belief.Selected, r.Belief.Reading)
	fmt.Printf("  identified after    %d\n", r.Identified)
	fmt.Printf("  because             %s\n", trim(r.Belief.Because, 150))
	if len(r.Candidates) == 0 {
		fmt.Printf("  candidates          none were tracked\n")
		return
	}
	fmt.Printf("  candidates          %d\n", len(r.Candidates))
	fmt.Printf("    %-5s %-6s %-6s %-5s %-6s %-5s %-6s %-5s %-7s %s\n",
		"rows", "frames", "judged", "excl", "shared", "quiet", "unread", "moves",
		"holders", "verdict")
	for _, c := range r.Candidates {
		verdict := "refused"
		if c.Qualifies {
			verdict = "IDENTIFIED"
		}
		fmt.Printf("    %-5d %-6d %-6d %-5d %-6d %-5d %-6d %-5d %-7d %s\n",
			c.Members, c.Frames, c.Judged, c.Exclusive, c.Shared, c.Quiet,
			c.Unreadable, c.Transfers, c.Holders, verdict)
	}
	shortfall := map[string]int{}
	for _, c := range r.Candidates {
		for _, u := range c.Unmet {
			shortfall[u.Requirement]++
		}
	}
	if len(shortfall) > 0 {
		keys := make([]string, 0, len(shortfall))
		for k := range shortfall {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s %d", k, shortfall[k]))
		}
		fmt.Printf("  shortfalls          %s\n", strings.Join(parts, "   "))
	}
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// readJSONL decodes a whole JSON-lines file.
func readJSONL[T any](path string) ([]T, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []T
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var v T
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, v)
	}
	return out, nil
}
