package runref_test

// ONE LINE, WRITTEN BY ONE PROCESS AND READ BY ANOTHER.
//
// The reference exists to be retrieved, and since a front end started spawning the runner it is
// retrieved by a program as well as by a person. Which makes the shape a CONTRACT between two
// binaries: the runner prints it, `marco` reads the outcome out of it and tells somebody what
// happened. There is no compiler between those two, so these are the tests that stand in for one.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
)

// AND THE LINE SURVIVES THE ROUND TRIP, in both the shapes a run prints.
func TestAReferenceComesBackAsWhatWasPrinted(t *testing.T) {
	const id = "run-20261019-201500-abcd"
	for _, tc := range []struct {
		name string
		path string
		out  runref.Outcome
	}{
		{"the opening citation", `C:\home\marco\runs\run-20261019-201500-abcd.json`, ""},
		{"and the closing one", `C:\home\marco\runs\run-20261019-201500-abcd.json`,
			runref.Verified},
		// A PATH WITH A SPACE IN IT, which is an ordinary thing for a Windows home
		// directory to contain and would be cut in half by splitting on whitespace.
		{"a path through somebody's name", `C:\Users\A Person\marco\runs\r.json`,
			runref.Unconfirmed},
		// AND A PATH THAT ENDS IN BRACKETS. Windows produces these by itself when a
		// name is taken, and a parser that read the last bracket as the outcome
		// would report a perfectly good run as one that ended in a word nobody
		// knows — and lose the path as well.
		{"a path the filesystem renamed", `C:\home\runs\r (1).json`, runref.Refused},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := runref.Cite(id, tc.path)
			if tc.out != "" {
				line = runref.Ended(id, tc.path, tc.out)
			}
			got, ok := runref.Parse(line)
			if !ok {
				t.Fatalf("%q was not recognised as a reference", line)
			}
			if got.ID != id {
				t.Errorf("the id came back as %q", got.ID)
			}
			if got.Path != tc.path {
				t.Errorf("the path came back as %q, want %q", got.Path, tc.path)
			}
			if got.Outcome != tc.out {
				t.Errorf("the outcome came back as %q, want %q",
					got.Outcome, tc.out)
			}
		})
	}
}

// AND A RECORD THAT COULD NOT BE WRITTEN SAYS SO, rather than reading as a path.
//
// The id is still worth printing — it names the frames directory — and what must not happen is a
// front end reporting `(its record could not be written)` as the place to go and look.
func TestAReferenceWithNoRecordCarriesNoPath(t *testing.T) {
	line := runref.Cite("run-20261019-201500-abcd", "")
	if !strings.Contains(line, "could not be written") {
		t.Fatalf("the line does not say the record is missing: %q", line)
	}
	got, ok := runref.Parse(line)
	if !ok {
		t.Fatalf("%q was not recognised as a reference", line)
	}
	if got.Path != "" {
		t.Errorf("the path came back as %q, which is a sentence and not a place",
			got.Path)
	}
	if got.Outcome != "" {
		t.Errorf("the outcome came back as %q", got.Outcome)
	}
}

// AND NOTHING ELSE A RUN PRINTS IS MISTAKEN FOR ONE.
//
// A transcript is mostly prose and a front end reads every line of it. A parser that recognised
// one of these would hand somebody an outcome the run never reported.
func TestOrdinaryTranscriptLinesAreNotReferences(t *testing.T) {
	for _, line := range []string{
		`Performing "ark graphics live" against arkascended (hwnd:1)`,
		"  Starting in 5s.",
		"  the run was stopped (cancelled)",
		"running late",
		"",
		"  run",
		"  run    ",
	} {
		if got, ok := runref.Parse(line); ok && got.ID != "" {
			t.Errorf("%q was read as a reference to %q", line, got.ID)
		}
	}
}

// AND THE VOCABULARY IS WHOLE, with nothing outside it answering to it.
//
// `Known` is what the parser leans on to tell an outcome from a bracket that happens to be at the
// end of a path, so a word missing from the list is a word a run can end in and a front end
// cannot read.
func TestEveryWordARunCanEndInIsKnown(t *testing.T) {
	if len(runref.Every) != 7 {
		t.Errorf("there are %d outcomes; the seven are the ones the record, the "+
			"front end and the report all agree on", len(runref.Every))
	}
	for _, o := range runref.Every {
		if !runref.Known(string(o)) {
			t.Errorf("%q is an outcome and is not known to be one", o)
		}
	}
	for _, not := range []string{"", "performed", "ok", "1", "its record could not be written"} {
		if runref.Known(not) {
			t.Errorf("%q is treated as an outcome", not)
		}
	}
}
