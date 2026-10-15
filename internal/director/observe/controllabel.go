package observe

// What a demonstrated click remembers about WHAT it clicked.
//
// # The gap this closes, measured live
//
// [[Experiment-049-the-control-it-could-see-and-could-not-name]] ran the demonstrated route
// against the real game. Marco pressed PLAY, the screen changed, the Play page was recognised —
// and the next step refused, because `control_2` was a rectangle and nothing else. The
// re-grounder fell back to the only human-supplied meaning in the artifact, the workflow's own
// NAME, and reported this:
//
//	nothing on this screen is named in the workflow's own name ("enter free play")
//	  "QUICK PLAY"      x  284.. 386  y  890.. 924
//	  "TRAINING"        x  402.. 520  y  718.. 750       <- the control it wanted
//	  "PLAY OFFLINE"    x  780.. 948  y  720.. 752
//
// The word was on the screen, read, and in the list. Nothing connected it to the control,
// because the demonstration never recorded that the thing the person clicked said TRAINING.
//
// `Artifact.ExpectedWords` has preferred saved control text over the workflow's name since
// [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — with the provenance beside it,
// because the two are not equally good evidence. The rule was already there. The field was
// always empty.
//
// # What may become a label, and what may not
//
// The words INSIDE the rectangle this press identified the control as. Not the nearest word, not
// the words of the screen, not the workflow's title:
//
//   - `NameCandidates` is the association, unchanged and shared with the live identity tier, so
//     a control is named here exactly as a candidate is named there. It assigns each word to the
//     SMALLEST locating candidate that wholly contains it.
//   - and it refuses a region that `Locates` refuses. A band across the window is not a
//     candidate however much is written on it, so a press whose hypothesis is the whole window
//     learns NOTHING rather than learning the screen's whole vocabulary as a control's name.
//     That is the measured case of `control_3` — the Free Play control saved as 0,0 1.000x0.957
//     — and it is the difference between leaving a target unrecoverable and teaching a wrong one.
//
// # Why presses have to agree
//
// [[ADR-194-a-screen-is-also-what-it-says]] measured that one reading of a dark game menu
// carries its own garbage, and a single reading as evidence split one main menu into eight
// identities. A control's rectangle is far smaller than a screen and the risk is correspondingly
// smaller, but the direction of the error is the same and it is worse here: this label will be
// used to FIND a control and press it.
//
// So a control clicked twice keeps what both readings agreed on, and a control clicked once
// keeps what its one reading said, with `TextPresses` recording which of those happened. The
// intersection is taken only across presses that carried a reading at all — a press nobody read
// is missing evidence, and missing evidence does not erase what another press established.

import "sort"

// MaxRecentReadings is how many positioned readings are held for a control not yet clicked.
//
// A press is folded before anybody knows the rectangle it will establish, so the readings that
// name a control are usually the ones BEFORE it was clicked -- measured: the word PLAY was read
// on seventeen looks of the main menu and on none of the six before the press, because the entry
// was highlighted by the time the pointer reached it. Bounded because this is a live observer
// and an unbounded buffer of readings is a leak with a long fuse.
const MaxRecentReadings = 16

// LabelOf is what a control says, read off the rectangle a press identified it as.
//
// Returns nothing when the region locates nothing, when nobody read this look, or when no word
// lies wholly inside it. All three are the same answer to a caller — this press did not learn
// what the control is called — and none of them is a reason to reach for a word that is merely
// nearby.
func LabelOf(where Region, read []ReadWord) []string {
	if len(read) == 0 {
		return nil
	}
	named := NameCandidates([]Region{where}, read)
	if len(named) == 0 {
		return nil // the region locates nothing, so a word inside it says nothing
	}
	return Words(named[0].Text)
}

// controlWords is what one control's own rectangle has been read to say.
//
// Counted over looks rather than taken from one, for the reason the press that started all of
// this demonstrates: the person clicked PLAY on the look where OCR had stopped reading the word
// PLAY, because a highlighted menu entry renders differently from an unhighlighted one. One
// reading at the moment of the press is the single worst look to ask.
//
// So every look of the screen the control is on contributes, and a word is kept when it was
// read on at least `TextCommon` of them -- the same share, and the same reasoning, as a screen's
// own vocabulary in screentextfold.go. A caption OCR resolved on four looks of five is what that
// control says; one that appeared once is this reader's noise.
type controlWords struct {
	counts map[string]int
	looks  int
}

// saw folds one look's reading of this control's rectangle.
//
// A look that read NOTHING inside the rectangle is not counted at all, which is deliberate and
// is the difference between a control nobody could read and a control that says nothing: the
// first must not dilute what the other looks established, and the second has no words to
// dilute it with. Missing evidence is not contradictory evidence, one tier down.
func (c *controlWords) saw(words []string) {
	if len(words) == 0 {
		return
	}
	if c.counts == nil {
		c.counts = map[string]int{}
	}
	c.looks++
	seen := map[string]bool{}
	for _, w := range words {
		if seen[w] {
			continue // a word twice on one look is one look's evidence
		}
		seen[w] = true
		c.counts[w]++
	}
}

// list is what the control says: the words read on at least TextCommon of its looks.
func (c controlWords) list() []string {
	if c.looks == 0 {
		return nil
	}
	var out []string
	for w, n := range c.counts {
		if float64(n) >= TextCommon*float64(c.looks) {
			out = append(out, w)
		}
	}
	sort.Strings(out)
	return out
}

// agreedWords is the words two readings of one control both had.
//
// The same shape as `TextBoth` one level down, and for the same reason: two readings of one
// thing agree about what it says, and whatever only one of them saw is as likely to be the
// reader's noise as the control's name.
func agreedWords(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	in := make(map[string]bool, len(b))
	for _, w := range b {
		in[w] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, w := range a {
		if in[w] && !seen[w] {
			out = append(out, w)
			seen[w] = true
		}
	}
	sort.Strings(out)
	return out
}
