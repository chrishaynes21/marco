---
type: decision
status: accepted
date: 2026-08-20
supersedes: []
affects:
  - execution
  - perception
  - verification
source_paths:
  - internal/director/observe/reach.go
  - internal/director/observe/place.go
  - internal/director/rehearse/evidence.go
  - internal/director/rehearse/live.go
  - internal/director/rehearse/evidence_test.go
  - internal/director/rehearse/proof_test.go
  - internal/director/service/protocol.go
  - cmd/director/perform.go
  - cmd/director/performevidence_test.go
---

# ADR-090 — a verified outcome is the next step's evidence

## Marco proved the same unchanged fact four times

A two-edge play crosses three screens. Carrying it out established where Marco was standing
**four** times:

| | |
|---|---|
| plan | a fresh look, to decide which edges to walk |
| edge one | establish the source before acting — **the screen the plan had just resolved** |
| edge two | establish the source before acting — **the screen edge one had just verified** |
| arrival | look again, to confirm the goal was reached — **the screen edge two had just verified** |

Each establishment is `establishSamples` readings of the accessibility tree with a settle gap
between them. Three of the four asked a question that had been positively answered moments
earlier, twice by the very action that would have changed the answer.

They were not *wrong*. Nothing had changed, and re-reading an unchanged screen returns the same
result. They were **redundant** — and "nothing had changed" is a claim that has to be justified
rather than assumed, which is exactly why it had never been removed.

## The decision

**Marco may avoid proving the same unchanged fact twice. Marco may not act on a fact it can no
longer justify.**

Both halves, together, are the whole of it. The second is the interesting one: a bug in the first
costs a redundant look, and a bug in the second sends real input into a window nobody checked.

### One type, and it carries its own justification

[`rehearse.StageEvidence`](../../internal/director/rehearse/evidence.go) is one positively
established fact about where Marco is: the Place, **the window reference it was established on**,
when, and how it came to be known.

`Justifies(now, application, wantSubject, inFront)` decides whether it may still be relied on, and
every arm is a reason to REFUSE:

- no window reference — there would be nothing to check against the foreground
- a different Place, or no Place asked about
- a different application
- no timestamp, a stale one, or one from the future
- no way to ask about the foreground at all
- a window that no longer leads

`inFront` is the caller's own foreground predicate, the same one the walker's gate uses. It asks
about **that reference**, not about the application name — which is what refuses a proof taken on
Settings once XBOX has come forward, both being `applicationframehost`, the ambiguity
[[ADR-078-a-learned-play-is-performed-by-the-director]] fixed on the activation side.

`EvidenceSource` records how a fact was learned — `planning`, `established`, `verified_outcome`.
Provenance is not authority: none of them permits anything.

It is **provider-neutral by construction**. A window and a Place are semantic facts; that
Accessibility saw them today is provenance, not identity. An OCR or fused observer producing the
same two facts produces the same evidence, and nothing in this type names a provider.

### Carried evidence shortens the look. It does not replace it.

This is the part that took two attempts, and the first one was wrong.

`Justifies` checks everything a *stored* fact can be checked against. What it cannot check is the
one thing that actually happens: **somebody clicked**. A person moving Settings from one screen to
another changes no window, no process, no generation and no foreground, and every arm still says
yes about a Place that is no longer up. Establishing from scratch would have caught it — so
trusting a carried proof outright is strictly LESS SAFE than the code it replaces, and it fails in
the worst available way: input planned for one screen, emitted into another.

So a proof buys a **shorter question**, never the absence of one. `confirmCarried` takes ONE
reading and requires it to resolve to the same Place. The screen was settled when the proof was
taken, so a single reading can contradict it; and any disagreement — a different Place, an
unreadable frame, a window that went away — falls through to the full `establish`, which is
authoritative. It must never refuse on the strength of one frame: a single reading can catch a
transition that six readings and a settle would look past, and turning that into a refusal would
make a correct walk fail intermittently.

`establishSamples` is 6; `confirmSamples` is 1. The difference is not a confidence dial. One asks
"where am I" of a screen it knows nothing about and has to wait for it to settle; the other asks
"am I still where I just proved I was".

### Where proofs come from, and where they go

```
freshPlace  ─ planningProof ─→  edge 1  ─ Arrived ─→  edge 2  ─ Arrived ─→  confirmArrival
```

- **`planningProof`** binds the Place the planning look resolved to a window reference acquired
  through `observationTarget` — the same object the walk will use, because two targets would be
  two opinions about window identity. The look itself produces no reference: a session identifies
  its window by *selector*, and a selector cannot be asked whether it is in front.
- **`provedBy`** is the only thing that mints a `verified_outcome`. Only on a completed route, and
  only from what perception RESOLVED.
- **`performPlan`** threads each edge's proof into the next.
- **`confirmArrival`** asks the last edge's proof whether it already answers the arrival question,
  and falls through to a fresh look when it cannot.

`Live.Perform` takes carried evidence as an optional argument. **There is one walker**; a caller
that passes nil gets exactly the behaviour that existed before this ADR.

### A look ends with the question it was asked

`freshPlace` starts an observation session to answer "which screen is in front". The session is
bounded by `freshLookWatch` — eight seconds — and the question is ordinarily answered in one or
two. Nothing ended it, so the remaining six were spent **sampling the screen** at
`freshLookInterval`, concurrently with the walk the look existed to begin, contending for the one
accessibility provider with every reading the route took.

It was never a leak. It was a session doing work nobody wanted, at exactly the moment this
roadmap is trying to make the walk cheap. `endLook` retires it — and only a session THIS look
started, because `lookNow` returns an empty id when it reused one that was already running, and
that one belongs to whoever started it.

**And retiring it means WAITING for it, which the first version did not do.** `Cancel` sets a
context and returns; the runner notices at the end of whatever sample it is taking, hundreds of
milliseconds later, and only then does `ActiveID` go empty. Nothing waited — and the consequence
is not a slow retirement, it is the NEXT look failing outright: `lookNow` returns early when a
session is running, on the reasonable theory that its evidence is live, so the caller polls a
retiring session until `freshLookTimeout` expires and reports `place_unknown`. Measured live: a
whole performance refused that way in 6.7 seconds without ever reading the screen.

`disambiguateWindow` has carried the same hazard since it was written. It calls `releaseLook`
precisely so each candidate window is judged on its own evidence, and its comment says so — which
is only true if the release has finished. The wait lives below both entrances so neither can
forget it, it is bounded, and it obeys the context: somebody who pressed stop is not made to wait
for tidying up.

**A look that runs out now says WHICH look ran out.** It used to return no reason at all, so
`place_unknown` reached the Audience as one sentence covering two unrelated problems — a screen
Marco genuinely does not recognise, which the person can fix by opening the right one, and a look
that never started, which is a fault. The reason existed in `freshPlace` the whole time and was
thrown away at the return.

### An observed edge earns execution evidence by being performed

[[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]] split what an edge can claim
and left this seam open. It is closed now, and the shape matters: `rememberRehearsal` **adds** a
`RehearsalEvidence` record when a walk completes and verifies. The demonstration is untouched, and
there is still one relationship.

The history can therefore say both things, which is the point: *the human demonstrated it, and
later Marco performed it and checked.* A failed run adds nothing and erases nothing — Marco
failing is a fact about Marco, not about what the person showed it, and the demonstration is often
the only record of how the route goes.

### The counters are part of the change, not commentary on it

`rehearse.Cost` — readings, Place resolutions, establishments, shortened confirmations, proofs
reused, time spent looking — is read off the walker by `Live.Spent()` and surfaces as
`service.PerformCost`.

**Off the walker, not out of the result, and the live run is why.** A refusal produces no
`RehearsalResult` — deliberately, and load-bearing: "Marco declined to try" and "Marco tried and
it went wrong" are different facts. So while the cost travelled on the result, a REFUSED edge
reported nothing. And the refusal path is where a walk looks most: a shortened confirmation that
disagreed, then a full establishment that could not place the screen — seven readings, reported as
none. Every route total was missing its most expensive edges, in the direction that flatters the
optimization. Found by a real run interrupted mid-way, not by the suite. The caller now snapshots
the tally either side of the walk, which covers both paths with one reading. It is
developer-facing and reaches no sentence anybody reads.

Counts and durations answer different questions and both are needed. A duration is what a person
feels and what a test cannot assert; a COUNT is deterministic and very nearly proportional to it,
because reading the screen is what a walk spends its time on. So the suite gates the counts and a
live run reports the durations.

**Protocol version 10.** Additive, optional, on a response — and bumped anyway, for the reason
version 7 was: a version-9 Director answering a version-10 client omits the object, which decodes
to zeros, and zeros in these fields read as "this route never looked at the screen" — the most
flattering possible reading of the optimization they exist to test. A silent zero is a guess
wearing a measurement's clothes.

## What this does not touch, deliberately

Speed was not traded for any of these:

- **Foreground safety.** The gate still runs before the claim, and again before every step.
  Carried evidence must itself pass the foreground predicate to be usable at all. A walk with a
  reused proof that finds its window behind refuses **without spending the permission** — the gate
  sits before `BeginAttempt` on purpose, and `RehearsalGrant.BeginAttempt` sets `GrantConsumed`,
  which `Attempt.Cancel` does not undo.
- **Authority.** A grant is minted for every edge whatever Marco already knows about where it is.
  Knowing where you are and being allowed to act are different questions with different owners.
- **Cancellation.** The context reaches the shortened confirmation, the establishment, the settle
  and every step. Nothing here makes a context of its own.
- **Per-edge verification.** Every edge is still positively verified as it is walked. The handoff
  consumes that verification; it does not replace it.
- **Final positive verification.** `confirmArrival` still refuses `did_not_arrive` when it can
  neither justify a proof nor answer by looking. `"" == ""` is still not arrival.
- **One walker, one resolver, one performance path.** `Live.Perform` is still the only walker;
  `observe.PlaceNow` is still the only current-place answer, reached in this package through
  `Live.placeNow` so nothing resolves a Place without being counted.
- **Evidence-based settle.** Untouched. No sleep was substituted for stability, anywhere.

## Considered and rejected

- **A time-to-live and nothing else.** The cheapest version, and wrong: a young fact about a
  window that is no longer in front is worthless. `MaxEvidenceAge` is a backstop measured against
  the millisecond gap between one verified outcome and the next edge, never the rule.
- **Trusting a justified proof without looking.** The first implementation. It is measurably less
  safe than what it replaces — see the click race above — and
  `TestCarriedEvidenceLosesToWhatIsOnScreen` exists because of it.
- **A cache keyed on the Place id.** A second answer to "where is Marco", with its own lifetime
  and its own way of being wrong. Evidence that travels with the walk has no lifetime of its own.
- **A flag to disable the handoff, so a harness could measure before and after.** A second code
  path through the one thing this repository is most careful about, to produce a number. The
  deterministic gates calibrate against the same production code walking the same edges with
  nothing carried, which is the same comparison without the second path.
- **Trusting the plan's expectation as the proof.** See the unreachability note below.
- **`StageEvidence.SameWindow`.** Written, used, and removed. Every use was measurably equivalent
  to nothing, because no caller acts on the proof's window: `confirmCarried` returns the reference
  it just acquired, the foreground gate asks about that one, and the step loop re-acquires before
  every step. Window identity is held where it bites — in `Justifies`, and in `sameWindow`.

## Consequences, including the costs

- **A route reads the screen half as many times, and the fixture predicted the live run exactly.**

  | | deterministic fixture | live run 1 | live run 2 |
  |---|---|---|---|
  | screen readings | 10 | **10** | **10** |
  | full establishments | 0 | **0** | **0** |
  | shortened confirmations / reused | 2 / 2 | **2 / 2** | **2 / 2** |
  | Place resolutions | 4 | **4** | **4** |
  | wall clock | — | 4533 ms | 4657 ms |
  | inside the walk | — | 3018 ms | *void, see below* |
  | inside the two confirmations | — | 399 ms | 422 ms |

  The route was Home → Bluetooth & devices → Mouse, both edges verified, arrival confirmed from
  the last edge's proof. **Four counts, identical across a scripted fixture and two separate runs
  against a real accessibility provider** — with the whole perception seam below landing between
  them. That is the strongest evidence available that the fixture describes the thing.

  Run 2's "inside the walk" read 0 ms and is discarded: that is the third instrument bug below,
  found by this very run and fixed after it. The wall clock is the harness's own stopwatch and was
  never affected.

  **Derived, and marked derived wherever it is printed:** one screen reading cost ~200-210 ms on
  that machine (399 ms across two single-reading confirmations). Each reused proof replaced six
  readings and five settle gaps with one reading, so the two of them avoided ~3.2 s — a walk of
  ~6.2 s reduced to ~3.0 s. That is arithmetic on measured quantities, not a second measurement:
  the same route was NOT run against the old code, because a switch to turn the handoff off would
  be a second path through the part of this system that must have only one.
- **A fourth thing can now be wrong about where Marco is standing**, and it is the only one not
  read fresh. That is why `Justifies` fails closed on six arms, why the proof is confirmed against
  a live reading before it is used, and why the worst outcome of a bug in either is that Marco
  does the work twice.
- **Two guards are unreachable from any lifecycle test**, and both are held directly:
  - `provedBy` reading `rec.Observed` rather than `rec.Expect`. A route completes only when its
    last step came out `DirectlyVerified`, which is DEFINED as those two agreeing — so no walk can
    produce a completed route whose fields differ, and the swap survives the whole suite.
    Measured. `TestOnlyACompletedWalkProvesAnything` holds it on the function, with a record no
    walker can produce. The day a route may complete on something weaker, `Expect` becomes a claim
    about a screen nothing ever resolved.
  - `Justifies` refusing an empty window reference. It looks subsumed by the foreground check and
    is not: the PRODUCTION predicate answers `true` for a window it cannot look up, because a dead
    handle is a different guard's business.
- **One guard was written and then removed for being untestable.** `performPlan` screened the
  handoff on `step.Verified`, above the check that already ends the walk on an unverified step.
  The mutation removing it survived. The assignment now sits past the failure check, where the
  control flow is the guard.
- **An instrument can be bypassed silently.** Calling `observe.PlaceNow` directly instead of
  `Live.placeNow` compiles, works, and under-reports — in the flattering direction.
  `TestEveryLookThatConcludedAPlaceIsCounted` checks the relation rather than a number.
- **The instrument was wrong three times, every one in the flattering direction, and every one
  found by running it rather than by testing it.**
  1. A refused edge reported no cost at all, because a refusal produces no result to carry one —
     and the refusal path is where a walk looks most.
  2. The harness printed a blank column for zero establishments: `omitempty` drops a zero int and
     the format string rendered `$null` as nothing. A hard zero, the single number most worth
     reading, displayed as absence — the same hazard this ADR bumped the protocol version over,
     reproduced in its own reporting.
  3. Every edge reported taking 0 ms, because the duration rode on the walker's tally and
     `Cost.Since` subtracts counts. A live route that took three and a half seconds said it had
     spent no time inside the walk.

  All three fixed and gated. The pattern is the lesson: an instrument's failures are not
  symmetrical, and a flattering number is the one nobody questions. The third was made
  structurally unavailable rather than merely corrected — a tally is counted and a duration is
  timed, so the stopwatch moved to the caller and there is no duration on the tally to forget.
- **The `no_authority` refusal in `performEdge` is a WORD, not a guard.** Making it conditional on
  carried evidence survives the suite, because the walker refuses a nil grant anyway. It is kept
  for the specific refusal it names, and the safety lives one layer down where
  `TestAProofIsNotPermissionToAct` holds it.

## An unknown Place is not an unreadable window

Found by running this, not by testing it, and fixed narrowly before returning to the roadmap.

A live acceptance refused three times with *"Marco does not recognise the screen it is looking
at"*. The window was right, in front, full screen. What the provider returned was **sixteen
structures where the same page had been learned with a hundred and forty-eight** — caption
buttons, a title strip, an account tile, and one rectangle covering three quarters of the frame
with nothing observed inside it. Settings is a hosted application; when its content is suspended
or unpainted the tree collapses to the frame it lives in.

Two states were wearing one answer:

| | means | the fix is |
|---|---|---|
| unknown Place | the page was read and matched nothing remembered | open the right screen |
| unreadable window | the window was seen and its page was not read | the reading is broken |

The first sentence sends a person to change the page. Doing that produced the identical refusal,
because the page was never the problem.

**Decided at `observe.PlaceNow`** — the one current-place answer, so `learn`, `pointing`,
`playbill`, `reach` and the walker all inherit it rather than each collapsing it independently.
`Place` gains `Reach` beside `Placed`: a shell reading IS describable, it simply describes the
window rather than the page in it, and memory is not asked to identify a screen from evidence
containing no screen. That recall would miss, honestly, and the miss is the lie.

**The rule is arrangement, never richness.** `148 → 16` is the evidence that exposed the defect,
and a rule built on it would be wrong — a collapsed navigation, a resized window, a personalised
page all legitimately change how rich a screen is, and `148 → 130` must keep meaning what it means
today. What `ReachOfState` asks instead: is there a space the window gives its content, covering a
serious share of it, with almost nothing observed inside — and nowhere else in the window
populated either? Provider-neutral by construction: nothing in it knows what UIA is, and the
refusal names no provider either, because the first observer that is not Accessibility would
inherit the lie.

The false positives are gated as carefully as the true one: a sparse dialog, a chrome-heavy
editor, **a mail client with a blank reading pane beside a full message list**, and a floating
palette with an empty swatch. The empty-panel case is why one populated panel anywhere is enough
to say the window was read — emptiness alone would have called it degraded, and it was read
perfectly well.

**Nothing became more permissive.** `source_unreadable` refuses as hard as `source_unrecognised`;
no input is emitted; no durable Place is created from a shell reading. The change is what Marco
says about why it will not act.

**And it is the seam a future Fusion needs.** "Accessibility saw an unknown page" and
"Accessibility barely saw the page" are now different facts, which is exactly the question another
provider would be escalated on. Fusion is NOT implemented here, and no policy says a degraded
reading should run OCR — that belongs to provider orchestration.

## KNOWN FOLLOW-ONS

1. **Nothing yet reports the counters on a normal surface.** They reach `director perform --json`
   and the acceptance harness. An Advanced view would be the natural home.
2. **The live measurement is taken once, on one machine, for one route.** One run is not a
   distribution: it says the architecture behaves as the fixture describes, not that 3.0 s is what
   this costs everywhere. `acceptance-35c.ps1` reproduces it in two commands against a copy of the
   operator's own learned route.
3. **Two Directors can serve one `$MARCO_HOME` and nothing notices.** The harness was starting a
   second on every `-Setup` and three were running before anybody looked; that is fixed there,
   but the product side is untouched -- `watchingElsewhere` only knows about sessions in its own
   process, so two Directors on one desktop have no guard against driving it at the same time.
   Pre-existing, out of 35C's scope, and worth an ADR of its own.
4. **One page, two Places, and a resize breaks a route.** Found while diagnosing a live
   `place_unknown`: the sandbox store holds TWO start Places for what is almost certainly the same
   Settings home page -- `button=18, list_item=22, image=14` in both, but `group=20/27`,
   `text=32/49`, `link=1/3`, and one of them carries a `scroll_bar`. That is one page at two window
   sizes, learned twice as two screens. A route learned at one size then refuses honestly at the
   other, and the refusal names neither the shape it wanted nor the shape it saw.

   Nothing in 35C caused it and nothing in 35C should fix it: it is the structural-identity model
   meeting a page whose content is dynamic. But it makes any live measurement fragile, and it is
   the most likely reason a person would say a learned Play "stopped working". Worth its own
   investigation.
5. **`place_unknown` cannot tell "I looked and did not recognise it" from "I could barely read
   that window at all", and they need opposite things done about them.** Measured live while
   chasing a run that failed repeatedly: Marco read SIXTEEN controls from a Settings window whose
   learned shape is one hundred and forty-eight -- caption buttons, a title, an account tile, and
   one 1594x926 box where the whole page should be. Settings is a UWP app hosted inside
   ApplicationFrameHost, and when the hosted content is suspended or unpainted the accessibility
   tree collapses to the frame.

   Marco said "I don't recognise this screen", which is true and sent the diagnosis to the page
   rather than to the reading. The evidence to say better was right there -- a control count an
   order of magnitude short, and a single box covering most of the frame. The reason exists one
   layer down and nothing carries it, which is the same shape as every reporting gap this campaign
   has found.
6. **Other `.ps1` files in this repository carry the same encoding hazard** the 35C harness hit: a
   BOM-less UTF-8 file with an em dash inside a double-quoted string breaks Windows PowerShell
   5.1's parser, pointing at a brace fifty lines away. They currently parse only because their
   em-dash counts happen to be even. `acceptance-35c.ps1` has a BOM; the rest do not.

## Enforced by

- **The perception seam** — `internal/director/observe`:
  `TestASparseWindowIsNotADegradedOne` (the live failure, and five readings that must not be
  confused with it); `TestAShellOnlyReadingIsNotAnUnknownPlace` (memory is not asked, and a page
  that IS unknown still is); `TestAShellReadingKeepsWhatMadeItOne`. `internal/director/rehearse`:
  `TestAnUnreadableWindowRefusesForItsOwnReason`; `TestAnUnreadableRefusalNamesNoProvider`.
  `cmd/director`: `TestAnUnreadableWindowIsNotAnUnknownPlace`;
  `TestALookSaysWhetherItCouldReadTheWindow`; `TestALookThatRanOutSaysWhichLookItWas`.

- `internal/director/rehearse` — `TestCarriedEvidenceIsRefusedWhenItCannotBeJustified` (every arm,
  premise asserted first); `TestCarriedEvidenceLosesToWhatIsOnScreen` (the click race);
  `TestAProofIsNotConfirmedByAScreenNobodyCanRead`; `TestADisagreeingFrameSendsMarcoToTheFullLook`
  (a disagreement asks properly, it never refuses); `TestCarriedEvidenceSparesTheWalkerItsOpeningLook`;
  `TestAWalkerHandedUnjustifiableEvidenceEstablishesForItself`;
  `TestAVerifiedWalkReturnsTheStageItProved`; `TestOnlyACompletedWalkProvesAnything`;
  `TestAProofDoesNotCrossWindowsOfOneProcess`; `TestAProofIsNotPermissionToAct`;
  `TestStoppingReachesAWalkCheckingItsProof`;
  `TestAReusedProofDoesNotSpendAPermissionOnAWindowBehind`.
- `cmd/director` — `TestOneRouteProvesEachScreenOnce` (the deterministic acceptance, calibrated
  against the same production code with nothing carried); `TestAnInvalidatedRouteFallsBackToLooking`;
  `TestAVerifiedOutcomeBecomesTheNextEdgesSource`; `TestAWalkThatProvedNothingCarriesNothing`;
  `TestThePlanningLookIsEdgeOnesSource`; `TestPerformGoalHandsThePlanningLookToTheWalk`;
  `TestArrivalReusesTheProofTheLastEdgeProduced`; `TestArrivalIsConfirmedByLookingNotByFinishing`;
  `TestARefusedEdgeReportsWhatItSpent`; `TestAWalkedEdgeReportsHowLongItTook`;
  `TestALookThatRanOutSaysWhichLookItWas`;
  `TestEveryLookThatConcludedAPlaceIsCounted`; `TestALookEndsWhenItHasItsAnswer`;
  `TestOnlyTheSessionALookStartedIsItsToEnd`; `TestPerformingAnObservedEdgeAddsExecutionEvidence`;
  `TestAFailedPerformancePreservesWhatWasDemonstrated`.

## Related

[[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]] ·
[[ADR-078-a-learned-play-is-performed-by-the-director]] ·
[[ADR-070-one-production-body-and-the-caller-brings-the-verification]] ·
[[ADR-029-resolution-is-not-permission]] ·
[[ADR-055-an-authorised-rehearsal-waits-for-its-start]] ·
[[ADR-085-a-performance-is-a-registry-command]]
