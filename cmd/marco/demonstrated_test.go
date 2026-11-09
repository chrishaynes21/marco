package main

// TYPING THE NAME OF SOMETHING YOU DEMONSTRATED, AND HAVING IT HAPPEN.
//
// # The condition
//
// Both halves of the product were validated and neither could reach the other. A person shows
// Marco a game menu by hand and saves it under a name; the Director's supervised runner can
// perform exactly that, one authorised action at a time. Typing the name went to the one door,
// which knew about plays, found none, and handed the words to Director — which read them against
// the screen and reported that nothing matching them was there.
//
// So the product could be demonstrated and could not be used, and the only path to the thing a
// person had taught was a developer's command line.
//
// # What these drive
//
// The production `runInvocation`, the production door with BOTH stores behind it, the production
// resolver reading a real demo store in a temporary MARCO_HOME, and the production argument list.
// The one substitution is the spawn: `marco do` must be exercisable without launching a process
// that drives the real mouse.

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aPixelDemonstration is the shape `director demonstrate-visual` saves: named, scoped to the
// application it watched, one step with an outcome expectation, and a control carrying the pixels
// it was recognised by — and no verb anywhere, which is what makes it a routine performed by
// SIGHT rather than run.
func aPixelDemonstration(name string) workflowstore.Artifact {
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: name,
		SavedAt:     time.Date(2026, 10, 19, 20, 0, 0, 0, time.UTC),
		Application: workflowstore.NormaliseScope("arkascended"),
		Screens:     []workflowstore.Screen{{Label: "screen_a"}},
		Controls: []workflowstore.Control{{Label: "control_1",
			Readings: []observe.Region{
				{X: 0.410, Y: 0.241, Width: 0.072, Height: 0.028}},
			Text: []string{"GRAPHICS"}}},
		Steps: []workflowstore.Step{{
			Position: 0, From: "screen_a", To: "screen_a", Control: "control_1",
			Cause: observe.CauseClick, Seen: 1,
			Outcome: []string{"bloom", "foliage", "presets", "shadows", "textures"},
		}},
	}
}

// aHomeHolding puts demonstrations in a temporary MARCO_HOME and points this process at it.
func aHomeHolding(t *testing.T, arts ...workflowstore.Artifact) {
	t.Helper()
	t.Setenv("MARCO_HOME", t.TempDir())
	store, err := demo.Open(directorDir())
	if err != nil {
		t.Fatalf("opening a store in a temporary home: %v", err)
	}
	for _, a := range arts {
		if err := store.SaveSequence(a); err != nil {
			t.Fatalf("saving %q: %v", a.Name, err)
		}
	}
}

// aSpawnThatAnswers stands where the runner would, and records what it was asked to run.
//
// It writes a transcript shaped like the real one — the reference first, as the runner prints it
// before anything happens, and again at the end with the outcome — because the outcome a front
// end reports is read out of exactly those lines.
func aSpawnThatAnswers(t *testing.T, out runref.Outcome) *[]string {
	t.Helper()
	var asked []string
	was := spawnRunner
	t.Cleanup(func() { spawnRunner = was })
	spawnRunner = func(args []string, w io.Writer) (runref.Reference, error) {
		asked = args
		id, path := "run-20261019-201500-abcd", `C:\home\runs\run-20261019-201500-abcd.json`
		io.WriteString(w, runref.Cite(id, path)+"\n")
		io.WriteString(w, "  the control was pressed\n")
		if out != "" {
			io.WriteString(w, runref.Ended(id, path, out)+"\n")
		}
		return runref.Reference{ID: id, Path: path, Outcome: out}, nil
	}
	return &asked
}

// depsForIntake is the least that lets `runInvocation` run: a registry with no plays in it, and
// nothing that could reach a real Director.
func depsForIntake(t *testing.T) orchestrator.Deps {
	t.Helper()
	// NO PENDING QUESTION, asked of nothing. The production `pendingQuestion` dials a
	// socket, and a developer with a Director running on their own desktop would
	// otherwise have their session decide this test.
	wasPending := pendingQuestion
	t.Cleanup(func() { pendingQuestion = wasPending })
	pendingQuestion = func() bool { return false }
	return orchestrator.Deps{Reg: routes.Registry{Dir: t.TempDir()}}
}

// A DEMONSTRATED ROUTINE IS REACHED BY ITS NAME, THROUGH THE ONE DOOR.
//
// Mutation: hand `invoke.Decide` a nil sights resolver again. The words then belong to Director
// and this fails.
func TestADemonstratedRoutineIsReachedByItsName(t *testing.T) {
	aHomeHolding(t, aPixelDemonstration("ark graphics live"))
	asked := aSpawnThatAnswers(t, runref.Verified)
	d := depsForIntake(t)

	got, err := runInvocation(d, invoke.Request{
		Text: "ark graphics live", Source: invoke.SourceTyped})
	if err != nil {
		t.Fatalf("the invocation returned an error: %v", err)
	}
	if got != OutcomePerformed {
		t.Fatalf("the outcome is %q, want %q", got, OutcomePerformed)
	}
	if len(*asked) == 0 {
		t.Fatal("the supervised runner was never asked to do anything, so the " +
			"name reached something else")
	}
	// AND THE NAME THE STORE ANSWERED TO IS WHAT IT WAS ASKED FOR, case folded back to
	// the saved spelling: the runner looks the name up again and anything else is a
	// second chance to miss.
	line := strings.Join(*asked, " ")
	if !strings.Contains(line, `--name ark graphics live`) {
		t.Errorf("the runner was asked for %q", line)
	}
}

// AND THE SAVED SPELLING TRAVELS, however the person typed it.
func TestTheSavedSpellingIsWhatThePerformerIsGiven(t *testing.T) {
	aHomeHolding(t, aPixelDemonstration("Ark Graphics Live"))
	asked := aSpawnThatAnswers(t, runref.Verified)
	d := depsForIntake(t)
	if _, err := runInvocation(d, invoke.Request{Text: "ark graphics live"}); err != nil {
		t.Fatal(err)
	}
	if line := strings.Join(*asked, " "); !strings.Contains(line, "--name Ark Graphics Live") {
		t.Errorf("the runner was asked for %q, not the name the store holds", line)
	}
}

// AND IT IS PERFORMED WITH THE FLAGS IT NEEDS, every one of which is load-bearing.
//
// # Why each one is asserted
//
//   - `--name` and `--application`: the routine, and the window to watch. Without the
//     application the runner reads the FOREGROUND — which, at the moment somebody types a name
//     into the overlay, is the overlay.
//   - `--live`: a person asking Marco to do the thing they demonstrated is asking for it to
//     happen. Without it the whole path runs and nothing is pressed, and the front end would
//     report a press that was never sent.
//   - `--hotkeys`: the questions are answered by function key. Without it the runner asks on its
//     own standard input, and a child spawned by a front end has no terminal to answer on, so
//     every action hangs until its question expires.
//   - `--reading`: the words on each look. The outcome expectation IS words, and two visually
//     similar menus are told apart by what they say.
func TestADemonstrationIsPerformedWithTheFlagsItNeeds(t *testing.T) {
	args := performWorkflowArgs(invoke.Demonstrated{
		Name: "ark graphics live", Application: "arkascended"})
	if len(args) == 0 || args[0] != "perform-workflow" {
		t.Fatalf("the sub-command is %v", args)
	}
	line := strings.Join(args, " ")
	for _, want := range []string{
		"--name ark graphics live",
		"--application arkascended",
		"--live",
		"--hotkeys",
		"--reading",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the invocation %q is missing %q", line, want)
		}
	}
	// AND NOTHING THAT WOULD AUTHORISE MORE THAN ONE ACTION AT A TIME.
	//
	// `--explore` authorises the whole session at launch. It is a measurement flag and
	// it must never be what a typed name gets: every action on this path is agreed to
	// individually, by a key this process cannot send.
	if strings.Contains(line, "--explore") {
		t.Error("a typed name authorises a whole session")
	}
}

// AND A VERB ROUTINE IS NOT CLAIMED BY THE SIGHT PATH.
//
// The compatibility guarantee, from the other side. A routine with a verb on every step is run by
// the ordinary execution path and reached through Director as it always was; claiming it here
// would change what an existing name does.
func TestAVerbRoutineIsNotClaimedBySight(t *testing.T) {
	a := aPixelDemonstration("desktop tour")
	a.Steps[0].Verb = "click"
	aHomeHolding(t, a)
	if d, ok := (demoSights{dir: directorDir()}).Demonstration("", "desktop tour"); ok {
		t.Errorf("a verb play was offered as something to perform by sight: %+v", d)
	}
}

// AND NEITHER IS A ROUTINE THE RUNNER WOULD REFUSE.
//
// A promise the runner then declines is, from the person's side, Marco accepting a request and
// then arguing with itself. So the door only claims what can actually happen: an UNSCOPED routine
// is valid for no application, and one with no visual evidence has nothing to ground.
func TestADemonstrationTheRunnerWouldRefuseIsNotClaimed(t *testing.T) {
	unscoped := aPixelDemonstration("unscoped play")
	unscoped.Application = ""
	blind := aPixelDemonstration("no pixels")
	blind.Controls[0].Readings = nil
	aHomeHolding(t, unscoped, blind)

	sights := demoSights{dir: directorDir()}
	for _, name := range []string{"unscoped play", "no pixels", "nothing saved under this"} {
		if d, ok := sights.Demonstration("", name); ok {
			t.Errorf("%q was offered as performable: %+v", name, d)
		}
	}
	// AND THE ONE THAT CAN BE PERFORMED STILL IS, so this test cannot pass by
	// refusing everything.
	aHomeHolding(t, aPixelDemonstration("ark graphics live"))
	if _, ok := (demoSights{dir: directorDir()}).Demonstration("", "ark graphics live"); !ok {
		t.Error("a scoped pixel play was not offered as performable")
	}
}

// AND EVERY WAY A RUN CAN END BECOMES A WORD EVERY SURFACE ALREADY READS.
//
// One case per outcome, from the whole vocabulary rather than a list written here, so a word
// added to the runner cannot quietly arrive as an unknown. The two arms worth reading twice:
// CONTRADICTED is `failed` rather than `unverified`, because the evidence decided against rather
// than failing to decide; and `started` is `failed`, because a run that reserved a reference and
// wrote no outcome is a defect in the runner and must not read as success.
func TestEveryWayARunCanEndBecomesAWordTheSurfacesKnow(t *testing.T) {
	want := map[runref.Outcome]Outcome{
		runref.Verified:     OutcomePerformed,
		runref.Unconfirmed:  OutcomeUnverified,
		runref.Contradicted: OutcomeFailed,
		runref.Refused:      OutcomeRefused,
		runref.Cancelled:    OutcomeCancelled,
		runref.Failed:       OutcomeFailed,
		runref.Started:      OutcomeFailed,
	}
	for _, out := range runref.Every {
		expect, named := want[out]
		if !named {
			t.Fatalf("the runner can end in %q and nothing here says what that "+
				"means to a person", out)
		}
		if got := marcoWord(out); got != expect {
			t.Errorf("%q became %q, want %q", out, got, expect)
		}
	}
	// AND A WORD THIS BUILD DOES NOT KNOW IS NOT SUCCESS. A front end that read an
	// unfamiliar outcome as performed would announce a press nobody can account for.
	if got := marcoWord(runref.Outcome("something new")); got == OutcomePerformed {
		t.Error("an unknown outcome was reported as performed")
	}
}

// AND THE OUTCOME TRAVELS ALL THE WAY TO THE PERSON, through the production path.
func TestTheRunsOwnOutcomeIsWhatTheInvocationReports(t *testing.T) {
	for _, tc := range []struct {
		ran  runref.Outcome
		want Outcome
	}{
		{runref.Unconfirmed, OutcomeUnverified},
		{runref.Refused, OutcomeRefused},
		{runref.Cancelled, OutcomeCancelled},
	} {
		t.Run(string(tc.ran), func(t *testing.T) {
			aHomeHolding(t, aPixelDemonstration("ark graphics live"))
			aSpawnThatAnswers(t, tc.ran)
			d := depsForIntake(t)
			got, err := runInvocation(d, invoke.Request{Text: "ark graphics live"})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("a %q run was reported as %q, want %q",
					tc.ran, got, tc.want)
			}
		})
	}
}

// AND A RUN THAT SAID NOTHING ABOUT HOW IT ENDED IS NOT A SUCCESS.
//
// The reference is the one line whose job is to say. Its absence is not "it worked": it is a run
// nobody can retrieve, which is exactly the state ADR-240 made findable rather than quiet.
func TestARunThatPrintedNoOutcomeIsNotPerformed(t *testing.T) {
	aHomeHolding(t, aPixelDemonstration("ark graphics live"))
	aSpawnThatAnswers(t, "")
	d := depsForIntake(t)
	got, err := runInvocation(d, invoke.Request{Text: "ark graphics live"})
	if err != nil {
		t.Fatal(err)
	}
	if got == OutcomePerformed {
		t.Error("a run that never said how it ended was reported as performed")
	}
}

// AND A RUNNER THAT COULD NOT BE STARTED IS "NOBODY TOOK IT".
//
// A different thing from a run that refused, and the difference matters to a person: one means
// Marco declined to act, the other means the request never arrived anywhere. See
// outcome.Unavailable.
func TestARunnerThatCannotBeStartedIsUnavailable(t *testing.T) {
	aHomeHolding(t, aPixelDemonstration("ark graphics live"))
	was := spawnRunner
	t.Cleanup(func() { spawnRunner = was })
	spawnRunner = func([]string, io.Writer) (runref.Reference, error) {
		return runref.Reference{}, errors.New("director.exe is not here")
	}
	d := depsForIntake(t)
	got, err := runInvocation(d, invoke.Request{Text: "ark graphics live"})
	if err != nil {
		t.Fatal(err)
	}
	if got != OutcomeUnavailable {
		t.Errorf("the outcome is %q, want %q", got, OutcomeUnavailable)
	}
}

// AND THE TRANSCRIPT REACHES THE PERSON WHOLE, with the reference read out of it.
//
// Both halves, because either one alone is a quiet failure: a reference picked out of a
// transcript nobody was shown is a run with no account of itself, and a transcript passed through
// with the reference unread is a front end that cannot say how anything ended.
func TestTheTranscriptIsPassedThroughAndTheReferenceReadOutOfIt(t *testing.T) {
	id, path := "run-20261019-201500-abcd", `C:\Users\A Person\marco\runs\run-abcd.json`
	transcript := strings.Join([]string{
		`Performing "ark graphics live" against arkascended (hwnd:1)`,
		runref.Cite(id, path),
		"  Starting in 5s.",
		"  step 1: the control was pressed",
		runref.Ended(id, path, runref.Unconfirmed),
	}, "\n") + "\n"

	var shown bytes.Buffer
	ref := referenceFrom(strings.NewReader(transcript), &shown)

	if shown.String() != transcript {
		t.Errorf("the transcript the person sees is not the one the runner "+
			"wrote:\n%s", shown.String())
	}
	if ref.ID != id {
		t.Errorf("the reference is %q", ref.ID)
	}
	// AND THE PATH SURVIVES A SPACE IN SOMEBODY'S NAME, which is an ordinary thing for
	// a Windows path to contain and would be cut in half by splitting on whitespace.
	if ref.Path != path {
		t.Errorf("the record's path came back as %q", ref.Path)
	}
	if ref.Outcome != runref.Unconfirmed {
		t.Errorf("the outcome came back as %q", ref.Outcome)
	}
	// AND THE OPENING CITATION DOES NOT ERASE AN OUTCOME THAT HAS ALREADY ARRIVED.
	later := transcript + runref.Cite(id, path) + "\n"
	if got := referenceFrom(strings.NewReader(later), io.Discard); got.Outcome != runref.Unconfirmed {
		t.Errorf("a later line with no outcome left the reference saying %q",
			got.Outcome)
	}
}

// ── WHAT HAPPENED, AT INFO, IN THE LOG A PERSON ACTUALLY READS ───────────────

// recordedLog swaps the default logger for a buffer a test can read.
func recordedLog(t *testing.T) *strings.Builder {
	t.Helper()
	var b strings.Builder
	was := mlog.Default
	t.Cleanup(func() { mlog.Default = was })
	mlog.Default = mlog.New(&b, mlog.LevelInfo)
	return &b
}

// EVERY INVOCATION SAYS HOW IT ENDED.
//
// # The condition
//
// The log said what was asked for and never what happened. Watching the console behind the
// overlay, a person saw their own words go in — `overlay: intake`, then `intake: routed` — and
// then nothing at all. The outcome went to the HUD's status line and history row, which are a
// glance and are gone by the time anybody is debugging, and to the `[result]` wire line, which is
// for the front end rather than for a person.
//
// So a run that failed, a run that was refused, and a run that was never dispatched were the same
// thing in the log: silence after the request.
//
// Mutation: delete the `intake: finished` line, or move it into one arm instead of after all of
// them. This fails.
func TestEveryInvocationSaysHowItEndedAtInfo(t *testing.T) {
	for _, tc := range []struct {
		name   string
		set    func(t *testing.T)
		req    invoke.Request
		decide string
		out    Outcome
	}{
		{
			name: "something demonstrated, performed",
			set: func(t *testing.T) {
				aHomeHolding(t, aPixelDemonstration("ark graphics live"))
				aSpawnThatAnswers(t, runref.Verified)
			},
			req:    invoke.Request{Text: "ark graphics live"},
			decide: "demonstration",
			out:    OutcomePerformed,
		},
		{
			name: "something demonstrated, refused",
			set: func(t *testing.T) {
				aHomeHolding(t, aPixelDemonstration("ark graphics live"))
				aSpawnThatAnswers(t, runref.Refused)
			},
			req:    invoke.Request{Text: "ark graphics live"},
			decide: "demonstration",
			out:    OutcomeRefused,
		},
		{
			// THE WORD, which reaches a different half and must also report.
			name:   "a stop",
			set:    func(t *testing.T) { t.Setenv("MARCO_HOME", t.TempDir()) },
			req:    invoke.Request{Text: "stop"},
			decide: "control",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.set(t)
			said := recordedLog(t)
			d := depsForIntake(t)
			out, err := runInvocation(d, tc.req)
			if err != nil {
				t.Fatal(err)
			}
			log := said.String()
			// RENDERED, not as written: mlog turns the "component: detail" convention
			// into "component - detail", which is what a person greps for.
			if !strings.Contains(log, "intake - finished") {
				t.Fatalf("nothing in the log says how it ended:\n%s", log)
			}
			for _, want := range []string{
				"decision=" + tc.decide,
				"outcome=" + string(out),
			} {
				if !strings.Contains(log, want) {
					t.Errorf("the log does not carry %q:\n%s", want, log)
				}
			}
			if tc.out != "" && out != tc.out {
				t.Errorf("outcome %q, want %q", out, tc.out)
			}
		})
	}
}

// AND A SUPERVISED RUN'S REFERENCE IS IN THERE TOO.
//
// It is the one thing that retrieves the attempt, and a person reading a log to find out what
// happened should not have to go hunting through a transcript to learn that the record exists or
// where it went.
func TestTheRunReferenceIsLoggedAtInfo(t *testing.T) {
	aHomeHolding(t, aPixelDemonstration("ark graphics live"))
	aSpawnThatAnswers(t, runref.Unconfirmed)
	said := recordedLog(t)
	if _, err := runInvocation(depsForIntake(t),
		invoke.Request{Text: "ark graphics live"}); err != nil {
		t.Fatal(err)
	}
	log := said.String()
	for _, want := range []string{"run=run-20261019-201500-abcd", "ran=unconfirmed"} {
		if !strings.Contains(log, want) {
			t.Errorf("the log does not carry %q:\n%s", want, log)
		}
	}
}

// ── ASKING WHERE WORDS WOULD GO, WITHOUT GOING THERE ────────────────────────

// saidByExplain runs the production explanation and returns what it printed.
func saidByExplain(t *testing.T, req invoke.Request) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	explainInvocation(depsForIntake(t), req)
	os.Stdout = saved
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// TestExplainSaysWhereTheWordsWouldGoAndPerformsNothing.
//
// # The condition
//
// A demonstrated routine, typed into the widget, and the surface said *failed*. The reason was
// three processes away and the only way to watch the decision was to make it, because `marco do`
// performs. So finding out which half of the product had taken the words meant reading source —
// and the answer, when it came, was that a recorded play of the same name had answered first.
//
// The one thing this must never do is act. A diagnostic that performed the thing it was
// describing would be a worse version of the command it is meant to replace.
func TestExplainSaysWhereTheWordsWouldGoAndPerformsNothing(t *testing.T) {
	aHomeHolding(t, aPixelDemonstration("ark graphics live"))
	asked := aSpawnThatAnswers(t, runref.Verified)

	said := saidByExplain(t, invoke.Request{Text: "ark graphics live"})

	for _, want := range []string{
		`"ark graphics live"`,
		"demonstration",
		"perform-workflow",
		"--live",
		"--hotkeys",
		navsource.ConsentKeys[navsource.ConsentApprove],
	} {
		if !strings.Contains(said, want) {
			t.Errorf("the explanation does not carry %q:\n%s", want, said)
		}
	}
	// AND NOTHING RAN. The one property that makes this safe to type at any moment.
	if len(*asked) != 0 {
		t.Fatalf("explaining performed the run: %v", *asked)
	}
}

// AND IT NAMES A DEMONSTRATION THE PLAY ARM IS SHADOWING.
//
// The live failure, in one line. One name, two artifacts: a recorded play of raw coordinates and
// a pixel demonstration with screens, control readings and an outcome expectation. The play arm
// answers first, so typing the name ran the coordinates — and from the outside the two were
// indistinguishable, because nothing said which of the two things you taught had just happened.
func TestExplainNamesTheDemonstrationAPlayIsShadowing(t *testing.T) {
	aHomeHolding(t, aPixelDemonstration("ark graphics live"))
	d := depsForIntake(t)
	// A PLAY OF THE SAME NAME, which is what `learn <name>` leaves behind.
	shadow := routes.Route{App: "arkascended", Slug: "ark-graphics-live"}
	said := saidByExplainWith(t, d, shadow, invoke.Request{
		Text: "ark graphics live", App: "arkascended"})

	if !strings.Contains(said, "decision     play") {
		t.Fatalf("the play did not answer first, so this is not the shadowing "+
			"case:\n%s", said)
	}
	for _, want := range []string{"ALSO saved", "perform-workflow"} {
		if !strings.Contains(said, want) {
			t.Errorf("the explanation does not say a demonstration is being "+
				"shadowed (%q):\n%s", want, said)
		}
	}
}

// AND IT SAYS WHY A SAVED ROUTINE CANNOT BE RUN BY SIGHT.
//
// A routine that exists and cannot be performed is indistinguishable, from the outside, from a
// name nobody ever saved — and the two send a person to completely different places: one to
// demonstrate it, the other to fix what is wrong with the one they have.
func TestExplainSaysWhyASavedRoutineCannotBeRunBySight(t *testing.T) {
	unscoped := aPixelDemonstration("ark menu 2")
	unscoped.Application = ""
	aHomeHolding(t, unscoped)

	said := saidByExplain(t, invoke.Request{Text: "ark menu 2"})
	if !strings.Contains(said, "decision     director") {
		t.Fatalf("an unscoped play was claimed after all:\n%s", said)
	}
	for _, want := range []string{"IS saved", "UNSCOPED", "demonstrate-visual"} {
		if !strings.Contains(said, want) {
			t.Errorf("the explanation does not say why (%q):\n%s", want, said)
		}
	}
}

// saidByExplain with a registry that answers for one play, so the shadowing case can be driven
// without writing Marco source to disk.
func saidByExplainWith(t *testing.T, d orchestrator.Deps, rt routes.Route,
	req invoke.Request) string {

	t.Helper()
	dir := t.TempDir()
	sub := filepath.Join(dir, rt.App, "context")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// THE PLAY AS A FILE, because that is what the registry reads. A recorded play's
	// source is exactly this shape -- see `marco learn`.
	src := "// a recorded play\nuse os.\n\nthe T is an actor.\n\nthis can Run.\n" +
		"this's Run does...\n    the p1 is a Point with X 10, Y 10, RelX 10, RelY 10.\n" +
		"    do OS's Click with p1.\n    this is ok!\n\nthe App is a script.\n\n" +
		"do T's Run...\n    when ok?\n        log \"done\".\n    or?\n        log that's error.\n"
	if err := os.WriteFile(filepath.Join(sub, rt.Slug+".marco"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	d.Reg = routes.Registry{Dir: dir}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	explainInvocation(d, req)
	os.Stdout = saved
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// AND EXPLAINING NEVER PERFORMS.
//
// The fork itself, not the function behind it. `--explain` sets a flag and the branch that
// explains has to LEAVE — a missing return would explain the invocation and then carry it out,
// which is the worst possible failure for a diagnostic somebody types because they are unsure
// what is about to happen.
//
// Driven through `doOrExplain`, the production fork, with the spawn stubbed: if the explanation
// falls through, the runner is asked to perform and this says so.
func TestExplainingNeverPerforms(t *testing.T) {
	aHomeHolding(t, aPixelDemonstration("ark graphics live"))
	asked := aSpawnThatAnswers(t, runref.Verified)
	d := depsForIntake(t)
	req := invoke.Request{Text: "ark graphics live"}

	if code := doOrExplain(d, req, true); code != 0 {
		t.Errorf("explaining exited %d", code)
	}
	if len(*asked) != 0 {
		t.Fatalf("explaining performed the run: %v", *asked)
	}

	// AND WITHOUT THE FLAG THE SAME FORK DOES PERFORM, so this test cannot pass
	// against a build where nothing works at all.
	if code := doOrExplain(d, req, false); code != 0 {
		t.Errorf("performing a verified run exited %d", code)
	}
	if len(*asked) == 0 {
		t.Fatal("without --explain the run was not performed")
	}
}

// AND THE ENTRANCE NEVER PUTS A FLAG IN THE PHRASE.
//
// An unrecognised flag falls into the words. The phrase then matches nothing, the invocation goes
// to Director, and it is PERFORMED — so a `--explain` nobody parsed is the one thing that makes
// the thing happen. Held over every flag the entrance takes, not just the new one.
func TestTheEntranceNeverPutsAFlagInThePhrase(t *testing.T) {
	req, explain, window, err := parseDoArgs([]string{"--explain", "--source=typed",
		"ark", "graphics", "live"})
	if err != nil {
		t.Fatalf("parseDoArgs: %v", err)
	}
	if window != "" {
		t.Errorf("a window came out of a command that named none: %q", window)
	}
	if !explain {
		t.Error("--explain was not recognised, so it is part of the phrase and " +
			"the invocation would be performed instead of described")
	}
	if req.Text != "ark graphics live" {
		t.Errorf("the phrase came out as %q", req.Text)
	}
	if req.Source != invoke.SourceTyped {
		t.Errorf("the source came out as %q", req.Source)
	}
	// AND THE OTHER FLAGS STILL DO WHAT THEY DID.
	req, explain, window, err = parseDoArgs([]string{"--play=open-mouse-settings",
		"--app=settings", "--focus", "--window=4660:77",
		"--arg=message=hello there", "--expect=abc123def456"})
	if err != nil {
		t.Fatalf("parseDoArgs: %v", err)
	}
	if window != "4660:77" {
		t.Errorf("the window came out as %q, so it is either lost or in the phrase",
			window)
	}
	if explain {
		t.Error("nothing asked for an explanation")
	}
	if req.Play == nil || req.Play.Slug != "open-mouse-settings" ||
		req.Play.App != "settings" || !req.Play.Focus {
		t.Errorf("the identity came out as %+v", req.Play)
	}
	if req.Text != "" {
		t.Errorf("a flag ended up in the phrase: %q", req.Text)
	}
	// AND THE VALUE CAME OUT AS A VALUE, with its spaces, rather than as words.
	//
	// A `--arg` that fell into the phrase would be the worst of all of them: the phrase
	// would then match nothing, the invocation would go to Director, and the text somebody
	// supplied would have become part of a request about the world.
	if got, ok := req.Named["message"]; !ok || got != "hello there" {
		t.Errorf("the value came out as %q (present=%v)", got, ok)
	}
	if len(req.Flagged) != 1 || req.Flagged[0] != "message" {
		t.Errorf("the entrance did not record which values arrived as flags: %q",
			req.Flagged)
	}
	// AND THE VERSION THIS RUN WAS CONFIRMED AGAINST came out as a version.
	//
	// An unparsed `--expect=` would be the same hazard as an unparsed `--explain`: it
	// falls into the words, the phrase then matches nothing, and the invocation goes to
	// Director and is PERFORMED -- the version pin turning into the thing that removes
	// every check it was added to make.
	if req.Expect != "abc123def456" {
		t.Errorf("the confirmed version came out as %q", req.Expect)
	}
}

// AND A VALUE GIVEN TWICE IS REFUSED RATHER THAN OVERWRITTEN.
//
// A map cannot hold the second, and nothing after this point could notice: one of two things
// somebody typed would be typed and the other silently would not. So it is caught here, where the
// flags are still a list — and a malformed one is refused for the same reason, because the
// alternative is that it falls into the phrase and gets performed.
func TestADuplicateOrMalformedValueFlagIsRefused(t *testing.T) {
	for _, args := range [][]string{
		{"--play=x", "--arg=message=one", "--arg=message=two"},
		{"--play=x", "--arg=message=one", "--arg=MESSAGE=two"},
		{"--play=x", "--arg=message"},
		{"--play=x", "--arg="},
		{"--play=x", "--arg==value"},
	} {
		if _, _, _, err := parseDoArgs(args); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
	// AND AN EMPTY VALUE IS NOT MALFORMED. `--arg=message=` is somebody saying "type
	// nothing here", which is a value.
	req, _, _, err := parseDoArgs([]string{"--play=x", "--arg=message="})
	if err != nil {
		t.Fatalf("an explicitly empty value was refused: %v", err)
	}
	if got, ok := req.Named["message"]; !ok || got != "" {
		t.Errorf("the empty value came out as %q (present=%v)", got, ok)
	}
}
