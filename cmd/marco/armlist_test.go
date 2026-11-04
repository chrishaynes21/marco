package main

// S4 AT THE DOOR: what the widget shows when the leader is pressed is `marco bindings --here
// --json`, and what a key then does is `marco hotkey`. The widget decides neither. So the claim
// worth holding is that the two answer the same thing KEY BY KEY, through the production
// functions: a listed key runs what the list says, a listed refusal refuses with the sentence the
// list showed, a key the list does not show says there is nothing on it, and a global key the list
// hides behind an application's own key runs the application's.

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

type armListing struct {
	App       string                        `json:"app"`
	Held      string                        `json:"held"`
	Shortcuts []orchestrator.ListedShortcut `json:"shortcuts"`
}

func listedHere(t *testing.T, d orchestrator.Deps, app string) armListing {
	t.Helper()
	var out bytes.Buffer
	listShortcuts(d, app, true, false, true, &out)
	var got armListing
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	return got
}

func theLeaderIs(t *testing.T, leader string) {
	t.Helper()
	prev := configuredLeader
	configuredLeader = func() string { return leader }
	t.Cleanup(func() { configuredLeader = prev })
}

func TestTheArmListIsWhatAPressWouldFire(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	theLeaderIs(t, "`")
	bindOK(t, d, "discord", "", "d", "open dad s settings") // hidden in discord by its own `d
	bindOK(t, d, "discord", "discord", "d", "open mouse settings")
	bindOK(t, d, "discord", "", "g", "open mouse settings")
	if err := d.Reg.Bind("discord", "l", "open mouse settings"); err != nil { // set up by name
		t.Fatal(err)
	}
	gone := routes.Route{Slug: "soon-gone"}
	if err := d.Reg.Save(gone, runnableSrc); err != nil {
		t.Fatal(err)
	}
	bindOK(t, d, "discord", "", "s", "soon gone")
	if err := os.Remove(d.Reg.Path(gone)); err != nil {
		t.Fatal(err)
	}

	got := listedHere(t, d, "discord")
	if got.App != "discord" || got.Held != "" {
		t.Fatalf("the listing is for %q, held %q", got.App, got.Held)
	}
	keys := map[string]orchestrator.ListedShortcut{}
	for _, row := range got.Shortcuts {
		if _, twice := keys[row.Key]; twice {
			t.Fatalf("`%s is listed twice here, so the list cannot say which one a press runs", row.Key)
		}
		keys[row.Key] = row
	}
	for _, k := range []string{"d", "g", "l", "s"} {
		if _, ok := keys[k]; !ok {
			t.Fatalf("`%s is not in the list: %+v", k, got.Shortcuts)
		}
	}
	if keys["d"].ListenIn != "discord" {
		t.Errorf("the `d listed is %+v, not discord's own", keys["d"])
	}

	for _, row := range got.Shortcuts {
		said := press(t, d, "discord", row.Key)
		switch row.State {
		case "ok":
			if !strings.Contains(said, "[route] ") || strings.Contains(said, "[result] refused") {
				t.Errorf("`%s is listed as running %q; a press said:\n%s", row.Key, row.Cmd, said)
			}
			p := d.ShortcutPressed("discord", row.Key)
			if strings.Join(p.Binding.Routes, ",") != strings.Join(row.Routes, ",") {
				t.Errorf("the list says `%s runs %v, a press fires %v", row.Key, row.Routes, p.Binding.Routes)
			}
		case "needs-reassigning", "stale":
			if strings.Contains(said, "[route] ") || !strings.Contains(said, "[result] refused") ||
				!strings.Contains(said, row.Says) {
				t.Errorf("`%s is listed as %s (%q); a press said:\n%s", row.Key, row.State, row.Says, said)
			}
		default:
			t.Errorf("`%s is listed in a state this test does not know: %+v", row.Key, row)
		}
	}
	if keys["l"].State != "needs-reassigning" || keys["s"].State != "stale" {
		t.Errorf("the set-up-by-name and gone-routine keys are listed as %q and %q",
			keys["l"].State, keys["s"].State)
	}

	// A KEY THE LIST DOES NOT SHOW says there is nothing on it, and announces no outcome.
	said := press(t, d, "discord", "x")
	if !strings.Contains(said, "nothing on `x in discord") {
		t.Errorf("an unbound key does not say so:\n%q", said)
	}
	if strings.Contains(said, "[result] ") || strings.Contains(said, "[route] ") {
		t.Errorf("an unbound key announced an invocation:\n%s", said)
	}
	// And with nothing in front that could be read, it does not name an application.
	if said := press(t, d, "", "x"); strings.TrimSpace(said) != "nothing on `x" {
		t.Errorf("an unbound key with no application read says %q", said)
	}
}

// The sentence is written with the configured leader, the way the widget shows the key.
func TestNothingOnAKeyIsSaidWithTheConfiguredLeader(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	theLeaderIs(t, "capslock")
	if said := press(t, d, "notepad", "x"); !strings.Contains(said, "nothing on CapsLock then x in notepad") {
		t.Errorf("an unbound key under a CapsLock leader says %q", said)
	}
}

// ── THE KEY IS FOR THE APPLICATION THE LEADER WAS PRESSED IN ──

func pressCarried(t *testing.T, d orchestrator.Deps, front, armedIn, key string) string {
	t.Helper()
	said, err := captureStdout(t, func() error { pressArmed(d, front, armedIn, true, key); return nil })
	if err != nil {
		t.Fatal(err)
	}
	return said
}

// discord's list shows `e for one routine and notepad has `e on another. The leader was pressed in
// discord and notepad came to the front before the key: nothing runs, and notepad's routine above
// all does not, whatever the list it was shown said. Nothing bound in the new foreground refuses
// too, because the press is not for there.
//
// Deleting the comparison in pressArmed must fail this.
func TestAKeyPressedAfterTheFocusMovedRunsNothing(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	theLeaderIs(t, "`")
	bindOK(t, d, "discord", "discord", "e", "open mouse settings")
	bindOK(t, d, "notepad", "notepad", "e", "open dad s settings")

	if l := listedHere(t, d, "discord"); len(l.Shortcuts) != 1 || l.Shortcuts[0].Cmd != "open mouse settings" {
		t.Fatalf("discord's list is %+v", l)
	}
	said := pressCarried(t, d, "notepad", "discord", "e")
	if strings.Contains(said, "[route] ") || !strings.Contains(said, "[result] refused") {
		t.Fatalf("a key pressed for discord with notepad in front ran something:\n%s", said)
	}
	if want := "`e was pressed for discord, and notepad is in front now — nothing was performed; press ` again"; !strings.Contains(said, want) {
		t.Errorf("the refusal says:\n%s\nwant %q", said, want)
	}
	// Into something with no shortcut on that key, and out of nothing that could be read.
	if said := pressCarried(t, d, "paint", "discord", "e"); !strings.Contains(said, "[result] refused") {
		t.Errorf("a press for discord with paint in front:\n%s", said)
	}
	if said := pressCarried(t, d, "notepad", "", "e"); strings.Contains(said, "[route] ") ||
		!strings.Contains(said, "pressed for nothing that could be read, and notepad is in front") {
		t.Errorf("a press armed over nothing readable, with notepad in front:\n%s", said)
	}
}

// The same application still in front runs its own key at once, the list or no list: the list is
// help and the press does not wait for it or read it.
func TestAKeyPressedWhereTheLeaderWasPressedRuns(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	theLeaderIs(t, "`")
	bindOK(t, d, "discord", "discord", "e", "open mouse settings")
	said := pressCarried(t, d, "discord", "discord", "e")
	if !strings.Contains(said, "[route] ") || strings.Contains(said, "[result] refused") {
		t.Errorf("a press in the application its leader was pressed in:\n%s", said)
	}
	if said := pressCarried(t, d, "discord", "discord", "x"); !strings.Contains(said, "nothing on `x in discord") {
		t.Errorf("an unbound key where the leader was pressed:\n%s", said)
	}
}

// What the widget sends is read as what it means: the carried application is the one listed and
// the one a press is checked against, even an empty one (nothing could be read), and a list or a
// press with nothing carried reads the foreground as before.
func TestTheCarriedApplicationIsWhatIsReadFromTheArguments(t *testing.T) {
	front := func() string { return "notepad" }
	if app, here, armed, js, ok := bindingsArgs([]string{"--here", "--armed-in=discord", "--json"}, front); !ok || app != "discord" || !here || !armed || !js {
		t.Errorf("a carried list reads as %q here=%v armed=%v json=%v ok=%v", app, here, armed, js, ok)
	}
	if app, here, armed, _, ok := bindingsArgs([]string{"--armed-in="}, front); !ok || app != "" || !here || !armed {
		t.Errorf("a list carried from nothing readable reads as %q here=%v", app, here)
	}
	if app, _, armed, _, ok := bindingsArgs([]string{"--here", "--json"}, front); !ok || app != "notepad" || armed {
		t.Errorf("an uncarried list here reads as %q", app)
	}
	if app, _, _, _, ok := bindingsArgs(nil, front); !ok || app != "" {
		t.Errorf("the whole list reads as %q", app)
	}
	if _, _, _, _, ok := bindingsArgs([]string{"--nonsense"}, front); ok {
		t.Error("an unknown argument was accepted")
	}
	if k, in, c := hotkeyArgs([]string{"E", "--armed-in=discord"}); k != "e" || in != "discord" || !c {
		t.Errorf("a carried press reads as %q %q %v", k, in, c)
	}
	if k, in, c := hotkeyArgs([]string{"e", "--armed-in="}); k != "e" || in != "" || !c {
		t.Errorf("a press carried from nothing readable reads as %q %q %v", k, in, c)
	}
	if k, _, c := hotkeyArgs([]string{"e"}); k != "e" || c {
		t.Errorf("an uncarried press reads as %q carried=%v", k, c)
	}
}

// A listing for the application a leader was pressed in says so, and an ordinary one does not:
// the echo is what tells the widget this engine checks the press, so it is said only when it is
// true. An empty application (nothing could be read at the leader) is echoed too.
func TestAListingSaysItWasForTheArmedApplication(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	for _, c := range []struct {
		app     string
		carried bool
	}{{"discord", true}, {"", true}, {"discord", false}} {
		var out bytes.Buffer
		listShortcuts(d, c.app, true, c.carried, true, &out)
		var doc map[string]any
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		v, echoed := doc["armed_in"]
		if echoed != c.carried || (echoed && v != c.app) {
			t.Errorf("a listing for %q carried=%v echoes %v (%v)", c.app, c.carried, v, echoed)
		}
	}
}
