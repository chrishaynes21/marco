package main

// `director what-i-did "<name>"` — the raw demonstration behind a saved routine.
//
// # Why this is its own command
//
// `director sequences "<name>"` says what Marco KEPT: screens, controls, steps, the readings
// that might find each control again. That is the executable interpretation, and it is the
// right answer to "what will this do".
//
// It is the wrong answer to the other question. A person who demonstrated something and got
// two steps out of three wants to know what became of the third, and until this existed the
// only account of it was a counter in a report that scrolled past:
//
//	input: 6 of 11 event(s) classified, 3 key_release, 2 no_session
//
// Eleven events, and not one of them could be named, placed in time, or recognised as the thing
// the person remembers doing. The owner's call:
//
//	I would stop requiring the user to repeat a demonstration simply because Marco didn't
//	preserve enough evidence the first time.
//
// So this reads the companion record and prints the demonstration as a sequence of acts. It
// writes nothing, interprets nothing, and performs nothing: a press shown here is evidence of
// where somebody pressed and is not a target. See workflowstore/evidence.go.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

func runWhatIDid(args []string) int {
	name, rest := "", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, rest = args[0], args[1:]
	}
	fs := flag.NewFlagSet("what-i-did", flag.ExitOnError)
	_ = fs.Parse(rest)
	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(os.Stderr, "director: say which play, e.g.\n"+
			`  director what-i-did "ark graphics"`)
		return 2
	}
	store, err := demo.Open(configDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	a, ok := store.Sequence(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "director: no play is saved as %q\n", name)
		return 1
	}
	ev, ok := store.Evidence(a)
	if !ok {
		// TWO CAUSES, AND THEY SEND A READER TO DIFFERENT PLACES. A routine saved
		// before the raw record existed has none; a routine re-demonstrated since has
		// one that belongs to the earlier session and is deliberately refused.
		fmt.Fprintf(os.Stderr, "director: %q is saved, and no raw demonstration of it "+
			"is.\n  Either it was taught before the raw record existed, or the "+
			"record on disk\n  belongs to a different session of the same name. "+
			"Demonstrate it again to keep one.\n", a.Name)
		return 1
	}
	reportRawDemonstration(os.Stdout, a, ev)
	return 0
}

// reportRawDemonstration prints the record as a sequence of acts.
func reportRawDemonstration(w io.Writer, a workflowstore.Artifact, e workflowstore.Evidence) {
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 74))
	fmt.Fprintf(w, "WHAT YOU DID, and what %q was learned from\n\n", a.Name)
	where := e.Window.Application
	if e.Window.Title != "" {
		where = fmt.Sprintf("the window matching %q", e.Window.Title)
	}
	fmt.Fprintf(w, "  watching %s over %d look(s)", where, e.Looks)
	if e.Window.Generations > 1 {
		// THE WINDOW CHANGED UNDER THE DEMONSTRATION. Geometry either side of that
		// join is not comparable, and a reader has to be told rather than left to
		// find two screens that should have been one.
		fmt.Fprintf(w, ", and the window was replaced %d time(s) during it",
			e.Window.Generations-1)
	}
	fmt.Fprintln(w)
	if e.Truncated > 0 {
		fmt.Fprintf(w, "  %d moment(s) past the record's bound were not kept\n",
			e.Truncated)
	}

	// ── AND THE MOMENTS WHEN SOMETHING ELSE WAS IN FRONT ──
	//
	// Named only when it happened. A window capture is not occlusion-safe, so a look
	// taken while something covered the target can hold another application's pixels
	// under the target's name — and an input that arrived on one of those looks may not
	// have reached the application at all. Nothing is discarded for it; the person is
	// told which part of their demonstration Marco cannot vouch for.
	if covered := notFront(e); len(covered) > 0 {
		fmt.Fprintf(w, "\n  Something else was in front for %d of the %d moment(s) "+
			"that carried input\n", len(covered), len(e.Moments))
		fmt.Fprintf(w, "    look(s) %s — what you pressed then may not have reached "+
			"%s at all\n", looksOf(covered), where)
	}

	// ── THE PRESSES, EVERY ONE, WITH WHAT BECAME OF IT ──
	if len(e.Presses) > 0 {
		fmt.Fprintf(w, "\n  Presses\n")
		for _, p := range e.Presses {
			fmt.Fprintf(w, "    look %3d  %s   %s\n", p.Look,
				pressWhere(p), whatCameOfThePress(p))
		}
	}

	// ── THE KEYS, AS PRESSES RATHER THAN AS A COUNT ──
	if ks := e.Keystrokes(); len(ks) > 0 {
		fmt.Fprintf(w, "\n  Keys\n")
		for _, k := range ks {
			fmt.Fprintf(w, "    %6dms  %-8s %s\n", k.AtMS, k.Intent, keyShape(k))
		}
	}

	// ── AND WHAT THE PRODUCER REFUSED, which is the rest of the account ──
	in := e.Counts
	fmt.Fprintf(w, "\n  %d event(s) reached the producer, %d became navigation",
		in.Received, in.Classified)
	// SORTED, so two readings of one record print the same line. A map range here would
	// reorder the refusals between runs, which makes a diff of two reports unreadable and
	// is the kind of noise that gets a report ignored.
	for _, why := range sortedIgnored(in.Ignored) {
		fmt.Fprintf(w, ", %d %s", in.Ignored[why], why)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  This is what you did. What Marco made of it is "+
		"`director sequences %q`.\n", a.Name)
}

// pressWhere is a press position, in percentages of the window, as it was recorded.
func pressWhere(p workflowstore.DemonstratedPress) string {
	on := "an unrecognised screen"
	if p.Screen != "" {
		on = p.Screen
	}
	return fmt.Sprintf("%4.1f%%,%4.1f%% on %-10s", p.At.X*100, p.At.Y*100, on)
}

// whatCameOfThePress is the READING of one press, said as a reading.
//
// Three outcomes and they are not ranked. A press on a control Marco could find again is the
// useful one; a press on nothing the proposer found is not a worse press, it is a press whose
// boundary nobody can name — and saying so is the difference between reporting what the person
// did and grading it.
func whatCameOfThePress(p workflowstore.DemonstratedPress) string {
	switch {
	case p.Control == "":
		return "nothing was proposed there, so there is no boundary to find again"
	case p.Ambiguous:
		return fmt.Sprintf("%s, one of %d regions containing it",
			p.Control, p.Containing)
	default:
		return p.Control
	}
}

// keyShape says what KIND of press one keystroke was: a tap, a hold, or one of a chord.
//
// From the edges and nothing else. "Held" and "with N repeats" are observations; "one of a
// chord" is deliberately worded as a shape and not as an intention, because two intents held
// together are what a chord looks like and are also what a slow hand looks like.
func keyShape(k workflowstore.Keystroke) string {
	var parts []string
	switch {
	case k.HeldMS < 0:
		parts = append(parts, "never released during the session")
	case k.HeldMS < 200:
		parts = append(parts, fmt.Sprintf("a tap, %dms", k.HeldMS))
	default:
		parts = append(parts, fmt.Sprintf("held %dms", k.HeldMS))
	}
	if k.Repeats > 0 {
		parts = append(parts, fmt.Sprintf("%d auto-repeat(s)", k.Repeats))
	}
	if len(k.Overlaps) > 0 {
		parts = append(parts, fmt.Sprintf("held at the same time as key %d — a chord, "+
			"or two presses that overlapped", k.Overlaps[0]+1))
	}
	if k.Conditional {
		parts = append(parts, "admitted only because the screen looked like choices")
	}
	return strings.Join(parts, "; ")
}

// notFront is the moments whose look was taken while something covered the application.
func notFront(e workflowstore.Evidence) []workflowstore.Moment {
	var out []workflowstore.Moment
	for _, m := range e.Moments {
		if !m.Front {
			out = append(out, m)
		}
	}
	return out
}

// looksOf lists the look indices of some moments, bounded so a long run reads as a run.
func looksOf(ms []workflowstore.Moment) string {
	const most = 8
	parts := make([]string, 0, most+1)
	for i, m := range ms {
		if i == most {
			parts = append(parts, fmt.Sprintf("and %d more", len(ms)-most))
			break
		}
		parts = append(parts, strconv.Itoa(m.Look))
	}
	return strings.Join(parts, ", ")
}

// sortedIgnored orders the refusal reasons so two readings of one record agree.
func sortedIgnored(m map[observe.IgnoreReason]int) []observe.IgnoreReason {
	out := make([]observe.IgnoreReason, 0, len(m))
	for r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
