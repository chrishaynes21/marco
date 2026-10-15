---
type: decision
status: accepted
date: 2026-10-14
deciders: director
affects:
  - passive-observation
  - execution
source_paths:
  - cmd/director/reground.go
  - cmd/director/readwords.go
  - internal/director/workflowrun/workflowrun.go
  - internal/director/workflowstore/stepplan.go
---

# ADR-193 — a control re-grounded from the workflow's own name

## Context

Four of the nine controls in the measured Rocket League artifact are bands across the whole
window. They re-acquire perfectly on every look and locate nothing, so
[[ADR-190-the-pointer-arrives-before-the-button-goes-down]]'s `Region.Locates` refuses them —
correctly, because the middle of the window is not the position of anything in it.

That leaves a step that can never be performed, however long it is given. No amount of looking
repairs it: the demonstration recorded a rectangle that says nowhere, and the rectangle is the
thing that is wrong.

Hand-editing the artifact would fix this workflow and teach Marco nothing. What is wanted is for
Marco to re-establish the control from what is actually on the screen, and to refuse when it
cannot.

## What evidence there is, and what there is not

There is plenty about WHERE things are. The learned detector boxes six tiles on the Training
page; OCR reads FREE PLAY, CUSTOM TRAINING, TUTORIAL, AERIAL, STRIKER and GOALIE at about 95%
confidence. Localisation was never the problem.

There is almost nothing about WHICH ONE WAS MEANT. The artifact calls it `control_3` — a
generated name — with no label, no text and no container. The demonstration captured where
somebody clicked and not what they clicked, so **the intent was never saved**.

The one human-supplied meaning in the whole file is the workflow's NAME: somebody typed
"enter free play".

## Decision

A control whose reading locates nothing may be re-established, for one session, from the words
on the screen agreeing with the workflow's own name. The rule is deliberately strict:

- every word of a candidate's text must appear in the workflow's name,
- whole words, lowercased, one direction only,
- the candidate must be a region a source actually found and which itself locates something,
- and **exactly one** candidate may qualify.

Zero, or two or more, and the step stays refused with the candidates reported. A rule that took
the best of several would be choosing Marco's own click target on a similarity score, which is
the thing every boundary in this subsystem exists to prevent.

The strictness is the value. A substring rule would make PLAY match PLAYLIST and REPLAY; a
bidirectional one would let any label sharing a common word with a long task name qualify. On
the measured run, 65 candidates carried text and one qualified: CUSTOM TRAINING, TUTORIAL,
AERIAL, STRIKER and GOALIE did not.

### Four separations the repair does not collapse

- **It replaces the file's reading, and is not narrowed against it.** The agreement rule of
  ADR-190 asks what the file and this look both describe, and in this case the file's reading is
  the broken input — narrowing a good live region against a window-spanning one hands the
  window-spanning one straight back.
- **It does not skip `Locates`.** A repair that named another band would be the same unpressable
  step with a better provenance, and provenance is not a position.
- **It is not permission.** Re-grounding is asked at the stage where the step has no position at
  all, which is earlier than any question about who agreed. That is right — where a control is,
  is a perception question, and the answer actuates nothing. A re-grounded step is still refused
  unless somebody has agreed to it and the session carries live authorisation.
- **It is asked once per control, not once per look.** The loop runs ten times a second; a
  second answer that differed from the first would mean the run pressed two different places for
  one step.

### And exactly what it is worth

Less than a person pointing at it. The workflow's name agreeing with a label on screen is **the
workflow corroborating itself** — the same weakness `Arrival` reports as depth — and it is
reported that way rather than presented as a finding. It lasts one session, is held in
`Now.Reground` with a screen binding's lifetime, and writes nothing.

`--repair <path>` emits a proposal beside the artifact, never into it: the new region, the
evidence sentence, the readings it would replace, and **two** outcome fields — `the_screen_changed`
and `the_expected_destination_was_recognised`. Two, because a repair that produced a transition
nobody recognised is a good region and a recognition problem, while one that produced nothing is
a bad region. Collapsing them would file the first as a failure of the thing that worked.

A workflow corrected in place by the run that used it would make the next run's evidence depend
on this one's guess, with the provenance gone by the time anybody looked.

## Consequences

Measured live, in Experiment-041: Marco opened Main Menu → Play → Training → **Free Play**, and
the third target was recovered from the live screen rather than from the file. Worth a reviewer's
eye: the repair grounded the text label (340..482 × 466..500), not the tile. It works because the
label is inside the clickable area, and a reviewer can see that it does.

The honest limit is the same one the experiment records: the navigation succeeded and the arrival
was not recognised. The repair is about where a control is. It says nothing about whether Marco
can tell which screen it arrived on.

### Three defects the deletion check found afterwards

Writing the mutants for this path turned up three places where the code said something it could
not mean, all of the same shape as the `target:` line that printed the saved rectangle while
pressing the right point:

- **A refusal that could never be printed for the reason it stated.** "N candidate region(s) and
  none of them contains any text" fired only when there were no locatable candidate regions at
  all — a different finding, now reported in its own words.
- **A count labelled as something it was not.** "65 candidate(s) carry text" was every
  candidate; how many carried text was never computed. The live listing's first eight rows were
  all blank, which is how it went unnoticed.
- **A metric that could print a negative.** `Sent - Established` is not the number of actions
  that moved an unrecognised screen: a destination can be established with no press behind it, so
  the difference can cross zero. It is counted per step now.

## Enforced by

`cmd/director` — `TestOnlyALabelWhollyNamedInTheWorkflowsNameQualifies`,
`TestANamelessWorkflowGroundsNothing`,
`TestAStrayLetterFromOCRIsNotAWord`,
`TestABandIsNotACandidateHoweverWellItsTextMatches`,
`TestTheSmallestContainingCandidateTakesTheWord`,
`TestAWordOutsideEveryCandidateIsNotAttributedToOne`,
`TestAWordStraddlingACandidatesEdgeIsNotInsideIt`,
`TestALookThatOfferedOnlyBandsIsReportedAsThat`,
`TestExactlyOneQualifyingCandidateIsTheAnswer`,
`TestTwoCandidatesThatQualifyEquallyAreRefused`,
`TestNoQualifyingCandidateIsRefusedAsAnIntentNeverSaved`,
`TestEachWayTheWordsCanBeMissingIsReportedSeparately`,
`TestAProposalSeparatesAChangedScreenFromARecognisedArrival`,
`TestARunWithNoRepairsWritesNothing`,
`TestNoRegrounderInstallsNothingRatherThanATypedNil`.

`internal/director/workflowrun` — `TestAControlThatLocatesNothingIsAskedAboutAndThenPressed`,
`TestARepairIsReportedWithItsEvidenceAndTheArtifactIsNotTouched`,
`TestARegrounderThatRefusesLeavesTheStepRefusedAndSendsNothing`,
`TestAControlIsAskedAboutOnceAndNotOncePerLook`,
`TestARepairSurvivesToTheNextLook`,
`TestAStepRefusedForAnotherReasonIsNotRegrounded`,
`TestARepairDoesNotStandInForSomebodyAgreeingOrAuthorising`,
`TestARunWithNoRegrounderStillRefusesAControlThatLocatesNothing`.

`internal/director/workflowstore` — `TestAControlRegroundedForThisSessionReplacesWhatTheFileSays`,
`TestARegroundedRegionIsNotNarrowedAgainstTheReadingItReplaces`,
`TestARepairThatNamesAnotherBandAcrossTheWindowIsRefused`,
`TestARepairIsAboutOneControl`.

Every one of these is in the deletion check in `mutate220.py`.

## Related

- [[ADR-190-the-pointer-arrives-before-the-button-goes-down]] — `Locates`, and the agreement rule
  this one deliberately replaces rather than extends.
- [[ADR-192-a-second-source-in-the-live-loop]] — where the candidate regions come from.
- [[ADR-188-performing-a-saved-workflow]] — the walk this repair happens inside.
- [[Experiment-041-the-control-Marco-found-for-itself]] — the live run.
- [[ADR-196-a-control-that-moved-is-not-a-control-that-was-covered]] — the case the exclusion
  argued here does not cover, and when an unresolved control may be asked about.
