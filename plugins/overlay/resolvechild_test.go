package main

// THE RESOLVER CHILD IS BOUNDED, AND IT DOES NOT OUTLIVE THE ASK.
//
// # Why this needs a real child
//
// Because the thing that was wrong is not a decision, it is a process. The bound used to be a
// `select` around a goroutine: the wait ended, the caller moved on, and the child kept running —
// holding a pipe nobody would read and, through it, whatever the resolver plugin had started. A
// surface that abandons a process it spawned is the shape of the eight `marco do` processes this
// repository spent a release gate on, and no amount of stubbing `askWhichRoutine` can see it.
//
// So this points `$MARCO_BIN` at a child that hangs and asks the REAL `runResolveChild`: the ask
// must come back, say that it timed out, and leave nothing running.
//
// The child is this test binary re-executed with `$MARCO_OVERLAY_HANG` set, which `TestMain`
// answers before any test runs — because `runResolveChild` builds its own argv (`resolve --json
// <phrase>`) and cannot be told to run one named test.

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMain is the hanging child, when it was spawned as one.
//
// Before any test runs, because the thing spawning it cannot pass `-test.run`. It sleeps past
// every bound this test uses and exits; what the test asserts is that nobody waited for it.
func TestMain(m *testing.M) {
	if os.Getenv("MARCO_OVERLAY_HANG") != "" {
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	// ONE SUBCOMMAND HANGS, or answers late, while the others answer: the list of keys and the
	// press of one are two children, and what is worth holding is the one not waiting for the other.
	if len(os.Args) > 1 && os.Args[1] == os.Getenv("MARCO_OVERLAY_HANG_ON") {
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if s := os.Getenv("MARCO_OVERLAY_SLOW_IF"); s != "" && strings.Contains(strings.Join(os.Args[1:], " "), s) {
		time.Sleep(800 * time.Millisecond)
	}
	// AND THE CHILD THAT ANSWERS, which is the control beside it: one JSON object on
	// stdout and out, which is what the real `marco resolve --json` does.
	if said := os.Getenv("MARCO_OVERLAY_ANSWER"); said != "" {
		// What it was asked, for a test that holds which subcommand a surface spawns.
		// One line per child, appended, since a leader and its key are two.
		if p := os.Getenv("MARCO_OVERLAY_ARGV"); p != "" {
			if f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
				_, _ = f.WriteString(strings.Join(os.Args[1:], " ") + "\n")
				_ = f.Close()
			}
		}
		// {armed} is the application the leader carried, so a list says whose it is.
		for _, a := range os.Args[1:] {
			if v, ok := strings.CutPrefix(a, "--armed-in="); ok {
				said = strings.ReplaceAll(said, "{armed}", v)
			}
		}
		os.Stdout.WriteString(said)
		code, _ := strconv.Atoi(os.Getenv("MARCO_OVERLAY_ANSWER_EXIT"))
		os.Exit(code)
	}
	// NO TEST READS A LINE THROUGH A REAL `marco` unless it says which engine answers: the
	// default is no reading, which is the line going to the intake as words, as before S6.
	askLine = func(string, *lineProposal) (lineReading, bool) { return lineReading{}, false }
	os.Exit(m.Run())
}

func TestTheResolverChildIsBoundedAndReaped(t *testing.T) {
	// THE CHILD IS THIS BINARY, HANGING. Not `marco.exe`: on this surface that performs
	// real input, and a test may never run it.
	t.Setenv("MARCO_BIN", os.Args[0])
	t.Setenv("MARCO_OVERLAY_HANG", "1")
	was := resolveWithin
	resolveWithin = 300 * time.Millisecond
	t.Cleanup(func() { resolveWithin = was })

	began := time.Now()
	r, why := runResolveChild("make the pictures nicer")
	took := time.Since(began)

	// IT CAME BACK, well inside the child's own life. A bound that waited for the child
	// would be no bound at all.
	if took > 5*time.Second {
		t.Fatalf("the ask took %s, so it waited for the child rather than bounding it",
			took)
	}
	if why == "" {
		t.Fatalf("a child that never answered produced an answer: %+v", r)
	}
	// AND IT SAYS WHAT HAPPENED, which is a timeout and not a resolution of nothing: the
	// caller shows this instead of offering to record a routine.
	if !strings.Contains(why, "did not answer") {
		t.Errorf("the timeout is reported as %q", why)
	}
	if !strings.Contains(why, resolveWithin.String()) {
		t.Errorf("the reason does not say how long it waited: %q", why)
	}
	if r.Answer.Kind != "" || r.Answer.ID != "" {
		t.Errorf("a timeout carried an answer: %+v", r.Answer)
	}
}

// AND A CHILD THAT ANSWERS IS READ, which is the control: without it, a `runResolveChild` that
// always timed out would pass everything above.
func TestTheResolverChildsAnswerIsRead(t *testing.T) {
	// A CHILD THAT PRINTS ONE OBJECT AND EXITS, which is what `marco resolve --json`
	// does. The echo helper already in this package is the production streamer's child;
	// this one only has to write to stdout, so it is the same binary with its own flag.
	t.Setenv("MARCO_BIN", os.Args[0])
	t.Setenv("MARCO_OVERLAY_ANSWER",
		`{"answer":{"kind":"run","id":"arkascended/focus/ark-graphics",`+
			`"app":"arkascended","slug":"ark-graphics","focus":true,`+
			`"name":"ark graphics","source":"advisor"}}`)
	t.Cleanup(func() { _ = os.Unsetenv("MARCO_OVERLAY_ANSWER") })

	r, why := runResolveChild("make the pictures nicer")
	if why != "" {
		t.Fatalf("a child that answered was reported as %q", why)
	}
	if r.Answer.ID != "arkascended/focus/ark-graphics" || !r.Answer.Focus {
		t.Errorf("the answer did not survive the child: %+v", r.Answer)
	}
}
