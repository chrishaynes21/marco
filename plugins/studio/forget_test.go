package main

// BEING RID OF THINGS, WHICH THIS SURFACE COULD NOT DO.
//
// It could list demonstrations, Acts and runs and remove none of them, so a machine accumulated
// everything anybody had ever tried on it — 443 run records on the owner's, and ten
// demonstrations of two games. Their ask was not per-card tidying:
//
//	I want to be able to start fresh with the testing whenever I'd like
//
// Three sizes, one promise: the first press removes nothing and says what would go, and the
// second does it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// aSceneAndAnActOverIt saves a demonstration and composes an Act from its first interaction,
// through the production path, and returns the Act's name.
func aSceneAndAnActOverIt(t *testing.T, a *Acts) string {
	t.Helper()
	ds := a.Demonstrations()
	if len(ds) == 0 {
		t.Fatal("the fixture store has no demonstration to compose from")
	}
	from := ds[0].Name
	var pick string
	for _, s := range ds[0].Steps {
		if s.Pressable {
			pick = "0=the first interaction"
			_ = s
			break
		}
	}
	if pick == "" {
		t.Skip("the fixture demonstration presses nothing, so no Act can be made")
	}
	c := a.Compose(from, "a disposable act", []string{pick})
	if c.Error != "" {
		t.Fatalf("composing the fixture Act: %s", c.Error)
	}
	return c.Act
}

// DELETING A SCENE TAKES THE ACTS BUILT FROM IT, and says so before it does.
//
// An Act IS its demonstration's interactions by name: its preflight rebuilds the play from the
// bindings and the artifact, so one whose demonstration has gone says `ready` and cannot run.
// That is the dangling reference deleting a store by hand produces.
func TestDeletingASceneTakesTheActsBuiltFromIt(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	act := aSceneAndAnActOverIt(t, a)
	scene := a.Demonstrations()[0].Name
	// THE EVIDENCE OF HOW IT WAS DEMONSTRATED, written as the recorder leaves it. The
	// fixture store has none, so without this the assertion below passes whether or
	// not anything removes it — which a mutation run proved.
	evidence := filepath.Join(a.home, "evidence", sceneFile(scene))
	if err := os.MkdirAll(filepath.Dir(evidence), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidence, []byte(`{"version":1,"name":"`+scene+`"}`),
		0o644); err != nil {
		t.Fatal(err)
	}

	asked := a.ForgetScene(scene, false)
	if asked.Error != "" {
		t.Fatalf("asking was refused: %s", asked.Error)
	}
	if !strings.Contains(asked.Asks, act) {
		t.Errorf("the question does not name the Act that would go with it: %q",
			asked.Asks)
	}
	if len(a.Demonstrations()) == 0 || len(a.Saved()) == 0 {
		t.Fatal("the first press removed something")
	}

	if done := a.ForgetScene(scene, true); done.Error != "" || done.Deleted < 2 {
		t.Fatalf("deleted %d thing(s): %s", done.Deleted, done.Error)
	}
	for _, d := range a.Demonstrations() {
		if d.Name == scene {
			t.Error("the demonstration is still listed")
		}
	}
	if saved := a.Saved(); len(saved) != 0 {
		t.Errorf("%d Act(s) survived the demonstration they perform: %+v",
			len(saved), saved)
	}
	// AND ITS EVIDENCE WENT WITH IT. Filed beside the artifact and read by PASSING the
	// artifact, so once that is gone nothing can match it: a record of somebody's own
	// desktop — where they pressed, what was read, which window — that nothing reaches
	// and nothing cleans up.
	if _, err := os.Stat(evidence); err == nil {
		t.Error("the evidence of a deleted demonstration is still on disk")
	}
}

// sceneFile is the file a demonstration of this name is saved under, by the store's own rule.
func sceneFile(name string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		}
		return '_'
	}, name)
	return safe + ".json"
}

// AND AN ACT CAN GO ON ITS OWN, leaving what was shown Marco where it is.
func TestDeletingAnActLeavesTheDemonstrationAlone(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	act := aSceneAndAnActOverIt(t, a)
	scene := a.Demonstrations()[0].Name

	asked := a.ForgetAct(act, false)
	if !strings.Contains(asked.Asks, "NOT touched") {
		t.Errorf("the question does not say the demonstration survives: %q",
			asked.Asks)
	}
	if done := a.ForgetAct(act, true); done.Error != "" || done.Deleted != 1 {
		t.Fatalf("deleted %d: %s", done.Deleted, done.Error)
	}
	if len(a.Saved()) != 0 {
		t.Error("the Act is still listed")
	}
	found := false
	for _, d := range a.Demonstrations() {
		found = found || d.Name == scene
	}
	if !found {
		t.Error("deleting an Act took the demonstration it was composed from")
	}
	// AND THE NAME IS FREE AGAIN, which it is not if the bindings were left behind:
	// `demonstrated.Save` refuses a name whose `.scenes.json` is still there.
	if c := a.Compose(scene, act, []string{"0=the first interaction"}); c.Error != "" {
		t.Errorf("the name could not be used again after deleting it: %s", c.Error)
	}
}

// AND STARTING FRESH TAKES ALL THREE KINDS, counted before it does.
func TestStartingFreshRemovesEverythingAndSaysWhatThatIs(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	aSceneAndAnActOverIt(t, a)
	// A HOTKEY, so the claim that these survive is a claim about something.
	bindings := filepath.Join(a.routesDir, "bindings.json")
	if err := os.MkdirAll(a.routesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bindings,
		[]byte(`[{"app":"rocketleague","key":"e","cmd":"enter freeplay"}]`),
		0o644); err != nil {
		t.Fatal(err)
	}
	aRecordOf(t, a, "run-20261021-131859-b456", `{"version":1,
	  "run":"run-20261021-131859-b456","outcome":"refused",
	  "started":"2026-10-21T13:18:59-07:00","ended":"2026-10-21T13:19:39-07:00"}`)

	asked := a.ForgetEverything(false)
	if asked.Error != "" {
		t.Fatalf("asking was refused: %s", asked.Error)
	}
	for _, want := range []string{"demonstration", "Act", "run record", "hotkeys"} {
		if !strings.Contains(asked.Asks, want) {
			t.Errorf("the question says nothing about %s: %q", want, asked.Asks)
		}
	}
	if len(a.Demonstrations()) == 0 || len(a.Saved()) == 0 || len(a.History()) == 0 {
		t.Fatal("the first press removed something")
	}

	if done := a.ForgetEverything(true); done.Error != "" {
		t.Fatalf("starting fresh: %s", done.Error)
	}
	if d, s, h := a.Demonstrations(), a.Saved(), a.History(); len(d) != 0 ||
		len(s) != 0 || len(h) != 0 {
		t.Errorf("after starting fresh: %d demonstration(s), %d Act(s), %d run(s)",
			len(d), len(s), len(h))
	}
	// AND THE HOTKEYS ARE KEPT, because a key bound to a name works again the moment
	// that name is demonstrated again — throwing it away is a thing to redo rather
	// than a thing to clean up.
	kept, err := os.ReadFile(bindings)
	if err != nil {
		t.Fatalf("starting fresh took the hotkeys with it: %v", err)
	}
	if !strings.Contains(string(kept), "enter freeplay") {
		t.Errorf("the hotkeys were rewritten: %s", kept)
	}
}

// AND NOTHING GOES WHILE SOMETHING IS IN FLIGHT.
func TestNothingIsDeletedWhileSomethingIsRunning(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	act := aSceneAndAnActOverIt(t, a)
	scene := a.Demonstrations()[0].Name

	a.mu.Lock()
	a.busy, a.act = true, act
	a.mu.Unlock()

	for what, got := range map[string]Forgotten{
		"the Act that is running": a.ForgetAct(act, true),
		"the Scene under it":      a.ForgetScene(scene, true),
		"everything":              a.ForgetEverything(true),
	} {
		if got.Deleted != 0 || got.Error == "" {
			t.Errorf("%s: deleted %d and said %q", what, got.Deleted, got.Error)
		}
	}
	if len(a.Demonstrations()) == 0 || len(a.Saved()) == 0 {
		t.Error("something went while a run was in flight")
	}
}
