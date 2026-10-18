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
	// acted is when the action went out and deadline when waiting stops.
	acted, deadline time.Time
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
	if r.verify != nil && r.verify.screen == at.Screen &&
		len(r.verify.after) < observe.OutcomeSettleLooks {

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
		r.verify.after = append(r.verify.after, at.Words)
	}
}

// maxKeptReadings bounds the retained readings: two windows' worth plus the self-test's.
const maxKeptReadings = 4 * observe.OutcomeSettleLooks

// openVerification starts gathering post-action evidence for the step just acted on.
//
// Called at the press, because the BEFORE side is the readings already taken — a window that
// began after the action would compare the new panel against itself.
func (r *Runner) openVerification(step int, screen observe.VisualScreenID, at time.Time) {
	v := &verification{
		step: step, screen: screen,
		acted: at, deadline: at.Add(r.VerifyDeadline()),
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
	enough := len(v.after) >= observe.OutcomeSettleLooks
	held, why := r.art.OutcomeHeld(v.step, v.before, v.after, v.quiet, v.moved)
	switch {
	case held:
		return VerdictVerified, why
	case enough:
		// ENOUGH EVIDENCE, AND IT DOES NOT AGREE. The only case in which a refusal
		// is a statement about the application rather than about the evidence.
		return VerdictContradicted, why
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
