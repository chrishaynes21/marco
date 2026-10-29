package main

// SELECT A SAVED ROUTINE AND PRESS PLAY, through Studio's own path.
//
// Everything here enters through `newActs` and the methods the page is bound to. The one thing
// that cannot be exercised in a test is the press itself: `marco do` performs real input, so the
// run is reached in two halves instead — what the window HANDS the command (`doCommand`, which
// carries the route's identity and the library to read it from), and what the window MAKES of
// what the command said (`readRoutine` over a pipe this test writes). Both halves are where the
// defects would be, and neither needs anything on anybody's desktop.
//
// The end-to-end spawn is exercised too, once, by a routine that presses nothing — see
// TestPlayingARoutineRunsItAndReportsWhatItSaid, which skips when there is no marco to spawn.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aSavedRoutine records one and saves it, and hands back the surface.
//
// Through `recorded` and `SaveDraft`, which is the product's own path from Stop to disk: a
// fixture that wrote a `.marco` by hand would be testing this listing against a file nothing in
// Marco produces.
func aSavedRoutine(t *testing.T, scope string, lose func(*recorder.RecordedEvent)) *Acts {
	t.Helper()
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, lose)
	if why := a.SaveDraft(scope, "arkascended"); why != "" {
		t.Fatalf("saving the recording: %s", why)
	}
	return a
}

func routineCalled(rs []Routine, name string) (Routine, bool) {
	for _, r := range rs {
		if strings.EqualFold(r.Name, name) {
			return r, true
		}
	}
	return Routine{}, false
}

// routineWithID is one row of the listing by its whole identity, which is the only way to ask for
// ONE of two routines that share a name.
func routineWithID(rs []Routine, id string) (Routine, bool) {
	for _, r := range rs {
		if r.ID == id {
			return r, true
		}
	}
	return Routine{}, false
}

// playing presses Play on the routine the listing calls `name`, through its IDENTITY — which is
// what the page does, and the only thing `PlayRoutine` accepts.
//
// A name is not an identity and a slug is not either; see Routine.ID. The lookup is part of the
// helper rather than of each test because every caller here means a routine it has just saved, and
// a test that could not find its own fixture is a broken test rather than a refusal worth
// asserting.
func playing(t *testing.T, a *Acts, name string) string {
	t.Helper()
	withAMarcoBeside(t)
	r, found := routineCalled(a.Routines(), name)
	if !found {
		t.Fatalf("there is no routine called %q in the library: %+v", name,
			a.Routines())
	}
	return a.PlayRoutine(r.ID)
}

// withAMarcoBeside states where marco is, for a test about something else.
//
// # Why a test has to say this at all
//
// Because Studio REFUSES to spawn a child it cannot point at, rather than handing a bare name
// to `exec.Command` and getting whatever is on PATH — see theBundleIsIncomplete. A test binary
// runs from a temporary directory with nothing beside it, so without this every test that
// presses Play gets the incomplete-bundle refusal instead of the thing it is about.
//
// It names the repo root's `marco.exe` when there is one and says MARCO_BIN. There is no
// fallback to a bare name, which is the point: `os.Stat("marco.exe")` resolves against the
// WORKING directory and `exec.Command("marco.exe")` resolves against PATH, and a test that
// confused the two is how this was found.
//
// # And it refuses a STALE one, which cost most of an evening
//
// The root `marco.exe` is built by `build.cmd` and by hand, and nothing rebuilds it. A test that
// spawns it is testing whenever that last happened: a thirty-second hang in a played routine was
// reproduced six times against a binary two days old, while the fix for it sat in the tree
// unbuilt. Every window looked current, because every window WAS current — the child was not.
// That is the trap CLAUDE.md describes about Studio's siblings, arriving through a test.
//
// So it is NOT RUN rather than run against the wrong thing, and the message says the command.
// Rebuilding here would be a test writing a tracked binary, which is worse.
func withAMarcoBeside(t *testing.T) {
	t.Helper()
	if filepath.IsAbs(marcoBin()) {
		return
	}
	abs, err := filepath.Abs(filepath.Join("..", "..", "marco.exe"))
	if err != nil || !fileThere(abs) {
		t.Skipf("no marco.exe at the repo root to point at (%s): build it with "+
			"`go build -o marco.exe ./cmd/marco` and this runs", abs)
	}
	if why := olderThanTheEngine(t, abs); why != "" {
		t.Skipf("%s: rebuild it with `go build -o marco.exe ./cmd/marco` and this "+
			"runs against what is in the tree", why)
	}
	t.Setenv("MARCO_BIN", abs)
}

// olderThanTheEngine says why a marco.exe cannot be trusted to be this tree's, or "".
//
// By the newest source under `cmd/marco` and `internal`, which is what it is built from. A
// timestamp is a weak identity and it is the only one a binary on disk has from outside —
// `internal/buildid` can say what it is, but asking means running it, and a test that ran an
// unknown binary to find out whether to run it has already run it.
func olderThanTheEngine(t *testing.T, bin string) string {
	t.Helper()
	b, err := os.Stat(bin)
	if err != nil {
		return fmt.Sprintf("%s cannot be read (%v)", bin, err)
	}
	var newest time.Time
	var at string
	for _, dir := range []string{
		filepath.Join("..", "..", "cmd", "marco"),
		filepath.Join("..", "..", "internal"),
	} {
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
				return nil
			}
			if info, ierr := d.Info(); ierr == nil &&
				info.ModTime().After(newest) {
				newest, at = info.ModTime(), p
			}
			return nil
		})
	}
	if newest.IsZero() || !newest.After(b.ModTime()) {
		return ""
	}
	return fmt.Sprintf("%s was built %s and %s changed %s, so this would spawn a "+
		"marco that is not the one in this tree", filepath.Base(bin),
		b.ModTime().Format("2006-01-02 15:04"), at,
		newest.Format("2006-01-02 15:04"))
}

// ── THE LIBRARY, LISTED ───────────────────────────────────────────────────────

// WHAT WAS JUST RECORDED IS THERE TO CHOOSE, WITHOUT LEAVING THE WINDOW.
//
// This is the gap the whole file is about. Saving worked and the flow stopped there: the only way
// to run what had just been kept was to open a terminal and type a command, which proves a
// Studio-recording-to-CLI path and is not the product.
func TestASavedRoutineIsListedAndReadyToPlay(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	r, found := routineCalled(a.Routines(), "ark graphics")
	if !found {
		t.Fatalf("the routine just saved is not in the library: %+v", a.Routines())
	}
	if !r.Runnable {
		t.Errorf("it is listed as unable to run: %s", r.Why)
	}
	if r.Slug != "ark-graphics" {
		t.Errorf("its slug is %q", r.Slug)
	}
	if r.App != "arkascended" || r.Scope != string(orchestrator.ScopeAppAnywhere) {
		t.Errorf("it is listed as %q in %q", r.Scope, r.App)
	}
	// AND WHAT IT SAYS ABOUT ITS OWN PRESSES, which is the one thing worth reading
	// before pressing Play on it. See ADR-251.
	if r.Contract != string(orchestrator.Guarded) {
		t.Errorf("its contract is %q, want %q", r.Contract, orchestrator.Guarded)
	}
	if !strings.Contains(r.Checks, "checked against the window it was recorded in") {
		t.Errorf("the sentence about its presses does not say they are checked: %q",
			r.Checks)
	}
	// AND THE MARCO IT WAS SAVED AS, because a routine is legal Marco so that somebody
	// can read what they kept and recognise it.
	if !strings.Contains(r.Source, "is a script") {
		t.Errorf("the saved Marco is not shown: %q", r.Source)
	}
	if r.Path == "" || r.SavedAt == "" {
		t.Errorf("it does not say where it is (%q) or when it was kept (%q)",
			r.Path, r.SavedAt)
	}
}

// AN ACT IS NOT A ROUTINE, and is not listed here.
//
// Two lists, two kinds of thing, two ways of performing them — and nothing in either list that
// the other should be showing. `Saved` skips a play with no bindings for the same reason this
// skips one with them: fail-closed in both directions, so nothing appears twice and nothing
// appears nowhere.
func TestAnActIsNotListedAsARoutine(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	// AN ACT ON DISK, through the one function that writes one.
	rt := routes.Route{Slug: "open-the-audio-settings"}
	src := "the App is a script.\n\ndo OS's Sleep with 1.\n"
	binds := demonstrated.Bindings{Act: "open the audio settings",
		Application: "arkascended", Source: routes.DigestOf(src)}
	if err := demonstrated.Save(a.reg(), rt, src, binds, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, listed := routineCalled(a.Routines(), "open the audio settings"); listed {
		t.Error("an Act is listed as a routine, so it would be performed by the " +
			"wrong thing — a routine runs through `marco do` and an Act " +
			"through the supervised runner under an authorisation")
	}
	if _, still := routineCalled(a.Routines(), "ark graphics"); !still {
		t.Error("the recorded routine went missing from the library")
	}
}

// AND THE ACT SURFACE MARCO SEEDS INTO THE LIBRARY IS NOT A ROUTINE EITHER.
//
// `os.marco` is a module of declarations. It is a route by the registry's reckoning, it is legal
// Marco, and pressing Play on it would perform nothing — so a list that offered it would be
// offering a button that does nothing to a file nobody recorded.
// A MODULE IS NOT A ROUTINE: it declares things for other programs to use and has no entrance, so
// pressing Play on it would perform nothing.
//
// Not `os.marco`, which is the one name `routes.routeSlug` already excludes everywhere — a
// fixture built out of it asks nothing, and a mutation deleting this filter outright survived
// exactly that. The case that reaches here is any OTHER module sitting beside the routines that
// import it: `the driver is a script` is how a program says it can be run, and a file with no
// such line is one the registry lists, Marco can read, and nobody can perform.
func TestAModuleBesideTheRoutinesIsNotOfferedAsOne(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	beside := filepath.Join(a.routesDir, routes.GlobalDir, "helpers.marco")
	if err := os.MkdirAll(filepath.Dir(beside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(beside,
		[]byte("the Helpers is an act.\nthis exports Tidy.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// THE REGISTRY DOES LIST IT, which is what makes the filter load-bearing: a module
	// in a scope directory is a route by the registry's reckoning and a readable file.
	listed := false
	for _, rt := range a.reg().List() {
		if rt.Slug == "helpers" {
			listed = true
		}
	}
	if !listed {
		t.Fatal("the registry does not list a module beside the routines, so this " +
			"test is not asking the question it was written for")
	}
	for _, r := range a.Routines() {
		if r.Slug == "helpers" {
			t.Fatalf("a module is offered as a routine, and pressing Play on "+
				"it would perform nothing: %+v", r)
		}
	}
}

// ── AND WHAT IS NOT OFFERED FOR PLAY ──────────────────────────────────────────

// A ROUTINE WHOSE PRESSES CAN NEVER BE CHECKED IS LISTED AND IS NOT PLAYABLE.
//
// Saved before the rule that a guarded recording brings its application forward: it reads like a
// routine and refuses every press. Told now, in the words that say what to do about it, rather
// than discovered as four refusals in a row.
func TestARoutineThatCannotPressAnythingIsNotOfferedForPlay(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, nil)
	// SAVED THE WAY THE OLD RULE ALLOWED, through the backend: the state on disk this
	// exists for. `SaveDraft` refuses this scope now, which is why it is not used.
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	if _, err := d.Save(a.capture.draft, orchestrator.ScopeApp, "arkascended"); err != nil {
		t.Fatal(err)
	}
	r, found := routineCalled(a.Routines(), "ark graphics")
	if !found {
		t.Fatal("it is not listed at all, so there is nothing to say about it")
	}
	if r.Runnable {
		t.Error("it is offered for Play, and every press in it would refuse")
	}
	if !strings.Contains(r.Why, "no window for those presses to be checked against") {
		t.Errorf("the refusal does not say why: %q", r.Why)
	}
	if why := playing(t, a, "ark graphics"); why == "" {
		t.Fatal("pressing Play on it started a run")
	} else if !strings.Contains(why, "cannot be performed as it stands") {
		t.Errorf("the refusal reads: %s", why)
	}
	if a.busy {
		t.Error("the run slot was claimed by a refusal")
	}
}

// AND A ROUTINE MISSING A MEASUREMENT IS NOT OFFERED EITHER, NAMING WHAT IT LACKS.
//
// It saves and then refuses, which is the contract working rather than a broken file — and it is
// still not something to press Play on and watch. Which measurement is missing is read off the
// saved Marco, so the sentence is about this routine rather than about the state in general.
func TestARoutineMissingAMeasurementIsNotOfferedForPlay(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", func(p *recorder.RecordedEvent) {
		p.Dpi = 0 // nobody could read the scale that window was drawn at
	})
	r, found := routineCalled(a.Routines(), "ark graphics")
	if !found {
		t.Fatal("it is not listed at all")
	}
	if r.Contract != string(orchestrator.Incomplete) {
		t.Errorf("its contract is %q, want %q", r.Contract, orchestrator.Incomplete)
	}
	if r.Runnable {
		t.Error("it is offered for Play, and it will refuse rather than press")
	}
	if !strings.Contains(r.Why, "what scale it was drawn at") {
		t.Errorf("the refusal does not name the missing measurement: %q", r.Why)
	}
	if why := playing(t, a, "ark graphics"); why == "" {
		t.Fatal("pressing Play on it started a run")
	}
}

// AND EVERY MEASUREMENT A DRAG CAN LACK IS SAID IN WORDS.
//
// The card names what a recording is missing, and the saved Marco spells those as `FromRelX` and
// `Dpi` — terms in a file, not things anybody says. A drag's four offsets had no reading at all
// when guarded drags landed, so a person would have read the field name off their own card; the
// same oversight that had the contract reader calling a guarded drag legacy.
//
// The list is read off `ContractOf` rather than written here, because a second list of the terms
// is a second list to fall behind: a drag that asks to be checked and states nothing else lacks
// every one of them, and each one has to come back as a sentence.
func TestEveryMeasurementADragCanLackIsSaidInWords(t *testing.T) {
	bare := `    the drag1 is a Drag with FromX 1, FromY 2, ToX 3, ToY 4, ` +
		`Button "left", RelTo "window".`
	contract, lacks := orchestrator.ContractOf(bare)
	if contract != orchestrator.Incomplete || len(lacks) == 0 {
		t.Fatalf("a drag that asks to be checked and measures nothing reads as %q "+
			"missing %v", contract, lacks)
	}
	said := saidTerms(lacks)
	for i := range lacks {
		if said[i] == lacks[i] {
			t.Errorf("a routine missing %s shows a person the term out of the "+
				"file rather than what it means", lacks[i])
		}
	}
}

// AND SOMETHING NOBODY SAVED SAYS WHICH LIBRARY WAS LOOKED IN.
//
// Which is the whole of the other half of this work: "there is no routine called that" with no
// directory named is the sentence somebody reads while their routine sits in a folder beside a
// binary they launched from somewhere else.
func TestPlayingAnUnknownRoutineNamesTheLibrary(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	why := a.PlayRoutine("arkascended/focus/nothing-like-this")
	if why == "" {
		t.Fatal("a routine nobody saved was performed")
	}
	if !strings.Contains(why, a.routesDir) {
		t.Errorf("the refusal does not say where it looked: %s", why)
	}
}

// AND A NAME IS NOT AN IDENTITY, so handing one over performs nothing.
//
// Not pedantry about an argument: it is what closes the defect. `PlayRoutine` matched a name OR a
// slug and took the FIRST row that answered, so a term that identifies two routines picked one of
// them — and everything downstream was then correct about the wrong route.
func TestPlayingARoutineByNameAloneIsRefused(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	raised := 0
	a.raising = func(string) string { raised++; return "" }
	for _, loose := range []string{"ark graphics", "ark-graphics", "arkascended"} {
		if why := a.PlayRoutine(loose); why == "" {
			t.Errorf("%q performed a routine, so a term that names two of them "+
				"would perform one of the two", loose)
		}
	}
	if raised != 0 {
		t.Errorf("something was brought forward for a routine nobody identified "+
			"(%d time(s))", raised)
	}
	if a.busy {
		t.Error("the run slot was claimed by a refusal")
	}
}

// ── ONE RUN AT A TIME, FOR EITHER KIND ────────────────────────────────────────

// THE SLOT IS ONE SLOT.
//
// There is one screen and one person in front of it. An Act's run asks for presses at a consent
// desk reading a global keyboard hook; a routine's run presses things. Two of either, or one of
// each, is two things pressing on one desktop.
//
// The refusals used to be written inside `start`, which had one caller. A second kind of run
// arriving beside it would have had to repeat the three questions or ask none of them, and the
// second is not something anybody notices until two runs are in flight.
func TestARoutineWillNotRunWhileSomethingElseIs(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	a.raising = func(string) string { return "" }
	if why := a.reserve(kindAct); why != "" {
		t.Fatalf("the slot could not be claimed for an Act: %s", why)
	}
	why := playing(t, a, "ark graphics")
	if why == "" {
		t.Fatal("a routine started while a run was already in flight")
	}
	if !strings.Contains(why, "one at a time") {
		t.Errorf("the refusal reads: %s", why)
	}
}

// AND NOT WHILE SOMETHING IS BEING RECORDED, which is the third thing the one slot refuses: a
// recorder watching the screen while a routine presses on it would put Marco's own clicks into
// somebody's draft.
func TestARoutineWillNotRunWhileSomethingIsBeingRecorded(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, nil)
	if why := a.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatal(why)
	}
	a.raising = func(string) string { return "" }
	// A SECOND RECORDING IN FLIGHT, through the production path.
	a.recording = func() recorder.Recorder { return &heldRecorder{} }
	if why := a.StartRecording("something else", ""); why != "" {
		t.Fatalf("starting a recording: %s", why)
	}
	why := playing(t, a, "ark graphics")
	if why == "" {
		t.Fatal("a routine ran while the screen was being watched")
	}
	if !strings.Contains(why, "recording") {
		t.Errorf("the refusal reads: %s", why)
	}
}

// ── THE APPLICATION COMES FORWARD, AND ONLY THIS WINDOW CAN DO IT ─────────────

// WINDOWS REFUSES `SetForegroundWindow` TO A PROCESS THAT DOES NOT ALREADY OWN THE FOREGROUND.
//
// The child about to be started is a process nobody is looking at, so it cannot raise anything —
// measured on this machine from a harness trying it from outside, where the call returned false
// and the foreground did not move. Studio owns the foreground at the instant somebody clicks a
// button in it, and that is the one moment it may hand it on.
//
// So this is not a convenience. Without it a guarded routine's `Activate` fails, every press
// refuses for want of a window to be checked against, and the run reports a routine that did
// nothing — which looks exactly like a defect in the guard. Deleting the raise must fail this.
func TestTheApplicationIsBroughtForwardBeforeTheRunStarts(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	raised := []string{}
	a.raising = func(app string) string {
		raised = append(raised, app)
		// AND IT REFUSES, so nothing is spawned: what is under test is that the
		// raising happened and that it happened FIRST.
		return "that application is not running"
	}
	why := playing(t, a, "ark graphics")
	if len(raised) != 1 || raised[0] != "arkascended" {
		t.Errorf("the window brought forward %v before the run, want "+
			"[arkascended] — a child process cannot raise a window, so a "+
			"routine started without this presses into whatever was in front",
			raised)
	}
	if why != "that application is not running" {
		t.Errorf("a run started although its application could not be brought "+
			"forward: %q", why)
	}
	if a.busy {
		t.Error("the run slot was claimed although nothing was started")
	}
}

// AND A ROUTINE WITH NO APPLICATION RAISES NOTHING.
//
// `everywhere` switches nothing by definition. Bringing a window forward for it would be this
// window choosing a foreground the routine did not ask for.
func TestARoutineThatNamesNoApplicationRaisesNothing(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, func(p *recorder.RecordedEvent) {
		p.RelTo, p.WinRel, p.Dpi, p.Process = "", false, 0, ""
		p.WinW, p.WinH, p.RelX, p.RelY = 0, 0, 0, 0
	})
	if why := a.SaveDraft("everywhere", ""); why != "" {
		t.Fatalf("saving it to run anywhere: %s", why)
	}
	asked := 0
	a.raising = func(string) string { asked++; return "" }
	r, found := routineCalled(a.Routines(), "ark graphics")
	if !found {
		t.Fatalf("it is not listed: %+v", a.Routines())
	}
	if r.App != "" || r.Scope != string(orchestrator.ScopeEverywhere) {
		t.Fatalf("it is listed as %q in %q", r.Scope, r.App)
	}
	// THE REFUSAL IS THE SLOT, taken first so nothing is spawned by this test.
	if why := a.reserve(kindAct); why != "" {
		t.Fatal(why)
	}
	if why := playing(t, a, "ark graphics"); why == "" {
		t.Fatal("a run started while the slot was claimed")
	}
	if asked != 0 {
		t.Errorf("something was brought forward for a routine that switches "+
			"nothing (%d time(s))", asked)
	}
}

// ── WHAT THE WINDOW HANDS THE COMMAND ─────────────────────────────────────────

// THE ROUTE'S WHOLE IDENTITY, AND THE LIBRARY IT IS IN.
//
// Two things, both invisible from outside the child, and both the kind of thing that works
// perfectly in testing and fails on somebody's machine. A run told only the slug performs
// whichever scope the registry finds first; a run not told the library reads whichever directory
// it was started in — which is the defect this whole increment is about, arriving a second time
// one process further down.
func TestTheCommandCarriesTheRoutesIdentityAndTheLibrary(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	r, found := routineCalled(a.Routines(), "ark graphics")
	if !found {
		t.Fatal("the routine is not listed")
	}
	// AND THE AMBIENT ENVIRONMENT NOW SAYS SOMETHING ELSE.
	//
	// Which is the whole of what makes this a test. Asserting that the child's
	// environment CONTAINS the library is satisfied by `os.Environ()` on its own
	// whenever the two agree — and in a test they agree by construction, so the
	// assertion passed with the explicit setting deleted. It has to be asked where the
	// two DISAGREE: the window listed this routine out of the library it resolved when
	// it opened, and that is the one the run must read whatever has happened to the
	// environment since.
	somewhereElse := t.TempDir()
	t.Setenv("MARCO_ROUTES", somewhereElse)
	t.Setenv("MARCO_HOME", t.TempDir())

	_, args, env := a.doCommand(r)
	for _, want := range []string{"do", "--play=ark-graphics",
		"--app=arkascended", "--focus"} {
		if !hasArg(args, want) {
			t.Errorf("the command does not carry %q: %v", want, args)
		}
	}
	// THE LAST SETTING OF A NAME IS THE ONE THE CHILD GETS. `os/exec` dedups the
	// environment keeping the last value, which is what makes an explicit append an
	// override rather than a duplicate.
	if got := effective(env, "MARCO_ROUTES"); got != a.routesDir {
		t.Errorf("the command would read %s and the window listed this routine "+
			"from %s. A run not told which library to read reads whichever "+
			"directory it was started in, which is the defect this surface "+
			"was built on top of.", got, a.routesDir)
	}
	if got := effective(env, "MARCO_HOME"); got != a.home {
		t.Errorf("the command would read the store at %s, not %s", got, a.home)
	}
}

// effective is the value a child would see for name: the LAST setting of it, which is the one
// `os/exec` keeps when it dedups.
func effective(env []string, name string) string {
	got := ""
	for _, kv := range env {
		if k, v, cut := strings.Cut(kv, "="); cut && k == name {
			got = v
		}
	}
	return got
}

// AND A CONTEXT-SCOPED ROUTINE IS NOT TOLD TO BRING ANYTHING FORWARD.
//
// `--focus` selects the per-application folder that activates; passing it for a routine saved to
// run only while its application is in front would ask for a route that is not there, and the
// run would refuse about a name rather than perform the one that exists.
func TestAContextRoutineIsNotAskedForAsAFocusOne(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, func(p *recorder.RecordedEvent) {
		// AN ORDINARY PRESS, so the context scope is allowed at all: a guarded
		// one cannot be saved to switch nothing.
		p.RelTo, p.WinRel, p.Dpi, p.Process = "", false, 0, ""
		p.WinW, p.WinH, p.RelX, p.RelY = 0, 0, 0, 0
	})
	if why := a.SaveDraft("app", "arkascended"); why != "" {
		t.Fatalf("saving it to run in context: %s", why)
	}
	r, found := routineCalled(a.Routines(), "ark graphics")
	if !found {
		t.Fatalf("it is not listed: %+v", a.Routines())
	}
	if r.Scope != string(orchestrator.ScopeApp) {
		t.Fatalf("it is listed as %q", r.Scope)
	}
	_, args, _ := a.doCommand(r)
	if hasArg(args, "--focus") {
		t.Errorf("a context routine is asked for as a focus one: %v", args)
	}
	if !hasArg(args, "--app=arkascended") {
		t.Errorf("it is not asked for in its application: %v", args)
	}
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// ── WHAT THE WINDOW MAKES OF WHAT THE COMMAND SAID ────────────────────────────

// aRunThatSaid plays these lines at the window as a child's output would, and hands back what the
// page would then be shown.
//
// Through the production reader over a real pipe, because what is under test is the reading: the
// answer has to come off `internal/outcome`'s own wire line rather than off a sentence this
// window recognised. A surface that matched on wording is a surface whose verdict changes when
// somebody improves a message.
func aRunThatSaid(t *testing.T, lines ...string) Running {
	t.Helper()
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	if why := a.reserve(kindRoutine); why != "" {
		t.Fatal(why)
	}
	a.act, a.live = "ark graphics", true
	a.routine = &routineRun{name: "ark graphics", slug: "ark-graphics",
		app: "arkascended"}
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for _, l := range lines {
			fmt.Fprintln(pw, l)
		}
		pw.Close()
	}()
	a.readRoutine(pr)
	// THE CHILD HAS GONE AND ITS OUTPUT HAS DRAINED — the slot is given back, which is
	// the instant the window is allowed to say how the run went.
	a.mu.Lock()
	a.busy = false
	a.mu.Unlock()
	return a.Watch()
}

// THE ANSWER IS THE RUN'S OWN PUBLISHED WORD.
func TestTheAnswerComesFromTheRunsOwnWord(t *testing.T) {
	w := aRunThatSaid(t, "[route] ark graphics", outcome.Line(outcome.Performed))
	if w.Outcome != string(outcome.Performed) {
		t.Errorf("the run announced %q and the window shows %q",
			outcome.Performed, w.Outcome)
	}
	if !w.Complete || w.Incomplete {
		t.Errorf("a run that announced an answer is complete=%v incomplete=%v",
			w.Complete, w.Incomplete)
	}
	// AND IT DOES NOT CLAIM THE APPLICATION DID ANYTHING. A routine presses what it was
	// shown; nothing on this path reads the screen afterwards, and a sentence saying it
	// worked would be this window asserting the one thing it cannot see.
	if !strings.Contains(w.Said, "Nothing here checked") {
		t.Errorf("the sentence claims more than was checked: %q", w.Said)
	}
}

// A REFUSED PRESS IS NOT A SUCCESSFUL RUN, AND THE REASON IS KEPT.
//
// This is the whole of the guarded contract arriving at a window: the command exits unsuccessfully
// and says which press could not be checked, and both have to survive the trip. A window that
// showed `failed` without the sentence would be showing a tally.
func TestARefusedPressReachesTheWindowWithItsReason(t *testing.T) {
	said := "ark-graphics stopped: the window this was recorded in was 1920x1080 and " +
		"is now 1280x720"
	// THE ROUTINE'S OWN OUTPUT COMES FIRST, which is what makes this the LAST sentence
	// rather than merely a sentence: a run says things on its way through — the route it
	// resolved to, whatever the program logs — and the refusal is the thing it says on
	// its way out. Keeping the first of them reads back the routine's own log line as
	// the reason it stopped, and a fixture of two lines cannot tell the two apart.
	w := aRunThatSaid(t, "[route] ark graphics",
		"ark graphics: bringing arkascended forward", said,
		outcome.Line(outcome.Failed))
	if w.Outcome != string(outcome.Failed) {
		t.Errorf("the run announced %q and the window shows %q",
			outcome.Failed, w.Outcome)
	}
	if w.Reason != said {
		t.Errorf("the reason the window kept is %q, want %q", w.Reason, said)
	}
	if !strings.Contains(w.Said, "nothing after the press that did not happen") {
		t.Errorf("the sentence does not say what a stopped routine means: %q",
			w.Said)
	}
	// AND THE RUN'S OWN SENTENCE IS IN WHAT THE PERSON READS, not only in the
	// transcript behind Technical details. The word is what happened and the sentence is
	// why; a banner carrying only the word is a tally, which is the thing ADR-251
	// insists a run's answer is not.
	if !strings.Contains(w.Said, said) {
		t.Errorf("the banner does not carry what the run said about the press it "+
			"would not make: %q", w.Said)
	}
}

// AND A RUN THAT ANNOUNCED NOTHING IS NOT REPORTED AS EITHER.
//
// A child that was killed, or that is not the binary this expected, leaves a stream with no end
// to it. Reported apart from a failure, because a routine that refused SAYS it refused — and
// apart from a success for the obvious reason.
func TestARunThatAnnouncedNothingIsNotAnAnswer(t *testing.T) {
	w := aRunThatSaid(t, "[route] ark graphics", "something happened")
	if w.Outcome != "" {
		t.Errorf("a run that announced nothing is reported as %q", w.Outcome)
	}
	if w.Complete {
		t.Error("a run that announced nothing is reported as complete")
	}
	if !w.Incomplete {
		t.Error("a run nobody can speak for is not reported as one")
	}
	if !strings.Contains(w.Said, "nobody can speak for") {
		t.Errorf("the sentence reads: %q", w.Said)
	}
}

// AND A LINE THAT BEGINS LIKE THE WIRE LINE AND CONTINUES WITH SOMETHING ELSE IS NOT AN ANSWER.
//
// `outcome.FromLine` requires the whole line after the prefix to be one of the seven, because a
// line that is not is a disagreement between two builds — and salvaging half of it is how a
// refusal becomes a success.
func TestAWordThatIsNotOneOfTheSevenIsNotAnAnswer(t *testing.T) {
	w := aRunThatSaid(t, outcome.ResultPrefix+"mostly fine")
	if w.Outcome != "" {
		t.Errorf("an unrecognised word was adopted as the answer: %q", w.Outcome)
	}
	if !w.Incomplete {
		t.Error("a run whose answer could not be read is not reported as one " +
			"nobody can speak for")
	}
}

// A ROUTINE'S RUN IS NOT RENDERED AS A SUPERVISED PERFORMANCE.
//
// It is `marco do` performing a program, not the runner writing an account of a performance.
// It cites a reference and writes a record of its own now, and the window READS that citation —
// so what is checked here is that nothing INVENTS one when the runner printed none, and that no
// runner's state appears beside it. A view that expected the other thing would tell somebody
// their perfectly good routine "cited no reference, so there is no canonical account of it".
func TestARoutinesRunSaysItIsOne(t *testing.T) {
	w := aRunThatSaid(t, outcome.Line(outcome.Performed))
	if !w.Routine {
		t.Error("the run does not say it is a routine's, so the page would look " +
			"for a reference, a phase and a question that will never come")
	}
	if w.Reference != "" {
		t.Errorf("it invented a run reference: %q", w.Reference)
	}
	if w.State != nil {
		t.Error("it carries a runner's state, which nothing wrote")
	}
}

// AND THE JOURNAL SAYS WHAT HAPPENED, as a routine's run rather than as a performance.
func TestTheTrialJournalRecordsARoutinesRun(t *testing.T) {
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	a.trial.pressing("ark graphics", "arkascended", string(orchestrator.Guarded))
	a.trial.pressed("ark graphics", string(outcome.Failed),
		"ark-graphics stopped: the window has closed")
	a.trial.flush()
	held, err := trialsIn(a.home)
	if err != nil || len(held) == 0 {
		t.Fatalf("nothing was written to the journal: %v", err)
	}
	tr := held[len(held)-1]
	n, had := tr.last(trialPressed)
	if !had {
		t.Fatal("the journal has no note of the run")
	}
	if n.Name != "ark graphics" || n.Outcome != string(outcome.Failed) {
		t.Errorf("the note says %q ended as %q", n.Name, n.Outcome)
	}
	if !strings.Contains(n.Because, "the window has closed") {
		t.Errorf("the note does not keep what the run said: %q", n.Because)
	}
	// AND THE VERDICT READS IT AS A ROUTINE'S RUN. Filed as a performance it would say
	// "the run cited no reference", which no routine ever does.
	said := strings.Join(resultVerdict(tr), " ")
	if strings.Contains(said, "cited no reference") {
		t.Errorf("a routine's run is read back as a performance that cited "+
			"nothing: %s", said)
	}
	if !strings.Contains(said, string(outcome.Failed)) ||
		!strings.Contains(said, "the window has closed") {
		t.Errorf("the verdict does not say how it went: %s", said)
	}
	// AND THE CONTRACT IT RAN UNDER, which is the question such a trial is run to ask.
	p, started := tr.last(trialPressing)
	if !started || p.Contract != string(orchestrator.Guarded) {
		t.Errorf("the journal does not say its presses were checked: %+v", p)
	}
	// AND `-trial` PRINTS IT, which is where §63 of E2E.md sends somebody to read back
	// what a trial showed. A listing that skipped a routine's run would leave the owner
	// reading a diagnostic with no run in it — and the branch that prints one survived
	// being deleted entirely until this was here.
	printed := printedBy(t, func() { printRun(tr) })
	if !strings.Contains(printed, "ark graphics") ||
		!strings.Contains(printed, string(outcome.Failed)) {
		t.Errorf("the run is not in what -trial prints:\n%s", printed)
	}
	if strings.Contains(printed, "cited no reference") {
		t.Errorf("it is printed as a performance that cited nothing:\n%s", printed)
	}
}

// printedBy is what a function wrote to stdout.
//
// The layout of this diagnostic is deliberately not asserted anywhere — `printVerdict` says so in
// its own words, and each of its judgements is a function returning a sentence for that reason.
// What IS asserted here is narrower and not layout: that a routine's run reaches the listing at
// all.
func printedBy(t *testing.T, fn func()) string {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	was := os.Stdout
	os.Stdout = pw
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, rerr := pr.Read(buf)
			b.Write(buf[:n])
			if rerr != nil {
				break
			}
		}
		done <- b.String()
	}()
	fn()
	pw.Close()
	os.Stdout = was
	out := <-done
	pr.Close()
	return out
}

// ── AND THE LIBRARY SAYS WHERE IT IS ──────────────────────────────────────────

// WHICH LIBRARY, AND WHETHER ANYBODY'S IS SOMEWHERE ELSE.
//
// The default moved: every library saved under the old rule sits in a `routes` folder beside
// whichever directory a binary was started from. The honest failure of changing that is silence —
// the routines are there, nothing says so, and a person reads an empty list and concludes their
// work is gone.
func TestTheWindowSaysWhichLibraryItIsReading(t *testing.T) {
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	if got := a.Library().Dir; got != a.routesDir {
		t.Errorf("the window says it reads %s and reads %s", got, a.routesDir)
	}
}

func TestALibraryLeftBesideTheLaunchDirectoryIsNamed(t *testing.T) {
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	here := t.TempDir()
	old := filepath.Join(here, "routes", "global")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "ark-graphics.marco"),
		[]byte("the App is a script.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(here)
	l := a.Library()
	if l.Elsewhere == "" {
		t.Fatal("a library sitting beside this window with a routine in it is " +
			"not mentioned, so somebody reads an empty list and concludes " +
			"their work is gone")
	}
	if !strings.Contains(l.Says, "1 routine") {
		t.Errorf("it does not say how much is in there: %q", l.Says)
	}
	// AND NOTHING IS MOVED. Adopting a directory nobody named would be this window
	// deciding which of two libraries is theirs.
	if _, err := os.Stat(filepath.Join(old, "ark-graphics.marco")); err != nil {
		t.Errorf("the old library was disturbed: %v", err)
	}
}

// ── AND THE WHOLE THING, ONCE, WITH A ROUTINE THAT PRESSES NOTHING ────────────

// PRESS PLAY AND THE WINDOW REPORTS WHAT THE RUN SAID.
//
// The real spawn: `marco do` started by this window, reading the library this window listed the
// routine from, and announcing its answer back up the pipe. The routine logs and presses nothing,
// which is what makes this safe to run at all — `marco do` performs real input, and a test that
// clicked somewhere on whoever's desktop ran it is a test nobody may run.
//
// Skipped when there is no marco to spawn, because this module can be tested without the engine
// built beside it. That is a real gap and the two halves above are why it is not a hole: what is
// handed to the command and what is made of its answer are both covered unconditionally.
func TestPlayingARoutineRunsItAndReportsWhatItSaid(t *testing.T) {
	withAMarcoBeside(t)
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	a := newActs(t.TempDir())
	// A ROUTINE THAT PRESSES NOTHING, written as a routine is: a program with an
	// entrance. Legal Marco, in the library, reachable by name — and it sends no input,
	// which is the only reason this may be run.
	where := filepath.Join(dir, "global")
	if err := os.MkdirAll(where, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "// marco learn \"says hello\"\n\nthe App is a script.\n\n" +
		"log \"says hello: nothing was pressed\".\n"
	if err := os.WriteFile(filepath.Join(where, "says-hello.marco"),
		[]byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	r, found := routineCalled(a.Routines(), "says hello")
	if !found {
		t.Fatalf("the routine is not in the library: %+v", a.Routines())
	}
	if !r.Runnable {
		t.Fatalf("it is listed as unable to run: %s", r.Why)
	}
	if why := playing(t, a, "says hello"); why != "" {
		t.Fatalf("pressing Play: %s", why)
	}
	if !a.awaitRun(30 * time.Second) {
		// WHERE IT STOPPED, NOT JUST THAT IT DID.
		//
		// This failed under suite load and passed in isolation, and "the run did not
		// finish" is the one sentence that cannot say which half is stuck: a child
		// still running its program, a child that exited with its output still in the
		// pipe, or a child that ran everything and did not exit.
		//
		// So the failure reads the RUN RECORD the child claimed — which is the
		// artifact that exists for exactly this, and whose `ended` field is absent
		// while the attempt has not finished. A record with no `ended` says the child
		// never reached its own end; one with an outcome says the child finished and
		// this window did not notice.
		w := a.Watch()
		t.Fatalf("the run did not finish in 30s.\n"+
			"  active:     %v\n  outcome:    %q\n  reference:  %q\n"+
			"  record:     %s\n  transcript: %d line(s)\n%s",
			w.Active, w.Outcome, w.Reference, theRecordSays(a, w.Reference),
			len(w.Transcript), strings.Join(w.Transcript, "\n"))
	}
	w := a.Watch()
	if w.Outcome != string(outcome.Performed) {
		t.Errorf("the run ended as %q, want %q — transcript:\n%s", w.Outcome,
			outcome.Performed, strings.Join(w.Transcript, "\n"))
	}
	if !w.Routine {
		t.Error("the run is not reported as a routine's")
	}
	if w.Active {
		t.Error("the slot is still claimed after the run finished")
	}
	// AND IT REALLY RAN THE PROGRAM. The routine's own log line is the proof that the
	// command found the library this window handed it rather than some other one.
	if !strings.Contains(strings.Join(w.Transcript, "\n"), "nothing was pressed") {
		t.Errorf("the routine's own output is not in the transcript:\n%s",
			strings.Join(w.Transcript, "\n"))
	}
}

func fileThere(p string) bool { _, err := os.Stat(p); return err == nil }

// ── TWO ROUTINES OF ONE NAME ──────────────────────────────────────────────────

// aSecondRoutineCalled records and saves another routine under `name` for `app`.
//
// Through the production path, like the first: `recorded` builds a draft whose steps and
// application come from the recorded events, so what lands on disk is what Studio writes.
func aSecondRoutineCalled(t *testing.T, a *Acts, name, app, scope string) {
	t.Helper()
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: []recorder.RecordedEvent{
			{Kind: recorder.EvAppSwitch, KeyName: app, T: at(0)},
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 640, Y: 480,
				App: app, PID: gamePID + 1, T: at(100), WinRel: true,
				RelX: 500, RelY: 400, RelTo: macroir.RelToWindow,
				WinW: 1280, WinH: 720, Dpi: 96, Process: app + ".exe"},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 640, Y: 480,
				App: app, PID: gamePID + 1, T: at(105)},
		}}
	}
	if why := a.StartRecording(name, ""); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}
	if why := a.SaveDraft(scope, app); why != "" {
		t.Fatalf("saving the second %q: %s", name, why)
	}
}

// SAME NAME, TWO APPLICATIONS: EACH SHOWS ITS OWN SOURCE AND LAUNCHES ITS OWN ROUTE.
//
// The case the whole identity exists for. A name is not unique: one name saved for two
// applications is two routines, and every lookup used to answer with the FIRST row that matched —
// so the window could display one and perform the other. Handing the child the route's full
// identity was already right and was happening AFTER the wrong routine had been chosen.
func TestTwoRoutinesOfOneNameAreTwoRoutines(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, nil)
	if why := a.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatal(why)
	}
	// AND THE SAME NAME AGAIN, in another application.
	a.capture = nil
	aSecondRoutineCalled(t, a, "ark graphics", "discord", "app-anywhere")

	listed := a.Routines()
	var same []Routine
	for _, r := range listed {
		if strings.EqualFold(r.Name, "ark graphics") {
			same = append(same, r)
		}
	}
	if len(same) != 2 {
		t.Fatalf("saving one name in two applications gave %d routine(s): %+v",
			len(same), listed)
	}
	if same[0].ID == same[1].ID {
		t.Fatalf("both have the identity %q, so nothing can tell them apart",
			same[0].ID)
	}
	// AND EACH ONE IS ITSELF: its own application, its own saved Marco, its own route.
	for _, want := range []struct{ id, app string }{
		{"arkascended/focus/ark-graphics", "arkascended"},
		{"discord/focus/ark-graphics", "discord"},
	} {
		t.Run(want.app, func(t *testing.T) {
			r, found := routineWithID(a.Routines(), want.id)
			if !found {
				t.Fatalf("there is no routine %q: %+v", want.id,
					a.Routines())
			}
			if r.App != want.app {
				t.Errorf("%q is listed in %q", want.id, r.App)
			}
			// THE SOURCE IS ITS OWN, which is what the preview shows.
			if !strings.Contains(r.Source, want.app) {
				t.Errorf("%q shows the other routine's saved Marco:\n%s",
					want.id, r.Source)
			}
			if !r.Runnable {
				t.Fatalf("%q cannot be run: %s", want.id, r.Why)
			}
			// AND PRESSING PLAY ON IT ASKS FOR EXACTLY THAT ROUTE.
			raised := ""
			a.raising = func(app string) string {
				raised = app
				return "that application is not running"
			}
			if why := a.PlayRoutine(want.id); why == "" {
				t.Fatal("a run started although the application was " +
					"not there")
			}
			if raised != want.app {
				t.Errorf("pressing Play on %q brought %q forward",
					want.id, raised)
			}
			_, args, _ := a.doCommand(r)
			if !hasArg(args, "--app="+want.app) {
				t.Errorf("the command for %q asks for %v", want.id, args)
			}
		})
	}
}

// AND AN UNRUNNABLE NAMESAKE DOES NOT DISABLE A VALID ROUTINE.
//
// The listing keyed "cannot press anything" by SLUG, and a slug is shared by every scope it was
// ever saved under — so a context-scoped routine being unable to press anything marked the
// focus-scoped routine of the same name unable to press anything too. A valid routine refused for
// somebody else's reason, with that routine's sentence beside it describing a scope it is not in.
func TestAnUnrunnableNamesakeDoesNotDisableTheValidRoutine(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, nil)
	dr := a.capture.draft
	// THE BROKEN ONE FIRST, under the scope the old rule allowed: guarded presses and
	// nothing to bring a window forward, which is the state on disk `Unrunnable` exists
	// for. Through the backend, because `SaveDraft` refuses this scope now.
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	if _, err := d.Save(dr, orchestrator.ScopeApp, "arkascended"); err != nil {
		t.Fatal(err)
	}
	// AND THE GOOD ONE BESIDE IT: the same name, the same slug, the scope its presses
	// need.
	if _, err := d.Save(dr, orchestrator.ScopeAppAnywhere, "arkascended"); err != nil {
		t.Fatal(err)
	}

	broken, had := routineWithID(a.Routines(), "arkascended/context/ark-graphics")
	if !had {
		t.Fatalf("the context-scoped namesake is not listed: %+v", a.Routines())
	}
	if broken.Runnable {
		t.Error("the one that cannot press anything is offered for Play")
	}
	good, found := routineWithID(a.Routines(), "arkascended/focus/ark-graphics")
	if !found {
		t.Fatalf("the valid routine is not listed: %+v", a.Routines())
	}
	if !good.Runnable {
		t.Fatalf("a valid routine is disabled by its namesake in another scope, "+
			"with that one's reason beside it: %s", good.Why)
	}
	if good.Why != "" {
		t.Errorf("the valid routine carries a refusal: %s", good.Why)
	}
	// AND PRESSING PLAY ON THE VALID ONE IS NOT REFUSED FOR THE OTHER'S REASON.
	a.raising = func(string) string { return "that application is not running" }
	if why := a.PlayRoutine(good.ID); !strings.Contains(why, "not running") {
		t.Errorf("pressing Play on the valid routine answered: %s", why)
	}
}

// AND SAVING ONE OF TWO BROKEN NAMESAKES AGAIN REPAIRS THE ONE BESIDE THE BUTTON.
//
// The repair card had the same defect in the same file: `Resave` matched a NAME and took the
// first that answered, so the button beside the second row saved the first row's routine and
// reported the first one's name while doing it.
func TestSavingOneOfTwoNamesakesAgainRepairsTheOneAskedFor(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, nil)
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	if _, err := d.Save(a.capture.draft, orchestrator.ScopeApp, "arkascended"); err != nil {
		t.Fatal(err)
	}
	// THE SAME NAME, BROKEN THE SAME WAY, IN ANOTHER APPLICATION.
	a.capture = nil
	aSecondRoutineCalled(t, a, "ark graphics", "discord", "app-anywhere")
	if _, err := d.Save(a.capture.draft, orchestrator.ScopeApp, "discord"); err != nil {
		t.Fatal(err)
	}

	listed := a.Unrunnable()
	if len(listed) != 2 {
		t.Fatalf("two routines cannot press anything and %d are listed: %+v",
			len(listed), listed)
	}
	var want Unrunnable
	for _, u := range listed {
		if u.App == "discord" {
			want = u
		}
	}
	if want.ID == "" {
		t.Fatalf("the discord one is not in the listing: %+v", listed)
	}
	// A NAME IS NOT AN IDENTITY HERE EITHER, so handing one over repairs nothing: it
	// names both of these, and repairing "whichever answered first" is the defect.
	if why := a.Resave(want.Name); why == "" {
		t.Fatalf("%q repaired a routine, and it names both of them", want.Name)
	}
	if len(a.Unrunnable()) != 2 {
		t.Fatalf("a name repaired one of them after all: %+v", a.Unrunnable())
	}
	if why := a.Resave(want.ID); why != "" {
		t.Fatalf("saving it again was refused: %s", why)
	}
	// THE ONE ASKED FOR IS REPAIRED AND THE OTHER IS UNTOUCHED. Repairing the wrong one
	// leaves this list the same length with the other entry gone, which is why both are
	// checked rather than just the count.
	left := a.Unrunnable()
	if len(left) != 1 {
		t.Fatalf("after one repair, %d routine(s) cannot press anything: %+v",
			len(left), left)
	}
	if left[0].App != "arkascended" {
		t.Errorf("the repair took the wrong routine: %q was asked for and %q is "+
			"what is left", want.ID, left[0].ID)
	}
}

// A ROUTINE'S RUN HAS A REFERENCE NOW, AND THIS WINDOW READS IT RATHER THAN MINTING ONE.
//
// The executor claims the reference before it sends anything and prints the citation; a window
// that made up an id of its own would be a second author of the same run, and a routine somebody
// performed from a terminal would have one this had never heard of. It is also what makes Report
// reachable for a routine — the whole point of the record.
//
// AND THE CITATION IS NOT THE REASON IT STOPPED. The citation is the last thing a child says
// before it starts pressing, so a window that read the lines in the wrong order would show a
// file path as the sentence explaining what went wrong. The refusal sentence comes AFTER it here
// for exactly that reason.
func TestTheRunsOwnReferenceReachesTheWindow(t *testing.T) {
	const id = "run-20261024-150405-ab12"
	said := "ark-graphics stopped: the window this was recorded in was 1920x1080 and " +
		"is now 1280x720"
	w := aRunThatSaid(t, "[route] ark graphics",
		runref.Cite(id, `C:\home\marco\runs\`+id+`.json`),
		said, outcome.Line(outcome.Failed))

	if w.Reference != id {
		t.Errorf("the window shows the reference %q, want %q", w.Reference, id)
	}
	if w.Reason != said {
		t.Errorf("the reason the window kept is %q — the citation was read as the "+
			"sentence explaining what went wrong", w.Reason)
	}
	if !w.Routine {
		t.Error("the run no longer says it is a routine's")
	}
}

// AND SAVING ONE AGAIN KEEPS WHAT IT SAYS ABOUT HOW IT WAS RECORDED.
//
// The repair button is in THIS window, and this window is open all afternoon under whatever
// launched it. `Resave` rebuilt the routine from the demonstration beside it while stating
// nothing about the mode, so a routine that said it was recorded as coordinates came back out
// saying nothing — and with `$MARCO_CV` inherited on, the same rebuild capped the recorded
// pauses of a routine that has no anchor to wait on in their place.
//
// Here because the card is where somebody reads it: the listing's "recorded as" sentence comes
// off the saved source through the one reader of that line, so a repair that dropped the line
// takes the sentence off the card. The pauses and the word are held at the repair itself, in
// `internal/orchestrator` `TestARepairKeepsTheRecordedPausesOfACoordinateRoutine`.
func TestSavingAgainKeepsWhatTheRoutineSaysItRecorded(t *testing.T) {
	// THE SENSING FLAGS INHERITED ON, which is the case that made it a defect rather
	// than a tidiness.
	t.Setenv("MARCO_CV", "on")
	t.Setenv("MARCO_ANCHORS", "1")
	t.Setenv("MARCO_ROUTES", t.TempDir())

	a := recorded(t, nil)
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	rt, err := d.Save(a.capture.draft, orchestrator.ScopeApp, "arkascended")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(a.reg().Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if got := orchestrator.ModeOf(string(raw)); got != orchestrator.Coordinates {
		t.Fatalf("this window saved a routine stating %q, so there is nothing here "+
			"for a repair to lose:\n%s", got, raw)
	}

	listed := a.Unrunnable()
	if len(listed) != 1 {
		t.Fatalf("%d routines cannot press anything, want the one just saved: %+v",
			len(listed), listed)
	}
	if why := a.Resave(listed[0].ID); why != "" {
		t.Fatalf("saving it again was refused: %s", why)
	}

	var row Routine
	for _, r := range a.Routines() {
		if r.Slug == rt.Slug {
			row = r
		}
	}
	if row.ID == "" {
		t.Fatalf("the repaired routine is not in the listing: %+v", a.Routines())
	}
	if row.Mode != string(orchestrator.Coordinates) {
		t.Errorf("after being saved again the card says it was recorded as %q, and "+
			"the routine was recorded as %q — the repair took the statement out "+
			"of the file the card reads:\n%s", row.Mode,
			orchestrator.Coordinates, row.Source)
	}
	// AND IN A SENTENCE, because the card shows somebody what the word means for the
	// routine in front of them rather than the word on its own.
	if row.Captures == "" {
		t.Error("the card carries the mode with nothing said about what it means")
	}
}

// A STOPPED ROUTINE SHOWS WHAT IT LEFT HELD DOWN.
//
// A stop is not a failure and this window says so in its own words — *"ark graphics" was stopped
// part of the way through.* But a run stopped between a chord's modifiers and its key can leave a
// modifier down on the machine, and until it is released every keystroke anybody types arrives
// modified. The executor now says that before it announces the outcome, and keeps it in the
// durable record; a window that printed only the stop sentence would be the one place a person
// looks, not mentioning it.
//
// The window does not parse the warning and must not: it keeps the last thing the child said and
// shows it. So this is a test about the sentence reaching the surface, not about the surface
// understanding it. The executor's half is `cmd/marco`
// `TestAStoppedRunRecordsTheKeyItLeftHeldDown`; the sequence that produces it is
// `internal/oshost` `TestARunStoppedMidChordLeavesNothingPressedAndSaysWhatIsStuck`.
// See [[ADR-258-a-chord-is-a-promise-to-let-go]].
func TestAStoppedRoutineShowsWhatItLeftHeldDown(t *testing.T) {
	stuck := "after the ctrl+shift+a key, the machine would not release ctrl, which may " +
		"still be held down on this computer — until it is, every keystroke anybody " +
		"types arrives modified"
	w := aRunThatSaid(t, "[route] ark graphics", stuck,
		outcome.Line(outcome.Cancelled))

	if w.Outcome != string(outcome.Cancelled) {
		t.Fatalf("the run was stopped and the window shows %q", w.Outcome)
	}
	if w.Reason != stuck {
		t.Errorf("the window kept %q as what the run said, and the run said %q",
			w.Reason, stuck)
	}
	// AND IT IS IN THE SENTENCE SOMEBODY READS, not only in a field.
	switch {
	case !strings.Contains(w.Said, "stopped part of the way through"):
		t.Errorf("the window no longer says a stopped routine was stopped: %q", w.Said)
	case !strings.Contains(w.Said, "would not release ctrl"):
		t.Errorf("the window says %q about a run that left Ctrl held down — the stop "+
			"is reported and the key still down is not, which is the half "+
			"somebody has to do something about", w.Said)
	}
}

// AND A STOPPED ROUTINE SHOWS A BUTTON IT LEFT HELD DOWN THE SAME WAY.
//
// The same channel, the other device, and the worse one: a held left button drags whatever the
// pointer passes over, with no window in front of it to show what is happening. The window does
// not parse this sentence either — it keeps the last thing the child said — so what is being
// tested is that a warning about a BUTTON reaches the surface through the path built for keys,
// and that the stop still reads as a stop.
//
// The sentence is `oshost.ButtonNotReleased`'s, as a person would receive it. The executor's half
// is `cmd/marco` `TestAStoppedRunRecordsTheButtonItLeftHeldDown`; the gesture that produces it is
// `internal/oshost`
// `TestARunStoppedMidDragKeepsTheStuckButtonWhereACancellationCannotDiscardIt`.
// See [[ADR-258-a-chord-is-a-promise-to-let-go]].
func TestAStoppedRoutineShowsTheButtonItLeftHeldDown(t *testing.T) {
	stuck := "after dragging from 10,20 to 250,20, the machine would not release the " +
		"left mouse button, which may still be held down at 130,20 on this " +
		"computer — until it is, the pointer drags whatever it passes over"
	w := aRunThatSaid(t, "[route] ark graphics", stuck,
		outcome.Line(outcome.Cancelled))

	if w.Outcome != string(outcome.Cancelled) {
		t.Fatalf("the run was stopped and the window shows %q", w.Outcome)
	}
	if w.Reason != stuck {
		t.Errorf("the window kept %q as what the run said, and the run said %q",
			w.Reason, stuck)
	}
	switch {
	case !strings.Contains(w.Said, "stopped part of the way through"):
		t.Errorf("the window no longer says a stopped routine was stopped: %q", w.Said)
	case !strings.Contains(w.Said, "left mouse button"):
		t.Errorf("the window says %q about a run that left the left button down — the "+
			"stop is reported and the button still held is not, which is the "+
			"half that is still happening to everything the pointer touches",
			w.Said)
	}
}

// AND A STOPPED ROUTINE SHOWS A FIELD IT LEFT HALF FILLED, WHICH IS NOT A KEY AT ALL.
//
// The same channel, a third device, and the first one that is not about something held down. A
// typing stopped between two characters leaves part of a value in a field — nothing is pressed,
// nothing afterwards arrives modified, and the thing somebody has to know is that the field they
// are looking at is not what the routine was going to put there.
//
// The window does not parse this sentence either; it keeps the last thing the child said. So what
// is tested is that a warning about TEXT reaches the surface through the path built for keys, that
// the stop still reads as a stop, and that what reaches the card does not describe a half-typed
// field as a stuck key.
//
// The sentence is `oshost.PartlyTypedText`'s, as a person receives it. The executor's half is
// `cmd/marco` `TestAStoppedRunRecordsTheTextItLeftHalfSubmitted`; the gesture that produces it is
// `internal/oshost` `TestAStoppedTypingIsKeptWhereACancelledRunCannotDiscardIt`.
// See [[ADR-261-typed-text-is-a-sequence-of-submissions]].
func TestAStoppedRoutineShowsTheTextItLeftHalfSubmitted(t *testing.T) {
	half := oshost.PartlyTypedText(4, 11, false).Error()
	w := aRunThatSaid(t, "[route] ark graphics", half,
		outcome.Line(outcome.Cancelled))

	if w.Outcome != string(outcome.Cancelled) {
		t.Fatalf("the run was stopped and the window shows %q", w.Outcome)
	}
	if w.Reason != half {
		t.Errorf("the window kept %q as what the run said, and the run said %q",
			w.Reason, half)
	}
	switch {
	case !strings.Contains(w.Said, "stopped part of the way through"):
		t.Errorf("the window no longer says a stopped routine was stopped: %q", w.Said)
	case !strings.Contains(w.Said, "4 of 11"):
		t.Errorf("the window says %q about a run that left part of a value in a "+
			"field — the stop is reported and the field is not, which is the "+
			"half somebody is looking at", w.Said)
	case strings.Contains(w.Said, "held down"):
		t.Errorf("the window describes a half-filled field as something still held "+
			"down: %q", w.Said)
	}
}

// AND A SECRET SHOWS NO NUMBER ON THE CARD EITHER.
//
// The last layer the suppression has to hold at, and the one a person actually reads. A length is
// a fact about a password, and a failure position is a prefix whose size somebody now knows.
func TestAStoppedRoutineShowsNoNumberAboutASecret(t *testing.T) {
	half := oshost.PartlyTypedSecret(false).Error()
	w := aRunThatSaid(t, "[route] ark graphics", half,
		outcome.Line(outcome.Cancelled))

	if !strings.Contains(w.Said, "secret") {
		t.Errorf("the window says %q and does not mention the secret that was left "+
			"unfinished", w.Said)
	}
	if strings.ContainsAny(w.Reason, "0123456789") {
		t.Errorf("the card carries %q, and a number about a secret is a fact about "+
			"the value", w.Reason)
	}
}

// AND AN ORDINARY STOP SAYS ONLY THAT IT STOPPED.
//
// The control. A window that appended whatever the child last said to every cancelled run would
// satisfy the case above and would also show somebody a file path, a log line, or nothing at all
// dressed up as a warning.
func TestAnOrdinaryStopSaysOnlyThatItStopped(t *testing.T) {
	w := aRunThatSaid(t, "[route] ark graphics", outcome.Line(outcome.Cancelled))

	if w.Outcome != string(outcome.Cancelled) {
		t.Fatalf("the window shows %q", w.Outcome)
	}
	if w.Reason != "" {
		t.Errorf("a plain stop kept %q as a reason", w.Reason)
	}
	if !strings.Contains(w.Said, "stopped part of the way through") ||
		strings.Contains(w.Said, "It said:") {
		t.Errorf("a plain stop reads as %q", w.Said)
	}
}

// theRecordSays is the run record a stuck run claimed, or why there is none.
//
// For the diagnostic above. The record is the product's own account of an attempt and it is
// written BEFORE the first input, so it exists even for a run that never reached its end — which
// is the whole reason it is written then. `ended` absent means the child never finished.
func theRecordSays(a *Acts, reference string) string {
	if reference == "" {
		return "the run cited no reference, so there is no record to look in"
	}
	at := filepath.Join(a.home, "runs", reference+".json")
	raw, err := os.ReadFile(at)
	if err != nil {
		return fmt.Sprintf("no record at %s (%v)", at, err)
	}
	return string(raw)
}
