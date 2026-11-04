package main

// A ROUTINE'S SHORTCUTS, FROM THE WINDOW, AGAINST A REAL LIBRARY (S3).
//
// Through `Acts` with routines saved on disk and the shortcuts file the owner writes, so what is
// held is the wiring as well as the words: a key set here has to be the key a press runs. The rules
// themselves are S2's owner, `internal/orchestrator` shortcuts.go, and are tested there; what is
// here is that this window asks the owner and shows what it said.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// theDefaultLeader pins the widget's configuration to "nothing configured", so the leader is the
// widget's default and no setting on this machine reaches the test.
func theDefaultLeader(t *testing.T) {
	t.Helper()
	t.Setenv("MARCO_OVERLAY_CONFIG", filepath.Join(t.TempDir(), "overlay.json"))
	t.Setenv("MARCO_OVERLAY_LEADER", "")
}

// twoRoutinesInStudio is a library with a From-anywhere routine and an Only-in one, both about
// notepad.
func twoRoutinesInStudio(t *testing.T) (*Acts, Routine, Routine) {
	t.Helper()
	theDefaultLeader(t)
	a := aStudioLibrary(t, "type two letters", typingIntoNotepad())
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	dr := d.DraftOf("save as pdf", typingIntoNotepad(), "notepad", "the stop key",
		orchestrator.Unstated)
	if _, err := d.Save(dr, orchestrator.ScopeApp, "notepad"); err != nil {
		t.Fatal(err)
	}
	var anywhere, onlyIn Routine
	for _, r := range a.Routines() {
		switch r.Scope {
		case string(orchestrator.ScopeAppAnywhere):
			anywhere = r
		case string(orchestrator.ScopeApp):
			onlyIn = r
		}
	}
	if anywhere.ID == "" || onlyIn.ID == "" {
		t.Fatalf("the fixture holds %+v", a.Routines())
	}
	return a, anywhere, onlyIn
}

func keyPressed(t *testing.T, a *Acts, app, key string) orchestrator.Pressed {
	t.Helper()
	return (orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}).ShortcutPressed(app, key)
}

// A routine with no shortcut: the leader, no keys, the control, its own application offered first,
// and nothing to complain about.
func TestTheShortcutTabOffersTheLeaderAndTheRoutinesOwnApplicationFirst(t *testing.T) {
	a, r, _ := twoRoutinesInStudio(t)
	got := a.RoutineShortcuts(r.ID)
	if !got.Askable || got.Why != "" {
		t.Fatalf("an ordinary routine cannot be given a shortcut: %+v", got)
	}
	if got.Leader != "`" || !strings.Contains(got.LeaderSays, "(`)") {
		t.Errorf("the leader is %q (%q), want the widget's default", got.Leader, got.LeaderSays)
	}
	if len(got.Keys) != 0 || len(got.Legacy) != 0 {
		t.Errorf("a routine with no shortcut lists %+v %+v", got.Keys, got.Legacy)
	}
	var def []ShortcutPlace
	for _, p := range got.Places {
		if p.Default {
			def = append(def, p)
		}
	}
	if len(def) != 1 || def[0].ListenIn != "notepad" {
		t.Errorf("the place offered first is %+v, want notepad and only notepad", def)
	}
}

// A key set in Studio is the key a press runs, and the row shows it.
func TestAKeySetInStudioIsWhatAPressRuns(t *testing.T) {
	a, r, _ := twoRoutinesInStudio(t)
	got := a.SetShortcut(r.ID, "e", "notepad", "")
	if got.Failed || got.Taken != nil {
		t.Fatalf("setting a free key: %+v", got)
	}
	if len(got.Keys) != 1 || got.Keys[0].Pressed != "`e" || got.Keys[0].Where != "in notepad" {
		t.Errorf("the tab lists %+v", got.Keys)
	}
	if !strings.Contains(got.Said, "`e in notepad now runs") {
		t.Errorf("what it came to: %q", got.Said)
	}
	p := keyPressed(t, a, "notepad", "e")
	if len(p.Steps) != 1 || orchestrator.RouteID(p.Steps[0].Route) != r.ID {
		t.Fatalf("`e in notepad fires %+v (%s), want %s", p.Steps, p.Refused, r.ID)
	}
	row, _ := a.routineWithID(r.ID)
	if len(row.Keys) != 1 || row.Keys[0] != "`e notepad" {
		t.Errorf("the routine's row shows %q", row.Keys)
	}
}

// A key in use is SHOWN, with its version, and nothing is written. Only a second Set carrying
// that version replaces it; a version nobody was shown is refused.
func TestAKeyInUseIsShownAndReplacedOnlyWithItsVersion(t *testing.T) {
	a, r, other := twoRoutinesInStudio(t)
	if s := a.SetShortcut(other.ID, "e", "notepad", ""); s.Failed {
		t.Fatal(s.Said)
	}
	got := a.SetShortcut(r.ID, "e", "notepad", "")
	if got.Taken == nil || got.Taken.Version == "" || !strings.Contains(got.Taken.Runs, "save as pdf") {
		t.Fatalf("a key in use was not shown with what is on it: %+v", got)
	}
	if p := keyPressed(t, a, "notepad", "e"); orchestrator.RouteID(p.Steps[0].Route) != other.ID {
		t.Fatal("showing the key in use replaced it")
	}
	if s := a.SetShortcut(r.ID, "e", "notepad", "not-the-version"); !s.Failed {
		t.Fatalf("a version nobody was shown replaced the key: %+v", s)
	}
	if s := a.SetShortcut(r.ID, "e", "notepad", got.Taken.Version); s.Failed {
		t.Fatalf("replacing with the version shown: %s", s.Said)
	}
	if p := keyPressed(t, a, "notepad", "e"); orchestrator.RouteID(p.Steps[0].Route) != r.ID {
		t.Fatalf("after the replacement `e fires %+v", p.Steps)
	}
}

// A change moves the key in one write: the old one is gone and the new one runs the routine.
func TestAChangeInStudioMovesTheKey(t *testing.T) {
	a, r, _ := twoRoutinesInStudio(t)
	set := a.SetShortcut(r.ID, "e", "notepad", "")
	k := set.Keys[0]
	got := a.ChangeShortcut(r.ID, k.ListenIn, k.Key, k.Version, "j", "", "")
	if got.Failed {
		t.Fatal(got.Said)
	}
	if p := keyPressed(t, a, "notepad", "e"); p.Found {
		t.Error("the old key still runs something after the change")
	}
	if p := keyPressed(t, a, "paint", "j"); len(p.Steps) != 1 ||
		orchestrator.RouteID(p.Steps[0].Route) != r.ID {
		t.Errorf("`j everywhere fires %+v (%s)", p.Steps, p.Refused)
	}
	// AND A CHANGE FROM A VERSION NOBODY WAS SHOWN WRITES NOTHING.
	stale := a.ChangeShortcut(r.ID, "", "j", "not-the-version", "k", "", "")
	if !stale.Failed {
		t.Fatal("a change went ahead from a version nobody was shown")
	}
	if p := keyPressed(t, a, "paint", "k"); p.Found {
		t.Error("the refused change wrote its new key")
	}
}

// Removing needs the version shown.
func TestRemovingInStudioNeedsTheVersionShown(t *testing.T) {
	a, r, _ := twoRoutinesInStudio(t)
	set := a.SetShortcut(r.ID, "e", "notepad", "")
	if s := a.RemoveShortcut(r.ID, "notepad", "e", "not-the-version"); !s.Failed {
		t.Fatal("a removal went ahead against a version nobody was shown")
	}
	if !keyPressed(t, a, "notepad", "e").Found {
		t.Fatal("the refused removal removed it")
	}
	got := a.RemoveShortcut(r.ID, "notepad", "e", set.Keys[0].Version)
	if got.Failed || len(got.Keys) != 0 || !strings.Contains(got.Said, "no longer runs") {
		t.Fatalf("removing with the version shown: %+v", got)
	}
}

// An Only-in routine is not offered everywhere, and the owner refuses it if asked anyway.
func TestAnOnlyInRoutineIsNotOfferedEverywhere(t *testing.T) {
	a, _, onlyIn := twoRoutinesInStudio(t)
	got := a.RoutineShortcuts(onlyIn.ID)
	for _, p := range got.Places {
		if (p.ListenIn == "" || p.Named) && (p.Allowed || p.Why == "") {
			t.Errorf("an Only-in routine is offered %+v", p)
		}
	}
	if s := a.SetShortcut(onlyIn.ID, "e", "", ""); !s.Failed {
		t.Fatalf("an Only-in routine was given a shortcut everywhere: %+v", s)
	}
	if s := a.SetShortcut(onlyIn.ID, "e", "paint", ""); !s.Failed {
		t.Fatalf("an Only-in routine was given a shortcut in another application: %+v", s)
	}
	if keyPressed(t, a, "paint", "e").Found {
		t.Error("it was written anyway")
	}
}

// A shortcut set up by name is shown on the routine its words name, with exactly what it would
// run, is NOT migrated by being looked at, and is reassigned only to the plan shown.
func TestAShortcutSetUpByNameIsReassignedOnlyOnRequest(t *testing.T) {
	a, r, _ := twoRoutinesInStudio(t)
	if err := a.reg().Bind("notepad", "s", "type two letters"); err != nil {
		t.Fatal(err)
	}
	got := a.RoutineShortcuts(r.ID)
	if len(got.Legacy) != 1 || !got.Legacy[0].Reassignable || got.Legacy[0].Plan != r.ID {
		t.Fatalf("the shortcut set up by name is shown as %+v", got.Legacy)
	}
	if b, _ := a.reg().BindingAt("notepad", "s"); !b.Legacy() {
		t.Fatal("looking at a shortcut set up by name migrated it")
	}
	if p := keyPressed(t, a, "notepad", "s"); p.Refused == "" {
		t.Fatal("a shortcut set up by name runs before it is reassigned")
	}
	l := got.Legacy[0]
	if s := a.ReassignShortcut(r.ID, l.ListenIn, l.Key, l.Version, "global/something-else"); !s.Failed {
		t.Fatal("a reassignment went ahead to a plan nobody was shown")
	}
	if s := a.ReassignShortcut(r.ID, l.ListenIn, l.Key, l.Version, l.Plan); s.Failed {
		t.Fatalf("reassigning to the plan shown: %s", s.Said)
	}
	if p := keyPressed(t, a, "notepad", "s"); len(p.Steps) != 1 ||
		orchestrator.RouteID(p.Steps[0].Route) != r.ID {
		t.Errorf("after reassignment `s fires %+v (%s)", p.Steps, p.Refused)
	}
	if b, _ := a.reg().BindingAt("notepad", "s"); b.Command() != "type two letters" {
		t.Errorf("reassignment changed its words to %q", b.Command())
	}
}

// Nothing is offered while Studio is recording.
func TestTheShortcutTabHoldsOffWhileRecording(t *testing.T) {
	a, r, _ := twoRoutinesInStudio(t)
	a.recording = func() recorder.Recorder { return &heldRecorder{} }
	if why := a.StartRecording("something else", ""); why != "" {
		t.Fatal(why)
	}
	defer a.CancelRecording()
	if got := a.RoutineShortcuts(r.ID); got.Askable || got.Why == "" {
		t.Fatalf("shortcuts were offered during a recording: %+v", got)
	}
	if s := a.SetShortcut(r.ID, "e", "notepad", ""); s.Askable {
		t.Error("a shortcut was set during a recording")
	}
	if keyPressed(t, a, "notepad", "e").Found {
		t.Error("it was written anyway")
	}
}

// A shortcuts file that cannot be read is said, never shown as "no shortcuts", and nothing is
// offered over it.
func TestAnUnreadableShortcutsFileIsSaidAndNothingIsOffered(t *testing.T) {
	a, r, _ := twoRoutinesInStudio(t)
	if err := os.WriteFile(filepath.Join(a.reg().Dir, "bindings.json"), []byte("{broken"),
		0o644); err != nil {
		t.Fatal(err)
	}
	got := a.RoutineShortcuts(r.ID)
	if got.Askable || !strings.Contains(got.Why, "could not be read") {
		t.Fatalf("an unreadable shortcuts file was shown as %+v", got)
	}
}

// A global shortcut that an application's own shortcut hides there says so.
func TestAGlobalShortcutSaysWhereItIsHidden(t *testing.T) {
	a, r, other := twoRoutinesInStudio(t)
	if s := a.SetShortcut(r.ID, "c", "", ""); s.Failed {
		t.Fatal(s.Said)
	}
	if s := a.SetShortcut(other.ID, "c", "notepad", ""); s.Failed {
		t.Fatal(s.Said)
	}
	got := a.RoutineShortcuts(r.ID)
	if len(got.Keys) != 1 || !strings.Contains(got.Keys[0].Shadows, "In notepad, `c runs \"save as pdf\" instead") {
		t.Fatalf("the hidden global shortcut is listed as %+v", got.Keys)
	}
}

// THE REPLACE CARD'S QUESTION SAYS A ROUTINE'S SHORTCUTS ARE KEPT, with the leader, and after the
// replacement the key runs the new version (the owner's ruling, 2026-11-03).
func TestTheReplaceQuestionSaysItsShortcutsAreKept(t *testing.T) {
	theDefaultLeader(t)
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	first := aRecordingCalled(t, "settings", 842, 274)
	if why := first.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatal(why)
	}
	rt := routes.Route{App: "arkascended", Focus: true, Slug: "settings"}
	if s := first.SetShortcut(orchestrator.RouteID(rt), "e", "arkascended", ""); s.Failed {
		t.Fatal(s.Said)
	}
	again := aRecordingCalled(t, "settings", 700, 500)
	at := again.Collision("app-anywhere", "arkascended")
	if len(at.Shortcuts) != 1 || at.Shortcuts[0] != "`e in arkascended" {
		t.Fatalf("the question names the shortcuts %q", at.Shortcuts)
	}
	if why := again.ReplaceDraft(at.ID, at.Source, at.Recording, "app-anywhere",
		"arkascended"); why != "" {
		t.Fatal(why)
	}
	now, err := os.ReadFile(again.reg().Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	p := keyPressed(t, again, "arkascended", "e")
	if len(p.Steps) != 1 || p.Steps[0].Pin != routes.DigestOf(string(now)) {
		t.Fatalf("after the replacement `e fires %+v (%s), want the new version", p.Steps,
			p.Refused)
	}
}
