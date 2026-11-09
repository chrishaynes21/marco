package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A HOTKEY BOUND WITHOUT THE LEADER RUNS ITS PLAY AT THE DOOR, the press the widget sends being
// `marco hotkey direct:<keys>`; and `f9 after the leader is not it. See ADR-341.
func TestAHotkeyWithoutTheLeaderRunsItsPlayAtTheDoor(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	if err := d.Reg.Save(routes.Route{Slug: "save"}, runnableSrc); err != nil {
		t.Fatal(err)
	}
	said := bindOK(t, d, "discord", "", routes.DirectKey("Ctrl + 1"), "save")
	if !strings.Contains(said, "bound ctrl+1 ->") {
		t.Errorf("the binding is said as %q", said)
	}
	if said := press(t, d, "paint", "direct:ctrl+1"); !strings.Contains(said, "[result] performed") {
		t.Fatalf("ctrl+1 on its own did not perform the play bound to it:\n%s", said)
	}
	if said := press(t, d, "paint", "ctrl+1"); !strings.Contains(said, "nothing on") {
		t.Errorf("ctrl+1 after the leader ran the hotkey bound without it:\n%s", said)
	}
}

// AND `--direct` IS HOW THE COMMAND LINE SAYS IT, taken off with the other flags in any order.
func TestDirectIsAFlagOfBindAndUnbind(t *testing.T) {
	for _, args := range [][]string{
		{"--direct", "--in=game", "f9", "slide"}, {"--in=game", "--direct", "f9", "slide"},
	} {
		direct, rest := directFlag(args)
		if !direct || strings.Join(rest, " ") != "--in=game f9 slide" {
			t.Errorf("%v is read as direct=%v %v", args, direct, rest)
		}
	}
	if direct, rest := directFlag([]string{"f9", "--direct"}); direct || len(rest) != 2 {
		t.Error("--direct after the key was taken for the flag")
	}
}

// THE LIST AFTER THE LEADER IS OF WHAT THE LEADER RUNS: a hotkey pressed on its own is not a key to
// press next, so the widget's list leaves it out, and the full list keeps it.
func TestTheListAfterTheLeaderLeavesOutAHotkeyWithoutIt(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	if err := d.Reg.Save(routes.Route{Slug: "save"}, runnableSrc); err != nil {
		t.Fatal(err)
	}
	bindOK(t, d, "discord", "", routes.DirectKey("f9"), "save")
	bindOK(t, d, "discord", "", "s", "save")
	var armed, all bytes.Buffer
	listShortcuts(d, "discord", true, true, true, &armed)
	listShortcuts(d, "", false, false, false, &all)
	if strings.Contains(armed.String(), "direct:") || !strings.Contains(armed.String(), `"key":"s"`) {
		t.Errorf("the list after the leader is %s", armed.String())
	}
	if !strings.Contains(all.String(), "f9 ") || strings.Contains(all.String(), "direct:") {
		t.Errorf("the full list is %s", all.String())
	}
}

// AND ITS SENTENCES SAY IT AS IT IS PRESSED: nothing on F8, and a press for an application that
// is no longer in front asks for the key again, not for the leader.
func TestAHotkeyWithoutTheLeaderIsSaidWithoutIt(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	if said := press(t, d, "paint", "direct:f8"); !strings.Contains(said, "nothing on f8 in paint") {
		t.Errorf("an unbound F8 says %q", said)
	}
	moved := contextMoved("game", "notepad", "direct:f9")
	if !strings.HasPrefix(moved, "f9 was pressed for game") || !strings.HasSuffix(moved, "press it again") {
		t.Errorf("a press whose game is no longer in front says %q", moved)
	}
}
