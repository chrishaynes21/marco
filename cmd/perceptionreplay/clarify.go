package main

// `perceptionreplay -frames -clarify <artifact.json>` — print what Marco would ask, and what
// each possible answer would change.
//
// Read-only on the artifact: nothing is written, no answer is given, and no workflow is altered.
// The point is the sentence a person would actually see, and the honest account of what their
// answer buys — which, for a reading nested inside its own container, is different depending on
// which one they name.

import (
	"fmt"
	"os"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// pixelsOf renders a normalised rectangle at the resolution the demonstration was recorded at,
// because "0.0885x0.0130" is not something a person can picture and "170x14 px" is.
func pixelsOf(r observe.Region, w, h int) string {
	return fmt.Sprintf("%dx%d px at (%d,%d)",
		int(r.Width*float64(w)+0.5), int(r.Height*float64(h)+0.5),
		int(r.X*float64(w)+0.5), int(r.Y*float64(h)+0.5))
}

func reportClarifications(path string, w, h int) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	a, err := workflowstore.Unmarshal(body)
	if err != nil {
		return err
	}

	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("WHAT MARCO WOULD ASK ABOUT %q\n", a.Name)
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  %d step(s), %d control(s), %d clarification(s) already recorded\n",
		len(a.Steps), len(a.Controls), len(a.Clarifications))

	qs := a.Questions()
	if len(qs) == 0 {
		fmt.Printf("\n  Nothing is open. Every control the demonstration left ambiguous\n")
		fmt.Printf("  has been answered, or there were none.\n")
		return nil
	}
	fmt.Printf("\n  %d control(s) cannot be settled from the demonstration alone.\n", len(qs))

	for _, q := range qs {
		fmt.Printf("\n  %s\n", repeat("-", 74))
		fmt.Printf("  MARCO NEEDS CLARIFICATION — %s, used by step(s) %v\n",
			q.Control, q.Steps)
		fmt.Printf("  %s\n", repeat("-", 74))
		fmt.Printf("  %s\n\n", wrapAt(q.Because, 70, "  "))
		for _, o := range q.Options {
			// THE SIZE, IN TERMS A PERSON CAN JUDGE. "1184x624" and "166x40" are
			// both just numbers until one of them is most of the screen.
			share := ""
			if f := o.Rect.Width * o.Rect.Height; f >= 0.1 {
				share = fmt.Sprintf("  \u2014 %.0f%% of the window", f*100)
			}
			fmt.Printf("    [%s]  %s%s\n", o.Ref, pixelsOf(o.Rect, w, h), share)
			fmt.Printf("        %s\n", wrapAt(o.What, 64, "        "))
		}
		fmt.Printf("\n    [refuse]   none of these is the control\n")
		fmt.Printf("    [defer]    not now; ask me again\n")

		// ── AND WHAT EACH ANSWER WOULD CHANGE ──
		//
		// Simulated on a COPY, so the artifact on disk is untouched and this command
		// stays a question rather than an answer.
		fmt.Printf("\n    what each answer would change:\n")
		for _, o := range q.Options {
			trial, uerr := workflowstore.Unmarshal(body)
			if uerr != nil {
				return uerr
			}
			if cerr := trial.Clarify(workflowstore.Answer{
				Control: q.Control, Outcome: workflowstore.AnswerChosen,
				Ref: o.Ref, By: "(simulated, nothing is recorded)",
				At: time.Unix(0, 0).UTC(),
			}); cerr != nil {
				return cerr
			}
			for _, r := range trial.Resolutions() {
				if r.Control != q.Control {
					continue
				}
				mark := "still refused"
				if r.Settled {
					mark = "MATCHABLE"
				}
				fmt.Printf("      [%s] -> %s\n", o.Ref, mark)
				fmt.Printf("        %s\n", wrapAt(r.Because, 62, "        "))
			}
		}
	}

	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  AND WHAT AN ANSWER DOES NOT DO\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  It is evidence about one control of this workflow. It is not a rule:\n")
	fmt.Printf("  no other control learns that a label's container is its button. It is\n")
	fmt.Printf("  not recognition: choosing a region in a recorded frame says nothing\n")
	fmt.Printf("  about whether that region is on the screen now, and binding still\n")
	fmt.Printf("  refuses. And it is not authorisation: the workflow stays unconfirmed\n")
	fmt.Printf("  and unexecutable, because answering a question is not agreeing to\n")
	fmt.Printf("  press anything.\n")
	return nil
}
