package main

// CAN THE BUTTON SOMEBODY CLICKED BE FOUND INSIDE THE PANEL THE PROPOSER OFFERED?
//
// # The question, and why it is not "find a smaller rectangle"
//
// [[Experiment-082-a-control-that-is-half-the-window]] measured a human click in Ark's menu
// becoming a control 932x690 pixels across, carrying thirteen words — the menu's whole
// vocabulary, not including the button that was pressed. The owner's reading of it:
//
//	Marco can recognize a visual region consistently without knowing whether that region is
//	actually a button. [...] The goal is not to find a smaller rectangle. It's to find a
//	defensible control boundary.
//
// So this measures candidates and the evidence under each, and it is allowed to abstain. Four
// candidates are computed for every press, from observations that already exist:
//
//	today        the smallest proposal containing the click. What the correlator picks now.
//	one-label    the smallest proposal containing the click that holds exactly ONE word box.
//	             A visual boundary AND a unique label, which is the hypothesis under test.
//	word box     the word box that contains the click, when one does. Absent is the PADDING
//	             case and is the thing that makes "just use the label" fail.
//	nearest      the word box whose centre is nearest the click, inside `today`. Computed to
//	             be REPORTED and never preferred — the owner was explicit:
//
//	Do not assume the nearest word is the target. [...] A click on button padding might be far
//	from the label, and nearby words could belong to different controls.
//
// # Why one-label is not another invented threshold
//
// "Exactly one" is not a tuned number. A rectangle holding two labels cannot be the control
// either of them names, and a rectangle holding none cannot be named at all. One is the only
// count that makes the rectangle and the word agree about a single thing, and it is the same
// shape of reasoning as `agreedWords` — see the inversion measured in
// [[Experiment-082-a-control-that-is-half-the-window]], where thirteen words made corroboration
// a rubber stamp because sharing ONE word is all it asks.
//
// # What this cannot measure, stated before any number
//
// There is no Ark capture with frames and click positions. `demonstrate-visual` keeps no frames
// on purpose, and at the time of writing it kept no press position either — it keeps every
// press now, in the raw record beside the routine, and still keeps no frames, so the Ark panel
// click could not have been inspected from one then and could not be now. The recording
// `director capture-game-session` writes is the only thing that keeps both, and on the day this
// was written it had never been run on Ark; it has since (see
// [[Experiment-085-the-graphics-tab-is-a-button]], which answered this).
//
// So this runs on `rl-freeplay-1`: 900 frames, 19 pointer
// presses with positions, and 300 looks of OCR carrying each phrase's rectangle, on Rocket
// League's main menu — a panel with twelve labels in it, which is the ambiguous case the
// investigation asks for.
//
// One press in it has a known answer. [[Experiment-050-two-readings-of-one-button]] measured the
// PLAY entry at 166x40 from (26,468), established by two presses at (125,480) and (159,499), and
// the first of those is press 1 of this recording to the pixel.

import (
	"fmt"
	"math"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// playButton is Experiment 050's measured PLAY entry, normalised on 1920x1080.
//
// The one oracle available here. Not a fixture: a rectangle a person established from two of
// their own presses, which is the only kind of ground truth this question has.
var playButton = observe.Region{X: 26.0 / 1920, Y: 468.0 / 1080,
	Width: 166.0 / 1920, Height: 40.0 / 1080}

// holds reports whether a rectangle contains a point.
//
// Local, and deliberately not borrowed from `observe`: this is a question about a click and a
// rectangle, not about whether two rectangles are one control. That relation is `SameRegion` and
// is used below exactly where identity is the question.
func holds(r observe.Region, at observe.PointerAt) bool {
	return at.X >= r.X && at.X <= r.X+r.Width && at.Y >= r.Y && at.Y <= r.Y+r.Height
}

func boxArea(r observe.Region) float64 { return r.Width * r.Height }

// inside reports whether a word box lies within a rectangle, by its CENTRE.
//
// By the centre rather than wholly, because OCR boxes overrun their glyphs and a label whose box
// is two pixels wider than its button would otherwise never be counted as being on it.
func inside(outer observe.Region, w observe.ReadWord) bool {
	return holds(outer, observe.PointerAt{
		X: w.Where.X + w.Where.Width/2, Y: w.Where.Y + w.Where.Height/2})
}

// candidate is one answer with the evidence that produced it.
type candidate struct {
	name  string
	where observe.Region
	words []string
	// found is false when this kind of evidence had nothing to say, which is an
	// abstention and is reported as one.
	found bool
}

func (c candidate) String() string {
	if !c.found {
		return fmt.Sprintf("%-10s —", c.name)
	}
	s := fmt.Sprintf("%-10s %5.1f%%x%5.1f%% at %.3f,%.3f", c.name,
		c.where.Width*100, c.where.Height*100, c.where.X, c.where.Y)
	if len(c.words) > 0 {
		s += fmt.Sprintf("  %v", c.words)
	}
	return s
}

// pressEvidence is everything known about one demonstrated click.
type pressEvidence struct {
	look int
	at   observe.PointerAt
	// readLook is the look the words came from, which may not be `look`: OCR answers on
	// about one look in three here, and a menu that is not moving reads the same on the
	// neighbouring one. The gap is reported rather than hidden.
	readLook  int
	proposals []observe.Region // containing the click, smallest first
	wordsIn   [][]string       // the words inside each proposal, parallel
	today     candidate
	oneLabel  candidate
	wordBox   candidate
	nearest   candidate
	// anywhere is the nearest word box on the whole look, and anywhereAt how far its
	// centre is from the click. The rule nobody may use, measured.
	anywhere   candidate
	anywhereAt float64
	// padding is true when no word box contains the click.
	padding bool
	// twins is how many OTHER proposals on this look have the same shape as the chosen
	// candidate — the "visually similar control elsewhere" case, counted with the
	// relation that decides whether two readings are one control.
	twins int
}

// buttonEvidence works out the four candidates for one press.
func buttonEvidence(look int, at observe.PointerAt, regions []observe.Region,
	words []observe.ReadWord, readLook int) pressEvidence {

	e := pressEvidence{look: look, at: at, readLook: readLook,
		today:    candidate{name: "today"},
		oneLabel: candidate{name: "one-label"},
		wordBox:  candidate{name: "word box"},
		nearest:  candidate{name: "nearest"},
		anywhere: candidate{name: "nearest any"},
	}
	for _, r := range regions {
		if holds(r, at) {
			e.proposals = append(e.proposals, r)
		}
	}
	sort.Slice(e.proposals, func(i, j int) bool {
		return boxArea(e.proposals[i]) < boxArea(e.proposals[j])
	})
	for _, p := range e.proposals {
		var in []string
		for _, w := range words {
			if inside(p, w) {
				in = append(in, w.Text)
			}
		}
		e.wordsIn = append(e.wordsIn, in)
	}
	if len(e.proposals) > 0 {
		e.today = candidate{name: "today", where: e.proposals[0],
			words: e.wordsIn[0], found: true}
	}
	// ONE LABEL, smallest first — which is what the sort above is for.
	for i, p := range e.proposals {
		if len(e.wordsIn[i]) == 1 {
			e.oneLabel = candidate{name: "one-label", where: p,
				words: e.wordsIn[i], found: true}
			break
		}
	}
	// THE WORD BOX UNDER THE CLICK. Its absence is the padding case.
	for _, w := range words {
		if holds(w.Where, at) {
			e.wordBox = candidate{name: "word box", where: w.Where,
				words: []string{w.Text}, found: true}
			break
		}
	}
	e.padding = !e.wordBox.found
	// AND THE NEAREST, inside today's proposal only, reported and never preferred.
	if e.today.found {
		best := -1.0
		for _, w := range words {
			if !inside(e.today.where, w) {
				continue
			}
			dx := w.Where.X + w.Where.Width/2 - at.X
			dy := w.Where.Y + w.Where.Height/2 - at.Y
			d := dx*dx + dy*dy
			if best < 0 || d < best {
				best = d
				e.nearest = candidate{name: "nearest", where: w.Where,
					words: []string{w.Text}, found: true}
			}
		}
	}
	// AND THE NEAREST WORD ANYWHERE ON THE LOOK, with how far away it is.
	//
	// This is the rule the owner forbade, measured so that the forbidding has a number
	// under it: *Do not assume the nearest word is the target.* On a game menu the label
	// of the control you just clicked is the one most likely to be MISSING, because
	// clicking it highlights it and a filled bar with inverted text reads differently --
	// `controlWords`' own doc records the press that proved it, and
	// [[Experiment-050-two-readings-of-one-button]] is that press. So the nearest word to
	// a click on the selected entry is a word belonging to a DIFFERENT control.
	best := -1.0
	for _, w := range words {
		dx := w.Where.X + w.Where.Width/2 - at.X
		dy := w.Where.Y + w.Where.Height/2 - at.Y
		d := dx*dx + dy*dy
		if best < 0 || d < best {
			best = d
			e.anywhere = candidate{name: "nearest any", where: w.Where,
				words: []string{w.Text}, found: true}
			e.anywhereAt = math.Sqrt(d)
		}
	}
	// TWINS of the chosen candidate, with the identity relation rather than a new one.
	chosen := e.oneLabel
	if !chosen.found {
		chosen = e.today
	}
	if chosen.found {
		for _, r := range regions {
			if observe.SameRegion(r, chosen.where) && !sameRegionExactly(r, chosen.where) {
				e.twins++
			}
		}
	}
	return e
}

// sameRegionExactly is identity of the rectangle itself, for excluding a candidate from its own
// twin count. Not a judgement about controls — that is SameRegion, used beside it.
func sameRegionExactly(a, b observe.Region) bool {
	return a.X == b.X && a.Y == b.Y && a.Width == b.Width && a.Height == b.Height
}
