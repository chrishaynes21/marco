package main

import (
	"flag"
	"testing"
	"time"
)

// The valued-flag table, guarded.
//
// This has now failed twice in production, both times SILENTLY: `--window <handle>` had its
// handle become a positional, and `--application chrome --json` printed non-JSON because
// "chrome" was reordered behind "--json" and the flag package read the application as
// "--json". A flag missing from the table does not error — it quietly gets no value and its
// argument becomes a positional, which is the worst kind of CLI bug because the output
// still looks plausible.
//
// So rather than trusting the table, this parses real argument orders and asserts the
// values arrive.

func TestValuedFlagsSurviveEveryArgumentOrder(t *testing.T) {
	orders := [][]string{
		{"--application", "rocketleague", "--duration", "3m", "--interval", "500ms", "--json"},
		{"--json", "--application", "rocketleague", "--duration", "3m", "--interval", "500ms"},
		{"--duration", "3m", "--json", "--application", "rocketleague", "--interval", "500ms"},
		{"--interval", "500ms", "--duration", "3m", "--application", "rocketleague", "--json"},
		{"--json", "--interval", "500ms", "--application", "rocketleague", "--duration", "3m"},
	}
	for _, args := range orders {
		fs := flag.NewFlagSet("observe-game", flag.ContinueOnError)
		application := fs.String("application", "", "")
		duration := fs.Duration("duration", 0, "")
		interval := fs.Duration("interval", 0, "")
		jsonOut := fs.Bool("json", false, "")
		if err := fs.Parse(flagsFirst(args)); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if *application != "rocketleague" {
			t.Errorf("%v: application = %q, want rocketleague", args, *application)
		}
		if *duration != 3*time.Minute {
			t.Errorf("%v: duration = %v, want 3m", args, *duration)
		}
		if *interval != 500*time.Millisecond {
			t.Errorf("%v: interval = %v, want 500ms", args, *interval)
		}
		if !*jsonOut {
			t.Errorf("%v: --json was not set; its neighbour swallowed it", args)
		}
	}
}

func TestEveryValueTakingFlagIsRegistered(t *testing.T) {
	// The omission this catches is invisible at runtime, so it is asserted directly.
	for _, name := range []string{
		"--application", "--window-id", "--window-title", "--process",
		"--duration", "--interval", "--region", "--window", "--app", "--n",
	} {
		if !valued[name] {
			t.Errorf("%s takes a value but is missing from the valued table; its argument "+
				"will be silently treated as a positional", name)
		}
	}
}

func TestABooleanFlagDoesNotSwallowThePositionalAfterIt(t *testing.T) {
	// The mirror-image failure: adding a boolean to the table would eat the id.
	got := flagsFirst([]string{"--json", "observe_1"})
	if len(got) != 2 || got[0] != "--json" || got[1] != "observe_1" {
		t.Fatalf("flagsFirst = %v, want the boolean and the positional kept apart", got)
	}
}
