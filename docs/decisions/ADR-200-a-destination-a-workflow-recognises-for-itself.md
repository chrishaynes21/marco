---
type: decision
status: accepted
date: 2026-10-15
deciders: director, owner
affects:
  - passive-observation
  - execution
source_paths:
  - internal/director/workflowstore/reached.go
  - internal/director/workflowstore/anchor.go
  - internal/director/workflowstore/arrival.go
  - internal/director/workflowrun/workflowrun.go
  - cmd/director/performworkflow.go
  - cmd/perceptionreplay/reload.go
  - cmd/perceptionreplay/workflow.go
---

# ADR-200 — a destination a workflow recognises for itself

## Context

[[Experiment-046-the-destination-it-reached-and-did-not-recognise]]'s live run pressed PLAY,
arrived at the Play page, recognised it, pressed the Training tile, arrived at the Training page
— and then stood on it for sixty observations without establishing it:

```
  step 0: destination established — screen_b inferred as visual_2, arrangement agreed 0.662
  step 1: refused — screen_c not established within 60 observation(s); the screen
          standing is visual_2
```

[[Experiment-048-the-destination-a-workflow-can-see-for-itself]] found that nothing was wrong
with the rule that would have recognised it. **The rule was never asked**, and when it was asked
it was handed the wrong pixels. Two lines, both of them the global screen model answering a
workflow's question:

- `promptGrounder.Establish` returned as soon as the step's SOURCE was still established. A
  binding is live for exactly one observation generation, and the epoch moves only when the
  global observer records a screen CHANGE. The Play page and the Training page agree at 0.592
  against the 0.55 that merges them, so no change was recorded, so `screen_b` stayed live for
  ever and `screen_c` was never put to the test.
- and `SessionBinding.Infer` was scored against `obs.Arrangement(up.Screen)`, the standing
  screen's STORED signature — fixed when that screen was minted by `promote` and never updated.
  Standing on the Training page under `visual_2`, the saved destination was being compared
  against the Play page's own arrangement.

The owner's reading of that, and the instruction:

> The key finding is that Marco already has sufficient evidence to recognize Training. […] I
> would keep the correction narrow: evaluate the current look against both saved source and
> destination, require fresh admissible OCR, preserve the distinction between workflow-local
> inference and independent verification, and ensure saved workflows actually retain their
> screen vocabulary.

and, on the evidence itself:

> OCR that doesn't contradict a destination is weaker than OCR that positively identifies it.
> The implementation should report those separately, particularly for destinations with fewer
> than three saved words.

## Decision

**A workflow may establish the destination it is going to from the look in front of it, without
the global observer minting an identity for it first. It may not do anything else.**

`SessionBinding.Reached` is a second establishment path beside `Infer`, deliberately a separate
function rather than a loosening of it: an inference FOLLOWS a screen change the observer
recorded and is the stronger claim for that reason. Three terms, all required:

| | what it is | why |
|---|---|---|
| the destination | agrees with THIS LOOK at `VisualScreenMatch` | the existing threshold, unmoved; `observe.ArrangeLook` of the look's own regions, never a stored signature |
| the contrast | and agrees better than the saved SOURCE does | the comparison the global model cannot make — it has one standing claim, where a workflow names both ends of the step. Alone it cuts the Play-page false arrivals from 10 of 34 to 2 |
| the corroboration | and a fresh admissible reading does not contradict it | measured: the arrangement alone establishes the Training page on 10 of 34 looks of the Play page and 21 of 28 looks of the submenu, because a submenu replaces one panel of the page behind it |

The corroboration is **required, not consulted when available**. A destination saved with no
words is refused rather than treated as agreeing: a screen nobody read cannot be contradicted,
so accepting it would make the term vacuous for exactly the artifacts that carry no text.
*Missing evidence is not contradictory evidence* read the other way round — it is not
corroboration either.

One term was measured and **rejected**: requiring the reading to also contradict the SOURCE. It
is perfect on Play → Training (10 of 10) and destroys Main Menu → Play, where it drops arrivals
from 11 readings in 12 to one — because the Play page's whole saved vocabulary is four words
(`match offline play private`) that the main menu also says. **The arrangement and the words fail
in opposite places**, which is the argument for requiring both and for not requiring more.

### Two strengths of the reading, counted apart

`Bound.Positive` says the reading kept at least `TextKept` of what the destination was read to
say — it IDENTIFIED it. `Bound.Thin` says the destination was saved with fewer than
`TextDisagreementWords` words, so no reading could have refused it however wrong it was.
`Arrival` carries both, the run report prints them under `destinations established`, and the
sentence a person reads says which it was. A single count would hide the difference, and the
owner is right that they are not the same evidence: `TextsDisagree` abstains for several honest
reasons, and an abstention counted as corroboration is an abstention counted as a yes.

### What it is not

- **Not independent.** `Ground` is `reached`, and `Independent()` stays true only for
  `confirmed`. The expectation came from the same workflow whose step is being checked.
- **Not an action verified.** `Arrival.Verified()` is still false and still says why.
- **Not a change to the global screen model.** Nothing here mints an identity, writes an
  arrangement or folds a vocabulary, and the separation is structural rather than promised:
  `internal/director/observe` does not import `workflowstore`.
- **Not durable.** A `Bound` lives in one `SessionBinding`, which has no JSON tags and is not a
  field of `Artifact`, and dies with its observation generation.
- **Not evidence that an action was sent.** No part of the rule consults it.

### And a saved workflow keeps its vocabulary

The live artifact had no `text` key on any of its nine screens, and two things had to be true for
that. Both are fixed: the learning fold now folds the readings the recording carries instead of
only under `-semantic`, and every save path goes through `SaveWith`. The default invocation now
produces a byte-identical artifact to the one `-semantic` produced — 13 screens, 11 of them with
words — where before it produced one that could never have a destination corroborated.

Folding text in the learning path also changes what is learned, and this is the honest size of
it: on the same recording the artifact goes from 9 screens and 21 steps to **13 screens and 39
steps**. Pages that were silently merged are now distinguished, so transitions that produced no
step now produce one. The artifact a live run carries should be regenerated.

## Consequences

**Measured through the production rule itself**, probing every look of four verified pages with
an independent session per look (the whole table is in
[[Experiment-048-the-destination-a-workflow-can-see-for-itself]]):

```
  the step screen_b -> screen_c, the one the live run could not make
    ON the Training page   TRUE arrival   10 of 28 look(s)   all 10 identified
    on the Play page        false          0 of 34
    on the main menu        false          0 of 28
    on the Free Play menu   false          0 of 28     (21 of 28 by arrangement alone)

  the step screen_a -> screen_b, which the live run DID make
    ON the Play page       TRUE arrival   11 of 34 look(s)   all 11 identified
    everywhere else         false          0 of 84

  HELD OUT -- rl-playtrain-1, a separate capture nothing was learned from
    ON the Training page   TRUE arrival   39 of 40 look(s)   all 39 identified
```

Arrivals land only on looks carrying a reading, which is why 10 of 28 and not 28 of 28: the
recording holds a reading on every third frame. Of the looks that have one, every one arrives.

**Over the 222 folds** of [[Experiment-047-what-one-reading-is-allowed-to-do]]'s grid, 69 of
which merge the Training page onto the Play page's identity — the live failure, reproduced:

```
  established in the 69 merged folds        69      (0 before this change)
    by a reading that identified it         69
    by one that only failed to refuse it     0
  looks after the page appeared             mean 5.33, worst 8
  FALSE arrivals on the Play page, all 222 folds    0 look(s)
```

The arrangement answers on the first look; the corroborating reading takes five looks on average
and eight at worst, about three to five seconds at the live cadence, against a step patience of
60 observations. **The four-look dwell does not bind a destination**, because the destination is
the page the workflow stays on to act.

**Two false arrivals survive at full vocabulary, both on `screen_c -> screen_d`, and both are
worth stating rather than filing:**

```
  frame 141  agrees with screen_d at 0.759 against 0.606, reading keeps 40%
  frame  93  agrees with screen_d at 0.604 against 0.523, reading keeps 40%
```

Frame 141 is the transition frame — `FREE PLA…` and `CREATE` are already on the glass, because
the submenu is appearing — so calling it an arrival one look early is not a wrong answer. Frame
93 is a Play-page look, and it is a real false positive: the 40% it keeps of
`create free join online play` is `create` and `play`, which half this game's menus say. **The
positive term's strength depends on how distinctive a saved vocabulary is, not only on how long
it is**, and the rule has no way to know the difference. It is reachable only off-path — the
source `screen_c` would have to be established while a Play-page look is in front of the planner
— and it is not fixed here.

**Thin destinations, measured by cutting a destination's saved words to two:**

```
  screen_c cut to two words    the Play page   0 -> 1 false arrival(s)
                               the submenu     0 -> 6
  screen_d cut to two words    the Play page   1 -> 5
                               the Training page 1 -> 10
```

Every one of those is `Positive == false` and `Thin == true`, so the report names them as the
weakest evidence there is. This is the owner's concern, quantified: below three saved words the
veto is simply unavailable, and what remains is an arrangement contrast plus a reading that could
not have disagreed. The rule still fires, and says so; whether it should is the next decision.

## Since — an execution policy for a thin destination

The owner read the thin-vocabulary measurement above and drew a line that is not a change to the
rule:

> I would make that second point a narrow execution policy: don't automatically advance on
> `Bound.Thin`. Keep the observation and diagnostics, but refuse progression when the
> destination's saved vocabulary is too thin to support meaningful contradiction. This does not
> require changing the recognition algorithm.

So `Runner.Advance` refuses to advance the walk on an arrival marked `Thin`, with the stage
`workflowstore.StepArrivalTooThin`. **Both halves are load-bearing.** The arrival is still
established, still attached to the attempt, still counted in `Established` and `ThinWords`, and
still prints everything it rested on — what it may not do is carry the walk forward by itself.
The refusal ends the run with its report intact, rather than weakening the semantic requirement
to get past it.

It is never resolved by patience: a vocabulary is a property of the artifact, not of this look.
The repairs are to re-teach the screen or to have somebody confirm it.

**And the three claims are now three lines, per step**, which the owner asked for separately —
*"do not report success solely because a click was sent"*:

```
  step 1: arrived — ...
    pointer: the operating system puts the pointer at (577,640), where it was sent
    hover:   the control's pixels changed by 161.3/255 over 748 px
    destination: established as "reached" from this look, and a reading IDENTIFIED it
    action: NOT independently verified — nothing was executed, so there is no
            post-action evidence; and arriving where a step expected is not by
            itself proof that the step's action caused it
```

The four readings of the destination line — independent, identified, only failed to contradict,
and too thin to have refused — are what separate an arrival somebody vouched for from one that
rested on an abstention.

## Enforced by

`internal/director/workflowstore` —
`TestTheLookThisRuleRestsOnAgreesWithTheDestinationAndNotTheSource`,
`TestAWorkflowReachesItsDestinationWhileTheGlobalScreenIsUnchanged`,
`TestAReachedArrivalIsNeverAnActionVerified`,
`TestADestinationNoCloserThanItsSourceIsNotAnArrival`,
`TestAReadingThatContradictsTheDestinationRefusesTheArrival`,
`TestAnArrangementAloneDoesNotArriveAnywhere`,
`TestADestinationSavedWithoutWordsIsNotReached`,
`TestWhatAReadingHasToSayBeforeAnArrivalIsIdentifiedRatherThanUncontradicted`,
`TestAnArrivalOnlyUncontradictedSaysSoInTheArrival`,
`TestAReachedArrivalDoesNotDisplaceAConfirmation`,
`TestAnUndemonstratedStepIsNotReached`,
`TestAnArrivalNeedsSomewhereToHaveArrivedFrom`,
`TestAReachedArrivalDoesNotSurviveTheObservationItWasMadeIn`.

`internal/director/workflowrun` — `TestARunDoesNotAdvanceOnADestinationTooThinToHaveRefusedIt`,
`TestARunAdvancesOnADestinationThatCouldHaveRefusedIt`.

`cmd/director` — `TestAnAttemptSaysWhatItEstablishedAndThatTheActionIsStillNotVerified`,
`TestTheGrounderAsksAboutTheDestinationWhileTheSourceIsStillEstablished`,
`TestTheGrounderWillNotArriveOnAReadingThatSaysSomewhereElse`,
`TestTheGrounderWillNotArriveWhileTheLookIsStillTheSourcePage`,
`TestTheDestinationIsScoredAgainstThisLookAndNotTheStandingScreensSignature`.

`cmd/perceptionreplay` — `TestTheLearningFoldKeepsWhatEachScreenWasReadToSay`,
`TestASavedArtifactCarriesItsScreensVocabulary`,
`TestNoReplayModeSavesAWorkflowWithoutItsVocabulary`.

`internal/director/observe` — `TestAVisualScreenCannotReachDurableMemory`, whose allowlist
records why `reached.go` is permitted to name an arrangement at all.

Seventeen mutants run and all seventeen killed. For the policy and the report: a thin arrival
advancing anyway; the refusal dropping the arrival's diagnostics; every arrival refused as thin;
the not-verified line dropped; a merely-uncontradicted arrival reading as identified; a thin
arrival not named as thin. For the rule itself: the destination question never asked; the grounder fed
the standing screen's stored signature; the contrast term dropped; the corroboration dropped; no
reading required; a wordless destination accepted; every reading reported as having identified
the destination; a thin vocabulary not reported; a confirmation overwritten; the learning fold
dropping each look's reading; and the fold keeping no vocabulary.

## Related

- [[Experiment-048-the-destination-a-workflow-can-see-for-itself]] — the investigation, the two
  gates, and the before-and-after.
- [[Experiment-046-the-destination-it-reached-and-did-not-recognise]] — the live run, and the
  sixty observations.
- [[ADR-186-a-person-can-say-where-they-are]] — the establishment path this adds a ground to, and
  why an inference is weaker than somebody looking.
- [[ADR-183-remembering-recognising-and-being-allowed-to-act]] — the three lifetimes, which is
  why a reached arrival cannot become durable knowledge.
- [[ADR-194-a-screen-is-also-what-it-says]] and
  [[ADR-199-one-reading-may-refuse-a-merge-and-may-not-mint-a-screen]] — what words may do to the
  GLOBAL model, which this leaves exactly as it was.
- [[ADR-195-a-reading-that-arrives-after-the-look-it-describes]] — the admission rule the
  corroborating reading goes through.
