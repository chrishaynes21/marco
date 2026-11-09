//go:build windows

package main

// A HOTKEY WITHOUT THE LEADER, entered through the production hook function `handleKey` with a
// table and a window in front that the test states. See direct.go and ADR-341.

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recordingnow"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

const (
	vk1  = 0x31
	vk2  = 0x32
	vkF9 = 0x78
)

// aGameInFront states a table for window 42, the game, and that window 42 is in front.
func aGameInFront(t *testing.T, keys ...string) {
	t.Helper()
	was, front := directNow.Load(), foregroundHandle
	t.Cleanup(func() {
		directNow.Store(was)
		foregroundHandle = front
		heldCtrl, heldAlt, heldShift, heldWin = false, false, false, false
		caughtVK = map[uint16]bool{}
		disarm()
		drainActions()
	})
	m := map[string]bool{}
	for _, k := range keys {
		m[k] = true
	}
	directNow.Store(&directTable{hwnd: 42, app: "game", keys: m})
	foregroundHandle = func() uintptr { return 42 }
	drainActions()
}

func drainActions() {
	for {
		select {
		case <-actCh:
		default:
			return
		}
	}
}

// nextAction is the next action the hook pushed, if one came.
func nextAction() (action, bool) {
	select {
	case a := <-actCh:
		return a, true
	case <-time.After(100 * time.Millisecond):
		return action{}, false
	}
}

// CTRL+1 BOUND WITHOUT THE LEADER IS CAUGHT, ASKS FOR ITS PLAY ONCE, and its repeats and release
// are caught with it so the game never sees half a key. Ctrl itself reaches the game.
func TestAHotkeyWithoutTheLeaderIsCaughtAndAsksForItsPlayOnce(t *testing.T) {
	aGameInFront(t, "ctrl+1")
	if handleKey(vkCtrl, true) {
		t.Fatal("Ctrl on its own was kept from the game")
	}
	if !handleKey(vk1, true) {
		t.Fatal("ctrl+1 reached the game")
	}
	a, ok := nextAction()
	if !ok || a.kind != actDirect || a.hot != "direct:ctrl+1" || a.app != "game" {
		t.Fatalf("the hook asked for %+v (%v)", a, ok)
	}
	if !handleKey(vk1, true) || !handleKey(vk1, false) {
		t.Error("the repeat or the release of a caught key reached the game")
	}
	if a, ok := nextAction(); ok {
		t.Errorf("a repeat asked for the play again: %+v", a)
	}
	if handleKey(vkCtrl, false) {
		t.Error("the release of Ctrl was kept from the game")
	}
	// AND THE NEXT 1 WITHOUT CTRL IS THE GAME'S.
	if handleKey(vk1, true) || handleKey(vk1, false) {
		t.Error("1 without Ctrl was caught")
	}
}

// NOTHING IS CAUGHT IN A WINDOW THE TABLE WAS NOT READ FOR: the instant after a switch, the key
// belongs to whatever is in front now.
func TestAHotkeyIsNotCaughtInAnotherWindow(t *testing.T) {
	aGameInFront(t, "f9")
	foregroundHandle = func() uintptr { return 43 }
	if handleKey(vkF9, true) {
		t.Fatal("F9 was caught in a window its table was not read for")
	}
	if a, ok := nextAction(); ok {
		t.Errorf("it asked for %+v", a)
	}
}

// A KEY NOTHING IS BOUND TO, AND EVERY KEY WITH NO TABLE AT ALL (while somebody records), REACHES
// THE GAME.
func TestAnUnboundKeyAndAMissingTableCatchNothing(t *testing.T) {
	aGameInFront(t, "f9")
	if handleKey(vk2, true) {
		t.Error("an unbound key was caught")
	}
	directNow.Store(nil)
	if handleKey(vkF9, true) {
		t.Error("F9 was caught with no table")
	}
}

// AFTER THE LEADER THE NEXT KEY IS THE LEADER'S, even one bound on its own: `f9 is a different
// shortcut from F9.
func TestAfterTheLeaderTheKeyIsTheLeaders(t *testing.T) {
	aGameInFront(t, "f9")
	arm()
	handleKey(vkF9, true)
	a, ok := nextAction()
	if ok && a.kind == actDirect {
		t.Fatalf("the key after the leader ran the hotkey bound without it: %+v", a)
	}
}

// THE ACTION LOOP SENDS IT THROUGH THE LEADER'S DOOR, carrying the application it was caught for,
// so the engine refuses if something else is in front by the time it runs.
func TestTheActionLoopCarriesTheApplicationItWasCaughtFor(t *testing.T) {
	ch := make(chan action, 1)
	var got []event
	ch <- action{kind: actDirect, hot: "direct:f9", app: "game"}
	close(ch)
	processFrom(ch, &model{}, func(e event) { got = append(got, e) })
	if len(got) != 1 || got[0].Feed != "Hotkeys" || got[0].Data != "direct:f9" {
		t.Fatalf("the loop emitted %+v", got)
	}
	if app, ok := takeCarried("direct:f9"); !ok || app != "game" {
		t.Errorf("the press carries %q (%v), want game", app, ok)
	}
}

// THE HOOK SPELLS A COMBO THE WAY A SHORTCUT IS STORED, whatever order somebody typed it in, so a
// bound combo is a combo the hook can match.
func TestTheHookSpellsAComboAsItIsStored(t *testing.T) {
	for _, typed := range []string{"Shift + Ctrl + F9", "ctrl+shift+f9", "Win+Alt+Ctrl+Shift+F9"} {
		c := macroir.CanonicalChord(typed)
		ctrl, alt, shift, win := false, false, false, false
		for _, p := range []string{"ctrl", "alt", "shift", "win"} {
			for _, q := range splitPlus(c) {
				if q == p {
					switch p {
					case "ctrl":
						ctrl = true
					case "alt":
						alt = true
					case "shift":
						shift = true
					case "win":
						win = true
					}
				}
			}
		}
		if got := directCombo(ctrl, alt, shift, win, "f9"); got != c {
			t.Errorf("%q is stored as %q and the hook spells it %q", typed, c, got)
		}
	}
}

func splitPlus(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '+' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}

// A TABLE HOLDS THE APPLICATION'S OWN HOTKEYS AND EVERYWHERE'S, and no other application's, and
// no key pressed after the leader.
func TestATableHoldsWhatAPressHereCouldRun(t *testing.T) {
	bs := []routes.Binding{
		{App: "game", Key: "direct:f9"}, {App: "", Key: "direct:ctrl+1"},
		{App: "notepad", Key: "direct:f8"}, {App: "game", Key: "f7"},
	}
	got := directKeysFor(bs, "game")
	if len(got) != 2 || !got["f9"] || !got["ctrl+1"] {
		t.Errorf("the table for the game is %v", got)
	}
}

// WHILE SOMEBODY IS RECORDING THERE IS NO TABLE, so a bound key reaches the recording rather than
// being caught out of it.
func TestThereIsNoTableWhileSomebodyIsRecording(t *testing.T) {
	home, lib := t.TempDir(), t.TempDir()
	t.Setenv("MARCO_HOME", home)
	reg := routes.Registry{Dir: lib}
	if err := reg.UpdateBindings(time.Second, func([]routes.Binding) ([]routes.Binding, error) {
		return []routes.Binding{{Key: "direct:ctrl+1", Cmd: "copy", Routes: []string{"global/copy"}}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	front := foregroundHandle
	t.Cleanup(func() { foregroundHandle = front })
	foregroundHandle = func() uintptr { return 42 }
	var seen time.Time
	var bs []routes.Binding
	var valid bool
	if readDirect(reg, &seen, &bs, &valid) == nil {
		t.Fatal("with nobody recording, a hotkey bound everywhere has no table")
	}
	m, err := recordingnow.Mark(home, "a test recording")
	if err != nil {
		t.Fatal(err)
	}
	defer recordingnow.Clear(m)
	if tb := readDirect(reg, &seen, &bs, &valid); tb != nil {
		t.Errorf("while somebody records, the hook would catch %v", tb.keys)
	}
}
