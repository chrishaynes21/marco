package main

// WHAT SAVING OVER A ROUTINE DOES, AND WHAT IT MUST NOT.
//
// `Registry.Save` is a file write, so until this existed, recording a routine under a name
// already in the library replaced it — its source and the demonstration beside it, neither of
// them recoverable. The person most exposed to that is the one re-recording all afternoon to
// compare two attempts, which is exactly what a repeatable product invites.
//
// Every case here asserts the BYTES of what was already saved, not just the absence of an error.
// A refusal that returned a sentence and wrote the file anyway would pass a test that only read
// the sentence, and that is the defect being prevented.

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aRecordingCalled records one checked press under `name`, in its own window, into whatever
// library `$MARCO_ROUTES` names.
//
// Its own `Acts` each time, which is how two recordings reach one library the way two sittings
// would. The press point differs per call so the saved sources differ — a test that replaced one
// routine with a byte-identical one could not tell a replacement from a refusal.
func aRecordingCalled(t *testing.T, name string, x, y int) *Acts {
	t.Helper()
	a := newActs(t.TempDir())
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: []recorder.RecordedEvent{
			{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
			{Kind: recorder.EvClick, Button: "left", Down: true, X: x, Y: y,
				App: "arkascended", PID: gamePID, T: at(100), WinRel: true,
				RelX: x - 120, RelY: y - 64, RelTo: macroir.RelToWindow,
				WinW: 1920, WinH: 1080, Dpi: 144, Process: "arkascended.exe"},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: x, Y: y,
				App: "arkascended", PID: gamePID, T: at(105)},
		}}
	}
	if why := a.StartRecording(name, ""); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}
	return a
}

// kept is the saved routine and the demonstration beside it, as bytes.
func kept(t *testing.T, a *Acts, app, slug string) (src, demo []byte) {
	t.Helper()
	rt := routes.Route{App: app, Slug: slug, Focus: true}
	src, err := os.ReadFile(a.reg().Path(rt))
	if err != nil {
		t.Fatalf("reading what was saved at %s/%s: %v", app, slug, err)
	}
	demo, err = os.ReadFile(a.reg().RecordingPath(rt))
	if err != nil {
		t.Fatalf("reading the demonstration at %s/%s: %v", app, slug, err)
	}
	return src, demo
}

// unchanged fails unless both files are byte for byte what they were.
func unchanged(t *testing.T, what string, wasSrc, wasDemo, nowSrc, nowDemo []byte) {
	t.Helper()
	if !bytes.Equal(wasSrc, nowSrc) {
		t.Errorf("%s: the saved play's source changed\n--- was ---\n%s\n--- now ---\n%s",
			what, wasSrc, nowSrc)
	}
	if !bytes.Equal(wasDemo, nowDemo) {
		t.Errorf("%s: the demonstration beside it changed, which is the copy a "+
			"play is saved again from", what)
	}
}

// AN ORDINARY SAVE OVER AN EXISTING ROUTINE IS REFUSED, AND LEAVES IT EXACTLY AS IT WAS.
func TestSavingOverARoutineIsRefusedAndLeavesItAlone(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())

	first := aRecordingCalled(t, "ark graphics", 842, 274)
	if why := first.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatalf("the first recording would not save: %s", why)
	}
	wasSrc, wasDemo := kept(t, first, "arkascended", "ark-graphics")

	second := aRecordingCalled(t, "ark graphics", 500, 400)

	// THE WINDOW CAN ASK FIRST, which is what lets it offer the choice.
	c := second.Collision("app-anywhere", "arkascended")
	if !c.Exists || c.ID != "arkascended/focus/ark-graphics" {
		t.Errorf("the window was told %+v about a name that is taken", c)
	}

	why := second.SaveDraft("app-anywhere", "arkascended")
	if why == "" {
		t.Fatal("the second recording was saved over the first without anybody " +
			"being asked")
	}
	for _, say := range []string{"already a play saved there", "ark graphics",
		"another name", "on purpose"} {
		if !strings.Contains(why, say) {
			t.Errorf("the refusal does not say %q:\n%s", say, why)
		}
	}
	nowSrc, nowDemo := kept(t, first, "arkascended", "ark-graphics")
	unchanged(t, "after a refused save", wasSrc, wasDemo, nowSrc, nowDemo)
}

// SAVE AS KEEPS BOTH, AND REFUSES A SECOND NAME THAT IS ALSO TAKEN.
func TestSavingUnderAnotherNameKeepsBoth(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())

	first := aRecordingCalled(t, "ark graphics", 842, 274)
	if why := first.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatalf("the first recording would not save: %s", why)
	}
	wasSrc, wasDemo := kept(t, first, "arkascended", "ark-graphics")

	second := aRecordingCalled(t, "ark graphics", 500, 400)
	if why := second.SaveDraftAs("ark graphics two", "app-anywhere",
		"arkascended"); why != "" {
		t.Fatalf("saving under another name was refused: %s", why)
	}

	// BOTH ROUTINES ARE THERE, and the first is untouched.
	nowSrc, nowDemo := kept(t, first, "arkascended", "ark-graphics")
	unchanged(t, "after a save under another name", wasSrc, wasDemo, nowSrc, nowDemo)
	altSrc, _ := kept(t, second, "arkascended", "ark-graphics-two")
	if bytes.Equal(altSrc, nowSrc) {
		t.Error("the two plays are the same bytes, so this test cannot tell " +
			"them apart")
	}

	// AND THE WAY OUT HAS THE SAME DOOR ON IT. The alternative name can be taken too,
	// and saving over THAT one would be the same loss by a longer route.
	third := aRecordingCalled(t, "ark graphics three", 600, 600)
	why := third.SaveDraftAs("ark graphics two", "app-anywhere", "arkascended")
	if why == "" {
		t.Fatal("Save as replaced a play that was already under that name")
	}
	if !strings.Contains(why, "already a play saved there") {
		t.Errorf("the refusal does not say what is in the way:\n%s", why)
	}
	stillSrc, _ := kept(t, second, "arkascended", "ark-graphics-two")
	if !bytes.Equal(altSrc, stillSrc) {
		t.Error("Save as changed the play it refused to replace")
	}
}

// EXPLICIT REPLACE CHANGES THE ONE THAT WAS CHOSEN, AND NOTHING THAT SHARES ITS NAME.
func TestReplacingChangesOnlyTheOneChosen(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())

	// THE SAME NAME IN TWO APPLICATIONS, which is two routines. The whole risk of a
	// replacement is that it reaches the namesake.
	inGame := aRecordingCalled(t, "settings", 842, 274)
	if why := inGame.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatal(why)
	}
	elsewhere := aRecordingCalled(t, "settings", 300, 300)
	if why := elsewhere.SaveDraft("app-anywhere", "discord"); why != "" {
		t.Fatal(why)
	}
	gameWas, _ := kept(t, inGame, "arkascended", "settings")
	otherSrc, otherDemo := kept(t, elsewhere, "discord", "settings")

	again := aRecordingCalled(t, "settings", 700, 500)
	// AS THE PAGE DOES IT: the card is drawn from a collision, and what it hands back is
	// that collision's identity AND the version it reported.
	at := again.Collision("app-anywhere", "arkascended")
	if !at.Exists || at.ID != "arkascended/focus/settings" {
		t.Fatalf("the collision to replace is %+v", at)
	}
	if why := again.ReplaceDraft(at.ID, at.Source, at.Recording, "app-anywhere",
		"arkascended"); why != "" {
		t.Fatalf("an explicit replacement was refused: %s", why)
	}

	gameNow, _ := kept(t, inGame, "arkascended", "settings")
	if bytes.Equal(gameWas, gameNow) {
		t.Error("the play that was chosen for replacement did not change")
	}
	nowSrc, nowDemo := kept(t, elsewhere, "discord", "settings")
	unchanged(t, "the namesake in another application", otherSrc, otherDemo,
		nowSrc, nowDemo)
}

// AND A REPLACEMENT REFUSES WHEN THE ROUTINE CHANGED SINCE THE CARD WAS DRAWN.
//
// A routine’s identity is its place in the library, and a place does not change when its contents
// do """ + DASH + """ so "the same routine is still there" was true of one that had been saved again while the
// card sat on screen, and that newer version was overwritten. Somebody agreed to lose the one the
// card described.
//
// This is the case the page’s round trip of the digests is for: the window hands back what it was
// SHOWN, and the backend reads the file again before it writes.
func TestAReplacementRefusesWhenTheRoutineChangedSinceTheCardWasDrawn(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())

	inGame := aRecordingCalled(t, "settings", 842, 274)
	if why := inGame.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatal(why)
	}
	again := aRecordingCalled(t, "settings", 700, 500)
	// THE CARD IS DRAWN.
	at := again.Collision("app-anywhere", "arkascended")
	if !at.Exists || at.Source == "" {
		t.Fatalf("the collision does not say which version it is about: %+v", at)
	}

	// AND SOMEBODY ELSE SAVES OVER IT WHILE IT SITS THERE. Same place, same identity,
	// contents nobody in this window has seen.
	rt := routes.Route{App: "arkascended", Focus: true, Slug: "settings"}
	newer := "// somebody else’s play\n" + string(mustKeep(t, inGame, rt))
	if err := inGame.reg().Replace(rt, newer); err != nil {
		t.Fatal(err)
	}

	why := again.ReplaceDraft(at.ID, at.Source, at.Recording, "app-anywhere",
		"arkascended")
	if why == "" {
		t.Fatal("it replaced a play that had been saved again since the card was " +
			"drawn")
	}
	if !strings.Contains(why, "saved again since you were shown it") {
		t.Errorf("the refusal does not say what changed:\n%s", why)
	}
	if now := string(mustKeep(t, inGame, rt)); now != newer {
		t.Errorf("the newer play was written over:\n%s", now)
	}
}

// mustKeep is a saved routine’s source through this window’s own library.
func mustKeep(t *testing.T, a *Acts, rt routes.Route) []byte {
	t.Helper()
	raw, err := os.ReadFile(a.reg().Path(rt))
	if err != nil {
		t.Fatalf("reading %s: %v", a.reg().Path(rt), err)
	}
	return raw
}

// AND A REPLACEMENT IS CHECKED AGAIN AT THE SAVE, NOT TRUSTED FROM THE WINDOW'S EARLIER LOOK.
//
// Two ways it can be wrong by then: the identity this save would write is not the one somebody
// agreed to replace, or the routine they chose has gone. Both refuse, and neither writes.
func TestReplacingSomethingElseIsRefused(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())

	inGame := aRecordingCalled(t, "settings", 842, 274)
	if why := inGame.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatal(why)
	}
	elsewhere := aRecordingCalled(t, "settings", 300, 300)
	if why := elsewhere.SaveDraft("app-anywhere", "discord"); why != "" {
		t.Fatal(why)
	}
	gameWas, gameDemo := kept(t, inGame, "arkascended", "settings")
	otherWas, otherDemo := kept(t, elsewhere, "discord", "settings")

	// ── A ROUTINE THAT IS NOT THE ONE THIS SAVE WOULD WRITE ──
	again := aRecordingCalled(t, "settings", 700, 500)
	at := again.Collision("app-anywhere", "arkascended")
	why := again.ReplaceDraft("discord/focus/settings", at.Source, at.Recording,
		"app-anywhere", "arkascended")
	if why == "" {
		t.Fatal("a replacement went ahead naming a play it would not have replaced")
	}
	for _, say := range []string{"discord/focus/settings", "arkascended/focus/settings"} {
		if !strings.Contains(why, say) {
			t.Errorf("the refusal does not name %q, so nobody can see what "+
				"happened:\n%s", say, why)
		}
	}
	gameNow, gameDemoNow := kept(t, inGame, "arkascended", "settings")
	unchanged(t, "the play the save would have written", gameWas, gameDemo,
		gameNow, gameDemoNow)
	otherNow, otherDemoNow := kept(t, elsewhere, "discord", "settings")
	unchanged(t, "the play that was named", otherWas, otherDemo, otherNow,
		otherDemoNow)

	// ── AND ONE THAT HAS GONE SINCE IT WAS CHOSEN ──
	if err := os.Remove(inGame.reg().Path(routes.Route{App: "arkascended",
		Slug: "settings", Focus: true})); err != nil {
		t.Fatal(err)
	}
	gone := again.ReplaceDraft("arkascended/focus/settings", at.Source, at.Recording,
		"app-anywhere", "arkascended")
	if gone == "" {
		t.Fatal("a replacement of a play that is no longer there went ahead " +
			"as a replacement")
	}
	if !strings.Contains(gone, "no play") {
		t.Errorf("the refusal does not say it has gone:\n%s", gone)
	}
}

// AND EVERY OTHER CALLER STILL OVERWRITES, BECAUSE THAT IS THEIR CONTRACT.
//
// Teaching a name again is how the overlay and `marco learn` correct one. The protection above is
// this window's policy and belongs to this window; pushing it into the orchestrator would change
// what every other caller was written against, which is a product change nobody asked for.
func TestTheOrchestratorStillOverwritesForEverybodyElse(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	d := orchestrator.Deps{Reg: reg, Out: &strings.Builder{}}
	press := func(x, y int) macroir.Step {
		return macroir.Step{Kind: macroir.StepClick, Button: "left", X: x, Y: y,
			WinRel: true, RelX: x - 120, RelY: y - 64,
			RelTo: macroir.RelToWindow, WinW: 1920, WinH: 1080, Dpi: 144,
			Process: "arkascended.exe"}
	}
	draft := func(x, y int) *orchestrator.Draft {
		return &orchestrator.Draft{Name: "ark graphics", App: "arkascended",
			Steps: []macroir.Step{press(x, y)}}
	}
	rt, err := d.Save(draft(842, 274), orchestrator.ScopeAppAnywhere, "arkascended")
	if err != nil {
		t.Fatal(err)
	}
	was, err := os.ReadFile(reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Save(draft(500, 400), orchestrator.ScopeAppAnywhere,
		"arkascended"); err != nil {
		t.Fatalf("the orchestrator refused to save over a play, which is the "+
			"behaviour every other caller was written against: %v", err)
	}
	now, err := os.ReadFile(reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(was, now) {
		t.Error("the orchestrator left the first play in place; teaching a name " +
			"again is how the overlay and the CLI correct one")
	}
}
