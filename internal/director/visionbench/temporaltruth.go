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

// TrackTruth declares when one identity is on screen.
type TrackTruth struct {
	Identity string `json:"identity"`
	// Present are the spans the identity is visible in. Everything outside them, within
	// the sequence, is asserted absent.
	Present []Span `json:"present"`
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

		prev := -1
		for _, sp := range t.Present {
			switch {
			case sp.From > sp.To:
				out = append(out, fmt.Errorf("sequence %s track %q: span [%d,%d] runs "+
					"backwards", s.Sequence, t.Identity, sp.From, sp.To))
			case sp.From < 0 || sp.To >= frames:
				out = append(out, fmt.Errorf("sequence %s track %q: span [%d,%d] falls "+
					"outside the sequence's %d frames", s.Sequence, t.Identity,
					sp.From, sp.To, frames))
			case sp.From <= prev:
				// Ordered and disjoint, so "present frames" is a plain sum and the
				// absent complement is unambiguous.
				out = append(out, fmt.Errorf("sequence %s track %q: span [%d,%d] overlaps "+
					"or precedes the one before it; spans must be ordered and disjoint",
					s.Sequence, t.Identity, sp.From, sp.To))
			}
			if sp.To > prev {
				prev = sp.To
			}
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
