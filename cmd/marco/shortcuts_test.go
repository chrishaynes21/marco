package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// S2 AT THE DOOR: the terminal's (and the widget's) bind, unbind, reassign, list and press, entered
// through the production functions with a synthetic library and a stated foreground.

// answering makes every yes/no question get these answers, in order.
func answering(t *testing.T, answers ...string) {
	t.Helper()
	prev := askLine
	askLine = func() string {
		if len(answers) == 0 {
			return ""
		}
		a := answers[0]
		answers = answers[1:]
		return a
	}
	t.Cleanup(func() { askLine = prev })
}

// noRecording points the recording fact at an empty home of the test's own.
func noRecording(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	prev := recordingHome
	recordingHome = func() string { return home }
	t.Cleanup(func() { recordingHome = prev })
	return home
}

func press(t *testing.T, d orchestrator.Deps, app, key string) string {
	t.Helper()
	said, err := captureStdout(t, func() error { pressHotkey(d, app, key); return nil })
	if err != nil {
		t.Fatal(err)
	}
	return said
}

func bindOK(t *testing.T, d orchestrator.Deps, fg, listenIn, key, cmd string) string {
	t.Helper()
	var out bytes.Buffer
	if err := bindKey(d, fg, listenIn, key, cmd, &out); err != nil {
		t.Fatalf("binding `%s %q: %v\n%s", key, cmd, err, out.String())
	}
	return out.String()
}

// The namesake test AT THE DOOR: a key bound to the global save runs the global save in an
// application that has its own save. The in-place namesake's source does not compile, so if it ran
// the outcome would be a failure rather than performed.
func TestAPressedShortcutIsTheRoutineItWasAssigned(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	if err := d.Reg.Save(routes.Route{Slug: "save"}, runnableSrc); err != nil {
		t.Fatal(err)
	}
	bindOK(t, d, "discord", "", "s", "save")
	if err := d.Reg.Save(routes.Route{App: "paint", Slug: "save"}, intakeSrc); err != nil {
		t.Fatal(err)
	}
	said := press(t, d, "paint", "s")
	if !strings.Contains(said, "[result] performed") {
		t.Fatalf("`s in paint did not perform the global save it was bound to:\n%s", said)
	}
}

// Taking a key that is in use asks, and Enter means no.
func TestBindAsksBeforeReplacingAndEnterMeansNo(t *testing.T) {
	d, _ := hotkeyWorld(t)
	bindOK(t, d, "discord", "", "d", "open dad s settings")
	answering(t, "") // Enter
	out := bindOK(t, d, "discord", "", "d", "open mouse settings")
	if !strings.Contains(out, "already") && !strings.Contains(out, "runs") {
		t.Errorf("it did not say what is on the key:\n%s", out)
	}
	if b, _ := d.Reg.BindingAt("", "d"); !strings.Contains(b.Cmd, "dad") {
		t.Fatalf("Enter replaced the shortcut: %+v", b)
	}
	answering(t, "y")
	bindOK(t, d, "discord", "", "d", "open mouse settings")
	if b, _ := d.Reg.BindingAt("", "d"); !strings.Contains(b.Cmd, "mouse") {
		t.Fatalf("an explicit yes did not replace it: %+v", b)
	}
}

// The terminal's bind is listened for in the foreground, or where it is told.
func TestBindIsListenedForWhereItWasTyped(t *testing.T) {
	d, _ := hotkeyWorld(t)
	bindOK(t, d, "discord", "discord", "o", "open mouse settings")
	if _, ok := d.Reg.BindingAt("discord", "o"); !ok {
		t.Error("a bind typed in discord is not listened for in discord")
	}
	bindOK(t, d, "discord", "", "g", "open mouse settings")
	if _, ok := d.Reg.BindingAt("", "g"); !ok {
		t.Error("--everywhere did not bind everywhere")
	}
}

// A shortcut set up by name is reassigned only after it says which routine and somebody agrees.
func TestReassignShowsTheRoutineAndAsks(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	if err := d.Reg.Bind("discord", "e", "open mouse settings"); err != nil { // the old shape
		t.Fatal(err)
	}
	if said := press(t, d, "discord", "e"); strings.Contains(said, "[route]") ||
		!strings.Contains(said, "reassign") {
		t.Fatalf("a legacy binding ran, or did not say to reassign it:\n%s", said)
	}
	var out bytes.Buffer
	answering(t, "") // Enter is no
	if err := reassignKey(d, "discord", "discord", "e", &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "settings/focus/open-mouse-settings") {
		t.Errorf("the question does not name the play:\n%s", out.String())
	}
	if b, _ := d.Reg.BindingAt("discord", "e"); !b.Legacy() {
		t.Fatal("Enter reassigned it")
	}
	answering(t, "y")
	out.Reset()
	if err := reassignKey(d, "discord", "discord", "e", &out); err != nil {
		t.Fatal(err)
	}
	b, _ := d.Reg.BindingAt("discord", "e")
	if b.Legacy() || b.Cmd != "open mouse settings" {
		t.Fatalf("reassigned to %+v", b)
	}
}

func TestUnbindSaysWhatItRemoved(t *testing.T) {
	d, _ := hotkeyWorld(t)
	bindOK(t, d, "discord", "discord", "o", "open mouse settings")
	var out bytes.Buffer
	if err := unbindKey(d, "discord", "o", &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "it ran open mouse settings") {
		t.Errorf("unbind does not say what it removed: %s", out.String())
	}
	if _, ok := d.Reg.BindingAt("discord", "o"); ok {
		t.Error("the shortcut is still there")
	}
}

// Forgetting a routine removes its shortcuts, through the forget every terminal command uses.
//
// Deleting the ForgetShortcutsOf call in forgetPlay must fail this.
func TestForgettingARoutineRemovesItsShortcutsAtTheDoor(t *testing.T) {
	d, _ := hotkeyWorld(t)
	bindOK(t, d, "discord", "discord", "o", "open mouse settings")
	rt := routes.Route{App: "settings", Focus: true, Slug: "open-mouse-settings"}
	if err := forgetPlay(d.Reg, rt); err != nil {
		t.Fatal(err)
	}
	if bs := d.Reg.Bindings(); len(bs) != 0 {
		t.Errorf("forgetting the play left its shortcuts: %+v", bs)
	}
}

// What the list says a key does here is what pressing it here fires.
func TestTheBindingsListIsWhatAPressWouldFire(t *testing.T) {
	d, _ := hotkeyWorld(t)
	bindOK(t, d, "discord", "", "d", "open dad s settings")
	bindOK(t, d, "discord", "discord", "d", "open mouse settings")
	bindOK(t, d, "discord", "", "g", "open mouse settings")
	var out bytes.Buffer
	listShortcuts(d, "discord", true, false, true, &out)
	var got struct {
		App       string                        `json:"app"`
		Shortcuts []orchestrator.ListedShortcut `json:"shortcuts"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if len(got.Shortcuts) != 2 {
		t.Fatalf("listed %+v, want `d (discord's) and `g", got.Shortcuts)
	}
	for _, row := range got.Shortcuts {
		p := d.ShortcutPressed("discord", row.Key)
		if strings.Join(row.Routes, ",") != strings.Join(p.Binding.Routes, ",") {
			t.Errorf("the list says `%s runs %v, a press fires %v", row.Key, row.Routes,
				p.Binding.Routes)
		}
	}
}

// A press asks no model and no Director: nothing on the shortcut path interprets words.
func TestAShortcutNeedsNoModel(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	prev := submitPhrase
	submitPhrase = func(string, bool) int {
		t.Fatal("a shortcut press was handed to Director to interpret")
		return 0
	}
	t.Cleanup(func() { submitPhrase = prev })
	if err := d.Reg.Save(routes.Route{Slug: "open-dad-s-settings"}, runnableSrc); err != nil {
		t.Fatal(err)
	}
	bindOK(t, d, "discord", "", "d", "open dad s settings")
	press(t, d, "discord", "d")
}

// No production code writes a name-only binding: every surface assigns through the owner.
//
// `routes.Registry.Bind` writes the legacy shape for tests that need a file as an older build
// wrote it. A production caller would create shortcuts that never run.
func TestNoProductionCodeWritesALegacyBinding(t *testing.T) {
	root := filepath.Join("..", "..")
	var found []string
	_ = filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if e.IsDir() && (e.Name() == ".git" || e.Name() == "node_modules" ||
			e.Name() == ".tmp") {
			return filepath.SkipDir
		}
		if e.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") ||
			strings.HasSuffix(filepath.ToSlash(p), "internal/routes/bindings.go") {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for _, call := range []string{"Reg.Bind(", "reg.Bind("} {
			if strings.Contains(string(raw), call) {
				found = append(found, p+": "+call)
			}
		}
		return nil
	})
	if len(found) > 0 {
		t.Errorf("production code writes name-only shortcuts, which never run:\n%s",
			strings.Join(found, "\n"))
	}
}

// The terminal's constructor announces its recordings in the shared home.
//
// Deleting `RecordingHome: marcopaths.Home()` in newDeps must fail this.
func TestTheTerminalAnnouncesItsRecordings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	if got := newDeps().RecordingHome; got != home {
		t.Errorf("newDeps announces recordings in %q, want the Marco home %q", got, home)
	}
}
