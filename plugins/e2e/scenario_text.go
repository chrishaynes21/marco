//go:build windows

package main

// SCENARIO: TYPED TEXT, AND THE STEP THAT FOLLOWS IT.
//
// Typing is the one input that can go in PART WAY. A press either happened or did not; a text can
// leave a field holding `marco typed th` — and the step after it was written for the field as it
// was going to be. So there are two things to establish and they are different claims:
//
//	THE RECEIPT   the characters that arrived are the characters that were demonstrated, in the
//	              field the demonstration put them in and in no other.
//	THE ORDER     the press after the typing arrived AFTER it. A routine that pressed Save while
//	              the text was still going in would submit a form holding part of a value, and
//	              every account inside Marco would call that a run that worked.
//
// # Why the application's own content is the oracle here, and could not be for the buttons
//
// Because text is the one thing the fixture can read back. A press is a message it received; a
// character is a message it received AND a change to what it holds, and the content it publishes
// is independent of anything Marco says about the run. `SendInput` returning a count says the
// system took the events — nothing more — so the count and the record agree on "submitted" and
// neither can tell you what is in the box.
//
// # What is deliberately not here
//
// A REFUSED typing. The machine takes what this harness sends, so a delivery failure cannot be
// arranged on a desktop at all — and the claim that matters about it is what does NOT happen
// afterwards, which is held headlessly: `internal/codegen`
// `TestATypingThatDidNotHappenStopsWhatFollowsIt` refuses each segment in turn and asserts the
// next step was never asked for. What this adds is the other half of that sentence — that when
// the typing DOES go in, the next step follows it rather than racing it.
//
// A SECRET. The text here is synthetic and non-sensitive on purpose; a scenario that typed a real
// credential-store value onto a desktop would be putting somebody's password into a fixture's
// text box and into this run's artifacts.
//
// NON-BMP TEXT. `printable` maps a key NAME to a character, so what can be DEMONSTRATED is what
// a keyboard has keys for — and an emoji has no key. The surrogate-pair path is exercised in
// `internal/oshost` against the real adapter instead.
//
// See [[ADR-261-typed-text-is-a-sequence-of-submissions]].

import (
	"fmt"
	"strings"
	"time"
)

// theTypedText is what the routine types: synthetic, non-sensitive, and varied enough that
// `simplify` folds it into one `Type` step rather than leaving it as keystrokes.
const theTypedText = "marco typed this 42"

// typedTextArrivesAndTheNextStepFollowsIt is this family's only scenario, and it is one run
// because both claims are about the same one.
func typedTextArrivesAndTheNextStepFollowsIt(w *world) result {
	started := time.Now()
	st, err := setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedTyping(f, w.art, "fixture typing", "Name", theTypedText)
	})
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	before, seq, records := st.f.mustRead(), st.f.lastSeq(), countRecords(st.store)
	// THE FIELD HAS TO START EMPTY, or "it holds the text now" says nothing about this
	// run. A fact about the arrangement, so NOT RUN rather than a failure.
	if before.Name != "" {
		return skip(started, fmt.Sprintf("the fixture's Name field already holds %q, "+
			"so what it holds afterwards would not be this run's doing",
			before.Name))
	}
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	rec, err := waitForRecord(60*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, fmt.Sprintf("Marco wrote no finished run record for the "+
			"typing routine: %v. The application received %s", err,
			st.f.arrivedSoFar(seq)))
	}
	w.art.step("Marco recorded %s", rec)
	if err := st.f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	after := st.f.mustRead()
	w.art.step("the fixture's Name field holds %q", after.Name)
	presses, err := st.f.presses(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	keys, err := st.f.keys(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	for _, e := range presses {
		w.art.step("the fixture received %s", e)
	}

	why := firstOf(
		theTextDidNotArrive(after, theTypedText),
		itWentIntoAnotherField(before, after),
		theNextStepDidNotHappen(before, after),
		theNextStepDidNotWaitForTheText(presses, keys),
		marcoSaidSomethingElse(rec, st.seed),
	)
	if why != "" {
		st.fail()
		return fail(started, why,
			fmt.Sprintf("the field holds %q and the routine typed %q", after.Name,
				theTypedText),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("the application's own Name field holds %q, character for "+
			"character", after.Name),
		fmt.Sprintf("and Save was pressed after the last character arrived, going "+
			"%d → %d", before.SavePresses, after.SavePresses),
		fmt.Sprintf("Marco's record: %s", rec))
}

// ── THE DISAGREEMENTS THIS FAMILY ADDS ────────────────────────────────────────

// theTextDidNotArrive is the receipt, read off the application's own content.
//
// CHARACTER FOR CHARACTER, and not a prefix or a contains: a field holding the first fourteen
// characters is exactly what a partial submission leaves behind, and it is the thing this whole
// contract exists to be able to report rather than to tolerate.
func theTextDidNotArrive(after fixtureState, want string) string {
	if after.Name == want {
		return ""
	}
	switch {
	case after.Name == "":
		return fmt.Sprintf("the routine typed %q and the application's Name field is "+
			"empty, so nothing arrived at all", want)
	case strings.HasPrefix(want, after.Name):
		return fmt.Sprintf("the field holds %q and the routine typed %q — the first "+
			"%d character(s) of it arrived and the rest did not, which is "+
			"exactly the partial submission this is about", after.Name, want,
			len([]rune(after.Name)))
	}
	return fmt.Sprintf("the field holds %q and the routine typed %q, so what arrived "+
		"is not what was demonstrated", after.Name, want)
}

// itWentIntoAnotherField catches text that arrived somewhere nobody chose.
//
// A key goes wherever the keyboard is, and what put the keyboard in the Name field was the press
// in front of the typing. If that press missed, the characters go to whatever has focus — the
// other field, a button, the window itself — and a scenario reading only the Name field would
// report that nothing arrived without saying where it went.
func itWentIntoAnotherField(before, after fixtureState) string {
	if after.Notes == before.Notes {
		return ""
	}
	return fmt.Sprintf("the Notes field went from %q to %q, so the typing went into a "+
		"field the demonstration never pressed", before.Notes, after.Notes)
}

// theNextStepDidNotHappen is the step after the typing, by the application's own count.
func theNextStepDidNotHappen(before, after fixtureState) string {
	if after.SavePresses == before.SavePresses+1 {
		return ""
	}
	return fmt.Sprintf("the routine presses Save after it types, and the application's "+
		"own count of Save went %d → %d", before.SavePresses, after.SavePresses)
}

// theNextStepDidNotWaitForTheText is the ORDER, by the application's own sequence numbers.
//
// The claim this scenario exists for, over and above the receipt: a routine that sent the press
// while the characters were still going in would submit a form holding part of a value, and both
// of the assertions above would pass — the field ends up correct, the count goes up, and the
// form was submitted with something else in it.
//
// Read off the fixture's own counter, which is one clock: the press's sequence number has to be
// higher than the last key's. Nothing about Marco's own timing is involved, and nothing here
// counts the keys — a `KEYEVENTF_UNICODE` send produces a `VK_PACKET` key message per code unit
// and counting those would be asserting the encoding rather than the order.
func theNextStepDidNotWaitForTheText(presses, keys []fixtureEvent) string {
	if len(presses) == 0 {
		return "the application received no press after the typing, so there is " +
			"nothing to order against it"
	}
	if len(keys) == 0 {
		return "the application received no key messages at all, so the order " +
			"between the typing and the press after it cannot be established"
	}
	last := keys[len(keys)-1]
	save := presses[len(presses)-1]
	if save.Seq > last.Seq {
		return ""
	}
	return fmt.Sprintf("the press arrived as record #%d and the last character as "+
		"#%d, so Save was pressed while the text was still going in — the field "+
		"ends up correct and the form was submitted holding part of a value",
		save.Seq, last.Seq)
}
