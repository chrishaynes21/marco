package rehearse

import (
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// StageEvidence is one positively established fact about where Marco is.
//
// # Reusing proof is not skipping proof
//
// Marco proves the same unchanged fact several times in one execution. A two-edge play looks at
// the screen to plan, establishes its source before edge one, verifies the outcome of edge one,
// establishes that same outcome again as the source of edge two, verifies the outcome of edge two,
// and then looks once more to confirm arrival. Six establishments for three screens, and each
// costs a run of accessibility snapshots and a settle.
//
// Two of those are redundant only in the sense that nothing had changed — and "nothing had
// changed" is a claim that has to be justified, not assumed. That is what this type is for: it
// carries a proof forward together with everything needed to decide whether the proof still holds.
//
// The rule the whole optimization rests on:
//
//	Marco may avoid proving the same unchanged fact twice.
//	Marco may not act on a fact it can no longer justify.
//
// # Nothing new identifies anything
//
// The identity is [windowref.Ref], which already carries the window id, the owning process, the
// application, and a GENERATION — the tracker's epoch, which advances when the window a reference
// names stops being the window it named. Inventing a second identity here would be a second answer
// to "is this the same window", and the same-process ambiguity this repository already fixed once
// (Settings, XBOX and Realtek all being `applicationframehost`) is exactly what a weaker identity
// would let back in.
//
// It is also provider-neutral by construction. A window and a Place are semantic facts; that
// Accessibility is what saw them today is provenance, not identity, and an OCR or fused observer
// producing the same two facts produces the same evidence.
type StageEvidence struct {
	// Ref is the window this was established on, with its generation.
	Ref windowref.Ref
	// Subject is the durable Place that was positively established. Never empty in valid
	// evidence: "I could not tell" is the absence of evidence, not a kind of it.
	Subject string
	// At is when it was established, on the injected clock.
	At time.Time
	// From is how it came to be known. See EvidenceSource.
	From EvidenceSource
}

// EvidenceSource is how a Stage fact came to be known.
//
// The distinction earns its place because the same Place can be established four ways in one
// execution and they are not equally strong. A post-action VERIFIED outcome is the best evidence
// there is that Marco is standing where it thinks: it was taken after the thing that would have
// changed the screen, and it was checked against what the edge said should happen.
//
// Provenance is not authority. None of these permits anything; they describe how a fact was
// learned, and the grant is a separate object with a separate lifetime.
type EvidenceSource string

const (
	// EvidencePlanning is the fresh look taken to decide where Marco is before planning.
	EvidencePlanning EvidenceSource = "planning"
	// EvidenceEstablished is a walker's own look at its source before acting.
	EvidenceEstablished EvidenceSource = "established"
	// EvidenceVerifiedOutcome is a destination positively verified AFTER an action.
	EvidenceVerifiedOutcome EvidenceSource = "verified_outcome"
)

// MaxEvidenceAge bounds how long a Stage fact may be carried at all.
//
// # Why there is a bound, and why it is not the rule
//
// Time is the weakest of the invalidators and the only one that is always available. Everything
// that matters — the foreground moving, the window changing, an action being taken, a
// contradictory reading — is checked directly by [StageEvidence.Justifies], and a fact that
// survives all of those is very probably still true.
//
// But "very probably" is doing work no clock can check. Between two edges Marco is not watching
// continuously; something can happen on the desktop that leaves the window in front, the
// generation intact, and the screen different. The bound is the admission that the checks below
// are not omniscient, and it is deliberately short: it is measured against the gap between one
// verified outcome and the next edge, which is milliseconds, not against how long a person might
// leave a window open.
//
// It must never be the ONLY check. Evidence that is young and contradicted is worthless, and the
// mutation that makes a timestamp sufficient is one this package is tested against.
const MaxEvidenceAge = 3 * time.Second

// Justifies reports whether this evidence still supports acting on `wantSubject` in `application`
// right now.
//
// # Fail closed, and say nothing when unsure
//
// Every condition is a reason to REFUSE to reuse, never a reason to accept. A caller that gets
// false has lost nothing but the optimization: it establishes for itself, exactly as it always
// did. There is no path here that lets weak evidence through — the worst outcome of a bug in this
// function is that Marco does the work twice.
//
// `inFront` is the caller's foreground predicate, the same one the walker's own gate uses. Passing
// nil means the caller cannot check, and evidence that cannot be checked against the foreground is
// refused: an unverifiable claim about which window leads is exactly the claim that would put
// input into somebody else's window.
//
// Deleting any arm must fail a case of TestCarriedEvidenceIsRefusedWhenItCannotBeJustified.
func (e StageEvidence) Justifies(now time.Time, application, wantSubject string,
	inFront func(windowref.Ref) bool) bool {

	// A WINDOW IS PART OF THE EVIDENCE, and this arm cannot be folded into the foreground
	// check below. `inFront` is allowed to be permissive — the production predicate answers
	// TRUE when it cannot look a handle up, because a window that has gone is somebody else's
	// guard to raise — so an empty reference would be waved through down there. It is refused
	// here, where the question is whether there is anything to check at all.
	if e.Ref.ID == "" {
		return false
	}
	// THE PLACE THE CALLER IS ABOUT TO ACT FROM. Evidence for a different screen says nothing
	// about this one, however fresh it is. An empty subject on either side falls here too:
	// neither an unknown place nor an unasked question is a match, and `e.Subject == ""` is
	// checked nowhere else because this is where it means something.
	if wantSubject == "" || e.Subject != wantSubject {
		return false
	}
	// THE APPLICATION. Evidence established on Settings cannot authorise a step in Chrome.
	if application != "" && !strings.EqualFold(e.Ref.Application, application) {
		return false
	}
	// TIME, as the backstop and never as the rule. See MaxEvidenceAge.
	if e.At.IsZero() || now.Sub(e.At) > MaxEvidenceAge || now.Before(e.At) {
		return false
	}
	// AND THE WINDOW MUST STILL LEAD. The foreground gate, asked of the evidence rather than
	// of whatever happens to be in front now — which is the same question, because the check
	// is "does THIS reference still lead".
	if inFront == nil {
		return false
	}
	return inFront(e.Ref)
}

// There is deliberately no `SameWindow` here, and it was written before it was removed.
//
// Comparing a proof's window against a reference a caller now holds reads like the natural
// primitive for this type, and every use of it turned out to be measurably equivalent to nothing.
// The reason is that no caller ACTS on the proof's window: `confirmCarried` returns the reference
// it just acquired, the foreground gate asks about that one, and the step loop re-acquires and
// compares before every step. A proof about a window Marco is not using cannot authorise anything
// through this type, so a check saying so changed no outcome anywhere.
//
// Window identity is held in the two places it does bite. [StageEvidence.Justifies] requires the
// proof's own window to still LEAD the desktop, which is what refuses a proof taken on Settings
// once XBOX has come forward — both `applicationframehost`, which is exactly the ambiguity
// ADR-078 fixed. And `sameWindow` in live.go guards the walk step by step, where a window
// changing mid-route is a result rather than a refusal.

// provedBy is the Stage a finished walk may hand to the next one.
//
// # Two conditions, and one of them is unreachable today
//
// A walk proves where Marco is standing only when it COMPLETED — every planned step taken, the
// last one directly verified — and the proof is what perception actually RESOLVED, never what the
// plan said should happen.
//
// Those two are not independent right now: `terminalAfter` only returns CompletedRoute when the
// final step came out DirectlyVerified, and DirectlyVerified means the observation matched the
// expectation. So on a completed route `rec.Observed` and `rec.Expect` are equal by construction,
// and a mutation swapping one for the other survives every lifecycle test. Measured, not assumed.
//
// It reads from the observation anyway, and the guard is written out rather than folded away,
// because the equality is a property of ONE OTHER FUNCTION. The day a route may complete on
// something weaker than a directly verified last step — a contained ending, a route that ends by
// confirming containment — `Expect` becomes a claim about a screen nothing ever resolved, handed
// to the next edge as the place it may act from. That is the one mistake this handoff would make
// expensive, and it would arrive silently.
//
// Held DIRECTLY by TestOnlyACompletedWalkProvesAnything, because the lifecycle cannot reach the
// case: no walk can produce a completed route whose observation and expectation differ.
func provedBy(ref windowref.Ref, terminal Terminal, rec StepRecord, now time.Time) StageEvidence {
	if terminal != CompletedRoute || rec.Observed == "" {
		return StageEvidence{}
	}
	return StageEvidence{
		Ref: ref, Subject: rec.Observed, At: now, From: EvidenceVerifiedOutcome,
	}
}

// Cost is what one walk spent finding out where it was.
//
// # Why a count and a duration, and why they are not the same claim
//
// A duration is what a person feels and it is the only thing anybody actually wants improved. It
// is also the thing a test cannot assert: it depends on the machine, the provider, the size of
// the accessibility tree and what else the desktop is doing. A COUNT is deterministic — a
// fixture can hold "this route reads the screen seven times" exactly — and it is the honest
// deterministic proxy for the duration, because the duration is very nearly `Samples` multiplied
// by what one snapshot costs.
//
// So both are recorded, and they answer different questions. The counts are what the suite gates.
// The durations are what a live measurement reports, and this repository will not guess them.
//
// # Developer-facing, and only that
//
// None of this reaches a sentence anybody reads. It travels on the result so a harness and the
// Advanced surfaces can report it; turning the product's own output into a profiler was
// explicitly not the goal.
type Cost struct {
	// Samples is readings of the screen — the accessibility snapshots, which dominate.
	Samples int
	// Resolutions is how many times a reading was turned into a Place.
	Resolutions int
	// Establishments is full `establish` runs: "where am I", asked from nothing.
	Establishments int
	// Confirmations is shortened checks of a proof already held, and ProofsReused is how
	// many of those agreed. The difference between them is how often the shortcut was
	// attempted and thrown away, which is the number that says whether it is worth having.
	Confirmations int
	ProofsReused  int
	// Looking is wall time spent inside establishment and confirmation, on the injected
	// clock, so a fixture reports a fixture's time.
	//
	// THERE IS NO `Total` HERE, and there was. A tally is a running count read off the
	// walker; how long a walk took is a stopwatch its CALLER holds, and the two do not
	// belong in one type. While they did, `Since` subtracted the counts and left the
	// duration at zero — so a live run reported a route that had taken three and a half
	// seconds as having spent 0 ms inside the walk. A missing measurement rendered as a
	// hard zero: the third time this instrument has failed in the flattering direction.
	//
	// The caller times its own edge. See `costOf` in cmd/director.
	Looking time.Duration
}

// Add folds another walk's cost into this one, so a caller can total a route.
func (c *Cost) Add(o Cost) {
	c.Samples += o.Samples
	c.Resolutions += o.Resolutions
	c.Establishments += o.Establishments
	c.Confirmations += o.Confirmations
	c.ProofsReused += o.ProofsReused
	c.Looking += o.Looking

}

// Spent is what this walker has spent looking, in total, since it was built.
//
// # Why the tally is read off the walker and not returned in the result
//
// A refusal produces no RehearsalResult. That is a deliberate and load-bearing rule -- "Marco
// declined to try" and "Marco tried and it went wrong" are different facts about the world, and
// nothing here produces a result unless a program actually reached a host.
//
// It also means a cost carried on the result is missing from every refused walk. And the refusal
// path is where a walk looks MOST: a shortened confirmation that disagreed, followed by a full
// establishment that could not place the screen -- seven readings, reported as none.
//
// Measured live, and this is why the field moved. A route interrupted by somebody clicking mid-way
// reported five readings for the edge that succeeded and nothing at all for the edge that
// refused, so the route's total understated what it had actually done, in the direction that
// makes the optimization look good. An instrument that under-reports its own worst case is worse
// than no instrument.
//
// So the caller snapshots this either side of the walk. That works on both paths, it does not
// depend on the walker being freshly built, and there is one tally rather than one on the walker
// and a copy on the result that could disagree.
//
// Deleting the reading on the refusal path must fail TestARefusedEdgeReportsWhatItSpent.
func (l *Live) Spent() Cost { return l.cost }

// Since is what has been spent between two readings of a running total.
func (c Cost) Since(before Cost) Cost {
	return Cost{
		Samples:        c.Samples - before.Samples,
		Resolutions:    c.Resolutions - before.Resolutions,
		Establishments: c.Establishments - before.Establishments,
		Confirmations:  c.Confirmations - before.Confirmations,
		ProofsReused:   c.ProofsReused - before.ProofsReused,
		Looking:        c.Looking - before.Looking,
	}
}
