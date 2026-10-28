package orchestrator

// WHICH GENERATION OF THE GENERATOR'S RULES WROTE A SAVED ROUTINE.
//
// A routine keeps the shape it was generated with. Nothing rewrites saved Marco — the file is the
// only executable truth about a routine, and rewriting generated source by hand is how two things
// that have to agree stop agreeing. So every rule about what a generated routine DOES reaches a
// routine when that routine is written, and a library accumulates files written under rules that
// no longer match each other: a routine saved before typed text became a sequence of submissions
// goes on acting past text that never arrived, and reports a run that performed.
//
// Nothing on disk said which rules wrote a file, and nothing can infer it. The shapes in a
// routine are a function of the demonstration as much as of the generator, which is exactly how a
// guarded drag read as `legacy` before `draggedTerms` existed — a reader that does not know a
// shape answers confidently and wrongly. So the generation is STATED by the only thing that knows
// it, at the moment it is true, and read back here.
//
// # The four readings, and why none of them collapses into another
//
// CURRENT, OLDER, UNSTATED and UNKNOWN are four different states of knowledge, and the damage
// from merging any two of them is the same damage in a different direction:
//
//	CURRENT   states the generation this build writes.
//	OLDER     states an earlier generation this build can explain.
//	UNSTATED  says nothing. NOT "written before versioning" — the file could be handwritten, or
//	          have had the line taken out. "Says nothing" is the whole of what is known.
//	UNKNOWN   says something this cannot explain: a later build's generation, a malformed value,
//	          or the same question answered twice. It is not current and it is not unstated,
//	          because the file does say something and this reader is the one that cannot read it.
//
// An unknown generation stays inspectable and playable under exactly the execution rules it
// always had. Nothing here downgrades, rewrites or offers to regenerate a file it cannot explain:
// a reader that cannot say what a generation means cannot say what replacing it would change.
//
// # And it is provenance, not proof
//
// The stamp says which rules wrote the source. It does not say the source is guarded, or
// unmodified, or that anything else about it is still true. A routine of the current generation
// can carry legacy behaviour on purpose — a demonstration with no measured press generates
// exactly the unchecked route it always did — so what a routine's presses promise stays
// `ContractOf`'s separate question, read off the shapes rather than off this line.
//
// See [[ADR-262-a-routine-states-which-generation-wrote-it]].

import (
	"fmt"
	"strconv"
	"strings"
)

// generationStated is the comment line codegen writes and this reads back.
//
// One constant shared by the writer and the reader would be better, and it cannot be: `codegen`
// may not import this package. Held by test instead —
// `TestTheGenerationASaveWritesIsTheGenerationThisReads` generates a route and requires this to
// read the generation `codegen` says it writes.
const generationStated = "// Generation: "

// GenerationReading is which of the four things a saved routine's generation line reads as.
type GenerationReading string

const (
	// GenerationCurrent states the generation this build's generator writes.
	GenerationCurrent GenerationReading = "current"
	// GenerationOlder states an earlier generation this build can explain, which is what
	// makes a difference describable in words.
	GenerationOlder GenerationReading = "older"
	// GenerationUnstated is a routine with no generation line at all.
	//
	// The zero value on purpose: every routine already on disk reads this way, and so does
	// every file somebody wrote by hand.
	GenerationUnstated GenerationReading = "unstated"
	// GenerationUnknown states something this build cannot explain.
	GenerationUnknown GenerationReading = "unknown"
)

// StatedGeneration is one reading of one saved routine.
type StatedGeneration struct {
	// Reading is which of the four it is.
	Reading GenerationReading
	// Generation is the number stated, and is set only for a reading this build can
	// explain — current or older. Zero otherwise, including for a number that parsed and
	// turned out to be a later build's: a number this cannot explain is not a number to do
	// arithmetic with.
	Generation int
	// Declared is what the file actually said, verbatim, whenever it said anything that
	// could be read as text at all.
	//
	// KEPT SO A RECORD CAN CARRY IT. A record that held only this reader's verdict would
	// turn a newer build's routine into "unknown" and lose the one fact that identifies
	// it — and somebody diagnosing a run on an older executor needs to see the generation
	// the file states, not this build's inability to explain it.
	Declared string
	// Why is why an unknown reading is unknown, in a sentence a person reads. Empty for
	// the other three.
	Why string
}

// GeneratorRule is one thing a generation of the generator does, as a person reads it.
//
// The sentence is about what a ROUTINE does, not about the generator's internals: it is shown to
// somebody deciding whether to replace a file of their own.
type GeneratorRule struct {
	// Says is the sentence.
	Says string
	// Bears reports whether one saved routine's own contents bear on this rule — whether
	// the file contains the thing this generation changed.
	//
	// WHY A RULE CARRIES ITS OWN: because "older" does not mean "affected". A generation
	// difference is a fact about two GENERATORS; whether it changes anything for one
	// routine is a fact about that routine, and a reader that announced every difference
	// as a defect would be telling somebody to replace a file over a rule it never
	// exercises. Nil when nothing in a saved file can show it either way, which reads as
	// "not established" and offers a comparison rather than a verdict.
	Bears func(src string) bool
}

// GenerationRules is what one generation does, and the words for it.
type GenerationRules struct {
	Generation int
	Rules      []GeneratorRule
}

// history is the ordered account of the generator's rules, and the one authority for both halves
// of this: which generation is current, and what each one means.
//
// ONE LIST, because the alternative has already cost this repository a defect of exactly this
// shape. `everyMeasuredTerm` exists because a term added to a shape's list and not to the
// reporting list is read, found absent, and never named; a generation added to a version counter
// and not to this would be read, found older, and never explainable. So the current generation is
// the last entry's number rather than a constant beside it, and a generation with no sentence is
// a build that does not compile its own tests.
type history []GenerationRules

// theHistory is that account.
//
// # Why it starts at 1 with nothing behind it
//
// Because a generation is a claim about a file, and nobody stamped the files written before this.
// Numbering the rule changes that landed before today 1..5 would place routines in a history they
// never stated — the same inference that made a guarded drag read as legacy, with a version
// number on it. What came before is unstated, and unstated is read as unstated.
//
// So there is one entry, and its sentences are what this generation's generator does rather than
// what it does DIFFERENTLY. The difference-walking below has nothing to walk yet, which is why it
// is exercised against synthetic histories: a mechanism with no live input is one nobody can tell
// is wired, and this one has to work the first time it matters, on somebody's library.
var theHistory = history{{
	Generation: 1,
	Rules: []GeneratorRule{
		{Says: "A press that was measured against the window it was demonstrated in is " +
			"checked before it happens, and refuses rather than pressing somewhere " +
			"else when the window has changed."},
		{Says: "Every action that can fail is its own step, and each one runs the next " +
			"only when it succeeded — so the routine stops at the first action that " +
			"did not happen instead of carrying on."},
		{Says: "A press states which button it was, so a right or middle press is one " +
			"checked press rather than an aim followed by a click at the cursor."},
		{Says: "A drag states both of its ends, measured in the one window it was " +
			"demonstrated in."},
		{Says: "Typed text is a sequence of submissions that says how far it got, and " +
			"each run of text stops what follows it when part of it did not go in."},
		{Says: "A keystroke, a chord and a held key stop what follows them when the " +
			"machine did not take them."},
		{Says: "A routine whose application has to come forward does that as its first " +
			"step and stops if it did not, so nothing is pressed into whatever " +
			"happened to be in front."},
		{Says: "A demonstration containing a press that cannot be stopped at is refused " +
			"when it is saved, rather than written as a routine that reads as " +
			"checked and is not."},
	},
}}

// current is the generation this build writes, which is the last entry's.
func (h history) current() int {
	if len(h) == 0 {
		return 0
	}
	return h[len(h)-1].Generation
}

// explains is whether this history has words for a generation.
func (h history) explains(n int) bool {
	for _, g := range h {
		if g.Generation == n {
			return true
		}
	}
	return false
}

// since is every generation after n, in order: what a routine stating n was written without.
func (h history) since(n int) []GenerationRules {
	var out []GenerationRules
	for _, g := range h {
		if g.Generation > n {
			out = append(out, g)
		}
	}
	return out
}

// read is the four readings, off the source.
func (h history) read(src string) StatedGeneration {
	var said []string
	for _, line := range strings.Split(src, "\n") {
		// THE LINE HAS TO START WITH IT, which is what keeps a routine that TYPES
		// this text from being read as stating a generation: `do OS's Type with
		// "// Generation: 9."` is a line starting with `do`. Same rule as `ModeOf`,
		// and it is load-bearing for the same reason.
		rest, found := strings.CutPrefix(strings.TrimSpace(line), generationStated)
		if !found {
			continue
		}
		said = append(said, strings.TrimSuffix(strings.TrimSpace(rest), "."))
	}
	switch len(said) {
	case 0:
		return StatedGeneration{Reading: GenerationUnstated}
	case 1:
	default:
		// TWICE IS NOT ONCE, EVEN WHEN THE TWO AGREE. A file answering one
		// question twice has been edited by something, and which answer is the
		// file's is not knowable from the file. Reading the first, or the last, or
		// the one they agree on would each be a guess presented as a reading.
		return StatedGeneration{Reading: GenerationUnknown,
			Declared: strings.Join(said, ", "),
			Why: fmt.Sprintf("it states a generation %d times (%s), and which "+
				"of those the file means is not something the file says",
				len(said), strings.Join(said, ", "))}
	}
	declared := said[0]
	n, err := strconv.Atoi(declared)
	switch {
	case err != nil || n < 1:
		return StatedGeneration{Reading: GenerationUnknown, Declared: declared,
			Why: fmt.Sprintf("it states its generation as %q, which is not a "+
				"generation", declared)}
	case n > h.current():
		// A LATER BUILD'S ROUTINE, which is the reading an older executor gets and
		// must not get wrong. Relabelling it current would be this build claiming
		// to have written it; relabelling it unstated would be losing the only
		// fact about it there is.
		return StatedGeneration{Reading: GenerationUnknown, Declared: declared,
			Why: fmt.Sprintf("it states generation %d and this build writes "+
				"generation %d, so it was written by a later build and "+
				"this one cannot say what that generation does", n,
				h.current())}
	case !h.explains(n):
		// A GAP IN THE HISTORY, which the tests do not allow and which is read
		// defensively anyway: a generation in range that this cannot describe is
		// one it must not offer to replace.
		return StatedGeneration{Reading: GenerationUnknown, Declared: declared,
			Why: fmt.Sprintf("it states generation %d, which this build has no "+
				"account of", n)}
	case n == h.current():
		return StatedGeneration{Reading: GenerationCurrent, Generation: n,
			Declared: declared}
	}
	return StatedGeneration{Reading: GenerationOlder, Generation: n, Declared: declared}
}

// GenerationOf reads what a saved routine states about the generation that wrote it.
//
// Read from the SOURCE, like the contract and the mode, and for the same reason: the saved Marco
// is the only durable account of a routine, and a second record of it beside the file is a second
// record to disagree with it.
func GenerationOf(src string) StatedGeneration { return theHistory.read(src) }

// GenerationNow is the generation this build's generator writes.
func GenerationNow() int { return theHistory.current() }

// ThisGenerationDoes is what the current generation's routines do, in the words a person reads.
func ThisGenerationDoes() []GeneratorRule {
	if len(theHistory) == 0 {
		return nil
	}
	return theHistory[len(theHistory)-1].Rules
}

// Difference is one generator difference as it applies to ONE saved routine.
type Difference struct {
	// Generation is the generation that introduced it.
	Generation int
	// Says is the difference, in words.
	Says string
	// Bears is whether this routine's own contents bear on it — whether the file contains
	// the thing that generation changed.
	//
	// FALSE IS NOT "NO". It means this was not established from the contents, which covers
	// both a rule whose thing is absent and a rule that cannot be told from a saved file at
	// all. Either way what can honestly be offered is a comparison, not a verdict.
	Bears bool
}

// GenerationDifferencesOf is what a saved routine was written without, resolved against its own
// contents.
//
// Empty for the other three readings, and each for its own reason: a current routine was written
// without nothing; an unstated one states no generation to walk from, and walking from a guess
// would be the inference this whole thing exists to avoid; an unknown one cannot be explained, so
// neither can the gap.
func GenerationDifferencesOf(src string) []Difference {
	return theHistory.differences(src)
}

func (h history) differences(src string) []Difference {
	s := h.read(src)
	if s.Reading != GenerationOlder {
		return nil
	}
	var out []Difference
	for _, g := range h.since(s.Generation) {
		for _, r := range g.Rules {
			out = append(out, Difference{Generation: g.Generation, Says: r.Says,
				Bears: r.Bears != nil && r.Bears(src)})
		}
	}
	return out
}

// Bears reports whether any of these differences bears on the routine they were read from.
//
// WHICH IS THE ONLY THING THAT LICENSES SAYING A ROUTINE NEEDS AN UPDATE. Without it the claim
// would be "this file is older", which is a fact about two generators and not about anybody's
// routine.
func Bears(diffs []Difference) bool {
	for _, d := range diffs {
		if d.Bears {
			return true
		}
	}
	return false
}

// GenerationSentence is what is said to a person about one saved routine's generation.
//
// Two parts: what the file states, and what can be done about it. The second part exists because
// the Review button is greyed out by the same facts — a routine with no stored demonstration, and
// one whose generation this build cannot explain — and a disabled control with nothing beside it
// is a dead end this product has shipped before.
//
// `rebuildable` is whether its own demonstration is still stored beside it. See `Deps.Prepare`.
func GenerationSentence(s StatedGeneration, rebuildable bool) string {
	switch s.Reading {
	case GenerationCurrent:
		was := fmt.Sprintf("This routine states that generation %d of Marco's rules "+
			"wrote it, which is the generation this build writes.", s.Generation)
		if !rebuildable {
			// SAID EVEN THOUGH NOTHING IS WRONG, because the button is off
			// and somebody will wonder why. It matters later rather than now:
			// the day the rules change, this is the routine that cannot be
			// brought forward without being demonstrated again.
			return was + " Its demonstration is no longer stored beside it, so " +
				"there is nothing to rebuild a version from — which will " +
				"matter only if these rules change."
		}
		// AND IT DOES NOT PROMISE THAT A REBUILD WOULD MATCH.
		//
		// It used to say "it will be the same routine", which is a claim the stamp
		// cannot support: the generation says which rules WROTE the source, not
		// that the source is still what they wrote. A routine somebody edited by
		// hand keeps its stamp and rebuilds differently, and that person is
		// exactly who would be told their comparison was pointless. Only
		// `Prepared.Changes()` can answer whether the two differ, and it answers
		// after the rebuild — so what is offered here is the comparison itself.
		return was + " You can review a version rebuilt from its stored " +
			"recording, which will show whether it differs from what is " +
			"saved."
	case GenerationOlder:
		was := fmt.Sprintf("This routine states that generation %d of Marco's rules "+
			"wrote it, and this build writes generation %d.", s.Generation,
			theHistory.current())
		return was + " " + offer(rebuildable)
	case GenerationUnknown:
		// NOTHING IS OFFERED. See the note at the top of this file: a reader that
		// cannot say what a generation does cannot say what replacing it would
		// change, so this says what it found and stops.
		return fmt.Sprintf("This routine states a generation this build does not "+
			"recognise: %s. It is left exactly as it is, and runs under the "+
			"same rules it always has.", s.Why)
	}
	// AND UNSTATED SAYS ONLY THAT. Not "written before versioning", which would be a
	// claim about when the file was made: a handwritten routine says nothing here, and so
	// does one somebody took the line out of.
	return "This routine does not state which generation rules wrote it. " + offer(rebuildable)
}

// offer is the half of those sentences that depends on there being a demonstration, for a routine
// a rebuilt version could actually change.
func offer(rebuildable bool) string {
	if !rebuildable {
		return "Its demonstration is no longer stored beside it, so there is nothing to " +
			"rebuild a version from — recording it again is the way to get one " +
			"written by today's rules."
	}
	return "You can review a version rebuilt from its stored recording."
}

// Offerable is whether a rebuilt version may be OFFERED for a routine that reads this way.
//
// Three of the four readings may be. A current routine rebuilds to itself and comparing is
// harmless; an older or unstated one is the case this exists for. An unknown one may not: a
// reader that cannot say what a generation does cannot say what replacing it would change, so
// there is nothing to put in front of somebody as a choice. It stays inspectable and playable
// under exactly the rules it always had — withholding the offer is the whole of what happens.
//
// Asked here rather than in the window, so the button that is greyed out and the call that
// refuses are reading one answer. A page with its own copy of this is a page that can offer what
// the backend then declines, which is how a product tells somebody off for pressing what it
// offered them.
func (s StatedGeneration) Offerable() bool { return s.Reading != GenerationUnknown }
