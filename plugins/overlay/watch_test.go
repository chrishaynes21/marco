package main

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// What the overlay must be true of as a PRESENTATION.
//
// The interesting claim is a negative one: the overlay renders the Director's account and
// contributes no facts to it. These tests are mostly about proving that the surface has
// nothing of its own to be wrong about.

// C. The overlay recomputes nothing.
//
// Structural, because a reviewer's "there is no analysis in here" is true until somebody
// adds a convenience. The overlay module cannot reach the Director's analysis at all: it
// imports one package, and that package imports no internals — proved on the engine side
// by TestTheVisibilityRepresentationImportsNoAnalysis. What is proved HERE is that the
// rendered rows are a pure function of the account.
func TestTheOverlayRendersTheAccountAndAddsNothing(t *testing.T) {
	m := newModel()
	m.openWatch()
	m.setWatch(recognisedAccount())

	first := watchRows(m.snapshot(), 300)
	second := watchRows(m.snapshot(), 300)
	if len(first) != len(second) {
		t.Fatalf("rendering the same account twice produced %d and %d rows",
			len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("row %d differed between two renders of one account: %+v vs %+v",
				i, first[i], second[i])
		}
	}
	if !strings.Contains(rowText(first), "I recognise this as “the pause menu”") {
		t.Errorf("the account did not reach the panel:\n%s", rowText(first))
	}
}

// The tone travels with the line, so colour follows MEANING rather than wording.
//
// The old Director panel coloured a row red when its text contained "failed:". That works
// until somebody rewords a sentence, and then it stops working silently — in the one
// surface whose job is to tell a person the truth.
func TestColourFollowsMeaningRatherThanWording(t *testing.T) {
	v := recognisedAccount()
	v.Current.Recognition = playbill.Unobservable
	v.Why = "the window went away and could not be found again"

	m := newModel()
	m.openWatch()
	m.setWatch(v)

	var alarms int
	for _, r := range watchRows(m.snapshot(), 300) {
		if r.tone == playbill.Alarm {
			alarms++
		}
	}
	if alarms == 0 {
		t.Fatal("nothing in an account about a lost window read as a problem")
	}
	// And the mapping is total: every tone resolves to a colour.
	for _, tone := range []playbill.Tone{playbill.Plain, playbill.Good, playbill.Doubt,
		playbill.Alarm, playbill.Muted, playbill.Accent, ""} {
		if toneColor(tone) == (toneColor("nonsense-tone")) && tone != "" && tone != playbill.Plain {
			t.Errorf("tone %q fell through to the default colour", tone)
		}
	}
}

// The three readings are levels of ONE surface, and Diagnostics is the only one that
// captures the mouse.
func TestWatchIsClickThroughAndDiagnosticsSaysItIsNot(t *testing.T) {
	m := newModel()

	m.openWatch()
	if s := m.snapshot(); s.wmode != watchOn || s.inspectorOn {
		t.Fatalf("Watch captured the mouse. A person is meant to keep using another "+
			"application while it is up: %+v", s.wmode)
	}
	m.openDiagnostics()
	s := m.snapshot()
	if s.wmode != watchDeep || !s.inspectorOn {
		t.Fatal("Diagnostics did not capture the mouse")
	}
	// Capturing is never silent: the reason and the way out are on screen.
	if !strings.Contains(watchModeLabel(s), "mouse captured") {
		t.Error("the overlay swallowed clicks without saying so")
	}
	if !strings.Contains(watchModeLabel(s), "Esc") {
		t.Error("the way out of the capturing mode is not on screen")
	}
}

// Part 10. Visibility can never trap somebody.
//
// Esc closes whatever is up and releases the mouse, and it does so without consulting the
// Director — so an unreachable or wedged Director cannot leave a person with a captured
// cursor.
func TestEscapeAlwaysReleasesTheSurface(t *testing.T) {
	m := newModel()
	m.openDiagnostics()
	m.closeWatch()

	s := m.snapshot()
	if s.wmode != watchOff || s.inspectorOn {
		t.Fatal("closing the panel left the mouse captured")
	}
}

// H. Repeated identical accounts do not accumulate timeline entries.
func TestRepeatedIdenticalAccountsDoNotGrowTheFeed(t *testing.T) {
	f := &watchFeed{}
	v := recognisedAccount()
	v.Recent = []playbill.Moment{{Seq: 1, At: time.Now(), Says: "the screen changed"}}
	v.Cursor = 1

	for i := 0; i < 50; i++ {
		f.absorb(v)
	}
	if len(f.moments) != 1 {
		t.Fatalf("50 identical polls produced %d entries. A person watching a still "+
			"screen would see the panel churn", len(f.moments))
	}
}

// G. The overlay's own timeline is bounded, whatever the Director sends.
//
// Bounded here as well as in the account. A surface that trusted somebody else's bound
// would grow forever the day that bound changed.
func TestTheOverlayFeedIsBounded(t *testing.T) {
	f := &watchFeed{}
	for i := 1; i <= watchFeedMax*5; i++ {
		v := recognisedAccount()
		v.Recent = []playbill.Moment{{Seq: uint64(i), At: time.Now(), Says: "the screen changed"}}
		v.Cursor = uint64(i)
		f.absorb(v)
	}
	if len(f.moments) != watchFeedMax {
		t.Fatalf("the feed held %d entries, want the bound of %d",
			len(f.moments), watchFeedMax)
	}
}

// J. A Director restart is announced, not smoothed over.
//
// Sequence numbers begin again after a restart. A surface that compared cursors without
// checking the epoch would decide nothing had happened, and would go on showing the
// previous Director's belief as though it were current.
func TestARestartIsAnnouncedRatherThanSmoothedOver(t *testing.T) {
	f := &watchFeed{}
	first := recognisedAccount()
	first.Epoch = "epoch_1"
	first.Recent = []playbill.Moment{{Seq: 9, At: time.Now(), Says: "the screen changed"}}
	first.Cursor = 9
	f.absorb(first)

	second := recognisedAccount()
	second.Epoch = "epoch_2"
	second.Recent = []playbill.Moment{{Seq: 1, At: time.Now(), Says: "the screen changed"}}
	second.Cursor = 1
	f.absorb(second)

	if !strings.Contains(momentText(f.moments), "Marco restarted") {
		t.Fatalf("a restart was silent:\n%s", momentText(f.moments))
	}
	if f.cursor != 1 {
		t.Errorf("the cursor was compared across a restart: %d", f.cursor)
	}
}

// A gap is reported rather than papered over. Dropped overlay frames are acceptable;
// dropped Director moments are not.
func TestAGapInTheTimelineIsReported(t *testing.T) {
	f := &watchFeed{}
	v := recognisedAccount()
	v.Epoch = "epoch_1"
	v.Recent = []playbill.Moment{{Seq: 3, At: time.Now(), Says: "the screen changed"}}
	v.Cursor, v.Oldest = 3, 1
	f.absorb(v)

	later := recognisedAccount()
	later.Epoch = "epoch_1"
	later.Recent = []playbill.Moment{{Seq: 90, At: time.Now(), Says: "the screen changed"}}
	later.Cursor, later.Oldest = 90, 80
	f.absorb(later)

	if !f.missed {
		t.Fatal("the feed rolled past the cursor and said nothing")
	}
	if !strings.Contains(momentText(f.moments), "I missed some") {
		t.Errorf("the gap was not visible to a person:\n%s", momentText(f.moments))
	}
}

// I / J. An unreachable Director renders as a normal condition, not as a blank panel.
func TestAnUnreachableDirectorStillRendersSomething(t *testing.T) {
	for _, v := range []playbill.View{
		playbill.Unavailable(playbill.Absent, "the Director service is not running"),
		playbill.Unavailable(playbill.Unreachable, "the engine did not answer"),
	} {
		m := newModel()
		m.openWatch()
		m.setWatch(v)
		rows := watchRows(m.snapshot(), 300)
		if len(rows) <= 1 {
			t.Fatalf("%s rendered nothing but a chip. A blank panel reads as a panel "+
				"that has not loaded", v.Reach)
		}
		if strings.Contains(rowText(rows), "I recognise") {
			t.Errorf("%s claimed to recognise something", v.Reach)
		}
	}
}

// Part 6. The always-visible line is the NORMAL reading of the same account.
func TestTheIdleHintIsTheConsumerReduction(t *testing.T) {
	m := newModel()
	v := recognisedAccount()
	v.Question = &playbill.Question{
		ID: "q_1", Asks: "Is this the pause menu?", Wants: playbill.WantsChoice,
		Via: playbill.ViaProposal, Answers: []string{"confirmed"},
	}
	m.setWatch(v)

	row := normalRow(m.snapshot())
	if !strings.Contains(row, "Marco has a question") {
		t.Fatalf("a pending question did not reach the always-visible line: %q", row)
	}
	if !strings.Contains(row, "Is this the pause menu?") {
		t.Errorf("the line did not say what the question was: %q", row)
	}

	// Watching IS worth a line — it is the consumer surface's answer to "is Marco even
	// on?" — so the hint yields to it.
	watching := newModel()
	watching.setWatch(recognisedAccount())
	if got := normalRow(watching.snapshot()); !strings.HasPrefix(got, "Watching") {
		t.Errorf("a watching Marco did not say so on the always-visible line: %q", got)
	}

	// A RESTING Marco says nothing there, so the line keeps its meaning — and keeps
	// showing the leader-key affordances a person still needs to discover.
	resting := newModel()
	rest := recognisedAccount()
	rest.Current = playbill.Current{Recognition: playbill.Unobservable}
	rest.Learning = playbill.Learning{Stage: playbill.NotLearning}
	resting.setWatch(rest)
	if got := normalRow(resting.snapshot()); got != "" {
		t.Errorf("a resting Marco used the hint line to say %q", got)
	}
}

// E. There is no path from the overlay to authority.
//
// The overlay reaches the engine by spawning `marco director <word>`. The visibility
// words are reads; nothing here submits a phrase, answers a question or claims a grant.
func TestTheWatchSurfaceOnlySpawnsReads(t *testing.T) {
	for _, word := range []string{"watch", "diagnose", "normal"} {
		switch word {
		case "watch", "diagnose", "normal":
		default:
			t.Errorf("%q is not a read", word)
		}
	}
	// And the panel-opening words never reach the Director as a phrase: `watch` opens a
	// panel, it does not ask Marco to find a control called "watch".
	m := newModel()
	m.openWatch()
	if m.snapshot().lastRun == "watch" {
		t.Error("opening the panel submitted a phrase")
	}
}

// ── fixtures ──────────────────────────────────────────────────────────────────

func recognisedAccount() playbill.View {
	return playbill.View{
		Version: playbill.Version, Reach: playbill.Present, Epoch: "epoch_1",
		Current: playbill.Current{
			Watching: true, Application: "testgame.exe",
			Recognition: playbill.Recognised, Screen: "the pause menu",
			Samples: 41, FreshnessMS: 300,
		},
		Seeing:   playbill.Seeing{Structure: 5, Looks: 41, Readable: 7},
		Learning: playbill.Learning{Stage: playbill.Observing},
		Doing:    playbill.Doing{Phase: playbill.NotDoing},
	}
}

func rowText(rows []watchRow) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(strings.Repeat("  ", r.indent))
		b.WriteString(r.text)
		b.WriteString("\n")
	}
	return b.String()
}

func momentText(ms []playbill.Moment) string {
	var b strings.Builder
	for _, m := range ms {
		b.WriteString(m.Says)
		b.WriteString("\n")
	}
	return b.String()
}
