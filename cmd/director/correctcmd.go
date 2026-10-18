package main

// `director correct "<name>" --drop <n|last>` — a person changing what Marco learned.
//
// # What it is for
//
//	Make imperfect demonstrations normal. […] And if you say "ignore that T," Marco should
//	update the learned procedure without falsifying the raw recording.
//
// Before this, a demonstration with a mistake in it had exactly one remedy: do it again. That
// is the cost the owner identified as the real one — not questions, but re-demonstration — and
// it is charged to the person for a mistake that took them a tenth of a second to make.
//
// # The three records, and which one this touches
//
//	raw demonstration     evidence/<name>.json      UNTOUCHED. `director what-i-did` reads it.
//	learned procedure     sequences/<name>.json     changed here, with the change recorded in it.
//	guarded execution     perform-workflow          unchanged; it reads the procedure.
//
// So a corrected routine still says what the person did. The step comes out of `Steps`, where
// it would be performed, and goes into `Corrections`, where it is provenance — and
// `director what-i-did` still shows the press that caused it, because that file is not written
// by this command and cannot be.
//
// # Why it names a STEP and not a key
//
// "Ignore that T" is about a keypress, and a keypress names no control: a key becomes part of a
// procedure only when the screen changed after it, and then it is the CHANGE that was learned.
// So the correctable unit is a step, and a keystroke that changed nothing has nothing to
// correct — which this command says, in one sentence, rather than asking the person what they
// meant. See [[ADR-234-the-person-does-not-have-to-prove-the-click]].

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

func runCorrect(args []string) int {
	name, rest := "", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, rest = args[0], args[1:]
	}
	fs := flag.NewFlagSet("correct", flag.ExitOnError)
	drop := fs.String("drop", "", `the step to take out: a number, or "last"`)
	because := fs.String("because", "",
		"your own words, kept with the correction as the reason")
	dry := fs.Bool("dry-run", false, "say what would change and write nothing")
	_ = fs.Parse(flagsFirst(rest))

	if strings.TrimSpace(name) == "" || strings.TrimSpace(*drop) == "" {
		fmt.Fprintln(os.Stderr, "director: say which routine and which step, e.g.\n"+
			`  director correct "ark graphics" --drop 2 --because "that click was accidental"`+
			"\n"+`  director correct "ark graphics" --drop last --because "redo the last step"`)
		return 2
	}
	store, err := demo.Open(configDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	a, ok := store.Sequence(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "director: no routine is saved as %q\n", name)
		return 1
	}
	step, err := whichStep(*drop, len(a.Steps))
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 2
	}
	// THE RAW RECORD IS READ AND NOT WRITTEN, so the report can say what is being kept.
	// Its absence is not an error: a routine taught before the record existed can still
	// be corrected, and the sentence about what survives is simply not printed.
	raw, hasRaw := store.Evidence(a)

	out, err := a.Dropping(step, *because, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	reportCorrection(os.Stdout, a, out, step, hasRaw, raw)
	if *dry {
		fmt.Printf("\n--dry-run: nothing was written.\n")
		return 0
	}
	if err := store.SaveSequence(out); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	fmt.Printf("\n%q now has %d step(s). Your demonstration is unchanged.\n",
		out.Name, len(out.Steps))
	return 0
}

// whichStep turns what the person typed into a one-based step number.
//
// "last" because that is how the owner's own example is phrased — "redo the last step" — and a
// person who has just finished demonstrating knows which step was last without counting.
func whichStep(said string, steps int) (int, error) {
	if strings.EqualFold(strings.TrimSpace(said), "last") {
		if steps == 0 {
			return 0, fmt.Errorf("there is no last step: the routine has none")
		}
		return steps, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(said))
	if err != nil {
		return 0, fmt.Errorf("--drop takes a step number or \"last\", not %q", said)
	}
	return n, nil
}

// reportCorrection says what came out, what is left, and what is still on record.
func reportCorrection(w io.Writer, before, after workflowstore.Artifact, step int,
	hasRaw bool, raw workflowstore.Evidence) {

	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 74))
	fmt.Fprintf(w, "CORRECTING %q\n\n", before.Name)
	st := before.Steps[step-1]
	fmt.Fprintf(w, "  out:  %d %s -> %s via %s\n", step, st.From, st.To,
		shortCorrectedTarget(st))
	if len(after.Steps) == 0 {
		// A ROUTINE WITH NOTHING LEFT IS STILL A ROUTINE, and saying so is better than
		// refusing: the person knows what they took out, and an empty procedure
		// performs nothing rather than performing something wrong.
		fmt.Fprintf(w, "  left: nothing. The routine keeps its name and its screens, "+
			"and performs no step.\n")
	} else {
		fmt.Fprintf(w, "  left:\n")
		for i, s := range after.Steps {
			fmt.Fprintf(w, "    %d %s -> %s via %s\n", i+1, s.From, s.To,
				shortCorrectedTarget(s))
		}
	}
	// AND WHAT IS STILL ON RECORD, which is the half that makes this a correction rather
	// than a deletion.
	if hasRaw {
		fmt.Fprintf(w, "\n  Kept: your demonstration — %d press(es) and %d keystroke(s), "+
			"with where and when.\n", len(raw.Presses), len(raw.Keystrokes()))
		fmt.Fprintf(w, "        director what-i-did %q\n", before.Name)
	}
	fmt.Fprintf(w, "  Kept: the step itself, with your reason, in the routine's own "+
		"corrections.\n")
}

// shortCorrectedTarget names what a step pressed, or says that nothing was pressed.
//
// A step with no target is not a broken step: a screen that changed by itself, or after a key,
// is a real part of a routine and is reported as the thing it is.
func shortCorrectedTarget(s workflowstore.Step) string {
	if s.Control == "" {
		return fmt.Sprintf("(no control — %s)", causeWords(s.Cause))
	}
	return s.Control
}

// causeWords is why a screen changed, in words a person reads rather than a vocabulary word.
func causeWords(c observe.TransitionCause) string {
	switch c {
	case observe.CauseAutomatic:
		return "it followed a click and changed on its own"
	case observe.CauseUnknown:
		return "the screen changed for another reason"
	default:
		return string(c)
	}
}
