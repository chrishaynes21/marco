package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/rephrase"
	"github.com/chaynes-simpleclouds/marco/internal/director/satisfied"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// DOES MARCO UNDERSTAND WHAT YOU ASKED, OR ONLY WHAT IT WOULD HAVE SAID ITSELF?
//
// `director assess-rewording` runs ordinary instructions, phrased the way somebody who does
// not know an interface's exact vocabulary phrases them, against recorded readings of real
// applications — and scores each answer against what a person said beforehand was meant.
//
// # The oracle beside the verdict
//
// Each case carries the control a person says they meant, written down BEFORE the run. That
// is the pattern this repository keeps for exactly one reason: a resolver's own verdict is
// not evidence about whether it was right, and four false successes hid behind a green suite
// the last time acceptance was measured without one. See [[Oracle beside the verdict]].
//
// # The four counts, and the only one that is a regression
//
// CORRECT, WRONG CONTROL, ASKED UNNECESSARILY, REFUSED WHEN IT COULD HAVE ACTED.
//
// Wrong control must be zero, and it is the only one of the four that must. The other three
// are costs: asking too often is the thing rewording exists to reduce, and refusing too often
// is what it was measured doing. Acting on the wrong control is a different kind of thing
// altogether — it is the failure a person cannot see happening — and no reduction in the
// other three is worth buying with any of it.
//
// A fifth count stands apart: ALREADY SATISFIED. A request for a state the screen is already
// in is not a resolution problem and must not be scored as one. If Marco can say "Apps is
// already open" that is a better answer than pressing the right Apps control, and a scoring
// scheme that rewarded the press would be measuring the wrong thing. See
// internal/director/satisfied.
//
// # And the half of the measurement that is about what did NOT change
//
// Every case is resolved twice: once as production resolves it, and once with rewording
// switched off. A case the literal reading already answered must come out IDENTICAL — same
// status, same control, same confidence, same sentence. That is the first property of
// [[ADR-218-a-request-reworded-in-the-screens-own-words]], measured rather than asserted,
// and a single changed answer there fails the run.

// rewordSet is one application's evaluation set.
type rewordSet struct {
	Application string `json:"application"`
	Note        string `json:"note,omitempty"`
	// HeldOut marks a set that was written after the implementation was finished and
	// must not be tuned against. It changes nothing about how the run works; it is here
	// so a reader of the output knows which numbers count.
	HeldOut bool         `json:"held_out,omitempty"`
	Cases   []rewordCase `json:"cases"`
}

// rewordCase is one instruction, where it was said, and what it meant.
type rewordCase struct {
	// Say is the target text as the person phrased it — what the planner would carry
	// through from the instruction.
	Say string `json:"say"`
	// On is the corpus sample this was said against.
	On string `json:"on"`
	// Kind groups the case for the report: exact, extra-words, spelling, paraphrase,
	// ambiguous, absent, satisfied. Descriptive only — nothing branches on it.
	Kind string `json:"kind"`
	// Verb is the semantic action, default "open".
	Verb string `json:"verb,omitempty"`
	// Means is the oracle: what a person says was meant, written before the run.
	Means rewordOracle `json:"means"`
}

// rewordOracle is what a person says the right answer is. Exactly one form is set.
type rewordOracle struct {
	// Label and Role name the control that should be acted on.
	Label string `json:"label,omitempty"`
	Role  string `json:"role,omitempty"`
	// Ambiguous says two or more controls genuinely fit and the person should be asked.
	Ambiguous bool `json:"ambiguous,omitempty"`
	// Absent says nothing on this screen is what was asked for.
	Absent bool `json:"absent,omitempty"`
	// Satisfied says the screen is already in the state asked for and nothing should be
	// pressed at all.
	Satisfied bool `json:"satisfied,omitempty"`
	// Why records the person's reasoning, for the cases where it is arguable. It is
	// evidence about the oracle, which is the part of an acceptance measurement that is
	// easiest to quietly bend.
	Why string `json:"why,omitempty"`
}

// the outcome of one case, named as a person would describe it
const (
	outCorrect   = "correct"
	outWrong     = "WRONG CONTROL"
	outAsked     = "asked unnecessarily"
	outRefused   = "refused, could have acted"
	outAnswered  = "ANSWERED AN AMBIGUITY"
	outUnseen    = "unobservable"
	outSatisfied = "satisfaction missed"
	// outUnmeasured is for a case this reading cannot answer either way — an
	// already-satisfied case against a sample that records no selection state at all.
	// Reported as unmeasured rather than as failed, because a sample that never carried
	// the evidence is not evidence against the rule.
	outUnmeasured = "unmeasured here"
	// literalSuffix marks an outcome the literal reading produced on its own.
	literalSuffix = " (literal)"
)

// literally names an outcome rewording had no part in.
func literally(outcome string) string { return outcome + literalSuffix }

// introducedByRewording reports whether an outcome is one the change being measured caused.
func introducedByRewording(outcome string) bool {
	return !strings.HasSuffix(outcome, literalSuffix)
}

// runAssessRewording is `director assess-rewording`.
func runAssessRewording(args []string) int {
	fs := flag.NewFlagSet("assess-rewording", flag.ExitOnError)
	setPath := fs.String("set", "fixtures/rewording", "an evaluation set, or a directory of them")
	corpus := fs.String("corpus", "fixtures/perception/desktop/corpus",
		"the desktop perception corpus the cases are said against")
	verbose := fs.Bool("verbose", false, "print every case, not only the failures")
	// SHADOW MODE, and it is a flag rather than a setting because that is the experiment.
	//
	// With it, every case is ALSO put to the configured interpreter ($MARCO_INTERPRETER)
	// and scored in its own column. Nothing it says reaches resolution, the plan, the
	// policy or the executor — the proposals are compared with what Marco decided and
	// then thrown away. See internal/director/meaning.
	shadow := fs.Bool("interpreter", false,
		"also ask $MARCO_INTERPRETER which control each request meant, and score its "+
			"proposals separately (nothing it says is acted on)")
	_ = fs.Parse(flagsFirst(args))

	sets, err := loadRewordSets(*setPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	samples, err := loadDesktopSamples(*corpus)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	worlds := map[string]directorapi.WorldState{}
	for _, s := range samples {
		worlds[s.ID] = desktopWorld(s)
	}

	bad := 0
	for _, set := range sets {
		if assessRewordSet(os.Stdout, set, worlds, *verbose, *shadow) != 0 {
			bad = 1
		}
	}
	return bad
}

// rewordTally is one set's counts.
type rewordTally struct {
	byOutcome map[string]int
	byKind    map[string]map[string]int
	// changed is how many cases the literal reading already answered and rewording
	// answered differently — which must be none.
	changed []string
	// allowed is the changes a literal answer is permitted: one whose role
	// conflicted with the kind the person named, or a question that kind settles.
	allowed []string
	// reworded is how many answers were reached by rewording at all.
	reworded int
	// baselineCorrect is how many cases the LITERAL reading alone got right.
	baselineCorrect int
	// withRewording and literally are the per-case resolution costs.
	withRewording, literally []time.Duration
}

// assessRewordSet runs one set and prints its report. Non-zero when the run failed.
func assessRewordSet(out io.Writer, set rewordSet, worlds map[string]directorapi.WorldState,
	verbose, shadow bool) int {

	// PRODUCTION'S OWN RESOLVER, and a second one with rewording switched off. Not a
	// reimplementation of either: the same constructor, with one field cleared, so the
	// comparison is between production and production-minus-this-change and cannot
	// accidentally be between production and something simpler.
	live := target.NewResolver()
	literal := target.NewResolver()
	literal.Rephrasing = nil

	t := rewordTally{byOutcome: map[string]int{}, byKind: map[string]map[string]int{}}
	shade := newShadowTally()

	held := ""
	if set.HeldOut {
		held = "  [HELD OUT — not tuned against]"
	}
	fmt.Fprintf(out, "\n%s — %d cases%s\n", set.Application, len(set.Cases), held)
	if set.Note != "" {
		fmt.Fprintf(out, "%s\n", set.Note)
	}
	fmt.Fprintln(out, strings.Repeat("─", 96))

	for _, c := range set.Cases {
		w, ok := worlds[c.On]
		if !ok {
			fmt.Fprintf(out, "  %-34s  no sample %q in the corpus\n", quoted(c.Say), c.On)
			t.byOutcome[outUnseen]++
			continue
		}
		verb := c.Verb
		if verb == "" {
			verb = string(directorapi.SemanticOpen)
		}
		q := directorapi.ElementQuery{Label: c.Say}
		lic := directorapi.OperateLicence{Instruction: verb + " " + c.Say, Verb: verb}

		// Warmed before either is timed. Measured without it, whichever resolver ran
		// first paid for the sample's first walk and the other looked eight times
		// cheaper than it is — a cost comparison that was entirely about ordering.
		live.ResolveUnder(&w, q, lic, verb)
		literal.ResolveUnder(&w, q, lic, verb)

		started := time.Now()
		got := live.ResolveUnder(&w, q, lic, verb)
		t.withRewording = append(t.withRewording, time.Since(started))

		started = time.Now()
		base := literal.ResolveUnder(&w, q, lic, verb)
		t.literally = append(t.literally, time.Since(started))

		// WHAT DID NOT CHANGE, and the one class that is allowed to.
		//
		// A case the literal reading answered must be answered identically — except where
		// the control it chose was the wrong SORT of control for what the person said.
		// That exception is checked rather than trusted: the literal answer's role must
		// genuinely conflict with the kind the request named, and the replacement must
		// genuinely fit it. Anything else is a regression and fails the run.
		if base.Status != directorapi.ResolutionAbsent && !sameAnswer(base, got) {
			switch why := allowedChange(c.Say, base, got); why {
			case "":
				t.changed = append(t.changed, c.Say)
			default:
				t.allowed = append(t.allowed, c.Say+" ("+why+")")
			}
		}
		if got.Reworded != "" {
			t.reworded++
		}

		// BEFORE AND AFTER, over the same cases and the same oracle. The whole question
		// being asked is what rewording bought, and a report that gave only the new
		// number would leave the reader to guess at the old one.
		if before, _ := scoreRewordCase(&w, c, base, verb); before == outCorrect {
			t.baselineCorrect++
		}
		outcome, note := scoreRewordCase(&w, c, got, verb)
		// WHOSE ANSWER WAS IT. A wrong answer the literal reading gave on its own is a
		// defect this evaluation FOUND, not one rewording caused, and the two must not be
		// added together: the zero-wrong-control requirement is a requirement about the
		// change being made, and a tally that mixed them in would either excuse a new
		// defect or hide an old one behind a failing gate.
		if outcome != outCorrect && outcome != outUnmeasured &&
			got.Reworded == "" && sameAnswer(base, got) {
			outcome = literally(outcome)
		}
		t.byOutcome[outcome]++
		if t.byKind[c.Kind] == nil {
			t.byKind[c.Kind] = map[string]int{}
		}
		t.byKind[c.Kind][outcome]++

		shadowOutcome, shadowNote := "", ""
		if shadow {
			shadowOutcome, shadowNote = shade.askTheInterpreter(&w, c,
				outcome == outCorrect)
		}

		if verbose || outcome != outCorrect || strings.ToUpper(shadowOutcome) == shadowOutcome {
			fmt.Fprintf(out, "  %-34s %-13s %-13s %s\n", quoted(c.Say), c.Kind,
				strings.ToUpper(string(got.Status)), outcome)
			if note != "" {
				fmt.Fprintf(out, "  %-34s   %s\n", "", note)
			}
			if got.Reworded != "" {
				fmt.Fprintf(out, "  %-34s   by %s\n", "", got.Reworded)
			}
			if shadowOutcome != "" {
				fmt.Fprintf(out, "  %-34s   interpreter: %s — %s\n", "",
					shadowOutcome, shadowNote)
			}
		}
	}

	status := reportRewordTally(out, t, len(set.Cases))
	if shadow {
		shade.report(out)
	}
	return status
}

// scoreRewordCase compares one answer with what a person said was meant.
func scoreRewordCase(w *directorapi.WorldState, c rewordCase,
	got directorapi.Resolution, verb string) (string, string) {

	// SATISFACTION IS ASKED FIRST AND SCORED APART. A request for a state the screen is
	// already in has no right answer among the controls, so comparing the chosen control
	// with an oracle would be scoring the wrong question.
	if c.Means.Satisfied {
		if s := satisfactionOf(w, got, verb); s.Already {
			return outCorrect, s.Because
		}
		if !recordsSelection(w) {
			return outUnmeasured, "this reading records no selection state at all, so " +
				"nothing in it can say which page is the current one"
		}
		return outSatisfied, fmt.Sprintf("the screen is already as asked (%s) and Marco "+
			"answered %s instead", c.Means.Why, strings.ToUpper(string(got.Status)))
	}

	switch got.Status {
	case directorapi.ResolutionUnobservable:
		return outUnseen, got.Explanation

	case directorapi.ResolutionResolved:
		switch {
		case c.Means.Absent:
			return outWrong, fmt.Sprintf("nothing here is %q, and it chose %s %q",
				c.Say, got.Target.Role, got.Target.Label)
		case c.Means.Ambiguous:
			return outAnswered, fmt.Sprintf("several controls fit and it chose %s %q "+
				"without asking", got.Target.Role, got.Target.Label)
		case matchesOracle(got.Target, c.Means):
			return outCorrect, ""
		default:
			return outWrong, fmt.Sprintf("meant %s %q, chose %s %q", c.Means.Role,
				c.Means.Label, got.Target.Role, got.Target.Label)
		}

	case directorapi.ResolutionAmbiguous:
		switch {
		case c.Means.Ambiguous:
			return outCorrect, ""
		case c.Means.Absent:
			return outWrong, fmt.Sprintf("nothing here is %q, and it offered %d controls",
				c.Say, len(got.Contenders))
		default:
			return outAsked, fmt.Sprintf("meant %s %q; offered %s",
				c.Means.Role, c.Means.Label, contenderLabels(got.Contenders))
		}

	default: // absent
		if c.Means.Absent {
			return outCorrect, ""
		}
		return outRefused, fmt.Sprintf("meant %s %q, which is on screen",
			c.Means.Role, c.Means.Label)
	}
}

// satisfactionOf asks whether any control this answer points at is already as asked.
//
// Every contender, not only a resolved target: a request that could not be told apart
// between two controls, ONE of which the interface reports as the current choice, is
// satisfied — and asking which one to press would be asking a question that has no useful
// answer.
func satisfactionOf(w *directorapi.WorldState, res directorapi.Resolution,
	verb string) satisfied.Satisfaction {

	var ids []directorapi.ElementID
	if res.Target != nil {
		ids = append(ids, res.Target.ElementID)
	}
	for _, c := range res.Contenders {
		ids = append(ids, c.ElementID)
	}
	for _, id := range ids {
		el, ok := w.Element(id)
		if !ok {
			continue
		}
		if s := satisfied.AlreadyThere(el, verb); s.Already {
			return s
		}
	}
	return satisfied.Satisfaction{}
}

// recordsSelection reports whether this reading says anywhere which control is the current
// choice. A reading that says so nowhere cannot be asked whether a request is satisfied.
func recordsSelection(w *directorapi.WorldState) bool {
	for _, el := range w.Elements {
		if el.Selected {
			return true
		}
	}
	return false
}

func matchesOracle(got *directorapi.ResolvedTarget, want rewordOracle) bool {
	if !strings.EqualFold(strings.TrimSpace(got.Label), strings.TrimSpace(want.Label)) {
		return false
	}
	return want.Role == "" || string(got.Role) == want.Role
}

// sameAnswer is identity for the no-regression half of the measurement.
//
// Status, control, confidence and sentence, because a change in any of them is a change a
// caller can see: policy reads the confidence, the account prints the sentence, and the
// executor presses whatever the id names.
func sameAnswer(a, b directorapi.Resolution) bool {
	if a.Status != b.Status || a.Explanation != b.Explanation {
		return false
	}
	if (a.Target == nil) != (b.Target == nil) {
		return false
	}
	if a.Target != nil &&
		(a.Target.ElementID != b.Target.ElementID || a.Target.Confidence != b.Target.Confidence) {
		return false
	}
	return len(a.Contenders) == len(b.Contenders)
}

// allowedChange names the reason a literal answer was allowed to change, or "" if it was not.
//
// # Why this is a short closed list rather than a tolerance
//
// Because "no literal answer may change" is the property the whole rewording design rests on,
// and every exception to it has to be a decision somebody took and wrote down. There are two,
// both from the person's own sentence naming a KIND of control, and each is checked rather
// than trusted — using the resolver's own `target.RoleFits`, so the measurement and the
// behaviour cannot drift.
//
// RECONSIDERED: the words chose a control whose role conflicts with the kind that was named,
// and the replacement fits it. [[ADR-219-a-match-is-not-an-answer]].
//
// SETTLED: the words could not choose between several controls, and the kind that was named
// fits exactly one of them. [[ADR-221-a-question-the-person-already-answered]].
//
// Anything else is a regression and fails the run.
func allowedChange(say string, base, got directorapi.Resolution) string {
	kinds := rephrase.KindNamed(say)
	if len(kinds) == 0 || got.Target == nil ||
		got.Status != directorapi.ResolutionResolved {
		return ""
	}
	if !target.RoleFits(got.Target.Role, kinds) {
		return ""
	}
	switch base.Status {
	case directorapi.ResolutionResolved:
		if base.Target != nil && !target.RoleFits(base.Target.Role, kinds) {
			return "reconsidered"
		}
	case directorapi.ResolutionAmbiguous:
		fitting, offered := 0, false
		for _, c := range base.Contenders {
			if c.ElementID == got.Target.ElementID {
				offered = true
			}
			if target.RoleFits(c.Role, kinds) {
				fitting++
			}
		}
		if offered && fitting == 1 {
			return "settled by the kind they named"
		}
	}
	return ""
}

func contenderLabels(cs []directorapi.TargetCandidate) string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, fmt.Sprintf("%s %q", c.Role, c.Label))
	}
	return strings.Join(out, " / ")
}

// reportRewordTally prints the counts and decides the exit status.
func reportRewordTally(out io.Writer, t rewordTally, cases int) int {
	fmt.Fprintln(out, strings.Repeat("─", 96))
	for _, name := range []string{outCorrect, outWrong, outAnswered, outAsked,
		outRefused, outSatisfied, outUnmeasured, outUnseen,
		literally(outWrong), literally(outAnswered), literally(outAsked),
		literally(outRefused), literally(outSatisfied), literally(outUnseen)} {
		if n := t.byOutcome[name]; n > 0 || name == outWrong || name == outCorrect {
			fmt.Fprintf(out, "  %-34s %3d  (%.0f%%)\n", name, n,
				100*float64(n)/float64(max(cases, 1)))
		}
	}
	fmt.Fprintf(out, "  %-34s %3d\n", "answers reached by rewording", t.reworded)
	// BEFORE AND AFTER, over the same cases and the same oracle. The whole question being
	// asked is what rewording bought, and a report giving only the new number would leave
	// the reader to guess at the old one.
	fmt.Fprintf(out, "  %-34s %3d → %3d\n", "correct: literal only → reworded",
		t.baselineCorrect, t.byOutcome[outCorrect])
	if len(t.allowed) > 0 {
		fmt.Fprintf(out, "  %-34s %3d  %s\n", "literal answers changed, allowed",
			len(t.allowed), strings.Join(t.allowed, ", "))
	}
	fmt.Fprintln(out, "\n  (literal) marks an answer rewording had no part in — the literal\n"+
		"  reading gave it on its own, and this evaluation found it rather than caused it.")

	if len(t.byKind) > 0 {
		fmt.Fprintln(out, "\n  by phrasing:")
		kinds := make([]string, 0, len(t.byKind))
		for k := range t.byKind {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		for _, k := range kinds {
			total, right := 0, t.byKind[k][outCorrect]
			for _, n := range t.byKind[k] {
				total += n
			}
			// Unmeasured cases are taken OUT of the denominator rather than counted
			// as misses. A reading that never carried the evidence has not answered
			// wrongly, and scoring it as a miss would make the capture's limitation
			// look like Marco's.
			unmeasured := t.byKind[k][outUnmeasured]
			note := ""
			if unmeasured > 0 {
				note = fmt.Sprintf("   (%d unmeasured: the reading carries no evidence "+
					"either way)", unmeasured)
			}
			fmt.Fprintf(out, "    %-22s %d/%d%s\n", k, right, total-unmeasured, note)
		}
	}

	// COST. No additional observation: every attempt re-ranks a world that was already
	// read, which is the reason rewording can sit inside resolution at all.
	fmt.Fprintf(out, "\n  cost  literal %s/case, with rewording %s/case, worst %s "+
		"— no additional observation\n",
		mean(t.literally).Round(time.Microsecond),
		mean(t.withRewording).Round(time.Microsecond),
		worst(t.withRewording).Round(time.Microsecond))

	status := 0
	if n := t.byOutcome[outWrong]; n > 0 {
		fmt.Fprintf(out, "\n  FAILED: rewording chose %d wrong control(s). "+
			"This number must be zero.\n", n)
		status = 1
	}
	if n := t.byOutcome[outAnswered]; n > 0 {
		fmt.Fprintf(out, "  FAILED: rewording answered %d ambiguity(ies) without asking.\n", n)
		status = 1
	}
	if n := t.byOutcome[literally(outWrong)] + t.byOutcome[literally(outAnswered)]; n > 0 {
		fmt.Fprintf(out, "\n  NOTE: the literal reading gave %d wrong answer(s) of its own, "+
			"before rewording was reached.\n  Those are findings, not regressions, and they "+
			"are not fixed here — see the experiment record.\n", n)
	}
	if len(t.changed) > 0 {
		fmt.Fprintf(out, "  FAILED: rewording changed %d answer(s) the literal reading "+
			"already gave: %s\n", len(t.changed), strings.Join(t.changed, ", "))
		status = 1
	}
	return status
}

func mean(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range ds {
		total += d
	}
	return total / time.Duration(len(ds))
}

func worst(ds []time.Duration) time.Duration {
	var hi time.Duration
	for _, d := range ds {
		if d > hi {
			hi = d
		}
	}
	return hi
}

func quoted(s string) string { return fmt.Sprintf("%q", s) }

// loadRewordSets reads one evaluation set or a directory of them.
func loadRewordSets(path string) ([]rewordSet, error) {
	if s, err := readRewordSet(path); err == nil {
		return []rewordSet{s}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var out []rewordSet
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		s, err := readRewordSet(filepath.Join(path, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no evaluation sets in %s", path)
	}
	// Held-out sets last, so the report reads in the order the work happened.
	sort.SliceStable(out, func(i, j int) bool { return !out[i].HeldOut && out[j].HeldOut })
	return out, nil
}

func readRewordSet(path string) (rewordSet, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return rewordSet{}, err
	}
	var s rewordSet
	if err := json.Unmarshal(raw, &s); err != nil {
		return rewordSet{}, fmt.Errorf("%s: %w", path, err)
	}
	if len(s.Cases) == 0 {
		return rewordSet{}, fmt.Errorf("%s: no cases", path)
	}
	return s, nil
}
