package main

// THE TWO VERBS THAT MAKE A DEMONSTRATION CORRECTABLE ARE REACHABLE.
//
// Structural, for the reason `capture-game-session`'s test is: eight recorded cases in this
// repository of complete, correct code that production never invoked. A correction path nothing
// dispatches to would be the ninth, and from a file listing it would look exactly like a
// working feature.
//
// They are tested together because they are one capability from a person's side: read what you
// did, then change what Marco made of it.

import (
	"os"
	"strings"
	"testing"
)

func TestTheCorrectionAndRawRecordVerbsAreReachable(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading the dispatch: %v", err)
	}
	s := string(src)
	for _, w := range []struct{ verb, call string }{
		{`case "correct":`, "runCorrect(os.Args[2:])"},
		{`case "what-i-did":`, "runWhatIDid(os.Args[2:])"},
	} {
		if !strings.Contains(s, w.verb) {
			t.Errorf("main.go dispatches no %s verb", strings.Trim(w.verb, `case ":`))
		}
		if !strings.Contains(s, w.call) {
			t.Errorf("the verb does not call %s", w.call)
		}
	}
	// AND BOTH ARE IN THE USAGE, because a verb a person cannot discover is a verb for
	// whoever read the source.
	for _, w := range []string{"director correct ", "director what-i-did "} {
		if !strings.Contains(s, w) {
			t.Errorf("the usage does not mention %q", w)
		}
	}
}

// AND NEITHER OF THEM CAN ACT.
//
// `correct` edits a saved procedure and `what-i-did` prints a record; both are reached from a
// terminal while an application may be in front, and neither has any business touching an
// actuator. Asserted over their text rather than promised, the same way the observing commands
// assert that they write nothing.
func TestTheCorrectionAndRawRecordVerbsCannotAct(t *testing.T) {
	for _, file := range []string{"correctcmd.go", "whatidid.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		s := string(src)
		for _, forbidden := range []string{
			"marcoexec", "oshost", "internal/driver", "internal/runtime",
			"os/exec", "winctx", "navsource.New", "Perform", "Click(",
		} {
			if strings.Contains(s, forbidden) {
				t.Errorf("%s names %q; neither verb may reach anything that "+
					"presses, types or activates", file, forbidden)
			}
		}
	}
	// AND THE RAW RECORD IS READ-ONLY. A command that printed the demonstration and
	// could also rewrite it would be the one place a correction could falsify it.
	src, err := os.ReadFile("whatidid.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"SaveEvidence", "SaveSequence", "os.WriteFile"} {
		if strings.Contains(string(src), forbidden) {
			t.Errorf("what-i-did names %q and must only read", forbidden)
		}
	}
	// AND THE CORRECTION WRITES THE PROCEDURE AND NOT THE RECORD.
	src, err = os.ReadFile("correctcmd.go")
	if err != nil {
		t.Fatal(err)
	}
	c := string(src)
	if !strings.Contains(c, "store.SaveSequence(out)") {
		t.Error("the correction does not save the corrected procedure")
	}
	if strings.Contains(c, "SaveEvidence") {
		t.Error("the correction writes the raw demonstration, which it must never do: " +
			"a correction changes what Marco learned and not what the person did")
	}
}
