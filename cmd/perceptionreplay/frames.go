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
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
)

// THE REPLAY NO LONGER OWNS A RESPONSE WINDOW.
//
// It had one: a 400ms lookback, deciding for itself which frames a press was still relevant
// to. It was the right number and it was a second copy of a rule production also holds, which
// is how this repository's worst class of defect begins - a spectest that passed while
// production refused the same play, because two statements of one rule drifted.
//
// The inputs are now handed to ShadowTotals as the InputEvents they are, on the slot they
// happened during, and `observe.MaxResponseLatency` answers the question in exactly one place.
// A replay whose input differs from production's input measures the replay.

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
	// Where the press was, for a pointer press. Normalised against the rectangle that
	// frame was captured from, which is the same rectangle every region below is a
	// fraction of — so a press and a row are comparable without a transform.
	//
	// Read back because it was being dropped on BOTH sides: the writer took Intent and
	// AtMS off the event, and this took Intent and AtMS off the line. A mouse
	// demonstration replayed as a sequence of `point`s with no positions, which is a
	// record that somebody clicked and no record of what they clicked.
	Where observe.PointerAt `json:"where"`
	// Device is which hardware reported it. Carried for the same reason the recorder
	// carries it: a quiet keyboard and an unwatched one are different facts.
	Device string `json:"device"`
}

// annotationFile is a recording's labels, when it has any.
type annotationFile struct {
	Session    string               `json:"session"`
	Provenance string               `json:"provenance"`
	Intervals  []annotationInterval `json:"intervals"`
}

// annotationInterval is one run of frames the labels agree about.
//
// Named rather than anonymous because the confirmation measurement walks these in order to
// find where one labelled row gave way to the next, and an anonymous struct cannot be passed
// to a function that does.
type annotationInterval struct {
	From  int     `json:"from"`
	To    int     `json:"to"`
	State string  `json:"state"`
	Entry string  `json:"entry"`
	Y     float64 `json:"y"`
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
	// HeldDropped is why the mark's last sighting was let go, by cause. Each is a move
	// that could not be recorded between two sightings of one list.
	HeldDropped map[string]int
	// Retired is how many tracked lists were dropped for not being found again. A
	// population being pruned and one that was never crowded look the same from a count
	// of the survivors.
	Retired int
	// Proposed is total candidates over all frames, and PerFrame the mean. The precision
	// proxy: every false rectangle is a possible list member on every frame.
	Proposed int
	// Grouped is frames whose proposals produced a stack the tracker would consider.
	Grouped int
	// CoveredSelected is frames where some proposal covered the labelled selected row,
	// and LabelledFrames how many frames carried a label to check against.
	CoveredSelected, LabelledFrames int
	// WithSelection is frames where the belief named an entry; CorrectSelection those
	// where the named ENTRY's own region sat at the labelled row's height.
	WithSelection, CorrectSelection int
	// SameEntryCount is frames where the identified list had as many entries as the
	// annotation describes, and CorrectOrdinal those where it also named the same one,
	// counted from the top.
	//
	// THE SCORE A ROUTE WOULD DEPEND ON, and the only one that cannot be satisfied by a
	// rectangle that happens to overlap. "Move the mark to entry 5" is meaningless if the
	// list Marco is reading has four entries and the menu has nine.
	SameEntryCount, CorrectOrdinal int
	// ProposeMS and DecodeMS are measured cost, summed.
	ProposeMS, DecodeMS int64
	Belief              observe.InterfaceBelief
	Candidates          []observe.InterfaceCandidate
	Identified          int
	Inputs              int
	// VisualSeen, VisualTransitions and VisualStability are the SESSION-LOCAL screen
	// hypotheses: arrangements stabilised over time, which is the layer the raw clustering
	// below says is needed and does not provide. PressScreens is each press by the settled
	// screen it was made on and the one that followed it.
	//
	// Never durable. See observe/arrangement.go.
	// Walk is the screens in the order the session settled on them, which is where a
	// repeated route is read from. Counts lose the order and occurrences interleave.
	Walk []observe.VisualScreenID
	// Arrangement looks up a settled screen's stored arrangement, for the measurement
	// that compares them against one another. Nil on a result nobody replayed.
	Arrangement func(observe.VisualScreenID) observe.VisualArrangement
	// Clicks is the learning units — source screen, visual target, destination screen —
	// with the presses that changed nothing and the changes nobody pressed for reported
	// beside them. Targets is every session-local control identity minted.
	Clicks            observe.ClickEvidence
	Targets           []observe.TargetSeen
	VisualSeen        []observe.VisualScreenSeen
	VisualTransitions []observe.VisualTransition
	VisualStability   observe.VisualStability
	PressScreens      []pressScreen
	// PixelClusters is what an unnamed-geometry cue WOULD have segmented this recording
	// into, PixelScreenChanges how often adjacent frames fell in different clusters, and
	// PressOutcomes whether each press was followed by a different one.
	//
	// A MEASUREMENT AND NOT A BELIEF. Marco's own screen model is States/Transitions
	// below, and on the pixels path that is one state and no transitions. See screens.go
	// for why the question is worth asking separately.
	PixelClusters      []screenCluster
	PixelScreenChanges int
	PressOutcomes      []pressOutcome
	// PointsSeen is pointer presses the replay was given, PointsOnAShape those that
	// landed inside at least one region the proposer found on that frame, and
	// PointsAmbiguous those inside more than one.
	//
	// The three are the mouse half of the funnel. A press on nothing cannot be learned
	// as anything but a coordinate; a press inside two nested regions has an answer
	// that needs choosing, which is the same "smallest containing control wins" rule
	// the live pointer path applies and is NOT applied here — this counts the question,
	// it does not answer it.
	PointsSeen, PointsOnAShape, PointsAmbiguous int
	// NavsSeen is the directional intents the replay actually handed to a slot. Below
	// Inputs by the non-directional ones, and a replay that joined nothing reports zero
	// here rather than looking like a session nobody played.
	NavsSeen int
	// Moves is what the session RETAINED about the mark: every move it was seen making
	// between two entries of one list, which is the evidence a navigation route would be
	// built from. MovesUnplaced and MovesDropped are what the grounding and the bounds
	// refused, reported beside it rather than subtracted from it.
	Moves                       []observe.SelectionTransition
	MovesUnplaced, MovesDropped int
	// Changes is the screen transitions themselves, so the activation evidence on each one
	// can be reported rather than only counted. See reportActivations.
	Changes []observe.ScreenTransition
	// States and Transitions are how much SCREEN model formed. Reported because on the
	// pixels discovery path the honest answer is expected to be very little - segmentation
	// reads named roles and the proposer names nothing - and a reader has to be able to
	// tell that from a bug.
	States, Transitions int
	// Hypotheses is what the session would say about all of it.
	Hypotheses []observe.Hypothesis
	// Latencies is how long after a press the mark was first SEEN somewhere else, in
	// milliseconds, one entry per move. The measurement behind
	// observe.MaxResponseLatency, which until now was an assumption.
	Latencies []int64
	// Trace is what the belief said on every scored frame, and Score and Confirmations
	// what that comes to when counted against the labels.
	//
	// Kept whole rather than accumulated into counters because the questions that matter
	// are about sequences — how long after the pixels changed did the belief agree, and
	// what did it say in between — and a counter cannot answer those. See score.go for
	// why coverage and accuracy are never divided by each other.
	Trace         []frameObservation
	Score         selectionScore
	Confirmations []confirmation
	// Lists is which list the belief was reading, scored as an identity. See
	// listidentity.go for why that is a different question from the selection score.
	Lists listScoreboard
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
	// THE RECORDING'S OTHER HALF, on the same measured clock. Sorted so the join
	// below can walk it once: an input log written as events arrived is already in
	// order, and a replay that assumed so without checking would silently drop the
	// tail of a log some future writer buffered.
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })

	var ann *annotationFile
	if b, err := os.ReadFile(filepath.Join(dir, "annotations.json")); err == nil {
		var a annotationFile
		if err := json.Unmarshal(b, &a); err == nil {
			ann = &a
		}
	}

	// THE PRODUCTION ACCUMULATOR, not a bare interface watch.
	//
	// It was a watch, which answered the question that mattered then: can pixels read this
	// menu. The question now is what a session KEEPS once it has, and the keeping happens in
	// ShadowTotals.Observe - the call the live cycle makes. A replay that skipped it would be
	// measuring a belief nothing downstream ever sees, and the selection ledger, the screen
	// model and the attribution window would all be exercised by tests alone.
	var totals observe.ShadowTotals
	labelRows := ann.labelledRows()
	pitch := labelledPitch(labelRows)
	// A deterministic clock seeded from the recording's own measured offsets, so a replay
	// run twice gives byte-identical belief timestamps.
	base := time.Unix(1700000000, 0).UTC()
	opt := structure.DefaultOptions()
	nextInput := 0
	// The arrangement measurement's inputs, gathered per frame and resolved once at the
	// end. See screens.go: it is what an unnamed-geometry cue WOULD segment, and it is
	// not what Marco believes.
	var pixelSigs []observe.ScreenSignature
	var pressFrames []int
	// The stabilised layer, folded as the looks arrive because that is what it is for: a
	// screen hypothesis is a claim about consecutive time and cannot be reconstructed from
	// an unordered set of frames afterwards.
	var screens observe.VisualScreens
	var perLook []observe.VisualScreenID
	// The presses, correlated with the changes that followed them. Folded beside the
	// stabiliser because the learning unit needs both halves on one clock.
	var clicks observe.ClickTargets

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

		// WAS THE PERSON NAVIGATING AROUND WHEN THIS FRAME WAS TAKEN, joined on time.
		//
		// On time and never on the frame index, which is the trap the subsystem note
		// records: a press carries one counter and a belief moment another, and the two
		// are equal only when the inference blocks the cycle.
		//
		// A LOOKBACK AND NOT A CONSUMING WALK, and the measurement is the argument. A
		// menu repaints a few hundred milliseconds after the press; this recording is
		// sampled every 66ms. Attributing each press only to the frame that closes its
		// own interval credited 4 of the menu's 31 transfers, because the mark had not
		// moved yet when that frame was taken. A press is RECENT for a while, so one
		// press can be recent for several consecutive fine-grained frames - which is
		// exactly what a single coarse look would have covered on its own.
		// THE PRESSES THAT HAPPENED DURING THIS FRAME'S SLOT, consumed once each.
		//
		// A CONSUMING WALK, where this used to keep a 400ms lookback of its own. The
		// lookback existed because the tracker once counted its window in observations
		// and needed a replay-side correction; the window is a duration now, held in
		// observe.MaxResponseLatency, and it is applied to these events by the same code
		// that applies it live. A press belongs to the slot it happened in, exactly as
		// the input producer delivers it.
		var slot []observe.InputEvent
		for nextInput < len(inputs) && inputs[nextInput].AtMS <= fr.AtMS {
			in := inputs[nextInput]
			nextInput++
			intent := observe.NavIntent(in.Intent)
			switch intent {
			case observe.NavUp, observe.NavDown, observe.NavLeft, observe.NavRight:
				r.NavsSeen++
			case observe.NavPoint:
				r.PointsSeen++
			}
			slot = append(slot, observe.InputEvent{
				Intent: intent, AtMS: in.AtMS,
				Where:  in.Where,
				Device: observe.NavDevice(in.Device),
			})
		}
		t1 := time.Now()
		res := structure.Propose(img, opt)
		r.ProposeMS += time.Since(t1).Milliseconds()
		r.Proposed += len(res.Candidates)

		// THE PIXELS ARE MEASURED HERE, which is the whole point of replaying at this
		// level: shading comes from the frame through the production measurement, not
		// from whatever a previous detector happened to record.
		shapes := shapesFrom(img, res.Candidates)
		// ── DID THE PRESS LAND ON ANYTHING MARCO COULD SEE ──
		//
		// The first question a mouse demonstration raises, and the one that decides
		// whether a click is learnable at all: a press inside a region perception
		// proposed can be re-grounded later by finding that region again, and a press
		// on bare background can only ever be replayed as a coordinate.
		//
		// Asked of the RAW proposals rather than of the identified list, and counted
		// apart from it, because the two failures want opposite work: a press on
		// nothing says the proposer missed the control, and a press on a proposal that
		// no list claimed says grouping or membership did.
		//
		// Nothing here decides what was clicked. It counts whether an answer was
		// possible.
		scorePresses(slot, shapes, &r)
		// AND WHAT THE ARRANGEMENT WOULD HAVE SAID, collected per frame and clustered
		// once at the end: a greedy assignment needs the clusters that exist so far,
		// and building them here would interleave the measurement with the belief.
		pixelSigs = append(pixelSigs, pixelSignature(shapes))
		// ── THE PRESS FIRST, then the arrangement, then the change ──
		//
		// The screen a press was made ON is the one that was standing when the person
		// clicked, which is the screen BEFORE this look is folded. Folding the look
		// first would, on the one look in a session that both carries a press and
		// completes a promotion, record the press as having been made on the screen it
		// arrived at — a learning unit whose source is its own destination.
		rows := regionsOf(shapes)
		standing := screens.Screen()
		for _, e := range slot {
			if e.Intent != observe.NavPoint {
				continue
			}
			clicks.Press(len(pixelSigs)-1, e.Where, standing, rows)
		}
		perLook = append(perLook, screens.Observe(
			observe.ArrangeLook(rows),
			base.Add(time.Duration(fr.AtMS)*time.Millisecond)))
		if t, ok := screens.Promoted(); ok {
			clicks.Arrived(len(pixelSigs)-1, t)
		}
		for _, e := range slot {
			if e.Intent == observe.NavPoint {
				pressFrames = append(pressFrames, len(pixelSigs)-1)
			}
		}
		at := base.Add(time.Duration(fr.AtMS) * time.Millisecond)
		if sizes := observe.GroupedForLook(observe.InterfaceLook{
			Observed: true, Shapes: shapes,
		}); len(sizes) > 0 && sizes[0] >= minUsableStack {
			r.Grouped++
		}
		totals.Observe(observe.Sample{
			Sequence: fr.Index, Timestamp: at, WindowGeneration: 1,
			Shadow: &observe.ShadowSample{
				Detector: "structure-proposer", Ran: true, TargetProven: true,
				Front: observe.FrontTarget, Shapes: shapes, Inputs: slot,
			},
		})
		b := totals.Interface

		state, y, labelled := ann.labelAt(fr.Index)
		if labelled && state == "selected" {
			r.LabelledFrames++
			if coversHeight(res.Candidates, y, img.Bounds()) {
				r.CoveredSelected++
			}
		}
		if b.Selected >= 0 {
			r.WithSelection++
			if labelled && state == "selected" {
				// THE SCORE THAT MATTERS: did Marco point at the row the labels
				// say was selected, and not at its neighbour. Against the named
				// ENTRY's own region, at half the labelled pitch.
				if namesRow(b.SelectedWhere, y, pitch) {
					r.CorrectSelection++
				}
				// AND WHETHER THE LIST IT READ IS SHAPED LIKE THE MENU AT ALL.
				// Reported separately because a list with the right number of
				// entries can still be the wrong list, and on this corpus
				// frequently is: a six-row group plus three strays inside its
				// envelope counts nine.
				if b.Entries == len(labelRows) {
					r.SameEntryCount++
					if b.Selected == ordinalOf(labelRows, y) {
						r.CorrectOrdinal++
					}
				}
			}
		}
		// AND THE WHOLE FRAME'S ANSWER, for the scoring pass. Recorded for every
		// frame and not only the labelled ones, so the scorer rather than this loop
		// decides what counts — one place that knows the denominators.
		obs := frameObservation{
			Index: fr.Index, AtMS: fr.AtMS, Status: string(b.Status),
			Reading: b.Because, Selected: b.Selected, Entries: b.Entries,
			ListTop: b.Where.Y, ListBottom: b.Where.Y + b.Where.Height,
		}
		// WHERE THE SELECTED ROW IS LOST, asked of every labelled frame as a funnel.
		//
		// IN THE LIST'S OWN COLUMN, which the first version of this got wrong in an
		// instructive way. The annotation gives a row's HEIGHT and nothing else, so
		// "something was found at that height" is satisfied by anything anywhere across
		// the screen — and on this corpus it is: frame 273's labelled row at y 0.439 sits
		// at the same height as a six-row stack of its own at x 0.000 width 0.099, while
		// the menu being tracked is at x 0.107 width 0.348. Measured that way the row
		// looked grouped on 86% of frames and a member on 10%, and the gap was an
		// artefact of the question.
		//
		// The reference is the LIST'S column, taken from the list, which is the only thing
		// here that knows where the menu is. No geometry is assumed; a list on the other
		// side of the screen asks about its own side.
		if labelled && state == "selected" {
			obs.Labelled, obs.LabelledY = true, y

			members := totals.ActiveMembers
			obs.ListMembers = len(members)
			obs.MissingRows, obs.ListRows = totals.ActiveMissingRows, totals.ActiveRows
			if len(members) > 0 {
				x0, x1 := members[0].X, members[0].X+members[0].Width
				for _, m := range members {
					x0 = math.Min(x0, m.X)
					x1 = math.Max(x1, m.X+m.Width)
				}
				obs.RowIsMember = coversRow(members, y, x0, x1)
				shaped := make([]observe.Region, 0, len(shapes))
				for _, sh := range shapes {
					if len(sh.Shade) > 0 {
						shaped = append(shaped, sh.Region)
					}
				}
				obs.RowShaped = coversRow(shaped, y, x0, x1)
				stacks := observe.GroupedStacksForLook(observe.InterfaceLook{
					Observed: true, Shapes: shapes,
				})
				for _, stack := range stacks {
					if !coversRow(stack, y, x0, x1) {
						continue
					}
					obs.RowGrouped = true
					obs.RowStack = len(stack)
					for _, m := range members {
						if coversRow(stack, m.Y+m.Height/2, x0, x1) {
							obs.RowStackHolds++
						}
					}
					break
				}
			}
		}
		if b.Selected >= 0 {
			obs.NamedY = b.SelectedWhere.Y + b.SelectedWhere.Height/2
			obs.NamedHeight = b.SelectedWhere.Height
		}
		r.Trace = append(r.Trace, obs)

		if verbose {
			fmt.Printf("  frame %-5d %-12s sel %-3d  cands %-3d  label %-10s %s\n",
				fr.Index, b.Status, b.Selected, len(res.Candidates), state,
				trim(b.Because, 70))
		}
	}

	// THE MEASUREMENT, resolved. Greedy clustering needs every signature first, and a
	// settle window of ten frames is one second at the rate this recording ran at —
	// long enough for a menu to repaint and short enough that the next press has not
	// happened yet. The presses in rl-freeplay-1 are 2.3 seconds apart at the closest.
	const settleFrames = 10
	// A DEMONSTRATION'S LAST PRESS still belongs to it: the session ends before the
	// screen settles, and a press left pending would be lost rather than reported inert.
	clicks.Close()
	r.Clicks = clicks.Evidence()
	r.Targets = clicks.Targets()
	r.Walk = screens.Walk()
	r.Arrangement = screens.Arrangement
	r.VisualSeen = screens.Seen()
	r.VisualTransitions = screens.Transitions()
	r.VisualStability = screens.Stability()
	// EACH PRESS BY THE SCREEN IT WAS MADE ON AND THE ONE THAT FOLLOWED. The same settle
	// window the raw measurement uses, so the two answers are about the same question.
	for _, f := range pressFrames {
		if f < 0 || f >= len(perLook) {
			continue
		}
		after := f + settleFrames
		if after >= len(perLook) {
			after = len(perLook) - 1
		}
		r.PressScreens = append(r.PressScreens, pressScreen{
			Frame: f, Before: perLook[f], After: perLook[after]})
	}
	per, clusters := clusterScreens(pixelSigs)
	r.PixelClusters = clusters
	r.PixelScreenChanges = screenChanges(per)
	r.PressOutcomes = pressesAcrossScreens(pressFrames, per, settleFrames)

	r.Belief = totals.Interface
	r.Identified = r.Belief.After
	r.Candidates = totals.Candidates
	r.Retired = totals.RetiredCandidates
	r.HeldDropped = totals.HeldSightingsDropped
	r.Moves = totals.SelectionTransitions
	r.Latencies = responseLatencies(totals.InterfaceMoments, inputs, base)
	r.MovesUnplaced, r.MovesDropped = totals.SelectionMovesUnplaced, totals.SelectionMovesDropped
	r.States, r.Transitions = len(totals.States), len(totals.Transitions)
	r.Changes = totals.Transitions
	r.Hypotheses = observe.Hypotheses(totals, observe.DefaultHypothesisThresholds())
	r.Score = scoreSelection(r.Trace, ann)
	r.Confirmations = confirmations(r.Trace, ann)
	r.Lists = scoreLists(r.Trace, ann)
	reportFrames(r, ann)
	reportScreens(r)
	reportVisualScreens(r)
	reportClicks(r)
	reportClickRoute(r)
	verdicts := classifyScreens(r)
	reportTransients(r, verdicts)
	reportRouteStability(r, verdicts)
	reportTargetComparison(compareTargets(r))
	if r.Arrangement != nil {
		reportShapeVsCount(r, r.Arrangement)
	}
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

// scorePresses tallies what this frame's pointer presses landed on.
//
// # The question, and why it is only counted
//
// A press inside a region perception proposed can be re-grounded later by finding that region
// again. A press on bare background can only ever be replayed as a coordinate — and a route
// built on one would go somewhere else the moment the menu moved, with nothing to notice it.
// So this is the first thing to know about a mouse demonstration, before any question about
// what the thing clicked was called.
//
// Asked of the RAW proposals, and counted apart from the identified list, because the two
// failures want opposite work: a press on nothing says the proposer missed the control, and a
// press on a proposal no list claimed says grouping or membership did.
//
// Nothing here decides WHAT was clicked. The live pointer path resolves a press to the
// smallest containing control, and that rule lives there; a press under several regions is
// counted as needing that choice rather than having it made twice in two places.
//
// # Read from the events, not from the records they came from
//
// `slot` is what PRODUCTION receives. A press whose position failed to reach the event is a
// press the interface watch could not correlate against anything either, so a tally taken
// from the input records would be true about a field nothing downstream had.
func scorePresses(slot []observe.InputEvent, shapes []observe.ShadowShape, r *frameResult) {
	for _, e := range slot {
		if e.Intent != observe.NavPoint {
			continue
		}
		n := shapesUnder(shapes, e.Where)
		if n == 0 {
			continue
		}
		r.PointsOnAShape++
		if n > 1 {
			r.PointsAmbiguous++
		}
	}
}

// shapesUnder is how many proposed regions contain a normalised point.
//
// Inclusive of the left and top edges and exclusive of the right and bottom, the same
// half-open convention windowBounds.place uses, so a press at exactly the boundary between
// two stacked rows belongs to the lower one and to one of them only.
func shapesUnder(shapes []observe.ShadowShape, at observe.PointerAt) int {
	// A REGION WITH NO AREA NEEDS NO GUARD, and the one written here was dead code.
	// For zero width, `at.X >= r.X+r.Width` is `at.X >= r.X`, which is the negation of
	// the test beside it — so every point is already excluded, whatever its position.
	// Deleted rather than kept as reassurance: an unreachable check reads as a case
	// somebody handled.
	n := 0
	for _, sh := range shapes {
		r := sh.Region
		if at.X < r.X || at.Y < r.Y || at.X >= r.X+r.Width || at.Y >= r.Y+r.Height {
			continue
		}
		n++
	}
	return n
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

// namesRow reports whether a believed entry's region is the labelled row and not its
// neighbour.
//
// # The tolerance is half the LABELLED pitch, and that is the whole point
//
// What replaced it was three row-heights wide and centred below the row, so a believed region
// could name a row two places away and score. Worse, it was being handed the identified LIST's
// envelope rather than the named ENTRY's, and a list spanning three quarters of the screen
// contains every labelled row there is. The measurement reported 154 of 155 for two
// experiments. Scored against the named entry at this tolerance it is a small fraction of that.
//
// Half the pitch is the only tolerance that means anything here: a region is on this row
// exactly when it is closer to this row than to the one above or below. Tighter would score the
// annotation's eyeballed centres; looser cannot distinguish neighbours, which is the one
// distinction a navigation route is made of.
func namesRow(where observe.Region, y, pitch float64) bool {
	if where.Height <= 0 || pitch <= 0 {
		return false
	}
	return math.Abs(where.Y+where.Height/2-y) <= pitch/2
}

// labelledPitch is the spacing of the annotation's own rows, or 0 when it has too few to say.
func labelledPitch(rows []float64) float64 {
	if len(rows) < 2 {
		return 0
	}
	return (rows[len(rows)-1] - rows[0]) / float64(len(rows)-1)
}

// coversRow says one of these regions contains this height.
//
// Containment of a height and not of a rectangle, because the annotation gives the row's centre
// and nothing else: the question is whether the row at that height is among them.
func coversRow(rows []observe.Region, y, x0, x1 float64) bool {
	for _, r := range rows {
		if y < r.Y || y > r.Y+r.Height {
			continue
		}
		// AND IN THE SAME COLUMN. A row at the right height on the other side of the
		// screen is a different thing at the same height.
		lo, hi := math.Max(r.X, x0), math.Min(r.X+r.Width, x1)
		// The share is of the ROW'S OWN width, so a narrow row wholly inside the
		// list belongs to it and a band that merely crosses the list does not.
		if hi-lo >= r.Width/2 {
			return true
		}
	}
	return false
}

func reportFrames(r frameResult, ann *annotationFile) {
	labelRows := ann.labelledRows()
	fmt.Printf("\n%s\n", strings.Repeat("=", 78))
	fmt.Printf("REPLAY %s   (L0: the original pixels)\n", r.Name)
	fmt.Printf("%s\n", strings.Repeat("=", 78))
	fmt.Printf("  frames reprocessed  %d   presses in the recording %d\n",
		r.Frames, r.Inputs)
	// Attributions and not presses: one press is recent for several consecutive
	// fine-grained frames, so this exceeds the press count at a fast sampling rate
	// and equals it at a coarse one. A zero here on a session somebody played is
	// the tell that the join is broken.
	fmt.Printf("  nav attributions    %d directional press-to-frame attributions\n",
		r.NavsSeen)
	// THE MOUSE HALF, reported whenever the recording has one.
	//
	// Silent on a controller session rather than printing three zeroes, because a line
	// of zeroes on a recording with no mouse in it reads as a failure to see the mouse.
	// On a mouse session the three numbers are the first thing to look at: a press that
	// landed on nothing cannot be learned as anything but a coordinate.
	if r.PointsSeen > 0 {
		fmt.Printf("  pointer presses     %d, %d landed on a proposed region (%.0f%%), "+
			"%d inside more than one\n",
			r.PointsSeen, r.PointsOnAShape, pct(r.PointsOnAShape, r.PointsSeen),
			r.PointsAmbiguous)
	}
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
		// TWO POPULATIONS, AND THE PERCENTAGE IS GONE.
		//
		// WithSelection counts every frame the belief named something on, labelled or
		// not; CorrectSelection counts only labelled frames. A ratio between them is a
		// ratio between different populations, and printing one here was read — by its
		// own author, in a written report — as an accuracy when it is neither an accuracy
		// nor a coverage. Both counts are kept, because the regression floors in
		// corpus_test.go are set on them and a ratchet must not be silently re-based; the
		// percentage that cannot be formed from them is not printed, and the section below
		// prints both rates against their own denominators.
		fmt.Printf("  selection read      %d frame(s) named an entry (of all %d replayed);\n"+
			"                      %d naming(s) on a LABELLED frame were the labelled\n"+
			"                      row, to within half its pitch. Different denominators:\n"+
			"                      see the coverage and accuracy section below\n",
			r.WithSelection, r.Frames, r.CorrectSelection)
		// THE HARD SCORE, and the one a route depends on.
		fmt.Printf("  entry named         %d of those were read in a list with the same %d\n"+
			"                      entries the labels describe; %d named the same entry\n",
			r.SameEntryCount, len(labelRows), r.CorrectOrdinal)
	} else {
		fmt.Printf("  no annotations, so discovery and selection are unscored here\n")
	}

	// THE SPLIT, before anything derived from it.
	reportSelectionScore(r.Score, r.Confirmations)
	// AND WHICH LIST IT WAS READING, which score.go established is the question that
	// decides the rest: the reader is right 93% of the time when the list it is handed
	// contains the row the person is looking at.
	reportListIdentity(r.Lists, r.Identified)

	reportRetained(r)
	// AND THE OTHER HALF OF USING A MENU, and whether any of it would outlive the session.
	//
	// Separate sections because they are separate claims: the mark moved, the screen changed
	// while a row was marked, and a route could be built. A report that printed one number
	// for all three would be the collapse ADR-160 exists to prevent.
	reportActivations(r)
	reportRoute(r)

	fmt.Printf("\n  belief              %s, selected %d, reading %s\n",
		r.Belief.Status, r.Belief.Selected, r.Belief.Reading)
	fmt.Printf("  identified after    %d\n", r.Identified)
	fmt.Printf("  because             %s\n", trim(r.Belief.Because, 150))
	if len(r.Candidates) == 0 {
		fmt.Printf("  candidates          none were tracked\n")
		return
	}
	fmt.Printf("  candidates          %d tracked, %d retired for not being found again\n",
		len(r.Candidates), r.Retired)
	// SHARED IS IN THE TABLE, and it was not. The real menu on this corpus is refused
	// for having too many frames with more than one member marked, and the column that
	// says so was the one column missing - so the refusal read as unexplained.
	// QUALIFIED AND HULL ARE IN THE TABLE TOO. `qual` is the observation this candidate
	// first met every requirement: the tracker publishes a sole qualifier with no
	// comparison, so the order candidates cross the bar in is the decision. `hull` is
	// every extent it has ever had — a hull far larger than `where` is one tracked list
	// that has been several different rectangles, which `where` alone cannot show because
	// it is overwritten every frame.
	fmt.Printf("    %-5s %-6s %-6s %-5s %-5s %-5s %-6s %-5s %-6s %-6s %-7s %-5s %-6s "+
		"%-22s %-14s %s\n",
		"rows", "frames", "judged", "excl", "shar", "quiet", "unread", "moves", "prompt",
		"unprom", "holders", "qual", "rehome", "where y..y (x,w)", "hull y..y", "verdict")
	for _, c := range r.Candidates {
		verdict := "refused"
		if c.Qualifies {
			verdict = "IDENTIFIED"
		}
		where := fmt.Sprintf("%.3f..%.3f (%.3f,%.3f)",
			c.Where.Y, c.Where.Y+c.Where.Height, c.Where.X, c.Where.Width)
		hull := fmt.Sprintf("%.3f..%.3f", c.Hull.Y, c.Hull.Y+c.Hull.Height)
		fmt.Printf("    %-5d %-6d %-6d %-5d %-5d %-5d %-6d %-5d %-6d %-6d %-7d %-5d "+
			"%-6d %-22s %-14s %s\n",
			c.Members, c.Frames, c.Judged, c.Exclusive, c.Shared, c.Quiet,
			c.Unreadable, c.Transfers, c.Prompted, c.Unprompted, c.Holders,
			c.QualifiedAt, c.Rehomed, where, hull, verdict)
		// AND WHY, per candidate. The aggregate below says five candidates fell short
		// of something; it cannot say that the one list a person was actually using
		// fell short of exactly one requirement by eleven frames.
		if len(c.Unmet) > 0 && c.Transfers > 0 {
			var why []string
			for _, u := range c.Unmet {
				why = append(why, fmt.Sprintf("%s %d/%d", u.Requirement, u.Have, u.Want))
			}
			fmt.Printf("          unmet: %s\n", strings.Join(why, "   "))
		}
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

// reportRetained prints what the session KEPT about the mark, and what it refused to keep.
//
// The belief below it says what Marco is looking at right now; this says what it would still
// know tomorrow. They are different questions, and until this change the second had no answer
// at all: a person walking down a menu left behind a transfer count and nothing a route could
// be built from.
// reportActivations says what each screen change was observed being taken from.
//
// The other half of using a menu, and the half a route needs: moving a highlight is not opening
// a menu. Reported as its own section rather than folded into the move table, because the two
// are different claims and a reader must not be able to mistake one for the other.
//
// A zero here on this corpus is EXPECTED and is said out loud, which is the point of printing it
// at all: the pixels discovery path segments no screens, so there is nothing for a selection to
// be recorded against. That is a fact about segmentation, not about the activation evidence.
func reportActivations(r frameResult) {
	var withSelection, unknown, observations int
	for _, tr := range r.Changes {
		observations += tr.Count
		unknown += tr.SelectedUnknown
		withSelection += len(tr.SelectedBefore)
	}
	var acts []observe.Hypothesis
	for _, h := range r.Hypotheses {
		if h.Kind == observe.PossibleEntryActivation {
			acts = append(acts, h)
		}
	}
	fmt.Printf("  activations         %d change(s) over %d observation(s); %d distinct "+
		"selection(s) recorded,\n                      %d observation(s) with nothing "+
		"readable selected; %d hypothes(es)\n",
		len(r.Changes), observations, withSelection, unknown, len(acts))
	if len(r.Changes) == 0 {
		fmt.Printf("                      (no screen changed, so there is nothing a row " +
			"could have been\n                      selected before — see the screen " +
			"model line above)\n")
		return
	}
	for i, tr := range r.Changes {
		if i >= 6 {
			fmt.Printf("    ... and %d more\n", len(r.Changes)-i)
			break
		}
		fmt.Printf("    %-10s -> %-10s %2d seen, selected %v, %d unreadable\n",
			tr.From, tr.To, tr.Count, tr.SelectedBefore, tr.SelectedUnknown)
	}
	for _, h := range acts {
		fmt.Printf("    [%s] %s\n", h.Status, h.Observed)
	}
}

// reportBindings says how much of this session can name a direction, and why the rest cannot.
//
// # Why the four refusals are printed and not netted
//
// Because they call for opposite responses, and the corpus is the place that distinction gets
// measured rather than argued. `unattributed` is the sampling cadence or a quiet player;
// `ambiguous_press` is several directions inside one window; `contradicted` is the evidence
// genuinely disagreeing; `thin_evidence` is a binding on its way to being usable. A single
// "2 of 17 bound" would make all four look like the same shortfall.
func reportBindings(r frameResult) {
	bySupport := map[observe.BindingSupport]int{}
	named, ambiguous, unknown, prompted := 0, 0, 0, 0
	for _, m := range r.Moves {
		bySupport[m.Support()]++
		for _, d := range m.Directions {
			named += d.Count
		}
		ambiguous += m.DirectionAmbiguous
		unknown += m.DirectionUnknown
		prompted += m.Prompted
	}
	fmt.Printf("  directions          %d of %d prompted observation(s) named one "+
		"direction", named, prompted)
	if ambiguous > 0 || unknown > 0 {
		fmt.Printf(" (%d several, %d unrecoverable)", ambiguous, unknown)
	}
	fmt.Printf("\n")
	for _, st := range []observe.BindingSupport{
		observe.BindingRepeated, observe.BindingThin, observe.BindingContradicted,
		observe.BindingAmbiguous, observe.BindingUnattributed,
	} {
		if bySupport[st] > 0 {
			fmt.Printf("    %-16s %d move(s)\n", st, bySupport[st])
		}
	}
	if bySupport[observe.BindingRepeated] == 0 && len(r.Moves) > 0 {
		fmt.Printf("                      (so no chain over this session is one anybody " +
			"could be asked to\n                      take: a route can name the rows " +
			"and not a single control)\n")
	}
}

// reportRoute asks whether any of this would survive the session and be usable.
//
// # Why the replay runs the durable path rather than reporting the session
//
// Because "the graph received the evidence" and "a route could be built from it" are different
// claims, and the second is the one a reusable macro needs. The durable boundary refuses an
// edge whose endpoints are not both recognised — which on this corpus is every edge, and saying
// so with the reason is the whole value of the line.
//
// No store is opened and nothing is written. RelationshipsFrom is pure given a Memory, and the
// one here recognises nothing on purpose: this measures what the SESSION offers the durable
// side, not what a store would do with it.
func reportRoute(r frameResult) {
	obs, report := observe.RelationshipsFrom(
		observe.ShadowTotals{
			Transitions: r.Changes, SelectionTransitions: r.Moves,
		},
		r.Hypotheses, "replay", nothingRecognised{}, observe.Continuity{})
	fmt.Printf("  durable edges       %d offered, %d session-local", len(obs),
		report.SessionLocal)
	if why := report.Why(); why != "no cause recorded" {
		fmt.Printf(" (%s)", why)
	}
	fmt.Printf("\n")
	var withEntry, withMoves, withDirection int
	for _, o := range obs {
		if len(o.Evidence.SelectedBefore) > 0 {
			withEntry++
		}
		if len(o.Evidence.Moves) > 0 {
			withMoves++
		}
		for _, m := range o.Evidence.Moves {
			if m.Support().Usable() {
				withDirection++
				break
			}
		}
	}
	if len(obs) > 0 {
		fmt.Printf("                      %d carry a selected entry, %d carry observed "+
			"moves of the mark,\n                      %d carry a move with a "+
			"direction repeated evidence supports\n",
			withEntry, withMoves, withDirection)
		return
	}
	fmt.Printf("                      (so no plan over this session could name an entry: " +
		"a durable edge\n                      needs both endpoints recognised, and " +
		"nothing here is)\n")
}

// nothingRecognised is a Memory that recognises nothing, which is the honest stand-in.
//
// The replay has no store and must not acquire one — it would be a file handle in a diagnostic.
// Refusing every recall measures exactly what the question is: how much of this session is
// OFFERED to the durable side, before any store decides.
type nothingRecognised struct{}

func (nothingRecognised) Recall(string, observe.StructureSignature) observe.Recollection {
	return observe.Recollection{Verdict: observe.MatchDifferent}
}

func (nothingRecognised) Remember(string, observe.StructureSignature,
	observe.SemanticKnowledge) error {
	return nil
}

func (nothingRecognised) RememberRelationships(string,
	[]observe.RelationshipObservation) (observe.RelationshipUpdate, error) {
	return observe.RelationshipUpdate{}, nil
}

func (nothingRecognised) Topology(string) observe.Topology { return observe.Topology{} }

func (nothingRecognised) RememberLearning(string, observe.RelationshipRef,
	observe.LearningRequest) error {
	return nil
}

func (nothingRecognised) RememberFollowUp(string, observe.RelationshipRef,
	observe.LearningRequest) error {
	return nil
}

func (nothingRecognised) FulfilLearning(string, observe.RelationshipRef, int) error {
	return nil
}

func reportRetained(r frameResult) {
	fmt.Printf("\n  screen model        %d states, %d transitions\n", r.States, r.Transitions)
	if r.States == 0 {
		// EXPECTED on this path, and said rather than left as a zero. Segmentation
		// reads named roles; the structure proposer names nothing. So a move is grounded
		// in the LIST that was independently seen to respond, and the screen it sat on
		// is honestly unknown.
		fmt.Printf("                      (none: segmentation reads named roles and the " +
			"proposer names\n                      nothing, so every move below is " +
			"grounded in its list alone)\n")
	}
	reportLatencies(r.Latencies)
	if len(r.HeldDropped) > 0 {
		keys := make([]string, 0, len(r.HeldDropped))
		var total int
		for k, v := range r.HeldDropped {
			keys = append(keys, k)
			total += v
		}
		sort.Slice(keys, func(i, j int) bool {
			return r.HeldDropped[keys[i]] > r.HeldDropped[keys[j]]
		})
		fmt.Printf("  sightings dropped   %d time(s) the mark's last sighting was let go, "+
			"so\n                      no move could be recorded across it:\n", total)
		for _, k := range keys {
			fmt.Printf("                      %-44s %d\n", k, r.HeldDropped[k])
		}
	}
	fmt.Printf("  moves retained      %d distinct, %d observations",
		len(r.Moves), observationsIn(r.Moves))
	if r.MovesUnplaced > 0 || r.MovesDropped > 0 {
		fmt.Printf("   (%d unplaced, %d dropped)", r.MovesUnplaced, r.MovesDropped)
	}
	fmt.Printf("\n")
	if len(r.Moves) == 0 {
		return
	}
	// HOW LATE THE READER WAS, on the moves themselves and in the session's own terms —
	// no labels involved, which is what makes this the number a LIVE session can report.
	//
	// The ground-truth confirmation lag above measures the same delay against the
	// annotation and is the better measurement; this one exists because a live session has
	// no annotation, and a number that only exists offline cannot tell anybody that a live
	// cadence is costing the attribution.
	var dated, undated int
	var lateMS int64
	for _, m := range r.Moves {
		dated += m.Dated
		undated += m.Count - m.Dated
		lateMS += m.ConfirmLateMS
	}
	if dated > 0 {
		fmt.Printf("    confirmed late    %d of %d observation(s) could say when the "+
			"pixels moved;\n                      mean %dms between the pixels "+
			"moving and the reader agreeing\n",
			dated, dated+undated, lateMS/int64(dated))
	} else {
		fmt.Printf("    confirmed late    none of %d observation(s) could say when the "+
			"pixels moved,\n                      so every one of them is dated from "+
			"its confirmation\n", undated)
	}
	fmt.Printf("    %-10s %-7s %-6s %-7s %-7s %-4s %-7s %s\n",
		"state", "entry", "seen", "prompted", "unprom", "of", "press", "support")
	for i, m := range r.Moves {
		if i >= 12 {
			fmt.Printf("    ... and %d more\n", len(r.Moves)-i)
			break
		}
		state := string(m.State)
		if state == "" {
			state = "unplaced"
		}
		// AND WHICH DIRECTION WAS OBSERVED BEFORE IT, which is the difference between a
		// chain Marco can describe and one anybody could be asked to take.
		press := "-"
		if best, ok := observe.DominantDirection(m.Directions); ok {
			press = fmt.Sprintf("%s:%d", best.Intent, best.Count)
		}
		fmt.Printf("    %-10s %-7s %-6d %-7d %-7d %-4d %-7s %s\n", state,
			fmt.Sprintf("%d->%d", m.From, m.To), m.Count, m.Prompted, m.Unprompted,
			m.Entries, press, m.Support())
	}
	// THE DIRECTION ACCOUNT FOR THE WHOLE SESSION, because the table is bounded and the
	// question "can this corpus name a control at all" is about the total.
	reportBindings(r)
	// AND WHAT THE HYPOTHESIS LAYER MADE OF IT, because the ledger is not the product.
	// Nothing downstream reads the ledger; a route would ask the hypotheses.
	var sel []observe.Hypothesis
	for _, h := range r.Hypotheses {
		if h.Kind == observe.PossibleSelectionTransition {
			sel = append(sel, h)
		}
	}
	fmt.Printf("  hypotheses          %d of %d are selection transitions\n",
		len(sel), len(r.Hypotheses))
	byStatus := map[observe.HypothesisStatus]int{}
	for _, h := range sel {
		byStatus[h.Status]++
	}
	for _, st := range []observe.HypothesisStatus{observe.StatusSupported,
		observe.StatusContested, observe.StatusTentative} {
		if byStatus[st] > 0 {
			fmt.Printf("    %-10s %d\n", st, byStatus[st])
		}
	}
	if len(sel) > 0 {
		fmt.Printf("    first: %s\n", trim(sel[0].Observed, 72))
	}
}

// observationsIn totals how many times the retained moves were seen.
func observationsIn(in []observe.SelectionTransition) int {
	n := 0
	for _, m := range in {
		n += m.Count
	}
	return n
}

// responseLatencies measures how long after a press the mark was first seen somewhere else.
//
// # Why this is measured and not assumed
//
// Because observe.MaxResponseLatency is the one number in the attribution rule that was
// chosen rather than observed: 400ms, as "how long after a press a screen may still be
// repainting it". Everything downstream of it — which list is the active one, and therefore
// every move retained about that list — moves when it moves, and a number nobody checked is
// a number that will eventually be wrong on somebody's application.
//
// # What it measures, exactly
//
// The gap between a directional press and the next look at which the belief reported the mark
// on a DIFFERENT entry of the same list. Not the repaint: the repaint is invisible here, and
// what matters for attribution is when the move became VISIBLE TO MARCO, which includes the
// recorder's own sampling. A press with no move after it inside the search horizon contributes
// nothing, because its absence is not a latency.
//
// The horizon is deliberately far wider than the bound under test. A measurement clipped at
// the value it is supposed to be validating would confirm it by construction.
func responseLatencies(moments []observe.InterfaceMoment, inputs []inputRecord,
	base time.Time) []int64 {

	const horizonMS = 3000
	var presses []int64
	for _, in := range inputs {
		switch observe.NavIntent(in.Intent) {
		case observe.NavUp, observe.NavDown, observe.NavLeft, observe.NavRight:
			presses = append(presses, in.AtMS)
		}
	}
	// Every look at which the mark was seen on a different entry than the time before.
	var sightings []int64
	last := -1
	for _, m := range moments {
		if m.Status != observe.InterfaceProvisional || m.Selected < 0 {
			continue
		}
		if last >= 0 && m.Selected != last {
			sightings = append(sightings, m.At.Sub(base).Milliseconds())
		}
		last = m.Selected
	}
	var out []int64
	for _, seen := range sightings {
		// The NEAREST press before it. A move is attributable to the last thing the
		// person did, not to the first.
		best := int64(-1)
		for _, at := range presses {
			if at <= seen && seen-at <= horizonMS && (best < 0 || at > best) {
				best = at
			}
		}
		if best >= 0 {
			out = append(out, seen-best)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// reportLatencies prints the distribution beside the bound it is a measurement of.
func reportLatencies(in []int64) {
	if len(in) == 0 {
		fmt.Printf("  response latency    no move followed a press closely enough to " +
			"measure\n")
		return
	}
	at := func(pct int) int64 {
		i := (len(in) * pct) / 100
		if i >= len(in) {
			i = len(in) - 1
		}
		return in[i]
	}
	within := 0
	for _, v := range in {
		if time.Duration(v)*time.Millisecond <= observe.MaxResponseLatency {
			within++
		}
	}
	fmt.Printf("  response latency    %d moves measured: min %dms, median %dms, "+
		"p90 %dms, max %dms\n", len(in), in[0], at(50), at(90), in[len(in)-1])
	fmt.Printf("                      %d of %d fall within MaxResponseLatency (%s)\n",
		within, len(in), observe.MaxResponseLatency)
}

// labelledRows is the list the ANNOTATION describes: one normalised centre per distinct
// labelled entry, top to bottom.
//
// # Why this exists
//
// Because "the named row sat at the labelled row's height" turned out to be almost unfalsifiable
// and nobody noticed for two experiments. It was scored against the identified LIST's bounding
// box rather than the named entry's own region, and the list in question spanned three quarters
// of the screen — so the labelled row was inside it whatever the reader said. The measurement
// reported 154 of 155 while the reader was, by the stricter test below, right about a third of
// the time.
//
// So the score is now taken against the thing a route would actually use: the ORDINAL of the
// entry, among the entries the list has. That cannot be satisfied by a loose rectangle, and it
// cannot be satisfied at all by a list with the wrong number of entries — which is the defect
// the loose version was hiding.
//
// Derived from the annotation alone. Nothing here reaches the perception under test.
func (a *annotationFile) labelledRows() []float64 {
	if a == nil {
		return nil
	}
	var ys []float64
	for _, iv := range a.Intervals {
		if iv.State != "selected" {
			continue
		}
		known := false
		for _, y := range ys {
			if math.Abs(y-iv.Y) < 0.01 {
				known = true
				break
			}
		}
		if !known {
			ys = append(ys, iv.Y)
		}
	}
	sort.Float64s(ys)
	return ys
}

// ordinalOf is which entry of the labelled list a labelled height is, or -1.
func ordinalOf(rows []float64, y float64) int {
	for i, at := range rows {
		if math.Abs(at-y) < 0.01 {
			return i
		}
	}
	return -1
}
