package workflowrun

// DID THE ACTION WORK? THE POST-ACTION EVIDENCE, ON A CLOCK.
//
// # What was missing
//
// `Report.Verified` has always answered false with the same sentence: *no post-action evidence
// is collected about any of them*. That was true. The coordinator pressed, then asked one
// question afterwards — has the step's DESTINATION SCREEN been established — and on a workflow
// whose step has the same screen at both ends the answer is permanently no.
// [[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]] measured that case on Ark:
// the panel below the tab bar changes completely and the screen does not, so the press was
// followed by `patience` observations of nothing and then a refusal.
//
// [[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]] built the evidence that
// answers it — the words elsewhere on the window that appeared and stayed — and measured its
// four terms on two recordings. This file is that verifier put on a clock and into the replay
// lifecycle, which is the one integration the audit found missing.
//
// # Three answers, and the difference between them matters
//
//	VERIFIED       the words this step's demonstration recorded appeared where the control is
//	               not, and stayed. The step did what it did when it was shown.
//	CONTRADICTED   enough fresh evidence arrived and it says something else is standing.
//	UNCONFIRMED    the deadline passed without enough readings to decide. NOT a failure of the
//	               action and NOT a success: the question could not be put.
//
// A run that cannot tell the third from the first would report success whenever OCR was slow,
// which is the single most dangerous thing a verifier can do. So the deadline produces its own
// verdict and the run stops either way: nothing proceeds to another action on an unconfirmed
// one.
//
// # Why the clock is counted in READINGS and the deadline in TIME
//
// Because they are different scarcities. The EVIDENCE the verifier needs is five readings a
// side — Experiment 087 measured that a window counted in looks cannot be satisfied on a live
// cadence, because `readingEyes` drops a frame whenever a reading is still running and a
// reading costs hundreds of milliseconds against a hundred-millisecond look. The PATIENCE is
// time, because a person is waiting and an application that is never going to respond has to
// stop being waited for.
//
// The offline captures read one look in three, which puts five readings at about 1.5 seconds.
// The live rate is NOT that number and is not known until it is measured, which is why
// `Report` carries the readings per second it actually saw: a deadline chosen once and never
// checked is how the window unit was wrong in the first place.

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Verdict is what the observations after an action say about it.
type Verdict string

const (
	// VerdictUnasked is the zero value: no action was performed, or the step carries no
	// outcome expectation to check. NOT a refusal — the question was not put.
	VerdictUnasked Verdict = ""
	// VerdictVerified is the demonstrated local outcome standing in fresh evidence.
	VerdictVerified Verdict = "verified"
	// VerdictContradicted is enough fresh evidence saying something else.
	VerdictContradicted Verdict = "contradicted"
	// VerdictUnconfirmed is the deadline passing without enough evidence to decide.
	VerdictUnconfirmed Verdict = "unconfirmed"
)

// Decided reports whether a verdict is an answer rather than an absence.
func (v Verdict) Decided() bool { return v == VerdictVerified || v == VerdictContradicted }

// DefaultVerifyDeadline is how long after an action the evidence is waited for.
//
// Six seconds. The offline captures read one look in three, which puts the five readings a side
// that Experiment 087's terms require at about 1.5 seconds — so this is roughly four times the
// only measured rate there is, for a path known to be slower than the one that produced it.
//
// It is deliberately NOT presented as the right number. `Report.ReadingsPerSecond` is what the
// live runs measure, and the owner's instruction is the standing one:
//
//	Do not make an arbitrary 2-second deadline authoritative if measurements show it cannot
//	collect the evidence the verifier itself requires.
const DefaultVerifyDeadline = 6 * time.Second

// screenRead is one admitted reading and the screen it was taken on.
type screenRead struct {
	screen observe.VisualScreenID
	words  []observe.ReadWord
	at     time.Time
}

// verification is the post-action evidence being gathered for one step.
type verification struct {
	step   int
	screen observe.VisualScreenID
	// before is the readings of that screen taken BEFORE the action, and after those
	// taken since. Both oldest first.
	before, after [][]observe.ReadWord
	// quiet and moved are the stationarity self-test, taken from the readings before the
	// action: a screen that produces an outcome with nothing done to it has outcomes that
	// mean nothing, and `OutcomeHeld` refuses on them.
	quiet, moved int
	// skipped is how many readings after the action were dropped as mid-response. See
	// observe.OutcomeSkipReadings.
	skipped int
	// restarts is how many times a transition landed inside the window and it began
	// again. Reported, because a verdict reached after three restarts rests on a screen
	// that changed three times and a reader should know.
	restarts int
	// acted is when the action went out and deadline when waiting stops.
	acted, deadline time.Time
	// generation is the window generation the action was sent under. The after-readings are
	// scoped to it rather than to the screen id, so a change that mints a new screen does
	// not discard the evidence that it happened.
	generation uint64
	// sawScreens is every screen id the after-readings were taken under, for the report: a
	// verdict reached across a screen change is worth saying out loud.
	sawScreens []observe.VisualScreenID
	// actedSeq is WHICH reading was standing when the action went out. Every reading up
	// to and including it describes the screen the press landed on, so only a later one
	// is after-evidence. See Sighting.WordsSeq.
	actedSeq int
}

// WithVerifyDeadline sets how long after an action its evidence is waited for.
func (r *Runner) WithVerifyDeadline(d time.Duration) *Runner {
	if d > 0 {
		r.verifyFor = d
	}
	return r
}

// VerifyDeadline is the deadline in force.
func (r *Runner) VerifyDeadline() time.Duration {
	if r.verifyFor <= 0 {
		return DefaultVerifyDeadline
	}
	return r.verifyFor
}

// noteReading keeps one look's admitted reading, for the before side and the self-test.
//
// Bounded: only what a verification could use. A run that watched for five minutes must not
// hold five minutes of words.
func (r *Runner) noteReading(at Sighting) {
	if len(at.Words) == 0 || at.Screen == "" {
		return
	}
	// ── ONE READING IS ONE PIECE OF EVIDENCE, HOWEVER MANY LOOKS IT STANDS FOR ──
	//
	// OCR answers about twice a second against a loop that looks ten times, so the same
	// reading arrives on several consecutive sightings. Counting it once per sighting
	// filled a five-reading window from a single reading, and did it faster than any
	// application could respond. See Sighting.WordsSeq for the live measurement.
	//
	// Guarded on WordsSeq being set, so a caller that does not distinguish readings keeps
	// the old behaviour rather than silently collapsing everything to one.
	if at.WordsSeq != 0 && at.WordsSeq == r.lastWordsSeq {
		return
	}
	r.lastWordsSeq = at.WordsSeq
	r.reads = append(r.reads, screenRead{screen: at.Screen, words: at.Words, at: at.At})
	if len(r.reads) > maxKeptReadings {
		r.reads = r.reads[len(r.reads)-maxKeptReadings:]
	}
	r.readings++
	if r.firstRead.IsZero() {
		r.firstRead = at.At
	}
	r.lastRead = at.At
	// AND INTO AN OPEN VERIFICATION, when it is about this screen.
	//
	// Scoped to the screen the action was taken on, for the reason the demonstration's
	// own watch is: without it any later reading of anywhere would fill in the after
	// side, and the action would be credited with whatever Marco looked at next.
	// ── SCOPED TO THE WINDOW ACTED ON, NOT TO THE SCREEN HYPOTHESIS ──
	//
	// This required the after-readings to carry the SAME screen id as the one the action
	// was sent to, so that a later reading of somewhere else could not be credited to the
	// action. The window is what that rule was reaching for, and the screen id is a
	// stricter thing that breaks the case it matters most in.
	//
	// Measured live on the controlled test menu: the press worked, the run itself observed
	// `visual_1 -> visual_2`, and visual_2's words were the eighteen the step expected —
	// and not one of them reached the verifier, because they were read under a new screen
	// id. The check was built for the case where the screen does NOT change
	// ([[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]]); a change big
	// enough to mint a screen must not be the one thing that disables it.
	//
	// What stops an unrelated screen being credited is not the id: it is that the words
	// read have to overlap the expectation this step SAVED, by `OutcomeWords` of them.
	// The window generation keeps it to the same window across the same lifetime, which
	// is the fact the rule actually needs.
	if r.verify != nil && at.Generation == r.verify.generation &&
		len(r.verify.after) < observe.OutcomeSettleLooks &&
		// AND IT MUST BE A READING TAKEN AFTER THE ACTION, not the one that was
		// already standing when the action went out. That one describes the screen
		// the press landed on and is the BEFORE side's evidence; admitting it here
		// as well would be comparing a reading against itself.
		(at.WordsSeq == 0 || at.WordsSeq > r.verify.actedSeq) {

		// THE READING TAKEN MID-RESPONSE IS NOT EVIDENCE EITHER WAY.
		//
		// The same skip the demonstration side applies, and for the measured reason:
		// the persistence term wants a word on every reading of the window, and on
		// the first reading after a press the new panel is not drawn yet. Without
		// it, every word of Ark's new panel fell out as transient and the verifier
		// answered 0 of 3 where the offline measurement answered 2 of 3. See
		// observe.OutcomeSkipReadings.
		if r.verify.skipped < observe.OutcomeSkipReadings {
			r.verify.skipped++
			return
		}
		// ── A TRANSITION LANDING INSIDE THE WINDOW RESTARTS IT ──
		//
		// `OutcomeSkipReadings` drops the readings taken mid-response, and TWO is the
		// number measured offline on a recording read at every look. Live, OCR
		// answers about four times a second and an application may take longer than
		// two readings to redraw — so the window straddles the change, the new
		// panel's words are on its later readings only, and the persistence term
		// correctly calls every one of them TRANSIENT.
		//
		// Measured live on the controlled test menu: the press worked and the run
		// reported `everything that appeared was transient` on five readings
		// spanning 2.196s, because the first of them still showed the old panel.
		//
		// So a reading that brings `OutcomeWords` words the previous one did not have
		// is the transition itself, and the window starts again from there. A fixed
		// skip cannot know when the response happened; this asks.
		//
		// Through `OutcomeBetween`, which is the one statement of what "appeared"
		// means, over a window of one reading each side. And bounded, because a
		// screen that keeps changing substantially would restart forever — past the
		// bound the readings are taken as they come and the verdict is whatever they
		// say, which on a screen like that is a refusal.
		if n := len(r.verify.after); n > 0 && r.verify.restarts < observe.OutcomeSettleLooks {
			o := observe.OutcomeBetween(r.art.OutcomeExcluded(r.verify.step),
				[][]observe.ReadWord{r.verify.after[n-1]},
				[][]observe.ReadWord{at.Words})
			if len(o.Appeared) >= observe.OutcomeWords {
				r.verify.after = nil
				r.verify.restarts++
			}
		}
		r.verify.after = append(r.verify.after, at.Words)
		if at.Screen != r.verify.screen && !hasScreen(r.verify.sawScreens, at.Screen) {
			r.verify.sawScreens = append(r.verify.sawScreens, at.Screen)
		}
	}
}

// maxKeptReadings bounds the retained readings: two windows' worth plus the self-test's.
const maxKeptReadings = 4 * observe.OutcomeSettleLooks

// openVerification starts gathering post-action evidence for the step just acted on.
//
// Called at the press, because the BEFORE side is the readings already taken — a window that
// began after the action would compare the new panel against itself.
func (r *Runner) openVerification(step int, seen Sighting, at time.Time) {
	screen := seen.Screen
	v := &verification{
		step: step, screen: screen,
		acted: at, deadline: at.Add(r.VerifyDeadline()),
		actedSeq:   r.lastWordsSeq,
		generation: seen.Generation,
	}
	for _, rd := range r.reads {
		if rd.screen != screen {
			continue
		}
		v.before = append(v.before, rd.words)
	}
	if n := len(v.before); n > observe.OutcomeSettleLooks {
		v.before = v.before[n-observe.OutcomeSettleLooks:]
	}
	// THE SELF-TEST, from the readings taken before anything was done.
	//
	// Pairs of consecutive windows of this screen's readings, with no action between
	// them. Anything that "appears and stays" across one of those is the screen moving by
	// itself — Rocket League's main menu did it in 2 to 12 of every 25 to 64 windows, and
	// Ark's settings panel in 0 of 54.
	v.quiet, v.moved = observe.Stationary(r.art.OutcomeExcluded(step),
		windowsOf(v.before))
	r.verify = v
}

// readsOf is how many readings of one screen this run is holding.
//
// The question `openVerification` would ask, asked BEFORE the action instead of after it: a
// step whose outcome has to be verified may not be acted on until the screen it stands on has
// been read at least once, or the verdict is decided against the application by an omission of
// Marco's. See the wait in `Advance`.
//
// # The per-screen filter is defence in depth, and a mutation run says so
//
// Removing it — counting every reading this run holds — survives the suite, and it is not a
// coverage gap that a test would close. Reaching it needs a run that has read screen X while
// standing on unread screen Y, and `Advance` cannot produce that: `noteReading` is called after
// the usability gate and before establishment, so the reading and the step both belong to
// whatever screen the look carried. A look Marco could act on is a look Marco read.
//
// Kept because the screen is what the verification is opened on, and a reader of this function
// has to be able to trust that "read" means read HERE. The day a grounder holds a screen across
// a change, this is already right. The same argument the `asked` term in `verdict` makes, and
// the same one `observe.Independent`'s site filter makes about a word it cannot change.
func (r *Runner) readsOf(screen observe.VisualScreenID) int {
	n := 0
	for _, rd := range r.reads {
		if rd.screen == screen {
			n++
		}
	}
	return n
}

// windowsOf splits a run of readings into consecutive pairs of windows for the self-test.
//
// Halves, so each side is as long as the evidence a real verification rests on. Fewer than two
// readings yields nothing, and `Stationary` then reports 0 quiet windows — which `OutcomeHeld`
// treats as "the question was not asked" rather than as a pass.
func windowsOf(reads [][]observe.ReadWord) [][][]observe.ReadWord {
	if len(reads) < 2 {
		return nil
	}
	half := len(reads) / 2
	return [][][]observe.ReadWord{reads[:half], reads[half:]}
}

// verdict asks what the evidence gathered so far says, or that it is not yet enough.
//
// Returns VerdictUnasked while the answer is still coming: the caller keeps waiting, within
// its own patience, until this returns something decided or the deadline turns it into
// UNCONFIRMED.
func (r *Runner) verdict(now time.Time) (Verdict, string) {
	v := r.verify
	switch {
	case v == nil:
		return VerdictUnasked, ""
	case len(r.art.Steps[v.step].Outcome) == 0:
		// NOTHING TO CHECK AGAINST, which is not a refusal. Every artifact saved
		// before outcomes were recorded is in this case, and a run that reported it
		// as a contradiction would be blaming the application for the file.
		return VerdictUnasked, "this step was demonstrated without an outcome, so " +
			"there is nothing its action could be verified against"
	}
	// ── A CONTRADICTION NEEDS BOTH SIDES OF THE ACTION ──
	//
	// `enough` counted the readings AFTER the action only. So an action with five
	// readings after it and NONE before it was reported CONTRADICTED — on the evidence
	// that the question had not been asked.
	//
	// Measured live on the controlled test menu, and it is the first guarded pixel action
	// this project has sent: the press landed, the panel arrived, and the window's own
	// title went from `FXM 0 0` to `FXM 1 1` — an independent witness, outside Marco,
	// saying one press was received and the expected panel was standing. The run called
	// the outcome contradicted. That is blaming the application for a reading Marco never
	// took, and it is the difference between "it did not work" and "I did not look".
	// And it is UNREACHABLE from `Advance` while the wait in section 4b stands, which a
	// mutation run confirms by surviving: 4b will not let a step with an outcome be acted
	// on until the screen it stands on has been read, and `openVerification` takes the
	// before side from the readings of that same screen. So `asked` is false only if this
	// is reached another way.
	//
	// Kept, because it is the statement of what a contradiction IS, and because the live
	// defect it closes was exactly this branch's absence: the run that produced it had a
	// reading of the screen nowhere and blamed the application. A verdict function that
	// could not express "one side is missing" would have to be trusted never to be handed
	// it. See the comment above for what that cost.
	asked := len(v.before) > 0
	// ── AND A VERDICT EITHER WAY NEEDS A FULL SETTLED WINDOW ──
	//
	// The persistence term is "on every reading of the settled window", and over a window
	// of ONE reading it says nothing at all: the first reading after a transition always
	// carries every word it carries. So a verified verdict on one reading is a verdict
	// with the persistence term switched off, and it was reachable — the window restart
	// above empties `after`, and the next reading filled it and verified.
	//
	// Measured live on the controlled test menu: `verified after 774ms on 1 fresh
	// reading(s)`, 18 of 18 expected words, and the press really had worked. The right
	// answer for the right reason is still a verdict this package is not entitled to yet.
	settled := len(v.after) >= observe.OutcomeSettleLooks
	held, why := r.art.OutcomeHeld(v.step, v.before, v.after, v.quiet, v.moved)
	switch {
	case held && settled:
		return VerdictVerified, why
	case held && now.Before(v.deadline):
		// STANDING, AND NOT YET FOR LONG ENOUGH. Keep waiting rather than
		// answering: the words are there and the question is whether they stay.
		//
		// WHILE THERE IS STILL TIME, and that clause is the fix. This branch
		// returned `Unasked` unconditionally, so a window holding one qualifying
		// reading and then receiving no more waited past its own deadline — the
		// verification deadline existed and could not expire, and the run went on
		// until some other bound ended it. Positive partial evidence is the one
		// case where waiting feels most reasonable and is least justified: the
		// whole question is whether those words STAY, and readings have stopped.
		//
		// Past the deadline this falls through to the unconfirmed branches below,
		// which is the honest answer — the words were seen and never shown to
		// stand.
		return VerdictUnasked, fmt.Sprintf("the %d expected word(s) are standing on "+
			"%d of the %d reading(s) persistence needs",
			len(r.art.Steps[v.step].Outcome), len(v.after),
			observe.OutcomeSettleLooks)
	case asked && settled:
		// ENOUGH EVIDENCE, AND IT DOES NOT AGREE. The only case in which a refusal
		// is a statement about the application rather than about the evidence.
		return VerdictContradicted, why
	case !now.Before(v.deadline) && !asked:
		return VerdictUnconfirmed, fmt.Sprintf("%s elapsed after the action and the "+
			"screen it was sent to was never read BEFORE it, so the %d reading(s) "+
			"since have nothing to be compared against (%s)",
			now.Sub(v.acted).Round(time.Millisecond), len(v.after), why)
	case !now.Before(v.deadline):
		return VerdictUnconfirmed, fmt.Sprintf("%s elapsed after the action and %d of "+
			"the %d reading(s) the check needs arrived (%s), so whether it "+
			"worked is unknown — which is not the same as its having failed",
			now.Sub(v.acted).Round(time.Millisecond), len(v.after),
			observe.OutcomeSettleLooks, why)
	default:
		return VerdictUnasked, fmt.Sprintf("%d of %d reading(s) since the action, %s "+
			"of %s elapsed", len(v.after), observe.OutcomeSettleLooks,
			now.Sub(v.acted).Round(time.Millisecond), r.VerifyDeadline())
	}
}

// closeVerification records a decided verdict and stops gathering.
func (r *Runner) closeVerification(v Verdict, why string, now time.Time) Verification {
	out := Verification{
		Step: r.verify.step, Verdict: v, Because: why,
		Took:     now.Sub(r.verify.acted),
		Readings: len(r.verify.after), Before: len(r.verify.before),
		Quiet: r.verify.quiet, Moved: r.verify.moved,
		Restarts: r.verify.restarts, SawScreens: r.verify.sawScreens,
	}
	r.verifications = append(r.verifications, out)
	r.verify = nil
	return out
}

// Verification is one action's post-action evidence, as a record.
type Verification struct {
	Step    int
	Verdict Verdict
	Because string
	// Took is from the action going out to the verdict, and Readings how many fresh
	// readings of the screen it rested on. Reported because the deadline was chosen from
	// an offline rate and these are what would revise it.
	Took     time.Duration
	Readings int
	Before   int
	// Quiet and Moved are the stationarity self-test on the screen that was acted on.
	Quiet, Moved int
	// Restarts is how many times a transition landed inside the settled window, and
	// SawScreens the other screen ids the after-readings were taken under.
	Restarts   int
	SawScreens []observe.VisualScreenID
}

// Summary is one line about one verification.
func (v Verification) Summary() string {
	return fmt.Sprintf("step %d %s after %s on %d fresh reading(s) of %d before: %s",
		v.Step, v.Verdict, v.Took.Round(time.Millisecond), v.Readings, v.Before,
		v.Because)
}

// Verifications is what each action's evidence said.
func (r *Runner) Verifications() []Verification {
	return append([]Verification{}, r.verifications...)
}

// tallyVerifications fills a report's verdict counts and the measured reading rate.
//
// Derived rather than accumulated, like `Report`'s other sums: a count kept beside the list it
// counts is a count that can disagree with it.
func (r *Runner) tallyVerifications(out *Report) {
	for _, v := range r.verifications {
		switch v.Verdict {
		case VerdictVerified:
			out.VerifiedOK++
		case VerdictContradicted:
			out.Contradicted++
		case VerdictUnconfirmed:
			out.Unconfirmed++
		}
	}
	// THE RATE THE READINGS ACTUALLY ARRIVED AT, over the span they arrived in. Not
	// over the run's whole length: a run that waited five minutes for a person to answer
	// a question did not spend those minutes failing to read.
	if span := r.lastRead.Sub(r.firstRead); span > 0 && r.readings > 1 {
		out.ReadingsPerSecond = float64(r.readings-1) / span.Seconds()
	}
}

// hasScreen reports whether a screen id is already noted.
func hasScreen(seen []observe.VisualScreenID, id observe.VisualScreenID) bool {
	for _, s := range seen {
		if s == id {
			return true
		}
	}
	return false
}
