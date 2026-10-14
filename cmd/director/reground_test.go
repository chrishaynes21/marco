package main

// Which control was meant, from the one human-supplied meaning in the artifact.
//
// The artifact calls it `control_3`. That is a generated name: the demonstration recorded where
// somebody clicked and not what they clicked, so the intent was never saved. The only words a
// person actually typed are the workflow's NAME -- "enter free play" -- and that is what this
// matches against.
//
// So the rule is strict and the strictness is the whole value of it. Every word of a candidate's
// text must appear in the workflow's name, whole words, one direction, and exactly one candidate
// may qualify. A rule that picked the best of several would be choosing Marco's own click target
// on a similarity score, which is the thing every boundary in this subsystem exists to prevent.
//
// These hold the rule, and they hold it by trying to break it the ways it would actually break:
// a label that is a prefix of a word in the name, a label that is longer than the name, two
// labels that qualify equally, and a band with the right words written across it.

import (
	"context"
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// ── the word rule ─────────────────────────────────────────────────────────────

// EVERY WORD OF THE LABEL MUST BE IN THE NAME, WHOLE, AND ONLY THAT WAY ROUND.
//
// The measured case is the first row: "FREE PLAY" against "enter free play". Everything after it
// is a way the rule would go wrong if it were written the obvious way instead.
func TestOnlyALabelWhollyNamedInTheWorkflowsNameQualifies(t *testing.T) {
	const name = "enter free play"
	for _, tc := range []struct {
		label string
		want  bool
		why   string
	}{
		{"FREE PLAY", true, "the measured case"},
		{"free play", true, "case is not meaning"},
		{"PLAY", true, "one word of the name is still words of the name"},
		{"CUSTOM TRAINING", false, "neither word is in the name"},
		{"CREATE FREE PLAY", false, "one word is not, so the label is not covered"},
		{"TUTORIAL", false, "nothing to do with the name"},

		// A SUBSTRING RULE WOULD TAKE THESE. "PLAY" is a prefix of "PLAYLIST" and a
		// substring of "REPLAY", so a rule that asked whether the name contained the
		// label's letters would ground the wrong control on a menu that has both.
		{"PLAYLIST", false, "a word the name's word is a prefix of"},
		{"REPLAY", false, "a word containing the name's word"},
		{"FREEPLAY", false, "the two words run together are a third word"},

		// A BIDIRECTIONAL RULE WOULD TAKE THIS. "enter" is in the name, so asking
		// "do these share a word" rather than "is the label covered" would let any
		// label that happens to contain a common word qualify.
		{"ENTER TOURNAMENT", false, "shares a word and is not covered by the name"},

		{"", false, "no text is not a match"},
		{"123", false, "digits nobody typed"},
	} {
		t.Run(tc.label+"/"+tc.why, func(t *testing.T) {
			if got := covered(words(tc.label), words(name)); got != tc.want {
				t.Errorf("%q against %q: %v, want %v (%s)",
					tc.label, name, got, tc.want, tc.why)
			}
		})
	}
}

// AND A WORKFLOW WITH NO NAME MATCHES NOTHING.
//
// Rather than matching everything, which is what an empty "all words must appear in this" set
// does if nobody checks. A nameless workflow carries no human-supplied meaning at all and the
// honest answer is that the intent is not recoverable.
func TestANamelessWorkflowGroundsNothing(t *testing.T) {
	if words("") != nil {
		t.Errorf("an empty name yields %v", words(""))
	}
	if covered(words("FREE PLAY"), words("")) {
		t.Error("a label qualified against a workflow with no name")
	}
	if covered(nil, words("enter free play")) {
		t.Error("a candidate with no text qualified")
	}
}

// AND A SINGLE STRAY LETTER IS NOT A WORD.
//
// OCR produces them constantly -- an edge read as "I", a bullet as "l" -- and a one-letter
// token would match almost any name by accident.
func TestAStrayLetterFromOCRIsNotAWord(t *testing.T) {
	if got := words("I l FREE PLAY x"); strings.Join(got, ",") != "free,play" {
		t.Errorf("words are %v, want free and play alone", got)
	}
}

// ── which candidate the words belong to ───────────────────────────────────────

func atPx(x, y, w, h int) observe.Region {
	return observe.Region{X: float64(x) / 1920, Y: float64(y) / 1080,
		Width: float64(w) / 1920, Height: float64(h) / 1080}
}

func word(text string, x, y, w, h int) readWord {
	return readWord{Text: text, X: x, Y: y, W: w, H: h, Conf: 95}
}

const tallPx = 1080

// A BAND IS NOT A CANDIDATE, WHATEVER IS WRITTEN ACROSS IT.
//
// The whole window contains every word on the screen, so a window-spanning candidate reads as
// "FREE PLAY" and everything else at once. Admitting it would hand the repair the exact region
// the repair exists to replace.
func TestABandIsNotACandidateHoweverWellItsTextMatches(t *testing.T) {
	g := &regrounder{
		art:        workflowstore.Artifact{Name: "enter free play"},
		out:        &strings.Builder{},
		candidates: []observe.Region{{X: 0, Y: 0, Width: 1, Height: 0.957}},
	}
	got := g.labelledCandidates([]readWord{word("FREE", 340, 466, 70, 34),
		word("PLAY", 420, 466, 62, 34)}, image.Rect(0, 0, 1920, tallPx))
	if len(got) != 0 {
		t.Fatalf("%d candidate(s) from one band: %+v", len(got), got)
	}
}

// AND THE SMALLEST REGION THAT CONTAINS A WORD TAKES IT.
//
// A tile and the panel behind it both contain the label. If both claimed it, the panel would
// qualify on the tile's text and the press would land in the middle of the panel.
func TestTheSmallestContainingCandidateTakesTheWord(t *testing.T) {
	tile := atPx(399, 529, 355, 223)
	panel := atPx(100, 400, 1200, 500)
	g := &regrounder{
		art:        workflowstore.Artifact{Name: "enter free play"},
		out:        &strings.Builder{},
		candidates: []observe.Region{panel, tile},
	}
	got := g.labelledCandidates([]readWord{word("FREE", 440, 600, 70, 34),
		word("PLAY", 520, 600, 62, 34)}, image.Rect(0, 0, 1920, tallPx))
	if len(got) != 2 {
		t.Fatalf("%d candidate(s) for two regions", len(got))
	}
	for _, c := range got {
		inTile := c.region == tile
		switch {
		case inTile && c.text != "FREE PLAY":
			t.Errorf("the tile reads %q", c.text)
		case !inTile && c.text != "":
			t.Errorf("the panel behind the tile also claims %q", c.text)
		}
	}
}

// AND A WORD OUTSIDE EVERY CANDIDATE BELONGS TO NONE OF THEM.
func TestAWordOutsideEveryCandidateIsNotAttributedToOne(t *testing.T) {
	g := &regrounder{
		art:        workflowstore.Artifact{Name: "enter free play"},
		out:        &strings.Builder{},
		candidates: []observe.Region{atPx(399, 529, 355, 223)},
	}
	got := g.labelledCandidates([]readWord{word("FREE", 1700, 80, 70, 34)},
		image.Rect(0, 0, 1920, tallPx))
	if len(got) != 1 {
		t.Fatalf("%d candidate(s)", len(got))
	}
	if got[0].text != "" {
		t.Errorf("a candidate claims %q from a word outside it", got[0].text)
	}
}

// AND A WORD THAT STRADDLES A CANDIDATE'S EDGE IS NOT INSIDE IT.
//
// All of the word, not the start of it. A label half outside a tile belongs to whatever
// contains the whole of it, and attributing it to the tile would ground a control on text
// that is mostly somewhere else.
func TestAWordStraddlingACandidatesEdgeIsNotInsideIt(t *testing.T) {
	tile := atPx(399, 529, 355, 223)
	g := &regrounder{
		art:        workflowstore.Artifact{Name: "enter free play"},
		out:        &strings.Builder{},
		candidates: []observe.Region{tile},
	}
	// Starts inside the tile at x 700 and runs past its right edge at 754.
	got := g.labelledCandidates([]readWord{word("PLAY", 700, 600, 120, 34)},
		image.Rect(0, 0, 1920, tallPx))
	if len(got) != 1 {
		t.Fatalf("%d candidate(s)", len(got))
	}
	if got[0].text != "" {
		t.Errorf("the tile claims %q from a word that runs past its edge",
			got[0].text)
	}
}

// AND A LOOK THAT OFFERED ONLY BANDS SAYS THAT, not that nothing was written anywhere.
//
// Different findings: a source offering only window-spanning regions is a capture or a
// proposer to look at, and a screen of unlabelled tiles is a workflow to re-record. One
// sentence for both made the first read as the second.
func TestALookThatOfferedOnlyBandsIsReportedAsThat(t *testing.T) {
	g, out := regrounderOver("enter free play",
		[]observe.Region{{X: 0, Y: 0, Width: 1, Height: 0.957},
			{X: 0, Y: 0.468, Width: 1, Height: 0.461}},
		[]readWord{word("FREE", 340, 466, 70, 34), word("PLAY", 420, 466, 62, 34)})

	if _, _, ok := g.Reground(context.Background(), 2, "control_3",
		workflowrun.Sighting{}); ok {
		t.Fatal("a look offering only bands grounded a control")
	}
	if !strings.Contains(out.String(), "none of them locates anything") {
		t.Errorf("the refusal does not say the regions were bands:\n%s",
			out.String())
	}
	// AND IT IS NOT THE OTHER REFUSAL.
	if strings.Contains(out.String(), "contains any text") {
		t.Errorf("a look offering only bands is reported as a screen with no "+
			"text on it:\n%s", out.String())
	}
}

// ── exactly one, or nothing ───────────────────────────────────────────────────

// a fixed set of words, so these tests are about the rule and not about OCR.
func regrounderOver(name string, cands []observe.Region, read []readWord) (
	*regrounder, *strings.Builder) {

	out := &strings.Builder{}
	return &regrounder{
		art:        workflowstore.Artifact{Name: name},
		out:        out,
		candidates: cands,
		words: func(ctx context.Context) ([]readWord, image.Rectangle, bool) {
			return read, image.Rect(0, 0, 1920, tallPx), len(read) > 0
		},
	}, out
}

// EXACTLY ONE CANDIDATE QUALIFIES, AND THAT ONE IS THE ANSWER.
//
// The measured run: 65 candidates carried text and one of them was worded in the task's name.
func TestExactlyOneQualifyingCandidateIsTheAnswer(t *testing.T) {
	tile := atPx(399, 529, 355, 223)
	other := atPx(799, 529, 355, 223)
	g, out := regrounderOver("enter free play",
		[]observe.Region{tile, other},
		[]readWord{
			word("FREE", 440, 600, 70, 34), word("PLAY", 520, 600, 62, 34),
			word("CUSTOM", 840, 600, 90, 34), word("TRAINING", 940, 600, 110, 34),
		})

	where, why, ok := g.Reground(context.Background(), 2, "control_3",
		workflowrun.Sighting{})
	if !ok {
		t.Fatalf("refused: %s", out.String())
	}
	if where != tile {
		t.Errorf("grounded %+v, want the tile reading FREE PLAY %+v", where, tile)
	}
	// THE EVIDENCE NAMES BOTH SIDES OF IT, or a reviewer cannot weigh it.
	for _, w := range []string{"FREE PLAY", "enter free play"} {
		if !strings.Contains(why, w) {
			t.Errorf("the evidence does not name %q: %s", w, why)
		}
	}
	// AND IT SAYS WHAT IT IS WORTH, in both halves of the sentence that says it.
	//
	// The bound on the claim is the second half: the workflow agreeing with itself is
	// WEAKER THAN SOMEBODY POINTING AT IT. A test that checked only "agreeing with
	// itself" would pass with the comparison dropped, and a repair presented without its
	// limit reads as a finding.
	for _, w := range []string{
		"agreeing with itself",
		"weaker than somebody pointing at it",
	} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("the report does not say %q:\n%s", w, out.String())
		}
	}
}

// TWO THAT QUALIFY EQUALLY IS A REFUSAL, however much better one of them looks.
//
// On a screen where two labels share words with the task name there is nothing here that chooses
// between them -- and a tie-break on size, score or position would be Marco picking its own
// target. The same rule as every other ambiguity in this subsystem.
func TestTwoCandidatesThatQualifyEquallyAreRefused(t *testing.T) {
	big := atPx(399, 529, 500, 300)
	small := atPx(999, 529, 120, 80)
	g, out := regrounderOver("enter free play",
		[]observe.Region{big, small},
		[]readWord{
			word("FREE", 440, 600, 70, 34), word("PLAY", 520, 600, 62, 34),
			word("PLAY", 1020, 560, 62, 34),
		})

	where, why, ok := g.Reground(context.Background(), 2, "control_3",
		workflowrun.Sighting{})
	if ok {
		t.Fatalf("two qualifying candidates grounded %+v: %s", where, why)
	}
	if !strings.Contains(out.String(), "nothing here chooses between them") {
		t.Errorf("the refusal does not say why:\n%s", out.String())
	}
}

// AND NOTHING THAT QUALIFIES IS A REFUSAL THAT SAYS THE INTENT IS NOT RECOVERABLE.
func TestNoQualifyingCandidateIsRefusedAsAnIntentNeverSaved(t *testing.T) {
	g, out := regrounderOver("enter free play",
		[]observe.Region{atPx(399, 529, 355, 223)},
		[]readWord{word("CUSTOM", 440, 600, 90, 34),
			word("TRAINING", 540, 600, 110, 34)})

	if _, _, ok := g.Reground(context.Background(), 2, "control_3",
		workflowrun.Sighting{}); ok {
		t.Fatal("a screen with nothing named in the workflow's name was grounded")
	}
	if !strings.Contains(out.String(), "not recoverable") {
		t.Errorf("the refusal does not say the intent was never saved:\n%s",
			out.String())
	}
}

// AND NO TEXT AT ALL IS ITS OWN REFUSAL, not a silent one.
//
// OCR unavailable and a screen of unlabelled tiles are different situations and both are
// reported, because the first is a machine to fix and the second is a workflow to re-record.
func TestEachWayTheWordsCanBeMissingIsReportedSeparately(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func() (*regrounder, *strings.Builder)
		say   string
	}{
		{"no text could be read", func() (*regrounder, *strings.Builder) {
			return regrounderOver("enter free play",
				[]observe.Region{atPx(399, 529, 355, 223)}, nil)
		}, "no text could be read"},
		{"candidates with no text in them", func() (*regrounder, *strings.Builder) {
			return regrounderOver("enter free play",
				[]observe.Region{atPx(399, 529, 355, 223)},
				[]readWord{word("FREE", 1700, 80, 70, 34)})
		}, "none of them contains any text"},
		{"the workflow has no name", func() (*regrounder, *strings.Builder) {
			return regrounderOver("", []observe.Region{atPx(399, 529, 355, 223)},
				[]readWord{word("FREE", 440, 600, 70, 34)})
		}, "no name to match against"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, out := tc.build()
			if _, _, ok := g.Reground(context.Background(), 2, "control_3",
				workflowrun.Sighting{}); ok {
				t.Fatal("grounded")
			}
			if !strings.Contains(out.String(), tc.say) {
				t.Errorf("want %q in:\n%s", tc.say, out.String())
			}
		})
	}
}

// ── the proposal beside the artifact ──────────────────────────────────────────

// A REPAIR THAT MOVED THE SCREEN AND A REPAIR THAT ARRIVED ARE DIFFERENT FINDINGS.
//
// Two fields, because a repair that produced a transition nobody recognised is a good region and
// a recognition problem, while one that produced nothing is a bad region. Collapsing them would
// file the first as a failure of the thing that worked -- which is exactly what happened on the
// measured run: the navigation succeeded and the destination was never established.
func TestAProposalSeparatesAChangedScreenFromARecognisedArrival(t *testing.T) {
	art := workflowstore.Artifact{
		Name: "enter free play",
		Controls: []workflowstore.Control{{
			Label:    "control_3",
			Readings: []observe.Region{{X: 0, Y: 0, Width: 1, Height: 0.957}},
		}},
	}
	live := atPx(340, 466, 142, 34)
	path := filepath.Join(t.TempDir(), "proposed.json")
	err := writeProposal(path, art, []workflowrun.Repair{{
		Step: 2, Control: "control_3", Where: live,
		Because: "the only control whose text is made of words from the name",
	}}, map[int]bool{} /* nothing was recognised */, true /* the screen changed */)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got proposal
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Repairs) != 1 {
		t.Fatalf("%d repair(s)", len(got.Repairs))
	}
	r := got.Repairs[0]
	if !r.Changed {
		t.Error("the proposal does not record that the screen changed")
	}
	if r.Arrived {
		t.Error("the proposal claims the destination was recognised, and it was not")
	}
	if r.Where != live {
		t.Errorf("the proposal carries %+v, want the re-grounded region %+v",
			r.Where, live)
	}
	// THE READING IT WOULD REPLACE, so a reviewer can see what was wrong with it
	// without opening the artifact beside this file.
	if len(got.Original) != 1 || len(got.Original[0].Reading) != 1 {
		t.Fatalf("the proposal does not carry the original reading: %+v", got.Original)
	}
	if ok, _ := got.Original[0].Reading[0].Locates(); ok {
		t.Error("the fixture's original reading is not the defect under test")
	}
	// AND IT SAYS IT IS A PROPOSAL.
	if !strings.Contains(got.Note, "not a correction") {
		t.Errorf("the note does not say this is a proposal: %s", got.Note)
	}
}

// AND A RUN WITH NO REPAIRS WRITES NO FILE.
//
// An empty proposal beside an artifact reads as "somebody looked and found nothing wrong", which
// is a different claim from "nothing needed repairing on this run".
func TestARunWithNoRepairsWritesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proposed.json")
	if err := writeProposal(path, workflowstore.Artifact{Name: "x"}, nil,
		nil, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a proposal was written for a run that repaired nothing")
	}
}

// ── the wiring ────────────────────────────────────────────────────────────────

// A MISSING REGROUNDER IS A NIL INTERFACE, not a typed nil that panics.
//
// A typed nil in an interface is not nil, so handing the coordinator `(*regrounder)(nil)`
// directly would install something that passes `r.regrounder != nil` and panics the first time a
// control locates nothing -- which is the only case it is ever consulted for.
func TestNoRegrounderInstallsNothingRatherThanATypedNil(t *testing.T) {
	if got := regroundOrNil(nil); got != nil {
		t.Errorf("a missing regrounder installed %v", got)
	}
	if got := regroundOrNil(&regrounder{}); got == nil {
		t.Error("a real regrounder was not installed")
	}
}
