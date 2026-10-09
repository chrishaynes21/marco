package visionbench

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// When each declared identity is actually on screen.
//
// # Why intervals, and why the previous model was not enough
//
// The first attempt gave each sequence a MODE and a single BOUNDARY: static, appearance at
// frame n, disappearance at frame n. That repaired the original defect — mirror-image
// transitions scoring oppositely because one fell either side of a majority threshold — but it
// could only describe a scene that changes once.
//
// `pause-close` changes twice. Read from the pixels:
//
//	0  menu at full opacity
//	1  menu at full opacity
//	2  no menu; gameplay, with a control legend top-right
//	3  menu attenuated behind a transition effect, every item still legible
//	4  menu at full opacity
//	5  menu at full opacity
//
// A single disappearance boundary cannot say that, and forcing it to produced a corpus that
// asserted "no interface" over frames plainly containing a menu. The detector was then charged
// with false positives for being right.
//
// So the model is intervals: per identity, the spans in which it is present. Everything else in
// the sequence is asserted ABSENT. One mechanism describes all of it —
//
//	present throughout                [[0,n]]        static
//	absent then present               [[k,n]]        appearance
//	present then absent               [[0,k]]        disappearance
//	present, absent, present          [[0,1],[3,5]]  recurring
//	brief presence                    [[2,3]]        transient
//
// — with no mode to choose, no boundary to place, and nothing in the scorer that knows what a
// pause menu is.
//
// # Why absence must be DECLARED rather than inferred from missing annotations
//
// This is the load-bearing reason the file exists at all. The corpus is deliberately partial:
// only structures the benchmark scores are marked, and an unmatched detection is neither
// credited nor penalised precisely because nobody annotated everything. If the scorer treated
// "no annotation here" as "asserted absent", every detector would be charged for the corpus's
// incompleteness.
//
// A declared interval is the difference between *nobody wrote it down* and *it is not there*.
// Only declared identities are scored for timing; everything else keeps the static rule.

// SequenceSchema is the version of the sequence-level temporal declaration.
//
// 2: intervals replace mode+boundary. Bumped rather than extended because a schema-1 file's
// `mode`/`boundary` cannot be reinterpreted as spans without guessing, and a corpus that
// silently reads half a declaration is the failure this whole area exists to prevent.
const SequenceSchema = 2

// Span is an inclusive range of frame indices.
//
// Marshals as `[from, to]`, because these files are edited by hand beside the frames they
// describe and `[[0,1],[3,5]]` can be checked against six PNGs at a glance.
type Span struct{ From, To int }

func (s Span) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]int{s.From, s.To})
}

func (s *Span) UnmarshalJSON(b []byte) error {
	var pair [2]int
	if err := json.Unmarshal(b, &pair); err != nil {
		return fmt.Errorf("a presence span must be [from, to]: %w", err)
	}
	s.From, s.To = pair[0], pair[1]
	return nil
}

// Covers reports whether an index falls inside the span.
func (s Span) Covers(index int) bool { return index >= s.From && index <= s.To }

// Len is how many frames the span covers.
func (s Span) Len() int { return s.To - s.From + 1 }

// TrackTruth declares when one identity is on screen, and when it is the chosen one.
type TrackTruth struct {
	Identity string `json:"identity"`
	// Present are the spans the identity is visible in. Everything outside them, within
	// the sequence, is asserted absent.
	Present []Span `json:"present"`
	// Selected are the spans this identity is the CHOSEN ONE OF ITS SIBLINGS in.
	//
	// The word is `observe.Selected`'s, deliberately, rather than a third spelling of a
	// state the tree already names: a highlighted menu entry and a list's chosen row are
	// the same claim seen through two providers, and a corpus that called it "highlighted"
	// would need a translation nobody would maintain.
	//
	// # Why presence alone cannot describe a menu
	//
	// Everything above this field is about a thing ARRIVING or LEAVING. That is the right
	// model for a panel that fades in, and the wrong one for a menu somebody is moving
	// through: the panel and every entry are present in every frame, so presence intervals
	// describe nine frames of a held-open pause menu as perfectly static — which is true,
	// and useless. The only thing that changes is which entry is chosen.
	//
	// That matters beyond annotation tidiness. A search that steps through a menu is bound
	// by ADR-143's rule that an unchanged view is not a place that was searched. In a menu
	// the selection IS the change, so without it every step looks like a view with nowhere
	// to go, and the search would report that it had looked everywhere while sitting on
	// the first entry. A corpus that cannot say where the selection is cannot measure a
	// menu search at all.
	//
	// # Unannotated is not unselected
	//
	// Selection is scoped to the SEQUENCE, not to the track: if no track here declares a
	// selected span, this sequence says nothing about selection and a metric over it must
	// refuse rather than report zero. If any track declares one, selection is annotated
	// throughout, and an identity outside its spans is asserted NOT chosen. The same
	// distinction Present draws between a declared absence and an unwritten one, one level
	// up — see ADR-006, unknown is not false. Read it through Selection().
	//
	// Sequence-level rather than per-frame on purpose. FrameTruth could carry a flag per
	// region, and then nine frames would each restate a fact this file already holds — a
	// second source of truth about one thing, which Shape's own comment refuses for
	// exactly this reason.
	//
	// No schema bump. A schema-2 file written before this field is read correctly as one
	// that does not annotate selection, which is precisely what it is; the bump rule
	// exists to stop a file being MISREAD, and mode-versus-intervals could be, where this
	// cannot.
	Selected []Span `json:"selected,omitempty"`
}

// PresentAt reports whether the identity is declared visible at a frame index.
func (t TrackTruth) PresentAt(index int) bool {
	for _, s := range t.Present {
		if s.Covers(index) {
			return true
		}
	}
	return false
}

// SelectedAt reports whether the identity is declared the chosen sibling at a frame index.
//
// False here means one of two different things, and the caller cannot tell them apart from
// this method alone: not chosen, or nobody said. Ask the sequence through Selection() first.
func (t TrackTruth) SelectedAt(index int) bool {
	for _, s := range t.Selected {
		if s.Covers(index) {
			return true
		}
	}
	return false
}

// Frames is how many frames the identity is declared present in.
func (t TrackTruth) Frames() int {
	n := 0
	for _, s := range t.Present {
		n += s.Len()
	}
	return n
}

// Shape names this track's temporal pattern, for reporting only.
//
// Descriptive, never an input to scoring — the scorer reads intervals and nothing else. A label
// that could change a number would be a second source of truth about the same fact.
func (t TrackTruth) Shape(frames int) string {
	switch {
	case len(t.Present) == 0:
		return "absent"
	case len(t.Present) > 1:
		return "recurring"
	case t.Frames() >= frames:
		return "static"
	case t.Present[0].From == 0:
		return "disappearance"
	case t.Present[0].To >= frames-1:
		return "appearance"
	default:
		return "transient"
	}
}

// SequenceTruth is one sequence's temporal declaration.
type SequenceTruth struct {
	Schema   int          `json:"schema"`
	Sequence string       `json:"sequence"`
	Tracks   []TrackTruth `json:"tracks"`
	Note     string       `json:"note,omitempty"`
	// SelectionRead records that somebody looked at selection across this sequence and
	// wrote down what they saw — including, where that is the answer, that nothing was
	// chosen.
	//
	// Without it "no track declares a selected span" has two readings and the file cannot
	// tell them apart: nobody looked, or somebody looked and there was no highlight. The
	// corpus contains a real instance of the second, which is what forced this field —
	// `pause-close` shows the pause menu at full opacity with all four entries identical
	// and no glyph, the state Rocket League leaves a menu in when the pointer takes over
	// from the controller. Annotating that as "unannotated" would throw away a measured
	// fact, and annotating it as a chosen entry would invent one.
	SelectionRead bool `json:"selection_read,omitempty"`
}

// Track returns the declaration for one identity.
func (s SequenceTruth) Track(identity string) (TrackTruth, bool) {
	for _, t := range s.Tracks {
		if t.Identity == identity {
			return t, true
		}
	}
	return TrackTruth{}, false
}

// SelectionCoverage says what a sequence is able to tell anyone about selection.
//
// Three values because there are three states and collapsing the first two is the mistake
// this type exists to prevent: a sequence that never mentions selection and a sequence whose
// selection never moves look identical to any metric that only counts matches, and they are
// opposite findings. One is a gap in the corpus; the other is a property of the scene.
type SelectionCoverage int

const (
	// SelectionUnannotated — nobody wrote selection down here. A selection metric over
	// this sequence has no truth to compare against and must be withheld, not zeroed.
	SelectionUnannotated SelectionCoverage = iota
	// SelectionNone — selection WAS read, and nothing is chosen. A real answer, and the
	// one that makes "always say the first entry" a falsifiable claim rather than a free
	// pass. Distinct from unannotated by `SelectionRead` alone, which is the whole reason
	// that field exists.
	SelectionNone
	// SelectionStill — selection is annotated and stays on one identity throughout. Enough
	// to score "did the detector see which one is chosen", and NOT enough to score a menu
	// search, which is judged on noticing the selection move.
	SelectionStill
	// SelectionMoves — selection is annotated and passes between identities. The only
	// coverage that can measure stepping through a menu.
	SelectionMoves
)

func (c SelectionCoverage) String() string {
	switch c {
	case SelectionNone:
		return "none-chosen"
	case SelectionStill:
		return "still"
	case SelectionMoves:
		return "moves"
	}
	return "unannotated"
}

// Measurable reports whether a selection metric over this sequence means anything.
//
// The one question a scorer should ask. Everything but `unannotated` has a truth to compare
// against, including `none-chosen`, where the correct detection is no highlight at all.
func (c SelectionCoverage) Measurable() bool { return c != SelectionUnannotated }

// Selection reports what this sequence can say about the chosen sibling.
func (s SequenceTruth) Selection() SelectionCoverage {
	holders := 0
	for _, t := range s.Tracks {
		if len(t.Selected) > 0 {
			holders++
		}
	}
	switch {
	case holders == 0 && !s.SelectionRead:
		return SelectionUnannotated
	case holders == 0:
		return SelectionNone
	case holders == 1:
		return SelectionStill
	}
	return SelectionMoves
}

// SelectedAt names the identity declared chosen at a frame index.
//
// The second return is false when nothing is chosen there, which — as everywhere in this
// file — is only a claim at all once Selection() says the sequence annotates selection.
func (s SequenceTruth) SelectedAt(index int) (string, bool) {
	for _, t := range s.Tracks {
		if t.SelectedAt(index) {
			return t.Identity, true
		}
	}
	return "", false
}

// Shape summarises the sequence for a report: the distinct track shapes it contains.
func (s SequenceTruth) Shape(frames int) string {
	if len(s.Tracks) == 0 {
		return "undeclared"
	}
	seen := map[string]bool{}
	var out []string
	for _, t := range s.Tracks {
		if sh := t.Shape(frames); !seen[sh] {
			seen[sh] = true
			out = append(out, sh)
		}
	}
	sort.Strings(out)
	return strings.Join(out, "+")
}

// Validate reports every structural problem with a declaration.
func (s SequenceTruth) Validate(frames int) []error {
	var out []error
	if s.Schema != SequenceSchema {
		out = append(out, fmt.Errorf("sequence %s: schema %d, this build understands %d",
			s.Sequence, s.Schema, SequenceSchema))
	}
	seen := map[string]bool{}
	for _, t := range s.Tracks {
		if strings.TrimSpace(t.Identity) == "" {
			out = append(out, fmt.Errorf("sequence %s: a track declares no identity",
				s.Sequence))
			continue
		}
		if seen[t.Identity] {
			out = append(out, fmt.Errorf(
				"sequence %s: identity %q is declared twice; one declaration would win "+
					"silently", s.Sequence, t.Identity))
		}
		seen[t.Identity] = true

		out = append(out, s.checkSpans(t.Identity, "present", t.Present, frames)...)
		out = append(out, s.checkSpans(t.Identity, "selected", t.Selected, frames)...)

		// A thing that is not on screen cannot be the chosen one of its siblings. The two
		// statements would both be in the file, and a scorer reading one would disagree
		// with a scorer reading the other — the same shape of contradiction
		// CheckAnnotations exists to refuse between this file and the frames.
		for _, sp := range t.Selected {
			for i := sp.From; i <= sp.To; i++ {
				if i < 0 || i >= frames || t.PresentAt(i) {
					continue
				}
				out = append(out, fmt.Errorf("sequence %s track %q: selected at frame %d, "+
					"where it is declared absent", s.Sequence, t.Identity, i))
				break
			}
		}
	}

	// Writing down a chosen entry IS reading selection. A file that does one without
	// claiming the other reports `unannotated` from Selection() while plainly holding an
	// annotation, so the metric would be withheld over evidence that exists.
	if !s.SelectionRead {
		for _, t := range s.Tracks {
			if len(t.Selected) > 0 {
				out = append(out, fmt.Errorf("sequence %s track %q: declares selected "+
					"spans while the sequence does not set selection_read; set it, or "+
					"the annotation is not counted as read", s.Sequence, t.Identity))
			}
		}
	}

	// One chosen sibling, which is what the word means. Two annotated at once is not a
	// scene anybody photographed; it is an editing slip, and letting the first one win
	// would silently decide which.
	for i := 0; i < frames; i++ {
		var holders []string
		for _, t := range s.Tracks {
			if t.SelectedAt(i) {
				holders = append(holders, t.Identity)
			}
		}
		if len(holders) > 1 {
			out = append(out, fmt.Errorf("sequence %s: frame %d declares %d identities "+
				"selected at once (%s); only one sibling is the chosen one",
				s.Sequence, i, len(holders), strings.Join(holders, ", ")))
		}
	}
	return out
}

// checkSpans holds the rules every interval list obeys, whichever field it came from.
//
// Shared rather than written twice: presence and selection are the same vocabulary, and two
// copies of these four checks would drift the first time one was fixed.
func (s SequenceTruth) checkSpans(identity, field string, spans []Span, frames int) []error {
	var out []error
	prev := -1
	for _, sp := range spans {
		switch {
		case sp.From > sp.To:
			out = append(out, fmt.Errorf("sequence %s track %q: %s span [%d,%d] runs "+
				"backwards", s.Sequence, identity, field, sp.From, sp.To))
		case sp.From < 0 || sp.To >= frames:
			out = append(out, fmt.Errorf("sequence %s track %q: %s span [%d,%d] falls "+
				"outside the sequence's %d frames", s.Sequence, identity, field,
				sp.From, sp.To, frames))
		case sp.From <= prev:
			// Ordered and disjoint, so "present frames" is a plain sum and the absent
			// complement is unambiguous.
			out = append(out, fmt.Errorf("sequence %s track %q: %s span [%d,%d] overlaps "+
				"or precedes the one before it; spans must be ordered and disjoint",
				s.Sequence, identity, field, sp.From, sp.To))
		}
		if sp.To > prev {
			prev = sp.To
		}
	}
	return out
}

// CheckAnnotations refuses a declaration that disagrees with the frames.
//
// Bidirectional, and that is the whole point. The corpus must not be able to say an identity is
// absent while the same frame annotates it, nor claim presence where no annotated region
// exists. `pause-close` did the first: six identities declared gone across four frames, two of
// which show the menu at full opacity. Nothing caught it, because nothing compared the two
// statements.
//
// Reported, never repaired. Silently preferring one of two contradictory statements is how a
// corpus starts lying.
func (s SequenceTruth) CheckAnnotations(seq []FrameTruth) []error {
	annotated := map[string]map[int]bool{}
	for _, ft := range seq {
		for _, r := range ft.Regions {
			id := identityOf(r)
			if annotated[id] == nil {
				annotated[id] = map[int]bool{}
			}
			annotated[id][ft.Index] = true
		}
	}

	var out []error
	declared := map[string]bool{}
	for _, t := range s.Tracks {
		declared[t.Identity] = true
		for _, ft := range seq {
			want := t.PresentAt(ft.Index)
			got := annotated[t.Identity][ft.Index]
			if want == got {
				continue
			}
			if want {
				out = append(out, fmt.Errorf(
					"sequence %s: %q is declared present at frame %d but no region there "+
						"annotates it — temporal truth cannot claim presence the frame "+
						"does not show", s.Sequence, t.Identity, ft.Index))
			} else {
				out = append(out, fmt.Errorf(
					"sequence %s: %q is declared ABSENT at frame %d but the frame annotates "+
						"it — a detector that finds it would be charged with a false "+
						"positive for being right", s.Sequence, t.Identity, ft.Index))
			}
		}
	}
	// An annotated identity with no declaration is not an error — it keeps the static rule
	// and is never charged for absence. But an identity that recurs across frames is
	// exactly the kind the timing metric exists for, so say so.
	ids := make([]string, 0, len(annotated))
	for id := range annotated {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !declared[id] && len(annotated[id]) < len(seq) {
			out = append(out, fmt.Errorf(
				"sequence %s: %q appears in %d of %d frames but declares no presence "+
					"intervals — without them the scorer cannot tell 'not there' from "+
					"'nobody annotated it', and will not measure its timing",
				s.Sequence, id, len(annotated[id]), len(seq)))
		}
	}
	return out
}

// LoadSequenceTruth reads a sequence's temporal declaration.
//
// Required, not optional. A missing declaration would leave every identity on the static
// majority rule, which is how a two-frame menu in a six-frame sequence scored 0/0 and was
// rendered as a 0% detector failure.
func LoadSequenceTruth(dir string, seq []FrameTruth) (SequenceTruth, error) {
	name := filepath.Base(dir)
	path := filepath.Join(dir, "sequence.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return SequenceTruth{}, fmt.Errorf(
			"%s declares no temporal truth: %s is missing. Every sequence must say which "+
				"frames each identity is present in; without that the scorer cannot tell an "+
				"absent element from an unannotated one", name, filepath.Base(path))
	}
	var s SequenceTruth
	if err := json.Unmarshal(raw, &s); err != nil {
		return SequenceTruth{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if strings.TrimSpace(s.Sequence) == "" {
		s.Sequence = name
	}
	if s.Sequence != name {
		return SequenceTruth{}, fmt.Errorf(
			"%s declares sequence %q but lives in directory %q", path, s.Sequence, name)
	}
	problems := s.Validate(len(seq))
	problems = append(problems, s.CheckAnnotations(seq)...)
	if len(problems) > 0 {
		msg := fmt.Sprintf("%d problem(s) in %s:", len(problems), path)
		for _, p := range problems {
			msg += "\n  " + p.Error()
		}
		return SequenceTruth{}, errors.New(msg)
	}
	return s, nil
}
