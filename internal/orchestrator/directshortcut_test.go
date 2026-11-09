package orchestrator

// A HOTKEY WITHOUT THE LEADER (ADR-341): a key of its own beside the leader's, spelled the way the
// widget sees it pressed, and refused on the keys it would take away from somebody.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// F9 ON ITS OWN AND `f9 AFTER THE LEADER ARE TWO SHORTCUTS: each press runs its own play, and
// neither is found by the other's press.
func TestAHotkeyWithoutTheLeaderIsNotTheLeadersKey(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "f9", ListenIn: "game", Steps: []routes.Route{freeplay}})
	assign(t, d, Assignment{Key: routes.DirectKey("f9"), ListenIn: "cloudapp",
		Steps: []routes.Route{cloudPull}})
	if p := d.ShortcutPressed("cloudapp", "direct:f9"); p.Refused != "" || len(p.Steps) != 1 ||
		p.Steps[0].Route != cloudPull {
		t.Fatalf("F9 on its own in cloudapp fires %+v (%s)", p.Steps, p.Refused)
	}
	if p := d.ShortcutPressed("cloudapp", "f9"); p.Found {
		t.Errorf("`f9 after the leader fired the hotkey that is pressed without it: %+v", p)
	}
	if p := d.ShortcutPressed("game", "direct:f9"); p.Found {
		t.Errorf("F9 on its own fired the leader's `f9: %+v", p)
	}
}

// A COMBO IS STORED THE WAY IT IS PRESSED, however it was typed: "Shift + Ctrl + 1" is the
// `ctrl+shift+1` the widget builds from the keys held.
func TestAHotkeyIsStoredInTheSpellingItIsPressedIn(t *testing.T) {
	d := shortcutWorld(t)
	b := assign(t, d, Assignment{Key: "direct:Shift + Ctrl + 1", Steps: []routes.Route{copyAny}})
	if b.Key != "direct:ctrl+shift+1" {
		t.Fatalf("stored as %q", b.Key)
	}
	if p := d.ShortcutPressed("notepad", "direct:ctrl+shift+1"); !p.Found {
		t.Error("the press the widget would send does not find it")
	}
}

// THE KEYS IT WOULD TAKE AWAY FROM SOMEBODY ARE REFUSED, each saying why.
func TestAHotkeyThatWouldTakeAKeyAwayIsRefused(t *testing.T) {
	d := shortcutWorld(t)
	for _, c := range []struct {
		keys, in, says string
	}{
		{"hyper", "game", "not a key Marco knows"},
		{"shift", "game", "only modifiers"},
		{"ctrl+alt", "", "only modifiers"},
		{"esc", "game", "Esc"},
		{"`", "game", "leader"},
		{"f", "", "types text"},
		{"shift+1", "", "types text"},
		{"num5", "", "types text"},
		{"space", "", "types text"},
	} {
		_, err := d.AssignShortcut(Assignment{Key: routes.DirectKey(c.keys), ListenIn: c.in,
			Steps: []routes.Route{copyAny}, Leader: "`"})
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s %q: %v, want it refused saying %q", c.keys, c.in, err, c.says)
		}
	}
}

// AND THE ONES THAT TAKE NOTHING ARE ALLOWED: a key that types, in one application; anything
// that does not type, or is held with Ctrl, Alt or Win, everywhere; and the widget's own `m and
// `h, which are only the widget's after the leader.
func TestAHotkeyThatTakesNothingIsAllowed(t *testing.T) {
	d := shortcutWorld(t)
	for _, c := range []struct{ keys, in string }{
		{"f", "game"}, {"num5", "game"}, {"f9", ""}, {"ctrl+1", ""}, {"alt+q", ""},
		{"shift+f2", ""}, {"m", "game"}, {"h", "game"},
	} {
		steps := []routes.Route{copyAny}
		if _, err := d.AssignShortcut(Assignment{Key: routes.DirectKey(c.keys), ListenIn: c.in,
			Steps: steps, Leader: "`"}); err != nil {
			t.Errorf("%s %q was refused: %v", c.keys, c.in, err)
		}
	}
}

// ITS SENTENCES NAME IT AS IT IS PRESSED, with no leader in front of it.
func TestAHotkeyIsSaidWithoutTheLeader(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "direct:f9", ListenIn: "game", Steps: []routes.Route{freeplay}})
	st := d.ShortcutAt("game", "direct:f9")
	if !strings.HasPrefix(st.Says, "f9 in game runs") {
		t.Errorf("it is said as %q", st.Says)
	}
	rows := d.ListShortcuts("game", true)
	if len(rows) != 1 || !rows[0].Direct {
		t.Errorf("the listing does not mark it as pressed on its own: %+v", rows)
	}
}
