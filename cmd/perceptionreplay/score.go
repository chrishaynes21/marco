package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Coverage and accuracy are two different failures and this file refuses to average them.
//
// # The defect this file exists because of
//
// A report said "189 frames named an entry; 43 of those named the labelled row (23%)" and the
// two numbers did not share a denominator: WithSelection counted every frame on which the
// belief named something, labelled or not, while CorrectSelection counted only labelled
// frames. The percentage was therefore a ratio between two different populations, and it was
// read — by its own author — as one failure when it is two:
//
//	COVERAGE   how often Marco names a row at all. Low coverage is slow learning.
//	ACCURACY   how often a named row is the right one. Low accuracy CORRUPTS THE GRAPH,
//	           because a wrong row is retained as a move between rows that never happened.
//
// They want opposite responses. Coverage is improved by reading more frames; accuracy is
// improved by reading fewer. A single percentage cannot be optimised without knowing which of
// the two it is made of, and a change that trades one for the other looks like progress in
// whichever direction the single number happens to move.
//
// So every number below carries its own denominator, and the wrong namings are partitioned by
// CAUSE rather than counted. The partition is the part that decides what to work on: a reading
// that named a row the identified list does not even contain is a failure of which list was
// chosen, and no amount of work on the reader can fix it.

// frameObservation is what one replayed frame's belief said, kept so the scoring can run once
// over the whole session rather than inside the frame loop.
//
// Kept rather than counted because the interesting questions are about SEQUENCES — how long
// after the pixels changed did the belief agree — and a counter cannot answer those. 864 of
// these is a few tens of kilobytes.
type frameObservation struct {
	Index int
	AtMS  int64
	// Labelled says the annotation claims a selected row on this frame, and LabelledY
	// where it is. The three fields below are only meaningful when it is true.
	Labelled  bool
	LabelledY float64
	// RowGrouped says grouping put the labelled row in a stack, and RowIsMember that the
	// active list counts it among its own rows. ListMembers is how many rows that list
	// believes it has.
	//
	// THE COMPLETENESS FUNNEL. A proposal covers the labelled row on every labelled frame
	// of this corpus, so a frame that cannot name it lost the row after that, and these
	// say where: grouping left it out of every stack, the tracked list never took it as a
	// member, or the list published is a different list. Counting them apart is the
	// difference between "the detector is weak" and "the list is incomplete", which want
	// opposite work.
	// RowShaped says a measurable shape was found at the labelled row's height IN THE
	// LIST'S OWN COLUMN, which is the first rung of the funnel and the one the
	// whole-screen discovery number cannot speak to.
	RowShaped               bool
	RowGrouped, RowIsMember bool
	ListMembers             int
	// RowStack is how many rows were in the stack grouping put the labelled row in, and
	// RowStackHolds how many of the active list's members are in that same stack.
	//
	// THE QUESTION THAT DECIDES THE REPAIR. If the labelled row is in a stack that holds
	// none of the list's members, the menu has been SPLIT and the list is one piece of it.
	// If it is in the same stack as the list's members, grouping found it and the list
	// never took it. Those are grouping and membership, and they are not the same work.
	RowStack, RowStackHolds int
	// Status and Reading are the belief's own two words, kept apart: the first says
	// whether a list was identified, the second whether a row in it could be read. A
	// frame that named nothing is silent for one of those two reasons and they are not
	// the same problem.
	Status, Reading string
	// Selected is the entry index inside the list, or -1.
	Selected int
	// NamedY is the named entry's own vertical centre, normalised. The ENTRY's, never the
	// list's: scoring against the list envelope is the measurement error recorded in
	// namesRow, and it scored 154 of 155 for a reader that was mostly wrong.
	NamedY float64
	// ListTop and ListBottom are the identified list's vertical extent, which is what
	// decides whether the labelled row was inside the list being read at all.
	ListTop, ListBottom float64
	Entries             int
	// NamedHeight is the named ENTRY's own height, kept so the scoring can check that
	// what it scored was an entry.
	//
	// THE MEASUREMENT TRAP, made checkable. A previous version of this score was handed
	// the identified LIST's envelope instead of the named entry's region, and the list in
	// question spanned three quarters of the screen — so every labelled row on the frame
	// was inside it and the measurement reported 154 of 155 for a reader that was mostly
	// wrong. The repair was to score the entry, and nothing held the repair in place; the
	// mutation gate put the envelope back and every floor still passed.
	NamedHeight float64
}

// selectionScore is the split, with every denominator stated.
type selectionScore struct {
	// COVERAGE. Labelled is every frame the annotation says carried a selection;
	// Named is how many of those the belief named an entry on. That ratio and no other
	// is the coverage.
	Labelled, Named int
	// Why the rest were silent, partitioned. NoList is the frames where no list was
	// identified — ambiguous or unknown — so the reader was never asked. Unread is the
	// frames where a list WAS identified and no row in it could be read. One is work on
	// disambiguation, the other work on the reader, and they are not interchangeable.
	NoList, Unread int
	// ACCURACY, over Named. Right is the named entry's region sitting on the labelled
	// row, to within half the labelled pitch — the same tolerance namesRow applies.
	Right int
	// The wrong namings, partitioned by HOW wrong. OffByOne is the neighbouring row,
	// which is the error a confirmation rule can plausibly fix; OffByMore is further
	// away, which is not a reading error at all.
	OffByOne, OffByMore int
	// OutsideList is how many of the wrong namings happened while the labelled row was
	// not inside the identified list's own extent.
	//
	// THE NUMBER THAT SAYS WHERE THE WORK IS. A list that does not contain the row the
	// person is looking at cannot name it however well the reader works: every reading
	// inside it is wrong before the reader is consulted. Counted alongside the off-by
	// split rather than instead of it, because the two cuts are independent — a reading
	// can be one row out inside the right list, or one row out because the list starts
	// three rows down.
	OutsideList int
	// InsideNamed and InsideRight are the namings made while the labelled row WAS inside
	// the list being read, and how many of those were right.
	//
	// THE READER'S OWN SCORE, separated from the disambiguator's. Accuracy over every
	// naming answers "can the graph trust what it is told", which is the number that
	// matters for the graph and tells you nothing about where to work. This answers "when
	// the right list is in front of it, can the reader read it" — and the two came out 68
	// points apart, which is the whole reason the single figure was misleading enough to
	// be reported as a reader failure.
	InsideNamed, InsideRight int
	// THE COMPLETENESS FUNNEL, over the labelled frames, in the order a row is lost in.
	//
	// Grouped is how many had the labelled row in some stack; Member, how many had it
	// among the active list's own rows; Inside, how many had it inside the published
	// extent. Each is a subset of the one before it on this corpus, because a proposal
	// covers the labelled row on every labelled frame.
	//
	// WhileNamed and WhileNotMember cut the same frames the other way: how many namings
	// were made while the labelled row WAS a member of the list being read, and how many
	// while it was not. The second is the number that says a selection claim never had a
	// chance, and it is the one a list that is merely INCOMPLETE produces — which is a
	// different failure from reading the wrong list and a different failure again from
	// reading the right list badly.
	// Split is labelled frames where the row was in a stack that holds none of the list's
	// members, and Dropped where it was in the SAME stack as them and still not a member.
	// Shaped is labelled frames with a shape at the row's height in the list's column.
	Shaped                         int
	Split, Dropped                 int
	Grouped, Member, Inside        int
	WhileNamed, WhileNotMember     int
	NotMemberRight, NotMemberWrong int
	// MembersSeen is how many rows the active list believed it had, as a histogram over
	// labelled frames. A list that is sometimes six rows and sometimes nine is not a
	// list whose entry count can be trusted for anything.
	MembersSeen map[int]int
	// NamedRight and NamedWrong are what the reader rested its claim on, in its own
	// words, split by whether the claim was right.
	//
	// THE QUESTION AN ABSTENTION RULE HAS TO ANSWER: is there anything in the evidence
	// itself that separates a claim about a row the list does not contain from a claim
	// about one it does? If the wrong ones rest on weaker evidence than the right ones,
	// the rule is in the evidence. If they rest on the same evidence, nothing the reader
	// can see distinguishes them and the rule has to come from the list.
	NamedRight, NamedWrong map[string]int
	// Silent is why the reader said nothing, in the READER'S OWN WORDS, on the frames
	// where a list was identified and no entry was named.
	//
	// `Unread` counts those frames and cannot say which failure they are, and the two it
	// conflates want opposite work: a reader that looked at the list and found nothing
	// standing out has read it correctly if nothing was selected, where a reader that could
	// not assemble enough of a list to have a middle has not read it at all. Keyed on the
	// outcome the reader published, so this cannot disagree with the belief.
	//
	// It became the question that mattered the moment the published extent stopped being a
	// bucket: handed the whole screen the reader always found some spike, and handed the
	// list itself it went quiet on nine frames in ten.
	Silent map[string]int
	// SilentEntries is how many entries the reader had in hand on those frames, as a
	// histogram.
	//
	// THE OTHER HALF OF THE SAME QUESTION, and the half that turned out to matter. The
	// reader is given the rows of THIS FRAME that fall inside the identified list, and a
	// detector that found three rows of a nine-row menu leaves it three rows to find a
	// middle in. "No row stands out from its siblings" over three siblings is not the same
	// failure as the same sentence over nine, and the outcome alone cannot tell them apart.
	SilentEntries map[int]int
	// NamedAsTallAsList is how many namings were scored against a region at least half as
	// tall as the whole list. An entry is not half a list, so any of these means the score
	// is being computed against the wrong rectangle — see frameObservation.NamedHeight.
	NamedAsTallAsList int
	// Offsets is the signed distance from the labelled row, in labelled pitches, as a
	// histogram. Positive means the belief named a row BELOW the labelled one.
	//
	// The one view that distinguishes noise from a constant offset, which are different
	// defects with different repairs: a spread around zero is a reader that cannot tell
	// neighbours apart, and a spike at -3 is a list that begins three rows too low.
	Offsets map[int]int
	// Lists is every identified list that ever named a row, by its own vertical extent,
	// with how often it was right.
	//
	// THE LINE THAT SAYS WHOSE FAULT IT IS. The off-by histogram says how wrong a naming
	// was; this says which list was being read when it was. A corpus where one extent
	// accounts for most of the namings and almost none of the right ones has a
	// disambiguation problem, and no work on the reader touches it.
	Lists map[string]*listScore
	// Held counts the namings that came from the reader's hold path — no readable
	// evidence on that frame, the previous claim repeated — and HeldRight how many of
	// those were right. A claim held over an unreadable frame is the one kind of wrong
	// naming the reader produces on purpose, so it is measured and not assumed small.
	Held, HeldRight int
}

// listScore is one identified list's record of naming rows.
type listScore struct {
	Top, Bottom  float64
	Entries      int
	Named, Right int
	// Outside is how many of its namings happened while the labelled row was not inside
	// this list at all, which is the case no reader can recover from.
	Outside int
}

// rate is a percentage with its denominator guarded, so an empty session prints 0 and not NaN.
func rate(n, d int) float64 {
	if d <= 0 {
		return 0
	}
	return 100 * float64(n) / float64(d)
}

// scoreSelection runs the whole session's trace against the labels.
func scoreSelection(trace []frameObservation, ann *annotationFile) selectionScore {
	s := selectionScore{Offsets: map[int]int{}, Silent: map[string]int{},
		SilentEntries: map[int]int{}, MembersSeen: map[int]int{},
		NamedRight: map[string]int{}, NamedWrong: map[string]int{}, Lists: map[string]*listScore{}}
	rows := ann.labelledRows()
	pitch := labelledPitch(rows)
	if pitch <= 0 {
		return s
	}
	for _, f := range trace {
		state, y, ok := ann.labelAt(f.Index)
		if !ok || state != "selected" {
			// A frame the labels are not confident about scores nothing, in either
			// direction. The annotation marks the blend frames `uncertain` on
			// purpose, and they are exactly the frames a reader is expected to
			// refuse — counting them as missed coverage would charge the reader for
			// its correct answer.
			continue
		}
		s.Labelled++
		// THE FUNNEL, counted before anything about the reading, because it is about
		// the list rather than about what was read from it.
		if f.RowShaped {
			s.Shaped++
		}
		if f.RowGrouped {
			s.Grouped++
		}
		if f.RowIsMember {
			s.Member++
		} else if f.RowGrouped && f.ListMembers > 0 {
			if f.RowStackHolds == 0 {
				s.Split++
			} else {
				s.Dropped++
			}
		}
		if f.ListMembers > 0 {
			s.MembersSeen[f.ListMembers]++
		}
		if f.ListBottom > f.ListTop && y >= f.ListTop && y <= f.ListBottom {
			s.Inside++
		}
		if f.Selected < 0 {
			switch f.Status {
			case string(observe.InterfaceProvisional):
				s.Unread++
				s.Silent[readingClause(f.Reading)]++
				s.SilentEntries[f.Entries]++
			default:
				s.NoList++
			}
			continue
		}
		s.Named++
		if f.RowIsMember {
			s.WhileNamed++
		} else {
			s.WhileNotMember++
		}
		key := fmt.Sprintf("%.3f..%.3f/%d", f.ListTop, f.ListBottom, f.Entries)
		ls := s.Lists[key]
		if ls == nil {
			ls = &listScore{Top: f.ListTop, Bottom: f.ListBottom, Entries: f.Entries}
			s.Lists[key] = ls
		}
		ls.Named++
		if h := f.ListBottom - f.ListTop; h > 0 && f.NamedHeight >= h/2 {
			s.NamedAsTallAsList++
		}
		// Held is the reader's own word for it, carried through the belief's prose.
		// Matched on the phrase rather than inferred from a counter, because the
		// phrase is what the reader actually said and a counter here would be a
		// second opinion about it.
		held := strings.Contains(f.Reading, "still claiming")
		if held {
			s.Held++
		}
		off := int(math.Round((f.NamedY - y) / pitch))
		s.Offsets[off]++
		// Whether this naming had a chance of being right at all, asked before it is
		// scored, so the two populations are cut by the same test in both branches.
		inside := f.ListBottom <= f.ListTop || (y >= f.ListTop && y <= f.ListBottom)
		if inside {
			s.InsideNamed++
		}
		if !f.RowIsMember {
			if math.Abs(f.NamedY-y) <= pitch/2 {
				s.NotMemberRight++
			} else {
				s.NotMemberWrong++
			}
		}
		if math.Abs(f.NamedY-y) <= pitch/2 {
			s.NamedRight[readingClause(f.Reading)]++
		} else {
			s.NamedWrong[readingClause(f.Reading)]++
		}
		if math.Abs(f.NamedY-y) <= pitch/2 {
			s.Right++
			ls.Right++
			if inside {
				s.InsideRight++
			}
			if held {
				s.HeldRight++
			}
			continue
		}
		if off == 1 || off == -1 {
			s.OffByOne++
		} else {
			s.OffByMore++
		}
		// Could the list being read have named this row at all? Its own extent
		// answers that, and a row outside it is a failure of which list was chosen.
		if !inside {
			s.OutsideList++
			ls.Outside++
		}
	}
	return s
}

// readingClause is the reader's own sentence, lifted out of the belief's prose.
//
// The prose is one line carrying the list's whole record and the reading together, which is
// right for a person reading one frame and useless as a tally key. Taken from the prose rather
// than from a second field, for the same reason the publishing arm is: a key that could
// disagree with the sentence a person reads is a key that eventually does.
func readingClause(because string) string {
	const mark = "reading: "
	i := strings.Index(because, mark)
	if i < 0 {
		return "(the belief named no reading)"
	}
	out := because[i+len(mark):]
	if j := strings.Index(out, ";"); j >= 0 {
		out = out[:j]
	}
	return strings.TrimSpace(out)
}

// confirmation is one labelled change of row, and how long the belief took to agree with it.
//
// # Why the ground truth is the annotation and not a press
//
// Because the existing response-latency measurement starts from a press, and a press is not
// when the screen changed. It measures intent-to-belief, which is the sum of the game's own
// repaint and Marco's confirmation delay, and cannot separate them. The annotation says which
// frame the new row was selected on, so starting there measures Marco's half alone — which is
// the half that can be worked on.
type confirmation struct {
	// From and To are the labelled rows, counted from the top of the labelled list.
	From, To int
	// At is the first frame the labels put the new row in place.
	At int
	// SeenAt is the first frame at or after At on which the belief named the new row, and
	// Frames and MS the distance. Never is true when no frame in the session did.
	SeenAt int
	Frames int
	MS     int64
	Never  bool
	// Wrong is how many frames in between named some OTHER row. A gap spent silent is a
	// slower answer; a gap spent naming the row the selection has left is a wrong one,
	// and the graph cannot tell them apart from the moves alone.
	Wrong int
}

// confirmations measures every labelled change of row against the trace.
func confirmations(trace []frameObservation, ann *annotationFile) []confirmation {
	rows := ann.labelledRows()
	pitch := labelledPitch(rows)
	if pitch <= 0 || ann == nil {
		return nil
	}
	// THE LABELLED CHANGES, taken from the intervals in order. Consecutive `selected`
	// intervals naming different heights are a change; an `uncertain` interval between
	// them is the blend and is skipped rather than treated as a change to nothing.
	type change struct {
		from, to int
		at       int
	}
	var changes []change
	prev := -1
	ivs := append([]annotationInterval(nil), ann.Intervals...)
	sort.Slice(ivs, func(i, j int) bool { return ivs[i].From < ivs[j].From })
	for _, iv := range ivs {
		if iv.State != "selected" {
			continue
		}
		o := ordinalOf(rows, iv.Y)
		if o < 0 {
			continue
		}
		if prev >= 0 && o != prev {
			changes = append(changes, change{from: prev, to: o, at: iv.From})
		}
		prev = o
	}
	byIndex := map[int]frameObservation{}
	order := make([]int, 0, len(trace))
	for _, f := range trace {
		byIndex[f.Index] = f
		order = append(order, f.Index)
	}
	sort.Ints(order)

	var out []confirmation
	for _, c := range changes {
		want := rows[c.to]
		conf := confirmation{From: c.from, To: c.to, At: c.at, Never: true}
		var startMS int64 = -1
		for _, idx := range order {
			if idx < c.at {
				continue
			}
			f := byIndex[idx]
			if startMS < 0 {
				startMS = f.AtMS
			}
			if f.Selected >= 0 && math.Abs(f.NamedY-want) <= pitch/2 {
				conf.Never, conf.SeenAt = false, idx
				conf.Frames = countBetween(order, c.at, idx)
				conf.MS = f.AtMS - startMS
				break
			}
			if f.Selected >= 0 {
				conf.Wrong++
			}
		}
		out = append(out, conf)
	}
	return out
}

// countBetween is how many replayed frames lie in [from, to].
func countBetween(order []int, from, to int) int {
	n := 0
	for _, i := range order {
		if i >= from && i <= to {
			n++
		}
	}
	return n
}

// reportSelectionScore prints the split. Every line carries its own denominator.
func reportSelectionScore(s selectionScore, confs []confirmation) {
	if s.Labelled == 0 {
		return
	}
	fmt.Printf("\n  COVERAGE AND ACCURACY, counted separately (they are different failures)\n")
	fmt.Printf("  coverage            %d of %d labelled frames named an entry (%.0f%%)\n",
		s.Named, s.Labelled, rate(s.Named, s.Labelled))
	fmt.Printf("    no list           %d frame(s) where no list was identified, so the\n"+
		"                      reader was never asked\n", s.NoList)
	fmt.Printf("    unreadable        %d frame(s) where a list was identified and no row\n"+
		"                      in it could be read\n", s.Unread)
	fmt.Printf("  accuracy            %d of %d named entries were the labelled row (%.0f%%)\n",
		s.Right, s.Named, rate(s.Right, s.Named))
	wrong := s.Named - s.Right
	fmt.Printf("    confidently wrong %d frame(s) named a row the labels put elsewhere\n"+
		"                      (%.0f%% of every naming) <-- the graph-corrupting one\n",
		wrong, rate(wrong, s.Named))
	fmt.Printf("      off by one      %d   the neighbouring row\n", s.OffByOne)
	fmt.Printf("      off by more     %d   two or more rows away\n", s.OffByMore)
	fmt.Printf("    outside the list  %d of those %d: the labelled row is not inside the\n"+
		"                      list being read, so no reading of it could be right\n",
		s.OutsideList, wrong)
	// AND THE READER'S OWN SCORE, which is a different question from the one above.
	if s.NamedAsTallAsList > 0 {
		fmt.Printf("    SCORED WRONG      %d naming(s) were scored against a region at "+
			"least half as\n                      tall as the whole list. An entry is "+
			"not half a list, so these\n                      numbers are measuring "+
			"the envelope and not the entry\n", s.NamedAsTallAsList)
	}
	// ── LIST COMPLETENESS, which is where a wrong selection now comes from ────────
	fmt.Printf("  LIST COMPLETENESS, the labelled row's way from a proposal to a list\n")
	fmt.Printf("    a shape at all    %d of %d labelled frames had a measurable shape at the\n"+
		"                      labelled row's height IN THE LIST'S OWN COLUMN (%.0f%%)\n"+
		"                      (the discovery figure above is over the whole screen,\n"+
		"                      which anything at that height satisfies)\n",
		s.Shaped, s.Labelled, rate(s.Shaped, s.Labelled))
	fmt.Printf("    in a stack        %d of %d had it in a group grouping made, in that\n"+
		"                      same column (%.0f%%)\n",
		s.Grouped, s.Labelled, rate(s.Grouped, s.Labelled))
	fmt.Printf("    a member          %d of %d had it among the active list's OWN rows\n"+
		"                      (%.0f%%)  <-- a claim about a row the list does not\n"+
		"                      have is wrong before the reader is consulted\n",
		s.Member, s.Labelled, rate(s.Member, s.Labelled))
	fmt.Printf("    inside the extent %d of %d had it inside the published extent (%.0f%%)\n",
		s.Inside, s.Labelled, rate(s.Inside, s.Labelled))
	if len(s.MembersSeen) > 0 {
		ns := make([]int, 0, len(s.MembersSeen))
		for n := range s.MembersSeen {
			ns = append(ns, n)
		}
		sort.Ints(ns)
		var hist []string
		for _, n := range ns {
			hist = append(hist, fmt.Sprintf("%d:%d", n, s.MembersSeen[n]))
		}
		fmt.Printf("    rows it believes  %s\n"+
			"                      (how many rows the active list thought it had)\n",
			strings.Join(hist, "  "))
	}
	fmt.Printf("    where it was lost %d frame(s) had it in a stack holding NONE of the\n"+
		"                      list's members, so the menu was split and the list is one\n"+
		"                      piece of it; %d had it in the SAME stack as them and the\n"+
		"                      list still never took it\n", s.Split, s.Dropped)
	fmt.Printf("    named anyway      %d naming(s) were made while the labelled row was NOT\n"+
		"                      a member of the list being read; %d of them happened to\n"+
		"                      be right and %d were wrong  <-- the incomplete-list error\n",
		s.WhileNotMember, s.NotMemberRight, s.NotMemberWrong)
	fmt.Printf("    named fairly      %d naming(s) were made while it WAS a member\n",
		s.WhileNamed)

	// ── WHAT THE CLAIMS RESTED ON, right ones against wrong ones ──────────────────
	byWhy := func(label string, m map[string]int) {
		if len(m) == 0 {
			return
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
		fmt.Printf("    %-17s\n", label)
		for _, k := range keys {
			fmt.Printf("                      %-52s %d\n", trim(k, 52), m[k])
		}
	}
	byWhy("rested on, right", s.NamedRight)
	byWhy("rested on, WRONG", s.NamedWrong)

	fmt.Printf("  the reader alone    %d of %d naming(s) made while the labelled row WAS\n"+
		"                      inside the list being read were right (%.0f%%)\n"+
		"                      <-- what the reader does when handed the right list\n",
		s.InsideRight, s.InsideNamed, rate(s.InsideRight, s.InsideNamed))
	if len(s.Silent) > 0 {
		keys := make([]string, 0, len(s.Silent))
		for k := range s.Silent {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return s.Silent[keys[i]] > s.Silent[keys[j]] })
		fmt.Printf("    why silent        the reader's own outcome on the %d frame(s) that\n"+
			"                      identified a list and named nothing:\n", s.Unread)
		for _, k := range keys {
			fmt.Printf("                      %-52s %d\n", trim(k, 52), s.Silent[k])
		}
		ns := make([]int, 0, len(s.SilentEntries))
		for n := range s.SilentEntries {
			ns = append(ns, n)
		}
		sort.Ints(ns)
		var hist []string
		for _, n := range ns {
			hist = append(hist, fmt.Sprintf("%d:%d", n, s.SilentEntries[n]))
		}
		fmt.Printf("      entries in hand %s\n"+
			"                      (how many rows of the list the detector found on\n"+
			"                      those frames; three siblings have no middle)\n",
			strings.Join(hist, "  "))
	}
	if s.Held > 0 {
		fmt.Printf("    held over a gap   %d naming(s) repeated a previous claim on an\n"+
			"                      unreadable frame; %d of those were right\n",
			s.Held, s.HeldRight)
	} else {
		fmt.Printf("    held over a gap   0 naming(s) came from the hold path\n")
	}
	// THE SHAPE OF THE ERROR, which is what says whether this is a reader or a list.
	if len(s.Offsets) > 0 {
		keys := make([]int, 0, len(s.Offsets))
		for k := range s.Offsets {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		fmt.Printf("    signed offset     rows below the labelled row, as a histogram:\n" +
			"                      ")
		for _, k := range keys {
			fmt.Printf("%+d:%d  ", k, s.Offsets[k])
		}
		fmt.Printf("\n                      (a spread around 0 is a reader that cannot tell\n" +
			"                      neighbours apart; a spike elsewhere is the wrong list)\n")
	}
	// AND WHICH LIST WAS BEING READ. Sorted by how much of the naming each accounts for,
	// because the question is where the namings came from and not how many lists there were.
	if len(s.Lists) > 0 {
		keys := make([]string, 0, len(s.Lists))
		for k := range s.Lists {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return s.Lists[keys[i]].Named > s.Lists[keys[j]].Named
		})
		fmt.Printf("    the list it read  extent y..y   entries  named  right  outside\n")
		for i, k := range keys {
			if i >= 6 {
				fmt.Printf("                      ... and %d more\n", len(keys)-i)
				break
			}
			l := s.Lists[k]
			fmt.Printf("                      %.3f..%.3f  %-7d  %-5d  %-5d  %d\n",
				l.Top, l.Bottom, l.Entries, l.Named, l.Right, l.Outside)
		}
	}
	reportConfirmations(confs)
}

// reportConfirmations prints how long the belief took to agree with each labelled change.
func reportConfirmations(confs []confirmation) {
	if len(confs) == 0 {
		return
	}
	var seen []int64
	var frames []int
	never, wrong := 0, 0
	for _, c := range confs {
		if c.Never {
			never++
			wrong += c.Wrong
			continue
		}
		seen = append(seen, c.MS)
		frames = append(frames, c.Frames)
		wrong += c.Wrong
	}
	fmt.Printf("  confirmation lag    %d labelled change(s) of row; %d were ever named,\n"+
		"                      %d never were\n", len(confs), len(seen), never)
	if len(seen) > 0 {
		sort.Slice(seen, func(i, j int) bool { return seen[i] < seen[j] })
		sort.Ints(frames)
		fmt.Printf("                      from the frame the labels put the new row in\n"+
			"                      place: min %dms, median %dms, max %dms\n"+
			"                      (%d, %d, %d replayed frames)\n",
			seen[0], seen[len(seen)/2], seen[len(seen)-1],
			frames[0], frames[len(frames)/2], frames[len(frames)-1])
	}
	fmt.Printf("                      %d frame(s) in those gaps named some OTHER row, which\n"+
		"                      is a wrong answer rather than a slow one\n", wrong)
}
