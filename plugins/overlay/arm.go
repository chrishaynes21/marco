package main

// THE WIDGET EXPLAINS ITS KEYS (S4 of docs/Surfaces-alignment-proposal-2026-11-02.md).
//
// When the leader is pressed, the HUD lists what each key does with the application in front, and
// what each key does is decided by the engine, not here: the list is `marco bindings --here
// --armed-in=<app> --json` (orchestrator.ListShortcuts), and a key then runs `marco hotkey`
// (ShortcutPressed). This file only words the answer. cmd/marco's
// TestTheArmListIsWhatAPressWouldFire holds the two engine answers together key by key.
//
// # Off the hook thread, and never a gate
//
// The hook pushes an action and returns (CLAUDE.md: a hook callback must return fast). The action
// loop then starts the listing on a goroutine of its own, because the loop is also what handles
// the NEXT key, and a person who knows their key presses it at once. The list is help: the arm
// still lasts armWindow, the key works whether or not the list has arrived, and a list that
// arrives after the leader has gone is dropped (model.armSeq).
//
// # One application per press of the leader
//
// The foreground is read once, at the leader, and the list and the key are both for that
// application: the list is asked for it, and the key carries it to the engine, which refuses if
// something else is in front when it reads (startArm, hotkey.go's carryPress, cmd/marco
// pressArmed). Two readings, one each, were each right and together ran a routine nobody was shown.

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// armWindow is how long the leader stays armed with no key after it. Two seconds, unchanged by
// the list (proposal §5.8: the list is help, never a gate, and does not stretch the window).
const armWindow = 2 * time.Second

// armListWithin bounds the listing child. It is never longer than the arm itself: a list that
// cannot arrive while the leader is armed has nobody to be shown to. A variable so a test can
// shorten it.
var armListWithin = armWindow

// frontApp is the application in front, named by the engine's own function, so the name the
// leader carries is the name the engine's press reads. A variable so a test can state it.
var frontApp = winctx.Active

// armedFlag carries the application a leader was pressed in to `marco bindings` and `marco hotkey`.
const armedFlag = "--armed-in="

// leaderShown is the leader as the HUD shows it: the key the widget is listening for.
func leaderShown() string { return widgetconfig.Named(cfgLeader()) }

// keyShown is a key after the leader as the HUD shows it ("`e", "CapsLock then e"), the same
// form Studio and the CLI write a shortcut in.
func keyShown(key string) string { return widgetconfig.Pressed(cfgLeader(), key) }

// listedShortcut is one row of `marco bindings --json` (orchestrator.ListedShortcut), decoded
// into the fields this surface words. The engine is a separate module's binary, so the fields
// are the wire, not a shared type.
type listedShortcut struct {
	Key      string `json:"key"`
	ListenIn string `json:"listen_in"`
	Cmd      string `json:"cmd"`
	State    string `json:"state"`
	Says     string `json:"says"`
}

// armListing is the whole of `marco bindings --here --json`.
type armListing struct {
	App       string           `json:"app"`
	Held      string           `json:"held"`
	Shortcuts []listedShortcut `json:"shortcuts"`
}

// fetchArmList asks the engine what each key does in app, the application the leader was pressed
// in, bounded and reaped. why is the sentence
// when there is no answer to show; it is never "none".
func fetchArmList(app string) (l armListing, why string) {
	ctx, cancel := context.WithTimeout(context.Background(), armListWithin)
	defer cancel()
	out, err := exec.CommandContext(ctx, marcoBin(), "bindings", "--here", armedFlag+app, "--json").Output()
	if ctx.Err() != nil {
		return armListing{}, "the shortcuts did not arrive within " + armListWithin.String()
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		return armListing{}, "the shortcuts could not be listed: " + marcoBin() +
			" answered nothing (" + errText(err) + ")"
	}
	if err := json.Unmarshal(out, &l); err != nil {
		return armListing{}, "the shortcuts could not be read: " + err.Error()
	}
	return l, ""
}

// startArm begins a press of the leader: it reads which application is in front, shows the leader
// and starts the list of that application's keys, and returns at once. Called from the action
// loop, never from the hook: reading the foreground is a few system calls, and the hook does none.
//
// THE SESSION IS BOUND TO THAT APPLICATION. The list is of it, and the key after the leader carries
// it (model.endArm, carryPress) to the engine, which refuses if something else is in front by the
// time it reads (cmd/marco pressArmed). The key does not wait for the list to do that.
//
// Moving the fetch onto the caller must fail TestTheArmListIsFetchedOffTheActionLoop.
func startArm(h *model) {
	app := frontApp()
	seq := h.beginArm(app)
	h.setLeaderEcho(leaderShown())
	go func() {
		l, why := fetchArmList(app)
		h.setArmList(seq, armLines(l, why))
	}()
}

// sendable is whether the widget can send key after the leader at all: the controller passes
// on one letter or digit (vkToRune), and keeps m and h for its command line and help. A
// shortcut on any other key is real, and pressing the leader here can never reach it.
func sendable(key string) bool {
	if key == "m" || key == "h" {
		return false
	}
	r := []rune(key)
	return len(r) == 1 && (r[0] >= 'a' && r[0] <= 'z' || r[0] >= '0' && r[0] <= '9')
}

// armLines words a listing for the HUD. Nothing here decides what a key does: each row is the
// engine's, and only the marks are this surface's words for the engine's states.
func armLines(l armListing, why string) []string {
	head := leaderShown() + "  in " + l.App
	if l.App == "" {
		head = leaderShown() + "  nothing in front could be read"
	}
	if why != "" {
		return []string{leaderShown(), why, armFooter}
	}
	lines := []string{head}
	if l.Held != "" {
		lines = append(lines, l.Held)
	}
	for _, s := range l.Shortcuts {
		row := s.Key + "  " + s.Cmd
		if s.ListenIn == "" {
			row += "  (everywhere)"
		}
		switch s.State {
		case "ok":
		case "needs-reassigning":
			row += "  · set up by name: reassign it to run it"
		case "stale":
			row += "  · its routine is gone: change or remove it"
		default: // "wider", and any state added later: the engine's own sentence
			row += "  · " + s.Says
		}
		if !sendable(s.Key) {
			row += "  · the widget cannot send this key"
		}
		lines = append(lines, row)
	}
	if len(l.Shortcuts) == 0 {
		if l.App == "" {
			lines = append(lines, "no shortcuts here")
		} else {
			lines = append(lines, "no shortcuts in "+l.App)
		}
	}
	return append(lines, armFooter)
}

// armFooter is the keys the widget keeps for itself after the leader.
const armFooter = "m command line · h help · Esc closes"
