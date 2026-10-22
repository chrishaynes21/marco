package main

// THE JOURNAL IS WRITTEN BY USING STUDIO, NOT BY ASKING IT TO WRITE ONE.
//
// Every test here goes in through the verb the page calls — `Learn`, `Learning`, `Picture`,
// `Compose`, `Run` — and reads the file afterwards. That is the whole claim being tested: the
// four facts the owner should never have to collect are collected by an ordinary flow, so a test
// that called `trials.note` directly would prove the writer works and leave the wiring
// unexercised. See the "prove wiring by deleting it" rule; every call site added for this has a
// test here that fails when the call is removed.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/learnstate"
)

// theTrial is the single journal a test's Studio wrote, parsed.
func theTrial(t *testing.T, a *Acts) trial {
	t.Helper()
	all, err := trialsIn(a.home)
	if err != nil {
		t.Fatalf("no trial was written in %s: %v", a.home, err)
	}
	if len(all) != 1 {
		t.Fatalf("%d journals were written; a Studio writes one", len(all))
	}
	if all[0].Unreadable > 0 {
		t.Errorf("%d line(s) of the journal it just wrote are unreadable to it",
			all[0].Unreadable)
	}
	return all[0]
}

// notes is the notes of one kind, in order.
func notes(t trial, k trialKind) []trialNote {
	var out []trialNote
	for _, n := range t.Notes {
		if n.Kind == k {
			out = append(out, n)
		}
	}
	return out
}

// THE STATES A RECORDING PASSED THROUGH ARE WRITTEN DOWN AS THE WINDOW SHOWS THEM.
//
// The owner's report was "settling is ass, it doesn't finish in time", and answering it took
// reading their recorder's own stream off their machine to find that the second press reached a
// verdict in 2.7 seconds and the window said "wait for the result to settle" for the 29 after
// that. This is that reading, taken while it happens, of what the WINDOW said rather than of
// what the recorder wrote — the two differed, and the gap between them was the defect.
func TestARecordingWritesDownTheStatesTheWindowShowed(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	aRecorderStandingIn(t, a)
	if why := a.Learn("the audio tab opening", "ArkAscended.exe", 60); why != "" {
		t.Fatalf("the recorder would not start: %s", why)
	}
	// THE RECORDING ITSELF IS THE FIRST THING WRITTEN.
	began := notes(theTrial(t, a), trialRecording)
	if len(began) != 1 || began[0].Name != "the audio tab opening" ||
		began[0].Application != "ArkAscended.exe" || began[0].Seconds != 60 {
		t.Fatalf("the journal's account of the recording is %+v", began)
	}

	// ── AND THEN EACH DISTINCT STATE, IN ORDER ──
	//
	// Written through the page's own poll: `Learning` is what the window calls four
	// times every two seconds, and what it hands back is what a person is looking at.
	for _, at := range []learnstate.Event{
		{Phase: learnstate.Reading},
		{Phase: learnstate.Ready, StartKnown: true, ScreenRead: true},
		{Phase: learnstate.Checking, Presses: 1, Watched: 1,
			StartKnown: true, ScreenRead: true},
		{Phase: learnstate.Captured, Presses: 1, Watched: 1, Captured: 1,
			StartKnown: true, ScreenRead: true},
	} {
		sayRecording(t, a, at)
		waitForPhase(t, a, at.Phase)
		// AND POLLED AGAIN WITH NOTHING CHANGED, which must add nothing: the
		// page polls for as long as it is open, and a line per poll would be a
		// file whose size says how long somebody left the window up.
		for i := 0; i < 5; i++ {
			a.Learning()
		}
	}

	states := theTrial(t, a).states()
	var phases []string
	for _, s := range states {
		phases = append(phases, s.Phase)
	}
	// THE FIRST PHASE MAY BE EITHER. `Learning` reports `reading` both from the
	// stream and as its own answer before the first look lands, and which of the two
	// a test sees depends on when it polled — so what is pinned is the sequence from
	// `ready` on, and that nothing repeats.
	want := []string{"ready", "checking", "captured"}
	if len(phases) < len(want) {
		t.Fatalf("the states written down are %v", phases)
	}
	if got := phases[len(phases)-len(want):]; !reflect.DeepEqual(got, want) {
		t.Errorf("the states written down end %v, not %v (all of them: %v)",
			got, want, phases)
	}
	for i := 1; i < len(phases); i++ {
		if phases[i] == phases[i-1] {
			t.Errorf("state %d repeats %q; an unchanged poll must write "+
				"nothing: %v", i, phases[i], phases)
		}
	}
	// AND THE EVIDENCE A VERDICT ARRIVED, with the sentence the window was showing.
	last := states[len(states)-1]
	if last.Captured != 1 || last.Presses != 1 || !last.ScreenRead {
		t.Errorf("the last state is %+v", last)
	}
	if !strings.Contains(last.Doing, learnstate.Doing(learnstate.Captured,
		"ArkAscended.exe")) {
		t.Errorf("the state does not carry what the window was saying: %q",
			last.Doing)
	}
	// AND THE SECONDS, which is the measurement the owner's report was about.
	if last.At.Before(began[0].At) {
		t.Errorf("the states are stamped before the recording began")
	}
}

// AND WHAT CAME OF IT, WITH THE PICTURES THE STORE KEPT.
func TestTheTrialSaysWhatCameOfTheRecording(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	aRecorderStandingIn(t, a)
	if why := a.Learn("ark graphics live", "ArkAscended.exe", 60); why != "" {
		t.Fatalf("the recorder would not start: %s", why)
	}
	// THE DEMONSTRATION IN THE FIXTURE IS CALLED THAT, so what the recorder is
	// pretending to have saved is already in the store with pictures of it.
	store, err := demo.Open(a.home)
	if err != nil {
		t.Fatal(err)
	}
	art, ok := store.Sequence("ark graphics live")
	if !ok {
		t.Fatal("the fixture's demonstration is not in the store")
	}
	for _, label := range []string{"screen_a", "screen_b", "control_1"} {
		if err := store.SavePreview(art, label, []byte(label)); err != nil {
			t.Fatal(err)
		}
	}
	if why := a.Done(true); why != "" {
		t.Fatalf("finishing the recording said %q", why)
	}
	learningSaid(t, a)

	over := notes(theTrial(t, a), trialRecorded)
	if len(over) != 1 {
		t.Fatalf("the journal holds %d accounts of what came of it", len(over))
	}
	if over[0].Name != "ark graphics live" || over[0].Said == "" {
		t.Errorf("what came of it is %+v", over[0])
	}
	// THE PICTURES, SORTED THE WAY THE SURFACE SORTS THEM. Evidence that there is
	// something to review at all — "the pictures are gone" was a report about this.
	if len(over[0].Screens) != 2 || len(over[0].Controls) != 1 {
		t.Errorf("the pictures it says were kept are %v and %v",
			over[0].Screens, over[0].Controls)
	}
}

// HOW OFTEN THE PAGE HAD TO ASK FOR A PICTURE IS A NUMBER, NOT A REPORT.
//
// "Also the images blink in the ui" was 66 requests in 4.2 seconds with nothing on screen
// changing. The window counts them now, so the next time it happens nobody has to describe it.
func TestThePicturesThePageAskedForAreCountedAndJudged(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	store, err := demo.Open(a.home)
	if err != nil {
		t.Fatal(err)
	}
	art, ok := store.Sequence("ark graphics live")
	if !ok {
		t.Fatal("the fixture's demonstration is not in the store")
	}
	for _, label := range []string{"screen_a", "control_1"} {
		if err := store.SavePreview(art, label, []byte(label)); err != nil {
			t.Fatal(err)
		}
	}
	// ONE EACH, WHICH IS WHAT A PAGE THAT KEEPS ITS ELEMENTS DOES.
	if a.Picture("ark graphics live", "screen_a") == "" {
		t.Fatal("the window served no picture")
	}
	if a.Picture("ark graphics live", "control_1") == "" {
		t.Fatal("the window served no picture")
	}
	a.Close()

	held := theTrial(t, a)
	worst, most, labels, total := held.refetched()
	if labels != 2 || total != 2 || most != 1 {
		t.Fatalf("the tally is %d label(s), %d request(s), most %d",
			labels, total, most)
	}
	if len(worst) != 0 {
		t.Errorf("a picture asked for once is reported as re-fetched: %v", worst)
	}
	if got := picturesVerdict(held); !strings.HasPrefix(got, "held") {
		t.Errorf("the verdict on two pictures fetched once each is %q", got)
	}

	// ── AND THE SAME PICTURE ASKED FOR AGAIN AND AGAIN IS THE DEFECT ──
	//
	// Through the production verb, as many times as the blinking review did in a
	// second. The tally is written at most every couple of seconds while they are
	// arriving — see trialTally — so the one that matters is the one `Close` flushes,
	// which is what makes a window closed by hand still leave the evidence.
	b, _ := aStoreWithADemonstration(t)
	bs, err := demo.Open(b.home)
	if err != nil {
		t.Fatal(err)
	}
	bart, _ := bs.Sequence("ark graphics live")
	if err := bs.SavePreview(bart, "screen_b", []byte("x")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < trialRefetch+5; i++ {
		b.Picture("ark graphics live", "screen_b")
	}
	// BEFORE THE FLUSH the throttle has only let the first one through, which is the
	// reason the flush exists.
	if _, most, _, _ := theTrial(t, b).refetched(); most != 1 {
		t.Errorf("the throttled tally says %d, so it is not throttled", most)
	}
	b.Close()
	worst, most, _, _ = theTrial(t, b).refetched()
	if most != trialRefetch+5 {
		t.Fatalf("after closing, the tally says %d of %d requests", most,
			trialRefetch+5)
	}
	if len(worst) != 1 || !strings.Contains(worst[0], "screen_b") {
		t.Fatalf("asking %d times for one picture is reported as %v",
			trialRefetch+5, worst)
	}
	if got := picturesVerdict(theTrial(t, b)); !strings.Contains(got, "NOT HELD") {
		t.Errorf("the verdict on a picture fetched %d times is %q",
			trialRefetch+5, got)
	}
}

// AN ACT IS WRITTEN DOWN WHETHER IT SAVED OR WAS REFUSED.
//
// The refusal is the more interesting half: the one limitation this surface knows it has is a
// press whose destination has nothing readable on it, and a trial run to confirm that is a trial
// whose evidence is a sentence explaining why nothing was saved.
func TestTheTrialSaysWhetherAnActWasSavedOrRefused(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if c := a.Compose("ark graphics live", "the audio page",
		[]string{"1=the audio tab"}); c.Error != "" {
		t.Fatal(c.Error)
	}
	// AND ONE THAT CANNOT BE: an Act may not be named after one of its own
	// interactions, which is refused with a sentence rather than a compile error.
	if c := a.Compose("ark graphics live", "the audio tab",
		[]string{"1=the audio tab"}); c.Error == "" {
		t.Fatal("an Act named after its own interaction was composed")
	}

	acts := notes(theTrial(t, a), trialAct)
	if len(acts) != 2 {
		t.Fatalf("the journal holds %d accounts of composing", len(acts))
	}
	if acts[0].Act != "the audio page" || acts[0].From != "ark graphics live" ||
		acts[0].Error != "" {
		t.Errorf("the Act that saved is written down as %+v", acts[0])
	}
	if len(acts[0].Steps) != 1 || acts[0].Steps[0] != "the audio tab" {
		t.Errorf("the steps written down are %v", acts[0].Steps)
	}
	if acts[1].Error == "" {
		t.Errorf("the Act that was refused is written down as saved: %+v", acts[1])
	}
}

// THE CANONICAL RESULT OF A RUN IS CITED, AND READ OUT OF THE RECORD THE RUN WROTE.
func TestTheTrialCitesTheRunsOwnRecord(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	if c := a.Compose("ark graphics live", "the audio page",
		[]string{"1=the audio tab"}); c.Error != "" {
		t.Fatal(c.Error)
	}
	// THE RECORD THE STAND-IN'S REFERENCE NAMES, written where `director` writes one.
	//
	// The trial reads the record rather than this window's bookkeeping, so the
	// numbers in it are the run's own: the outcome, how many presses actually went
	// out, and WHICH DIRECTOR performed it.
	runs := filepath.Join(a.home, "runs")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	const ref = "run-20261019-000000-aaaa"
	raw, err := json.Marshal(map[string]any{
		"version": 1, "run": ref, "act": "the audio page",
		"application": "ArkAscended.exe", "build": "deadbee",
		"started": time.Now().UTC().Format(time.RFC3339),
		"ended":   time.Now().UTC().Format(time.RFC3339),
		"outcome": "verified", "because": "the screen said so",
		"actions_sent": 1, "verified": 1, "live": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runs, ref+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Setenv("STUDIO_STANDIN_QUICK", "1")
	a.raising = func(string) string { return "" }
	if why := a.Run("the audio page", "ArkAscended.exe", false, false); why != "" {
		t.Fatalf("the run would not start: %s", why)
	}
	for i := 0; i < 500; i++ {
		if !a.Watch().Active {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if a.Watch().Active {
		t.Fatal("the run never finished")
	}

	held := theTrial(t, a)
	if started := notes(held, trialPlay); len(started) != 1 ||
		started[0].Act != "the audio page" || started[0].Live {
		t.Fatalf("the performance is written down as %+v", started)
	}
	ran := notes(held, trialRan)
	if len(ran) != 1 {
		t.Fatalf("the journal holds %d results", len(ran))
	}
	if ran[0].Reference != ref {
		t.Errorf("the result cites %q, not the reference the run printed",
			ran[0].Reference)
	}
	if ran[0].Outcome != "verified" || ran[0].Sent != 1 || ran[0].Verified != 1 {
		t.Errorf("the result reads %+v, not what the record says", ran[0])
	}
	if ran[0].Ran != "deadbee" {
		t.Errorf("the director that performed it is written down as %q",
			ran[0].Ran)
	}
	if got := resultVerdict(held); !strings.Contains(got, ref) ||
		!strings.Contains(got, "verified") {
		t.Errorf("the verdict on the run is %q", got)
	}
}

// TWO BUILDS TAKING PART IS THE VERDICT, NOT A TIMESTAMP TO COMPARE.
//
// The owner lost a day of recordings to a window and a recorder built a day apart, visible only
// by looking at file modification times. The instruction that followed was that the build
// revision has to be available in Studio's diagnostics and run evidence, and that the owner
// should never need to compare executables. This is the diagnostic saying it.
func TestTheTrialSaysWhenTwoBuildsTookPart(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	a.revision = func() string { return "1111111" }
	aRecorderStandingIn(t, a)
	if why := a.Learn("the audio tab opening", "ArkAscended.exe", 60); why != "" {
		t.Fatalf("the recorder would not start: %s", why)
	}
	// A STREAM THIS BUILD CAN READ, FROM A BUILD IT IS NOT. The version matches, so
	// nothing refuses it; the behaviour behind it is another day's. That is the case
	// the protocol check cannot catch and the one that cost the day.
	a.mu.Lock()
	path := a.learn.progress
	a.mu.Unlock()
	if path == "" {
		t.Skip("this recording has no progress stream to stand in for")
	}
	if err := os.WriteFile(path, []byte(`{"version":`+
		strconv.Itoa(learnstate.Version)+`,"seq":1,"phase":"checking",`+
		`"presses":1,"watched":1,"start_known":true,"screen_read":true,`+
		`"build":"2222222"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, a, func(l Learning) bool { return l.Recorder == "2222222" })

	held := theTrial(t, a)
	named, silent, agree := held.builds()
	if agree {
		t.Fatalf("two builds took part and the journal says they agree: %v", named)
	}
	if len(silent) != 0 {
		t.Errorf("both builds said what they were, and %v is reported as "+
			"having said nothing", silent)
	}
	if got := buildVerdict(held); len(got) < 3 ||
		!strings.Contains(got[0], "DIFFERENT BUILDS") ||
		!strings.Contains(strings.Join(got, "\n"), "1111111") ||
		!strings.Contains(strings.Join(got, "\n"), "2222222") {
		t.Errorf("the verdict on the builds is %q", got)
	}
	if who := named["1111111"]; len(who) != 1 || who[0] != "the window" {
		t.Errorf("1111111 is named as %v", who)
	}
	if who := named["2222222"]; len(who) != 1 || who[0] != "the recorder" {
		t.Errorf("2222222 is named as %v", who)
	}
	// AND THE VERDICT ON THE RECORDING SAYS THE SAME THING, because a mismatched
	// recorder makes every other state in the trial a claim about a binary nobody
	// was running.
	if got := statesVerdict(held); !strings.Contains(got, "DIFFERENT BUILD") {
		t.Errorf("the verdict on the states is %q", got)
	}
}

// AN UNSTAMPED BUILD IS NOT A BUILD IDENTITY, AND TWO OF THEM ARE NOT AGREEMENT.
//
// `buildid.Same` refuses to vouch for a pair of unknowns and `buildid.Differ` refuses to call
// them a difference, which is why neither is the negation of the other. A verdict that folded
// two unknowns together would read "one build throughout: unknown" — a claim about identity made
// out of two binaries that could not say what they were, which is the precise mistake the whole
// build-identity change exists to stop.
func TestAnUnstampedBuildIsNotEvidenceOfAgreement(t *testing.T) {
	// EVERY PART UNSTAMPED, which is what a `go test` binary and a `go build` with
	// no VCS information both are.
	blind := trial{Notes: []trialNote{
		{Kind: trialOpened, Build: "unknown"},
		{Kind: trialState, Build: "unknown", Recorder: "unknown",
			Phase: "ready"},
		{Kind: trialRan, Build: "unknown", Reference: "run-1"}}}
	named, silent, _ := blind.builds()
	if len(named) != 0 {
		t.Errorf("an unstamped build is reported as a revision: %v", named)
	}
	if len(silent) != 3 {
		t.Errorf("the parts that said nothing about themselves are %v", silent)
	}
	if got := buildVerdict(blind); len(got) != 1 ||
		!strings.Contains(got[0], "NOT STAMPED") {
		t.Errorf("the verdict on three unstamped builds is %q", got)
	}

	// AND THREE PARTS OF ONE BUILD READ AS ONE BUILD, naming all three.
	same := trial{Notes: []trialNote{
		{Kind: trialOpened, Build: "34f6122"},
		{Kind: trialState, Build: "34f6122", Recorder: "34f6122",
			Phase: "captured"},
		{Kind: trialRan, Build: "34f6122", Ran: "34f6122",
			Reference: "run-1"}}}
	named, silent, agree := same.builds()
	if !agree || len(named["34f6122"]) != 3 || len(silent) != 0 {
		t.Fatalf("one build throughout reads as %v, silent %v, agree %v",
			named, silent, agree)
	}
	got := buildVerdict(same)
	if len(got) != 1 || !strings.Contains(got[0], "one build throughout: 34f6122") {
		t.Errorf("the verdict on one build is %q", got)
	}
	for _, who := range []string{"the window", "the recorder",
		"the director that performed the run"} {
		if !strings.Contains(got[0], who) {
			t.Errorf("the verdict does not name %s: %q", who, got[0])
		}
	}
}

// A PRESS WHOSE RESULT WAS MEASURED AND EMPTY IS SAID AS ITSELF.
//
// The known limitation, and the negative trial: a press that opens a map, a video, or anything
// else with nothing readable on it can be demonstrated and cannot anchor an Act. The trial has
// to report that as the explained result it is, rather than as a recording that merely never
// reached a verdict.
func TestTheVerdictTellsAnEmptyResultFromOneStillComing(t *testing.T) {
	why := "that press opened a different screen and only 2 word(s) could be read " +
		"on it that stayed"
	for _, c := range []struct {
		name  string
		state trialNote
		says  string
	}{{
		name: "measured and empty",
		state: trialNote{Kind: trialState, Phase: "unchecked", Presses: 1,
			Watched: 1, Unchecked: 1, Missing: why},
		says: "measured and empty",
	}, {
		name: "still coming",
		state: trialNote{Kind: trialState, Phase: "checking", Presses: 1,
			Watched: 1},
		says: "no press was checked",
	}, {
		name: "checked",
		state: trialNote{Kind: trialState, Phase: "captured", Presses: 1,
			Watched: 1, Captured: 1},
		says: "1 press(es) checked",
	}} {
		held := trial{Notes: []trialNote{
			{Kind: trialRecording, Name: "a map"}, c.state}}
		got := statesVerdict(held)
		if !strings.Contains(got, c.says) {
			t.Errorf("%s: the verdict is %q, which does not say %q",
				c.name, got, c.says)
		}
	}
	// AND THE EXPLANATION TRAVELS WITH IT, which is the whole of what makes the
	// negative trial readable: the number says a press was not checked and the
	// sentence says a screen with nothing on it cannot be.
	held := trial{Notes: []trialNote{{Kind: trialRecording, Name: "a map"},
		{Kind: trialState, Phase: "unchecked", Presses: 1, Watched: 1,
			Unchecked: 1, Missing: why}}}
	if got := statesVerdict(held); !strings.Contains(got, why) {
		t.Errorf("the verdict does not carry why: %q", got)
	}
}

// A STORE NOBODY HAS USED STUDIO AGAINST HAS NO TRIALS, AND READING ONE WRITES NOTHING.
//
// `-acts` builds the same surface to report what a store would show. A diagnostic that left a
// file in the store it was pointed at would be changing the thing it was asked about — and the
// journal is lazy for exactly that reason.
func TestReadingAStoreLeavesNoTrialInIt(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if code := runActsCheck(a.home); code != 0 {
		t.Fatalf("-acts over the fixture's store exited %d", code)
	}
	// AND THE SURFACE'S OWN READ-ONLY VERBS, which is what `-acts` drives.
	a.Demonstrations()
	a.Saved()
	a.History()
	a.Watch()
	a.Learning()
	if _, err := os.Stat(filepath.Join(a.home, "trials")); !os.IsNotExist(err) {
		all, _ := trialsIn(a.home)
		t.Fatalf("reading a store left %d journal(s) in it", len(all))
	}
	if code := runTrialCheck(a.home); code != 0 {
		t.Errorf("-trial over a store with no trials exited %d, and a store "+
			"nobody has recorded in is not a fault", code)
	}
}

// THE NOTE'S VERSION MOVES WITH ITS SHAPE.
//
// The same handshake `learnstate` has, for the reason learnstate's comment gives: `omitempty`
// makes a writer that has never heard of a field indistinguishable from one whose value is
// false, so the version is the only thing that lets a reader tell a missing fact from a negative
// one. A journal is read by whatever build is installed when somebody asks what happened, which
// is not always the build that wrote it.
func TestTheTrialNoteShapeMovesWithItsVersion(t *testing.T) {
	shapes := map[int]string{
		1: "version,seq,at,kind,build,recorder,ran,mismatched,director,store," +
			"routes,name,application,seconds,phase,doing,presses,watched," +
			"captured,unchecked,missing,start_known,screen_read,said,next," +
			"failed,screens,controls,asked,act,from,steps,error,live," +
			"reference,outcome,because,actions_sent,verified",
	}
	var names []string
	ty := reflect.TypeOf(trialNote{})
	for i := 0; i < ty.NumField(); i++ {
		tag := ty.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			t.Fatalf("%s carries no json tag, so what a reader sees is the "+
				"Go field name", ty.Field(i).Name)
		}
		names = append(names, strings.Split(tag, ",")[0])
	}
	got := strings.Join(names, ",")
	want, known := shapes[trialVersion]
	if !known {
		t.Fatalf("trialVersion is %d and no shape is recorded for it. Add this "+
			"one:\n  %d: %q", trialVersion, trialVersion, got)
	}
	if got != want {
		t.Errorf("the note's shape is not the one version %d describes.\n"+
			"  version %d: %s\n  actual:     %s\nMove the version and add "+
			"the new shape.", trialVersion, trialVersion, want, got)
	}
	// AND A LINE FROM A SHAPE THIS BUILD DOES NOT KNOW IS COUNTED, NOT GUESSED AT.
	dir := t.TempDir()
	path := filepath.Join(dir, "20261021-000000.jsonl")
	if err := os.WriteFile(path, []byte(
		`{"version":`+strconv.Itoa(trialVersion)+`,"seq":1,"kind":"opened",`+
			`"build":"aaaaaaa"}`+"\n"+
			`{"version":`+strconv.Itoa(trialVersion+1)+
			`,"seq":2,"kind":"opened","build":"zzzzzzz"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	held := readTrial(path)
	if len(held.Notes) != 1 || held.Unreadable != 1 {
		t.Fatalf("a journal of one readable and one foreign line parsed as "+
			"%d note(s) and %d unreadable", len(held.Notes), held.Unreadable)
	}
	if named, _, _ := held.builds(); len(named["zzzzzzz"]) != 0 {
		t.Errorf("a build named only by a line this build cannot read is " +
			"reported as having taken part")
	}
}
