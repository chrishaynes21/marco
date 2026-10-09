package visionbench_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// What a corpus can and cannot say about which entry is chosen.
//
// The distinction these tests defend is the one between "nobody wrote selection down" and
// "nothing is selected". Every metric built on top of this will be tempted to collapse them,
// because both produce no matches.

func declaredTrack(identity string, present, selected []visionbench.Span) visionbench.TrackTruth {
	return visionbench.TrackTruth{Identity: identity, Present: present, Selected: selected}
}

func span(from, to int) visionbench.Span { return visionbench.Span{From: from, To: to} }

// menu is the shape of a held-open menu: a panel and three entries, all present throughout.
//
// Declaring any chosen entry means selection was read, so the helper sets the flag with it;
// the sequences that leave `selected` empty are the ones where nobody looked.
func menu(selected map[string][]visionbench.Span) visionbench.SequenceTruth {
	s := visionbench.SequenceTruth{Schema: 2, Sequence: "pause", SelectionRead: len(selected) > 0}
	for _, id := range []string{"pause_panel", "menu_resume", "menu_settings", "menu_exit"} {
		s.Tracks = append(s.Tracks, declaredTrack(id, []visionbench.Span{span(0, 5)}, selected[id]))
	}
	return s
}

func TestASequenceThatNeverMentionsSelectionIsNotASequenceWithNothingSelected(t *testing.T) {
	s := menu(nil)

	if got := s.Selection(); got != visionbench.SelectionUnannotated {
		t.Fatalf("a sequence with no selected spans should be unannotated, got %v", got)
	}
	// And the per-frame answer is false everywhere, which is exactly why the coverage has
	// to be asked first: these two sequences are indistinguishable frame by frame.
	for i := 0; i < 6; i++ {
		if id, ok := s.SelectedAt(i); ok {
			t.Fatalf("frame %d: nothing was annotated, but %q came back chosen", i, id)
		}
	}
	if errs := s.Validate(6); len(errs) > 0 {
		t.Fatalf("an unannotated sequence is not an invalid one: %v", errs)
	}
}

// TestAMenuWithNothingChosenIsAnAnswerNotAGap is the distinction the real corpus forced.
//
// `pause-close` shows the pause menu at full opacity with all four entries identical and no
// glyph — the state Rocket League leaves a menu in when the pointer takes over. Measured: the
// four entry rects read 95/100/93/99 mean brightness there, against 215/100/93/99 in
// `pause-stable`, where Resume is chosen. Nothing is highlighted, and that IS the observation.
func TestAMenuWithNothingChosenIsAnAnswerNotAGap(t *testing.T) {
	looked := menu(nil)
	looked.SelectionRead = true

	if got := looked.Selection(); got != visionbench.SelectionNone {
		t.Fatalf("a menu somebody read and found no highlight in is none-chosen, got %v", got)
	}
	if !looked.Selection().Measurable() {
		t.Error("none-chosen has a truth to score against: the correct answer is no highlight")
	}
	// The sequence nobody looked at produces identical tracks, and must not be measurable.
	if menu(nil).Selection().Measurable() {
		t.Error("an unannotated sequence must withhold the metric")
	}
	if errs := looked.Validate(6); len(errs) > 0 {
		t.Fatalf("declaring that nothing is chosen is legal: %v", errs)
	}
}

func TestAChosenEntryWithoutSelectionReadIsRefused(t *testing.T) {
	// The two halves of one statement. Writing down a chosen entry IS reading selection,
	// and a file that does one without the other reports `unannotated` while holding an
	// annotation — withholding a metric over evidence that exists.
	s := menu(map[string][]visionbench.Span{"menu_resume": {span(0, 5)}})
	s.SelectionRead = false

	joined := errsText(s.Validate(6))
	if !strings.Contains(joined, "selection_read") {
		t.Fatalf("want the contradiction named, got %q", joined)
	}
}

func TestSelectionThatStaysPutCannotMeasureAMenuSearch(t *testing.T) {
	s := menu(map[string][]visionbench.Span{
		"menu_resume": {span(0, 5)},
	})

	if got := s.Selection(); got != visionbench.SelectionStill {
		t.Fatalf("one identity holding selection throughout is still, got %v", got)
	}
	// This is the shipped corpus's situation if its frames were annotated as they stand:
	// enough to ask "which one is chosen", not enough to ask "did it move".
	if got := s.Selection(); got == visionbench.SelectionMoves {
		t.Fatal("a still selection must not read as a moving one")
	}
}

func TestSelectionPassingBetweenEntriesIsTheCoverageAMenuSearchNeeds(t *testing.T) {
	s := menu(map[string][]visionbench.Span{
		"menu_resume":   {span(0, 1)},
		"menu_settings": {span(2, 3)},
		"menu_exit":     {span(4, 5)},
	})

	if got := s.Selection(); got != visionbench.SelectionMoves {
		t.Fatalf("selection passing between three entries moves, got %v", got)
	}
	for _, c := range []struct {
		frame int
		want  string
	}{{0, "menu_resume"}, {1, "menu_resume"}, {2, "menu_settings"}, {5, "menu_exit"}} {
		got, ok := s.SelectedAt(c.frame)
		if !ok || got != c.want {
			t.Errorf("frame %d: chosen %q (%v), want %q", c.frame, got, ok, c.want)
		}
	}
	if errs := s.Validate(6); len(errs) > 0 {
		t.Fatalf("a moving selection is a legal declaration: %v", errs)
	}
}

func TestAnEntryCannotBeChosenWhileItIsDeclaredAbsent(t *testing.T) {
	s := visionbench.SequenceTruth{Schema: 2, Sequence: "pause-open", SelectionRead: true, Tracks: []visionbench.TrackTruth{
		// The menu arrives at frame 3, and the declaration claims its entry was already
		// the chosen one at frame 0.
		declaredTrack("menu_resume", []visionbench.Span{span(3, 5)}, []visionbench.Span{span(0, 5)}),
	}}

	errs := s.Validate(6)
	if len(errs) == 0 {
		t.Fatal("selected where declared absent must be refused")
	}
	if !strings.Contains(errs[0].Error(), "declared absent") {
		t.Fatalf("the error should say what contradicts what, got %v", errs[0])
	}
}

func TestTwoEntriesChosenAtOnceIsRefused(t *testing.T) {
	s := menu(map[string][]visionbench.Span{
		"menu_resume":   {span(0, 3)},
		"menu_settings": {span(2, 5)},
	})

	errs := s.Validate(6)
	if len(errs) == 0 {
		t.Fatal("two identities selected in one frame must be refused, not resolved")
	}
	joined := errsText(errs)
	if !strings.Contains(joined, "menu_resume") || !strings.Contains(joined, "menu_settings") {
		t.Fatalf("the error must name both claimants, got %v", joined)
	}
}

func TestSelectionSpansObeyTheSameRulesAsPresenceSpans(t *testing.T) {
	cases := []struct {
		name string
		sel  []visionbench.Span
		want string
	}{
		{"backwards", []visionbench.Span{span(4, 1)}, "runs backwards"},
		{"off the end", []visionbench.Span{span(0, 99)}, "outside the sequence"},
		{"overlapping", []visionbench.Span{span(0, 3), span(2, 5)}, "ordered and disjoint"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := visionbench.SequenceTruth{Schema: 2, Sequence: "pause", SelectionRead: true, Tracks: []visionbench.TrackTruth{
				declaredTrack("menu_resume", []visionbench.Span{span(0, 5)}, c.sel),
			}}
			joined := errsText(s.Validate(6))
			if !strings.Contains(joined, c.want) {
				t.Fatalf("want %q in the errors, got %q", c.want, joined)
			}
			// The annotator is editing one of two interval lists and has to be told
			// which, or the fix lands in the wrong field.
			if !strings.Contains(joined, "selected span") {
				t.Fatalf("the error must name the field it is about, got %q", joined)
			}
		})
	}
}

func TestPresenceSpansStillObeyTheirRules(t *testing.T) {
	// The rules moved into a shared helper when selection arrived. They are the older and
	// more load-bearing half, so they get their own claim rather than being assumed.
	s := visionbench.SequenceTruth{Schema: 2, Sequence: "pause", Tracks: []visionbench.TrackTruth{
		declaredTrack("menu_resume", []visionbench.Span{span(0, 3), span(2, 5)}, nil),
	}}
	joined := errsText(s.Validate(6))
	if !strings.Contains(joined, "ordered and disjoint") {
		t.Fatalf("overlapping presence spans must still be refused, got %q", joined)
	}
	if !strings.Contains(joined, "present span") {
		t.Fatalf("the error must name the field it is about, got %q", joined)
	}
}

// TestTheShippedCorpusCannotYetMeasureAMovingHighlight records a gap, on purpose.
//
// Every sequence in the Rocket League corpus annotates presence and nothing else, and the
// pause frames show one entry — Resume Game — highlighted identically across all nine. So a
// detector that always answered "the first one" would be unfalsifiable here, and a menu
// search cannot be scored on this corpus at all.
//
// WHEN THIS TEST FAILS, THAT IS THE GOOD OUTCOME: it means a capture with a moving highlight
// landed. Update it to assert the coverage the new sequences actually have, rather than
// deleting it — the point is that nobody adds a selection metric without first reading what
// the corpus can support.
func TestTheShippedCorpusCannotYetMeasureAMovingHighlight(t *testing.T) {
	c, err := visionbench.LoadV2("../../../fixtures/vision/v2/rocketleague")
	if err != nil {
		t.Skipf("no v2 corpus: %v", err)
	}
	for _, name := range c.SequenceNames() {
		got := c.Modes[name].Selection()
		if got != visionbench.SelectionUnannotated {
			t.Errorf("sequence %s now reports selection coverage %v — a capture landed; "+
				"read this test's comment", name, got)
		}
	}
}

func errsText(errs []error) string {
	var b strings.Builder
	for _, e := range errs {
		b.WriteString(e.Error())
		b.WriteString("; ")
	}
	return b.String()
}
