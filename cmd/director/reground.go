package main

// Where a control is, when the file cannot say.
//
// # The case this exists for
//
// Four of the nine controls in the measured Rocket League artifact are bands across the whole
// window. They re-acquire perfectly on every look and locate nothing, and `Region.Locates`
// refuses them. No amount of looking repairs that: the demonstration recorded a rectangle that
// says nowhere, and the rectangle is the thing that is wrong.
//
// So the step is unperformable forever, and hand-editing the artifact would fix this workflow
// while teaching Marco nothing. What is wanted instead is for Marco to re-establish the control
// from what is actually on the screen — and to refuse when it cannot.
//
// # What evidence there is, and what there is not
//
// There is plenty of evidence about WHERE things are. The detector boxes six tiles on the
// Training page; OCR reads FREE PLAY, CUSTOM TRAINING, TUTORIAL, AERIAL, STRIKER and GOALIE at
// 95% confidence. Localisation is not the problem.
//
// There is almost no evidence about WHICH ONE WAS MEANT. The artifact calls it `control_3` — a
// generated name — and records no label, no text and no container. The demonstration captured
// where somebody clicked and not what they clicked, so the intent was never saved.
//
// The one human-supplied meaning in the whole artifact is the workflow's NAME: somebody typed
// "enter free play". That is what this matches against, and the match is deliberately strict:
//
//   - every word of a candidate's text must appear in the workflow's name
//   - the candidate must be one of the regions a source actually found
//   - EXACTLY ONE candidate may qualify
//
// Anything else is reported as ambiguous and the step stays refused. A rule that picked the best
// of several would be choosing Marco's own click target on a similarity score, which is the
// thing every boundary in this subsystem exists to prevent.
//
// # And what it is worth
//
// Less than a person pointing at it. The workflow's name agreeing with a label on screen is the
// workflow corroborating itself — the same weakness `Arrival` reports as depth, and it is
// reported here too rather than presented as a finding. It is used for ONE SESSION and written
// nowhere; `--propose` writes a correction somebody else can weigh, beside the artifact and never
// into it.

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"os"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// labelled is one candidate region and whatever can be read inside it.
type labelled struct {
	region observe.Region
	text   string
	from   directorapi.ObservationSource
	score  float64
}

// regrounder re-establishes a control from the live screen, or refuses.
type regrounder struct {
	art   workflowstore.Artifact
	out   io.Writer
	words func(ctx context.Context) ([]readWord, image.Rectangle, bool)
	// candidates is what the sources found on the look being considered, supplied by the
	// loop because this cannot look for itself.
	candidates []observe.Region
	asked      int
}

// readWord is one word OCR read, in window pixels.
type readWord struct {
	Text       string
	X, Y, W, H int
	Conf       float64
}

func (g *regrounder) Reground(ctx context.Context, step int, control string,
	stage workflowstore.StepStage, at workflowrun.Sighting) (observe.Region, string, bool) {

	g.asked++
	// WHICH REFUSAL THIS IS, because they send a reader to different places. A control
	// that locates nothing is a rectangle the demonstration recorded wrong. A control
	// that was not found on any look of the patience window is a control that is no
	// longer where it was — which is the ordinary fate of a menu, not a broken file.
	fmt.Fprintf(g.out, "\n  step %d: %s %s. Looking for it on the screen instead.\n",
		step, control, whyRegrounding(stage))

	// WHAT THIS CONTROL IS RECORDED AS SAYING, and where that came from.
	//
	// The text a demonstration read ON the control is somebody's actual intent and
	// wins whenever there is any. The workflow's NAME is the fallback, and it is the
	// workflow corroborating itself -- which is why the provenance is printed beside
	// the decision rather than left for a reader to assume.
	wanted, from, demonstrated := g.art.ExpectedWords(control)
	if len(wanted) == 0 {
		fmt.Fprintf(g.out, "    %s, so there is nothing here that says which "+
			"control was meant\n", from)
		return observe.Region{}, "", false
	}

	read, bounds, ok := g.words(ctx)
	if !ok {
		fmt.Fprintf(g.out, "    no text could be read from the screen, so nothing "+
			"distinguishes one candidate from another\n")
		return observe.Region{}, "", false
	}

	// TWO WAYS THERE IS NOTHING TO MATCH, AND THEY ARE DIFFERENT FINDINGS.
	//
	// No candidate region that locates anything means the sources offered only bands on
	// this look -- a capture or a source to look at. Candidates with no text in them
	// means the sources found controls and OCR read nothing inside any of them -- a
	// screen to look at. One sentence for both made the first read as the second, and
	// the sentence that was there could only ever be printed for the first.
	cands := g.labelledCandidates(read, bounds)
	if len(cands) == 0 {
		fmt.Fprintf(g.out, "    %d region(s) were offered on this look and none of "+
			"them locates anything, so there is nowhere here to read a label "+
			"inside\n", len(g.candidates))
		return observe.Region{}, "", false
	}
	withText := 0
	for _, c := range cands {
		if c.text != "" {
			withText++
		}
	}
	if withText == 0 {
		fmt.Fprintf(g.out, "    %d candidate region(s) and none of them contains any "+
			"text\n", len(cands))
		return observe.Region{}, "", false
	}

	var matched []labelled
	for _, c := range cands {
		if c.text != "" && observe.NamesControl(observe.Words(c.text), wanted,
			demonstrated) {
			matched = append(matched, c)
		}
	}

	fmt.Fprintf(g.out, "    matching against %s; %d region(s) are candidates, "+
		"%d of them carry text and %d of those are worded in it:\n",
		from, len(cands), withText, len(matched))
	// THE ONES CARRYING TEXT FIRST.
	//
	// The listing is here so somebody can weigh the decision, and on the measured run it
	// printed eight blank rows out of 65 and then "... and 57 more" -- every row of which
	// was a region with nothing written in it. The candidates that bear on the choice are
	// the ones with words in them.
	shown := make([]labelled, len(cands))
	copy(shown, cands)
	sort.SliceStable(shown, func(i, j int) bool {
		return shown[i].text != "" && shown[j].text == ""
	})
	for i, c := range shown {
		if i >= 8 {
			fmt.Fprintf(g.out, "      ... and %d more, %d of them carrying text\n",
				len(shown)-8, max0(withText-8))
			break
		}
		mark := " "
		if c.text != "" && observe.NamesControl(observe.Words(c.text), wanted,
			demonstrated) {
			mark = "*"
		}
		fmt.Fprintf(g.out, "    %s %-28q %s\n", mark, c.text, pxOf(c.region))
	}

	switch len(matched) {
	case 1:
		c := matched[0]
		why := fmt.Sprintf("the only control on this screen whose text (%q) is "+
			"made of %s", c.text, from)
		fmt.Fprintf(g.out, "    re-grounded: %s\n", why)
		fmt.Fprintf(g.out, "    this is the workflow agreeing with itself, which is "+
			"weaker than somebody pointing at it\n")
		return c.region, why, true
	case 0:
		fmt.Fprintf(g.out, "    nothing on this screen is named in %s, so which "+
			"control was meant is not recoverable from what was saved\n",
			from)
	default:
		fmt.Fprintf(g.out, "    %d candidates match equally and nothing here chooses "+
			"between them\n", len(matched))
	}
	return observe.Region{}, "", false
}

// labelledCandidates pairs each candidate region with the words read inside it.
//
// The rule itself lives in `observe.NameCandidates`, because readiness asks the same
// question of the same candidates one step earlier -- whether the region a geometric match
// landed on is the control it was matched to -- and two implementations of "which candidate
// said this" would eventually disagree. The shape of that disagreement is a repair that
// nominates a region readiness would then refuse. All that is left here is the conversion
// from the pixels OCR answers in to the normalised window this subsystem speaks.
func (g *regrounder) labelledCandidates(read []readWord, bounds image.Rectangle) []labelled {
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	if w <= 0 || h <= 0 {
		return nil
	}
	placed := make([]observe.ReadWord, 0, len(read))
	for _, word := range read {
		placed = append(placed, observe.ReadWord{Text: word.Text, Where: observe.Region{
			X: float64(word.X) / w, Y: float64(word.Y) / h,
			Width: float64(word.W) / w, Height: float64(word.H) / h,
		}})
	}
	named := observe.NameCandidates(g.candidates, placed)
	out := make([]labelled, 0, len(named))
	for _, n := range named {
		out = append(out, labelled{region: n.Region, text: n.Text,
			from: directorapi.SourceVision})
	}
	return out
}

// whyRegrounding is the half-sentence that says what the file could not do.
//
// Three stages reach here and they are not the same finding. `StepControlUnlocated` is a
// recorded rectangle that names nowhere. `StepControlUnresolved` is a control looked for on
// every observation of the patience window and found on none of them, which is what a menu
// that has moved looks like. `StepControlContradicted` is a control that WAS matched, by
// position alone, onto a region the words on this look say is something else.
//
// One sentence for all three would read as the first, and send whoever reads it to the
// artifact to look at a rectangle that is perfectly well-formed.
func whyRegrounding(stage workflowstore.StepStage) string {
	switch stage {
	case workflowstore.StepControlUnresolved:
		return "was not found on any look while the run waited for it"
	case workflowstore.StepControlContradicted:
		return "was matched by position onto something that is not it"
	default:
		return "locates nothing"
	}
}

// max0 keeps a count from reading as a negative number of remaining candidates.
func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func pxOf(r observe.Region) string {
	return fmt.Sprintf("x %4d..%4d  y %4d..%4d", int(r.X*1920), int((r.X+r.Width)*1920),
		int(r.Y*1080), int((r.Y+r.Height)*1080))
}

// ── the proposed correction ───────────────────────────────────────────────────

// proposal is a correction somebody else can weigh, with where it came from.
type proposal struct {
	Workflow string         `json:"workflow"`
	At       string         `json:"proposed_at"`
	By       string         `json:"by"`
	Note     string         `json:"note"`
	Repairs  []proposedFix  `json:"repairs"`
	Original []savedReading `json:"original_readings"`
}

type proposedFix struct {
	Step    int            `json:"step"`
	Control string         `json:"control"`
	Where   observe.Region `json:"where"`
	Because string         `json:"because"`
	// Changed says the screen differed after the action and Arrived says the workflow's
	// expected destination was recognised. TWO FIELDS, because a repair that produced a
	// transition nobody recognised is a good region and a recognition problem, and one
	// that produced nothing is a bad region. Collapsing them would file the first as a
	// failure of the thing that worked.
	Changed bool `json:"the_screen_changed"`
	Arrived bool `json:"the_expected_destination_was_recognised"`
}

type savedReading struct {
	Control string           `json:"control"`
	Reading []observe.Region `json:"readings"`
}

// writeProposal records a repair beside the artifact, never into it.
//
// A workflow corrected in place by the run that used it would make the next run's evidence
// depend on this one's guess, with the provenance gone by the time anybody looked. So this is a
// separate file that says what was changed, on what evidence, and whether the action it enabled
// actually arrived where the workflow said it would.
func writeProposal(path string, art workflowstore.Artifact, repairs []workflowrun.Repair,
	reached map[int]bool, changed bool) error {

	if len(repairs) == 0 {
		return nil
	}
	p := proposal{
		Workflow: art.Name,
		At:       time.Now().UTC().Format(time.RFC3339),
		By:       "re-grounded live from the screen, during a bounded exploration",
		Note: "A PROPOSAL, not a correction. The region was established for one " +
			"session from text agreeing with the workflow's own name, which is " +
			"the workflow corroborating itself and is weaker than somebody " +
			"pointing at the control. Nothing has been written to the artifact.",
	}
	for _, r := range repairs {
		p.Repairs = append(p.Repairs, proposedFix{
			Step: r.Step, Control: r.Control, Where: r.Where,
			Because: r.Because,
			Changed: changed, Arrived: reached[r.Step],
		})
		for _, c := range art.Controls {
			if c.Label == r.Control {
				p.Original = append(p.Original, savedReading{
					Control: c.Label, Reading: c.Readings,
				})
			}
		}
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// regroundOrNil hands the coordinator a regrounder only when there is one.
//
// A typed nil in an interface is not nil, so returning the pointer directly would install a
// regrounder that panics the first time a control locates nothing — which is exactly the case it
// would be installed for.
func regroundOrNil(g *regrounder) workflowrun.Regrounder {
	if g == nil {
		return nil
	}
	return g
}
