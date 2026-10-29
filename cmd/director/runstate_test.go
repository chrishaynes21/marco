package main

// WHAT A RUN SAYS ABOUT ITSELF, SAID BY THE THING THAT DECIDES IT.
//
// `--events` is a second account of the same decisions, not a replacement for the one a person
// reads: the terminal output is unchanged, and these are written at the points the runner decides
// something — the start wait beginning, a question going up, the same question coming down
// answered or withdrawn, an action going out, the run ending.
//
// The emission is what these establish. That Studio reads it without ever looking at the prose is
// `plugins/studio`'s to prove.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

// phasesIn is the phases a stream went through, in order, without repeats.
func phasesIn(events []runstate.Event) []runstate.Phase {
	var out []runstate.Phase
	for _, e := range events {
		if len(out) == 0 || out[len(out)-1] != e.Phase {
			out = append(out, e.Phase)
		}
	}
	return out
}

// questionsIn is every question a stream put, by id.
func questionsIn(events []runstate.Event) []string {
	var out []string
	seen := map[string]bool{}
	for _, e := range events {
		if e.Question != nil && !seen[e.Question.ID] {
			seen[e.Question.ID] = true
			out = append(out, e.Question.ID)
		}
	}
	return out
}

// finalOf is the last event and whether the stream ended.
func finalOf(t *testing.T, path string) (runstate.Event, bool) {
	t.Helper()
	e, final, err := runstate.Latest(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return e, final
}

// A REFUSAL BEFORE ANYTHING IS ACQUIRED STILL ENDS THE STREAM.
//
// Every early exit goes through `endedEarly`, and a refusal is a COMPLETE answer: a stream with
// no final event describes a run nobody can speak for, and calling an honest refusal that would
// be a lie about it.
func TestAnEarlyRefusalEndsTheEventStream(t *testing.T) {
	art := aDemonstrationToComposeFrom(t)
	if code := runComposeAct([]string{"--from", art.Name, "--name", "refused early",
		"--interaction", "0=the graphics tab"}); code != 0 {
		t.Fatal("composing failed")
	}
	path := filepath.Join(t.TempDir(), "events.jsonl")
	// AIMED AT AN APPLICATION THAT IS NOT THERE, so it refuses at acquisition.
	code := runPerformWorkflow([]string{"--play", "refused early",
		"--application", "notepad", "--seconds", "2", "--events", path})
	if code == 0 {
		t.Fatal("the run returned 0")
	}
	e, final := finalOf(t, path)
	if !final {
		t.Fatalf("the stream has no final event: %+v", e)
	}
	if e.Phase != runstate.Ended {
		t.Errorf("the final event's phase is %q", e.Phase)
	}
	if e.Outcome == "" || e.Reason == "" {
		t.Errorf("the final event says outcome=%q reason=%q", e.Outcome, e.Reason)
	}
	if e.Run == "" {
		t.Error("the final event carries no run reference")
	}
	if e.Act != "refused early" {
		t.Errorf("the final event says the Act was %q", e.Act)
	}
}

// AND THE PREPARATION DEADLINE IS SAID BEFORE ANYBODY IS ASKED TO BE READY.
//
// A front end counts down to that instant. It is emitted by the runner because only the runner
// decides when preparation ends — a window acting on its own arithmetic would be deciding
// something it is not in a position to know.
//
// This also covers EARLY START: the person says they are ready before the delay is spent, and the
// run proceeds without waiting out the rest of it.
func TestThePreparationDeadlineIsSaidAndAnEarlyStartIsHonoured(t *testing.T) {
	name := anActOverAStatedWindow(t)
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	was := newKeySource
	t.Cleanup(func() { newKeySource = was })
	newKeySource = func() (*navsource.Source, string) { return src, "" }

	path := filepath.Join(t.TempDir(), "events.jsonl")
	out := &liveOut{}
	stop := answerOnlyTheStart(t, out, press)
	defer stop()

	began := time.Now()
	captureStdoutInto(t, out, func() int {
		return runPerformWorkflow([]string{
			"--play", name, "--foreground", "--hotkeys",
			// A LONG PREPARATION, SPENT EARLY. The start key arrives in
			// milliseconds; a run that waited out the delay would take 30s.
			"--delay", "30s", "--seconds", "4", "--answer", "1s",
			"--patience", "40", "--dwell", "0s", "--events", path})
	})
	took := time.Since(began)

	events, err := runstate.Read(path)
	if err != nil || len(events) == 0 {
		t.Fatalf("no events were written: %v", err)
	}
	first := events[0]
	if first.Phase != runstate.Preparing {
		t.Errorf("the first phase is %q, want preparing", first.Phase)
	}
	if first.Preparation.IsZero() {
		t.Fatal("preparation was announced with no deadline to count to")
	}
	if d := first.Preparation.Sub(first.At); d < 25*time.Second {
		t.Errorf("the preparation deadline is %s away, not the 30s asked for", d)
	}
	// EARLY START: the run did not wait out the delay it announced.
	if took > 25*time.Second {
		t.Fatalf("the run took %s; saying ready early did not end preparation",
			took.Round(time.Second))
	}
	// AND THE STREAM ENDED, with the same outcome the record got.
	e, final := finalOf(t, path)
	if !final {
		t.Fatalf("the stream has no final event: %+v", e)
	}
	rec := theOnlyRunRecord(t)
	if e.Outcome != rec.Outcome {
		t.Errorf("the final event says %q and the record says %q", e.Outcome,
			rec.Outcome)
	}
	if e.Reason != rec.Because {
		t.Errorf("the final event's reason and the record's differ:\n  %q\n  %q",
			e.Reason, rec.Because)
	}
	if e.Run != rec.Run {
		t.Errorf("the final event cites %q and the record is %q", e.Run, rec.Run)
	}
}

// A QUESTION PUT AND NOT ANSWERED COMES DOWN AGAIN.
//
// Which is what makes a stale prompt impossible for a front end rendering the latest snapshot:
// the event after a question that expired has no question in it. Nobody answers anything here, so
// every question the run puts expires.
func TestAQuestionThatExpiresIsWithdrawnFromTheState(t *testing.T) {
	name := anActOverAStatedWindow(t)
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	was := newKeySource
	t.Cleanup(func() { newKeySource = was })
	newKeySource = func() (*navsource.Source, string) { return src, "" }

	path := filepath.Join(t.TempDir(), "events.jsonl")
	out := &liveOut{}
	// THE START KEY ONLY, so the run gets as far as asking about the screen and then
	// nobody answers it.
	stop := answerOnlyTheStart(t, out, press)
	defer stop()
	captureStdoutInto(t, out, func() int {
		return runPerformWorkflow([]string{
			"--play", name, "--foreground", "--hotkeys",
			"--delay", "0s", "--seconds", "4", "--answer", "500ms",
			"--patience", "40", "--dwell", "0s", "--events", path})
	})

	events, err := runstate.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	asked := questionsIn(events)
	if len(asked) < 2 {
		t.Fatalf("the run put %d question(s): %v\nphases %v", len(asked), asked,
			phasesIn(events))
	}
	// EVERY QUESTION IS FOLLOWED BY A SNAPSHOT WITHOUT IT. Checked over the whole
	// stream rather than at the end, because a prompt left pending for even one
	// snapshot is a prompt a person could answer twice.
	for i, e := range events {
		if e.Question == nil {
			continue
		}
		withdrawn := false
		for _, later := range events[i+1:] {
			if later.Question == nil || later.Question.ID != e.Question.ID {
				withdrawn = true
				break
			}
		}
		if !withdrawn && !e.Final {
			t.Errorf("question %q was never withdrawn; the stream ends with it "+
				"still pending", e.Question.ID)
		}
	}
	// AND EVERY QUESTION CARRIES THE KEYS IT ACCEPTS AND WHEN IT CLOSES, so a window
	// shows what the runner is really listening for rather than a set it decided on.
	for _, e := range events {
		if e.Question == nil {
			continue
		}
		if len(e.Question.Keys) == 0 {
			t.Errorf("question %q accepts no keys", e.Question.ID)
		}
		if e.Question.Until.IsZero() {
			t.Errorf("question %q never closes", e.Question.ID)
		}
		if e.Question.Asks == "" {
			t.Errorf("question %q has no words for a person to read",
				e.Question.ID)
		}
	}
}

// AND THE ORDINARY OUTPUT IS UNCHANGED BY ASKING FOR EVENTS.
//
// The events are a second account, not a replacement. A person at a terminal must see exactly
// what they saw before — and a front end that showed the transcript would otherwise be showing
// something different from what the CLI prints.
func TestAskingForEventsDoesNotChangeWhatIsPrinted(t *testing.T) {
	art := aDemonstrationToComposeFrom(t)
	if code := runComposeAct([]string{"--from", art.Name, "--name", "same output",
		"--interaction", "0=the graphics tab"}); code != 0 {
		t.Fatal("composing failed")
	}
	say := func(events string) string {
		out := &liveOut{}
		args := []string{"--play", "same output", "--application", "notepad",
			"--seconds", "2"}
		if events != "" {
			args = append(args, "--events", events)
		}
		captureStdoutInto(t, out, func() int { return runPerformWorkflow(args) })
		return out.String()
	}
	without := say("")
	with := say(filepath.Join(t.TempDir(), "events.jsonl"))

	// THE RUN REFERENCE DIFFERS BETWEEN TWO RUNS, so it is the only thing normalised.
	strip := func(s string) string {
		var kept []string
		for _, l := range strings.Split(s, "\n") {
			if strings.Contains(l, "run-2") {
				continue
			}
			kept = append(kept, l)
		}
		return strings.Join(kept, "\n")
	}
	if strip(without) != strip(with) {
		t.Errorf("asking for events changed the output:\n--- without ---\n%s\n"+
			"--- with ---\n%s", strip(without), strip(with))
	}
}

// AND A STREAM NOBODY ASKED FOR IS NOT WRITTEN.
func TestNoEventsAreWrittenWithoutTheFlag(t *testing.T) {
	art := aDemonstrationToComposeFrom(t)
	if code := runComposeAct([]string{"--from", art.Name, "--name", "quiet",
		"--interaction", "0=the graphics tab"}); code != 0 {
		t.Fatal("composing failed")
	}
	dir := t.TempDir()
	runPerformWorkflow([]string{"--play", "quiet", "--application", "notepad",
		"--seconds", "2"})
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("%d file(s) appeared without --events", len(entries))
	}
}

// AND A PATH THAT CANNOT BE WRITTEN IS A REFUSAL, not a silently eventless run.
//
// A front end that asked for events and got none would show a person nothing while a run was
// really happening, which is worse than refusing to start it.
func TestAnUnwritableEventPathRefusesTheRun(t *testing.T) {
	art := aDemonstrationToComposeFrom(t)
	if code := runComposeAct([]string{"--from", art.Name, "--name", "nowhere",
		"--interaction", "0=the graphics tab"}); code != 0 {
		t.Fatal("composing failed")
	}
	code := runPerformWorkflow([]string{"--play", "nowhere",
		"--application", "notepad", "--seconds", "2",
		"--events", filepath.Join(t.TempDir(), "no-such-dir", "events.jsonl")})
	if code == 0 {
		t.Fatal("a run whose events could not be written returned 0")
	}
}
