package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// ============================================================================
// THE MATCHERS THAT WERE STILL IN FRONT OF THE DOOR.
//
// Phase 2 took `nlu.Resolve` at 0.75 out of `marco do`, because a near miss silently beating an
// exact identity is not a convenience — it is the product running the wrong thing. Two commands
// kept their own copy of the same rule, and both were reachable from the overlay's command line:
//
//	marco simplify   resolved fuzzily AND THEN REWROTE THE PLAY'S SOURCE
//	marco args       resolved fuzzily to tell the HUD what a play takes
//
// The fixture below is the pair the audit measured: "open settings" registered, "open the
// settings" asked for. `nlu.normalize` drops "the" as a stop word, so the matcher does not merely
// score these highly — it calls them EXACT, at 1.0. Nothing about a threshold would have saved it.
// `routes.Slug` keeps every word, so the exact lookup the intake makes says no.
// ============================================================================

// argSrc is a play with one argument label, so `marco args` has something to print.
//
// It is never executed by these tests; `runArgs` reads the file and scans it for placeholders, so
// what matters is the `{{who}}`, not whether the program would run.
const argSrc = `use os.

the App is a script.

log "hello {{who}}".
`

// twoPhrasingWorld registers ONE play, under the phrasing a person did not type.
func twoPhrasingWorld(t *testing.T) routes.Registry {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	t.Setenv("MARCO_HOME", t.TempDir())
	reg := routes.Registry{Dir: dir}
	if err := reg.Save(routes.Route{Slug: "open-settings"}, argSrc); err != nil {
		t.Fatal(err)
	}
	return reg
}

// The HUD's argument hints describe the play that would actually run.
//
// # The failure this replaces
//
// `marco args` is shelled by the overlay on every keystroke while somebody is typing, and its
// output becomes the coloured "with name:" guide in front of the command line. It resolved at
// 0.75, so it answered for a play the intake would then refuse: the person was shown an argument
// slot for "open settings" while typing "open the settings", filled it in, pressed Enter, and got
// "no play matches".
//
// A hint that describes a DIFFERENT play from the one that will run is worse than no hint at all,
// because it reads as confirmation that Marco understood the words. Printing nothing is a
// perfectly good answer here and always was — the overlay treats empty as "no hint".
//
// Mutation: put `nlu.Resolve(base, d.Reg.Slugs())` back in front of the lookup. This fails.
func TestArgumentHintsDescribeThePlayThatWouldRun(t *testing.T) {
	twoPhrasingWorld(t)

	// The control. A play that IS named answers, so the test cannot pass by printing nothing.
	hit, err := captureStdout(t, func() error {
		runArgs([]string{"open settings"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(hit) != "who" {
		t.Fatalf("the play that IS named offered no argument hint (%q); the near-miss half of "+
			"this test would then pass for the wrong reason", hit)
	}

	// The near miss. `nlu` calls this one EXACT — it drops "the" as a stop word — so the old
	// behaviour was not a marginal score, it was full confidence in the wrong play.
	miss, err := captureStdout(t, func() error {
		runArgs([]string{"open the settings"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(miss) != "" {
		t.Fatalf("the HUD was offered %q as the arguments of a play the intake would refuse to "+
			"run.\nThe hint and the door have to agree about which play the words name, or "+
			"the guide in front of somebody's typing is describing something else.", miss)
	}
}

// `marco simplify` rewrites the play it was asked for, or none.
//
// # Why this one mattered most
//
// Simplify does not merely PERFORM the play it picked — it regenerates the source from the stored
// demonstration and overwrites the file. A `marco do` that guesses wrong wastes a few seconds; a
// `marco simplify` that guesses wrong destroys somebody's work, and the only record of what had
// been there is the file it just replaced. The overlay exposes `simplify` as one of its own
// command words, so it was one typed phrase away from a person.
//
// The comment above it claimed the "same confident-match rule as `do`" — a rule Phase 2 had
// already deleted. That is how a guess survives an audit: it cites a sibling that no longer
// exists.
//
// # What is observed
//
// Which play the command REACHED. With only "open settings" registered and no recording beside it,
// the old code resolved "open the settings" onto it and got as far as the recording check, whose
// message names the play it had chosen. The new code never chooses one, so the words a person
// typed come back to them unchanged, with the near neighbour offered as a QUESTION.
//
// That distinction — suggest, never decide — is the line. The matcher is still allowed to say
// "did you mean"; it is not allowed to pick the file.
//
// Mutation: resolve `target` through nlu.Resolve again. The message names "open settings" and
// mentions a recording, and this fails.
func TestSimplifyRewritesOnlyThePlayItWasAskedFor(t *testing.T) {
	exe := buildMarco(t)
	dir := t.TempDir()
	home := t.TempDir()
	reg := routes.Registry{Dir: dir}
	if err := reg.Save(routes.Route{Slug: "open-settings"}, argSrc); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(exe, "simplify", "open the settings")
	cmd.Env = append(os.Environ(), "MARCO_ROUTES="+dir, "MARCO_HOME="+home,
		"DIRECTOR_BIN="+filepath.Join(home, "no-director-here.exe"))
	outBytes, err := cmd.CombinedOutput()
	said := string(outBytes)
	if err == nil {
		t.Fatalf("`marco simplify` succeeded at simplifying a play nobody named:\n%s", said)
	}

	if strings.Contains(said, "recording") {
		t.Fatalf("`marco simplify \"open the settings\"` reached a play's stored demonstration "+
			"— it had already CHOSEN one, and the only play here is a differently phrased "+
			"neighbour.\nThis command rewrites the file it picks.\n%s", said)
	}
	if !strings.Contains(said, "I don't know") {
		t.Errorf("the person was not told that the words they typed name nothing:\n%s", said)
	}
	if !strings.Contains(said, "open the settings") {
		t.Errorf("the message does not quote the words the person actually typed:\n%s", said)
	}
	// The near miss survives — as a question. A guess may propose; it may not choose.
	if !strings.Contains(said, "did you mean") {
		t.Errorf("no suggestion was offered, so a person one stop-word away from their own "+
			"play is told only that it does not exist:\n%s", said)
	}
	if !strings.Contains(said, "open settings") {
		t.Errorf("the suggestion does not name the neighbour it found:\n%s", said)
	}
}

// A near miss SUGGESTS and never decides.
//
// The unit both commands share, tested directly, because the distinction is the whole of 34F's
// recommendation and it lives in one function. `resolveExactly` returns three values and the third
// is deliberately not the second: a play, whether there IS one, and — only on a miss — a name to
// offer a person. Nothing may promote the third into the first.
//
// Mutation: return the suggestion as the resolved route. This fails.
func TestANearMissSuggestsAndNeverDecides(t *testing.T) {
	twoPhrasingWorld(t)
	d := newDeps()

	rt, ok, suggestion := resolveExactly(d, "open the settings")
	if ok {
		t.Fatalf("a near miss RESOLVED, to %q. Every caller of this treats `ok` as permission "+
			"to act on the play — one of them rewrites its source.", rt.Slug)
	}
	if rt.Slug != "" {
		t.Errorf("a miss handed back a play anyway (%q)", rt.Slug)
	}
	if suggestion != "open settings" {
		t.Errorf("the near neighbour was not offered as a suggestion (got %q); a person one "+
			"stop-word away from their own play should be told which one it might be",
			suggestion)
	}

	// And the exact phrasing still resolves, so exactness is not just refusal.
	if _, hit, _ := resolveExactly(d, "open settings"); !hit {
		t.Error("the play's own name no longer resolves")
	}
}
