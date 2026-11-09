//go:build windows

package main

// Windows-only because the leader's key code and the arming timer live in the Windows controller.

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

// The leader every surface shows is the key the hook listens for: for every spelling the settings
// accept, and for the ones they do not (which leave the hook on the backtick).
//
// A widgetconfig.Named that showed a setting as typed must fail this.
func TestTheLeaderIsShownAsTheKeyItListensFor(t *testing.T) {
	named := map[uint16]string{0xC0: "`", 0x14: "CapsLock", 0x09: "Tab", 0xDC: "\\", 0xBF: "/", 0xDB: "["}
	for c := 'a'; c <= 'z'; c++ {
		named[uint16(c-'a'+'A')] = string(c)
	}
	for c := '0'; c <= '9'; c++ {
		named[uint16(c)] = string(c)
	}
	for n := 1; n <= 12; n++ {
		named[uint16(0x70+n-1)] = fmt.Sprintf("F%d", n)
	}
	specs := []string{"`", "backtick", "grave", "tilde", "capslock", "caps", "CapsLock", "tab",
		"\\", "backslash", "/", "slash", "[", "lbracket", "a", "Z", "0", "9", "f1", "F8", "f12",
		"f13", "f0", "", "nonsense", "ctrl"}
	specs = append(specs, leaderNames...)
	for _, spec := range specs {
		want, ok := named[parseLeader(spec)]
		if !ok {
			t.Fatalf("the hook listens for key %#x for %q, which this test cannot name", parseLeader(spec), spec)
		}
		if got := widgetconfig.Named(spec); got != want {
			t.Errorf("a leader set to %q is shown as %q; the hook listens for %q", spec, got, want)
		}
	}
}

// The leader stays armed two seconds.
func TestTheLeaderIsArmedForTwoSeconds(t *testing.T) {
	if leaderTimeout != armWindow {
		t.Errorf("the controller disarms after %s, not the arm window %s", leaderTimeout, armWindow)
	}
}

// THE PRODUCTION ACTION LOOP shows the list after the leader and puts it away with the key after
// it, entered through processFrom with actions of the test's own.
//
// Dropping startArm or either endArm from the loop must fail this.
func TestTheActionLoopShowsTheListAndPutsItAway(t *testing.T) {
	withFront(t, "notepad")
	withLeader(t, "capslock") // not the backtick, so an echo written as one is seen
	marcoAnswers(t, aListing, 0)
	h := &model{}
	var mu sync.Mutex
	var fed []event
	ch := make(chan action, 8)
	go processFrom(ch, h, func(e event) { mu.Lock(); fed = append(fed, e); mu.Unlock() })
	t.Cleanup(func() { close(ch) })

	waitFor := func(what string, ok func(snapshot) bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if ok(h.snapshot()) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("%s never happened: %+v", what, h.snapshot())
	}

	ch <- action{kind: actArm}
	waitFor("the list after the leader", func(s snapshot) bool {
		return s.leaderEcho == "CapsLock" && len(s.arm) > 0 && s.arm[0] == "CapsLock  in notepad"
	})
	ch <- action{kind: actHotkey, r: 'e'}
	waitFor("the key after it putting the list away", func(s snapshot) bool {
		return s.leaderEcho == "CapsLock then e" && len(s.arm) == 0
	})
	// WAITED FOR, LIKE EVERYTHING ELSE HERE: the loop sets the echo and then emits the press, so
	// reading the feed the instant the echo shows raced the emit and failed under load.
	var got string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		mu.Lock()
		got = fmt.Sprint(fed)
		mu.Unlock()
		if strings.Contains(got, "Hotkeys Key e") {
			break
		}
	}
	if !strings.Contains(got, "Hotkeys Key e") {
		t.Errorf("the key was not sent on as a press: %s", got)
	}

	ch <- action{kind: actArm}
	waitFor("the list after the leader again", func(s snapshot) bool { return len(s.arm) > 0 })
	ch <- action{kind: actBeginEdit}
	waitFor("the command line putting the list away", func(s snapshot) bool {
		return s.editing && len(s.arm) == 0
	})
}

// THE PRODUCTION ACTION LOOP binds the key to the leader's application: the leader pressed in
// discord, then notepad to the front, then the key. The list is of discord, and the press carries
// discord, so the engine reads notepad, refuses, and notepad's own binding is never selected
// (cmd/marco TestAKeyPressedAfterTheFocusMovedRunsNothing).
//
// Carrying nothing from the loop, or reading the foreground at the key, must fail this.
func TestAKeyIsForTheApplicationItsLeaderWasPressedIn(t *testing.T) {
	withLeader(t, "`")
	front := withFront(t, "discord")
	asked := marcoAnswers(t, `{"app":"{armed}","shortcuts":[]}`, 0)
	h := &model{}
	keys := make(chan event, 4)
	ch := make(chan action, 8)
	go processFrom(ch, h, func(e event) { keys <- e })
	t.Cleanup(func() { close(ch) })

	ch <- action{kind: actArm}
	deadline := time.Now().Add(10 * time.Second)
	for s := h.snapshot(); len(s.arm) == 0 || s.arm[0] != "`  in discord"; s = h.snapshot() {
		if time.Now().After(deadline) {
			t.Fatalf("discord's list never arrived: %+v", s.arm)
		}
		time.Sleep(10 * time.Millisecond)
	}
	*front = "notepad"
	ch <- action{kind: actHotkey, r: 'e'}
	var e event
	select {
	case e = <-keys:
	case <-time.After(10 * time.Second):
		t.Fatal("the key was never sent on")
	}
	dispatch(h, request{Action: "Hotkey", Input: e.Data.(string)})
	got := asked()
	if !strings.Contains(got, "bindings --here --armed-in=discord --json") ||
		!strings.Contains(got, "hotkey e --armed-in=discord") {
		t.Errorf("the list and the key were not both for discord:\n%s", got)
	}
}
