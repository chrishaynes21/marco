package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/meaning"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// SHADOW MODE: what WOULD a semantic interpreter have said, and would it have been right?
//
// Every case in the evaluation is also put to the configured interpreter
// ($MARCO_INTERPRETER), scored against the same oracle, and compared with what Marco actually
// decided. Nothing it proposes reaches resolution, the plan, the policy or the executor.
//
// # Why shadow first, rather than wiring it in behind a confidence threshold
//
// Because the question is not only "is it right more often". It is whether it identifies the
// right control when a person describes something naturally AND recognises when it does not
// know enough — and a mechanism that is already acting cannot be measured against what it
// displaced. Four numbers decide it, and they are kept apart on purpose:
//
//	WOULD ADD      cases Marco got wrong or refused, where the interpreter was right.
//	WOULD BREAK    cases Marco got RIGHT, where the interpreter proposed something else.
//	ABSTAINED      cases it declined — the capability, not the shortfall.
//	WRONG CONTROL  proposals naming a control the person did not mean.
//
// WOULD BREAK and WRONG CONTROL are what a shadow run exists to find. An interpreter whose
// ADD is large and whose BREAK is zero has earned a role in control selection; one that buys
// paraphrases by occasionally overruling a correct literal answer has not, however good its
// accuracy looks in total.
//
// # And an abstention is a good answer
//
// A model asked to choose one of eighty things will choose one. "None of these is what you
// asked for" is the right reply to a great many requests, and an interpreter that cannot say
// it is worse than none: its confident wrong proposals become indistinguishable from its
// confident right ones. So abstentions are counted as a capability rather than as a miss, and
// the only abstentions scored against it are the ones where the oracle named a control that
// was on screen.

// shadowTally is what the interpreter did, kept entirely separate from Marco's own counts.
type shadowTally struct {
	asked, abstained int
	correct          int
	wrong            int
	// brokeAmbiguity is a proposal of ONE control where the person should have been asked.
	brokeAmbiguity int
	// proposedTheAbsent is a proposal where nothing on screen is what was asked for.
	proposedTheAbsent int
	wouldAdd          []string
	wouldBreak        []string
	// gatedGain and gatedLoss are the only two numbers that decide whether an interpreter
	// may be wired in, and they are measured under the narrowest arrangement that could
	// ever be safe: consulted ONLY where the deterministic reading returned ABSENT.
	//
	// Under that gate the interpreter cannot override a correct answer and cannot settle a
	// tie, because it is never asked about either — so WOULD BREAK goes to zero by
	// construction and the whole question becomes these two:
	//
	//	gatedGain  Marco refused, the control WAS there, and the interpreter named it.
	//	gatedLoss  Marco refused, nothing there was what was asked for, and the
	//	           interpreter named something anyway.
	//
	// A model with gain above loss has earned a role. A model with any loss at all has
	// turned an honest refusal into a wrong action, which is the failure a person cannot
	// see happening.
	gatedGain []string
	gatedLoss []string
	// byKind is accuracy per phrasing, which is the whole point: the deterministic
	// reading already closes form, and what is being bought is meaning.
	byKind  map[string]*[2]int
	latency []time.Duration
	// controlsSent is the cost: an interpreter is charged per control it is shown.
	controlsSent int
}

func newShadowTally() *shadowTally {
	return &shadowTally{byKind: map[string]*[2]int{}}
}

// askTheInterpreter puts one case to the interpreter and scores the proposal.
//
// `marcoWasRight` is what Marco's own answer scored, so ADD and BREAK can be attributed. The
// interpreter is never told it.
func (t *shadowTally) askTheInterpreter(w *directorapi.WorldState, c rewordCase,
	marcoWasRight bool, marcoRefused bool) (string, string) {

	// NOT ASKED AT ALL when nothing is configured, rather than asked and recorded as an
	// abstention. The difference matters in the report: forty abstentions reads as an
	// interpreter that knew nothing, and "not asked" reads as what it is.
	if !meaning.Available() {
		return "", ""
	}
	controls := meaning.Candidates(w)
	t.controlsSent += len(controls)

	started := time.Now()
	proposal, ok := meaning.Ask(context.Background(), w, phrasingOf(c))
	t.latency = append(t.latency, time.Since(started))
	t.asked++

	if t.byKind[c.Kind] == nil {
		t.byKind[c.Kind] = &[2]int{}
	}
	tally := t.byKind[c.Kind]
	tally[1]++

	if !ok {
		t.abstained++
		// An abstention is only a shortfall where the control WAS there to be found.
		if c.Means.Label != "" {
			return "abstained", fmt.Sprintf("meant %s %q", c.Means.Role, c.Means.Label)
		}
		tally[0]++
		return "abstained", "correctly, nothing here fits"
	}

	el, exists := w.Element(proposal.ElementID)
	if !exists {
		// Cannot happen — meaning.Ask checks the world — and is reported rather than
		// ignored, because a proposal naming nothing is the failure this whole
		// arrangement is shaped to make impossible.
		t.wrong++
		return "INVENTED A CONTROL", string(proposal.ElementID)
	}
	named := fmt.Sprintf("%s %q", el.Role, el.Label)

	switch {
	case c.Means.Absent:
		t.proposedTheAbsent++
		t.wrong++
		if marcoRefused {
			t.gatedLoss = append(t.gatedLoss, c.Say)
		}
		return "PROPOSED THE ABSENT", named + " — " + proposal.Because
	case c.Means.Ambiguous:
		t.brokeAmbiguity++
		return "ANSWERED AN AMBIGUITY", named + " — " + proposal.Because
	case c.Means.Satisfied:
		// Nothing to score: the right answer is that no control needs pressing, and
		// this question cannot express that. Counted as asked and nothing else.
		tally[1]--
		return "not scored", "an already-satisfied request has no right control"
	case strings.EqualFold(strings.TrimSpace(el.Label), strings.TrimSpace(c.Means.Label)):
		t.correct++
		tally[0]++
		if !marcoWasRight {
			t.wouldAdd = append(t.wouldAdd, c.Say)
		}
		if marcoRefused {
			t.gatedGain = append(t.gatedGain, c.Say)
		}
		return "correct", named + " — " + proposal.Because
	default:
		t.wrong++
		if marcoWasRight {
			t.wouldBreak = append(t.wouldBreak, c.Say)
		}
		return "WRONG CONTROL", fmt.Sprintf("meant %s %q, proposed %s — %s",
			c.Means.Role, c.Means.Label, named, proposal.Because)
	}
}

// phrasingOf is what the interpreter is asked, which is the person's own words and the verb.
//
// The target phrase as the planner would carry it, with the verb in front — because "open
// bluetooth settings" and "turn off bluetooth" mean different controls and a target phrase
// alone cannot say which.
func phrasingOf(c rewordCase) string {
	verb := c.Verb
	if verb == "" {
		verb = string(directorapi.SemanticOpen)
	}
	return strings.ReplaceAll(verb, "_", " ") + " " + c.Say
}

// report prints the interpreter's own column.
func (t *shadowTally) report(out io.Writer) {
	fmt.Fprintf(out, "\n  INTERPRETER (shadow — nothing below was acted on)\n")
	if t.asked == 0 {
		fmt.Fprintln(out, "    not asked: $MARCO_INTERPRETER is unset")
		return
	}
	// WHICH MODEL THIS WAS, so a result is self-describing. Read from the environment
	// rather than asked of the plugin: the protocol has no field for it, and inventing one
	// would be the engine taking an interest in which model answered — which is exactly
	// what the one-executable boundary exists to avoid. A plugin that ignores the variable
	// makes this line wrong, which is why it says "told to use" rather than "used".
	if m := os.Getenv("MARCO_LLM_MODEL"); m != "" {
		fmt.Fprintf(out, "    the plugin was told to use %s\n", m)
	}
	scored := t.asked - t.notScored()
	fmt.Fprintf(out, "    %-30s %3d\n", "asked", t.asked)
	fmt.Fprintf(out, "    %-30s %3d  (%.0f%%)\n", "correct", t.correct,
		100*float64(t.correct)/float64(max(scored, 1)))
	fmt.Fprintf(out, "    %-30s %3d\n", "WRONG CONTROL proposals", t.wrong)
	fmt.Fprintf(out, "    %-30s %3d\n", "of those, absent on screen", t.proposedTheAbsent)
	fmt.Fprintf(out, "    %-30s %3d\n", "ambiguities answered", t.brokeAmbiguity)
	fmt.Fprintf(out, "    %-30s %3d  (%.0f%%)\n", "abstained", t.abstained,
		100*float64(t.abstained)/float64(max(t.asked, 1)))
	fmt.Fprintf(out, "    %-30s %3d  %s\n", "WOULD ADD", len(t.wouldAdd),
		strings.Join(t.wouldAdd, ", "))
	fmt.Fprintf(out, "    %-30s %3d  %s\n", "WOULD BREAK", len(t.wouldBreak),
		strings.Join(t.wouldBreak, ", "))
	// THE ONLY ARRANGEMENT THAT COULD EVER BE SAFE, measured on its own. Consulted only
	// where the deterministic reading found NOTHING, an interpreter cannot override a
	// correct answer and cannot settle a tie — it is never asked about either. What is
	// left is these two numbers, and a model with any loss at all has turned an honest
	// refusal into a wrong action.
	fmt.Fprintf(out, "    %-30s +%d / -%d\n", "if asked only where ABSENT",
		len(t.gatedGain), len(t.gatedLoss))
	if len(t.gatedGain) > 0 {
		fmt.Fprintf(out, "      %-28s %s\n", "gained", strings.Join(t.gatedGain, ", "))
	}
	if len(t.gatedLoss) > 0 {
		fmt.Fprintf(out, "      %-28s %s\n", "LOST", strings.Join(t.gatedLoss, ", "))
	}

	kinds := make([]string, 0, len(t.byKind))
	for k := range t.byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	fmt.Fprintln(out, "    by phrasing:")
	for _, k := range kinds {
		fmt.Fprintf(out, "      %-22s %d/%d\n", k, t.byKind[k][0], t.byKind[k][1])
	}
	fmt.Fprintf(out, "    cost  %s/request mean, %s worst, %d controls described "+
		"(%d per request)\n",
		mean(t.latency).Round(time.Millisecond), worst(t.latency).Round(time.Millisecond),
		t.controlsSent, t.controlsSent/max(t.asked, 1))
}

// notScored is the already-satisfied cases, which this question cannot be right about.
func (t *shadowTally) notScored() int {
	total := 0
	for _, v := range t.byKind {
		total += v[1]
	}
	return t.asked - total
}
