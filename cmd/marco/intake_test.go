package main

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The ONE intake, proved through the function every entrance actually enters.
//
// # Why these tests look like they are testing the same thing several times
//
// Because they are, from several entrances, and that IS the property. The defect this phase closed
// was not that any one path was wrong — each worked — but that TYPING and SPEAKING took different
// paths and reached different conclusions about the same words. A test per surface that never
// compares them across surfaces would have passed happily throughout.

const intakeSrc = "script main...\n  do nothing.\n"

// runnableSrc is a play that COMPILES AND RUNS and performs no input at all.
//
// Most tests here stop at the routing decision and never execute anything, so a source that only
// has to exist is enough for them. The subprocess test actually runs the play, and a fixture that
// fails to compile would make it report `failed` for a reason that has nothing to do with what it
// is measuring — which is exactly what happened the first time this was written.
const runnableSrc = `use os.

the Quiet is an actor.

this can Run.
this's Run does...
  do OS's Sleep with 1.
  this is ok!

the App is a script.

do Quiet's Run...
  when ok?
    log "done".
  or?
    log that's error.
`

// intakeWorld is a registry with one registered play, one staged play, and one play whose name
// does not survive a round trip through its own slug.
func intakeWorld(t *testing.T) orchestrator.Deps {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	t.Setenv("MARCO_HOME", t.TempDir())
	reg := routes.Registry{Dir: dir}
	for _, rt := range []routes.Route{
		{App: "settings", Focus: true, Slug: "open-mouse-settings"},
		{App: "notepad", Slug: "save-and-close"},
		{Slug: "open-dad-s-settings"},
	} {
		if err := reg.Save(rt, intakeSrc); err != nil {
			t.Fatal(err)
		}
	}
	staged := routes.Route{App: "settings", Focus: routes.LearnedFocus, Slug: "open-bluetooth"}
	err := reg.SaveStaged(staged, intakeSrc,
		routes.Origin{Kind: routes.KindLearned, Application: "settings"})
	if err != nil {
		t.Fatal(err)
	}
	d := newDeps()
	d.Reg = reg
	d.App = func() string { return "discord" } // an unrelated application in front
	return d
}

// decideAs is the routing half of one invocation from one entrance, with nothing performed.
//
// It stops at the decision on purpose: what this phase has to prove is WHERE a request goes, and
// running it would need a desktop.
func decideAs(t *testing.T, d orchestrator.Deps, src invoke.Source, text string) invoke.Decision {
	t.Helper()
	return invoke.Decide(d.Reg, invoke.Request{Text: text, Source: src, App: d.App()})
}

// Typed, spoken and CLI all reach the SAME play for a name Marco already has.
//
// Mutation (M1/M2/M20): give any entrance its own path. This fails.
func TestEveryEntranceReachesTheSamePlay(t *testing.T) {
	d := intakeWorld(t)
	var first invoke.Decision
	for i, src := range []invoke.Source{
		invoke.SourceTyped, invoke.SourceSpoken, invoke.SourceCLI,
		invoke.SourceHotkey, invoke.SourceControlCentre, invoke.SourceWeb,
	} {
		got := decideAs(t, d, src, "Open Mouse Settings")
		if got.Kind != invoke.KindPlay {
			t.Fatalf("%s: an exactly known play decided %q", src, got.Kind)
		}
		if i == 0 {
			first = got
			continue
		}
		if got.Play != first.Play {
			t.Fatalf("typed reached %+v and %s reached %+v — the same words mean two things",
				first.Play, src, got.Play)
		}
	}
	if first.Play.Slug != "open-mouse-settings" {
		t.Fatalf("resolved to %q", first.Play.Slug)
	}
}

// And an unknown request reaches Director from every entrance, with the same words.
//
// Mutation (M19): offer Learn on a miss instead of asking Director. This fails.
func TestEveryEntranceReachesDirectorOnAMiss(t *testing.T) {
	d := intakeWorld(t)
	const novel = "turn bluetooth off"
	var first invoke.Decision
	for i, src := range []invoke.Source{invoke.SourceTyped, invoke.SourceSpoken, invoke.SourceCLI} {
		got := decideAs(t, d, src, novel)
		if got.Kind != invoke.KindDirector {
			t.Fatalf("%s: a novel request decided %q — Director never saw it", src, got.Kind)
		}
		if i == 0 {
			first = got
			continue
		}
		if got.Phrase != first.Phrase {
			t.Fatalf("Director was told %q from typed and %q from %s", first.Phrase, got.Phrase, src)
		}
	}
	if first.Phrase != novel {
		t.Errorf("Director was told %q; the person said %q", first.Phrase, novel)
	}
}

// Case, whitespace and punctuation land on one identity, identically from every entrance.
func TestNormalizationIsTheSameFromEveryEntrance(t *testing.T) {
	d := intakeWorld(t)
	for _, text := range []string{
		"open mouse settings", "Open Mouse Settings", "OPEN MOUSE SETTINGS",
		"open-mouse-settings", "Open Mouse Settings!", "open mouse settings?",
		`"open mouse settings"`, "  open mouse settings  ", "open  mouse  settings",
	} {
		var first invoke.Decision
		for i, src := range []invoke.Source{invoke.SourceTyped, invoke.SourceSpoken, invoke.SourceCLI} {
			got := decideAs(t, d, src, text)
			if i == 0 {
				first = got
				continue
			}
			if got.Kind != first.Kind || got.Play != first.Play {
				t.Fatalf("%q: typed %+v, %s %+v", text, first, src, got)
			}
		}
		if first.Kind != invoke.KindPlay || first.Play.Slug != "open-mouse-settings" {
			t.Errorf("%q decided %q/%q", text, first.Kind, first.Play.Slug)
		}
	}
}

// An apostrophe in a name is not an obstacle to invoking it.
//
// `routes.Slug` turns the apostrophe into a word boundary, so the play is stored as
// open-dad-s-settings. Both spellings must still reach it, because a person says the name they
// gave it and has no idea what the file is called.
func TestAnApostropheDoesNotHideAPlay(t *testing.T) {
	d := intakeWorld(t)
	for _, text := range []string{"open dad's settings", "Open Dad's Settings", "open dad s settings"} {
		got := decideAs(t, d, invoke.SourceSpoken, text)
		if got.Kind != invoke.KindPlay || got.Play.Slug != "open-dad-s-settings" {
			t.Errorf("%q decided %q/%q", text, got.Kind, got.Play.Slug)
		}
	}
}

// A staged play never intercepts an invocation, from any entrance.
//
// Mutation (M4): let staged plays participate in exact resolution. This fails.
func TestAStagedPlayNeverInterceptsFromAnyEntrance(t *testing.T) {
	d := intakeWorld(t)
	for _, src := range []invoke.Source{invoke.SourceTyped, invoke.SourceSpoken, invoke.SourceCLI} {
		got := decideAs(t, d, src, "open bluetooth")
		if got.Kind != invoke.KindDirector {
			t.Fatalf("%s: a saved-but-unregistered play answered as %q", src, got.Kind)
		}
	}
}

// A registered play is reached BEFORE Director is consulted.
//
// Mutation (M3): consult Director first. This fails.
func TestARegisteredPlayInterceptsBeforeDirector(t *testing.T) {
	d := intakeWorld(t)
	// A FOCUS play answers from anywhere, so it intercepts with Discord in front.
	if got := decideAs(t, d, invoke.SourceSpoken, "open mouse settings"); got.Kind != invoke.KindPlay {
		t.Fatalf("a focus play decided %q — Director was asked about something Marco has",
			got.Kind)
	}
	// A CONTEXT play answers only in its own application, and that is not a failure to
	// intercept — it is the scope doing its job. With Notepad in front it intercepts; with
	// Discord in front the words are a request Director should read against the screen.
	inApp := invoke.Decide(d.Reg, invoke.Request{
		Text: "save and close", Source: invoke.SourceSpoken, App: "notepad"})
	if inApp.Kind != invoke.KindPlay || inApp.Play.Slug != "save-and-close" {
		t.Fatalf("in its own application a context play decided %q/%q",
			inApp.Kind, inApp.Play.Slug)
	}
	if away := decideAs(t, d, invoke.SourceSpoken, "save and close"); away.Kind != invoke.KindDirector {
		t.Fatalf("a context play answered from another application as %q — scope stopped "+
			"meaning anything", away.Kind)
	}
}

// A control phrase reaches the thing that is running, from every entrance, and is never a play.
//
// Mutation (M13): treat stop as ordinary semantic text. This fails.
func TestStopReachesTheRunningWorkFromEveryEntrance(t *testing.T) {
	d := intakeWorld(t)
	// Even with a play literally called "stop" registered.
	if err := d.Reg.Save(routes.Route{Slug: "stop"}, intakeSrc); err != nil {
		t.Fatal(err)
	}
	for _, src := range []invoke.Source{invoke.SourceTyped, invoke.SourceSpoken, invoke.SourceCLI} {
		if got := decideAs(t, d, src, "stop"); got.Kind != invoke.KindControl {
			t.Fatalf("%s: stop decided %q", src, got.Kind)
		}
	}
	// And it reaches the ACTIVE EXECUTION AUTHORITY rather than being interpreted.
	var asked bool
	prev := stopWhatIsRunning
	stopWhatIsRunning = func(bool) int { asked = true; return exitOK }
	t.Cleanup(func() { stopWhatIsRunning = prev })
	noPendingQuestion(t)
	prevSubmit := submitPhrase
	submitPhrase = func(string, bool) int {
		t.Fatal("a control phrase was submitted to Director as a request to interpret")
		return 0
	}
	t.Cleanup(func() { submitPhrase = prevSubmit })

	out, err := runInvocation(d, invoke.Request{Text: "stop", Source: invoke.SourceSpoken})
	if err != nil {
		t.Fatal(err)
	}
	if !asked {
		t.Fatal("stop did not reach the active execution authority")
	}
	if out != OutcomeCancelled {
		t.Errorf("stop reported %q", out)
	}
}

// A phrase that answers Director's pending question is not claimed by a play of the same name.
//
// Mutation: drop the pending check. This fails.
func TestAnAnswerReachesTheQuestionAndNothingElseDoes(t *testing.T) {
	d := intakeWorld(t)
	noDirector(t)
	prev := pendingQuestion
	pendingQuestion = func() bool { return true }
	t.Cleanup(func() { pendingQuestion = prev })

	var submitted string
	prevSubmit := submitPhrase
	submitPhrase = func(p string, _ bool) int { submitted = p; return exitOK }
	t.Cleanup(func() { submitPhrase = prevSubmit })

	// AN ANSWER goes to the question, and is not resolved against the registry.
	if _, err := runInvocation(d, invoke.Request{
		Text: "the second one", Source: invoke.SourceSpoken,
	}); err != nil {
		t.Fatal(err)
	}
	if submitted != "the second one" {
		t.Fatalf("an answer went somewhere other than the question (submitted %q)", submitted)
	}

	// This half actually PERFORMS the play, so it needs a fixture that compiles.
	if err := d.Reg.Save(routes.Route{App: "settings", Focus: true, Slug: "open-mouse-settings"},
		runnableSrc); err != nil {
		t.Fatal(err)
	}

	// AND A PLAY NAME IS STILL A PLAY. A question nobody answered must not capture every
	// invocation after it — measured live, it did, and typing the name of a play you have
	// answered `nothing matching "test" is present in the observed window`.
	//
	// Deleting the ParseClarification test in invoke.Decide must fail this.
	submitted = ""
	said, err := captureStdout(t, func() error {
		_, e := runInvocation(d, invoke.Request{
			Text: "open mouse settings", Source: invoke.SourceSpoken,
		})
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if submitted != "" {
		t.Fatalf("a play Marco has was delivered to Director as an answer to a question "+
			"nobody asked about it (submitted %q)", submitted)
	}
	if !strings.Contains(said, "[route] open mouse settings") {
		t.Errorf("the play did not run while a stale question was pending:\n%s", said)
	}
}

// Every outcome is announced on the wire, and refusal is not success.
//
// Mutation (M15/M16, and the refusal-exits-0 defect): report performed for anything that did not
// positively verify. This fails.
func TestTheOutcomeIsAnnouncedOnTheWire(t *testing.T) {
	for _, c := range []struct {
		out  Outcome
		exit int
	}{
		{OutcomePerformed, 0},
		{OutcomeFailed, 1},
		{OutcomeUnavailable, 3},
		{OutcomeClarify, 4},
		{OutcomeRefused, 5},
		{OutcomeCancelled, 6},
	} {
		if got := c.out.Exit(); got != c.exit {
			t.Errorf("%q exits %d, want %d", c.out, got, c.exit)
		}
		said, err := captureStdout(t, func() error { announce(c.out); return nil })
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(said, resultPrefix+string(c.out)) {
			t.Errorf("%q was not announced: %q", c.out, said)
		}
	}
	// ONLY performed may exit 0. A front-end that can see nothing but the exit code still
	// cannot mistake a refusal, a cancellation or an outage for a success.
	for _, o := range []Outcome{OutcomeClarify, OutcomeRefused, OutcomeUnavailable,
		OutcomeCancelled, OutcomeFailed} {
		if o.Exit() == 0 {
			t.Errorf("%q exits 0 — it will be read as success", o)
		}
	}
}

// The door is crossed whichever entrance the invocation came from, and however it was identified.
//
// Mutation (M11): let an exact play match bypass authority. This fails.
func TestExactPlayMatchDoesNotBypassAuthority(t *testing.T) {
	src := readRepoFile(t, "cmd/marco/intake.go")
	i := strings.Index(src, "func performOnePlay(")
	if i < 0 {
		t.Fatal("performOnePlay is gone; every play performance used to pass through it")
	}
	body := src[i:]
	if end := strings.Index(body, "\n// "); end > 0 {
		body = body[:end]
	}
	if !strings.Contains(body, "orchestrator.Authorize(") {
		t.Fatal("a play is performed without crossing the authority seam")
	}
	if !strings.Contains(body, "decision.Allow()") {
		t.Fatal("the door's verdict is not consulted")
	}
	// And an explicit identity takes exactly the same route into it — there is one performer.
	if strings.Count(src, "orchestrator.Authorize(") != 1 {
		t.Errorf("there are %d authority calls in the intake; there should be one door",
			strings.Count(src, "orchestrator.Authorize("))
	}
}

// Nothing but the intake decides what an invocation means.
//
// Mutation: route a surface's words anywhere else. This fails.
func TestEveryEntranceRoutesThroughTheOneIntake(t *testing.T) {
	// stop.go is here for the same reason the other two are: `marco stop` is an ENTRANCE, and a
	// stop verb that reached the stop machinery directly would be the fourth intake this phase
	// exists to remove.
	for _, rel := range []string{"cmd/marco/assistant.go", "cmd/marco/bind.go",
		"cmd/marco/stop.go"} {
		src := readRepoFile(t, rel)
		if !strings.Contains(src, "runInvocation(") {
			t.Errorf("%s no longer enters the one intake", rel)
		}
	}
	// The two semantic layers that used to sit in front of the door are gone from the
	// product path. `internal/nlu` and `internal/resolver` survive as a developer surface;
	// what may not come back is either of them deciding, unasked, which play a phrase meant.
	src := readRepoFile(t, "cmd/marco/assistant.go")
	if strings.Contains(src, "func resolveTarget(") {
		t.Error("resolveTarget is back: a fuzzy matcher and an external model in front of the door")
	}
	if strings.Contains(readRepoFile(t, "cmd/marco/panicstop.go"), "func dispatchDo(") {
		t.Error("dispatchDo is back — there are two intakes again")
	}
}

// Browsing and routing do not write anything.
func TestRoutingAnInvocationWritesNothing(t *testing.T) {
	d := intakeWorld(t)
	dir := d.Reg.Dir
	before := treeOf(t, dir)
	for _, text := range []string{"open mouse settings", "turn bluetooth off", "stop", "open bluetooth"} {
		for _, src := range []invoke.Source{invoke.SourceTyped, invoke.SourceSpoken} {
			_ = decideAs(t, d, src, text)
		}
	}
	if after := treeOf(t, dir); after != before {
		t.Fatalf("routing changed the tree:\n%s\n---\n%s", before, after)
	}
}

func treeOf(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		b.WriteString(filepath.ToSlash(rel))
		if !info.IsDir() {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			b.WriteString(":" + routes.DigestOf(string(data)))
		}
		b.WriteString("\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// hotkeyWorld is a registry, a binding, and a record of every outcome a press produced.
func hotkeyWorld(t *testing.T) (orchestrator.Deps, *[]Outcome) {
	t.Helper()
	d := intakeWorld(t)
	noDirector(t)
	var got []Outcome
	prev := exitProcess
	exitProcess = func(int) {}
	t.Cleanup(func() { exitProcess = prev })
	_ = prev
	return d, &got
}

// A hotkey performs the play it is bound to, and keeps performing it when the words would mean
// something else.
//
// # Why the words-vs-identity difference only shows in these two cases
//
// `pressHotkey` resolves the binding's text itself before handing anything on, so passing the text
// instead of the identity USUALLY lands on the same play — which is why a naive test cannot tell
// the two apart, and why the first version of this test could not. The difference is real in
// exactly two places, and both are product behaviour a person would notice:
//
//   - a play whose name IS a control word. Pressing its key must perform it, not cancel whatever
//     is running. With text, "stop" is arm 1 and the play never runs.
//   - a press made while Director is holding a question. The key means that play; it is not an
//     answer somebody typed. With text, the phrase is claimed by arm 2 and submitted as an answer.
//
// Mutation: pass `Text: step` instead of `Play: &rt` in pressHotkey. This fails on both.
func TestAHotkeyPerformsTheBoundPlayItself(t *testing.T) {
	t.Run("even when the play is named like a control word", func(t *testing.T) {
		d, _ := hotkeyWorld(t)
		if err := d.Reg.Save(routes.Route{Slug: "stop"}, intakeSrc); err != nil {
			t.Fatal(err)
		}
		if err := d.Reg.Bind("discord", "s", "stop"); err != nil {
			t.Fatal(err)
		}
		var stopped bool
		prev := stopWhatIsRunning
		stopWhatIsRunning = func(bool) int { stopped = true; return exitOK }
		t.Cleanup(func() { stopWhatIsRunning = prev })

		said, err := captureStdout(t, func() error {
			pressHotkey(d, "discord", "s")
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if stopped {
			t.Fatal("pressing the key bound to a play called \"stop\" cancelled whatever was " +
				"running instead of performing the play — the binding's identity was thrown " +
				"away and its words were read")
		}
		if !strings.Contains(said, "[route] stop") {
			t.Errorf("the bound play was not announced: %q", said)
		}
	})

	t.Run("even while Director is waiting for an answer", func(t *testing.T) {
		d, _ := hotkeyWorld(t)
		prev := pendingQuestion
		pendingQuestion = func() bool { return true }
		t.Cleanup(func() { pendingQuestion = prev })
		var submitted string
		prevSubmit := submitPhrase
		submitPhrase = func(p string, _ bool) int { submitted = p; return exitOK }
		t.Cleanup(func() { submitPhrase = prevSubmit })

		if err := d.Reg.Bind("discord", "m", "open mouse settings"); err != nil {
			t.Fatal(err)
		}
		said, err := captureStdout(t, func() error {
			pressHotkey(d, "discord", "m")
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if submitted != "" {
			t.Fatalf("a hotkey press was submitted to Director as an answer (%q) — a key is "+
				"not somebody answering a question", submitted)
		}
		if !strings.Contains(said, "[route] open mouse settings") {
			t.Errorf("the bound play was not performed: %q", said)
		}
	})
}

// A hotkey resolves in the scope it was bound in.
//
// Mutation: bind or resolve with the wrong app. This fails.
// # Why two fixtures rather than one
//
// The two ways to get the scope wrong are only visible under opposite conditions, because
// `HotkeyCmd` deliberately falls back from the application's bindings to the global ones. A
// binding stored globally still fires over the application it was made in, so only a press over a
// DIFFERENT application can see it; and a press that resolves globally still finds an app-less
// play, so only an app-SCOPED play can see that. One fixture catches one of them and lets the
// other through, which is exactly what happened the first time this was written.
func TestAHotkeyResolvesInTheScopeItWasBoundIn(t *testing.T) {
	t.Run("the binding lands in the application it was made in", func(t *testing.T) {
		d, _ := hotkeyWorld(t)
		// An APP-LESS play, so the press can find it whatever is in front. What is being
		// measured is where the BINDING went, not where the play is.
		if err := d.Reg.Save(routes.Route{Slug: "restart"}, intakeSrc); err != nil {
			t.Fatal(err)
		}
		if _, err := captureStdout(t, func() error { return bindKey(d, "discord", "r", "restart") }); err != nil {
			t.Fatalf("binding: %v", err)
		}
		said, err := captureStdout(t, func() error { pressHotkey(d, "notepad", "r"); return nil })
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(said, "[route]") {
			t.Errorf("a key bound while Discord was in front fired over Notepad — the "+
				"binding was stored somewhere wider than the scope it was made in:\n%s", said)
		}
		// And it does still fire where it was made.
		said, err = captureStdout(t, func() error { pressHotkey(d, "discord", "r"); return nil })
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(said, "[route] restart") {
			t.Errorf("the binding does not fire where it was made:\n%s", said)
		}
	})

	t.Run("the press resolves in the application it was pressed over", func(t *testing.T) {
		d, _ := hotkeyWorld(t)
		// Registered ONLY under Discord, so a press that resolves anywhere else finds
		// nothing at all.
		if err := d.Reg.Save(routes.Route{App: "discord", Slug: "restart"}, intakeSrc); err != nil {
			t.Fatal(err)
		}
		if _, err := captureStdout(t, func() error { return bindKey(d, "discord", "r", "restart") }); err != nil {
			t.Fatalf("binding: %v", err)
		}
		said, err := captureStdout(t, func() error { pressHotkey(d, "discord", "r"); return nil })
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(said, "[route] restart") {
			t.Fatalf("a key bound under Discord did not fire with Discord in front — the "+
				"scope it was stored in and the scope the press looks in are not the same "+
				"one:\n%s", said)
		}
	})
}

// A binding is validated the way it will be resolved.
//
// A fuzzy check at bind time and an exact one at press time meant a typo could be accepted,
// reported as bound, and then do nothing forever.
//
// Mutation: validate with nlu at 0.6 again. This fails.
func TestABindingIsValidatedTheWayItWillBeResolved(t *testing.T) {
	d := intakeWorld(t)
	// A near-miss that a 0.6 fuzzy matcher accepts and an exact resolve does not.
	const typo = "open mouse settingz"
	if _, ok := d.Reg.Resolve("discord", typo); ok {
		t.Fatalf("%q resolves exactly; pick a better near miss for this test", typo)
	}
	src := readRepoFile(t, "cmd/marco/bind.go")
	i := strings.Index(src, "func runBind(")
	if i < 0 {
		t.Fatal("runBind is gone")
	}
	body := src[i:]
	if end := strings.Index(body, "\n// runUnbind"); end > 0 {
		body = body[:end]
	}
	if strings.Contains(body, "nlu.Resolve(") {
		t.Error("`marco bind` validates with a fuzzy matcher while `marco hotkey` resolves " +
			"exactly, so a binding can be accepted and then never fire")
	}
	if !strings.Contains(body, "d.Reg.Resolve(app, base)") {
		t.Error("`marco bind` no longer validates with the call the press will make")
	}
}

// A play the door declined is not reported as performed.
//
// # The failure this guards
//
// "You said no" is not an error, so the old code returned nil — and a front end that reads an exit
// code cannot tell nil-because-nothing-was-wrong from nil-because-it-worked. The overlay rendered
// `ok` for a play Marco had just refused to run. This phase's whole outcome vocabulary exists for
// this, and nothing tested it.
//
// Mutation: return OutcomePerformed from the `!decision.Allow()` arm. This fails.
func TestARefusedPlayIsNotReportedAsPerformed(t *testing.T) {
	d := intakeWorld(t)
	noDirector(t)
	d.Authority = declineGate{} // the person said no

	// A LEARNED play, because that is the only kind the door stops.
	rt := routes.Route{App: "settings", Focus: true, Slug: "learned-one"}
	err := d.Reg.SaveWithOrigin(rt, intakeSrc,
		routes.Origin{Kind: routes.KindLearned, Application: "settings", To: "subj_x"})
	if err != nil {
		t.Fatal(err)
	}
	// If the fork were reached, this would fail the test rather than dial anything.
	prev := dialPerformer
	dialPerformer = func() (directorPerformer, error) {
		t.Fatal("a declined play was still handed to the Director")
		return nil, nil
	}
	t.Cleanup(func() { dialPerformer = prev })

	out, err := runInvocation(d, invoke.Request{Text: "learned one", Source: invoke.SourceTyped})
	if err != nil {
		t.Fatalf("a refusal is not an error: %v", err)
	}
	if out != OutcomeRefused {
		t.Fatalf("a play the door declined reported %q", out)
	}
	if out.Exit() == 0 {
		t.Fatal("a refusal exits 0, so every front end reading an exit code calls it a success")
	}
}

// A play somebody stopped is not reported as performed.
//
// A cancelled script leaves no error behind — `runFromEntryCtx` returns whatever the program left,
// and a cancelled one left nothing — so a stopped play used to exit 0 and read as a success.
//
// Mutation: return OutcomePerformed from the `case cancelled:` arm. This fails.
func TestACancelledPlayIsNotReportedAsPerformed(t *testing.T) {
	d := intakeWorld(t)
	noDirector(t)

	// A play that waits long enough to be stopped in the middle.
	rt := routes.Route{Slug: "slow-one"}
	const slow = "use os.\n\nthe Waiter is an actor.\n\nthis can Run.\nthis's Run does...\n" +
		"  do OS's Sleep with 4000.\n  this is ok!\n\nthe App is a script.\n\ndo Waiter's Run...\n" +
		"  when ok?\n    log \"done\".\n  or?\n    log that's error.\n"
	if err := d.Reg.Save(rt, slow); err != nil {
		t.Fatal(err)
	}
	// Stop it from underneath, the way the stop key does: cancel the context the run is given.
	prevStop := stopper
	stopper = func(realInput bool, fn func(context.Context) error) error {
		ctx, cancel := context.WithCancel(context.Background())
		go func() { time.Sleep(60 * time.Millisecond); cancel() }()
		defer cancel()
		return fn(ctx)
	}
	t.Cleanup(func() { stopper = prevStop })

	out, err := runInvocation(d, invoke.Request{Text: "slow one", Source: invoke.SourceSpoken})
	if err != nil {
		t.Logf("the cancelled run also reported: %v", err)
	}
	if out != OutcomeCancelled {
		t.Fatalf("a play that was stopped mid-run reported %q", out)
	}
	if out.Exit() == 0 {
		t.Fatal("a cancellation exits 0 and reads as a success")
	}
}

// `marco do` announces its result on stdout, for every outcome.
//
// # Why this is a subprocess test
//
// Because the thing that must not break crosses a process AND a module boundary: plugins/overlay
// reads the `[result] ` line off the child's stdout to tell a refusal from a failure from a
// question. Deleting `announce(out)` from runAssistantDo leaves both suites green and silently
// collapses six outcomes back into three — the overlay falls back to the exit code, and the Learn
// offer, which needs the literal word `unavailable`, stops firing altogether.
//
// It is the twin of TestABridgeFailureStillAnnouncesTheResolvedRoute, which guards `[route] ` and
// is exactly why the `[route] ` mutation was caught and this one was not.
//
// Mutation: remove announce(out) from runAssistantDo. This fails.
func TestMarcoDoAnnouncesItsResultOnStdout(t *testing.T) {
	exe := buildMarco(t)
	dir := t.TempDir()
	home := t.TempDir()
	reg := routes.Registry{Dir: dir}
	if err := reg.Save(routes.Route{Slug: "quiet-one"}, runnableSrc); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		what string
		args []string
		want Outcome
	}{
		{"a play that ran", []string{"do", "--source=cli", "quiet one"}, OutcomePerformed},
		{"a request nobody could take", []string{"do", "--source=cli", "turn bluetooth off"}, OutcomeUnavailable},
		{"a control phrase", []string{"do", "--source=cli", "stop"}, OutcomeCancelled},
	} {
		t.Run(c.what, func(t *testing.T) {
			cmd := exec.Command(exe, c.args...)
			cmd.Env = append(os.Environ(),
				"MARCO_ROUTES="+dir, "MARCO_HOME="+home,
				"DIRECTOR_BIN="+filepath.Join(home, "no-director-here.exe"))
			out, _ := cmd.CombinedOutput()
			if !strings.Contains(string(out), resultPrefix+string(c.want)) {
				t.Fatalf("`marco do` did not announce %q%s.\nA front end reading stdout cannot "+
					"tell what happened.\n%s", resultPrefix, c.want, out)
			}
		})
	}
}

// buildMarco builds the real binary once per test that needs one.
func buildMarco(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "marco-test.exe")
	build := exec.Command("go", "build", "-o", exe, "github.com/chaynes-simpleclouds/marco/cmd/marco")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building marco: %v\n%s", err, out)
	}
	return exe
}

// A request Director tried and could not do is not offered as something to learn.
//
// "I could not work out how to do that" and "I have never heard of that" are different answers,
// and only the second one makes an offer to record a demonstration sensible. Answering a failure
// with "shall I learn this?" invites somebody to demonstrate the thing they just watched go wrong.
//
// Mutation: widen the learn-error arm to OutcomeFailed as well. This fails.
func TestDirectorFailureIsNotOfferedAsSomethingToLearn(t *testing.T) {
	d := intakeWorld(t)
	noPendingQuestion(t)
	prev := submitPhrase
	submitPhrase = func(string, bool) int { return exitNotVerified } // Director ran it and failed
	t.Cleanup(func() { submitPhrase = prev })

	out, err := runInvocation(d, invoke.Request{Text: "do something impossible", Source: invoke.SourceSpoken})
	if out != OutcomeFailed {
		t.Fatalf("a Director failure reported %q", out)
	}
	if err != nil && strings.HasPrefix(err.Error(), "no play matches ") {
		t.Fatal("a request Director tried and failed was reported as an unknown command, so " +
			"the overlay offers to record a demonstration of it")
	}
	// And the genuinely-undeliverable case still does say so.
	submitPhrase = func(string, bool) int { return exitUnavailable }
	out, err = runInvocation(d, invoke.Request{Text: "do something impossible", Source: invoke.SourceSpoken})
	if out != OutcomeUnavailable {
		t.Fatalf("an undeliverable request reported %q", out)
	}
	if err == nil || !strings.HasPrefix(err.Error(), "no play matches ") {
		t.Fatalf("nothing took the request and nothing said so: %v", err)
	}
}

// A play's own name beats the invocation grammar that would take it apart.
//
// # Why this test has to live here and not in internal/invoke
//
// `invoke.Decide` knows nothing about the grammar. The ORDER — whole phrase against the registry
// first, grammar only after Director has been ruled out — is a property of cmd/marco/intake.go,
// so a test in internal/invoke cannot see it move. Hoisting grammar() ahead of Decide leaves
// invoke's suite entirely green while "log in with google" starts running "log in" with the
// argument "google", and "wait then click" becomes two commands.
//
// Mutation: call grammar() before invoke.Decide in runInvocation. This fails.
func TestAPlayNameBeatsTheInvocationGrammar(t *testing.T) {
	d := intakeWorld(t)
	noDirector(t)
	// The trap: each long name's PREFIX is also a play, so a grammar that runs first finds
	// something and runs it.
	for _, slug := range []string{
		"log-in-with-google", "log-in",
		"wait-then-click", "wait", "click",
	} {
		if err := d.Reg.Save(routes.Route{Slug: slug}, runnableSrc); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ text, want string }{
		{"log in with google", "log-in-with-google"},
		{"wait then click", "wait-then-click"},
	} {
		got := invoke.Decide(d.Reg, invoke.Request{Text: c.text, App: "discord"})
		if got.Kind != invoke.KindPlay || got.Play.Slug != c.want {
			t.Errorf("%q decided %q/%q, want the play %q", c.text, got.Kind, got.Play.Slug, c.want)
		}
		// And the intake really performs THAT one, not the prefix.
		said, err := captureStdout(t, func() error {
			_, e := runInvocation(d, invoke.Request{Text: c.text, Source: invoke.SourceTyped, App: "discord"})
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(said, "[route] "+plays.Pretty(c.want)) {
			t.Errorf("%q ran something else — the grammar took the name apart before anybody "+
				"checked whether it was a play:\n%s", c.text, said)
		}
	}
}

// A chain is all or nothing.
//
// One unknown step and the WHOLE phrase belongs to Director. Running the known prefix and then
// asking about the rest is half an answer to a question nobody asked in halves — and it means
// "open notepad then do the thing I described" performs the first half before anybody has worked
// out whether the second half is possible.
//
// Mutation: in grammar(), run the steps that resolve and send only the rest on. This fails.
func TestAChainIsAllOrNothing(t *testing.T) {
	d := intakeWorld(t)
	noDirector(t)
	for _, slug := range []string{"first-thing", "second-thing"} {
		if err := d.Reg.Save(routes.Route{Slug: slug}, runnableSrc); err != nil {
			t.Fatal(err)
		}
	}
	var submitted string
	prev := submitPhrase
	submitPhrase = func(p string, _ bool) int { submitted = p; return exitOK }
	t.Cleanup(func() { submitPhrase = prev })

	const mixed = "first thing then something nobody taught"
	said, err := captureStdout(t, func() error {
		_, e := runInvocation(d, invoke.Request{Text: mixed, Source: invoke.SourceSpoken})
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(said, "[route]") {
		t.Fatalf("half a chain was performed before anybody decided the other half was "+
			"possible:\n%s", said)
	}
	if submitted != mixed {
		t.Fatalf("Director was told %q; the person said %q", submitted, mixed)
	}
	// A chain whose steps ALL resolve is still a chain.
	said, err = captureStdout(t, func() error {
		_, e := runInvocation(d, invoke.Request{Text: "first thing then second thing", Source: invoke.SourceSpoken})
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(said, "[route] first thing") || !strings.Contains(said, "[route] second thing") {
		t.Errorf("a chain of known plays no longer runs both:\n%s", said)
	}
}

// No fuzzy matcher and no external model may sit in front of the door again.
//
// # Why the guard is behavioural and not a name
//
// The first version of this asserted that a function called `resolveTarget` did not exist. The
// same 0.75-score matcher inlined into runAssistantDo under no name passed it — and reproduced the
// measured defect exactly: with "open settings" and "open the settings" both registered, asking
// for the second ran the first, silently and unconfirmed. A rule about a spelling is not a rule
// about behaviour.
//
// Mutation: inline nlu.Resolve (or resolver.Resolve) anywhere on the intake path. This fails.
func TestNoGuessSitsInFrontOfTheDoor(t *testing.T) {
	d := intakeWorld(t)
	noDirector(t)
	for _, slug := range []string{"open-settings", "open-the-settings"} {
		if err := d.Reg.Save(routes.Route{Slug: slug}, runnableSrc); err != nil {
			t.Fatal(err)
		}
	}
	// The measured case: these two are IDENTICAL to the fuzzy matcher (it deletes stop words),
	// so a matcher in front of the door runs the first for either phrase.
	for _, c := range []struct{ text, want string }{
		{"open settings", "open-settings"},
		{"open the settings", "open-the-settings"},
	} {
		said, err := captureStdout(t, func() error {
			_, e := runInvocation(d, invoke.Request{Text: c.text, Source: invoke.SourceTyped})
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(said, "[route] "+plays.Pretty(c.want)) {
			t.Errorf("%q ran something other than %q — an exact durable identity lost to a "+
				"fuzzy neighbour:\n%s", c.text, c.want, said)
		}
	}
	// And a near miss goes to Director rather than being guessed at.
	var submitted string
	prev := submitPhrase
	submitPhrase = func(p string, _ bool) int { submitted = p; return exitOK }
	t.Cleanup(func() { submitPhrase = prev })
	said, err := captureStdout(t, func() error {
		_, e := runInvocation(d, invoke.Request{Text: "open the setting", Source: invoke.SourceTyped})
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(said, "[route]") {
		t.Errorf("a near miss was guessed into a play:\n%s", said)
	}
	if submitted != "open the setting" {
		t.Errorf("Director was told %q", submitted)
	}
}

// The shipped binary does not guess which play you meant.
//
// # Why this one runs the real executable
//
// Because the guess does not have to live in the intake to do the damage — it only has to live in
// front of it. A matcher inlined into `runAssistantDo`, above the call to `runInvocation`, is
// invisible to every test that enters `runInvocation` itself, and it reproduces the measured
// defect exactly: with "open settings" and "open the settings" both registered, asking for the
// second runs the first, silently and unconfirmed.
//
// So this asks the shipped `marco do` and reads what it announces. Nothing upstream of the door
// can hide from it.
//
// Mutation: inline nlu.Resolve or resolver.Resolve anywhere between runAssistantDo and the
// registry. This fails.
func TestTheShippedBinaryDoesNotGuessWhichPlayYouMeant(t *testing.T) {
	exe := buildMarco(t)
	dir := t.TempDir()
	home := t.TempDir()
	reg := routes.Registry{Dir: dir}
	// Identical to a stop-word-deleting fuzzy matcher, distinct to an exact one.
	for _, slug := range []string{"open-settings", "open-the-settings"} {
		if err := reg.Save(routes.Route{Slug: slug}, runnableSrc); err != nil {
			t.Fatal(err)
		}
	}
	run := func(text string) string {
		t.Helper()
		cmd := exec.Command(exe, "do", "--source=cli", text)
		cmd.Env = append(os.Environ(), "MARCO_ROUTES="+dir, "MARCO_HOME="+home,
			"DIRECTOR_BIN="+filepath.Join(home, "no-director-here.exe"))
		out, _ := cmd.CombinedOutput()
		return string(out)
	}
	for _, c := range []struct{ text, want string }{
		{"open settings", "open settings"},
		{"open the settings", "open the settings"},
	} {
		got := run(c.text)
		if !strings.Contains(got, "[route] "+c.want+"\n") {
			t.Errorf("`marco do %q` did not run %q — an exact durable identity lost to a "+
				"fuzzy neighbour somewhere in front of the door:\n%s", c.text, c.want, got)
		}
	}
	// And a near miss is not guessed into either of them.
	got := run("open the setting")
	if strings.Contains(got, "[route]") {
		t.Errorf("a near miss was guessed into a play instead of going to Director:\n%s", got)
	}
}

// The REPL's advisor proposes a play; it does not perform one.
//
// `marco assistant` is a developer surface, but it is advertised in `marco help` and it can be
// pointed at an arbitrary external program ($MARCO_ASSISTANT / plugins/llama). That program used
// to choose which durable behaviour to perform and have it performed, unasked — the same shape
// this phase deleted from `marco do`, except that here the guesser can be anything at all.
//
// Mutation: remove the confirmation from the IntentRun arm. This fails.
func TestTheAdvisorProposesAndDoesNotPerform(t *testing.T) {
	src := readRepoFile(t, "cmd/marco/dispatch.go")
	// From converseTurn's own body: there is another `case dispatch.IntentRun:` earlier in the
	// file, in the JSON printer, and slicing from the first one measures the wrong arm.
	fn := strings.Index(src, "func converseTurn(")
	if fn < 0 {
		t.Fatal("converseTurn is gone")
	}
	i := strings.Index(src[fn:], "case dispatch.IntentRun:")
	if i < 0 {
		t.Fatal("the advisor's run arm is gone")
	}
	arm := src[fn+i:]
	if end := strings.Index(arm, "case dispatch.IntentLearn:"); end > 0 {
		arm = arm[:end]
	}
	if !strings.Contains(arm, "askYes(") {
		t.Fatal("an external model names a play and it is performed with no question asked — " +
			"a guess in front of the door, wearing the door's authority")
	}
	if strings.Index(arm, "askYes(") > strings.Index(arm, "runDo(") {
		t.Fatal("the play is performed before the person is asked")
	}
}

// The acquisition verb is `learn`, and it still answers to the word it used to have.
//
// # Why the alias exists and why it is undocumented
//
// LEARN is the word: the person acts, Marco acquires. TEACH is reserved for the opposite
// direction — Marco guiding a person through something it already knows — and using it for
// acquisition read backwards and squatted on a word a real feature needs.
//
// But `teach` is what scripts, E2E.md and muscle memory have, so it keeps working. It is a
// compatibility alias, not a synonym: nothing documents it as canonical, and it retires with the
// other legacy verbs.
//
// Mutation: drop either arm of the case. This fails.
func TestTheLearnVerbAnswersToItsOldName(t *testing.T) {
	src := readRepoFile(t, "cmd/marco/main.go")
	if !strings.Contains(src, `case "learn", "teach":`) {
		t.Error("`marco learn` is not the verb, or `marco teach` stopped answering")
	}
	if strings.Contains(src, "runAssistantTeach") {
		t.Error("the acquisition entry point is still named for the reserved word")
	}
	dir := readRepoFile(t, "cmd/director/main.go")
	if !strings.Contains(dir, `case "learn", "teach":`) {
		t.Error("`director learn` is not the verb, or `director teach` stopped answering")
	}
}

// No live acquisition code names itself Teach.
//
// # The rule this enforces
//
//	LEARN   the person acts, Marco acquires
//	TEACH   Marco guides the person   <- RESERVED, unbuilt
//	DO      Marco acts
//
// The word Teach is kept clear so that when the feature that means it arrives, it does not have to
// be disentangled from the one that does not. This walks EVERY non-test Go file in the repository,
// plugins included, and refuses any identifier that spells it.
//
// It does NOT police prose: a comment may say "teaches the reader", and a dated ADR may describe
// what the word used to mean — that is history, and rewriting it would destroy the record this
// rule exists to protect.
//
// Mutation: reintroduce any Teach-spelled acquisition identifier. This fails.
func TestNoLiveAcquisitionCodeIsNamedTeach(t *testing.T) {
	// The things allowed to spell it, named individually so none can grow silently — and so that
	// an excuse which has stopped matching anything can be seen to have stopped matching.
	allowed := []string{
		// 1. THE COMPATIBILITY ALIASES. The verb and the command word still answer to the
		// word the product shipped with, undocumented, until the legacy verbs retire.
		`case "learn", "teach":`,
		`"learn" || fields[0] == "teach"`,
		`"learn" || fields[1] == "teach"`,
		`"teach": true`, // the overlay highlights the alias as a verb too

		// 2. A FROZEN WIRE VALUE. `dispatch.IntentLearn` marshals to the string "teach" for
		// out-of-tree resolver plugins that already parse it. The CONSTANT is Learn; only the
		// bytes on the wire keep the old spelling, and renaming them would break somebody
		// else's plugin over a word no person ever sees. Same reasoning as `[route] `.
		`IntentLearn   = "teach"`,
		`run/teach/chat/clarify`,

		// 3. DOCUMENTING THE ALIAS, once, where a person would go looking for it.
		`aliases <b>narrate teach</b>`,

		// 4. THE SAME FROZEN WIRE VALUE, seen from the plugin side. plugins/llama asks a
		// local model to answer with one of `run|teach|chat|clarify` and parses what comes
		// back, which is `dispatch.IntentLearn`'s marshalled bytes and nothing else. The
		// spelling is fixed by the protocol in category 2, not chosen here; changing it here
		// alone would simply stop the plugin understanding its own model.
		`"intent":"run|teach|chat|clarify"`,
		`- teach: they want to CREATE a new command`,
		`case "teach", "chat", "clarify":`,

		// 5. HANDOFF — genuinely wrong, in a module this test's author could not edit.
		//
		// plugins/web-ui embeds its browser front end as Go string literals, and the LEARN
		// session's card is still spelled with the reserved word: a CSS class `.card.teach`,
		// the JS function `teachingCard`, and the `classList.add("teach")` that joins them.
		// They are identifiers, not prose, and they are exactly what this rule forbids.
		//
		// They are excused HERE, named one at a time and labelled as debt, for one reason:
		// the alternative was to leave the widened walk red for everybody. Renaming them is
		// three edits in one file and no protocol depends on them — a CSS class and a JS
		// function are private to that page. **Delete these three entries with the rename.**
		// If they are still here in a year, the rule has taught somebody the wrong lesson.
		`.card.teach{`,
		`function teachingCard(`,
		`c.classList.add("teach")`,
		`teachingCard(a.learnSession)`,
	}
	// EVERY live acquisition file, found rather than listed. A hand-written list is a list
	// somebody forgets to extend — `internal/orchestrator` was live acquisition code named
	// Teach for the whole of the first sweep precisely because it was not on one.
	for _, rel := range acquisitionFiles(t) {
		src := readRepoFile(t, rel)
		for i, line := range strings.Split(src, "\n") {
			code := line
			// Prose is out of scope: a comment may legitimately use the word, and the ADR
			// corpus depends on being able to.
			if c := strings.Index(code, "//"); c >= 0 {
				code = code[:c]
			}
			if !strings.Contains(strings.ToLower(code), "teach") {
				continue
			}
			var excused bool
			for _, a := range allowed {
				if strings.Contains(code, a) {
					excused = true
				}
			}
			if !excused {
				t.Errorf("%s:%d names the reserved word in live acquisition code: %s",
					rel, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// acquisitionFiles is every non-test Go file in the repository.
//
// # It used to walk eight named directories, and the list was the bug
//
// The comment above this function already said the right thing — "found by walking, not by
// listing" — and the function underneath it did not do that. It read eight hand-written
// directories, top-level entries only, and `plugins/` was not among them. So roughly ten live
// acquisition identifiers in the overlay spelled the reserved word for the whole life of the
// rule, in the one place the AUDIENCE actually reads them.
//
// The proof that this was an oversight rather than a decision is in the allow-list itself: three
// of its seven entries only ever matched files this walk could not open. Somebody had already
// seen the overlay spelling it, written down that it was excused, and never noticed that the
// excuse was doing nothing because the file was never read.
//
// # So it walks, the way the pump test walks
//
// `internal/platform/navsource/pump_test.go` is the house pattern for exactly this shape of rule:
// a structural invariant that must hold everywhere, discovered rather than listed, because the
// version that names the files it knows about is written against precisely the files that were
// already correct.
//
// **plugins/ is walked although those are separate Go modules.** Reading a file needs no module,
// and the rule is about the vocabulary the product uses, which has no opinion about where the
// go.mod boundaries fall. The overlay is the surface a person types into; if anywhere had to be
// covered it was there.
//
// Walking everything rather than the acquisition packages is deliberate too. "Which packages
// count as acquisition" is the judgement that produced the eight-directory list, and it is the
// judgement that was wrong. There is no cost to reading 500 small files, and the rule reads
// better as what it actually is: nothing in Marco spells Teach in code.
func acquisitionFiles(t *testing.T) []string {
	t.Helper()
	const repoRoot = "../.."
	var out []string
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil // an unreadable corner of the tree is not this test's business
		case d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules"):
			return fs.SkipDir
		case d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go"):
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	// A floor, so a walk that silently stopped finding anything cannot pass by finding nothing.
	// Set well below the real count (~500) so ordinary growth and deletion never touch it, and
	// well above what the eight-directory version found, so a reversion to a listing fails here.
	if len(out) < 300 {
		t.Fatalf("the walk found only %d files; it is not looking where it thinks it is", len(out))
	}
	var sawPlugins bool
	for _, f := range out {
		if strings.HasPrefix(f, "plugins/") {
			sawPlugins = true
		}
	}
	if !sawPlugins {
		t.Fatal("the walk did not reach plugins/, which is the surface the Audience types " +
			"into and the one place the previous version of this rule could not see")
	}
	return out
}

// An ANSWER is not claimed by a play of the same name.
//
// # The opposite arm from the stale-question one
//
// TestAStalePendingQuestionDoesNotHijackAKnownPlay holds that a play Marco has still runs while
// some forgotten question is outstanding. This holds the direction that arm is in tension with:
// a phrase Director is genuinely waiting to hear must reach the question even when a registered
// play answers to those exact words. Otherwise naming a play "the first" makes one of Director's
// own questions permanently unanswerable, and the person is offered a choice they cannot take.
//
// Both directions matter because only one line decides between them, and it lives here rather
// than in `invoke`: the registry knows the play, `intent.ParseClarification` knows an answer, and
// neither knows whether anything is actually asking.
//
// Mutation: drop `req.Pending = pendingQuestion()` from runInvocation. This fails.
func TestAnAnswerIsNotClaimedByAPlayOfTheSameName(t *testing.T) {
	d := intakeWorld(t)
	noDirector(t)

	// A play registered under the exact words of an answer. It RUNS, so a decision that went
	// the other way would announce itself rather than silently doing nothing.
	if err := d.Reg.Save(routes.Route{Slug: "the-second-one"}, runnableSrc); err != nil {
		t.Fatal(err)
	}
	prev := pendingQuestion
	pendingQuestion = func() bool { return true }
	t.Cleanup(func() { pendingQuestion = prev })

	var submitted string
	prevSubmit := submitPhrase
	submitPhrase = func(p string, _ bool) int { submitted = p; return exitOK }
	t.Cleanup(func() { submitPhrase = prevSubmit })

	said, err := captureStdout(t, func() error {
		_, e := runInvocation(d, invoke.Request{
			Text: "the second one", Source: invoke.SourceSpoken,
		})
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if submitted != "the second one" {
		t.Fatalf("the answer never reached the question (submitted %q).\nDirector is holding "+
			"an unanswered question and the words that answer it were spent starting a play "+
			"of the same name, so the question can never be answered at all.", submitted)
	}
	if strings.Contains(said, "[route] ") {
		t.Errorf("a play was performed instead of the question being answered:\n%s", said)
	}
}
