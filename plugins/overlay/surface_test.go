package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// What must stay true of the always-visible surface itself: that a word opens what it names,
// and that the HUD never says something happened before it knows.

// TestUiOpensAViewAndEditOpensAPlay pins the split.
//
// Both words used to reach `marco edit "<argument>"`, so the most obvious thing a person can
// type — `ui plays`, naming the view with the word the product now uses for what it lists —
// answered "No play named \"plays\"" and then reported success anyway. The two words take
// two different argument spaces and the dispatch has to know which is which.
//
// The engine's view vocabulary is NOT copied here. This asserts the shape of the request
// (`marco ui <view>` versus `marco edit <play>`), which is what the overlay owns; which
// views exist is cmd/marco's uiView and is still growing.
func TestUiOpensAViewAndEditOpensAPlay(t *testing.T) {
	for _, tc := range []struct {
		typed string
		want  []string
	}{
		{"ui", []string{"ui"}},
		{"ui plays", []string{"ui", "plays"}},
		{"ui settings", []string{"ui", "settings"}},                 // a view another session is adding
		{"edit open the inbox", []string{"edit", "open the inbox"}}, // a play, with spaces
		{"edit", []string{"ui"}},                                    // bare `edit` is the front door
	} {
		t.Run(tc.typed, func(t *testing.T) {
			got := argvOfNextUI(t)
			if res := dispatch(newModel(), request{Action: "Run", Input: tc.typed}); res.Status != "ok" {
				t.Fatalf("%q was refused: %+v", tc.typed, res)
			}
			args := got()
			if len(args) != len(tc.want) {
				t.Fatalf("%q spawned marco %q, want %q", tc.typed, args, tc.want)
			}
			for i := range args {
				if args[i] != tc.want[i] {
					t.Fatalf("%q spawned marco %q, want %q", tc.typed, args, tc.want)
				}
			}
		})
	}
}

// TestTheHudDoesNotClaimTheBrowserOpened is the honesty half of the same defect.
//
// "opened in your browser" was logged straight after cmd.Start(), which reports only that a
// process was created. Every real failure happens after that — a view or a play that does
// not exist exits 1 with a message the person never sees — so the one surface whose job is
// to say what is happening was the thing lying about it.
func TestTheHudDoesNotClaimTheBrowserOpened(t *testing.T) {
	// A child that STARTS and then stops: the shape of `marco ui <a view that is not one>`
	// and `marco edit <a play that is not there>`, both of which exit almost at once with a
	// message on a stderr nobody is reading. This is the case the old code got wrong — the
	// process existed, so it reported success.
	t.Run("a child that starts and then stops is reported as a failure", func(t *testing.T) {
		prev := uiStartupWindow
		uiStartupWindow = 5 * time.Second
		t.Cleanup(func() { uiStartupWindow = prev })
		t.Setenv("MARCO_BIN", os.Args[0])

		h := newModel()
		// The helper skips itself when its environment variable is absent, so this is a
		// real child that starts, does nothing and exits inside the window.
		launchUI(h, []string{"-test.run=TestOverlaySleeperHelper"}, "the control centre")
		waitForLog(t, h, "could not open")
		if logs := strings.Join(h.snapshot().logs, "\n"); strings.Contains(logs, "open in your browser") {
			t.Errorf("the HUD claimed the browser opened:\n%s", logs)
		}
	})

	// And a binary that is not there at all still says so, in the same words.
	t.Run("a child that cannot start is reported as a failure", func(t *testing.T) {
		noMarco(t)
		h := newModel()
		launchUI(h, []string{"ui"}, "the control centre")
		waitForLog(t, h, "could not open")
	})

	// A child that stays up IS the success condition: it is a local web server, and there is
	// nothing else for the overlay to wait for — it cannot see the browser.
	t.Run("a server that stays up is reported once it has", func(t *testing.T) {
		prev := uiStartupWindow
		uiStartupWindow = 50 * time.Millisecond
		t.Cleanup(func() { uiStartupWindow = prev })
		t.Setenv("MARCO_BIN", os.Args[0])
		t.Setenv("MARCO_OVERLAY_SLEEPER", "1")

		h := newModel()
		launchUI(h, []string{"-test.run=TestOverlaySleeperHelper"}, "the control centre")
		t.Cleanup(func() {
			editMu.Lock()
			if editCmd != nil && editCmd.Process != nil {
				_ = editCmd.Process.Kill()
			}
			editMu.Unlock()
		})

		// Before the window closes it must not have claimed anything.
		if logs := strings.Join(h.snapshot().logs, "\n"); strings.Contains(logs, "in your browser") {
			t.Errorf("success was claimed before it could be known:\n%s", logs)
		}
		waitForLog(t, h, "open in your browser")
	})
}

// TestTheStatusLineNeverNamesTheDirector holds item 5 of the same idea: the always-visible
// surface speaks the product's language.
//
// It said "director: open the settings" and "director asked — answer it", both on the status
// line of a window that is on screen all the time. A person who has learned one play is not
// required to know this product contains a thing called a Director, and naming it is the
// whole of what those two lines taught them.
func TestTheStatusLineNeverNamesTheDirector(t *testing.T) {
	lines := []string{
		"heard: open the settings",
		"CLARIFICATION_REQUIRED: which Save did you mean?",
		"CLARIFICATION_REQUIRED",
		"[3/5] looking at the screen",
	}
	for _, in := range lines {
		st, ok := directorStatusLine(in)
		if !ok {
			t.Errorf("%q is no longer recognised, so it would not reach the status line", in)
			continue
		}
		for _, backstage := range []string{"director", "Director", "rehears", "candidate", "session"} {
			if strings.Contains(st, backstage) {
				t.Errorf("%q became %q, which names a backstage concept", in, st)
			}
		}
	}
	// The heard line says what was heard, and says it as a quotation — the status line is one
	// line of running prose and an unquoted phrase runs into the words around it.
	if st, _ := directorStatusLine("heard: open the settings"); !strings.Contains(st, `"open the settings"`) {
		t.Errorf("the heard line no longer shows what was heard: %q", st)
	}
	// A question colours the panel as WAITING, never as an error: nothing has gone wrong.
	// The status text and that colour agree through one constant, so rewording the sentence
	// cannot silently turn a pending question grey.
	h := newModel()
	st, _ := directorStatusLine("CLARIFICATION_REQUIRED: which one?")
	h.setStatus(st)
	if got := h.snapshot().state; got != "listen" {
		t.Errorf("a question put the panel in state %q, not the waiting colour", got)
	}
}

// waitForLog waits for a line containing want to reach the HUD log.
//
// The claims under test are deliberately made LATE — after a child has proved it survived,
// or proved it did not — so there is nothing to poll for except the words themselves.
func waitForLog(t *testing.T, h *model, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(strings.Join(h.snapshot().logs, "\n"), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the HUD never said %q; it said %q", want, h.snapshot().logs)
}

// argvOfNextUI captures the arguments the next control-centre spawn is given, without
// starting anything.
//
// The request is built by the production path — the act handler, dispatch, startUI, launchUI
// — because the defect this file is about was correct code reached by the wrong route:
// `startUI` existed, took a view, and was only ever called with "help", while every typed
// argument went to `marco edit` instead. Nothing about the words was wrong; the wiring was.
func argvOfNextUI(t *testing.T) func() []string {
	t.Helper()
	noMarco(t)
	got := make(chan []string, 4)
	prev := startUIChild
	startUIChild = func(cmd *exec.Cmd) error {
		got <- cmd.Args[1:] // Args[0] is the binary
		return errors.New("not started: this test never spawns marco")
	}
	t.Cleanup(func() { startUIChild = prev })
	return func() []string {
		t.Helper()
		select {
		case args := <-got:
			return args
		case <-time.After(5 * time.Second):
			t.Fatal("nothing was asked to open the control centre")
			return nil
		}
	}
}
