package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// THE TWO LISTINGS ARE DIFFERENT ON PURPOSE, AND THIS IS WHERE THAT IS WRITTEN DOWN.
//
// `marco plays` is the product view: everything a person has, including the plays that are saved
// and not yet askable. `marco routes` is the compatibility view that front ends outside this
// module consume, and it may only ever name plays that can answer.
//
// Nothing else in the tree holds that line. The overlay is a separate Go module, so a `marco
// routes` widened to include the staging directory would compile, ship, and only be discovered
// when somebody asked for a name the overlay had just offered them.

const playCLISource = "script main...\n  do nothing.\n"

// twoPlays is one askable play and one waiting one, in a temporary routes directory.
//
// It writes them through the STORE, not through a fixture of its own: SaveWithOrigin is the call
// registration ends in and SaveStaged is the call Learn ends in, so a change to either shows up
// here rather than being papered over by hand-built files.
func twoPlays(t *testing.T) routes.Registry {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	reg := routes.Registry{Dir: dir}
	askable := routes.Route{App: "settings", Focus: routes.LearnedFocus, Slug: "open-mouse-settings"}
	if err := reg.SaveWithOrigin(askable, playCLISource,
		routes.Origin{Kind: routes.KindLearned, Application: "settings"}); err != nil {
		t.Fatalf("saving the registered play: %v", err)
	}
	waiting := routes.Route{App: "notepad", Focus: routes.LearnedFocus, Slug: "save-the-file"}
	if err := reg.SaveStaged(waiting, playCLISource,
		routes.Origin{Kind: routes.KindLearned, Application: "notepad"}); err != nil {
		t.Fatalf("staging the saved play: %v", err)
	}
	return reg
}

// say runs a CLI entry point and returns what it printed.
func say(t *testing.T, fn func()) string {
	t.Helper()
	out, _ := captureStdout(t, func() error { fn(); return nil })
	return out
}

// `marco plays` shows the saved play, in its own group, with the way to make it askable.
//
// Deleting the staged half of runPlays makes this fail: a play Learn wrote would be a file on
// disk that no command a person knows about mentions.
func TestMarcoPlaysShowsTheSavedPlayMarcoRoutesMustNotOffer(t *testing.T) {
	twoPlays(t)

	said := say(t, func() { runPlays(nil) })
	if !strings.Contains(said, "open mouse settings") {
		t.Fatalf("`marco plays` lost the registered play:\n%s", said)
	}
	if !strings.Contains(said, "save the file") {
		t.Fatalf("`marco plays` hides the saved play, so nothing a person can type mentions "+
			"the file Learn just wrote:\n%s", said)
	}
	// Two GROUPS, not one list with a badge. The difference is not a nuance: one of them
	// answers when you ask for it and the other does not.
	if !strings.Contains(said, "Known plays:") || !strings.Contains(said, "Saved, not askable yet:") {
		t.Errorf("the two groups are not headed separately:\n%s", said)
	}
	if strings.Index(said, "open mouse settings") > strings.Index(said, "save the file") {
		t.Errorf("the saved play is listed above the askable ones:\n%s", said)
	}
	// It says where each came from and where each stands, in the product's words.
	if !strings.Contains(said, "Learned") {
		t.Errorf("no row says where the play came from:\n%s", said)
	}
	if !strings.Contains(said, "Saved — not askable yet") {
		t.Errorf("the saved play is not told it cannot be asked for:\n%s", said)
	}
	// And it names a command that exists — see runRegister, which exists because of this.
	if !strings.Contains(said, `marco register "save the file"`) {
		t.Errorf("the saved play is shown with no way to make it askable:\n%s", said)
	}
}

// `marco routes` names only plays that can answer.
//
// The narrowness IS the feature. Its consumers are front ends — the overlay and the resolver
// plugins read it to decide what a phrase may resolve to — and a staged name offered here is Marco
// advertising a capability `marco do` cannot find.
//
// Widening runRoutes to plays.List makes this fail.
func TestMarcoRoutesOffersOnlyPlaysThatCanAnswer(t *testing.T) {
	reg := twoPlays(t)

	said := say(t, func() { runRoutes(nil) })
	if !strings.Contains(said, "open mouse settings") {
		t.Fatalf("`marco routes` lost the registered play:\n%s", said)
	}
	if strings.Contains(said, "save the file") {
		t.Fatalf("`marco routes` offers a saved play. A front end that prints this list would "+
			"suggest a name that cannot resolve:\n%s", said)
	}
	// The claim is not "this string is absent" but "this play cannot answer" — so prove the
	// second, from the same registry, rather than trusting the first.
	if _, ok := reg.Resolve("notepad", "save the file"); ok {
		t.Fatal("the staged play resolves, so its absence from `marco routes` proves nothing")
	}

	// The same narrowness on the machine-readable side.
	var rows []map[string]any
	if err := json.Unmarshal([]byte(say(t, func() { runRoutes([]string{"--json"}) })), &rows); err != nil {
		t.Fatalf("`marco routes --json` did not emit JSON: %v", err)
	}
	for _, row := range rows {
		if row["slug"] == "save-the-file" {
			t.Fatal("`marco routes --json` carries a saved play")
		}
		if row["registered"] != true {
			t.Errorf("a row in `marco routes --json` is not registered: %v", row)
		}
	}
}

// THE PUBLISHED JSON KEYS SURVIVE THE VOCABULARY CHANGE.
//
// name/slug/app/scope are parsed by consumers outside this module. Nothing else in this repository
// guards them, and renaming a struct tag is a one-character edit that compiles.
//
// Adding keys is fine and expected — a JSON decoder ignores what it does not know — which is why
// this pins the four by name rather than pinning the whole shape.
func TestMarcoRoutesJSONKeepsItsPublishedKeys(t *testing.T) {
	twoPlays(t)

	var rows []map[string]any
	if err := json.Unmarshal([]byte(say(t, func() { runRoutes([]string{"--json"}) })), &rows); err != nil {
		t.Fatalf("`marco routes --json` did not emit JSON: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected the one registered play, got %d rows: %v", len(rows), rows)
	}
	row := rows[0]
	for key, want := range map[string]any{
		"name":  "open mouse settings",
		"slug":  "open-mouse-settings",
		"app":   "settings",
		"scope": "focus",
	} {
		got, ok := row[key]
		if !ok {
			t.Errorf("the published key %q is gone. Two out-of-module consumers parse it.", key)
			continue
		}
		if got != want {
			t.Errorf("%q = %v, want %v", key, got, want)
		}
	}
	// The added keys are a superset, not a replacement.
	if row["kind"] != "Learned" || row["life"] != "ready" || row["registered"] != true {
		t.Errorf("the added keys do not describe the play: %v", row)
	}
	if row["activates"] != "settings" {
		t.Errorf("a focus play does not say what it brings forward: %v", row)
	}
}

// THE UNKNOWN-COMMAND ERROR IS ONE PREFIX, AND IT IS "no play matches ".
//
// plugins/overlay/acts.go matches it with strings.HasPrefix (const noPlayMatches) to decide that
// this particular failure gets an offer to teach instead of a log line. The overlay is a separate
// Go module, so a reworded error here breaks nothing at compile time and everything at run time:
// the overlay simply stops recognising an unknown command.
//
// The `marco teach` mention is part of the match and is pinned with it.
func TestTheUnknownCommandErrorIsOnePrefix(t *testing.T) {
	const want = "no play matches "

	// The producer, entered through the function `marco do` actually calls.
	twoPlays(t)
	t.Setenv("MARCO_NO_TEACH", "1") // the overlay's setting; without it this teaches instead
	d := newDeps()
	_, err := captureStdout(t, func() error {
		return dispatchDo(d, "nothing-called-this", nil, nil)
	})
	if err == nil {
		t.Fatal("an unknown command succeeded")
	}
	if !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("the unknown-command error is %q; the overlay matches on %q", err.Error(), want)
	}
	if !strings.Contains(err.Error(), "marco teach") {
		t.Errorf("the error no longer names the teach verb, which the overlay also matches on: %q",
			err.Error())
	}

	// The SECOND producer. `marco bind` exits the process on this path, so it is pinned by
	// reading its source — the alternative is no guard at all, and the two errors drifting
	// apart is exactly how one of them would stop being recognised.
	for _, rel := range []string{"cmd/marco/bind.go", "cmd/marco/panicstop.go"} {
		src := readRepoFile(t, rel)
		if !strings.Contains(src, `"`+want) {
			t.Errorf("%s no longer emits the %q prefix", rel, want)
		}
	}
	for _, rel := range []string{
		"cmd/marco/bind.go", "cmd/marco/panicstop.go",
		"cmd/marco/main.go", "cmd/marco/assistant.go",
	} {
		if strings.Contains(readRepoFile(t, rel), "no route matches ") {
			t.Errorf("%s still emits the old prefix; the overlay matches %q and would ignore it",
				rel, want)
		}
	}
}

// A STAGED PLAY THAT CANNOT BE REGISTERED IS NOT OFFERED REGISTRATION.
//
// A play edited since its provenance was written is not merely unregistered — routes.Register
// refuses it, on purpose, because registering it would be registering it as something Director
// verified. Printing `marco register "…"` beside it would be an instruction that fails when
// followed, which is worse than saying nothing.
func TestAStuckPlayIsNotOfferedRegistration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	reg := routes.Registry{Dir: dir}
	rt := routes.Route{App: "notepad", Focus: routes.LearnedFocus, Slug: "save-the-file"}
	if err := reg.SaveStaged(rt, playCLISource,
		routes.Origin{Kind: routes.KindLearned, Application: "notepad"}); err != nil {
		t.Fatal(err)
	}
	// The person edited their play. Ordinary, invited, and it costs the play its provenance.
	if err := os.WriteFile(reg.StagedPath(rt), []byte(playCLISource+"// mine now\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	said := say(t, func() { runPlays(nil) })
	if !strings.Contains(said, "save the file") {
		t.Fatalf("the play vanished from the listing entirely:\n%s", said)
	}
	if strings.Contains(said, "marco register") {
		t.Errorf("registration is offered for a play routes.Register will refuse:\n%s", said)
	}
	if !strings.Contains(said, "cannot be registered") {
		t.Errorf("nothing says why the play is stuck:\n%s", said)
	}
}

// REGISTERING A SAVED PLAY DOES NOT DEPEND ON WHAT IS IN FRONT.
//
// The application comes from the staged row, not from the foreground window — and the foreground
// window when somebody types `marco register` is a terminal. Building the Route from winctx.Active
// would register the play under the wrong application, or under none, and the resolver would then
// answer for it in the wrong place.
func TestRegisteringASavedPlayDoesNotDependOnWhatIsInFront(t *testing.T) {
	reg := twoPlays(t)
	rt := routes.Route{App: "notepad", Focus: routes.LearnedFocus, Slug: "save-the-file"}

	said := say(t, func() { runRegister([]string{"save", "the", "file"}) })
	if !strings.Contains(said, "save the file") {
		t.Fatalf("registering said nothing about the play:\n%s", said)
	}
	got, ok := reg.Resolve("notepad", "save the file")
	if !ok {
		t.Fatal("the play was not askable after being registered")
	}
	if got.App != "notepad" || !got.Focus {
		t.Errorf("registered as %+v, want notepad at focus scope", got)
	}
	// Registered OR staged, never both — the store's rule, checked from the caller that
	// invokes it, because a leftover staged copy is how the next register of the same slug
	// starts behaving strangely.
	if _, still := reg.StagedSource(rt); still {
		t.Error("the staged copy survived registration, so one play is now in two places")
	}
	// And it is no longer waiting.
	if strings.Contains(say(t, func() { runPlays(nil) }), "Saved, not askable yet:") {
		t.Error("`marco plays` still lists the registered play as waiting")
	}
}

// FOCUS READS DIFFERENTLY FROM CONTEXT, AND SAYS WHAT IT DOES.
//
// "From anywhere" is also true of a global play. The capability worth having is that Marco brings
// the application forward first, and a row that shortened the scope to a word would drop it — so
// the listing names the activation.
//
// Collapsing reachOf's focus arm into either neighbour makes this fail.
func TestFocusReadsDifferentlyFromContext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	reg := routes.Registry{Dir: dir}
	if err := reg.Save(routes.Route{App: "settings", Focus: true, Slug: "from-anywhere"},
		playCLISource); err != nil {
		t.Fatal(err)
	}
	if err := reg.Save(routes.Route{App: "settings", Slug: "only-here"}, playCLISource); err != nil {
		t.Fatal(err)
	}
	if err := reg.Save(routes.Route{Slug: "everywhere"}, playCLISource); err != nil {
		t.Fatal(err)
	}

	rows := map[string]string{}
	for _, line := range strings.Split(say(t, func() { runPlays(nil) }), "\n") {
		for _, name := range []string{"from anywhere", "only here", "everywhere"} {
			if strings.Contains(line, name) {
				rows[name] = line
			}
		}
	}
	if len(rows) != 3 {
		t.Fatalf("expected three rows, got %v", rows)
	}
	if !strings.Contains(rows["from anywhere"], "brings settings forward") {
		t.Errorf("the focus play does not say it brings its application forward: %q",
			rows["from anywhere"])
	}
	if strings.Contains(rows["only here"], "brings") {
		t.Errorf("a context play claims to bring its application forward: %q", rows["only here"])
	}
	if rows["only here"] == rows["from anywhere"] {
		t.Error("focus and context read identically")
	}
	if strings.Contains(rows["everywhere"], "settings") {
		t.Errorf("a global play was given an application: %q", rows["everywhere"])
	}
}
