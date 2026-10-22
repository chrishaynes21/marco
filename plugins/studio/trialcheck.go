package main

// WHAT A TRIAL OF STUDIO SHOWED, READ BACK OUT OF THE STORE.
//
// `studio -trial <store>` prints the journals Studio wrote while somebody used it: which builds
// took part, the states each recording passed through, how often the page had to ask for a
// picture, and the canonical result of each run. It opens nothing, spawns nothing and changes
// nothing — the same promise `-acts` makes, for the same reason.
//
// It exists so that the answer to "what happened when you tried it" is a command rather than a
// conversation. The four facts it ends on are the four the owner should never be asked to
// collect:
//
//	build identity · recording states · preview persistence · the canonical result
//
// They are printed as a verdict per trial, in that order, and each one says what it is reading.

import (
	"fmt"
	"sort"
	"strings"
)

// trialsShown is how many journals are printed in full. A trial is a sitting at the machine, and
// the ones worth reading are the ones somebody has just done.
const trialsShown = 5

// runTrialCheck prints the trials in a store. Returns an exit code.
func runTrialCheck(home string) int {
	if home == "" {
		fmt.Println("FAIL  -trial needs a store to read: -trial <dir>")
		return 2
	}
	all, err := trialsIn(home)
	if err != nil {
		// NO JOURNALS IS NOT A FAULT. It is a store nobody has used Studio
		// against yet, and saying so is more use than an error about a directory.
		fmt.Printf("TRIALS  none in %s\n\n  Studio writes one while somebody uses "+
			"it: record a demonstration, review it, compose an Act, press "+
			"Play.\n", home)
		return 0
	}
	fmt.Printf("TRIALS  %d in %s\n", len(all), home)
	if len(all) > trialsShown {
		fmt.Printf("  (the %d oldest are listed and not read)\n", len(all)-trialsShown)
		for _, t := range all[:len(all)-trialsShown] {
			fmt.Printf("  %s  %d notes\n", t.Path, len(t.Notes))
		}
		all = all[len(all)-trialsShown:]
	}
	// NEWEST FIRST, because the trial somebody is asking about is the one they just did.
	for i := len(all) - 1; i >= 0; i-- {
		printTrial(all[i])
	}
	fmt.Printf("\n  Nothing was run, nothing was pressed, and no window was opened.\n")
	return 0
}

// printTrial prints one journal and the verdict on it.
func printTrial(t trial) {
	fmt.Printf("\n────────────────────────────────────────────────────────────────\n")
	fmt.Printf("TRIAL  %s\n", t.Path)
	if t.Unreadable > 0 {
		// A LINE THIS BUILD CANNOT VOUCH FOR IS SAID, not skipped quietly. A
		// journal from a later build is partly readable, and a reader who is not
		// told would take a partial account for the whole one.
		fmt.Printf("  %d line(s) could not be read by this build\n", t.Unreadable)
	}
	if t.Older {
		// SAID, because a field this build would have written and that journal
		// could not is absent rather than false, and the difference matters on
		// every line of the verdict below.
		fmt.Printf("  written by an earlier build of Studio, whose shape this " +
			"one still reads; anything added since is absent rather than " +
			"false\n")
	}
	if open, ok := t.first(trialOpened); ok {
		fmt.Printf("  window    %s\n  director  %s\n  store     %s\n"+
			"  routes    %s\n", orNone(open.Build), open.Director,
			open.Store, open.Routes)
	}
	if len(t.Notes) > 0 {
		fmt.Printf("  began     %s\n",
			t.Notes[0].At.Local().Format("2006-01-02 15:04:05"))
	}

	printRecording(t)
	printPictures(t)
	printAct(t)
	printRun(t)
	printVerdict(t)
}

// printRecording prints each recording of the trial and the states it passed through.
func printRecording(t trial) {
	for _, r := range t.recordings() {
		fmt.Printf("\n  RECORDING  %q on %s, %ds\n", r.Name(),
			r.Began.Application, r.Began.Seconds)
		// THE STATES, WITH WHEN EACH ONE ARRIVED.
		//
		// The seconds are the point. "Settling is ass, it doesn't finish in
		// time" is a claim about the gap between two of these lines, and the
		// only way to answer it without asking somebody to hold a stopwatch
		// is to write down when each one was shown.
		for _, s := range r.States {
			fmt.Printf("    %6.1fs  %-10s %s\n",
				s.At.Sub(r.Began.At).Seconds(), s.Phase, s.Doing)
			if s.Missing != "" {
				fmt.Printf("              why: %s\n", s.Missing)
			}
		}
		presses, watched, captured, unchecked, read := r.Got()
		fmt.Printf("    presses %d, placed %d, checked %d, unchecked %d; "+
			"starting screen read: %s\n", presses, watched, captured,
			unchecked, yesNo(read))
		if rev, differs := r.Recorder(); rev != "" {
			fmt.Printf("    recorder  %s%s\n", rev,
				ifSo(differs, "  — A DIFFERENT BUILD FROM THE WINDOW"))
		}
		if r.Over == nil {
			fmt.Printf("    (this recording had not finished when the " +
				"journal ended)\n")
			continue
		}
		fmt.Printf("    result    %s\n", r.Over.Said)
		if r.Over.Next != "" {
			fmt.Printf("    what to do: %s\n", r.Over.Next)
		}
		kept := append(append([]string(nil), r.Over.Screens...),
			r.Over.Controls...)
		fmt.Printf("    pictures kept: %s\n", orNone(strings.Join(kept, " ")))
	}
}

// printPictures prints the preview tally.
func printPictures(t trial) {
	worst, most, burst, labels, total := t.refetched()
	if labels == 0 {
		return
	}
	fmt.Printf("\n  PICTURES ASKED FOR  %d label(s), %d request(s), most for any "+
		"one: %d, biggest burst: %d in one stretch\n", labels, total, most,
		burst)
	for _, w := range worst {
		fmt.Printf("    RE-FETCHED: %s\n", w)
	}
}

// printAct prints what was composed, saved or refused.
func printAct(t trial) {
	for _, n := range t.Notes {
		if n.Kind != trialAct {
			continue
		}
		state := "saved"
		if n.Error != "" {
			state = "REFUSED: " + n.Error
		}
		fmt.Printf("\n  ACT  %q from %q — %s\n", n.Act, n.From, state)
		if len(n.Steps) > 0 {
			fmt.Printf("    steps  %s\n", strings.Join(n.Steps, " · "))
		}
	}
}

// printRun prints each performance and its canonical result.
func printRun(t trial) {
	for _, n := range t.Notes {
		if n.Kind != trialRan {
			continue
		}
		fmt.Printf("\n  PLAY  %q %s\n", n.Act, liveOrDry(n.Live))
		if n.Reference == "" {
			fmt.Printf("    the run cited no reference, so there is no "+
				"canonical account of it%s\n", "")
			continue
		}
		fmt.Printf("    run       %s\n", n.Reference)
		if n.Outcome == "" {
			fmt.Printf("    NO RECORD was found for it, which is a run that " +
				"did not finish writing one\n")
			continue
		}
		fmt.Printf("    outcome   %s\n", n.Outcome)
		if n.Because != "" {
			fmt.Printf("    because   %s\n", n.Because)
		}
		// AND THE SENTENCE BEHIND THE TALLY. `because` counts; this explains.
		if n.Paused != "" {
			fmt.Printf("    waited on %s\n", n.Paused)
		}
		fmt.Printf("    %d action(s) sent, %d verified, performed by %s\n",
			n.Sent, n.Verified, orNone(n.Ran))
	}
}

// printVerdict ends each trial on the four facts, in the order they were asked for.
//
// Each of the four is a function that returns its own sentence rather than printing one, so a
// test asserts the JUDGEMENT and not the layout. The judgements are what somebody acts on: "two
// builds took part" and "this picture was fetched fourteen times" are the two findings this
// surface has cost the owner a day each, and a test that could only check that a line was
// printed would pass on either verdict.
func printVerdict(t trial) {
	fmt.Printf("\n  WHAT THIS TRIAL SHOWS\n")
	said := [][]string{buildVerdict(t), {statesVerdict(t)},
		{picturesVerdict(t)}, resultVerdict(t)}
	for i, about := range []string{"build identity", "recording states",
		"preview persistence", "canonical result"} {
		fmt.Printf("    %-19s %s\n", about, said[i][0])
		for _, more := range said[i][1:] {
			fmt.Printf("                        %s\n", more)
		}
	}
}

// buildVerdict is which builds took part, as the first line and then one line each.
func buildVerdict(t trial) []string {
	named, silent, agree := t.builds()
	revs := make([]string, 0, len(named))
	for rev := range named {
		revs = append(revs, rev)
	}
	sort.Strings(revs)
	quiet := ""
	if len(silent) > 0 {
		quiet = strings.Join(silent, ", ") + " did not say what build it was"
	}
	switch {
	case len(named) == 0:
		return []string{"NOT STAMPED — " + orNone(quiet)}
	case agree && quiet == "":
		return []string{fmt.Sprintf("one build throughout: %s (%s)", revs[0],
			strings.Join(named[revs[0]], ", "))}
	case agree:
		return []string{fmt.Sprintf("%s — %s", revs[0],
			strings.Join(named[revs[0]], ", ")), quiet}
	}
	out := []string{"DIFFERENT BUILDS TOOK PART:"}
	for _, rev := range revs {
		out = append(out, fmt.Sprintf("  %s — %s", rev,
			strings.Join(named[rev], ", ")))
	}
	if quiet != "" {
		out = append(out, "  "+quiet)
	}
	return out
}

// picturesVerdict is whether the page kept the pictures it was given.
func picturesVerdict(t trial) string {
	worst, _, burst, labels, _ := t.refetched()
	switch {
	case labels == 0:
		return "no picture was asked for in this trial"
	case len(worst) == 0:
		return fmt.Sprintf("held — %d label(s), and no picture was asked for "+
			"more than %d time(s) in one stretch", labels, burst)
	}
	return "NOT HELD — " + strings.Join(worst, "; ")
}

// statesVerdict is what the recording states amount to.
//
// Of the LAST recording in the trial, which is the one somebody has just done; the others are
// counted so that a verdict about one sitting's third demonstration does not read as a verdict
// about the sitting. The endings a demonstration has are each said as themselves: a press that
// was checked, a press whose result was measured and empty, and a recording that never got that
// far. The middle one is the map case — the known limitation this surface has, and the one a
// trial is most likely to be run to confirm.
func statesVerdict(t trial) string {
	all := t.recordings()
	if len(all) == 0 {
		return "nothing was recorded in this trial"
	}
	r := all[len(all)-1]
	also := ""
	if len(all) > 1 {
		also = fmt.Sprintf(" (the last of %d recordings in this trial)",
			len(all))
	}
	if len(r.States) == 0 {
		return "A RECORDING STARTED AND THE RECORDER SAID NOTHING THIS BUILD " +
			"COULD READ" + also
	}
	phases := make([]string, 0, len(r.States))
	for _, s := range r.States {
		if n := len(phases); n == 0 || phases[n-1] != s.Phase {
			phases = append(phases, s.Phase)
		}
	}
	chain := strings.Join(phases, " → ")
	took := r.States[len(r.States)-1].At.Sub(r.States[0].At).Seconds()
	_, _, captured, unchecked, _ := r.Got()
	if rev, differs := r.Recorder(); differs {
		return fmt.Sprintf("%s — AND THE RECORDER WAS A DIFFERENT BUILD (%s), "+
			"so nothing it reported is about this one%s", chain, rev, also)
	}
	switch {
	case unchecked > 0:
		// THE EMPTY RESULT OUTRANKS THE CHECKED ONE, when a recording has
		// both. A demonstration where one press verified and another could
		// not be checked is a demonstration that cannot be performed past the
		// second one, and reporting it by its successes would be reporting the
		// half that is not the finding.
		return fmt.Sprintf("%s in %.1fs — %d press(es) checked, %d whose "+
			"result was measured and empty: %s%s", chain, took,
			captured, unchecked, orNone(r.Why()), also)
	case captured > 0:
		return fmt.Sprintf("%s in %.1fs — %d press(es) checked%s", chain, took,
			captured, also)
	}
	return fmt.Sprintf("%s in %.1fs — no press was checked%s", chain, took, also)
}

// resultVerdict is the canonical result of the last run in this trial, and what it was waiting on.
func resultVerdict(t trial) []string {
	ran, ok := t.last(trialRan)
	if !ok {
		if _, started := t.last(trialPlay); started {
			return []string{"A RUN STARTED AND THE JOURNAL ENDS BEFORE IT DID"}
		}
		return []string{"nothing was performed in this trial"}
	}
	var said []string
	switch {
	case ran.Reference == "":
		said = []string{"the run cited no reference"}
	case ran.Outcome == "":
		said = []string{ran.Reference + " — NO RECORD, which is a run that did " +
			"not finish writing one"}
	default:
		said = []string{fmt.Sprintf("%s — %s, %d action(s) sent, %d verified",
			ran.Reference, ran.Outcome, ran.Sent, ran.Verified)}
	}
	// AND WHAT IT WAS WAITING ON, WHEN IT DID NOT PRESS.
	//
	// A verdict of "refused, 0 actions sent" is the safe outcome and not an
	// explanation, and the explanation is the runner's own last pause. Left off when
	// something WAS sent: the outcome is then about what happened to the press, and
	// an earlier pause would be the least interesting thing on the line.
	if ran.Paused != "" && ran.Sent == 0 {
		said = append(said, "  it was waiting on: "+ran.Paused)
	}
	return said
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(none)"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "NO"
}

func ifSo(b bool, then string) string {
	if b {
		return then
	}
	return ""
}

func liveOrDry(live bool) string {
	if live {
		return "(live)"
	}
	return "(dry)"
}
