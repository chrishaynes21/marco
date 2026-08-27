package main

import (
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// Where a thing seen twice becomes a thing Marco knows.
//
// # The three jobs, and they are deliberately separate
//
//	MATCH      is this the same relationship I have seen before?   — structure, not a hash
//	FOLD       add one sighting to what I already had              — pure, in ambient
//	JUDGE      has it earned durable memory?                       — pure, in ambient
//	ADMIT      make it durable                                     — the 36B boundary, reused
//
// Only the first and last are here, because only they need to reach anything: the matcher needs
// the canonical structure comparison, and the admission needs the stores. The two in the middle
// are pure functions over a summary and a policy, which is what makes the promotion rule testable
// without a desktop, a session or a file.
//
// # It runs on evidence, never on a clock
//
// There is no promotion loop. This is called from the one place a new semantic transition is
// recorded — a CHANGE, which happens when somebody does something — so an unchanged desktop
// costs nothing at all and 36A's idle profile is untouched. See ADR-095.

// noticed folds one observed transition into the candidate ledger, and promotes what has earned
// it.
//
// # Human work only
//
// A step Marco produced itself creates nothing and strengthens nothing. A play running while
// watching is on moves the screen exactly as a person would, and counting that as "I have seen
// the human do this again" is how a system comes to learn its own behaviour from itself — and
// then to be more confident about it every time it runs.
//
// Gated HERE, where evidence is collected, rather than in the policy: a candidate that could
// never be promoted is a candidate not worth keeping, and a refusal word for a case nothing can
// produce is a reader being told about a decision made nowhere.
//
// Deleting the provenance check must fail TestMarcosOwnWorkIsNotEvidenceOfAHabit.
func (a *ambientObserver) noticed(s ambient.Step, sameSession bool) {
	if s.By != ambient.ByHuman {
		return
	}
	store, ok := a.rt.watchedStore()
	if !ok {
		return
	}
	policy := a.policy()

	// ONE ACTION PER CANDIDATE. A leg with several — a menu opened and an item chosen inside
	// it, arriving somewhere in one transition — describes two things somebody did and one
	// change; splitting it would invent two relationships that were never separately observed,
	// and keeping it whole would need a durable representation of a compound action that does
	// not exist. So it is not candidate evidence, and explicit Learn remains the way to keep
	// it.
	if len(s.Did) != 1 {
		return
	}
	act := s.Did[0]

	from := endOf(s.From, s.FromShape)
	to := endOf(s.To, s.ToShape)
	if from.Subject == "" && from.Shape == nil {
		return
	}
	if to.Subject == "" && to.Shape == nil {
		return
	}

	existing := store.Watched(s.Application)
	// A CONTRADICTION FIRST, because it is the more important fact.
	//
	// The same screen, the same action, the same control, arriving somewhere ELSE. Recorded
	// against the candidate that already claims that beginning rather than resolved: a
	// majority is not an answer when the question is whether Marco understands the screen.
	//
	// Deleting this must fail TestOneControlThatLeadsTwoWaysIsNotLearned.
	for _, w := range existing {
		if !sameAct(w, act) || !sameEnd(w.From, from) {
			continue
		}
		if sameEnd(w.To, to) {
			continue
		}
		_ = store.RememberWatched(ambient.Contradict(w, s.At))
	}

	found := observe.WatchedEdge{}
	for _, w := range existing {
		if sameAct(w, act) && sameEnd(w.From, from) && sameEnd(w.To, to) {
			found = w
			break
		}
	}
	if found.ID == "" {
		found = observe.WatchedEdge{
			Application: s.Application, From: from, To: to,
			Kind: string(act.Kind), Target: act.Target.Label, Role: act.Target.Role,
		}
	}
	folded := ambient.Fold(found, s, sameSession, policy.Gap)
	// THE HANDLE IS ASSIGNED ONCE AND KEPT.
	//
	// It is a handle, not the identity test — exactly like RememberedSubject.ID, and for
	// exactly the same reason. Recomputing it on every fold looked tidier and was wrong: an
	// end whose reading changes between two sightings, or one that becomes recognised, moves
	// the handle, and the store writes a SECOND row beside evidence that already exists. Then
	// neither row ever reaches the threshold and nothing anywhere says why.
	//
	// Measured: one screen read at two window widths produced two candidates, and the
	// structural match that was supposed to unify them had already succeeded.
	//
	// Recomputing it here must fail TestAWideAndANarrowHomeAreOneCandidate.
	if folded.ID == "" {
		folded.ID = observe.WatchedID(folded.Application, folded.From, folded.To,
			folded.Kind, folded.Target)
	}
	if err := store.RememberWatched(folded); err != nil {
		return
	}
	a.mu.Lock()
	a.noticedEdges++
	a.mu.Unlock()

	j := ambient.Judge(folded, policy, s.At)
	if j.Verdict != ambient.Promote {
		return
	}
	if err := a.rt.admitWatched(folded); err != nil {
		return
	}
	folded.Promoted = s.At
	_ = store.RememberWatched(folded)
	a.mu.Lock()
	a.promoted++
	a.mu.Unlock()
}

// endOf describes one end of an observed step for the ledger.
func endOf(key string, shape *ambient.Shape) observe.WatchedEnd {
	if ambient.Recognised(key) {
		return observe.WatchedEnd{Subject: key}
	}
	if shape == nil {
		return observe.WatchedEnd{}
	}
	sig := shape.Signature
	return observe.WatchedEnd{Shape: &sig, Called: shape.Called}
}

// sameAct reports whether a candidate is about the same thing being done to the same control.
//
// The name is compared case-insensitively and nothing else is compared at all — no position, no
// bounds, no provider handle. A control is what it is CALLED, in a place; see ambient.Target.
func sameAct(w observe.WatchedEdge, act ambient.Act) bool {
	return w.Kind == string(act.Kind) &&
		strings.EqualFold(strings.TrimSpace(w.Target), strings.TrimSpace(act.Target.Label))
}

// sameEnd reports whether two ends are the same screen.
//
// # Structure, through the one comparison
//
// A recognised end is its subject id, and two of those are the same when the ids are. An
// unrecognised end is a structure, and two of those are the same when observe.CompareStructure
// says so — the canonical identity test, which is what 35D's aliasing already runs through, so a
// wide Settings Home and a narrow one strengthen one candidate rather than two.
//
// A hash would not do. The store itself matches signatures with tolerance and not by equality,
// precisely because two readings of one screen differ in small ways; keying candidates on an
// exact digest would split the evidence for the same screen across two records, neither of which
// ever reaches a threshold, and nothing would ever say why.
//
// Deleting the structural comparison must fail TestAWideAndANarrowHomeAreOneCandidate.
func sameEnd(a, b observe.WatchedEnd) bool {
	if a.Recognised() || b.Recognised() {
		return a.Subject == b.Subject
	}
	if a.Shape == nil || b.Shape == nil {
		return false
	}
	return observe.CompareStructure(*a.Shape, *b.Shape) == observe.MatchSame
}

// admitWatched makes one candidate durable, through the boundary an explicit Learn goes through.
//
// # It is the SAME boundary, and that is the whole design
//
// `promotion` is 36B's object: four methods, each refusing without the specific permission it
// needs. Ambient promotion builds one and hands it a one-step demonstration, so a place is
// established by the same call, an edge related by the same call, a control named by the same
// call. There is no `PromoteAmbientEdge` with rules of its own, because two admission paths would
// eventually be two policies and only one of them would be reviewed.
//
// # What it does NOT do, and the list is the point
//
// No goal. No play. No name invented from anonymous behaviour. No rehearsal, no grant, no lease,
// no input. It writes memory and stops — which is why `LearnRecent` keeps those four steps and
// this does not share them.
//
// Deleting the licence must fail TestAmbientPromotionCannotWriteWithoutItsLicence.
func (r *Runtime) admitWatched(w observe.WatchedEdge) error {
	memory, ok := r.durableMemory()
	if !ok {
		return errNoMemory
	}
	places, _ := memory.(observe.PlaceStore)
	candidates, _ := memory.(observe.CandidateStore)
	targets, _ := memory.(observe.TargetStore)

	p := promotion{
		licence: ambientPromotionLicence(), application: w.Application,
		memory: memory, places: places, candidates: candidates, targets: targets,
	}
	steps, _, err := resolvePlaces(p, watchedDemonstration(w))
	if err != nil {
		return err
	}
	if _, err := p.relate(steps); err != nil {
		return err
	}
	for _, s := range steps {
		c := candidateFor(w.Application, s, observe.MatchSame, w.Seen)
		if err := p.remember(c); err != nil {
			return err
		}
		if _, err := p.name(c); err != nil {
			return err
		}
	}
	return nil
}

// watchedDemonstration renders a candidate as the one-step walk the admission boundary takes.
func watchedDemonstration(w observe.WatchedEdge) ambient.Demonstration {
	step := ambient.Step{
		Application: w.Application, By: ambient.ByHuman, At: w.Last,
		From: endKey(w.From, "from"), To: endKey(w.To, "to"),
		FromShape: endShape(w.From), ToShape: endShape(w.To),
		Did: []ambient.Act{{Kind: ambient.ActionKind(w.Kind),
			Target: ambient.Target{Role: w.Role, Label: w.Target}}},
	}
	return ambient.Demonstration{Application: w.Application, Steps: []ambient.Step{step},
		From: step.From, To: step.To}
}

// endKey and endShape put one end back into the shape the admission boundary reads.
//
// # Why the two ends need DIFFERENT transient names
//
// Because the admission boundary resolves transient endpoints through a map keyed on the name it
// is given, so that a screen appearing as one leg's destination and the next leg's source is
// established once. Handing both ends of one relationship the same name makes it establish one
// place and produce an edge from that place to itself — which the store then refuses, having no
// such relationship, and the promotion fails with a message about a subject that appears twice.
//
// Measured: it did exactly that, and the two ends had genuinely different structures the whole
// time. The bug was entirely in the name.
//
// Deleting the side must fail TestWatchingTheSameThingTwiceIsRemembered.
func endKey(e observe.WatchedEnd, side string) string {
	if e.Recognised() {
		return e.Subject
	}
	// A NAME THAT IS NOT A SUBJECT ID, so the boundary establishes rather than looks up:
	// ambient.Recognised refuses it by its prefix.
	return ambient.TransientPrefix + "watched_" + side
}

func endShape(e observe.WatchedEnd) *ambient.Shape {
	if e.Recognised() || e.Shape == nil {
		return nil
	}
	return &ambient.Shape{Signature: *e.Shape, Called: e.Called}
}

// ambientPromotionLicence is what a person enabling ambient learning agreed to.
//
// # The same three permissions an explicit Learn declares, and the reason is not convenience
//
// Admitting one relationship needs all three — the screens either side have to be recognisable,
// the transition has to become route evidence, and the control has to keep the name that makes it
// findable — so a narrower licence would not make promotion safer, it would make it fail in the
// middle and leave half a relationship behind.
//
// The conservatism lives in the POLICY, which is where it belongs: what qualifies is decided by
// repeated, uncontradicted, well-described evidence, and a candidate that has not earned it never
// reaches this function. A weakened licence would be a second, quieter policy in a place nobody
// looks.
//
// # And the human semantic event behind it
//
// Somebody turned ambient learning on. That is the consent — an explicit act, a separate switch
// from watching, off by default, and reported by status — which is exactly why 36C insists the two
// be separately controllable. See ADR-095.
func ambientPromotionLicence() observesession.Licence {
	return observesession.LearnLicence()
}

// policy is the promotion rule this observer runs under.
func (a *ambientObserver) policy() ambient.Policy {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.promotion
}

// durableMemory and watchedStore are the two stores promotion needs, read once.
func (r *Runtime) durableMemory() (observe.Memory, bool) {
	if r == nil || r.observations == nil {
		return nil, false
	}
	r.observations.mu.RLock()
	m := r.observations.memory
	r.observations.mu.RUnlock()
	return m, m != nil
}

func (r *Runtime) watchedStore() (observe.WatchedStore, bool) {
	m, ok := r.durableMemory()
	if !ok {
		return nil, false
	}
	s, ok := m.(observe.WatchedStore)
	return s, ok
}

// errNoMemory is what a Director with nowhere to remember says.
var errNoMemory = &noMemory{}

type noMemory struct{}

func (*noMemory) Error() string {
	return "this Director has no durable memory, so it cannot learn from what it watches"
}

// promotionSince is a diagnostic helper: what the ledger has done, for status.
func (a *ambientObserver) promotionCounts() (noticed, promoted int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.noticedEdges, a.promoted
}

// AmbientEvidence is what Marco has seen repeatedly, and what it is waiting for.
//
// # The question this answers, and how long it had no answer
//
// "Marco has noticed four relationships and learned none of them" is a true report that tells
// somebody nothing they can act on. Is it one occasion short? Is the control unnameable? Does that
// button lead two different places? Those are four different situations with four different things
// to do about them, and until this existed the only way to tell was to open the store and run the
// policy in your head.
//
// The policy already had the sentences — `ambient.Describe` has said them since the day it was
// written and nothing called it. An unreachable explanation is the same defect as an unreachable
// discriminator, and this is what makes it reachable.
//
// # A read, and it changes nothing
//
// It judges, which is a pure function, and reports. It establishes nothing, promotes nothing and
// writes nothing — a diagnostic that promoted what it was asked about would be the worst possible
// answer to "what are you waiting for".
//
// Deleting the Judge call must fail TestAskingWhatMarcoIsWaitingForSaysWhy.
func (r *Runtime) AmbientEvidence() []service.WatchedView {
	store, ok := r.watchedStore()
	if !ok {
		return nil
	}
	a := r.ambient()
	policy := a.policy()
	now := time.Now()

	// EVERY APPLICATION, not just the one in front. Somebody asking what Marco has recorded
	// about them is not asking about this window.
	seen := map[string]bool{}
	var out []service.WatchedView
	for _, app := range r.applicationsWatched() {
		if seen[strings.ToLower(app)] {
			continue
		}
		seen[strings.ToLower(app)] = true
		for _, w := range store.Watched(app) {
			j := ambient.Judge(w, policy, now)
			out = append(out, service.WatchedView{
				Application: w.Application, Did: w.Kind, Control: w.Target,
				FromKnown: w.From.Recognised(), ToKnown: w.To.Recognised(),
				Seen: w.Seen, Occasions: w.Occasions, Contradicted: w.Contradicted,
				Verdict: string(j.Verdict), Why: string(j.Why),
				Said: ambient.Describe(j), Short: j.Short,
				Learned: !w.Promoted.IsZero(), LastSaw: w.Last.Format(time.RFC3339),
			})
		}
	}
	return out
}

// applicationsWatched is every application the candidate ledger holds evidence about.
//
// Read from the store rather than remembered, because the ledger outlives the observer: evidence
// from a Director that ran yesterday is still evidence, and a list built from what THIS process
// happens to have seen would hide it.
func (r *Runtime) applicationsWatched() []string {
	memory, ok := r.durableMemory()
	if !ok {
		return nil
	}
	lister, ok := memory.(interface{ WatchedApplications() []string })
	if !ok {
		return nil
	}
	return lister.WatchedApplications()
}
