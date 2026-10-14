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
	"strings"
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
	at workflowrun.Sighting) (observe.Region, string, bool) {

	g.asked++
	fmt.Fprintf(g.out, "\n  step %d: %s locates nothing. Looking for it on the screen "+
		"instead.\n", step, control)

	// THE WORDS THE PERSON USED FOR THIS TASK. The only human-supplied meaning in the
	// artifact; everything else in it is a generated name.
	wanted := words(g.art.Name)
	if len(wanted) == 0 {
		fmt.Fprintf(g.out, "    the workflow has no name to match against, so there "+
			"is nothing here that says which control was meant\n")
		return observe.Region{}, "", false
	}

	read, bounds, ok := g.words(ctx)
	if !ok {
		fmt.Fprintf(g.out, "    no text could be read from the screen, so nothing "+
			"distinguishes one candidate from another\n")
		return observe.Region{}, "", false
	}

	cands := g.labelledCandidates(read, bounds)
	if len(cands) == 0 {
		fmt.Fprintf(g.out, "    %d candidate region(s) and none of them contains any "+
			"text\n", len(g.candidates))
		return observe.Region{}, "", false
	}

	var matched []labelled
	for _, c := range cands {
		if c.text != "" && covered(words(c.text), wanted) {
			matched = append(matched, c)
		}
	}

	fmt.Fprintf(g.out, "    the workflow is called %q; %d candidate(s) carry text, "+
		"%d of them words it uses:\n", g.art.Name, len(cands), len(matched))
	for i, c := range cands {
		if i >= 8 {
			fmt.Fprintf(g.out, "      ... and %d more\n", len(cands)-8)
			break
		}
		mark := " "
		if c.text != "" && covered(words(c.text), wanted) {
			mark = "*"
		}
		fmt.Fprintf(g.out, "    %s %-28q %s\n", mark, c.text, pxOf(c.region))
	}

	switch len(matched) {
	case 1:
		c := matched[0]
		why := fmt.Sprintf("the only control on this screen whose text (%q) is made "+
			"of words from the workflow's own name (%q)", c.text, g.art.Name)
		fmt.Fprintf(g.out, "    re-grounded: %s\n", why)
		fmt.Fprintf(g.out, "    this is the workflow agreeing with itself, which is "+
			"weaker than somebody pointing at it\n")
		return c.region, why, true
	case 0:
		fmt.Fprintf(g.out, "    nothing on this screen is named in the workflow's "+
			"own name, so which control was meant is not recoverable from "+
			"what was saved\n")
	default:
		fmt.Fprintf(g.out, "    %d candidates match equally and nothing here chooses "+
			"between them\n", len(matched))
	}
	return observe.Region{}, "", false
}

// labelledCandidates pairs each candidate region with the words read inside it.
//
// A word belongs to a candidate when its box is inside that candidate's. Smallest containing
// candidate wins, so a tile and the panel behind it do not both claim the same label.
func (g *regrounder) labelledCandidates(read []readWord, bounds image.Rectangle) []labelled {
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	if w <= 0 || h <= 0 {
		return nil
	}
	out := make([]labelled, 0, len(g.candidates))
	for _, r := range g.candidates {
		if ok, _ := r.Locates(); !ok {
			continue // a band is not a candidate, whatever is written on it
		}
		out = append(out, labelled{region: r, from: directorapi.SourceVision})
	}
	// Smallest first, so the tightest region that contains a word takes it.
	sort.Slice(out, func(i, j int) bool { return out[i].region.Area() < out[j].region.Area() })
	for _, word := range read {
		wr := observe.Region{
			X: float64(word.X) / w, Y: float64(word.Y) / h,
			Width: float64(word.W) / w, Height: float64(word.H) / h,
		}
		for i := range out {
			if inside(wr, out[i].region) {
				out[i].text = strings.TrimSpace(out[i].text + " " + word.Text)
				break
			}
		}
	}
	return out
}

// inside reports whether all of `a` lies within `b`.
func inside(a, b observe.Region) bool {
	return a.X >= b.X && a.Y >= b.Y &&
		a.X+a.Width <= b.X+b.Width && a.Y+a.Height <= b.Y+b.Height
}

// words splits a phrase into lowercase words, dropping anything that is not a letter or digit.
func words(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if len(f) > 1 { // a stray letter from OCR is not a word
			out = append(out, f)
		}
	}
	return out
}

// covered reports whether every word of `some` appears in `all`.
//
// Deliberately one-directional and whole-word. "FREE PLAY" is covered by "enter free play";
// "CUSTOM TRAINING" is not. A substring rule would make "PLAY" match "PLAYLIST", and a
// bidirectional one would make any short label match any long name.
func covered(some, all []string) bool {
	if len(some) == 0 {
		return false
	}
	has := map[string]bool{}
	for _, a := range all {
		has[a] = true
	}
	for _, s := range some {
		if !has[s] {
			return false
		}
	}
	return true
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
