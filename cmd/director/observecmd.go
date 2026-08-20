package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// The passive observation commands.
//
//	director observe-game --application X --duration 3m   watch a window while you play
//	director observation-sessions                          what has been watched
//	director observation-session <id>                      live progress, or the outcome
//	director observation-insights <id>                     the evidence, then the guesses
//	director cancel-observation <id>                       stop early, keep the evidence
//
// Starting returns immediately: the service does the watching, so a three-minute session
// does not hold a terminal for three minutes.

// runObserveGame is `director observe-game`.
func runObserveGame(args []string) int {
	fs := flag.NewFlagSet("observe-game", flag.ExitOnError)
	windowID := fs.String("window-id", "", "watch the window with this ephemeral id")
	title := fs.String("window-title", "", "watch the window whose title contains this")
	application := fs.String("application", "", "watch this application's window")
	processID := fs.Int("process", 0, "watch this process's window")
	duration := fs.Duration("duration", observe.DefaultDuration, "how long to watch")
	interval := fs.Duration("interval", observe.DefaultInterval, "gap between samples")
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	target := windowref.Selector{
		EphemeralID: *windowID, Title: *title,
		Application: *application, ProcessID: uint32(*processID),
	}
	if err := target.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 2
	}

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	started, err := client.Observe(service.ObservePayload{
		Target: target, Duration: *duration, Interval: *interval,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(started)
	}
	fmt.Printf("Observation session started: %s\n", started.ID)
	fmt.Printf("Target: %s\n", started.Target)
	fmt.Printf("Duration: %s\n", started.Duration)
	fmt.Printf("Interval: %s\n", started.Interval)
	fmt.Printf("\nPlay normally. The Director is watching and will not touch anything.\n")
	fmt.Printf("Progress: director observation-session %s\n", started.ID)
	return 0
}

// runObservationSessions is `director observation-sessions`.
func runObservationSessions(args []string) int {
	return observationQuery(args, service.ObserveQuery{List: true}, renderObservationList)
}

// runObservationSession is `director observation-session <id>`.
func runObservationSession(args []string) int {
	id, rest := firstPositional(args)
	return observationQuery(rest, service.ObserveQuery{ID: id}, renderObservationSession)
}

// runObservationInsights is `director observation-insights <id>`.
func runObservationInsights(args []string) int {
	id, rest := firstPositional(args)
	return observationQuery(rest, service.ObserveQuery{ID: id, Insights: true}, renderInsights)
}

// runAnswer is `director answer <session> <proposal> yes|no|not-now`.
//
// The three words are kept apart at the CLI as deliberately as they are in the type: "not-now"
// is a decision not to answer and is never folded into "no". A user who is busy has not told
// Marco its interpretation is wrong.
func runAnswer(args []string) int {
	positional := make([]string, 0, 3)
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
		}
	}
	if len(positional) < 3 {
		fmt.Fprintln(os.Stderr,
			"usage: director answer <session> <question-id> yes|no|not-now")
		return 2
	}
	resp, ok := responseFor(positional[2])
	if !ok {
		fmt.Fprintf(os.Stderr, "%q is not an answer. Say yes, no, or not-now.\n", positional[2])
		return 2
	}

	return observationQuery(nil, service.ObserveQuery{
		ID: positional[0],
		Answer: &service.ObserveAnswer{
			ProposalID: positional[1], Response: string(resp),
		},
	}, renderAnswered)
}

// runNameScreen is `director name-screen <session> <question-id> "the pause menu"`.
//
// A SEPARATE command from `answer`, deliberately. Every other question this system asks is
// answered with one of three words; this one is answered with the user's own. Folding it into
// `answer` would mean the generic command had to accept arbitrary text for any question, which is
// how a closed vocabulary quietly stops being one.
func runNameScreen(args []string) int {
	positional := make([]string, 0, 3)
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
		}
	}
	if len(positional) < 3 {
		fmt.Fprintln(os.Stderr,
			`usage: director name-screen <session> <question-id> "what you call it"`)
		return 2
	}
	// The raw string travels as a raw string exactly this far. It becomes a ScreenName at the
	// request boundary in Runtime.Observation, which is where validation and provenance are
	// established for every caller, not only this one.
	return observationQuery(nil, service.ObserveQuery{
		ID: positional[0],
		Name: &service.ObserveScreenName{
			ProposalID: positional[1], Name: positional[2],
		},
	}, renderNamed)
}

// renderNamed confirms simply, and says what changed.
func renderNamed(raw json.RawMessage) string {
	var v observationView
	if err := json.Unmarshal(raw, &v); err != nil || v.Answered == nil {
		return "Noted.\n"
	}
	return fmt.Sprintf("Noted — Marco will call that screen %q.\n\n"+
		"That is a name for a screen and nothing else. It does not teach Marco to do "+
		"anything\nthere, and it does not give it permission to.\n", v.Answered.Named)
}

// responseFor maps the words a person types onto the closed vocabulary.
func responseFor(s string) (observe.UserResponse, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yes", "y", "confirm", "confirmed", "right":
		return observe.ResponseConfirmed, true
	case "no", "n", "wrong", "contradict", "contradicted":
		return observe.ResponseContradicted, true
	case "not-now", "notnow", "not-sure", "notsure", "unsure", "later", "skip",
		"decline", "declined":
		return observe.ResponseDeclined, true
	}
	return "", false
}

// renderAnswered reports which question the answer landed on, and what it changed.
func renderAnswered(raw json.RawMessage) string {
	var v observationView
	if err := json.Unmarshal(raw, &v); err != nil || v.Answered == nil {
		return "Answer recorded.\n"
	}
	p := *v.Answered
	var b strings.Builder
	fmt.Fprintf(&b, "Recorded against: %s\n", p.Question)
	switch p.Response {
	case observe.ResponseConfirmed:
		b.WriteString("\nMarco will treat this as confirmed. The observations that " +
			"disagreed, if any,\nremain in the record — a confirmation is strong evidence, " +
			"not a decision to stop\nnoticing.\n")
	case observe.ResponseContradicted:
		b.WriteString("\nMarco will treat its interpretation as contradicted. The " +
			"observations that supported\nit remain in the record and are still reported: " +
			"you disagreeing with them is the\nfinding, not their deletion.\n")
	case observe.ResponseDeclined:
		b.WriteString("\nNoted as declined. This is NOT recorded as evidence against the " +
			"interpretation.\nMarco will not ask again unless the evidence changes shape.\n")
	}
	return b.String()
}

// runCancelObservation is `director cancel-observation [id]`.
func runCancelObservation(args []string) int {
	id, rest := firstPositional(args)
	return observationQuery(rest, service.ObserveQuery{ID: id, Cancel: true},
		func(raw json.RawMessage) string {
			var v observationView
			if err := json.Unmarshal(raw, &v); err != nil {
				return "Cancelled.\n"
			}
			return fmt.Sprintf("Cancelling %s. The evidence collected so far is kept, and "+
				"the session is recorded as INCOMPLETE.\n", v.ID)
		})
}

// observationQuery runs one read against the service and renders it.
func observationQuery(args []string, q service.ObserveQuery,
	render func(json.RawMessage) string) int {

	fs := flag.NewFlagSet("observation", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))
	return observationRequest(*jsonOut, q, render)
}

// observationRequest is observationQuery with the arguments ALREADY parsed.
//
// It exists because a caller that has its own flags cannot hand its arguments on to be parsed a
// second time, and `rehearse` did exactly that: it declared -step and -live, parsed them, and then
// passed the SAME unconsumed slice to observationQuery, whose flag set knows only -json. Every
// invocation carrying a flag therefore died with "flag provided but not defined: -live" — so the
// only path to learned input could not be reached from the command line at all, while
// `rehearse` with no flags worked and hid it.
//
// Splitting the parse from the request is what makes the mistake unavailable rather than merely
// fixed: a caller with its own flags passes its own decision, and there is no argument slice left
// over to be re-parsed.
func observationRequest(jsonOut bool, q service.ObserveQuery,
	render func(json.RawMessage) string) int {

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	raw, err := client.Observation(q)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if jsonOut {
		fmt.Println(string(raw))
		return 0
	}
	fmt.Print(render(raw))
	return 0
}

// firstPositional pulls a leading non-flag argument out.
func firstPositional(args []string) (string, []string) {
	for i, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a, append(append([]string{}, args[:i]...), args[i+1:]...)
		}
	}
	return "", args
}

// ── rendering ─────────────────────────────────────────────────────────────────

func renderObservationList(raw json.RawMessage) string {
	var views []observationView
	if err := json.Unmarshal(raw, &views); err != nil || len(views) == 0 {
		return "No observation sessions.\n" +
			"Start one with: director observe-game --application <name> --duration 3m\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d session(s). These live in memory only and are lost when the "+
		"service restarts.\n\n", len(views))
	fmt.Fprintf(&b, "  %-12s %-18s %-20s %-9s %s\n", "ID", "TARGET", "STATE", "SAMPLES", "ELAPSED")
	for _, v := range views {
		fmt.Fprintf(&b, "  %-12s %-18s %-20s %-9d %s\n",
			v.ID, truncate(v.Application, 18), v.State, v.Samples,
			v.Elapsed.Round(time.Second))
	}
	return b.String()
}

func renderObservationSession(raw json.RawMessage) string {
	var v observationView
	if err := json.Unmarshal(raw, &v); err != nil {
		return "Could not read that session.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Session: %s\n", v.ID)
	fmt.Fprintf(&b, "State: %s\n", v.State)
	fmt.Fprintf(&b, "Target: %s", v.Selector)
	if v.Application != "" {
		fmt.Fprintf(&b, " → %s", v.Application)
	}
	b.WriteString("\n")
	if len(v.Generations) > 0 {
		fmt.Fprintf(&b, "Window generations: %v\n", v.Generations)
	}
	fmt.Fprintf(&b, "Elapsed: %s / %s\n", v.Elapsed.Round(time.Second), v.Duration)
	fmt.Fprintf(&b, "Samples: %d", v.Samples)
	if v.Skipped > 0 {
		fmt.Fprintf(&b, "   skipped: %d", v.Skipped)
	}
	if v.Late > 0 {
		fmt.Fprintf(&b, "   late: %d", v.Late)
	}
	b.WriteString("\n")
	if v.LabelPasses > 0 {
		fmt.Fprintf(&b, "Label passes: %d\n", v.LabelPasses)
	}
	// The pointer's own account, on the MAIN report rather than inside the vision
	// experiment's block. A press that was placed and then named nothing is what decides
	// whether a click can be learned, and that block does not print at all on a default
	// Director — so the one number that mattered was unreachable exactly where it was
	// needed. `on offer` separates "nothing was offered" from "it landed on nothing".
	if in := v.Stats.Shadow.Input; in.PointerResolved+in.PointerUnnamed+
		in.PointerUnresolved > 0 {
		fmt.Fprintf(&b, "Pointer: %d named a control, %d landed on one whose name is "+
			"withheld, %d landed on nothing (%d control(s) on offer)\n",
			in.PointerResolved, in.PointerUnnamed, in.PointerUnresolved,
			in.ControlsOffered)
	}

	// The target provenance guard's verdict over the session.
	//
	// Reported even when it never fired, because "the guard ran and admitted everything"
	// and "the guard never ran" are different facts and only one of them is reassuring —
	// and telling them apart is precisely what this session is for.
	if v.Stats.ProvenanceRefusals == 0 {
		if len(v.Stats.ProvenProviders) == 0 {
			b.WriteString("Provenance: no target-scoped provider contributed evidence\n")
		} else {
			b.WriteString("Provenance: every contributing provider proved its target\n")
			for name, gens := range v.Stats.ProvenProviders {
				fmt.Fprintf(&b, "  %-16s generation %v\n", name, gens)
			}
		}
	} else {
		fmt.Fprintf(&b, "Provenance: %d of %d samples refused evidence (%d observations "+
			"quarantined)\n", v.Stats.ProvenanceRefusals, v.Samples,
			v.Stats.ProvenanceQuarantined)
		for name, reason := range v.Stats.RefusedProviders {
			fmt.Fprintf(&b, "  %-16s %s\n", name, reason)
		}
	}
	renderProposals(&b, v.ID, v.Proposals)
	renderMemory(&b, v)
	renderRelationships(&b, v)
	renderLearning(&b, v)
	renderLearnedDemonstration(&b, v)
	renderAssessment(&b, v)
	renderFollowUps(&b, v)
	renderRehearsals(&b, v)
	renderHypotheses(&b, v.Hypotheses)
	// The experiment, beside the evidence and never mixed into it.
	renderShadow(&b, v.Stats.Shadow)
	if v.Reason != "" {
		fmt.Fprintf(&b, "\n%s\n", v.Reason)
	}
	if !v.Active && !v.Complete {
		b.WriteString("\nThis session is INCOMPLETE. Its evidence is real but its coverage " +
			"is not what was asked for.\n")
	}
	return b.String()
}

// renderInsights prints what was observed and, separately, what it might mean.
//
// The separation is the whole contract: a reader must never have to work out which of two
// adjacent sentences is established and which is a guess.
func renderInsights(raw json.RawMessage) string {
	var v observationView
	if err := json.Unmarshal(raw, &v); err != nil {
		return "Could not read that session.\n"
	}
	if v.Active {
		return fmt.Sprintf("Session %s is still observing (%d samples, %s of %s).\n"+
			"Insights are produced when it finishes.\n",
			v.ID, v.Samples, v.Elapsed.Round(time.Second), v.Duration)
	}

	var b strings.Builder
	if v.Complete {
		fmt.Fprintf(&b, "Observation session %s complete\n", v.ID)
	} else {
		fmt.Fprintf(&b, "Observation session %s ended as %s — INCOMPLETE\n", v.ID, v.State)
		if v.Reason != "" {
			fmt.Fprintf(&b, "  %s\n", v.Reason)
		}
	}
	fmt.Fprintf(&b, "\nSamples: %d taken, %d skipped, over %s\n",
		v.Samples, v.Skipped, v.Elapsed.Round(time.Second))

	if v.Findings == nil {
		b.WriteString("\nNo evidence was collected.\n")
		return b.String()
	}
	f := *v.Findings

	b.WriteString("\nObserved\n")
	if len(f.Stable) == 0 {
		b.WriteString("  nothing stayed put long enough to be called stable\n")
	}
	for _, s := range f.Stable {
		fmt.Fprintf(&b, "  %-10s %-22s present in %3.0f%% of samples, %s\n",
			s.Role, s.Label.Describe(), s.PresenceRatio*100, describeRegionText(s.Region))
	}

	if len(f.Transitions) > 0 {
		b.WriteString("\nChanges\n")
		counts := map[observe.Kind]int{}
		for _, t := range f.Transitions {
			counts[t.Kind]++
		}
		for _, k := range sortedKinds(counts) {
			fmt.Fprintf(&b, "  %-34s %d\n", k, counts[k])
		}
	}

	if len(f.Unstable) > 0 {
		b.WriteString("\nUnreliable evidence\n")
		shown := 0
		for _, s := range f.Unstable {
			if shown >= 8 {
				fmt.Fprintf(&b, "  ... and %d more\n", len(f.Unstable)-shown)
				break
			}
			fmt.Fprintf(&b, "  %-10s %-22s %3.0f%% of samples, %d flickers\n",
				s.Role, s.Label.Describe(), s.PresenceRatio*100, s.Flickers)
			shown++
		}
	}

	b.WriteString("\nHypotheses — these are GUESSES, listed separately on purpose\n")
	if len(v.Insights) == 0 {
		b.WriteString("  the evidence did not support any\n")
	}
	for _, in := range v.Insights {
		fmt.Fprintf(&b, "\n  %s  (confidence %.2f)\n", in.Kind, in.Confidence)
		fmt.Fprintf(&b, "    observed:   %s\n", in.Observed)
		for _, c := range in.Contradictions {
			fmt.Fprintf(&b, "    against:    %s\n", c)
		}
		fmt.Fprintf(&b, "    to confirm: %s\n", in.Validation)
	}

	fmt.Fprintf(&b, "\nThresholds: %d+ samples, %.0f%%+ presence, %.2f+ confidence\n",
		f.Thresholds.MinSamples, f.Thresholds.MinPresence*100, f.Thresholds.MinConfidence)
	return b.String()
}

func sortedKinds(counts map[observe.Kind]int) []observe.Kind {
	out := make([]observe.Kind, 0, len(counts))
	for k := range counts {
		out = append(out, k)
	}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// describeRegionText names a region in words, mirroring the analysis core's own phrasing.
func describeRegionText(r observe.Region) string {
	vertical := []string{"upper", "middle", "lower"}[clampThird(r.Y)]
	horizontal := []string{"left", "centre", "right"}[clampThird(r.X)]
	if vertical == "middle" && horizontal == "centre" {
		return "the centre"
	}
	return "the " + vertical + " " + horizontal
}

func clampThird(f float64) int {
	v := int(f * 3)
	switch {
	case v < 0:
		return 0
	case v > 2:
		return 2
	}
	return v
}

// runBenchmarkVision is `director benchmark-vision <session-id>`.
//
// Scores a completed session's evidence for SEMANTIC usefulness rather than volume. It
// re-reads a session the service already has, so the same evidence can be scored repeatedly
// with no game running — which is the point: two backends are comparable only against
// identical evidence.
func runBenchmarkVision(args []string) int {
	id, rest := firstPositional(args)
	return observationQuery(rest, service.ObserveQuery{ID: id, Insights: true},
		func(raw json.RawMessage) string {
			var v observationView
			if err := json.Unmarshal(raw, &v); err != nil {
				return "Could not read that session.\n"
			}
			if v.Findings == nil {
				return "That session has no findings to score.\n" +
					"Only a finished session can be benchmarked.\n"
			}
			backend := v.Application
			if backend == "" {
				backend = "unknown"
			}
			m := observe.Measure("icon_detect", "icon_detect.onnx", *v.Findings)
			var b strings.Builder
			fmt.Fprintf(&b, "Detector benchmark — session %s over %s\n\n", v.ID, backend)
			b.WriteString(observe.Compare([]observe.Metrics{m},
				observe.DefaultSelectionThresholds()))
			return b.String()
		})
}

// renderProposals puts Marco's open questions to the reader.
//
// # Why the question comes before the evidence
//
// Because it is the only part of this report that asks something of the person reading it.
// Everything below is Marco explaining itself; this is Marco waiting on an answer, and burying
// it under four screens of rectangles would make the loop unusable in exactly the way that keeps
// systems like this from ever getting feedback.
//
// The answer command is printed with the question, because a question somebody cannot see how to
// answer is a statement.
func renderProposals(b *strings.Builder, session observe.SessionID, ps []observe.Proposal) {
	var open, answered []observe.Proposal
	for _, p := range ps {
		if p.Status == observe.ProposalOpen {
			open = append(open, p)
			continue
		}
		answered = append(answered, p)
	}

	if len(open) > 0 {
		b.WriteString("\nMARCO HAS A QUESTION\n")
		for _, p := range open {
			fmt.Fprintf(b, "\n  %s\n", p.Question)
			// A naming question is answered with a word, so the line that says HOW has to
			// say a different thing. A question a reader cannot see how to answer is a
			// statement.
			if p.Ask == observe.AskNameScreen {
				fmt.Fprintf(b,
					"    answer with   director name-screen %s %s \"what you call it\"\n",
					session, p.ID)
				continue
			}
			fmt.Fprintf(b, "    answer with   director answer %s %s yes|no|not-now\n",
				session, p.ID)
		}
		b.WriteString("\n  Answering is optional. \"not now\" is not \"no\" — it records " +
			"that you did not\n  want to answer, and Marco will not ask again unless the " +
			"evidence changes shape.\n")
	}

	if len(answered) > 0 {
		b.WriteString("\n  previously asked\n")
		for _, p := range answered {
			what := string(p.Response)
			if p.Status == observe.ProposalDeclined {
				what = "declined — not treated as a no"
			}
			// WHY nothing was asked matters as much as the answer. A question that
			// silently fails to appear is indistinguishable from one the policy declined
			// to ask, and "I recognised this from an earlier session" is the explanation
			// the user is owed for the silence.
			if p.Recognised {
				what += "  (recognised from an earlier session)"
			}
			fmt.Fprintf(b, "    %-28s %s\n", readableKind(p.Kind), what)
		}
	}
}

// renderMemory reports what cross-session recognition did, and why it did nothing when it did.
//
// Printed even when memory is unavailable, because "Marco did not recognise this screen" and
// "Marco could not read its memory" are different sentences and only one of them is about the
// screen.
func renderMemory(b *strings.Builder, v observationView) {
	var recognised int
	for _, p := range v.Proposals {
		if p.Recognised {
			recognised++
		}
	}
	if v.MemoryUnavailable == "" && recognised == 0 {
		return
	}
	b.WriteString("\n  memory\n")
	if v.MemoryUnavailable != "" {
		fmt.Fprintf(b, "    UNAVAILABLE — %s\n", v.MemoryUnavailable)
		b.WriteString("    nothing was recognised for that reason, not because the " +
			"screens were new\n")
		return
	}
	fmt.Fprintf(b, "    %d interpretation(s) carried over from earlier sessions, so those "+
		"questions\n    were not asked again\n", recognised)
}

// renderRelationships reports what this session's transitions did to the durable topology.
//
// # What this section is careful not to say
//
// It reports adjacency and what was seen around it. It does not say a navigation intent caused
// a change, that performing it would reproduce one, or that Marco knows how to get anywhere —
// and `DescribeRelationship` prints the disclaimer on every edge rather than once at the top,
// because a reader who skims to the interesting edge should still meet it.
//
// It also reports the transitions that stayed SESSION-LOCAL. "No durable edges" has two
// explanations — nothing transitioned, or nothing was recognised — and a section that showed
// only the successes would let the second read as the first.
func renderRelationships(b *strings.Builder, v observationView) {
	r := v.Relationships
	if r.Durable == 0 && r.SessionLocal == 0 && r.Unavailable == "" {
		return
	}
	b.WriteString("\n  remembered topology\n")
	if r.Unavailable != "" {
		fmt.Fprintf(b, "    UNAVAILABLE — %s\n", r.Unavailable)
	}
	if r.Durable > 0 {
		fmt.Fprintf(b, "    %d transition(s) had both ends recognised: %d new, %d "+
			"corroborated\n", r.Durable, r.Created, r.Corroborated)
	}
	if r.SessionLocal > 0 {
		fmt.Fprintf(b, "    %d stayed session-local — one or both ends is a screen Marco "+
			"does not\n    recognise yet, so nothing durable was claimed about them\n",
			r.SessionLocal)
	}
	if r.Rejected > 0 {
		fmt.Fprintf(b, "    %d were refused by the store\n", r.Rejected)
	}
	for _, e := range v.Topology {
		b.WriteString("\n")
		for _, line := range e.Lines {
			fmt.Fprintf(b, "    %s\n", line)
		}
	}
}

// renderHypotheses answers "what is Marco learning while I play".
//
// # What this section is for, and what it must not become
//
// A person watching a session wants one thing: has this been worth leaving running. The state
// table above answers that in rectangles and counts, which is the right record and the wrong
// answer — nobody reads `state_3 inferences 8 episodes 3 button×4` and learns anything.
//
// So this prints the interpretation, its support, and — always, never conditionally — what
// argues against it. A hypothesis section that showed only the confident ones would be the most
// misleading part of the entire report, because a reader would take the absence of doubt as its
// absence in the evidence.
//
// Session-local state ids appear here ONLY as cross-references into the table above. They are
// printed dimly, in parentheses, because they mean nothing outside this session and a reader who
// starts remembering `state_3` is being taught something false.
func renderHypotheses(b *strings.Builder, hs []observe.Hypothesis) {
	if len(hs) == 0 {
		return
	}
	b.WriteString("\nWHAT THIS MIGHT BE — hypotheses from the evidence, not conclusions\n")
	for _, h := range hs {
		fmt.Fprintf(b, "\n  %s   [%s]\n", readableKind(h.Kind), h.Status)
		fmt.Fprintf(b, "    observed     %s\n", h.Observed)
		for _, e := range h.Support {
			fmt.Fprintf(b, "    %-12s %s\n", e.Source, e.Statement)
		}
		// Contradictions are printed with a marker that survives skim-reading. The whole
		// value of this section is that a reader can see the case against.
		for _, e := range h.Contradictions {
			fmt.Fprintf(b, "    AGAINST      %s\n", e.Statement)
		}
		if h.Subject.To != "" {
			fmt.Fprintf(b, "    subject      %s → %s (this session's labels only)\n",
				h.Subject.Ref, h.Subject.To)
		} else {
			fmt.Fprintf(b, "    subject      %s (this session's label only)\n", h.Subject.Ref)
		}
		fmt.Fprintf(b, "    to settle    %s\n", h.Validation)
	}
	b.WriteString("\n  These are guesses with their reasons attached. Marco has taken no " +
		"action on any of them,\n  and knows nothing about which application this was.\n")
}

// readableKind turns a vocabulary constant into something a person reads.
//
// The underscored constant is the durable identifier and belongs in JSON; a report that printed
// `possible_settings_like_state` at somebody would be leaking a schema into a sentence.
func readableKind(k observe.HypothesisKind) string {
	switch k {
	case observe.PossibleChoiceGroup:
		return "possibly a set of choices presented together"
	case observe.PossibleMenuLikeState:
		return "possibly a menu-like screen"
	case observe.PossibleSettingsLikeState:
		return "possibly a settings screen"
	case observe.PossibleTextEntryState:
		return "possibly a screen you type into"
	case observe.PossibleTransitionAction:
		return "possibly the action that causes a change"
	case observe.PossibleReversiblePlace:
		return "possibly somewhere you go and come back from"
	case observe.PossibleSelectionSequence:
		return "possibly a repeated selection sequence"
	}
	return string(k)
}

// renderShadow reports what an experimental provider observed, when one did.
//
// Every line says SHADOW and the section ends by saying it has no authority. A reader must
// never mistake experimental perception for what Marco currently believes, and a report that
// left that ambiguous would undo the structural guarantee that makes the experiment safe to
// run at all.
//
// Silent when no experiment was configured. Printing a row of zeroes would say "ScreenParser
// ran and saw nothing", which is a different and much more alarming claim than "you did not
// enable it".
func renderShadow(b *strings.Builder, s observe.ShadowTotals) {
	if s.Detector == "" && s.Unavailable == "" && s.Opportunities == 0 {
		return
	}
	name := s.Detector
	if name == "" {
		name = "shadow"
	}
	fmt.Fprintf(b, "\n%s · SHADOW — experimental, no authority\n", strings.ToUpper(name))
	if s.Unavailable != "" {
		fmt.Fprintf(b, "  unavailable  %s\n", s.Unavailable)
		return
	}
	if !s.Observed() {
		fmt.Fprintf(b, "  never ran    %d slots offered, all skipped by cadence\n",
			s.Opportunities)
		return
	}
	fmt.Fprintf(b, "  inferences   %d of %d slots   skipped %d   failures %d\n",
		s.Inferences, s.Opportunities, s.Skipped, s.Failures)
	if s.ProvenanceFailures > 0 {
		fmt.Fprintf(b, "  provenance   %d inference(s) could not prove they observed the "+
			"session's window generation\n", s.ProvenanceFailures)
	}
	fmt.Fprintf(b, "  latency      median %dms   p95 %dms   max %dms\n",
		s.MedianMS, s.P95MS, s.MaxMS)
	fmt.Fprintf(b, "  detections   %d total   %d nameable   %d unmapped\n",
		s.Detections, s.Nameable, s.Unknown)
	if len(s.Roles) > 0 {
		roles := make([]string, 0, len(s.Roles))
		for k := range s.Roles {
			roles = append(roles, k)
		}
		sort.Strings(roles)
		var parts []string
		for _, r := range roles {
			parts = append(parts, fmt.Sprintf("%s %d", r, s.Roles[r]))
		}
		fmt.Fprintf(b, "  roles        %s\n", strings.Join(parts, "  "))
	}
	c := s.Comparison
	fmt.Fprintf(b, "  comparison   agreed %d   shadow-only %d   authoritative-only %d\n",
		c.Agreed, c.ShadowOnly, c.AuthoritativeOnly)
	fmt.Fprintf(b, "               role-disagreement %d   geometry-disagreement %d   "+
		"uncomparable %d\n", c.RoleDisagreement, c.GeometryDisagreement, c.Uncomparable)
	fmt.Fprintf(b, "  gain         %d shadow-only NAMEABLE observations authoritative "+
		"perception did not supply\n", c.ShadowOnlyNameable)
	renderShadowStates(b, s)
	renderShadowTracks(b, s)
	renderInputProducer(b, s.Input)
	b.WriteString("  authority    NONE — this evidence cannot reach belief, planning, " +
		"policy or input\n")
}

// renderInputProducer reports what the navigation producer did, in semantic terms only.
//
// It prints even when nothing was classified, and that is the point. A correlation section with
// no attributed edges reads as a finding about the player; very often it is a fact about Marco —
// the hook was never installed, the platform has none, or the bounded queue shed events. This
// section is what makes those distinguishable without reading a log.
//
// No key is named here, because none exists to name: the counters below are the whole
// vocabulary that survives the platform adapter.
func renderInputProducer(b *strings.Builder, in observe.InputStats) {
	if in.Unavailable != "" {
		fmt.Fprintf(b, "  navigation   UNAVAILABLE — %s\n", in.Unavailable)
		fmt.Fprintf(b, "               every transition below is unattributed for that "+
			"reason, not because nothing was pressed\n")
		return
	}
	if in.Received == 0 {
		b.WriteString("  navigation   no physical input reached the producer this session\n")
		return
	}
	fmt.Fprintf(b, "  navigation   %d intents from %d events   %d ignored   %d dropped\n",
		in.Classified, in.Received, in.IgnoredTotal(), in.Dropped)
	if in.Conditional > 0 {
		fmt.Fprintf(b, "               %d of those intents came from keys that are only "+
			"navigation while a set of choices is on screen\n", in.Conditional)
	}
	// The pointer's own account: a press that was PLACED and then named nothing is admitted
	// evidence, not an ignored event, and it is the fact that decides whether a click can be
	// learned. `controls_offered` separates "nothing was on offer" from "it landed on
	// nothing", which are the two causes of the same silence.
	if in.PointerResolved > 0 || in.PointerUnresolved > 0 {
		fmt.Fprintf(b, "               pointer: %d named a control, %d landed on nothing "+
			"(%d control(s) on offer)\n",
			in.PointerResolved, in.PointerUnresolved, in.ControlsOffered)
	}
	if len(in.Ignored) > 0 {
		reasons := make([]string, 0, len(in.Ignored))
		for r := range in.Ignored {
			reasons = append(reasons, string(r))
		}
		sort.Strings(reasons)
		var parts []string
		for _, r := range reasons {
			parts = append(parts, fmt.Sprintf("%s %d", r, in.Ignored[observe.IgnoreReason(r)]))
		}
		fmt.Fprintf(b, "               %s\n", strings.Join(parts, "   "))
	}
	if in.Dropped > 0 {
		fmt.Fprintf(b, "               %d event(s) were shed by the bounded hook queue; "+
			"the correlation below is thinner than what the player did\n", in.Dropped)
	}
}

// renderShadowStates reports the screen states a session was segmented into.
//
// The numbers a reader needs to interpret every presence ratio below it: a track that was
// state-locally perfect across two inferences and one that was perfect across twenty are
// different claims, and only the state's own inference count separates them.
//
// Deliberately terse and bounded. The full model is in JSON; this is the part that changes how
// the track table is read.
func renderShadowStates(b *strings.Builder, s observe.ShadowTotals) {
	if len(s.States) == 0 {
		return
	}
	fmt.Fprintf(b, "  states       %d discovered\n", len(s.States))
	shown := 0
	for _, st := range s.States {
		if shown >= 6 {
			fmt.Fprintf(b, "               ... and %d more\n", len(s.States)-shown)
			break
		}
		roles := make([]string, 0, len(st.Roles))
		for r := range st.Roles {
			roles = append(roles, r)
		}
		sort.Strings(roles)
		var parts []string
		for _, r := range roles {
			parts = append(parts, fmt.Sprintf("%s×%d", r, st.Roles[r]))
		}
		fmt.Fprintf(b, "    %-10s inferences %-3d episodes %-3d tracks %-3d  %s\n",
			st.ID, st.Inferences, st.Episodes, st.Tracks, strings.Join(parts, " "))
		shown++
	}
	if s.EvictedStates > 0 {
		fmt.Fprintf(b, "               %d inference(s) could not be placed at the state "+
			"bound\n", s.EvictedStates)
	}
	// Transitions, counted and never named. "state_1 → state_2 ×3" is evidence a later
	// layer may turn into an action; calling it `pause` here would put an interpretation
	// underneath a measurement.
	if len(s.Transitions) > 0 {
		b.WriteString("  changes      the state graph's edges; navigation is CORRELATED, " +
			"never claimed as the cause\n")
		shown = 0
		for _, tr := range s.Transitions {
			if shown >= 6 {
				fmt.Fprintf(b, "               ... and %d more\n", len(s.Transitions)-shown)
				break
			}
			line := fmt.Sprintf("    %s→%s ×%d", tr.From, tr.To, tr.Count)
			// The support is printed with the intent so one observation and four out of
			// four cannot read the same. An edge nobody was seen to cause says so.
			if intent, n := tr.Dominant(); intent != "" {
				line += fmt.Sprintf("   after %s in %d/%d", intent, n, tr.Count)
			}
			if tr.Unattributed > 0 {
				line += fmt.Sprintf("   no input in %d", tr.Unattributed)
			}
			// Printed beside the support, never folded into it: an edge seen only through
			// keys that also mean movement is a weaker finding than one seen through
			// unambiguous navigation, and the two must not render identically.
			if tr.ConditionalOnly > 0 {
				line += fmt.Sprintf("   context-admitted in %d", tr.ConditionalOnly)
			}
			b.WriteString(line + "\n")
			shown++
		}
	}
	for _, g := range s.Groups {
		fmt.Fprintf(b, "    %-10s %d tracks co-occurring in %s   episodes %d   "+
			"uniformity %.2f\n", g.ID, len(g.Members), g.State, g.Episodes, g.Uniformity)
	}
}

// renderShadowTracks summarises the recurring structures a session discovered.
//
// Bounded on purpose: the high-value tracks, not every one. A diagnostic that dumped dozens of
// rows would bury the few that actually recur, and separating structure that survives time
// from a box that happened once is the entire point of tracking. The full bounded set is in
// JSON for anything that wants it.
func renderShadowTracks(b *strings.Builder, s observe.ShadowTotals) {
	if len(s.Tracks) == 0 {
		return
	}
	var stable, nameableStable, stateStable int
	shapes := map[observe.TemporalShape]int{}
	for _, t := range s.Tracks {
		shapes[t.Shape]++
		if t.Stable() {
			stable++
			if t.Nameable {
				nameableStable++
			}
		}
		if t.StateStable() {
			stateStable++
		}
	}
	fmt.Fprintf(b, "  tracks       %d discovered   %d stable   %d stable+nameable\n",
		len(s.Tracks), stable, nameableStable)
	if len(s.States) > 0 {
		fmt.Fprintf(b, "               %d stable WITHIN their own screen state — the "+
			"figure that matters for a state-dependent control\n", stateStable)
	}
	fmt.Fprintf(b, "               persistent %d  bursty %d  transient %d  rare %d  "+
		"unstable %d\n",
		shapes[observe.ShapePersistent], shapes[observe.ShapeBursty],
		shapes[observe.ShapeTransient], shapes[observe.ShapeRare],
		shapes[observe.ShapeUnstable])
	if s.Evicted > 0 {
		fmt.Fprintf(b, "               %d track(s) evicted at the retention bound\n", s.Evicted)
	}
	shown := 0
	for _, t := range s.Tracks {
		if shown >= 6 || t.Shape == observe.ShapeRare {
			continue
		}
		name := ""
		if t.Nameable {
			name = "  nameable"
		}
		fmt.Fprintf(b, "    %-10s %-7s seen %d/%d  episodes %d  %s  meanIoU %.2f%s\n",
			t.ID, t.Role, t.Seen, t.Eligible, t.Episodes, t.Shape, t.MeanIoU, name)
		// The state-local line, printed only when it says something the global line does
		// not. A menu control reads `bursty 8/16` globally and `persistent 8/8 in state_2`
		// here, and the second is the one a capability would reason about.
		if st, ok := t.PrimaryState(); ok {
			fmt.Fprintf(b, "               in %-10s %s  %d/%d (%.0f%%)\n",
				st.State, st.Shape, st.Seen, st.Eligible, st.PresenceRatio()*100)
		}
		shown++
	}
}

// renderLearning reports what Marco offered to learn, and why it offered nothing else.
//
// # Why the refusals are printed at all
//
// Because silence is the hard case. "Marco did not offer to learn anything" has a dozen
// explanations — too few sessions, no navigation evidence, all of it context-admitted, already
// declined, another question open — and a section that showed only the invitations would leave a
// reader unable to tell a working policy from a broken one.
//
// Every entry carries `authority: none`, unconditionally. A person reading "I've seen you go
// from settings to controller settings — want me to learn how?" could reasonably conclude Marco
// already can, and the one sentence that prevents that belongs beside every one of them rather
// than once at the top.
func renderLearning(b *strings.Builder, v observationView) {
	if len(v.Learning) == 0 {
		return
	}
	b.WriteString("\n  learning\n")
	asked := 0
	for _, e := range v.Learning {
		if e.Eligible {
			asked++
		}
	}
	if asked == 0 {
		b.WriteString("    nothing was offered for learning this session; the reasons are " +
			"below\n")
	}
	for _, e := range v.Learning {
		b.WriteString("\n")
		for _, line := range e.Lines {
			fmt.Fprintf(b, "    %s\n", line)
		}
	}
}

// renderDemonstration reports the bounded demonstration this session watched.
//
// Printed whether or not it completed. An incomplete one's REASON is the useful part — a person
// who said "yes, watch me" and then saw nothing needs to know whether Marco was waiting for them
// to get to the right screen, gave up at a bound, or watched them arrive somewhere else.
//
// `Describe` ends every candidate with "verified: no", unconditionally. A report that showed a
// clean list of steps without it would read as a procedure Marco can run.
func renderLearnedDemonstration(b *strings.Builder, v observationView) {
	if v.Capturing != nil {
		fmt.Fprintf(b, "\n  demonstration (%s)\n", v.Capturing.State)
		fmt.Fprintf(b, "    %d semantic event(s), %d checkpoint(s), %d observation(s)",
			v.Capturing.Events, v.Capturing.Checkpoints, v.Capturing.Inferences)
		if v.Capturing.Skipped > 0 {
			fmt.Fprintf(b, ", %d skipped", v.Capturing.Skipped)
		}
		b.WriteString("\n")
	}
	if len(v.Demonstration) == 0 {
		return
	}
	b.WriteString("\n  demonstration\n")
	for _, line := range v.Demonstration {
		fmt.Fprintf(b, "    %s\n", line)
	}
}

// renderAssessment reports what Marco concludes from the demonstration it watched.
//
// Recomputed every time this is read, so a demonstration whose intermediate screen the user has
// since named reads better today than it did yesterday — without a single new observation. The
// last line disclaims verification unconditionally, because a clean list of verifiable
// checkpoints is exactly what a procedure Marco could run would look like.
func renderAssessment(b *strings.Builder, v observationView) {
	if len(v.Assessment) == 0 {
		return
	}
	b.WriteString("\n  what Marco concludes\n")
	for _, line := range v.Assessment {
		fmt.Fprintf(b, "    %s\n", line)
	}
}

// renderFollowUps reports whether another demonstration would help, and why Marco did not ask.
//
// Printed even when nothing was asked — especially then. A user who agreed to teach Marco
// something and then heard nothing has no way to tell whether Marco is satisfied, stuck or
// broken, and "another demonstration would not help with: requires_text_entry" is the sentence
// that answers it.
func renderFollowUps(b *strings.Builder, v observationView) {
	if len(v.FollowUps) == 0 {
		return
	}
	b.WriteString("\n  another example?\n")
	for _, lines := range v.FollowUps {
		b.WriteString("\n")
		for _, line := range lines {
			fmt.Fprintf(b, "    %s\n", line)
		}
	}
}

// renderRehearsals reports whether Marco may ask to try something itself, and why not.
//
// The most important thing on this screen is the LAST line of every entry, which says that
// nothing here is permission. A user reading "rehearsal: eligible" needs to see, in the same
// breath, that Marco has not done anything and will not until they say so.
//
// No attempt id is printed. The identity of a grant is a claim token for one future experiment,
// and a token that appears in ordinary status output is a token in scrollback, in a screenshot,
// and in a bug report.
func renderRehearsals(b *strings.Builder, v observationView) {
	if len(v.Rehearsals) == 0 && len(v.Authorization) == 0 {
		return
	}
	if len(v.Rehearsals) > 0 {
		b.WriteString("\n  may I try it?\n")
		for _, lines := range v.Rehearsals {
			b.WriteString("\n")
			for _, line := range lines {
				fmt.Fprintf(b, "    %s\n", line)
			}
		}
	}
	if len(v.Authorization) > 0 {
		b.WriteString("\n  you said yes to one attempt\n\n")
		for _, line := range v.Authorization {
			fmt.Fprintf(b, "    %s\n", line)
		}
	}
}

// runRehearse is `director rehearse [session] [--step N] [--live]`.
//
// # Boring on purpose
//
// It takes no internal ids a person would have to copy: there is one outstanding grant at a time,
// and the command finds it. `--live` is the only thing that makes a computer move, it defaults to
// off, and the word appears in the output either way so nobody has to remember what they typed.
//
// This is also the ONLY thing that spends a grant. No session spends one, no review spends one,
// and nothing spends one on a timer.
func runRehearse(args []string) int {
	q, jsonOut := rehearseQuery(args)
	return observationRequest(jsonOut, q, renderRehearsal)
}

// rehearseQuery is the whole of `rehearse`'s argument handling, separated so it can be tested
// without a Director.
//
// The defect it replaces was invisible to every test in the suite because the arguments were
// never turned into a value anything could assert on: they were parsed for their side effects and
// then handed on to be parsed again. `rehearse --live` — the ONLY way to reach learned input —
// failed with "flag provided but not defined: -live" for as long as the flag has existed.
func rehearseQuery(args []string) (service.ObserveQuery, bool) {
	id, rest := firstPositional(args)
	fs := flag.NewFlagSet("rehearse", flag.ExitOnError)
	step := fs.Int("step", 1, "which step of the authorized plan to attempt")
	live := fs.Bool("live", false,
		"actually send the input. Without this, Marco says what it WOULD send and sends nothing")
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(rest))

	return service.ObserveQuery{
		ID:       id,
		Rehearse: &service.ObserveRehearse{Step: *step, Live: *live},
	}, *jsonOut
}

// renderRehearsal reports one attempt.
//
// The first line always says whether anything was actually sent, because that is the fact a reader
// needs before any other. A refusal is not a failed rehearsal — it is Marco declining to try.
func renderRehearsal(raw json.RawMessage) string {
	var v service.RehearsalView
	if err := json.Unmarshal(raw, &v); err != nil {
		return "Rehearsal reported nothing readable.\n"
	}
	var b strings.Builder
	if !v.Attempted {
		b.WriteString("Nothing was sent.\n\n")
		if v.Detail != "" {
			fmt.Fprintf(&b, "  %s\n", v.Detail)
		}
		if v.Refusal != "" {
			fmt.Fprintf(&b, "  reason: %s\n", v.Refusal)
		}
		b.WriteString("\nThe authorization is unchanged unless it had already been claimed.\n")
		return b.String()
	}
	if v.Live {
		b.WriteString("Marco sent real input.\n\n")
	} else {
		b.WriteString("Nothing reached the computer: this ran against a recording host.\n" +
			"Add --live to actually send it.\n\n")
	}
	for _, line := range v.Lines {
		b.WriteString(line + "\n")
	}
	return b.String()
}

// runLearned is `director learned [--application NAME]`.
//
// Shows what Marco has learned well enough to write down, as ordinary Marco. It saves nothing,
// registers nothing and runs nothing — writing a play down and being allowed to perform it are
// different things, and this is only the first.
func runLearned(args []string) int {
	fs := flag.NewFlagSet("learned", flag.ExitOnError)
	application := fs.String("application", "", "which application's routes to write down")
	name := fs.String("name", "", "what to call the play, e.g. Volume")
	verb := fs.String("verb", "", "what it does, e.g. Mute")
	save := fs.Bool("save", false, "keep it as a file you can read and edit")
	register := fs.Bool("register", false, "and let you ask for it by name")
	forget := fs.Bool("forget", false, "remove the play (not what Marco observed)")
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	raw, err := client.Learned(service.LearnedQuery{
		Application: *application, Name: *name, Verb: *verb,
		Save: *save, Register: *register, Forget: *forget,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		fmt.Println(string(raw))
		return 0
	}
	fmt.Print(renderLearned(raw))
	return 0
}

// renderLearned prints each route, and the play for the ones that earned it.
func renderLearned(raw json.RawMessage) string {
	var v service.LearnedView
	if err := json.Unmarshal(raw, &v); err != nil {
		return "Nothing readable came back.\n"
	}
	if len(v.Plays) == 0 {
		return "Marco has not watched anything it could write down yet.\n"
	}
	var b strings.Builder
	for _, p := range v.Plays {
		if !p.Eligible {
			fmt.Fprintf(&b, "\nnot yet:")
			for _, r := range p.Refusals {
				fmt.Fprintf(&b, " %s", r)
			}
			b.WriteString("\n")
			continue
		}
		b.WriteString("\nMarco could write this down:\n\n")
		for _, line := range strings.Split(strings.TrimRight(p.Source, "\n"), "\n") {
			b.WriteString("    " + line + "\n")
		}
	}
	b.WriteString("\nNothing has been saved and nothing can run it. " +
		"Copy it somewhere if you want to keep it.\n")
	return b.String()
}

// runRevise is `director revise <question-id> yes|no|not-sure` and
// `director withdraw <question-id>`.
//
// A SEPARATE command from `answer`, for the same reason it is a separate operation underneath:
// answering stays one-shot so a double submit cannot overwrite what somebody said, and changing
// your mind is something you have to mean. Folding it into `answer` would make an accidental
// second press indistinguishable from a correction.
//
// No session id: a question is found by its own identity across every record, because by the
// time somebody changes their mind they have usually restarted twice.
func runRevise(args []string, withdraw bool) int {
	rev, complaint := reviseRequest(args, withdraw)
	if rev == nil {
		fmt.Fprintln(os.Stderr, complaint)
		return 2
	}
	return observationQuery(nil, service.ObserveQuery{Revise: rev}, renderRevised)
}

// reviseRequest turns the words a person typed into the request, or into a complaint.
//
// A seam, and it exists because the alternative was untestable: the rest of this command is a
// round trip to a running service, so the one decision that matters — WHICH operation the typing
// asked for — had no test. The mutation that sent every `revise` as a withdrawal survived until
// this function existed. Everything that decides what a revision MEANS is still behind the
// service; this only decides what was asked for.
func reviseRequest(args []string, withdraw bool) (*service.ObserveRevise, string) {
	positional := make([]string, 0, 2)
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
		}
	}
	want := 2
	if withdraw {
		want = 1
	}
	if len(positional) < want {
		if withdraw {
			return nil, "usage: director withdraw <question-id>"
		}
		return nil, "usage: director revise <question-id> yes|no|not-sure"
	}

	rev := &service.ObserveRevise{ProposalID: positional[0], Withdraw: withdraw}
	if !withdraw {
		resp, ok := responseFor(positional[1])
		if !ok {
			return nil, fmt.Sprintf(
				"%q is not an answer. Say yes, no, or not-sure.", positional[1])
		}
		rev.Response = string(resp)
	}
	return rev, ""
}

// renderRevised says what the effective judgement is NOW, and nothing about what it was.
//
// Normal surfaces show the current truth; history belongs in a developer reading. Leaving both
// visible would be two answers on screen with nothing to say which one governs.
func renderRevised(raw json.RawMessage) string {
	var v observationView
	if err := json.Unmarshal(raw, &v); err != nil || v.Answered == nil {
		return "Changed.\n"
	}
	p := *v.Answered
	switch {
	case p.Retracted:
		return "Withdrawn. Marco will no longer use your previous answer.\n\n" +
			"What it observed is untouched — withdrawing an answer is not forgetting what " +
			"it saw.\nThe question may come back if the evidence changes shape.\n"
	case p.Response == observe.ResponseConfirmed:
		return "Changed. Marco will treat this as confirmed from now on.\n"
	case p.Response == observe.ResponseContradicted:
		return "Changed. Marco will treat its interpretation as contradicted from now on.\n"
	}
	return "Changed. Marco will not use this as evidence either way.\n"
}

// ── what Marco knows ──────────────────────────────────────────────────────────

// runKnows is `director knows`, and `director knows <subject> <kind> yes|no|withdraw`.
//
// The reading is the point of the command: everything else in this system reaches a judgement
// through the question that produced it, and a question dies with the session that could still
// recognise its subject. This lists what a person has actually TOLD Marco, whether or not anything
// can still find what it was about, and lets them change or take back any of it.
//
// It is not a memory browser. Guesses Marco never asked about do not appear here, because a list
// headed "what you told me" that contained things nobody said would invite people to correct
// something they never claimed.
func runKnows(args []string) int {
	positional := make([]string, 0, 3)
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
		}
	}
	k := &service.ObserveKnows{}
	switch len(positional) {
	case 0:
	case 3:
		k.Subject, k.Kind = positional[0], positional[1]
		if strings.EqualFold(positional[2], "withdraw") {
			k.Withdraw = true
			break
		}
		resp, ok := responseFor(positional[2])
		if !ok {
			fmt.Fprintf(os.Stderr,
				"%q is not an answer. Say yes, no, not-sure, or withdraw.\n", positional[2])
			return 2
		}
		k.Response = string(resp)
	default:
		fmt.Fprintln(os.Stderr,
			"usage: director knows\n"+
				"       director knows <subject> <kind> yes|no|not-sure|withdraw")
		return 2
	}
	// `args` rather than nil: this command has a machine reader (the browser surface), and
	// --json is how it reads.
	return observationQuery(args, service.ObserveQuery{Knows: k}, renderKnown)
}

// renderKnown puts a person's own judgements back in front of them.
//
// Grouped by application, current truth only. A withdrawn answer is simply absent: it is no longer
// something you told Marco, and showing it greyed out would be showing two answers with nothing to
// say which one governs.
func renderKnown(raw json.RawMessage) string {
	var v struct {
		Known []observe.KnownJudgement `json:"known"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return "Marco could not read what it knows.\n"
	}
	if len(v.Known) == 0 {
		return "WHAT MARCO KNOWS\n\n" +
			"  Nothing yet. Marco records something here when you answer a question about\n" +
			"  what part of an application means.\n"
	}
	var b strings.Builder
	b.WriteString("WHAT MARCO KNOWS\n")
	app := ""
	for _, k := range v.Known {
		if k.Application != app {
			app = k.Application
			fmt.Fprintf(&b, "\n%s\n", strings.ToUpper(app))
		}
		b.WriteString("\n  You told Marco:\n")
		fmt.Fprintf(&b, "    %s\n", k.Said)
		if k.Called != "" {
			fmt.Fprintf(&b, "    You call this screen %q.\n", k.Called)
		}
		fmt.Fprintf(&b, "    %s\n", k.About)
		if !k.Locatable {
			// NOT hidden, and not dressed up as findable. The judgement is real and the
			// person is entitled to withdraw it; what Marco has lost is the ability to
			// point at what it was about.
			b.WriteString("    Marco remembers your answer but can't currently locate " +
				"what it referred to.\n")
		}
		fmt.Fprintf(&b, "    To change it:   director knows %s %s yes|no\n", k.Subject, k.Kind)
		fmt.Fprintf(&b, "    To withdraw it: director knows %s %s withdraw\n", k.Subject, k.Kind)
	}
	return b.String()
}
