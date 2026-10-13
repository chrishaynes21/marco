package main

// WHY ONE DEMONSTRATED CONTROL CAN NEVER BE FOUND AGAIN.
//
// # The question, as the owner put it
//
// [[ADR-181-a-demonstration-marco-can-keep]] measured the learned Rocket League workflow at one
// step of twenty-one, and named the reason: two controls, `target_2` and `target_6`, that
// reacquisition refuses on every single look of their own screen. The owner declined another
// demonstration and asked for the diagnosis off the pixels already on disk:
//
//	Specifically, determine whether each failure comes from:
//
//	 1. Proposal failure: The intended control isn't represented by a usable candidate rectangle.
//	 2. Identity failure: A candidate exists, but `sameRect` rejects it or cannot distinguish it
//	    from another.
//	 3. Evidence failure: The demonstration doesn't establish which candidate represents the
//	    intended control.
//
//	Those are different problems and should not receive the same fix.
//
// So this does not score anything. It prints the RELATIONSHIP between the rectangle a press was
// recorded as and the rectangles the proposer offers on later looks of the same screen, because
// those three failures have three different geometric signatures:
//
//	nothing is near the reading at all          -> the control is not proposed    (proposal)
//	something is there and centre containment   -> the relation rejects it        (identity)
//	   is one-way, or it cannot be told from a      or cannot separate it
//	   region that merely contained the press
//	the reading itself is not the control        -> the press never established it (evidence)
//
// The third is the one a geometric report cannot settle by itself, which is why `-crops` writes
// the actual pixels of the reading, its containers and the best later proposal. A rectangle that
// is 7px tall is not a button whatever the relation says about it.
//
// Nothing here matches, folds, promotes or presses. It is a reader for evidence that already
// exists.

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// relation is how one proposal stands to one stored reading of a control.
//
// Ordered worst to best deliberately: the best relation any proposal reaches on a look is the
// whole diagnosis of that look, and an ordered type makes "the closest anything got" a max
// rather than a cascade of ifs somebody has to keep in step.
type relation int

const (
	// relNone is no overlap whatsoever. The control is not represented here.
	relNone relation = iota
	// relOverlap is overlapping pixels with neither centre inside the other.
	//
	// The signature of a proposer that found a DIFFERENT thing in the same place — a
	// taller block starting above the row, a column crossing it.
	relOverlap
	// relInsideReading is a proposal whose centre is inside the reading, while the
	// reading's centre is outside it: a FRAGMENT of the control.
	relInsideReading
	// relSwallowsReading is a proposal containing the reading's centre while the
	// reading does not contain its: the control read as part of something larger.
	relSwallowsReading
	// relMutualIndistinct is mutual centre containment with a region that merely
	// contained the original press, so it cannot separate the control from its
	// container.
	relMutualIndistinct
	// relMutual is mutual centre containment and distinct from every container: this is
	// support, and a look with one such group matches.
	relMutual
)

func (r relation) String() string {
	switch r {
	case relOverlap:
		return "overlapping, neither centre inside the other"
	case relInsideReading:
		return "a fragment inside the reading"
	case relSwallowsReading:
		return "larger, and it swallows the reading's centre"
	case relMutualIndistinct:
		return "mutual, but indistinct from a container"
	case relMutual:
		return "mutual and distinct — this is support"
	default:
		return "nothing overlaps the reading"
	}
}

// lookRecord is one look's proposals and the screen that was standing before it was folded.
//
// Kept for the whole recording so the geometry can be examined against the FINAL registry: a
// target demonstrated at look 600 has to be looked for on look 100 too, and the correlator does
// not know it exists until the press arrives.
type lookRecord struct {
	look          int
	screen        observe.VisualScreenID
	regions       []observe.Region
	presses       []observe.PointerAt
	file          string
	width, height int
}

// bestRelation is the closest any proposal on one look got to one stored reading.
type bestRelation struct {
	rel      relation
	proposal observe.Region
	iou      float64
	areas    float64 // the proposal's area as a multiple of the reading's
	reading  observe.Region
	found    bool
}

// demonstration is one press of one target and everything the look offered around it.
type demonstration struct {
	look int
	at   observe.PointerAt
	// containing is every proposal the press was inside, smallest first — the candidate
	// set `Press` itself ranked. Recomputed here for reporting; the production decision
	// is already in the registry and this never overrides it.
	containing []observe.Region
	// nearest is the proposals whose centres are closest to the press, whether or not
	// they contain it.
	//
	// What answers the owner's first category: a control that is not represented by any
	// candidate rectangle is a PROPOSAL failure, and the only way to see that is to look
	// at what the proposer did find in the same place.
	nearest []observe.Region
}

// excluded is one group of proposals that agreed with a control and could not be told from a
// container, with how many looks it happened on.
type excluded struct {
	rep   observe.Region
	looks int
}

// diagnosis is one target's whole record over the recording.
type diagnosis struct {
	rec observe.TargetRecord
	// pressLooks is the looks this target was pressed on, excluded from everything below.
	pressLooks []int
	presses    []demonstration
	// afterFirst is how many of the later looks happened AFTER the demonstration.
	//
	// Separated because the production matcher could not have been asked about the others:
	// a target does not exist until its press is folded, so a look of its screen from
	// before that is evidence about the proposer and not about the matcher. Reporting the
	// two as one number is the mistake this subsystem has now made twice.
	afterFirst int
	// dropped is the proposals that agreed with the control and were refused for being
	// indistinct from a container, grouped by rectangle.
	dropped []excluded
	// later is one entry per look of the target's own screen that was not a press look.
	later []bestRelation
	// counts is how many of those looks had each relation as their best.
	counts map[relation]int
	// closest is the single best relation reached anywhere, and on which look.
	closest     bestRelation
	closestLook int
	// proposalsMin/Med/Max is how many rectangles the proposer offered on those looks.
	proposalsMin, proposalsMed, proposalsMax int
	// resolved is how many of those looks the PRODUCTION matcher called matched. Taken
	// from the production path, not recomputed, so this report cannot disagree with the
	// thing it is diagnosing.
	resolved, ambiguous, unobservable int
}

// reportTargetDiagnosis replays the recording, then explains every demonstrated target's later
// looks in terms of the geometry that decided them.
func reportTargetDiagnosis(dir string, limit int, only string, cropDir string) error {
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

	const interval = 100 * time.Millisecond
	obs, err := fastobserve.New(interval, 64)
	if err != nil {
		return err
	}
	go func() {
		for range obs.Updates() {
		}
	}()

	opt := structure.DefaultOptions()
	base := time.Unix(1700000000, 0).UTC()
	nextInput := 0
	looks := make([]lookRecord, 0, len(frames))
	// production is the production matcher's own verdict per look per target, so the
	// explanation below can be checked against the thing it explains rather than
	// standing beside it as a second opinion.
	production := map[observe.VisualTargetID]map[int]observe.ReacquireResult{}
	look := 0
	for _, fr := range frames {
		look++
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
			res := structure.Propose(img, opt)
			rects := make([]image.Rectangle, 0, len(res.Candidates))
			for _, c := range res.Candidates {
				rects = append(rects, c.Bounds)
			}
			lk.Regions = observe.NormaliseRects(img.Bounds(), rects)
		}
		// THE PRODUCTION VERDICT, taken before the fold exactly as the scorer takes
		// it, so a look's explanation and its outcome come from one ordering.
		for _, r := range obs.Reacquire(lk) {
			if production[r.Target] == nil {
				production[r.Target] = map[int]observe.ReacquireResult{}
			}
			production[r.Target][look] = r.Result
		}
		looks = append(looks, lookRecord{
			look: look, screen: obs.Screen(), regions: lk.Regions,
			presses: lk.Presses, file: fr.File,
			width: fr.Width, height: fr.Height,
		})
		obs.Observe(lk)
	}
	obs.Close()

	want := map[string]bool{}
	for _, id := range strings.Split(only, ",") {
		if id = strings.TrimSpace(id); id != "" {
			want[id] = true
		}
	}
	var out []diagnosis
	for _, rec := range obs.Registry() {
		if len(want) > 0 && !want[string(rec.ID)] {
			continue
		}
		out = append(out, diagnose(rec, looks, production[rec.ID]))
	}
	printDiagnosis(dir, out)
	if cropDir != "" {
		if err := writeCrops(dir, cropDir, out, looks); err != nil {
			return err
		}
	}
	return nil
}

// diagnose explains one target's later looks.
func diagnose(rec observe.TargetRecord, looks []lookRecord,
	verdicts map[int]observe.ReacquireResult) diagnosis {

	d := diagnosis{rec: rec, counts: map[relation]int{}}
	pressed := map[int]bool{}
	for _, lr := range looks {
		// A PRESS LOOK IS THE DEMONSTRATION, not a reacquisition. Detected the same
		// way the scorer detects it: the press landed on a region that agrees with a
		// stored reading of this target.
		if lr.screen != rec.Screen {
			continue
		}
		for _, p := range lr.presses {
			landed, ok := smallestContaining(lr.regions, p)
			if ok && agreesWithAny(rec.Where, landed) {
				pressed[lr.look] = true
				d.pressLooks = append(d.pressLooks, lr.look)
				d.presses = append(d.presses, demonstrate(lr, p))
			}
		}
	}
	var proposals []int
	drop := map[observe.Region]int{}
	for _, lr := range looks {
		if lr.screen != rec.Screen || pressed[lr.look] {
			continue
		}
		b := closestProposal(rec, lr.regions)
		d.later = append(d.later, b)
		d.counts[b.rel]++
		if len(d.pressLooks) > 0 && lr.look > d.pressLooks[0] {
			d.afterFirst++
		}
		if b.rel == relMutualIndistinct {
			drop[b.proposal]++
		}
		proposals = append(proposals, len(lr.regions))
		if !d.closest.found || b.rel > d.closest.rel ||
			(b.rel == d.closest.rel && b.iou > d.closest.iou) {
			d.closest, d.closestLook = b, lr.look
		}
		switch verdicts[lr.look] {
		case observe.ReacquireMatched:
			d.resolved++
		case observe.ReacquireAmbiguous:
			d.ambiguous++
		case observe.ReacquireUnobservable:
			d.unobservable++
		}
	}
	for r, n := range drop {
		placed := false
		for i := range d.dropped {
			if sameRegion(d.dropped[i].rep, r) {
				d.dropped[i].looks += n
				placed = true
				break
			}
		}
		if !placed {
			d.dropped = append(d.dropped, excluded{rep: r, looks: n})
		}
	}
	sort.Slice(d.dropped, func(i, j int) bool {
		return d.dropped[i].looks > d.dropped[j].looks
	})
	if len(proposals) > 0 {
		sort.Ints(proposals)
		d.proposalsMin = proposals[0]
		d.proposalsMed = proposals[len(proposals)/2]
		d.proposalsMax = proposals[len(proposals)-1]
	}
	return d
}

// demonstrate records what one look offered around one press.
//
// The containing set is `Press`'s own ranking recomputed for the report. The nearest set is this
// file's alone and decides nothing: it exists so a reader can see whether the control was
// proposed at all, which is the difference between a matcher that refused and a proposer that
// found nothing to refuse.
func demonstrate(lr lookRecord, at observe.PointerAt) demonstration {
	d := demonstration{look: lr.look, at: at}
	for _, r := range lr.regions {
		if containsNormalised(r, at) {
			d.containing = append(d.containing, r)
		}
	}
	sort.Slice(d.containing, func(i, j int) bool {
		return area(d.containing[i]) < area(d.containing[j])
	})
	near := append([]observe.Region{}, lr.regions...)
	dist := func(r observe.Region) float64 {
		c := centre(r)
		dx, dy := c.X-at.X, c.Y-at.Y
		return dx*dx + dy*dy
	}
	sort.Slice(near, func(i, j int) bool { return dist(near[i]) < dist(near[j]) })
	if len(near) > 6 {
		near = near[:6]
	}
	d.nearest = near
	return d
}

// closestProposal is the best relation any of a look's proposals reaches with a target.
func closestProposal(rec observe.TargetRecord, regions []observe.Region) bestRelation {
	best := bestRelation{}
	for _, p := range regions {
		r := relate(rec, p)
		i := iou(rec.Where[0], p)
		if !best.found || r > best.rel || (r == best.rel && i > best.iou) {
			best = bestRelation{
				rel: r, proposal: p, iou: i, found: true,
				reading: rec.Where[0],
				areas:   area(p) / max(area(rec.Where[0]), 1e-12),
			}
		}
	}
	return best
}

// relate classifies one proposal against one target's readings and containers.
//
// The two mutual cases are the production rule verbatim — `sameRect` against the readings, then
// the container exclusion — so a `relMutual` here is a proposal the production matcher counted
// as support. The weaker cases are this file's own vocabulary and decide nothing.
func relate(rec observe.TargetRecord, p observe.Region) relation {
	if agreesWithAny(rec.Where, p) {
		if agreesWithAny(rec.Containers, p) {
			return relMutualIndistinct
		}
		return relMutual
	}
	w := rec.Where[0]
	inP := containsNormalised(p, centre(w))
	inW := containsNormalised(w, centre(p))
	switch {
	case inP:
		return relSwallowsReading
	case inW:
		return relInsideReading
	case iou(w, p) > 0:
		return relOverlap
	}
	return relNone
}

func centre(r observe.Region) observe.PointerAt {
	return observe.PointerAt{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
}

func area(r observe.Region) float64 { return r.Width * r.Height }

// iou is intersection over union, for reporting only.
//
// A fraction, and deliberately not a decision: the production relation is threshold-free and
// this report exists to show what it refused, not to propose a number that would have accepted
// it.
func iou(a, b observe.Region) float64 {
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1 := min(a.X+a.Width, b.X+b.Width)
	y1 := min(a.Y+a.Height, b.Y+b.Height)
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	in := (x1 - x0) * (y1 - y0)
	un := area(a) + area(b) - in
	if un <= 0 {
		return 0
	}
	return in / un
}

func printDiagnosis(dir string, all []diagnosis) {
	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("WHY A DEMONSTRATED CONTROL IS OR IS NOT FOUND AGAIN — %s\n",
		filepath.Base(dir))
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  Geometry only. Nothing here matches, promotes or presses.\n")

	order := []relation{relMutual, relMutualIndistinct, relSwallowsReading,
		relInsideReading, relOverlap, relNone}
	for _, d := range all {
		rec := d.rec
		w := rec.Where[0]
		fmt.Printf("\n%s\n", repeat("-", 78))
		fmt.Printf("%s on %s — %d press(es), resolved on %d of %d later look(s)\n",
			rec.ID, rec.Screen, rec.Presses, d.resolved, len(d.later))
		fmt.Printf("%s\n", repeat("-", 78))
		fmt.Printf("  THE DEMONSTRATION\n")
		for _, pr := range d.presses {
			fmt.Printf("    look %d, pressed at %.4f,%.4f — %d proposal(s) "+
				"contained it:\n", pr.look, pr.at.X, pr.at.Y,
				len(pr.containing))
			for i, r := range pr.containing {
				tag := "container"
				if i == 0 {
					tag = "THE IDENTITY"
				}
				if i > 0 && sameRegion(r, pr.containing[0]) {
					tag += " — AND THE SAME RECTANGLE AS IT"
				}
				fmt.Printf("      %s  %s\n", describe(r), tag)
			}
			fmt.Printf("      the 6 proposals nearest the press, containing " +
				"it or not:\n")
			for _, r := range pr.nearest {
				in := " "
				if containsNormalised(r, pr.at) {
					in = "*"
				}
				fmt.Printf("      %s %s\n", in, describe(r))
			}
		}
		fmt.Printf("    the reading (the identity): %s\n", describe(w))
		if rec.AmbiguousPresses > 0 {
			fmt.Printf("    %d of its presses landed inside more than one region\n",
				rec.AmbiguousPresses)
		}
		switch len(rec.Containers) {
		case 0:
			fmt.Printf("    no container was recorded: the press was inside " +
				"nothing larger\n")
		default:
			fmt.Printf("    %d recorded container(s), largest last:\n",
				len(rec.Containers))
			cs := append([]observe.Region{}, rec.Containers...)
			sort.Slice(cs, func(i, j int) bool { return area(cs[i]) < area(cs[j]) })
			for _, c := range cs {
				note := ""
				if sameRegion(c, w) {
					note = "  <-- THE SAME RECTANGLE AS THE READING"
				}
				fmt.Printf("      %s  %.1fx the reading%s\n",
					describe(c), area(c)/area(w), note)
			}
		}
		if rec.MaybeOneControlWith != nil {
			fmt.Printf("    the session could not tell this identity from %v\n",
				rec.MaybeOneControlWith)
		}
		fmt.Printf("\n  LATER LOOKS OF %s: %d, excluding %d press look(s)\n",
			rec.Screen, len(d.later), len(d.pressLooks))
		fmt.Printf("    %d of them AFTER the first press — the only ones the "+
			"matcher could be asked about\n", d.afterFirst)
		if len(d.later) == 0 {
			fmt.Printf("    its screen never stood again, so nothing here is " +
				"about the matcher\n")
			continue
		}
		fmt.Printf("    proposals per look: min %d, median %d, max %d\n",
			d.proposalsMin, d.proposalsMed, d.proposalsMax)
		fmt.Printf("    production verdict: %d matched, %d ambiguous, %d unobservable\n",
			d.resolved, d.ambiguous, d.unobservable)
		fmt.Printf("    the CLOSEST any proposal got, counted per look:\n")
		for _, r := range order {
			if d.counts[r] == 0 {
				continue
			}
			fmt.Printf("      %5d  %s\n", d.counts[r], r)
		}
		for _, e := range d.dropped {
			fmt.Printf("    refused on %d look(s) for being indistinct from a "+
				"container:\n      %s\n", e.looks, describe(e.rep))
		}
		if d.closest.found {
			c := d.closest
			fmt.Printf("    the closest relation reached anywhere, look %d:\n",
				d.closestLook)
			fmt.Printf("      %s  %.1fx the reading, IoU %.2f\n",
				describe(c.proposal), c.areas, c.iou)
			fmt.Printf("      it contains the reading's centre: %-5t   "+
				"the reading contains its centre: %t\n",
				containsNormalised(c.proposal, centre(c.reading)),
				containsNormalised(c.reading, centre(c.proposal)))
		}
	}
}

// describe prints a region as both a fraction and the pixels it was proposed from.
//
// Both, because they answer different questions: the fraction is what the matcher compares and
// the pixel size is what tells a reader whether the rectangle could be a button at all.
func describe(r observe.Region) string {
	return fmt.Sprintf("%.4f,%.4f %.4fx%.4f  (%dx%d px of 1920x1080)",
		r.X, r.Y, r.Width, r.Height,
		int(r.Width*1920+0.5), int(r.Height*1080+0.5))
}

// writeCrops writes the pixels behind the geometry: the reading, its containers and the closest
// later proposal, outlined on a context crop.
//
// Because the third failure the owner named cannot be settled by rectangles. A reading that is
// seven pixels tall is not a button whatever relation it reaches, and only the pixels say so.
func writeCrops(dir, out string, all []diagnosis, looks []lookRecord) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	byLook := map[int]lookRecord{}
	for _, lr := range looks {
		byLook[lr.look] = lr
	}
	for _, d := range all {
		w := d.rec.Where[0]
		if len(d.pressLooks) > 0 {
			lr := byLook[d.pressLooks[0]]
			name := fmt.Sprintf("%s-press-look%d", d.rec.ID, lr.look)
			if err := crop(dir, out, name, lr, w, d.rec.Containers, nil); err != nil {
				return err
			}
			if err := full(dir, out, fmt.Sprintf("%s-screen-look%d", d.rec.ID,
				lr.look), lr, w); err != nil {
				return err
			}
		}
		// THE LOOK THE MATCHER CAME CLOSEST ON, which is the one worth seeing: if
		// the control is plainly there and the relation still refused it, that is an
		// identity failure and the pixels are the proof.
		if d.closestLook > 0 {
			lr := byLook[d.closestLook]
			name := fmt.Sprintf("%s-closest-look%d", d.rec.ID, lr.look)
			var best []observe.Region
			if d.closest.found {
				best = []observe.Region{d.closest.proposal}
			}
			if err := crop(dir, out, name, lr, w, d.rec.Containers, best); err != nil {
				return err
			}
			if err := full(dir, out, fmt.Sprintf("%s-screen-look%d", d.rec.ID,
				lr.look), lr, w); err != nil {
				return err
			}
		}
	}
	fmt.Printf("\n  crops written to %s\n", out)
	return nil
}

// crop writes a context crop around a reading with the rectangles outlined.
func crop(dir, out, name string, lr lookRecord, reading observe.Region,
	containers, best []observe.Region) error {

	img, err := readPNG(filepath.Join(dir, filepath.FromSlash(lr.file)))
	if err != nil {
		return err
	}
	b := img.Bounds()
	rr := pixels(reading, b)
	// A MARGIN PROPORTIONAL TO THE READING, floored so a tiny reading is still
	// legible: a 20px crop of a 20px rectangle shows a reader nothing about where it is.
	mx := maxInt(rr.Dx(), 80)
	my := maxInt(rr.Dy()*3, 80)
	box := image.Rect(rr.Min.X-mx, rr.Min.Y-my, rr.Max.X+mx, rr.Max.Y+my).Intersect(b)
	dst := image.NewRGBA(image.Rect(0, 0, box.Dx(), box.Dy()))
	draw.Draw(dst, dst.Bounds(), img, box.Min, draw.Src)
	off := box.Min
	for _, c := range containers {
		outline(dst, pixels(c, b).Sub(off), color.RGBA{0, 160, 255, 255})
	}
	for _, p := range best {
		outline(dst, pixels(p, b).Sub(off), color.RGBA{0, 255, 60, 255})
	}
	outline(dst, rr.Sub(off), color.RGBA{255, 40, 40, 255})
	return writePNG(filepath.Join(out, name+".png"), dst)
}

// full writes the whole look with the reading outlined, so a reader can see which screen it is.
func full(dir, out, name string, lr lookRecord, reading observe.Region) error {
	img, err := readPNG(filepath.Join(dir, filepath.FromSlash(lr.file)))
	if err != nil {
		return err
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	outline(dst, pixels(reading, b).Sub(b.Min), color.RGBA{255, 40, 40, 255})
	return writePNG(filepath.Join(out, name+".png"), dst)
}

// pixels converts a normalised region back to the pixels of one frame.
func pixels(r observe.Region, b image.Rectangle) image.Rectangle {
	w, h := float64(b.Dx()), float64(b.Dy())
	return image.Rect(
		b.Min.X+int(r.X*w+0.5), b.Min.Y+int(r.Y*h+0.5),
		b.Min.X+int((r.X+r.Width)*w+0.5), b.Min.Y+int((r.Y+r.Height)*h+0.5),
	)
}

// outline draws a two-pixel border, clipped to the image.
func outline(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	for t := 0; t < 2; t++ {
		e := r.Inset(t)
		for x := e.Min.X; x < e.Max.X; x++ {
			dst.Set(x, e.Min.Y, c)
			dst.Set(x, e.Max.Y-1, c)
		}
		for y := e.Min.Y; y < e.Max.Y; y++ {
			dst.Set(e.Min.X, y, c)
			dst.Set(e.Max.X-1, y, c)
		}
	}
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
