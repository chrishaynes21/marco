package main

// How many times would a person have to answer "yes, that's the screen"?
//
// # What is being counted, and what the oracle is standing in for
//
// A fresh observer walks fold B. Every time the observation generation moves, the saved workflow
// either knows where it is or does not:
//
//   - it can INFER, because it was established on a screen the workflow leaves for this one and
//     the arrangement agrees; or
//   - somebody has to be ASKED.
//
// The number reported is how many questions that comes to, and the split between confirmed and
// inferred bindings.
//
// The oracle answers the questions, and it is **not a simulated person**. It answers one
// narrow thing — whether the proposed saved screen really is the counterpart of the screen
// standing now — perfectly and instantly, which no person does. It cannot say whether the
// question is answerable by somebody looking at a monitor, whether the prompt is intelligible,
// or how often a person would get it wrong. It establishes the COUNT, under the assumption that
// every answer is right, and that is a lower bound on the questions and an upper bound on the
// accuracy.

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// anchorEvent is one moment the workflow had to establish where it was.
type anchorEvent struct {
	look  int
	epoch uint64
	// screen is the screen standing now, and truth the oracle's name for it.
	screen observe.VisualScreenID
	truth  observe.VisualScreenID
	// label is the saved screen this resolved to, empty when nothing did.
	label  string
	ground workflowstore.Ground
	// asked says a question was put; answered says it was yes.
	asked, answered bool
	depth           int
	because         string
}

func reportAnchoring(dir string, limit int, onnx string) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}
	if len(frames) < 20 {
		return fmt.Errorf("%s carries %d frames, too few to split", dir, len(frames))
	}
	inputs, _ := readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })

	regions, readable := proposeAll(dir, frames)
	newFeed := func() *slowFeed {
		if onnx == "" {
			return nil
		}
		fd, ferr := loadSlowFeed(onnx, 8)
		if ferr != nil {
			err = ferr
			return nil
		}
		return fd
	}

	// WHAT THE RECORDING WAS READ TO SAY, read once for every fold below it.
	read := wordsBeside(dir, frames)
	half := len(frames) / 2
	a, aerr := foldAndLearn(frames, inputs, regions, readable, read, newFeed(), 0, half)
	if aerr != nil {
		return aerr
	}
	defer a.obs.Close()
	oracle, oerr := foldRange(frames, regions, readable, nil, nil, 0, len(frames))
	if oerr != nil {
		return oerr
	}
	if err != nil {
		return err
	}

	// THE EVIDENCE FOR EACH SCREEN THE DRAFT NAMES, both halves of it.
	//
	// Through SaveWith rather than Save, so a saved destination carries what it was
	// READ to say and not only where its rectangles were. An artifact without that
	// cannot have an arrival corroborated by a reading, and the arrangement alone was
	// measured to arrive on the wrong page — see
	// [[Experiment-048-the-destination-a-workflow-can-see-for-itself]]. A screen
	// nobody read is still saved, as unread rather than as silent.
	arrangements := map[observe.VisualScreenID]observe.VisualArrangement{}
	for id, ar := range a.arr {
		arrangements[id] = ar
	}
	said := map[observe.VisualScreenID]observe.ScreenText{}
	for id, t := range a.said {
		said[id] = t
	}
	art, serr := workflowstore.SaveWith(a.draft, arrangements, said, time.Unix(0, 0).UTC())
	if serr != nil {
		return serr
	}
	body, merr := art.Marshal()
	if merr != nil {
		return merr
	}
	loaded, uerr := workflowstore.Unmarshal(body)
	if uerr != nil {
		return uerr
	}

	// THE ORACLE'S VIEW OF FOLD A, so a question can be answered: which saved label
	// corresponds to which oracle screen.
	labels := workflowstore.Labels(a.draft)
	truthOf := map[string]observe.VisualScreenID{}
	for _, s := range summarise(a.fold, oracle, regions) {
		if l, ok := labels[s.id]; ok {
			truthOf[l] = s.truth
		}
	}

	// ── FOLD B, DRIVEN LOOK BY LOOK ──
	session := workflowstore.NewSessionBinding()
	obs, berr := fastobserve.New(100*time.Millisecond, 64)
	if berr != nil {
		return berr
	}
	go func() {
		for range obs.Updates() {
		}
	}()
	defer obs.Close()
	feed := newFeed()

	var events []anchorEvent
	// EVERY LOOK, so a gap between two establishments can be reconstructed
	// afterwards rather than guessed at. Kept for the whole walk because the
	// interesting question is what happened on the looks nobody asked about.
	var timeline []walkLook
	// arrOf remembers each screen's arrangement as this fold settled it, so the
	// returning observation can be compared with the earlier one WITHOUT going
	// through the person's answer.
	arrOf := map[observe.VisualScreenID]observe.VisualArrangement{}
	base := time.Unix(1700000000, 0).UTC()
	lastEpoch := ^uint64(0)
	var lastLabel string
	for i := half; i < len(frames); i++ {
		fr := frames[i]
		lk := fastobserve.Look{
			At: base.Add(time.Duration(fr.AtMS) * time.Millisecond),
			Window: fastobserve.WindowRef{
				ID: "hwnd:recorded", Generation: fr.WindowGeneration,
				Bounds: directorapi.Rect{Width: fr.Width, Height: fr.Height},
			},
			Front: true, Regions: regions[i],
		}
		// MISSING MEANS NO PIXELS, not no proposals. A black frame is a
		// look that saw nothing, not one nobody could take.
		if !readable[i] {
			lk.Missing = true
		}
		lk.Slow = feed.due(i + 1)
		up := obs.Observe(lk)
		if up.Settled {
			feed.request(i+1, fr.WindowGeneration, up.Epoch)
		}
		timeline = append(timeline, walkLook{
			look: i + 1, atMS: fr.AtMS, epoch: up.Epoch,
			returned: up.Screen, standing: obs.Screen(),
			generation: fr.WindowGeneration, missing: lk.Missing,
			front: lk.Front, proposals: len(regions[i]),
		})
		if sc := obs.Screen(); sc != "" {
			if a := obs.Arrangement(sc); a.Observed() {
				arrOf[sc] = a
			}
		}
		if up.Epoch == lastEpoch {
			continue
		}
		lastEpoch = up.Epoch

		standing := obs.Screen()
		if standing == "" || standing == observe.VisualScreenUnknown ||
			standing == observe.VisualScreenUnobserved {
			session.Observed(up.Epoch, fr.WindowGeneration)
			lastLabel = ""
			continue
		}
		here := obs.Arrangement(standing)
		truth := oracleName(oracle, i)
		ev := anchorEvent{look: i + 1, epoch: up.Epoch, screen: standing, truth: truth}

		// ── FIRST, CAN IT BE INFERRED? ──
		//
		// Before the stale bindings are dropped, because the anchor is by definition
		// the binding that held in the observation we just left.
		inferred := false
		if lastLabel != "" {
			for _, to := range nextLabels(loaded, lastLabel) {
				agreed := observe.ArrangementSimilarity(
					arrangementOf(loaded, to), here)
				b, ierr := session.Infer(loaded, lastLabel, to, standing,
					agreed, up.Epoch, fr.WindowGeneration)
				if ierr == nil {
					ev.label, ev.ground, ev.depth = to, b.Ground, b.Depth
					ev.because = b.Because
					inferred = true
					break
				}
			}
		}

		// ── OTHERWISE, ASK ──
		if !inferred {
			ev.asked = true
			proposed := bestLabel(loaded, here)
			if proposed == "" {
				ev.because = "nothing saved resembles this screen, so there is " +
					"nothing to propose"
			} else {
				// THE ORACLE ANSWERS. Not a person: it is right every time
				// and instantly, which bounds the count and proves nothing
				// about whether a person could answer it.
				yes := truthOf[proposed] == truth && truth != ""
				if yes {
					if cerr := session.Confirm(proposed, standing, up.Epoch,
						fr.WindowGeneration, "the oracle, standing in",
						base); cerr == nil {
						ev.label, ev.ground = proposed, workflowstore.GroundConfirmed
						ev.answered = true
					}
				} else {
					_ = session.Reject(proposed, standing, "the oracle, standing in")
					ev.because = fmt.Sprintf("proposed %s; it is not this screen, "+
						"and the runner-up is not promoted", proposed)
				}
			}
		}
		session.Observed(up.Epoch, fr.WindowGeneration)
		lastLabel = ev.label
		events = append(events, ev)
	}

	printAnchoring(dir, loaded, half, len(frames), events, session)

	// ── WHY EACH REPEATED QUESTION WAS ASKED ──
	//
	// Only the questions that repeated a screen established earlier in this walk.
	// A first question about a screen is the honest price of not having been
	// there before; a second is the thing worth explaining.
	var gaps []gapReport
	lastAt := map[string]int{}
	lastScreen := map[string]observe.VisualScreenID{}
	for _, e := range events {
		if e.label == "" {
			continue
		}
		if prev, ok := lastAt[e.label]; ok && e.asked {
			gaps = append(gaps, classifyGap(e.label, timeline, prev, e.look,
				lastScreen[e.label], arrOf[lastScreen[e.label]],
				arrOf[e.screen]))
		}
		lastAt[e.label], lastScreen[e.label] = e.look, e.screen
	}
	printContinuity(gaps, timeline)
	return nil
}

// oracleName is the oracle's screen at one look.
func oracleName(o fold, look int) observe.VisualScreenID { return o.at[look] }

// nextLabels is every saved screen the workflow leaves `from` for.
func nextLabels(a workflowstore.Artifact, from string) []string {
	var out []string
	seen := map[string]bool{}
	for _, st := range a.Steps {
		if st.From == from && st.To != "" && !seen[st.To] {
			seen[st.To] = true
			out = append(out, st.To)
		}
	}
	return out
}

// arrangementOf is a saved screen's stored arrangement.
func arrangementOf(a workflowstore.Artifact, label string) observe.VisualArrangement {
	for _, s := range a.Screens {
		if s.Label == label {
			return s.Arrangement
		}
	}
	return observe.VisualArrangement{}
}

// bestLabel is the saved screen most like the one standing now — a PROPOSAL, which is all
// similarity is allowed to produce.
func bestLabel(a workflowstore.Artifact, here observe.VisualArrangement) string {
	best, bestSim := "", 0.0
	for _, s := range a.Screens {
		if sim := observe.ArrangementSimilarity(s.Arrangement, here); sim > bestSim {
			best, bestSim = s.Label, sim
		}
	}
	if bestSim < observe.VisualScreenMatch {
		return ""
	}
	return best
}
