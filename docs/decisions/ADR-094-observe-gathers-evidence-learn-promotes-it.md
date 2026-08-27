---
type: decision
status: accepted
date: 2026-08-26
supersedes: []
affects:
  - passive-observation
  - demonstrations
  - semantic-memory
  - learned-plays
source_paths:
  - internal/director/ambient/action.go
  - internal/director/ambient/select.go
  - cmd/director/observeaction.go
  - cmd/director/observelook.go
  - cmd/director/observepromote.go
  - cmd/director/observerecent.go
  - internal/director/observe/assessment.go
  - acceptance-36b.ps1
---

# ADR-094 — Observe gathers evidence, Learn promotes it

## The sentence this makes true

```
marco observe
   ... somebody uses their computer ...
marco learn "open mouse settings" --recent
done
```

No repeat demonstration. No naming a screen. No naming a button. No rehearsal. No input from
Marco at any point during the learn.

## The old shape and the new one

|  |  |
|---|---|
| **was** | Learn begins observation. Somebody enters a mode, demonstrates, answers questions, and Marco offers to try it. |
| **is** | Observe may already have the evidence. Learn selects a part of it and promotes that part. |

Both still exist and they are not in competition. Prospective Learn is unchanged and is what runs
when nothing was watching; retrospective Learn is what a person means when they say *learn what I
just did*, and it is only possible because something was.

## A demonstration is semantic evidence, not a recording

```
Place A  ->  the person activated the control called X  ->  Place B
```

That is the whole model. What makes it a play is that every part of it survives the window moving,
the screen changing resolution and the page reflowing — because none of it is a coordinate.

[[ADR-093-observe-is-attention-not-recording]]'s buffer held the two ends of that sentence and not
the middle, which is why 36A could say honestly that *"learn what I just did" is not implemented*:
a route reconstructed from it would have known where somebody went and not what they pressed.
`ambient.Step` now holds the middle.

### Coordinates stay transient, and there is nowhere for one to become durable

A pointer press is placed inside the watched window, normalised, and resolved AT EVENT TIME against
the controls that window offered — machinery that predates this roadmap. What crosses into the
trail is `ambient.Target{Role, Label}`, which has no position field, so `button = (742, 318)` is
not a thing this system can write down. The evidence that produced the resolution lives and dies
inside the perception layer.

**The Fusion seam is that resolution, and it already has the right shape.** `navsource.SetActionables`
receives a list of boxes with roles and admitted labels from the fused world, once per valid
inference, and a press is matched against it. A future provider that supplies better candidates —
a region, a visual match, an OCR reading — supplies them there, and nothing downstream changes.

## One word of somebody's screen crosses, and it is the narrowest one available

36A's buffer held ids, counts, times and a two-word provenance vocabulary. That boundary moves,
deliberately and by exactly one thing: **the name of the ONE control a person's own action landed
on**, and **what a screen appears to be called** when a name settled by recurrence.

Neither is a widening of what Marco PERCEIVES. Both already crossed into a passive session's own
evidence under the zero licence:

- a control's label passes `observe.AdmittedTargetLabel`'s canonical role allowlist —
  `NameablePlaintext`: button, menu item, menu, tab, checkbox, radio — which stands whatever
  anybody declared. The WIDER stage, which any clickable role passes, is opened only by
  `Episode.NameActivatedTargets`, and ambient watching does not hold it. So a list item, a link,
  an icon and a text field carry no name into the trail, which is where a document title, a
  contact and a chat line would otherwise arrive.
- a place's apparent name passes `observe.AdmittedPlaceName`, which is unconditional and applies
  the same shape filter.

What changes is RETENTION, and it is bounded exactly as everything else here is: one label per
action, in a trail of at most 64 steps, forgotten when watching stops.

**A screen's transient structural shape is held too**, so a walk through software Marco has never
seen can still be learned — the first time anybody uses a program is when they most want to show
Marco something. It is `observe.StructureSignature` carried WHOLE and unaltered, because that value
is the identity: narrowing it here and rebuilding it at promotion time would establish a
near-duplicate of the screen it described.

## The promotion boundary is an object, not a comment

Ambient watching holds `observesession.Episode{}` for its entire life. It may not establish a
Place, may not turn what it saw into route evidence, and may not keep a control's name. There is
no path from the observer to a durable write.

An explicit Learn is a person naming a behaviour and asking for it to be remembered — the same
human semantic event that licenses a live Learn session, arriving through a different door. So it
grants the same `observesession.LearnLicence()`, and every durable write goes through a method on
`promotion` that refuses without the specific permission it needs. A promotion built with the zero
licence writes nothing, and it is constructible, which is what makes the refusals testable rather
than asserted.

**It does not retroactively license the watching.** The evidence was gathered under no permission
and stayed transient the whole time. What is licensed is this operation, now, on the part of that
evidence the person just pointed at.

## An action belongs to the screen it was performed on

The one correlation that had to be got right, because getting it wrong produces evidence that
looks perfect and is about the wrong screen.

Events are drained about a second after they happen. Between the press and the drain the click has
usually already changed the screen — so what is in front when the event is finally read is very
often the DESTINATION of the action rather than its source. Attributing to it produces *"on the
Bluetooth page, press Bluetooth"*, which is wrong in a way that reads as entirely reasonable.

So every admitted event carries the session-local screen state that was current when the runner
banked it, every ambient reading records what that state resolved to, and the action is filed
against the state on its own stamp. Both halves come from one session and one counter: no clock to
drift, no window to tune. An action stamped with a state nothing recorded is held anyway and never
becomes a step — a gap is better than a guess.

## Noise, modality and order

**One press arrives as a burst** — pointer down, focus moves, control reports itself invoked — and
becomes one action. Same kind, same target, within 400 ms. Two presses on two DIFFERENT controls
are two actions however fast they arrived, because a menu opened and an item chosen inside it is
the commonest real case and collapsing it would lose the item.

**Moving a selection is not doing something.** Arrows produce no action at all; the confirm that
follows carries whatever the keyboard had landed on. Pointer motion, hover and focus-changing-on-
its-own never arrive: the navigation vocabulary has no word for any of them, so *"a hover became an
activation"* is a sentence this code cannot say.

**A click and a keyboard confirm on the same control produce the same act**, deliberately. A
demonstration teaches intent; the physical means is the demonstrator's habit, and a route that
recorded which would replay one person's hands rather than either person's intention.

## Loading is not a place, and a long load costs nothing

A reading Marco can neither recognise nor describe well enough to establish is CROSSED rather than
recorded — the same `PlaceToEstablish` gate a licensed session uses, asked as a question. So
`Home -> spinner -> spinner -> Bluetooth` is one step, carrying how many frames it crossed and how
many readings it settled over.

**Nothing here is a timeout.** A pending action stays filed against the screen it was performed on
until that screen is left, whether that takes 200 ms or thirty seconds. The buffer bounds by SIZE
and not by time precisely so slow software stays learnable, and adding an expiry to make it tidy
would make a remote desktop impossible to show Marco.

**The two completion questions stay distinct.** A prospective one-shot Learn still ends when the
PERSON says so — inactivity never infers it, for exactly the reason above. Retrospective Learn is
not waiting for anything: the actions already happened, and what it does is select.

## What "recent" means

A backward walk from the end of the trail, stopping at the first semantic boundary:

| | |
|---|---|
| a different program | plays are scoped to one, and a walk across two is a thing Marco cannot represent |
| a step Marco performed | its own work is not somebody's demonstration |
| a step already promoted | see the watermark below |
| a pause over five minutes | somebody who made a coffee did two things |
| a screen already in the walk | coming back ends one journey and begins another |

The last of those keeps the walk SIMPLE — the longest recent stretch that never goes anywhere
twice. Deterministic, explainable in a sentence, and never a pick from the middle of history.

**Selection reads no clock.** Every boundary is a gap between two things somebody DID, and nothing
is compared to the present: somebody who demonstrated something, was interrupted for twenty
minutes and came back still means the thing they demonstrated. The honest bound on how old this
evidence can be is the one watching already has — it is transient, and stopping forgets it.

**Naming an application means "the last thing I did in that program"**, so switching to your mail
for a minute before typing the command does not answer *I haven't seen you go anywhere*.

## Refusing, and refusing differently

Four outcomes, and the sentences are all different because they send somebody to different places:

| | |
|---|---|
| `nothing_recent` | there is nothing, or nothing was watching |
| `not_yours` | the last thing that happened was Marco's own work |
| `insufficient` | there is evidence and it does not say enough — with which |
| `ambiguous` | you got there two ways just now and Marco cannot tell which you mean |

The shortfalls matter as much: *I could not read what you pressed* and *I do not recognise the
screen you ended on* are different problems, and the commonest real one — a press on a control
whose name the plaintext allowlist withholds — is the privacy boundary working rather than
perception failing.

**There is no confidence score.** Every sufficiency condition is binary and every one of them is
something a future walk genuinely needs: an endpoint it can recognise on arrival, an action
attributed to the person, and a name for whatever that action landed on. Dressing a missing one up
as 0.7 would produce a play that fails in front of somebody.

## Observed, not verified

A person demonstrated this and Marco understood it. That is a different and weaker claim than
Marco having performed it, and the record says which: `ProcedureCandidate.Verified` is false, the
relationship carries the navigation that PRECEDED the change rather than a claim about what caused
it, and no rehearsal is recorded because none happened.

**No mandatory rehearsal.** [[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]]
settled that; retrospective Learn finishes after promotion, and the permission to act is asked
later, of whoever invokes the play, by the ordinary door.

### The defect this landed on

ADR-089 says in as many words that *"a Play can now be saved that Marco has never executed"*. It
changed the Learn coordinator and it changed planning. **It did not change the lowering gate**,
which went on asking `CandidateAssessment.Verified` — a field only a completed live rehearsal can
set. So Fast Learn produced every durable thing except the artifact: places, edges, candidates, the
goal, and then `no route is ready to be written down`.

Nothing caught it. Every save test in `cmd/director` writes a rehearsal record into its fixture
first, and the test that named the gate asserted the pre-089 rule and passed.

The gate now asks `CandidateAssessment.Writable()`. See [[ADR-027-what-marco-learned-becomes-marco]],
amended. What still refuses is a route with a rehearsal on record that has not produced a
verification — something about it stopped adding up, and the OTHER demonstration of it must not
quietly lower instead.

## Nothing here can act

- **No input.** Every actuating entrance funnels through `beginPerformance`; nothing on this path
  enters it, and the counter says so.
- **No desktop lease.** Claimed inside a live rehearsal, downstream of the same slot.
- **No authority.** The ephemeral rehearsal grant is the only authority object in the system and
  nothing here creates one.
- **No second performer, resolver or router.** The play is written by the same lowering, saved by
  the same persistence path, found by the same resolver and run by the same walker under the same
  authority. `ObserveLearn.Recent` is a field on the request Learn already answers.

## Watching is not something Marco did, and neither is learning from it

The action graph stays untouched. Activity is the account of what MARCO has done, and a person
navigating their own computer is not that — the same boundary ADR-093 drew for watching, from the
other end.

## Duplicates, and learning the same thing twice

An endpoint the observer RECOGNISED carries a durable subject id and never reaches the store;
`EstablishPlace` is idempotent by signature for the rest. So walking a route again and learning it
under another name costs zero new places.

**A promotion watermark** stops a second *learn what I just did* walking back over evidence the
first one already turned into knowledge — which would write two plays for one walk with nobody the
wiser. A WATERMARK rather than a deletion: the step after a promoted one still needs its
predecessor to know where it began, and the recent trail must not lie about what just happened.

The watermark moves LAST, after the play is written, so a promotion that failed part-way can be
retried on the same evidence rather than being consumed by the attempt.

## Failure is not a transaction, and does not pretend to be

Every write is its own atomic store operation and there is no rollback. A promotion that
establishes two places and then cannot write a candidate leaves two places established — which is
honest rather than tidy: establishing a Place asserts only that Marco can recognise a screen, and
that is true whatever happened afterwards. What it must never do is report success it did not
have, and the report carries what actually got written.

## Watching continues

The observer is not stopped, not restarted, and loses nothing unrelated. The licences do not
survive either: they belonged to the promotion, which has finished. The sentence a person reads
says so, because somebody who has just been told something was learned has no way to know whether
the mode they turned on is still on.

## What the mutation gate found, because it is the most useful part of this record

Fifty attacks, and six survived the first pass. Every one was a real gap and none of them was in
the behaviour — they were all in what the tests actually entered:

1. **The drain in `sample` was production code nothing invoked.** Every test called
   `attribute(drain(...))` itself, so deleting the call from the production path left the suite
   green. The third time this repository has found a complete, working, uncalled mechanism.
   Fixed with a seam and a test that enters through `sample`.
2. **The licence test passed with the licence check deleted.** It asked only whether an error came
   back, and one did — from the store, about the fixture's signature. An unlicensed operation now
   has to be refused *for being unlicensed*.
3. **`ambientLook` had no test at all.** Both fabricating the current Place and dropping the
   screen's name on the way into the shape survived. It is the only thing in this roadmap that
   reads live perception, and everything else supplied the trail directly.
4. **Nothing held the central safety claim.** Handing every ambient session the full Learn licence
   survived the whole suite — the sentence *"however long it runs it cannot make anything
   durable"* was true and enforced by nothing. Then the first fix for it drove the session RUNNER
   rather than the passive door `Start`, and the mutation survived again. The test now runs the
   same fixture twice through the two real entrances and requires the licensed one to establish
   something, because a test that cannot see the difference proves nothing.
5. **A write-only map that looked load-bearing.** The session-local state map was populated,
   bounded and evicted, and nothing ever read it; the correlation runs on the stamp and the
   pending table. Deleted.
6. **An equivalent guard.** The local "is this screen still loading" check restated a rule
   `PlacesToEstablish` already enforces. Removed rather than kept as a second statement of a rule
   with one enforcement.

And one vocabulary word was written and then deleted before it shipped: a `crossed_applications`
shortfall for a case selection never reaches, because a walk that leaves a program ends rather
than fails.

## KNOWN FOLLOW-ONS

1. **Ambient promotion is still not implemented, and this roadmap deliberately did not build it.**
   Repeated evidence does not become durable knowledge on its own. The seam is unchanged and is
   now much better fed: `ambient.Buffer` carries counts, first and last times, provenance, the
   actions and the targets. What is missing is a POLICY — how often, how confidently, and with
   what consent — and the explicit path had to prove the evidence model first.
2. **Text entry is structural only, and there is no plan here to change that.** Nothing observes
   what was typed. A demonstration that crosses a screen offering somewhere to type is refused by
   the existing `RequiresTextEntry` boundary rather than reconstructed. A play that genuinely needs
   a typed value needs the established secret mechanism, and that is its own consent conversation.
3. **A drag is not representable and is refused rather than flattened.** The action vocabulary has
   `activate`, `back` and `menu`; a drag would lower to an activation, which would be a lie about
   what happened. Scroll is absent for a different reason and a good one: a target resolved
   semantically does not need the scrolling that brought it into view, which is the whole advantage
   over macro recording.
4. **A failed rehearsal leaves no durable record.** `rememberRehearsal` is called only for a
   completed live route, so *"Marco tried this and could not do it"* is not a state the store can
   represent. It became visible while widening the lowering gate — an `Attempted` field was written
   and then removed rather than shipped as an unreachable discriminator. Storing failed attempts is
   a change to what the store keeps and a decision of its own.
5. **A multi-leg walk saves the terminal leg as the play.** Every leg becomes durable route
   evidence and the goal is where the person stopped, so planning walks the rest — the goal-centric
   model working as designed. `saveWalk` writes the whole ordered walk only when every leg is
   `EdgeVerified`, which a Fast Learn's legs are not.
6. **Cross-application demonstrations are not supported.** Selection ends at the boundary and takes
   what is inside one program. Making a play span two is a change to what a Play IS.
7. **Live acceptance is UNMEASURED.** `acceptance-36b.ps1` is the harness: `-Setup`, click through
   Settings, `-Learn`, `-Report`, `-Restart`. It drives nothing, and the one step that would drive
   the desktop — running the learned play — is deliberately handed to the person instead.
8. **`RecentGap`, `coalesceWindow` and `maxTrackedStates` are internal constants**, like 36A's
   three. They are not configurable and should become so before anybody runs this for a working
   day.

## Enforced by

- `internal/director/ambient` — `TestTheWalkSomebodyJustTookIsWhatComesBack` (the product target,
  as a fixture); `TestAnUnknownScreenInTheMiddleIsOneTransition`;
  `TestWhatMarcoDidIsNotWhatYouDemonstrated`; `TestAChangeNobodyCausedIsNotADemonstration`;
  `TestAPressMarcoCouldNotNameIsRefusedForTheRightReason`;
  `TestAWalkThroughUndescribableScreensIsRefused`;
  `TestAScreenMarcoCanDescribeIsGoodEnoughToLearn`; `TestTwoWaysToTheSamePlaceIsAQuestion`;
  `TestARoundTripIsNotOneDemonstration`; `TestADemonstrationStopsAtTheEdgeOfItsProgram`;
  `TestACoffeeBreakSeparatesTwoThingsYouDid`; `TestTheSameAfternoonIsNotLearnedTwice`;
  `TestAskingWhatYouJustDidChangesNothing`; `TestTheTrailIsStillBoundedWithActionsOnIt`;
  `TestForgottenStepsDoNotGiveTheirNumbersBack`; `TestAnInventedActionWordIsDropped`;
  `TestATransientShapeCarriesNoWordsAnybodyCouldRead`;
  `TestTheBufferStillHoldsNothingItShouldNot`.
- `cmd/director` (capture) — `TestOneNoisyPressIsOneAction`;
  `TestTwoQuickPressesOnTwoControlsStayTwoActions`;
  `TestTwoDeliberatePressesOnOneControlStayTwo`; `TestMovingASelectionIsNotAnAction`;
  `TestAnActionBelongsToTheScreenItWasPerformedOn`;
  `TestNothingCarriesAcrossASessionBoundary`; `TestADroppedInputLogDoesNotRepeatItself`;
  `TestTheClickSurvivesTwentyCopiesOfTheSameScreen`; `TestALoadingScreenIsNotSomewhereYouWent`;
  `TestThirtySecondsOfLoadingDoesNotLoseTheAction`; `TestWatchingSeesWhatYouPressed`;
  `TestWhatMarcoDidIsNotWhatYouDemonstrated`.
- `cmd/director` (promotion) — `TestLearningWhatYouJustDidAsksNothing` (the whole thing, through
  real stores, with nothing asked); `TestALearnedRecentWalkIsObservedAndNotVerified`;
  `TestObserveCannotMakeItsOwnEvidenceDurable`;
  `TestLearningTheSameWalkTwiceDoesNotMintNewPlaces`;
  `TestLearningRecentEvidenceRemembersTheGoal`; `TestWatchingSurvivesBeingLearnedFrom`;
  `TestTheSameAfternoonIsNotLearnedTwice`;
  `TestAskingToLearnTheRecentPastNeverStartsWatchingInstead`;
  `TestARetrospectiveLearnIsFinishedWhenItAnswers`; `TestEveryRefusalSaysSomethingDifferent`;
  `TestAnUnusableNameIsRefusedBeforeAnythingBecomesDurable`;
  `TestLearningWhatYouJustDidTouchesNothingThatActs`;
  `TestLearningFromWhatYouDidIsNotSomethingMarcoDid`;
  `TestARetrospectivelyLearnedPlayIsAnOrdinaryPlay`;
  `TestARouteMarcoOnlyWatchedCanStillBeWrittenDown`.
- `cmd/director` (the boundary itself) — `TestWatchingCannotMakeAnythingDurable` (the same fixture
  through both real entrances, so a licence that was withheld can be told from a screen nothing
  could describe); `TestOneAmbientLookSaysWhereYouAreThroughTheOneResolver`;
  `TestALookAtAnUnknownScreenCarriesWhatItIsCalled`;
  `TestALookCarriesWhatTheScreenAppearsToBeCalled`; `TestTheShapeCarriesWhatTheScreenIsCalled`.
- `internal/director/ambient` — `TestAWatermarkNeverMovesBackwards`.
- `internal/director/observe` — `TestTheLoweringRefusalMatrix`, whose "a rehearsal of another
  demonstration of the same route" case holds the `Rehearsed` veto.
- `cmd/marco` — `TestLearningTheRecentPastDoesNotStartADirector`.

## Related

[[ADR-093-observe-is-attention-not-recording]] ·
[[ADR-089-watching-is-how-marco-learns-performing-is-how-it-proves]] ·
[[ADR-027-what-marco-learned-becomes-marco]] ·
[[ADR-056-a-goal-is-a-destination-not-a-route]] ·
[[ADR-076-a-place-may-say-what-it-appears-to-be-called]] ·
[[ADR-047-a-place-is-remembered-a-meaning-is-answered]] ·
[[ADR-013-navigation-is-meaning-not-keys]] ·
[[Passive-Observation]]
