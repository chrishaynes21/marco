package learnstate_test

// A RECORDING'S OWN ACCOUNT OF ITSELF.
//
// The thing this replaces is a window doing arithmetic over the flags it passed: "five seconds
// have gone by, so it must be recording now". That is a true statement about a clock and says
// nothing about whether the demonstration is worth keeping — and the question that decides
// whether it is worth keeping, has the result of the click been read, was answered by printing
// advice and hoping. So every phase here has to mean something the recorder MEASURED, and every
// one of them has to have a sentence, because a phase with no sentence is a window that goes
// blank at the moment somebody is looking at it.

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/buildid"
	"github.com/chaynes-simpleclouds/marco/internal/director/learnstate"
)

// Every situation the recorder can be in, and the phase it is.
func TestEverySituationARecordingCanBeInIsAPhase(t *testing.T) {
	for _, c := range []struct {
		what                                  string
		startRead                             bool
		presses, watched, captured, unchecked int
		want                                  learnstate.Phase
	}{
		// NOTHING READ AND NOTHING PRESSED: the stage people skip, and the one
		// whose absence produces an Act that can only ever wait.
		{"watching, nothing read yet", false, 0, 0, 0, 0, learnstate.Reading},
		{"the starting screen has been read", true, 0, 0, 0, 0,
			learnstate.Ready},
		// A PRESS, AND ITS EVIDENCE IN FLIGHT. The ordinary case for most of a
		// second after every click.
		{"one press, placed, nothing captured", true, 1, 1, 0, 0,
			learnstate.Checking},
		{"one press captured", true, 1, 1, 1, 0, learnstate.Captured},
		{"two presses, one captured", true, 2, 2, 1, 0, learnstate.Checking},
		{"two presses, both captured", true, 2, 2, 2, 0, learnstate.Captured},
		// AND A PRESS WHOSE OUTCOME IS MEASURED AND NOT THERE. Its own phase for
		// the reason `Blind` is one: the five readings have arrived, the answer
		// cannot change, and `Checking` is a sentence that says waiting is all
		// there is to do. This is the owner’s Rocket League recording — one
		// captured press, one measured and empty, and twenty-nine seconds of
		// "wait for the result to settle" with nothing left to wait for.
		{"one press measured and empty", true, 1, 1, 0, 1,
			learnstate.Unchecked},
		{"one captured, then one measured and empty", true, 2, 2, 1, 1,
			learnstate.Unchecked},
		// AND SOMETHING STILL BEING MEASURED OUTRANKS BOTH OF THOSE, because it
		// is the press in flight and waiting is the thing to do about it.
		{"one measured and empty, then one in flight", true, 2, 2, 0, 1,
			learnstate.Checking},
		// A PRESS ON NOTHING. Its own phase, because waiting cannot fix it and
		// `Checking` is a sentence that says waiting is all there is to do.
		{"one press on nothing", true, 1, 0, 0, 0, learnstate.Blind},
		// AND AN OUTCOME IN FLIGHT OUTRANKS A PRESS THAT LANDED ON NOTHING: a
		// click beside a control and then one on it is one session, and the phase
		// is about the press being measured now.
		{"one blind press, then one being measured", true, 2, 1, 0, 0,
			learnstate.Checking},
		{"one blind press, then one captured", true, 2, 1, 1, 0,
			learnstate.Captured},
		// AND A PRESS BEFORE ANYTHING WAS READ IS STILL A PRESS. The starting
		// screen going unread is reported by what was SAVED, not by pretending the
		// recording has not moved on.
		{"pressed before the screen was read", false, 1, 1, 1, 0,
			learnstate.Captured},
	} {
		got := learnstate.PhaseOf(c.startRead, c.presses, c.watched, c.captured,
			c.unchecked)
		if got != c.want {
			t.Errorf("%s: phase is %q, want %q", c.what, got, c.want)
		}
	}
}

// AND EVERY PHASE HAS A SENTENCE, except the one where what was LEARNED is the headline.
func TestEveryPhaseIsSaidAsSomethingThePersonIsDoing(t *testing.T) {
	for _, p := range []learnstate.Phase{
		learnstate.Preparing, learnstate.Reading, learnstate.Ready,
		learnstate.Checking, learnstate.Blind, learnstate.Unchecked,
		learnstate.Captured,
	} {
		said := learnstate.Doing(p, "ArkAscended.exe")
		if said == "" {
			t.Errorf("phase %q has no sentence, so a window showing it goes "+
				"blank", p)
		}
		// AND IT IS NOT THE PHASE WORD. `checking` is the recorder's vocabulary
		// and a person who has just clicked something is owed a sentence about
		// the thing they did.
		if said == string(p) {
			t.Errorf("phase %q is shown as its own name", p)
		}
	}
	if said := learnstate.Doing(learnstate.Ended, "ArkAscended.exe"); said != "" {
		t.Errorf("an ended recording still says what it is doing: %q", said)
	}
	// THE FOUR THE REDESIGN NAMED, each in its own phase.
	for _, c := range []struct {
		phase learnstate.Phase
		says  string
	}{
		{learnstate.Reading, "Reading the starting screen"},
		{learnstate.Ready, "Click the control once"},
		{learnstate.Checking, "Checking what changed"},
		{learnstate.Captured, "Result captured"},
	} {
		if said := learnstate.Doing(c.phase, "ark"); !strings.Contains(said, c.says) {
			t.Errorf("phase %q says %q, which does not mention %q",
				c.phase, said, c.says)
		}
	}
	// AND PREPARING NAMES THE APPLICATION, because a window that says "bringing the
	// window forward" names nothing to go and look at.
	if said := learnstate.Doing(learnstate.Preparing, "ark"); !strings.Contains(said, "ark") {
		t.Errorf("the preparation does not name the application: %q", said)
	}
	if said := learnstate.Doing(learnstate.Preparing, ""); said == "" {
		t.Error("a preparation with no application named says nothing at all")
	}
}

// A STREAM IS WRITTEN ON CHANGE AND READ BACK AS THE LATEST STATE.
//
// On change because this is called from the recorder's own loop, which shares a thread with
// nothing but runs beside a low-level input hook — a write per look would put file I/O on that
// cadence. And the seconds remaining are excluded from the comparison on purpose: they tick by
// construction, so counting them as a change would emit every event there is.
func TestARecordingSaysWhatChangedAndNotWhatDidNot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "learn.jsonl")
	w, err := learnstate.NewWriter(path, "the audio tab", "ArkAscended.exe")
	if err != nil {
		t.Fatal(err)
	}
	w.Say(learnstate.Event{Phase: learnstate.Reading, Left: 60})
	w.Say(learnstate.Event{Phase: learnstate.Reading, Left: 59})
	w.Say(learnstate.Event{Phase: learnstate.Reading, Left: 58})
	w.Say(learnstate.Event{Phase: learnstate.Ready, Left: 57})
	w.Say(learnstate.Event{Phase: learnstate.Checking, Presses: 1, Watched: 1,
		Missing: "nothing elsewhere on the window appeared and stayed", Left: 56})
	w.Say(learnstate.Event{Phase: learnstate.Captured, Presses: 1, Watched: 1,
		Captured: 1, Left: 55})
	w.Close()

	all, err := learnstate.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("%d event(s) written for four distinct states: %+v", len(all), all)
	}
	for i, e := range all {
		if e.Seq != i+1 {
			t.Errorf("event %d is numbered %d", i, e.Seq)
		}
		if e.Name != "the audio tab" || e.Application != "ArkAscended.exe" {
			t.Errorf("event %d does not carry what is being recorded: %+v", i, e)
		}
		if e.At.IsZero() {
			t.Errorf("event %d is not stamped", i)
		}
	}
	// AND THE SECONDS ARE STILL THERE ON THE ONES THAT WERE WRITTEN.
	if all[0].Left != 60 {
		t.Errorf("the first event says %ds remain", all[0].Left)
	}
	if all[2].Missing == "" {
		t.Error("the reason the outcome is missing was dropped")
	}

	// AND THE LATEST IS THE STATE, with `final` false because this one did not end.
	at, final, err := learnstate.Latest(path)
	if err != nil {
		t.Fatal(err)
	}
	if at.Phase != learnstate.Captured || at.Captured != 1 {
		t.Errorf("the latest state is %+v", at)
	}
	if final {
		t.Error("a recording that never said it ended is reported as ended")
	}
}

// AND A RECORDING THAT ENDED SAYS SO, which is how a window tells it from a child that vanished.
func TestARecordingThatEndedIsFinal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "learn.jsonl")
	w, err := learnstate.NewWriter(path, "the audio tab", "ark")
	if err != nil {
		t.Fatal(err)
	}
	w.Say(learnstate.Event{Phase: learnstate.Captured, Presses: 1, Watched: 1,
		Captured: 1})
	w.Say(learnstate.Event{Phase: learnstate.Ended, Final: true,
		Reason: "you ended it", Presses: 1, Watched: 1, Captured: 1})
	w.Close()

	at, final, err := learnstate.Latest(path)
	if err != nil {
		t.Fatal(err)
	}
	if !final || at.Phase != learnstate.Ended || at.Reason != "you ended it" {
		t.Errorf("the end of the recording reads as %+v (final=%v)", at, final)
	}
}

// A PATH NOBODY ASKED FOR IS A WRITER THAT DOES NOTHING, so no caller has to check.
func TestNoPathIsAWriterThatDoesNothing(t *testing.T) {
	w, err := learnstate.NewWriter("", "x", "y")
	if err != nil {
		t.Fatal(err)
	}
	if w != nil {
		t.Fatal("an empty path produced a writer")
	}
	// EVERY METHOD ON IT IS SAFE, which is the whole point.
	w.Say(learnstate.Event{Phase: learnstate.Reading})
	w.Close()
}

// AND A TRUNCATED LAST LINE IS SKIPPED RATHER THAN REFUSED.
//
// A window polling a live recording will sometimes read half a line, and one that errored on it
// would blink at the person watching.
func TestHalfALineIsSkippedAndTheRestIsRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "learn.jsonl")
	w, err := learnstate.NewWriter(path, "x", "y")
	if err != nil {
		t.Fatal(err)
	}
	w.Say(learnstate.Event{Phase: learnstate.Ready})
	w.Close()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"version":1,"phase":"chec`); err != nil {
		t.Fatal(err)
	}
	f.Close()

	at, _, err := learnstate.Latest(path)
	if err != nil {
		t.Fatal(err)
	}
	if at.Phase != learnstate.Ready {
		t.Errorf("half a line displaced the state before it: %+v", at)
	}
}

// THE VERSION MOVES WHEN THE EVENT'S SHAPE DOES.
//
// # Why a test and not a note
//
// Because the note was there and the number did not move. `Version` sat at 1 while three fields
// were added to `Event`, and `omitempty` made the difference invisible: a writer that has never
// heard of `screen_read` omits it, and so does a writer whose value is false. A recorder a day
// old therefore produced a stream a current window read happily and wrongly.
//
// The owner lost a day of recordings to that, and what gave it away was not the version at all
// — it was two fields contradicting each other in the output. The handshake existed and was
// silent, because using it was left to somebody remembering.
//
// So the shape is pinned here. Adding, removing or renaming a field fails this until `Version`
// moves and the list below is brought with it, which is the one thing that makes an older
// writer detectable instead of merely different.
func TestTheStreamVersionMovesWithTheEventsShape(t *testing.T) {
	// EVERY SHAPE THIS FORMAT HAS HAD, by the version that carries it.
	//
	// A table and not one string, because the pair is what has to hold: a shape with
	// no version of its own is a change nobody announced, and a version whose shape is
	// not the one recorded here is a number moved without the thing it describes. A
	// mutation putting `Version` back to 1 survived a test that pinned only the shape.
	shapes := map[int]string{
		1: "version,seq,at,name,application,phase,presses,watched,captured," +
			"missing,left,final,reason",
		2: "version,seq,at,name,application,phase,presses,watched,captured," +
			"unchecked,missing,start_known,screen_read,left,final,reason",
		3: "version,seq,at,name,application,build,phase,presses,watched," +
			"captured,unchecked,missing,start_known,screen_read,left," +
			"final,reason",
	}

	var names []string
	ev := reflect.TypeOf(learnstate.Event{})
	for i := 0; i < ev.NumField(); i++ {
		tag := ev.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			t.Fatalf("%s carries no json tag, so what a reader of an older "+
				"build sees is the Go field name", ev.Field(i).Name)
		}
		names = append(names, strings.Split(tag, ",")[0])
	}
	got := strings.Join(names, ",")

	want, known := shapes[learnstate.Version]
	if !known {
		t.Fatalf("Version is %d and no shape is recorded for it. Add this "+
			"one to the table:\n  %d: %q", learnstate.Version, learnstate.Version, got)
	}
	if got != want {
		t.Errorf("the stream's shape is not the one Version %d describes.\n"+
			"  version %d: %s\n  actual:     %s\n"+
			"A reader cannot tell an older writer from a current one by a "+
			"missing field, because `omitempty` omits a false one too \u2014 so "+
			"this number is the only thing that makes the difference visible. "+
			"Move it, and add the new shape to the table.",
			learnstate.Version, learnstate.Version, want, got)
	}
	// AND NO SHAPE IS RECORDED TWICE, which would mean two versions nothing can
	// tell apart.
	for v, shape := range shapes {
		for w, other := range shapes {
			if v < w && shape == other {
				t.Errorf("versions %d and %d have the same shape, so one "+
					"of them announces nothing", v, w)
			}
		}
	}
}

// AND A STREAM THIS BUILD CANNOT VOUCH FOR IS NOT AN EMPTY ONE.
//
// They leave a reader with the same thing — no events — and want opposite responses: wait a
// moment, against stop and look at which binary is being run. Studio showed the first for a day
// while the second was true.
func TestAStreamFromAnotherBuildIsNotAStreamThatHasSaidNothing(t *testing.T) {
	dir := t.TempDir()

	// NOT WRITTEN YET. The ordinary first fraction of a second.
	quiet := filepath.Join(dir, "quiet.jsonl")
	if err := os.WriteFile(quiet, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := learnstate.Latest(quiet); err != nil {
		t.Errorf("an empty stream reads as an error: %v", err)
	}

	// AND WRITTEN BY SOMETHING ELSE. One version back, which is exactly the case
	// that happened: a recorder from the previous build, still running.
	foreign := filepath.Join(dir, "foreign.jsonl")
	if err := os.WriteFile(foreign, []byte(
		`{"version":`+strconv.Itoa(learnstate.Version-1)+`,"seq":1,"phase":"ready"}`+"\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := learnstate.Latest(foreign)
	if !errors.Is(err, learnstate.ErrForeign) {
		t.Errorf("a stream from another build reads as %v, so a window shows "+
			"\"the recorder has not said\" about a recorder that is saying "+
			"plenty in a shape this build cannot read", err)
	}
}

// AND A STREAM THIS PACKAGE WROTE SAYS WHICH BUILD WROTE IT.
//
// The version refuses a stream this build cannot parse. This is the other half: a stream whose
// shape matches and whose behaviour may not, which is what a recorder one commit apart looks
// like. Stamped by the Writer rather than by each caller, as `Name` and `Application` are, so
// no event can be written without it.
func TestAStreamSaysWhichBuildWroteIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.jsonl")
	w, err := learnstate.NewWriter(path, "a routine", "trackmania")
	if err != nil {
		t.Fatal(err)
	}
	w.Say(learnstate.Event{Phase: learnstate.Ready})
	w.Say(learnstate.Event{Phase: learnstate.Checking, Presses: 1})
	w.Close()

	all, err := learnstate.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("%d event(s) of 2", len(all))
	}
	for i, e := range all {
		if e.Build == "" {
			t.Errorf("event %d carries no build, so a reader has nothing to "+
				"compare against its own and is back to file timestamps", i)
		}
		if e.Build != buildid.Revision() {
			t.Errorf("event %d says build %q and this binary is %q",
				i, e.Build, buildid.Revision())
		}
	}
}
