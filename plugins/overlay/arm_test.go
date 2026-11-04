package main

// S4: THE WIDGET EXPLAINS ITS KEYS. Entered through the production functions (startArm,
// fetchArmList, dispatch's Hotkey) with this test binary standing in for `marco`: it answers what
// a test tells it to and writes down what it was asked. Nothing here runs marco.exe, which on this
// surface performs real input.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// withLeader states the widget's leader for one test. The package's cfg is read from the person's
// own settings file at start-up, so a test that depends on the leader must not inherit it.
func withLeader(t *testing.T, leader string) {
	t.Helper()
	cfgMu.Lock()
	prev := cfg.Leader
	cfg.Leader = leader
	cfgMu.Unlock()
	t.Cleanup(func() { cfgMu.Lock(); cfg.Leader = prev; cfgMu.Unlock() })
}

// marcoAnswers makes the next spawned `marco` print said and exit with code, and returns a function
// reading back the arguments it was given.
func marcoAnswers(t *testing.T, said string, code int) func() string {
	t.Helper()
	engineIsCurrent(t)
	argv := filepath.Join(t.TempDir(), "argv.txt")
	t.Setenv("MARCO_BIN", os.Args[0])
	t.Setenv("MARCO_OVERLAY_ANSWER", said)
	t.Setenv("MARCO_OVERLAY_ANSWER_EXIT", strconv.Itoa(code))
	t.Setenv("MARCO_OVERLAY_ARGV", argv)
	return func() string {
		b, _ := os.ReadFile(argv)
		return strings.TrimSpace(string(b))
	}
}

const aListing = `{"app":"notepad","shortcuts":[
 {"key":"e","listen_in":"notepad","cmd":"save draft","state":"ok","says":"` + "`" + `e in notepad runs save draft"},
 {"key":"l","listen_in":"notepad","cmd":"open mouse settings","state":"needs-reassigning","says":"set up by name"},
 {"key":"s","listen_in":"","cmd":"soon gone","state":"stale","says":"gone"},
 {"key":"c","listen_in":"","cmd":"copy","state":"ok","says":"runs copy"},
 {"key":"p","listen_in":"","cmd":"save as pdf","state":"wider","says":"\"save as pdf\" works only in acrobat"},
 {"key":"f5","listen_in":"","cmd":"refresh","state":"ok","says":"runs refresh"}]}`

func hasLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

func TestTheArmListIsTheEnginesAnswer(t *testing.T) {
	withLeader(t, "`")
	asked := marcoAnswers(t, aListing, 0)
	l, why := fetchArmList("notepad")
	if why != "" {
		t.Fatalf("the listing was not read: %s", why)
	}
	if got := asked(); got != "bindings --here --armed-in=notepad --json" {
		t.Fatalf("the widget asked %q, not the engine's listing of what fires where the leader was pressed", got)
	}
	lines := armLines(l, why)
	for _, want := range []string{
		"`  in notepad",
		"e  save draft",
		"l  open mouse settings  · set up by name: reassign it to run it",
		"s  soon gone  (everywhere)  · its routine is gone: change or remove it",
		"c  copy  (everywhere)",
		`p  save as pdf  (everywhere)  · "save as pdf" works only in acrobat`,
		"f5  refresh  (everywhere)  · the widget cannot send this key",
		armFooter,
	} {
		if !hasLine(lines, want) {
			t.Errorf("the list has no line %q:\n%s", want, strings.Join(lines, "\n"))
		}
	}
	if len(lines) != 8 {
		t.Errorf("the list is %d lines, want the header, six rows and the footer:\n%s",
			len(lines), strings.Join(lines, "\n"))
	}
}

func TestTheArmListSaysWhatHoldsShortcutsOff(t *testing.T) {
	withLeader(t, "`")
	marcoAnswers(t, `{"app":"notepad","held":"a recording of \"x\" is in progress, so shortcuts are held off until it ends","shortcuts":[]}`, 0)
	lines := armLines(fetchArmList("notepad"))
	if !hasLine(lines, `a recording of "x" is in progress, so shortcuts are held off until it ends`) || !hasLine(lines, "no shortcuts in notepad") {
		t.Errorf("the list does not say shortcuts are held off:\n%s", strings.Join(lines, "\n"))
	}
}

// A listing that could not be had is said as that, never as "no shortcuts".
//
// Reading a failure as an empty list must fail this.
func TestAListThatCouldNotBeHadIsNeverNone(t *testing.T) {
	withLeader(t, "`")
	for _, c := range []struct {
		name, said, want string
	}{
		{"an engine with no such subcommand", "\n", "the shortcuts could not be listed"},
		{"an unreadable answer", "usage: marco ...", "the shortcuts could not be read"},
	} {
		t.Run(c.name, func(t *testing.T) {
			marcoAnswers(t, c.said, 2)
			lines := armLines(fetchArmList("notepad"))
			joined := strings.Join(lines, "\n")
			if !strings.Contains(joined, c.want) || strings.Contains(joined, "no shortcuts") {
				t.Errorf("the list says:\n%s", joined)
			}
		})
	}
	t.Run("no engine at all", func(t *testing.T) {
		noMarco(t)
		joined := strings.Join(armLines(fetchArmList("notepad")), "\n")
		if !strings.Contains(joined, "could not be listed") || strings.Contains(joined, "no shortcuts") {
			t.Errorf("the list says:\n%s", joined)
		}
	})
}

// The fetch is not on the action loop: startArm returns at once with the child still hanging, the
// list says it did not arrive, and the child is bounded.
func TestTheArmListIsFetchedOffTheActionLoop(t *testing.T) {
	withFront(t, "notepad")
	engineIsCurrent(t)
	withLeader(t, "`")
	t.Setenv("MARCO_BIN", os.Args[0])
	t.Setenv("MARCO_OVERLAY_HANG", "1")
	was := armListWithin
	armListWithin = 300 * time.Millisecond
	t.Cleanup(func() { armListWithin = was })

	h := &model{}
	began := time.Now()
	startArm(h)
	if took := time.Since(began); took > 100*time.Millisecond {
		t.Fatalf("startArm took %s, so the listing ran on the loop that handles the next key", took)
	}
	if s := h.snapshot(); s.leaderEcho != "`" || len(s.arm) != 0 {
		t.Fatalf("straight after the leader: echo %q, list %q", s.leaderEcho, s.arm)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if s := h.snapshot(); len(s.arm) > 0 {
			if !strings.Contains(strings.Join(s.arm, "\n"), "did not arrive within 300ms") {
				t.Errorf("a listing that hung says %q", s.arm)
			}
			if time.Since(began) > 5*time.Second {
				t.Errorf("the hung listing was waited for (%s), not bounded", time.Since(began))
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the hung listing never said anything")
}

// A list that arrives after the leader has gone is not drawn, under this press or a later one.
//
// Dropping the press number from setArmList must fail this.
func TestAListForAPressThatHasGoneIsNotShown(t *testing.T) {
	withFront(t, "notepad")
	withLeader(t, "`")
	marcoAnswers(t, aListing, 0)
	h := &model{}
	startArm(h)
	h.dismiss() // the key after it, or the two seconds, came first
	time.Sleep(1500 * time.Millisecond)
	if s := h.snapshot(); len(s.arm) != 0 {
		t.Fatalf("a list arrived after the leader was gone and was drawn: %q", s.arm)
	}

	old := h.beginArm("notepad")
	h.beginArm("notepad") // the leader again
	if h.setArmList(old, []string{"stale"}) {
		t.Fatal("an older press's list was shown under a newer press")
	}
	cur := h.beginArm("notepad")
	h.endArm()
	if h.setArmList(cur, []string{"stale"}) {
		t.Fatal("a list was shown after the key after the leader")
	}
}

// The arm is two seconds, and the list is never waited for longer than the arm.
func TestTheArmStaysTwoSeconds(t *testing.T) {
	if armWindow != 2*time.Second {
		t.Errorf("the leader is armed for %s, not two seconds", armWindow)
	}
	if armListWithin > armWindow {
		t.Errorf("the list is waited for %s, longer than the %s arm", armListWithin, armWindow)
	}
}

// The echo is the configured leader, not a backtick written into the HUD.
func TestTheEchoIsTheConfiguredLeader(t *testing.T) {
	withFront(t, "notepad")
	withLeader(t, "capslock")
	marcoAnswers(t, `{"app":"notepad","shortcuts":[]}`, 0)
	h := &model{}
	startArm(h)
	if s := h.snapshot(); s.leaderEcho != "CapsLock" {
		t.Errorf("the armed echo under a CapsLock leader is %q", s.leaderEcho)
	}
	if got := keyShown("e"); got != "CapsLock then e" {
		t.Errorf("a key after a CapsLock leader is shown as %q", got)
	}
	if got := commandListing()[0]; got != "CapsLock then m then type:" {
		t.Errorf("the help listing starts %q", got)
	}
	if !strings.HasPrefix(idleHint(), "CapsLock then m ") {
		t.Errorf("the idle hint is %q", idleHint())
	}
}

// ── A PRESSED SHORTCUT IS IN THE HISTORY (F9) ──

func lastHistory(t *testing.T, h *model) histEntry {
	t.Helper()
	s := h.snapshot()
	if len(s.history) == 0 {
		t.Fatal("the press left no history row")
	}
	return s.history[len(s.history)-1]
}

func TestAPressedShortcutIsInTheHistory(t *testing.T) {
	withLeader(t, "`")
	for _, c := range []struct {
		name, said    string
		code          int
		row, word, st string
		failed        bool
	}{
		{"performed", "[route] save draft\n[result] performed\n", 0,
			"`e  save draft", "performed", "ran: `e  save draft", false},
		{"refused by the engine", "`e in notepad ran gone, which is no longer in the library\n[result] refused\n", 5,
			"`e — `e in notepad ran gone, which is no longer in the library", "refused",
			"refused: `e — `e in notepad ran gone, which is no longer in the library", true},
		{"nothing on it", "nothing on `e in notepad\n", 0,
			"nothing on `e in notepad", "", "nothing on `e in notepad", false},
		{"an older engine that says nothing", "\n", 0, "nothing on `e", "", "nothing on `e", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			asked := marcoAnswers(t, c.said, c.code)
			h := &model{}
			resp := dispatch(h, request{Action: "Hotkey", Input: "e"})
			if got := asked(); got != "hotkey e" {
				t.Fatalf("the press asked %q, not the engine's press", got)
			}
			e := lastHistory(t, h)
			if e.cmd != c.row || e.outcome != c.word {
				t.Errorf("history row %q / %q, want %q / %q", e.cmd, e.outcome, c.row, c.word)
			}
			if s := h.snapshot(); s.status != c.st {
				t.Errorf("status %q, want %q", s.status, c.st)
			}
			if (resp.Status != "ok") != c.failed {
				t.Errorf("the response is %+v", resp)
			}
		})
	}
}

// A pressed shortcut can still be stopped, and says it was.
//
// Registering the press without cancellation must fail this.
func TestAPressedShortcutCanStillBeStopped(t *testing.T) {
	withLeader(t, "`")
	stopStore(t)
	shortenStopGrace(t, 100*time.Millisecond)
	t.Setenv("MARCO_BIN", os.Args[0])
	t.Setenv("MARCO_OVERLAY_HANG", "1")
	h := &model{}
	done := make(chan struct{})
	go func() { defer close(done); dispatch(h, request{Action: "Hotkey", Input: "e"}) }()
	deadline := time.Now().Add(10 * time.Second)
	for !isRunning() {
		if time.Now().After(deadline) {
			t.Fatal("the press never became something a stop can reach")
		}
		time.Sleep(10 * time.Millisecond)
	}
	stopRunningPlays()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("the stopped press never ended")
	}
	if e := lastHistory(t, h); e.outcome != "cancelled" || !strings.HasPrefix(e.cmd, "`e") {
		t.Errorf("a stopped press is in the history as %q / %q", e.cmd, e.outcome)
	}
}

// ── THE SESSION IS BOUND TO THE APPLICATION ITS LEADER WAS PRESSED IN ──

// withFront states which application is in front, as the leader reads it, and starts and ends the
// test with no carried presses.
func withFront(t *testing.T, app string) *string {
	t.Helper()
	prev := frontApp
	front := app
	frontApp = func() string { return front }
	clear := func() { carried.Lock(); carried.q = nil; carried.Unlock() }
	clear()
	t.Cleanup(func() { frontApp = prev; clear() })
	return &front
}

// A key pressed before its list has arrived runs at once, carrying where its leader was pressed.
// The list here never arrives at all.
func TestAQuickKeyDoesNotWaitForTheList(t *testing.T) {
	withLeader(t, "`")
	withFront(t, "discord")
	asked := marcoAnswers(t, "[route] save draft\n[result] performed\n", 0)
	t.Setenv("MARCO_OVERLAY_HANG_ON", "bindings")
	h := &model{}
	startArm(h)
	carryPress("e", h.endArm()) // the key after it, as the action loop does
	began := time.Now()
	dispatch(h, request{Action: "Hotkey", Input: "e"})
	if took := time.Since(began); took > armListWithin {
		t.Errorf("the press took %s, so it waited for the list", took)
	}
	if e := lastHistory(t, h); e.outcome != "performed" {
		t.Errorf("the quick press is in the history as %q / %q", e.cmd, e.outcome)
	}
	if got := asked(); !strings.Contains(got, "hotkey e --armed-in=discord") {
		t.Errorf("the press did not carry where its leader was pressed:\n%s", got)
	}
}

// The list of one leader that arrives after the next leader is never shown under it, even when the
// two were pressed in different applications and the late one answers last.
//
// Dropping the press number from setArmList must fail this.
func TestALateListIsNotShownUnderALaterLeader(t *testing.T) {
	withLeader(t, "`")
	front := withFront(t, "discord")
	marcoAnswers(t, `{"app":"{armed}","shortcuts":[]}`, 0)
	t.Setenv("MARCO_OVERLAY_SLOW_IF", "--armed-in=discord")
	h := &model{}
	startArm(h) // in discord, answered late
	*front = "notepad"
	startArm(h) // in notepad, answered at once
	deadline := time.Now().Add(3 * time.Second)
	sawNotepad := false
	for time.Now().Before(deadline) {
		s := h.snapshot()
		if len(s.arm) > 0 {
			switch s.arm[0] {
			case "`  in notepad":
				sawNotepad = true
			case "`  in discord":
				t.Fatalf("discord's late list was shown under the leader pressed in notepad: %q", s.arm)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sawNotepad {
		t.Errorf("the current leader's own list never arrived: %q", h.snapshot().arm)
	}
}

// A press no leader carried (a script calling Overlay Hotkey) carries nothing, and a carried press
// that never became a press does not lend its context to a later one.
func TestOnlyALeadersOwnKeyCarriesItsContext(t *testing.T) {
	withFront(t, "discord")
	carryPress("e", "discord")
	carryPress("e", "notepad")
	if app, ok := takeCarried("e"); !ok || app != "discord" {
		t.Errorf("the first press took %q, %v", app, ok)
	}
	if app, ok := takeCarried("e"); !ok || app != "notepad" {
		t.Errorf("the second press took %q, %v", app, ok)
	}
	if _, ok := takeCarried("e"); ok {
		t.Error("a third press took a context nobody carried for it")
	}
	carried.Lock()
	carried.q = append(carried.q, carriedPress{key: "e", app: "discord", at: time.Now().Add(-2 * carryFor)})
	carried.Unlock()
	if app, ok := takeCarried("e"); ok {
		t.Errorf("an expired carried press lent %q to a later one", app)
	}
}

// The list is of the application the leader was pressed in, read once at the leader, and not of
// whatever is in front by the time the listing is asked for.
//
// Reading the foreground again for the listing must fail this.
func TestTheListIsOfTheApplicationTheLeaderWasPressedIn(t *testing.T) {
	withLeader(t, "`")
	withFront(t, "")
	var reads int
	var mu sync.Mutex
	frontApp = func() string { // discord at the leader, notepad on every later reading
		mu.Lock()
		defer mu.Unlock()
		reads++
		if reads == 1 {
			return "discord"
		}
		return "notepad"
	}
	asked := marcoAnswers(t, `{"app":"{armed}","shortcuts":[]}`, 0)
	h := &model{}
	startArm(h)
	deadline := time.Now().Add(5 * time.Second)
	for s := h.snapshot(); len(s.arm) == 0; s = h.snapshot() {
		if time.Now().After(deadline) {
			t.Fatal("the list never arrived")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if s := h.snapshot(); s.arm[0] != "`  in discord" || !strings.Contains(asked(), "--armed-in=discord") {
		t.Errorf("the leader was pressed in discord and the list is %q, asked as %q", s.arm, asked())
	}
	if got := h.endArm(); got != "discord" {
		t.Errorf("the key after it would carry %q", got)
	}
}

// ── NOT THROUGH AN ENGINE THAT WOULD IGNORE THE CHECK ──

// engineIsCurrent states that the engine checks a carried press, for a test about something else.
func engineIsCurrent(t *testing.T) {
	t.Helper()
	prev := contextCheck
	contextCheck = func() (bool, string) { return true, "" }
	t.Cleanup(func() { contextCheck = prev })
}

// theRealEngineCheck puts the production check back after marcoAnswers, with nothing kept.
func theRealEngineCheck(t *testing.T) {
	t.Helper()
	contextCheck = engineChecksContext
	forgetEngineCheck()
	t.Cleanup(forgetEngineCheck)
}

func countLines(s, prefix string) (n int) {
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

// An engine from S2 refuses `--armed-in` and one from before has no `bindings`: with either, a
// key after the leader is not sent at all, and the history says why. The leader's list says it too.
//
// Deleting the check from pressShortcut must fail this.
func TestAPressIsNotSentToAnEngineThatWouldNotCheckIt(t *testing.T) {
	withLeader(t, "`")
	for _, old := range []struct{ name, said string }{
		{"S2", "usage: marco bindings [--here] [--json]\n"},
		{"before S2", "unknown command: bindings\n"},
		{"a listing that does not say it was for the armed application", `{"app":"notepad","shortcuts":[]}`},
	} {
		t.Run(old.name, func(t *testing.T) {
			withFront(t, "discord")
			asked := marcoAnswers(t, old.said, 2)
			theRealEngineCheck(t)
			h := &model{}
			carryPress("e", "discord")
			dispatch(h, request{Action: "Hotkey", Input: "e"})
			if got := asked(); countLines(got, "hotkey") != 0 {
				t.Fatalf("the press was sent to an engine that would not check it:\n%s", got)
			}
			e := lastHistory(t, h)
			if e.outcome != "refused" || !strings.Contains(e.cmd, "older than this widget") {
				t.Errorf("the press is in the history as %q / %q", e.cmd, e.outcome)
			}
			startArm(h)
			deadline := time.Now().Add(5 * time.Second)
			for s := h.snapshot(); len(s.arm) == 0; s = h.snapshot() {
				if time.Now().After(deadline) {
					t.Fatal("the leader's list never said anything")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if s := h.snapshot(); !hasLine(s.arm, "shortcuts are off: "+os.Args[0]+" is older than this widget "+
				"and does not check which application a key was pressed for — rebuild marco beside the widget") {
				t.Errorf("the leader's list is %q", s.arm)
			}
		})
	}
}

// A current engine is asked once for the binary it is, and its presses carry the context. A press
// no leader carried is not gated, because it never had the check.
func TestACurrentEngineIsAskedOnceAndPressed(t *testing.T) {
	withLeader(t, "`")
	withFront(t, "discord")
	asked := marcoAnswers(t, `{"app":"{armed}","armed_in":"{armed}","shortcuts":[]}`, 0)
	theRealEngineCheck(t)
	h := &model{}
	carryPress("e", "discord")
	dispatch(h, request{Action: "Hotkey", Input: "e"})
	carryPress("e", "discord")
	dispatch(h, request{Action: "Hotkey", Input: "e"})
	got := asked()
	if countLines(got, "hotkey e --armed-in=discord") != 2 || countLines(got, "bindings --here --armed-in= --json") != 1 {
		t.Errorf("two presses through a current engine asked:\n%s", got)
	}

	// And an old engine does not stop a press a script made, which carries nothing.
	asked = marcoAnswers(t, "usage: marco bindings [--here] [--json]\n", 2)
	theRealEngineCheck(t)
	dispatch(h, request{Action: "Hotkey", Input: "e"})
	if got := asked(); countLines(got, "hotkey e") != 1 || countLines(got, "bindings") != 0 {
		t.Errorf("an uncarried press asked:\n%s", got)
	}
}

// An engine that does not answer in time is a no for that press, and is asked again.
func TestAnEngineThatDidNotAnswerIsAskedAgain(t *testing.T) {
	withLeader(t, "`")
	withFront(t, "discord")
	marcoAnswers(t, `{"app":"","armed_in":"","shortcuts":[]}`, 0)
	theRealEngineCheck(t)
	was := armListWithin
	armListWithin = 300 * time.Millisecond
	t.Cleanup(func() { armListWithin = was })
	t.Setenv("MARCO_OVERLAY_HANG_ON", "bindings")
	if ok, why := contextCheck(); ok || !strings.Contains(why, "for now") {
		t.Fatalf("an engine that never answered: %v %q", ok, why)
	}
	t.Setenv("MARCO_OVERLAY_HANG_ON", "")
	if ok, why := contextCheck(); !ok {
		t.Errorf("the slow answer was kept, and a current engine is refused: %q", why)
	}
}

// A `marco` rebuilt under a running widget is asked again: the kept answer is keyed by the path,
// size and modification time it was asked under. (Here the "rebuild" is the stand-in's modification time moving.)
func TestARebuiltEngineIsAskedAgain(t *testing.T) {
	marcoAnswers(t, "usage: marco bindings [--here] [--json]\n", 2)
	theRealEngineCheck(t)
	if ok, _ := contextCheck(); ok {
		t.Fatal("an S2 engine was taken as current")
	}
	st, err := os.Stat(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	marcoAnswers(t, `{"app":"","armed_in":"","shortcuts":[]}`, 0)
	contextCheck = engineChecksContext
	later := st.ModTime().Add(time.Second)
	if err := os.Chtimes(os.Args[0], later, later); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chtimes(os.Args[0], st.ModTime(), st.ModTime()) })
	if ok, why := contextCheck(); !ok {
		t.Errorf("the rebuilt engine is still refused: %q", why)
	}
}
