package acceptance

import (
	"fmt"
	"strings"
)

// What a failed acceptance tells an agent, and why it is composed rather than collected.
//
// # The report is the Director's own account
//
// Every stage below already exists: `director trace --json` keeps the ladder each command walked,
// with a detail and an ok for each rung, because a person debugging a refusal needed it long before
// this package existed. The report is that ladder plus the oracle, arranged.
//
// Writing a second diagnostic here would mean two accounts of one command, and the one nobody
// edits would be the one in the failure message. So nothing in this file decides anything or
// measures anything — it reads what production recorded and says which rung broke.
//
// # And what it deliberately does not contain
//
// No screenshot, no OCR dump, no accessibility tree. A first report has one job: tell an agent
// WHERE the vertical broke, in few enough lines that reading it is cheaper than reproducing the
// run. The tree is one call away — `Harness.Perceived()` — for the second question, once the stage
// is known.

// Stage is one phase of the Director's own run, as it records it.
//
// # Why phases and not the stage ladder with its details
//
// The ladder with a sentence per rung — `! resolve  nothing matching "Save" is present` — exists in
// the RESPONSE to `director execute`, which is a developer verb. Acceptance has to enter through
// `marco do`, and what `marco do` prints is the result word and the Director's one-line message.
//
// What IS available post-hoc is the phase trace the Director keeps for every command, and it
// answers the question a first report has to answer: HOW FAR did it get. Phases appear as they are
// entered, so the last one present is where it stopped — and the message says why.
//
// Reading `director execute --json` instead would mean running the request a second time, against a
// world Marco had already changed. A report that re-ran the thing it was reporting on would
// sometimes describe a different event.
type Stage struct {
	Name  string `json:"phase"`
	State string `json:"state"`
}

// Reached reports whether this phase ran at all.
func (s Stage) Reached() bool { return s.Name != "" }

// Trace is the Director's account of one command: which phases it entered, and how it ended.
type Trace struct {
	Phrase   string  `json:"phrase"`
	Terminal string  `json:"terminal"`
	Phases   []Stage `json:"phases"`
}

// State is the terminal word the Director ended on.
func (t Trace) State() string { return t.Terminal }

// Result is everything knowable about one request.
type Result struct {
	Request string
	// Output is what the person would have seen, and Exit what a script would have read.
	Output string
	Exit   int
	// Trace is the Director's own ladder.
	Trace Trace
	// Before and After are the oracle either side of the attempt.
	Before, After State
	// OfflineDeclined says the deterministic parser could not read this phrase, so any intent
	// that exists came from the interpreter. Asked of the parser directly — see Reading.
	OfflineDeclined bool
}

// Reading says which reader produced the intent.
//
// # Why it is asked of the parser and of the intake, not inferred from the trace
//
// Both readers produce the SAME recorded intent — deliberately, since they share one construction
// site — so nothing downstream can tell them apart. Two questions answer it between them:
//
//	did the INTAKE send this to the Director at all, or did a Play claim it?
//	would the OFFLINE PARSER have read it, or did the interpreter?
//
// The first is in `marco do`'s own routing line. The second is asked of the parser directly, after
// the fact, which classifies the request and changes nothing about the run.
func (r Result) Reading() string {
	switch {
	case strings.Contains(r.Output, "decision=play"):
		return "play"
	case !strings.Contains(r.Output, "decision=director"):
		// Neither a play nor the Director: the intake refused before anything read it.
		return "none"
	case r.OfflineDeclined && r.Message() != "" && !r.Understood():
		return "refused"
	case r.OfflineDeclined:
		return "interpreted"
	default:
		return "deterministic"
	}
}

// Understood reports whether anything produced a reading at all.
//
// The Director's own sentence for "nothing read this" is the parser's vocabulary list, which is the
// one message that means the request never became an operation.
func (r Result) Understood() bool {
	return !strings.Contains(r.Output, "I only understand")
}

// Message is the Director's own one-line account, as a person is shown it.
//
// Taken from the output rather than from a field, because the output is what the product actually
// said — and an acceptance that reported an internal message a user never saw would be reporting a
// different product.
func (r Result) Message() string {
	for _, l := range strings.Split(r.Output, "\n") {
		t := strings.TrimSpace(l)
		for _, p := range []string{"FAILED: ", "REFUSED: ", "CLARIFY: ", "UNAVAILABLE: "} {
			if strings.HasPrefix(t, p) {
				return strings.TrimPrefix(t, p)
			}
		}
	}
	return ""
}

// Reached reports whether the Director entered one phase of its own run.
func (r Result) Reached(phase string) bool {
	for _, p := range r.Trace.Phases {
		if p.Name == phase {
			return true
		}
	}
	return false
}

// Grounded, Authorised, Executed and Verified read how far the run got.
//
// Each is "did the Director enter this phase", and the WHY for all of them is the one message the
// Director gave — because that is what exists post-hoc through the product's own entrance. A rung
// reported with an invented reason would be the second diagnostic this file exists not to be.
// The names are the Director's own, from trace.CommandPhase. Spelled wrongly at first — "execute"
// rather than "runtime_execute" — and the consequence was a report that said
// `LEGAL_MARCO no · not reached` about a request that had plainly executed, which is worse than no
// report at all: it accuses the wrong rung.
//
// `policy` is genuinely absent from a successful run's phases and that is not a gap in the report:
// it is only timed when there is a decision to make. So authority reads "not reached" on an action
// nothing had to be weighed about, which is the honest reading.
func (r Result) Grounded() (bool, string)   { return r.rung("resolve") }
func (r Result) Authorised() (bool, string) { return r.rung("policy") }
func (r Result) Executed() (bool, string)   { return r.rung("runtime_execute") }
func (r Result) Verified() (bool, string)   { return r.rung("verify") }

// Performed reports whether the product said the thing happened.
//
// The six-word vocabulary, read off the output a person sees rather than off an internal state, so
// a result that reads "performed" to the harness is one that read "performed" to them.
func (r Result) Performed() bool {
	return strings.Contains(r.Output, "[result] performed")
}

// Refused reports whether Marco declined rather than failing at it. Different facts, and the
// product keeps them apart in one word each.
func (r Result) Refused() bool {
	return strings.Contains(r.Output, "[result] refused") ||
		strings.Contains(r.Output, "[result] clarify")
}

// Cancelled reports whether the Audience stopped it.
func (r Result) Cancelled() bool { return strings.Contains(r.Output, "[result] cancelled") }

// Outcome is the one word the product ended on.
func (r Result) Outcome() string {
	for _, w := range []string{"performed", "clarify", "refused", "unavailable",
		"cancelled", "failed"} {
		if strings.Contains(r.Output, "[result] "+w) {
			return w
		}
	}
	return "(no result word)"
}

// FailureStage is the phase the run stopped in, or empty when it succeeded.
//
// The LAST phase entered, because phases are entered in order and the one it stopped in is the one
// that did not let it go further. A request that never reached the Director has no phases at all,
// which is its own answer and a different investigation.
func (r Result) FailureStage() string {
	if r.Performed() {
		return ""
	}
	if len(r.Trace.Phases) == 0 {
		if !r.Understood() {
			return "intent — nothing read the request"
		}
		return "before the Director — the intake did not route it"
	}
	return r.Trace.Phases[len(r.Trace.Phases)-1].Name
}

// Report renders the structured account. One screen, no dumps.
func (r Result) Report(oracle string) string {
	var b strings.Builder
	f := func(k, v string) {
		if strings.TrimSpace(v) == "" {
			v = "—"
		}
		fmt.Fprintf(&b, "  %-14s %s\n", k, v)
	}
	b.WriteString("\n")
	f("REQUEST", fmt.Sprintf("%q", r.Request))
	f("PARSE", r.Reading())
	f("INTENT", r.phases())
	f("WORLD_BEFORE", r.Before.Summary())
	f("GROUNDING", rung(r.Grounded()))
	f("AUTHORITY", rung(r.Authorised()))
	f("LEGAL_MARCO", rung(r.Executed()))
	f("WORLD_AFTER", r.After.Summary())
	f("MARCO_VERDICT", fmt.Sprintf("%s · %s", r.Outcome(), rung(r.Verified())))
	f("ORACLE_VERDICT", oracle)
	f("FAILURE_STAGE", r.FailureStage())
	f("SAID", r.Message())
	return b.String()
}

// phases is the ladder as one line: what the Director actually entered.
func (r Result) phases() string {
	if len(r.Trace.Phases) == 0 {
		return "(the Director recorded no phases)"
	}
	names := make([]string, 0, len(r.Trace.Phases))
	for _, p := range r.Trace.Phases {
		names = append(names, p.Name)
	}
	return strings.Join(names, " → ")
}

// Summary is the part of the oracle a reader needs: what changed, not every field.
func (s State) Summary() string {
	if !s.Read {
		return "(not read)"
	}
	parts := []string{fmt.Sprintf("status=%q", strings.TrimPrefix(s.Status, "Status: "))}
	if s.Name != "" {
		parts = append(parts, fmt.Sprintf("name=%q", s.Name))
	}
	if s.Notes != "" {
		parts = append(parts, fmt.Sprintf("notes=%q", s.Notes))
	}
	if s.Remember {
		parts = append(parts, "remember=true")
	}
	if s.Selected != "" {
		parts = append(parts, fmt.Sprintf("selected=%q", s.Selected))
	}
	if s.SecondView {
		parts = append(parts, "second_view=true")
	}
	parts = append(parts, fmt.Sprintf("presses save=%d cancel=%d details=%d",
		s.SavePresses, s.CancelPresses, s.DetailsPresses))
	if s.Stubborn {
		parts = append(parts, "STUBBORN")
	}
	return strings.Join(parts, " · ")
}

// rung reads one phase as (entered, why).
func (r Result) rung(phase string) (bool, string) {
	if r.Reached(phase) {
		return true, ""
	}
	// NOT REACHED is not the same as FAILED, and saying so is the difference between
	// "authority refused this" and "nothing ever asked for authority because grounding had
	// already stopped".
	return false, "not reached"
}

func rung(ok bool, why string) string {
	if ok {
		if why == "" {
			return "ok"
		}
		return "ok · " + why
	}
	if why == "" {
		return "no"
	}
	return "no · " + why
}
