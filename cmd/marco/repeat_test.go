package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

var (
	socialRoute = routes.Route{Slug: "open-social"}
	typeRoute   = routes.Route{App: "notepad", Focus: true, Slug: "notepad-type"}
)

// repeatWorld is intakeWorld with `open social`, which runs and presses nothing, and `notepad
// type`, which takes one value.
func repeatWorld(t *testing.T) orchestrator.Deps {
	t.Helper()
	d := intakeWorld(t)
	noDirector(t)
	noStopKey(t)
	if err := d.Reg.Save(socialRoute, runnableSrc); err != nil {
		t.Fatal(err)
	}
	if err := d.Reg.Save(typeRoute, runnableSrc+"// {{text}}\n"); err != nil {
		t.Fatal(err)
	}
	return d
}

// stdoutOf runs fn and returns what it printed.
func stdoutOf(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	func() {
		defer func() { os.Stdout = saved }()
		fn()
	}()
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// noStopKey runs each play without the stop key's global hook, which a test must never install.
func noStopKey(t *testing.T) {
	t.Helper()
	prev := stopper
	stopper = func(_ bool, fn func(context.Context) error) error { return fn(context.Background()) }
	t.Cleanup(func() { stopper = prev })
}

// countingSteps replaces one run of a repeat and counts the runs.
func countingSteps(t *testing.T, each func(i int, d orchestrator.Deps, p playInvocation) (Outcome, error)) *int {
	t.Helper()
	n := 0
	prev := performStep
	performStep = func(d orchestrator.Deps, p playInvocation) (Outcome, error) {
		n++
		return each(n, d, p)
	}
	t.Cleanup(func() { performStep = prev })
	return &n
}

func lastRan(out string) string {
	ran := ""
	for _, l := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimRight(l, "\r"), outcome.RanPrefix); ok {
			ran = v
		}
	}
	return ran
}

func repeatOf(rt routes.Route, times int) invoke.Request {
	r := rt
	return invoke.Request{Play: &r, App: rt.App, Times: times, Source: invoke.SourceTyped}
}

func TestTheRepeatFormIsRead(t *testing.T) {
	for _, c := range []struct {
		line     string
		inner    string
		times    int
		explicit bool
		ok       bool
	}{
		{"repeat open social five times", "open social", 5, true, true},
		{"Repeat open social 5 times", "open social", 5, true, true},
		{"open social twenty five times", "open social", 25, false, true},
		{"repeat open social twice", "open social", 2, true, true},
		{"repeat open social once", "open social", 1, true, true},
		{"repeat open social one time", "open social", 1, true, true},
		{"repeat open social a hundred times", "open social", 100, true, true},
		{"repeat open social 500 times", "open social", 500, true, true}, // read, then refused
		{"repeat open social times", "", 0, false, false},
		{"repeat open social five", "", 0, false, false},
		{"repeat five times", "", 0, false, false},
		{"repeat open social 0 times", "", 0, false, false},
		{"repeat open social five time", "", 0, false, false},
		{"open social", "", 0, false, false},
	} {
		inner, times, explicit, ok := parseRepeat(c.line)
		if ok != c.ok || (ok && (inner != c.inner || times != c.times || explicit != c.explicit)) {
			t.Errorf("%q: read as %q ×%d explicit=%v ok=%v", c.line, inner, times, explicit, ok)
		}
	}
}

// THE COMMAND LINE READS A REPEAT BACK WITH ITS COUNT, through the routine's own reading.
func TestARepeatIsReadBackWithItsCount(t *testing.T) {
	d := repeatWorld(t)
	for _, c := range []struct {
		line, app string
		times     int
		readback  string
		value     string
		runnable  bool
	}{
		{"repeat open social five times", "", 5, "Open social ×5", "", true},
		{"open social 3 times", "", 3, "Open social ×3", "", true},
		{"repeat notepad type hello 3 times", "", 3, "Notepad type · text: hello ×3", "hello", true},
		// WITHOUT "repeat", A ROUTINE THAT TAKES THE WHOLE LINE AS ITS VALUE KEEPS IT.
		{"notepad type I said it 3 times", "", 0, "Notepad type · text: I said it 3 times", "I said it 3 times", true},
		{"repeat open social 500 times", "", 500, "", "", false},
	} {
		r := readLine(d, c.app, c.line, lineProposal{})
		if r.Kind != "routine" || r.Times != c.times || r.Readback != c.readback ||
			r.Values["text"] != c.value || r.Runnable != c.runnable {
			t.Errorf("%q: read as kind=%s ×%d %q value=%q runnable=%v (%s)", c.line, r.Kind, r.Times,
				r.Readback, r.Values["text"], r.Runnable, r.Says)
		}
	}
	// AN EXACT NAME WINS: a routine called "open social 3 times" is that routine, once.
	exact := routes.Route{Slug: "open-social-3-times"}
	if err := d.Reg.Save(exact, runnableSrc); err != nil {
		t.Fatal(err)
	}
	if r := readLine(d, "", "open social 3 times", lineProposal{}); r.Times != 0 || r.Slug != exact.Slug {
		t.Errorf("a routine's exact name was read as a repeat: %+v", r)
	}
	// AND A ROUTINE THAT TAKES TWO VALUES IS NOT GUESSED AT INSIDE A REPEAT EITHER.
	two := routes.Route{App: "notepad", Focus: true, Slug: "notepad-write"}
	if err := d.Reg.Save(two, runnableSrc+"// {{title}} {{body}}\n"); err != nil {
		t.Fatal(err)
	}
	if r := readLine(d, "", "repeat notepad write hello there 2 times", lineProposal{}); r.Runnable {
		t.Errorf("words after a two-value routine ran inside a repeat: %+v", r)
	}
}

// A CONFIRMED REPEAT RUNS THE ROUTINE THAT MANY TIMES, each the ordinary run, and says so.
func TestARepeatRunsTheRoutineThatManyTimes(t *testing.T) {
	d := repeatWorld(t)
	n := countingSteps(t, func(_ int, d orchestrator.Deps, p playInvocation) (Outcome, error) {
		return performOnePlay(d, p)
	})
	var out Outcome
	said := stdoutOf(t, func() { out, _ = runInvocation(d, repeatOf(socialRoute, 3)) })
	if out != OutcomePerformed || *n != 3 {
		t.Fatalf("a repeat of 3 ran %d time(s) and ended %q:\n%s", *n, out, said)
	}
	if got := strings.Count(said, "[route] "); got != 3 {
		t.Errorf("%d ordinary runs announced themselves, want 3:\n%s", got, said)
	}
	if got := lastRan(said); got != "open social ×3" {
		t.Errorf("the last [ran] line is %q, want %q", got, "open social ×3")
	}
	if !strings.Contains(said, "[repeat 2/3] Open social") {
		t.Errorf("no progress line for the second run:\n%s", said)
	}
}

// AND EACH RUN IS CHECKED: a save between two runs refuses the next, as it refuses a single run.
func TestEachRunOfARepeatIsCheckedAgain(t *testing.T) {
	d := repeatWorld(t)
	n := countingSteps(t, func(i int, d orchestrator.Deps, p playInvocation) (Outcome, error) {
		out, err := performOnePlay(d, p)
		if i == 2 {
			if err := d.Reg.Save(socialRoute, runnableSrc+"// changed\n"); err != nil {
				t.Fatal(err)
			}
		}
		return out, err
	})
	req := repeatOf(socialRoute, 4)
	req.Expect = routes.DigestOf(runnableSrc)
	var out Outcome
	said := stdoutOf(t, func() { out, _ = runInvocation(d, req) })
	if out != OutcomeRefused || *n != 3 {
		t.Fatalf("a repeat whose routine was saved after run 2 ended %q after %d run(s):\n%s", out, *n, said)
	}
	if got := lastRan(said); got != "open social ×2 of 4" {
		t.Errorf("the last [ran] line is %q, want %q", got, "open social ×2 of 4")
	}
}

// THE FIRST RUN THAT DOES NOT PERFORM ENDS IT, with that run's outcome and how many completed.
func TestARepeatStopsAtTheFirstRunThatDoesNotPerform(t *testing.T) {
	d := repeatWorld(t)
	n := countingSteps(t, func(i int, _ orchestrator.Deps, _ playInvocation) (Outcome, error) {
		if i == 3 {
			return OutcomeFailed, nil
		}
		return OutcomePerformed, nil
	})
	var out Outcome
	said := stdoutOf(t, func() { out, _ = runInvocation(d, repeatOf(socialRoute, 5)) })
	if out != OutcomeFailed || *n != 3 {
		t.Fatalf("a repeat whose third run failed ended %q after %d run(s)", out, *n)
	}
	if got := lastRan(said); got != "open social ×2 of 5" {
		t.Errorf("the last [ran] line is %q, want %q", got, "open social ×2 of 5")
	}
	if !strings.Contains(said, "stopped after 2 of 5") {
		t.Errorf("it did not say how many completed:\n%s", said)
	}
}

// A STOP BETWEEN TWO RUNS ENDS IT: no run is watching for it then.
func TestAStopBetweenRunsEndsTheRepeat(t *testing.T) {
	d := repeatWorld(t)
	n := countingSteps(t, func(i int, _ orchestrator.Deps, _ playInvocation) (Outcome, error) {
		if i == 2 {
			// The run has finished and the stop lands before the next one begins.
			if err := stopsignal.Raise(stopsignal.Home()); err != nil {
				t.Fatal(err)
			}
		}
		return OutcomePerformed, nil
	})
	var out Outcome
	said := stdoutOf(t, func() { out, _ = runInvocation(d, repeatOf(socialRoute, 5)) })
	if out != OutcomeCancelled || *n != 2 {
		t.Fatalf("a repeat stopped after run 2 ended %q after %d run(s)", out, *n)
	}
	if got := lastRan(said); got != "open social ×2 of 5" {
		t.Errorf("the last [ran] line is %q", got)
	}
}

// WORDS IN THE REPEAT FORM ARE PROPOSED AND NEVER RUN, whether Marco reads the routine's name
// itself or Director's resolver names it from a description.
func TestARepeatInWordsIsProposedNotRun(t *testing.T) {
	d := repeatWorld(t)
	n := countingSteps(t, func(int, orchestrator.Deps, playInvocation) (Outcome, error) {
		return OutcomePerformed, nil
	})
	asked := ""
	prev := proposeRoutine
	proposeRoutine = func(p string) (string, string, bool) {
		asked = p
		return `{"answer":{"kind":"run","id":"open-social","slug":"open-social","name":"open social"}}`,
			"open social", true
	}
	t.Cleanup(func() { proposeRoutine = prev })

	for _, c := range []struct{ said, asked string }{
		{"repeat open social five times", ""},
		{"repeat notepad type hello 2 times", ""},
		{"open the social menu five times", "open the social menu"},
	} {
		asked = ""
		var out Outcome
		said := stdoutOf(t, func() {
			out, _ = runInvocation(d, invoke.Request{Text: c.said, Source: invoke.SourceSpoken})
		})
		if out != OutcomeClarify || *n != 0 || strings.Contains(said, "[route] ") {
			t.Fatalf("%q ended %q with %d run(s):\n%s", c.said, out, *n, said)
		}
		if asked != c.asked {
			t.Errorf("%q: the resolver was asked about %q, want %q", c.said, asked, c.asked)
		}
		var p struct {
			Answer struct {
				Slug   string `json:"slug"`
				Values []struct {
					Name, Value string
				} `json:"values"`
			} `json:"answer"`
			Times int `json:"times"`
		}
		line := ""
		for _, l := range strings.Split(said, "\n") {
			if v, ok := strings.CutPrefix(strings.TrimRight(l, "\r"), outcome.ProposedPrefix); ok {
				line = v
			}
		}
		if json.Unmarshal([]byte(line), &p) != nil || p.Times == 0 || p.Answer.Slug == "" {
			t.Errorf("%q: proposed %q", c.said, line)
		}
		if strings.Contains(c.said, "hello") &&
			(len(p.Answer.Values) != 1 || p.Answer.Values[0].Value != "hello") {
			t.Errorf("%q: the value was not proposed with it: %q", c.said, line)
		}
	}

	// AND A COUNT PAST THE LIMIT IS REFUSED, not run once.
	var out Outcome
	_ = stdoutOf(t, func() {
		out, _ = runInvocation(d, invoke.Request{Text: "repeat open social 500 times"})
	})
	if out != OutcomeRefused || *n != 0 {
		t.Errorf("500 times ended %q with %d run(s)", out, *n)
	}
}

// A COUNT IS ONLY EVER FOR A ROUTINE: words for Director are not run N times.
func TestARepeatIsOnlyOfARoutine(t *testing.T) {
	d := repeatWorld(t)
	submitted := 0
	prev := submitPhrase
	submitPhrase = func(string, bool) int { submitted++; return outcome.ExitPerformed }
	t.Cleanup(func() { submitPhrase = prev })
	var out Outcome
	_ = stdoutOf(t, func() {
		out, _ = runInvocation(d, invoke.Request{Text: "make the coffee", Times: 3})
	})
	if out != OutcomeRefused || submitted != 0 {
		t.Errorf("a count on words for Director ended %q and reached it %d time(s)", out, submitted)
	}
}

func TestTheTimesFlagIsACount(t *testing.T) {
	req, _, _, err := parseDoArgs([]string{"--play=open-social", "--times=5"})
	if err != nil || req.Times != 5 || req.Text != "" {
		t.Errorf("--times=5 read as %d (%v), phrase %q", req.Times, err, req.Text)
	}
	for _, bad := range []string{"--times=0", "--times=101", "--times=five-ish", "--times="} {
		if _, _, _, err := parseDoArgs([]string{"--play=open-social", bad}); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
}
