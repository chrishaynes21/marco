---
type: decision
status: accepted
date: 2026-08-20
supersedes: []
affects:
  - demonstrations
  - learned-plays
  - planning
source_paths:
  - internal/director/learn/learn.go
  - internal/director/learn/admission_test.go
  - cmd/director/reach.go
---

# ADR-089 — watching is how Marco learns, performing is how it proves

## Learning something required Marco to do it first

Until now a demonstrated route could not be written down until Marco had **rehearsed every edge of
it**: "I think I got it. Want me to try?", a yes, a grant, and Marco driving the real desktop. Two
edges meant two questions and two live attempts, to learn something the person had just finished
showing it. `EdgeStatus` said so in as many words — *"EdgeVerified rehearsed and arrived. The only
status that makes a route executable."*

That was a reasonable thing to build while the vertical was being proved. It is not a reasonable
thing to ask of somebody who has just demonstrated what they wanted.

## What the measurement found, and it is the whole decision

The rehearsal question was raised under **exactly the conditions that already made the evidence
sufficient**:

- `CandidateConsistent` — *"every checkpoint is verifiable, the navigation has a clear shape, and
  nothing blocks a future attempt from being CHECKED"*
- nothing in `Blocking()` — the closed set of uncertainties another example must settle

Anything less never reached a rehearsal at all: it was refused upstream as `evidence_insufficient`,
or diverted to `NeedsAnotherExample`. There was no middle state — no "good enough to try, not good
enough to keep".

So the question obtained **no information**. It obtained a *permission*, for an action nobody had
asked Marco to take. That is ceremony, and it was measured rather than assumed: the first attempt
at this roadmap supplied a deliberately weaker demonstration on the theory that a middle state
existed, and the coordinator refused it before rehearsal was ever considered.

## The decision

**A clean demonstration is admitted on the strength of what the person showed. Marco replays
nothing to learn it.**

Three statuses where there was one good one, and the middle is the new claim:

```
EdgeObserved    the human demonstrated it and the evidence was clean
EdgeVerified    MARCO performed it and positively verified where it ended up
RouteObserved   every edge known, at least one only by having watched
```

**Observed is deliberately not folded into Verified.** They are claims about different actors. A
route Marco has never walked is a different fact from one it has, and it is exactly the fact a
person would want before trusting it. Writing "verified" for something nobody ever ran would put a
lie in the durable record, and every surface reading it would repeat the lie.

**The admission rule is not new machinery.** It is the two predicates above, already defined,
already measured, read from the same assessment the lowering path reads. There is no second
confidence model, no threshold, and deliberately no `demonstrations >= 2`: sufficient evidence is
sufficient, and one clean demonstration can be sufficient. That is not the same claim as *one
demonstration is always enough* — an ambiguous or insufficient one still asks.

**Rehearsal survives as a tool.** `Coordinator.WithRehearsal(true)` gives the old behaviour for a
doubtful edge, a capability check, or an Advanced surface, and what comes back is genuinely
stronger — `EdgeVerified` is a claim about Marco. What it stopped being is the definition of
*learned*.

### Planning had to widen, or Fast Learn would have produced nothing runnable

`verifiedEdges` — the predicate handed to `PlanToGoal` — accepted execution-proven edges only.
That was correct while an unrehearsed edge could only exist because something had gone wrong.
After this change a route can be perfectly well known and never have been walked, and a planner
that refused those would answer "I learned that" with "I don't know how".

It is now `plannableEdges`, and it accepts either kind of knowing.

**This weakens nothing, and the reason is that planning was never the safety boundary.**
[[ADR-056-a-goal-is-a-destination-not-a-route]] says so at its own definition: a plan *"says a route
is KNOWN, never that performing it is authorised"*. All three real boundaries are downstream and
untouched:

| | |
|---|---|
| **authority** | minted per invocation at the ordinary door ([[ADR-029-resolution-is-not-permission]]). A demonstration grants none |
| **foreground** | the window must lead before any input is emitted |
| **verification** | every edge is positively verified as it is walked, and arrival confirmed by a fresh look |

So the change is exactly: **Marco is willing to try what it watched you do, when you ask it to.** It
is not willing to claim it worked until it has checked.

## Considered and rejected

- **Relax `EdgeVerified` to cover observation.** One status, no lie-free way to say which happened.
  Every reader downstream — the planner, the Play record, Activity, a future Observe — would have
  lost the distinction, and the distinction is the point.
- **Require two demonstrations instead of a rehearsal.** Trades one piece of ceremony for another,
  and it is not what the evidence says: a second example closes the things `Blocking()` names and
  nothing else. Where the first was clean it adds a repetition, not a fact.
- **Keep the rehearsal and hide the question** — perform the edges silently during Learn. Marco
  would drive somebody's desktop without asking, to obtain proof it did not need.
- **Let the demonstration grant standing authority to execute.** The tempting shortcut, and the one
  this ADR exists to refuse. Watching is evidence; permission is a separate act by a person, and
  they give it when they invoke the Play.
- **Leave planning as it was and mark Fast-Learned edges `directly_verified`.** It would have made
  the acceptance pass and defeated the roadmap in the same line.

## Consequences, including the costs

- **A Play can now be saved that Marco has never executed.** Its first real run is the first proof,
  and that run can fail — honestly, at the edge that does not verify, with everything downstream
  unchanged. The old lifecycle bought certainty by spending the person's time up front; this
  spends it only if something is actually wrong.
- **The admission guard is defensive and could not be reached by any lifecycle test.** Deleting it
  entirely left the whole suite green, because bad evidence is refused before it gets there. It is
  kept as the statement of what admission means, and it is held DIRECTLY rather than by the
  lifecycle — see the note on the test, which says exactly that.
- **Two edge statuses now mean "good", and every fold has to decide about both.** `Session.Status`
  and `LearnedEdges` do; anything added later must, and a reader who forgets will silently under-
  count a Fast-Learned route.
- **`plannableEdges` treats the two kinds alike**, which is right for "can I plan a path" and would
  be wrong for almost any other question. It is the one place they are equivalent, and it says so.
- **Nothing yet upgrades an observed edge to execution-proven when a later run succeeds.** The
  evidence model supports it and the seam is `RehearsalEvidence`; doing the write belongs with
  Roadmap 35C's execution work rather than here.

## Enforced by

- `internal/director/learn` — `TestACleanDemonstrationIsLearnedWithoutBeingRehearsed` (nothing
  rehearsed, both edges observed, a Play exists); `TestAnObservedRouteIsLearnedWithoutBeingPerformed`
  (2/2 learned, 0 performed, and the route does not report itself verified);
  `TestAskingForRehearsalStillRehearses` (the tool survives, and yields `EdgeVerified`);
  `TestAdmissionNeedsCleanEvidence` and `TestABlockingReasonIsNotAdmissible` (the rule itself,
  tested where it is decided, because the lifecycle cannot reach it with bad evidence).
- `cmd/director` — `TestAnObservedEdgeCanBePlannedOver`: an observed edge enters a plan, and the
  read spends no authority.

## Related

[[ADR-051-one-demonstration-and-an-attempt]] · [[ADR-056-a-goal-is-a-destination-not-a-route]] ·
[[ADR-029-resolution-is-not-permission]] ·
[[ADR-070-one-production-body-and-the-caller-brings-the-verification]] · [[Learned-Plays]]
