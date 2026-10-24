package runs_test

// ONE REFERENCE, ONE ATTEMPT, AND NOTHING WRITTEN WHERE NOBODY CLAIMED.
//
// These were the supervised runner's tests and they are here because the discipline is here: a
// routine's run is about to claim a reference the same way, into the same directory, and a second
// implementation of "which attempt owns this id" would be two answers to the question a bug report
// is built on.
//
// The three things being held are the three that were found the hard way. Two attempts printed the
// same reference for a minute each, and the hover probe wrote one attempt's pixels into the
// other's directory, because the record was guarded at the END. An attempt that died before
// finalisation retrieved nothing at all, so the exits that left without writing an outcome were
// invisible. And a record could be created at save time by an id nobody had reserved.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/runs"
)

// kept is the shape a caller of this package records in. Deliberately NOT either real one: this
// package is handed a record it may not look inside, and a test using a real record shape would
// not notice the day it started looking.
type kept struct {
	Run     string `json:"run"`
	Outcome string `json:"outcome"`
	Because string `json:"because,omitempty"`
	Pressed int    `json:"pressed,omitempty"`
}

// provisional is what an attempt leaves in its reservation before it has finished.
func provisional(id string) any {
	return kept{Run: id, Outcome: "started",
		Because: "reserved; this attempt had not finished"}
}

// inAStore points this process's Marco home at a fresh directory, so these tests claim references
// in their own place and not in the one on this machine.
func inAStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(marcopaths.EnvHome, dir)
	return dir
}

// recordIn reads back what is at a reference, and fails if there is nothing there.
func recordIn(t *testing.T, id string) kept {
	t.Helper()
	raw, err := os.ReadFile(runs.Path(id))
	if err != nil {
		t.Fatalf("the record of %s could not be read: %v", id, err)
	}
	var k kept
	if err := json.Unmarshal(raw, &k); err != nil {
		t.Fatalf("the record of %s is not readable JSON: %v", id, err)
	}
	return k
}

// AND THE PLACE IS THE STORE'S, which is what lets a window in another process find it.
//
// Two binaries write records and a third reads them. If the directory were derived per command the
// reference somebody pasted into a bug report would retrieve nothing, which is the failure
// internal/marcopaths exists because of.
func TestRecordsAreKeptInTheStore(t *testing.T) {
	dir := inAStore(t)
	if got, want := runs.Dir(), filepath.Join(dir, marcopaths.RunsDir); got != want {
		t.Errorf("records are kept in %q, want %q", got, want)
	}
	if got, want := runs.Path("run-x"), filepath.Join(dir, marcopaths.RunsDir,
		"run-x.json"); got != want {
		t.Errorf("a record goes to %q, want %q", got, want)
	}
}

// AND IT IS THE STORE'S PLACE WITH NOTHING IN THE ENVIRONMENT EITHER.
//
// The mutation that found this survived everything above: a derivation reading `$MARCO_HOME` and
// stopping there agrees with the real one in every test, because every test sets it. Studio sets
// it too — and `marco do`, typed into a terminal, does not. `filepath.Join("", "runs")` is
// `runs`, a RELATIVE directory that is a different place for every process depending on where it
// was started, which is the whole of the defect internal/marcopaths exists because of. It cost a
// library once; the records are the same hazard with the reference in a bug report pointing at
// nothing.
//
// Reads two strings and writes nothing, so it does not touch the store on this machine.
func TestTheStoresPlaceIsAnswerableWithNothingInTheEnvironment(t *testing.T) {
	t.Setenv(marcopaths.EnvHome, "")
	if err := os.Unsetenv(marcopaths.EnvHome); err != nil {
		t.Fatal(err)
	}
	got := runs.Dir()
	if want := filepath.Join(marcopaths.Default(), marcopaths.RunsDir); got != want {
		t.Errorf("with no home named, records are kept in %q, want %q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("records are kept in %q, which is wherever the process was started", got)
	}
}

// A RESERVATION LEAVES A RECORD BEHIND AT ONCE, in the caller's own words.
//
// This is the killed-child case, and the reason the provisional record is not an empty file: an
// attempt that dies between the first press and the outcome has to be retrievable as an attempt
// that did not finish. Retrieving nothing is indistinguishable from a run that never happened,
// and that is how three exits that wrote no outcome stayed hidden.
func TestAReservationIsRetrievableBeforeAnythingFinishes(t *testing.T) {
	inAStore(t)
	got, err := runs.Reserve(runs.Claim{At: time.Now(), Provisional: provisional})
	if err != nil {
		t.Fatalf("nothing was reserved: %v", err)
	}
	k := recordIn(t, got.ID)
	if k.Outcome != "started" {
		t.Errorf("the reservation says %q, want the caller's own word for unfinished",
			k.Outcome)
	}
	// AND IT NAMES THE REFERENCE IT IS FILED UNDER. A provisional record describing
	// another attempt would send whoever read it to the wrong run.
	if k.Run != got.ID {
		t.Errorf("the record at %s calls itself %q", got.ID, k.Run)
	}
}

// AND A RESERVATION THAT WOULD LEAVE NOTHING BEHIND IS REFUSED.
//
// Rather than claiming the id with an empty file, which retrieves as a record that will not parse
// — a worse answer than either a real record or no record at all.
func TestAReservationNeedsTheRecordItLeaves(t *testing.T) {
	inAStore(t)
	got, err := runs.Reserve(runs.Claim{At: time.Now()})
	if err == nil {
		t.Fatalf("a claim with no record to leave reserved %q", got.ID)
	}
}

// TWO ATTEMPTS PROPOSING THE SAME REFERENCE GET TWO NAMESPACES.
//
// The one thing a reference must do is retrieve the attempt it names. `propose` here returns the
// same candidate every time, which is what two runs started in the same second did before the
// suffix existed — and the retry is what makes the second attempt its own.
func TestTwoAttemptsThatProposeOneReferenceGetTwo(t *testing.T) {
	inAStore(t)
	frames := filepath.Join(t.TempDir(), "frames")
	n := 0
	// SAME CANDIDATE TWICE, then distinct ones, which is the shape of a real
	// collision: the loser does not propose the taken name for ever.
	propose := func(time.Time) string {
		n++
		if n <= 2 {
			return "run-taken"
		}
		return "run-free"
	}
	at := time.Date(2026, 10, 24, 15, 0, 0, 0, time.UTC)
	first, err := runs.Reserve(runs.Claim{At: at, FramesRoot: frames,
		Provisional: provisional, Propose: propose})
	if err != nil {
		t.Fatalf("the first attempt reserved nothing: %v", err)
	}
	second, err := runs.Reserve(runs.Claim{At: at, FramesRoot: frames,
		Provisional: provisional, Propose: propose})
	if err != nil {
		t.Fatalf("the second attempt reserved nothing: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("both attempts hold %q", first.ID)
	}
	// AND NEITHER CAN REACH THE OTHER'S PIXELS. The frames directory is the half that
	// was overwritten while the record looked protected.
	if first.Frames == second.Frames {
		t.Errorf("both attempts write their frames into %q", first.Frames)
	}
	for _, d := range []string{first.Frames, second.Frames} {
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			t.Errorf("%q is not a directory this attempt holds: %v", d, err)
		}
	}
	// AND THE REFERENCES ARE TWO RECORDS, not one written over.
	if recordIn(t, first.ID).Run == recordIn(t, second.ID).Run {
		t.Errorf("one record answers for both attempts")
	}
}

// AND TWO ATTEMPTS IN THE SAME SECOND ARE TWO ATTEMPTS, through the real proposer.
//
// Not a probability argument: whether the suffix happens to collide or not, the exclusive claim
// and the retry mean the second attempt leaves with a reference of its own. This is the acceptance
// check "both repeated runs produce distinct references", at the layer that decides it.
func TestTwoAttemptsInOneSecondAreTwoReferences(t *testing.T) {
	inAStore(t)
	at := time.Date(2026, 10, 24, 15, 0, 0, 0, time.UTC)
	first, err := runs.Reserve(runs.Claim{At: at, Provisional: provisional})
	if err != nil {
		t.Fatalf("the first attempt reserved nothing: %v", err)
	}
	second, err := runs.Reserve(runs.Claim{At: at, Provisional: provisional})
	if err != nil {
		t.Fatalf("the second attempt reserved nothing: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("two attempts in one second both hold %q", first.ID)
	}
}

// A CANDIDATE THAT LOSES THE PIXELS IS ABANDONED WHOLE.
//
// Half a namespace is the state this was written to prevent: a record under one id and somebody
// else's frames directory under the same one. So the record the losing candidate already made is
// removed, and the id is left as free as it was.
func TestACandidateThatCannotHoldItsFramesKeepsNothing(t *testing.T) {
	inAStore(t)
	frames := filepath.Join(t.TempDir(), "frames")
	if err := os.MkdirAll(filepath.Join(frames, "run-taken"), 0o755); err != nil {
		t.Fatal(err)
	}
	n := 0
	propose := func(time.Time) string {
		n++
		if n == 1 {
			return "run-taken"
		}
		return "run-free"
	}
	got, err := runs.Reserve(runs.Claim{At: time.Now(), FramesRoot: frames,
		Provisional: provisional, Propose: propose})
	if err != nil {
		t.Fatalf("nothing was reserved: %v", err)
	}
	if got.ID != "run-free" {
		t.Fatalf("the attempt holds %q, and somebody else holds that frames "+
			"directory", got.ID)
	}
	if _, err := os.Stat(runs.Path("run-taken")); err == nil {
		t.Errorf("the abandoned candidate left a record at run-taken")
	}
}

// AND A RECORD CAN ONLY BE WRITTEN WHERE A RESERVATION ALREADY IS.
//
// The guard is the missing O_CREATE, and this is the whole of what it buys: an id nobody claimed
// cannot be recorded against, so an attempt cannot invent a reference at the end of itself.
func TestNothingIsRecordedAgainstAnUnreservedReference(t *testing.T) {
	inAStore(t)
	held, err := runs.Reserve(runs.Claim{At: time.Now(), Provisional: provisional})
	if err != nil {
		t.Fatalf("nothing was reserved: %v", err)
	}
	if _, err := runs.Finalise(held.ID, kept{Run: held.ID, Outcome: "performed",
		Pressed: 3}); err != nil {
		t.Fatalf("the attempt could not record itself: %v", err)
	}
	if _, err := runs.Finalise("run-never-reserved", kept{Run: "run-never-reserved",
		Outcome: "performed"}); err == nil {
		t.Error("a record was written for a reference nobody reserved")
	}
	if _, err := os.Stat(runs.Path("run-never-reserved")); err == nil {
		t.Error("an unreserved reference has a record on disk")
	}
}

// AND FINALISING ONE ATTEMPT LEAVES THE OTHER'S AS IT WAS.
//
// The failure this stands against is the one that cost the trial: both attempts citing the same
// reference, so the one that finished last described itself in the other's record. Here the second
// attempt finishes and the first is still, correctly, an attempt that did not finish.
func TestFinalisingOneAttemptDoesNotTouchAnother(t *testing.T) {
	inAStore(t)
	at := time.Date(2026, 10, 24, 15, 0, 0, 0, time.UTC)
	first, err := runs.Reserve(runs.Claim{At: at, Provisional: provisional})
	if err != nil {
		t.Fatalf("the first attempt reserved nothing: %v", err)
	}
	second, err := runs.Reserve(runs.Claim{At: at, Provisional: provisional})
	if err != nil {
		t.Fatalf("the second attempt reserved nothing: %v", err)
	}
	p, err := runs.Finalise(second.ID, kept{Run: second.ID, Outcome: "performed",
		Pressed: 2})
	if err != nil {
		t.Fatalf("the second attempt could not record itself: %v", err)
	}
	if p != runs.Path(second.ID) {
		t.Errorf("the second attempt recorded itself at %q", p)
	}
	if got := recordIn(t, second.ID); got.Outcome != "performed" || got.Pressed != 2 {
		t.Errorf("the second attempt reads back as %+v", got)
	}
	// THE ONE THAT DID NOT FINISH STILL SAYS SO.
	if got := recordIn(t, first.ID); got.Outcome != "started" {
		t.Errorf("the unfinished attempt now reads as %q", got.Outcome)
	}
}

// AND A REFERENCE IS SORTABLE AND NAMED AFTER ITS OWN MOMENT.
//
// A person reads it, pastes it, and reads it aloud. The timestamp is the half they recognise; the
// suffix is why two attempts in one second are two references.
func TestAReferenceCarriesTheMomentItWasProposedIn(t *testing.T) {
	at := time.Date(2026, 10, 24, 15, 4, 5, 0, time.UTC)
	got := runs.Candidate(at)
	want := "run-" + at.Format("20060102-150405") + "-"
	if len(got) != len(want)+4 || got[:len(want)] != want {
		t.Errorf("a reference reads %q, want %q and four more characters", got, want)
	}
}
