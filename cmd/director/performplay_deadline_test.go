package main

// `--seconds` BOUNDS A PLAY, AND IT USED NOT TO.
//
// The deadline was the cursor loop's own `for time.Now().Before(deadline)`, which the play driver
// never saw: its look closure called `looks.next(ctx)`, and `next` checked cancellation and the
// window and nothing else. So a play could keep observing — and keep asking for actions — past the
// time a person was told the run would take. An observation count is not a wall clock: it bounds
// how many looks ONE interaction may consume and says nothing about how long they take.
//
// This drives the production command past runner construction, which is the only place the bug
// lived. It runs against the window already in front, dry, with nobody answering the questions —
// so the run stalls, and the only thing that can end it is the clock.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/internal/routes"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
)

// anActOverTheWindowInFront saves a routine built from a real look and a one-interaction Act over
// it, and returns the name to select the Act by.
//
// The window in front is the one window a test may use without touching anybody's focus, and the
// routine is built from a REAL look at it so its control could be re-acquired — see
// `aRoutineFromALookAt`, whose reasoning this borrows wholesale.
func anActOverTheWindowInFront(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_ROUTES", filepath.Join(home, "routes"))

	windows := winprovider.New()
	art := aRoutineFromALookAt(t, windows, theForegroundWindow(t, windows))
	store := mustOpenStore(t)
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}
	src, binds, err := demonstrated.Compose(art, "an act",
		[]demonstrated.Named{{Position: 0, Name: "the control in front"}},
		time.Now(), marcoChecks{})
	if err != nil {
		t.Fatal(err)
	}
	rt := routes.Route{Slug: routes.Slug(art.Name)}
	if err := demonstrated.Save(routesHere(), rt, src, binds, time.Now()); err != nil {
		t.Fatal(err)
	}
	return art.Name
}

// A STALLED PLAY ENDS AT ITS DEADLINE, AND SAYS SO.
//
// Nobody answers, so the run cannot get past naming the screen — and with the clock not reaching
// the play driver it would have sat there consuming its whole look budget. The assertion is the
// wall clock: the command returns within a few seconds of `--seconds`, nothing was pressed, and
// the attempt still got a finalized record.
func TestAStalledPlayEndsAtItsDeadline(t *testing.T) {
	name := anActOverTheWindowInFront(t)

	// THE SYNTHETIC HOOK, in place of the desktop's — and NOBODY PRESSES ANYTHING on it.
	// A run whose questions go unanswered is exactly the stall this is about.
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	was := newKeySource
	t.Cleanup(func() { newKeySource = was })
	newKeySource = func() (*navsource.Source, string) { return src, "" }

	out := &liveOut{}
	// THE START KEY ONLY. Everything after it goes unanswered, which is the stall.
	stop := answerOnlyTheStart(t, out, press)
	defer stop()
	began := time.Now()
	code := captureStdoutInto(t, out, func() int {
		return runPerformWorkflow([]string{
			"--play", name, "--foreground", "--hotkeys",
			"--delay", "0s", "--seconds", "3", "--answer", "1s",
			"--patience", "400",
			// NO DWELL, BECAUSE THIS RUN IS DRY. See the hotkey test: in a dry
			// run the pointer never arrives, and that check would refuse the
			// click for a reason that has nothing to do with the clock.
			"--dwell", "0s",
		})
	})
	took := time.Since(began)

	// THE CLOCK ENDED IT. Generously bounded — a look costs a capture and a proposal,
	// and the last one may be in flight when the deadline passes — but far below the
	// 400-look budget this run was given, which is what the defect would have spent.
	// Measured at 3.1s with the clock wired through. The bound is generous for a loaded
	// machine and still far below the 640-look budget this run was given, which is what
	// the defect spent: 640 looks at the loop interval is about 64 seconds.
	if took > 20*time.Second {
		t.Fatalf("a --seconds 3 play ran for %s; the deadline does not reach the "+
			"play driver", took.Round(time.Second))
	}
	if code == 0 {
		t.Fatalf("a run nobody answered returned 0:\n%s", out.String())
	}
	// AND THE ATTEMPT WAS FINALIZED: a reference, with an outcome, written down. A run
	// that ended on the clock is still a run somebody has to be able to look up.
	said := out.String()
	if !strings.Contains(said, "run ") {
		t.Fatalf("no run reference was printed:\n%s", said)
	}
	for _, word := range []string{"time limit", "--seconds"} {
		if !strings.Contains(said, word) {
			t.Errorf("the run does not say the clock ended it (%q missing):\n%s",
				word, said)
		}
	}
	// AND NOTHING WAS PRESSED. Dry anyway — there is no `--live` — and asserted because
	// "it stopped in time" and "it stopped without acting" are different claims.
	if strings.Contains(said, "actions sent 1") {
		t.Errorf("an action went out:\n%s", said)
	}
}

// answerOnlyTheStart presses the start key and then answers nothing at all.
//
// Which is the stall this test needs: the run gets into its loop — past preparation, past the
// countdown, past runner construction — and then waits for somebody to name the screen. Nobody
// does, so the only thing that can end it is the clock.
//
// The first version of this test pressed nothing, and spent 63 seconds waiting for the START key
// before the play driver was ever reached. It was measuring the wrong wait and would have passed
// with the deadline bug still in place had the bound been looser.
func answerOnlyTheStart(t *testing.T, out *liveOut, press func(uint16, bool)) func() {
	t.Helper()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(40 * time.Millisecond):
			}
			if strings.Contains(out.String(), "press "+
				navsource.ConsentKeys[navsource.ConsentApprove]+" to start") {
				press(navsource.KeyF7, true)
				press(navsource.KeyF7, false)
				return
			}
		}
	}()
	return func() { close(done) }
}

// AND THE DURABLE RECORD IS WRITTEN FROM THE ACT'S ACCOUNT, not from the artifact cursor.
//
// The classifier and the sentence both: `outcomeOfAct` decides the word, and `Account.Short` is
// the `because`. Asserted on the record ON DISK rather than on the printed line, because the play
// branch prints the account either way — printing it and then recording something else was the
// defect, and only the file can tell the two apart.
//
// Plus the Act's own identity, which an audit needs beside the demonstration: a record with only
// the demonstration cannot tell two Acts over one demonstration apart.
func TestThePlaysRecordIsWrittenFromItsAccount(t *testing.T) {
	name := anActOverTheWindowInFront(t)
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	was := newKeySource
	t.Cleanup(func() { newKeySource = was })
	newKeySource = func() (*navsource.Source, string) { return src, "" }

	out := &liveOut{}
	stop := answerOnlyTheStart(t, out, press)
	defer stop()
	captureStdoutInto(t, out, func() int {
		return runPerformWorkflow([]string{
			"--play", name, "--foreground", "--hotkeys",
			"--delay", "0s", "--seconds", "3", "--answer", "1s",
			"--patience", "400", "--dwell", "0s",
		})
	})

	rec := theOnlyRunRecord(t)
	if rec.Act != name {
		t.Errorf("the record says the Act was %q, want %q", rec.Act, name)
	}
	if rec.ActSource == "" {
		t.Error("the record carries no digest of the play's own source")
	}
	if rec.Workflow == "" {
		t.Error("the record lost the demonstration the interactions came from")
	}
	// THE SENTENCE IS THE ACCOUNT'S. `Progress.Summary()` is the cursor's, and this run
	// is an Act: "N of M interaction(s) verified" is a fact about the play.
	if !strings.Contains(rec.Because, "interaction(s) verified") {
		t.Errorf("the record's reason is not the Act's account: %q", rec.Because)
	}
	// AND IT SAYS WHY THE RUN DID NOT REACH EVERY SENTENCE, which is the account's own
	// account of itself: `Cut` is what puts the clock into it. Without that the record
	// would say "0 of 1 verified" and never say what stopped it.
	if !strings.Contains(rec.Because, "time limit") {
		t.Errorf("the record does not say what ended the run: %q", rec.Because)
	}
	// AND THE OUTCOME IS A WORD THE ACCOUNT COULD HAVE PRODUCED. Nobody named the
	// screen, so nothing was verified and nothing was pressed.
	switch rec.Outcome {
	case RunRefused, RunUnconfirmed, RunCancelled:
	default:
		t.Errorf("a run that pressed nothing was recorded %q", rec.Outcome)
	}
	if rec.Sent != 0 {
		t.Errorf("the record says %d action(s) were sent", rec.Sent)
	}
}

// theOnlyRunRecord reads the one record this test's isolated store holds.
func theOnlyRunRecord(t *testing.T) RunRecord {
	t.Helper()
	dir := filepath.Join(configDir(), runsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var found []RunRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			t.Fatal(rerr)
		}
		var rec RunRecord
		if json.Unmarshal(raw, &rec) != nil || rec.Run == "" {
			// A RESERVATION WITH NOTHING IN IT YET. `reserveRun` creates the
			// file before the run; only a finalized one has a Run in it.
			continue
		}
		found = append(found, rec)
	}
	if len(found) != 1 {
		t.Fatalf("the store holds %d finalized record(s), want 1", len(found))
	}
	return found[0]
}
