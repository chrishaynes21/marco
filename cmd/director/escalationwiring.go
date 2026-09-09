package main

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// IS MORE PERCEPTION WORTH BUYING RIGHT NOW?
//
// The one place production asks, and it asks observe.EscalationOf rather than deciding. The
// policy names no sensor; this is the caller that happens to hold an expensive one.
//
// # What it reads, and why that is the previous reading
//
// The gate runs INSIDE a collection cycle, and the sufficiency of that cycle does not exist
// until fusion has finished with it. So what it reads is the last reading the session settled
// on — which is correct for the question being asked. This is a BUDGET decision, not an
// evidence one: nothing it returns becomes belief, and a stale answer costs at worst one
// inference that was not needed or one that was.
//
// # IT RUNS INSIDE THE PERCEPTION CYCLE, SO IT MAY TAKE NO LOCK THE CYCLE HOLDS
//
// Both callers ask from within `liveSampler.Sample`, which holds `Runtime.mu` for the whole
// collect-and-fuse so the pinned window cannot move under the providers: `liveSampler.request`
// is evaluated as an argument inside that section, and the shadow provider's own gate is called
// deeper still, from inside `Collect`.
//
// `sync.Mutex` is not reentrant. When `incompleteFor` took `Runtime.mu`, the second sample of
// EVERY observation session blocked forever — Learn, Light Mode, ambient watching, and the fresh
// look a performance takes to find out where it is. The first sample survived only because
// nothing was settled yet, so this returned at `!p.Placed` before reaching the lock, which is
// what made a hang look like a slow start.
//
// Measured on Windows Settings: 14 samples in 12 seconds with the gate bypassed, 1 sample and
// then silence with it. Nothing in an 85-package suite saw it, because every test of this gate
// calls it directly and no test held the cycle's lock.
//
// So: `incompleteSince` has its own mutex, and anything added here must not reach for `mu`.
// Enforced by TestTheSensorGateDoesNotReenterTheCycleLock.
//
// It must never be read the other way round. "The last reading was sufficient" is not a claim
// about the current screen and may not stand in for one.
// evidencePurchase is what a cycle decided to buy, and why.
//
// Two fields rather than a boolean because the caller cannot otherwise tell a repair from
// ordinary curiosity, and the two want different passes. See the Read field.
type evidencePurchase struct {
	// Buy is whether another pass is worth its cost at all.
	Buy bool
	// Read is whether THIS purchase must carry the reading budget.
	//
	// Set only for the bounded repair a semantically silent screen earns, and never for
	// ignorance — which is unbounded, recurs on every sample, and would therefore make
	// reading continuous. The distinction is the whole point: see moreEvidenceIsWorthBuying.
	Read bool
}

// noteEscalation records the last purchase decision for the diagnostic. Its own mutex, never
// Runtime.mu: this is reached from inside the perception cycle's lock.
func (r *Runtime) noteEscalation(format string, args ...any) {
	if r == nil {
		return
	}
	r.escalationMu.Lock()
	r.escalation = fmt.Sprintf(format, args...)
	r.escalationMu.Unlock()
}

// lastEscalation is the sentence noteEscalation left.
func (r *Runtime) lastEscalation() string {
	if r == nil {
		return ""
	}
	r.escalationMu.Lock()
	defer r.escalationMu.Unlock()
	return r.escalation
}

func (r *Runtime) moreEvidenceIsWorthBuying() evidencePurchase {
	if r == nil || r.observations == nil {
		r.noteEscalation("nothing is watching, so ignorance buys a look")
		return evidencePurchase{Buy: true}
	}
	p := r.observations.placeHere()
	if !p.Placed {
		// IGNORANCE, NOT AN ANSWER. No session running, no memory, or nothing settled
		// yet. None of those says the current reading is good enough.
		//
		// It buys a look and NOT a reading. This arm is unbounded — an unplaced reading
		// is unplaced on every sample — so setting Read here is exactly the continuous
		// OCR the budget below exists to refuse.
		r.noteEscalation("nothing is placed yet, so ignorance buys a look and not a reading")
		return evidencePurchase{Buy: true}
	}

	// NeedAnswer: the conservative middle. Passive watching would decline more often and
	// acting would demand more, and this gate serves whatever the session is for — which it
	// cannot see from here. Choosing the stricter of the two it might be would spend on
	// background curiosity; choosing the looser would starve a question.
	structural, sem := observe.SufficiencyOf(p), observe.SemanticSufficiencyOf(p)
	// SEMANTIC SILENCE MAY BUY, ONCE PER SETTLED OBSERVATION EPOCH.
	//
	// # Why a budget at all
	//
	// A structurally-sufficient reading that says nothing about which state it is will say
	// nothing about it again on the next cadence, and the one after. Without a bound this
	// would buy a 645–1379ms inference every sample for as long as somebody left that screen
	// in front — which is the expense 37C and 36A both exist to refuse, arriving from a new
	// direction. The answer changes when the SCREEN changes, so that is how often it is worth
	// asking.
	//
	// # Why it is keyed on the settled screen state and not on the Place
	//
	// Because the Place is the thing that is currently wrong. Xbox collapses every game into
	// one durable subject, so a budget keyed on it would let the first game spend and then
	// starve every other game of the repair that might tell them apart — the broken identity
	// silently preventing its own fix.
	//
	// `ScreenStateID` is the segmenter's own transient answer to "the screen materially
	// changed", session-local, upstream of recall and of everything durable. It is exactly
	// the lifecycle this budget wants, and it already exists.
	//
	// The session travels with it for the reason `transientKey` carries one: `state_2` in two
	// sessions are unrelated screens, and a budget that confused them would decline a repair
	// on a screen it has never seen.
	//
	// # It is transient and is never knowledge
	//
	// Nothing here is written down, nothing survives the process, and nothing about it reaches
	// a Place, a plan or an input. It is a spending record.
	//
	// Deleting the budget must fail TestOneSilentScreenBuysOneRepair.
	silent := sem.State == observe.StateSilent
	if silent && !r.mayRepairNow() {
		// THE TWO REFUSALS ARE DIFFERENT FACTS. `mayRepairNow` says no both when the
		// budget for this epoch is spent and when there is no settled epoch to budget
		// against at all, and those have opposite remedies — the first is the bound
		// working, the second is a screen that never held still long enough to be worth
		// buying evidence for. Reported apart because one reading cannot tell them apart.
		_, state := r.observations.currentEpoch()
		why := "the repair for this settled screen is already spent"
		if state == "" {
			why = "no screen state has settled, so there is no epoch to budget against"
		}
		r.noteEscalation("structurally %s, semantically %s; %s",
			structural.State, sem.State, why)
		return evidencePurchase{}
	}
	worth := observe.EscalationOf(observe.NeedAnswer, structural, sem,
		r.incompleteFor(p)).Worth()
	// AND THE REPAIR IS ALLOWED TO ANSWER THE QUESTION IT WAS BOUGHT FOR.
	//
	// A semantically silent screen buys one pass, once. Whether that one pass could READ
	// used to be decided somewhere else entirely — `req.ReadLabels`, the session's
	// LabelEvery cadence, which knows nothing about repairs. So the single expensive token
	// bought BECAUSE structured perception could not say which state this is had roughly a
	// 1-in-LabelEvery chance of performing the reading that is the only thing that could
	// say. The rest of the time it was spent blind and the screen stayed nameless until it
	// changed.
	//
	// Measured on Epic Games Launcher, whose accessibility tree exposes one element: the
	// detector finds 31 controls on every pass, deterministically, and a pass carrying the
	// reading budget names eight of them — "Store", "Library", "Unreal Engine", "Buy Now".
	// The sensing was never the problem. The repair simply could not use it.
	//
	// So Read rides on the repair and on nothing else. This is not "OCR more often": it is
	// the same one bounded purchase per settled screen state, spent on a pass capable of
	// answering. Ordinary sampling is untouched and still follows LabelEvery.
	//
	// Deleting this must fail TestTheSemanticRepairCanReadWhatItWasBoughtToRead.
	r.noteEscalation("structurally %s, semantically %s; buy=%t read=%t",
		structural.State, sem.State, worth, worth && silent)
	return evidencePurchase{Buy: worth, Read: worth && silent}
}

// mayRepairNow claims this settled screen's one semantic repair, and reports whether it was
// still there to claim.
//
// # It claims rather than asks
//
// Because the caller acts on the answer immediately, and a gate that reported "yes" without
// spending the budget would report "yes" again on the next sample. The two have to be one
// operation or the bound is advisory.
//
// A reading with no settled screen state claims nothing and is refused: an epoch that cannot be
// named cannot be budgeted, and spending against it would be spending without a bound.
func (r *Runtime) mayRepairNow() bool {
	if r == nil || r.observations == nil {
		return false
	}
	session, state := r.observations.currentEpoch()
	if state == "" {
		return false
	}
	key := string(session) + ":" + string(state)
	r.repairedMu.Lock()
	defer r.repairedMu.Unlock()
	if r.repaired == key {
		return false
	}
	r.repaired = key
	return true
}

// incompleteFor is how long the reading has been incomplete, zero when it is not.
//
// Time CORROBORATES here and never classifies — 37D's judgement never looks at a clock, and
// this does not change that. All it decides is whether the cheap remedy has had its chance: a
// page that has been blank for two seconds is not still arriving.
func (r *Runtime) incompleteFor(p observe.Place) time.Duration {
	now := sessionClock.Now()
	r.incompleteSinceMu.Lock()
	defer r.incompleteSinceMu.Unlock()
	if observe.SufficiencyOf(p).State != observe.Incomplete {
		r.incompleteSince = nil
		return 0
	}
	if r.incompleteSince == nil {
		at := now
		r.incompleteSince = &at
		return 0
	}
	return now.Sub(*r.incompleteSince)
}

// reopenRepair returns the current epoch's semantic repair to the budget.
//
// # Why an epoch can go stale without its NAME changing
//
// The budget is keyed on the segmenter's settled screen state, which is structural: it is
// derived from the fused world, and on an application whose accessibility tree exposes a single
// window that signature barely moves. Measured on Epic Games Launcher — a human navigation
// repainted 32.5% of the window, then 24.1%, and `ScreenStateID` did not change once. So the
// only sensor that could tell those screens apart was gated behind a discriminator that could
// not see they were different, and the application went blind for the rest of the session.
//
// The caller establishes that the previously repaired scene is no longer the visible one, from
// CURRENT pixel evidence that has settled. This does no more than return the token: the next
// eligible acquisition claims it in the ordinary way, once, through mayRepairNow.
//
// # What this is not
//
// It mints no Place, changes no screen signature, touches no durable identity and creates no
// graph endpoint. It says one thing: the answer bought for the previous scene does not describe
// the current one. See [[ADR-132-a-repaired-answer-can-stop-describing-the-screen]].
//
// Deleting this must fail TestASettledNewSceneReopensTheRepair.
func (r *Runtime) reopenRepair() {
	if r == nil {
		return
	}
	r.repairedMu.Lock()
	r.repaired = ""
	r.repairedMu.Unlock()
}
