package goal

// AN ANSWER TO A QUESTION MARCO ASKED IS NOT A NEW REQUEST.
//
// Measured live. "Make a new folder" asked "create a folder — what should it be called?", and an
// answer of "Reports" was re-run as a request to find a control called Reports — because an
// answer the clarification parser does not recognise is treated as a new command, which is the
// right reading of an unrecognised phrase and the wrong reading of an answer Marco asked for.

import "testing"

func TestAnAnsweredQuestionBecomesTheRequestThePersonMeant(t *testing.T) {
	for _, tc := range []struct {
		asked, answer, want, wantName string
	}{
		{"make a new folder", "Reports", "make a new folder called Reports", "Reports"},
		{"create a folder", "Q3", "create a folder called Q3", "Q3"},
		{"new folder", "My Notes", "new folder called My Notes", "My Notes"},
		// The question mark and the spacing the person typed are tidied, not carried.
		{"make a folder ", "  Archive ", "make a folder called Archive", "Archive"},
	} {
		got, ok := Supplying(tc.asked, RequiresName, tc.answer)
		if !ok {
			t.Errorf("%q + %q could not be completed", tc.asked, tc.answer)
			continue
		}
		if got != tc.want {
			t.Errorf("%q + %q = %q, want %q", tc.asked, tc.answer, got, tc.want)
		}
		// AND THE COMPLETED REQUEST PARSES BACK TO WHAT WAS MEANT, which is the whole
		// of why this reconstructs the phrase rather than patching the goal: the
		// patterns are the vocabulary, and this uses them.
		g, parsed := Parse(got)
		if !parsed || g.Kind != CreateFolder || g.Param(ParamName) != tc.wantName {
			t.Errorf("%q parses to %+v", got, g)
		}
	}
}

// AND AN ANSWER THAT CANNOT BE FITTED BACK IS REFUSED RATHER THAN RESHAPED.
//
// The self-check is the safety property. A requirement with no phrasing, an empty answer, or a
// request the completed phrase no longer parses as, must all come back false — because the
// alternative is running something the person did not say.
func TestAnAnswerThatCannotBeFittedBackIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name, asked, answer string
		missing             Requirement
	}{
		{"no answer", "make a new folder", "", RequiresName},
		{"only spaces", "make a new folder", "   ", RequiresName},
		// "Which one?" is answered by pointing or by choosing from a list, not by
		// appending a word to the sentence — so there is deliberately no phrasing for it.
		{"a target", "rename this file", "notes.txt", RequiresTarget},
		// A phrase that is not a goal at all cannot be completed into one.
		{"not a goal", "click the thing", "Reports", RequiresName},
	} {
		if got, ok := Supplying(tc.asked, tc.missing, tc.answer); ok {
			t.Errorf("%s: %q + %q was turned into %q", tc.name, tc.asked, tc.answer, got)
		}
	}
}

// AND A DESTINATION IS ANSWERED THE WAY A DESTINATION IS SAID.
func TestADestinationAnswerUsesItsOwnJoiner(t *testing.T) {
	got, ok := Supplying("move this file", RequiresDestination, "Downloads")
	if !ok {
		t.Fatal("a destination answer could not be completed")
	}
	g, parsed := Parse(got)
	if !parsed || g.Param(ParamDestination) != "Downloads" {
		t.Errorf("%q parses to %+v", got, g)
	}
}
