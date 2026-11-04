package main

import (
	"strings"
	"testing"
)

// F6, BOTH SIDES. A bare Enter at the widget's own offers declines: nothing runs and no recording
// begins, and only y does either. But Enter still SUBMITS A COMMAND: a typed line whose read-back
// is on screen runs, because that is the command line and not a yes/no question.
//
// Taking Enter as yes at an offer again must fail this.
func TestEnterDeclinesAnOfferAndStillSubmitsACommand(t *testing.T) {
	runs := runsRecorded(t)
	h := newModel()

	offerRoutine(h, "open settings", "open-settings", "", "open-settings", false)
	if p := h.snapshot().prompt; !strings.Contains(p, "[y]es / [N]o") {
		t.Errorf("the offer does not show that Enter means no: %q", p)
	}
	answerPrompt(h, "") // Enter
	if got := runs(); len(got) != 0 {
		t.Fatalf("Enter at an offer ran a routine: %q", got)
	}
	offerRoutine(h, "open settings", "open-settings", "", "open-settings", false)
	answerPrompt(h, "y")
	if got := runs(); len(got) != 1 || got[0] != "do --play=open-settings" {
		t.Errorf("y at the offer ran %q", got)
	}

	asked := marcoAnswers(t, "", 0)
	offerLearn(h, "polish the silver")
	answerPrompt(h, "") // Enter
	if got := asked(); got != "" {
		t.Errorf("Enter at the learn offer began a recording: %q", got)
	}
	if takePendingLearn() != "" {
		t.Error("the declined learn offer is still pending")
	}

	// And Enter on a typed line with its read-back on screen still runs it.
	r := aCloudPull("cloud pull stone", "stone")
	engineReads(t, map[string]lineReading{r.Line: r})
	onScreen(h, r)
	typedEnter(h, r.Line)
	if got := runs(); len(got) != 2 || !strings.Contains(got[1], "--arg=resource=stone") {
		t.Errorf("Enter on a ready command did not run it: %q", got)
	}
}
