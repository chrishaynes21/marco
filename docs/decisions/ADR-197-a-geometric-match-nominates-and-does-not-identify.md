---
type: decision
status: accepted
date: 2026-10-14
deciders: director
affects:
  - passive-observation
  - execution
source_paths:
  - internal/director/observe/identity.go
  - internal/director/workflowstore/stepplan.go
  - internal/director/workflowrun/workflowrun.go
  - internal/director/workflowrun/authorise.go
  - cmd/director/performworkflow.go
  - cmd/director/readingeyes.go
---

# ADR-197 — a geometric match nominates, and does not identify

## Context

Re-acquisition is `observe.sameControl`: two rectangles are two readings of one control when each
contains the other's centre. Symmetric, threshold-free, robust to a proposer that found the same
row slightly larger on a later frame — and the whole of the test. It consults no pixels and no
text.

[[Experiment-044-two-live-runs-that-pressed-nothing-and-pressed-grass]] measured what that costs.
Rocket League's main menu is bottom-anchored; signed out it loses two entries and both side
panels, so PLAY sat 95px below the rectangle the demonstration recorded. A proposer boxed a
128x24 patch of pitch inside the old rectangle, re-acquisition reported MATCHED, and the step
pressed grass — while the word PLAY was read, at half resolution, on the same look.

So the sentence the report printed, `re-acquired on this look`, was true of the geometry and
false of everything a reader would take it to mean. And the repair that would have been right —
[[ADR-193-a-control-re-grounded-from-the-workflows-own-name]], which grounds a control by what it
says — was never asked, because a match is not a refusal.

## Decision

**A geometric match may nominate a control. It is not sufficient to authorise a press when
stronger contradictory evidence is on the same look.**

The asymmetry is the one [[ADR-194-a-screen-is-also-what-it-says]] established for screen
identity, one level down: **text may REFUSE a match and may never propose one.** Nothing in
`observe.ControlIdentityOf` mints a target, moves a press or reaches the artifact. The strongest
thing it does is say "not that one", after which the step is refused and the repair — the thing
that IS allowed to nominate — is asked.

### The contradiction is a competing candidate, not a silence

```
  the matched region says what this control should say          corroborated, press
  it says nothing, and nothing else is named as this control     unexamined,  press
  no words were read on this look at all                         unexamined,  press
  it is not named as this control, and something else is         CONTRADICTED, refuse
```

The second and third rows are the whole safety argument. A control may be an icon, its label may
be drawn outside its clickable rectangle, or OCR may simply have missed it. **Missing evidence is
not contradictory evidence**, and a rule that treated it as such would make every unlabelled
control in every application unpressable on a bad frame.

Corroboration is asked FIRST, and that order is load-bearing: two rows of one menu can both be
named in a workflow called "enter free play", and a rule that refused whenever anything else also
qualified would refuse the right answer for having company.

Ambiguity is preserved rather than resolved. Several competing candidates still refuse the match
— whatever is right, the matched region is not — and nothing here chooses the replacement. That
is the repair's question, and it refuses ambiguity too.

A band across the window is never a competing candidate. It locates nothing, so a word inside it
says nothing about where anything is.

### Saved control text first, the workflow's name as a weaker fallback

`Artifact.ExpectedWords` returns the words AND their provenance, because they are not equally
good. Text a demonstration read ON the control is somebody's actual intent — they clicked the
thing that said PLAY. The workflow's name is the workflow corroborating itself. No demonstration
writes `Control.Text` yet; the field and the preference exist ahead of the recorder because a
fallback that nothing could ever be preferred over quietly becomes the only rule there is.

The provenance is printed wherever the evidence decides anything, with the words themselves, so a
refusal can be weighed from the report alone.

### The report stops describing a coincidence as an identity

`StepReadiness.Grounding` is separate from `Stage`, because a plannable step is plannable either
way:

```
  geometrically re-acquired                                   the weakest thing still pressed
  geometrically re-acquired and semantically corroborated
  semantically re-grounded for this session
  what was matched is not the control                         StepControlContradicted
  control not re-acquired                                     StepControlUnresolved
```

A repaired region is NOT examined. The repair established it by reading this screen; asking the
same words to confirm themselves would be one source voting twice, and `GroundingRegrounded`
already says what the evidence was, including that it is weaker than somebody pointing.

### Where the words come from, and when they stop being evidence

The look's words reach readiness through `Sighting` and `Now`, supplied by the loop — this
package can see no screen, and a coordinator that decided what a region said would be deciding
its own click target. They are the positions of an **admitted** `observe.ScreenReading`, held
across looks because OCR answers about twice a second against a loop that looks ten times, and
dropped the moment `observe.Current` stops being true of them. That is not "the same screen is
standing" but **no screen change since the capture** — see
[[ADR-195-a-reading-that-arrives-after-the-look-it-describes]].

### What this is NOT

**Not a hover gate.** The probe measured `0.0/255` on the grass it pressed, against `136.7/255`
for the same button in [[Experiment-041-the-control-Marco-found-for-itself]], and it stays
diagnostic. A control that does not highlight is an ordinary control, and two samples do not make
a threshold.

**Not a coordinate tolerance, and not a retry.** Those make Marco better at retrying the wrong
target rather than better at knowing what it is clicking.

## Consequences

Verified offline on the same menu, one frame through the production detector and the production
OCR at the production scale, against the region the live run actually matched:

```
  the detector proposed 7 region(s); OCR read 35 word(s) at half size
  * "PLAY"        x  60.. 367  y  564.. 608     the only candidate named in "enter free play"
    "GARAGE"      x  59.. 368  y  610.. 654
    "CLUB"        x  59.. 368  y  658.. 703
    "PROFILE °"   x  59.. 367  y  706.. 750
    "SETTINGS"    x  59.. 367  y  752.. 797

  the region the live run pressed:  x 100.. 227  y 478.. 501
    verdict: contradicted by a better-named candidate
    instead: "PLAY" at x 60..367 y 564..608
```

Exactly one candidate qualifies, so the repair that follows the refusal has an unambiguous
answer. **The press that was sent would not have been sent.**

One operation moved and nothing else did. Perception still proposes, identity evidence
corroborates, readiness authorises, execution acts — and the defect was readiness treating a
proposal as an identity.

## Enforced by

`internal/director/observe` — `TestAMatchOnBackgroundIsContradictedByTheLabelElsewhere`,
`TestAnUnlabelledControlIsNotContradictedByItsOwnSilence`,
`TestWithNoWordsAtAllNothingIsContradicted`,
`TestWithNothingRecordedToMatchAgainstTheQuestionIsNotAsked`,
`TestAMatchThatSaysWhatItShouldIsCorroboratedEvenWithCompany`,
`TestTwoCandidatesNamedTheSameAreReportedAndNotChosenBetween`,
`TestABandIsNotACompetingCandidate`, `TestTheSmallestContainingCandidateTakesTheWord`.

`internal/director/workflowstore` — `TestAMatchOntoSomethingElseIsRefusedBeforeItBecomesAPress`,
`TestACorroboratedMatchIsPlannableAndReportedAsCorroborated`,
`TestAnUnlabelledMatchIsPlannableAndReportedAsGeometryOnly`,
`TestCompetingCandidatesRefuseTheMatchAndChooseNeither`,
`TestARepairedRegionIsNotCheckedAgainstTheWordsThatMadeIt`,
`TestSavedControlTextIsPreferredOverTheWorkflowName`.

`cmd/director` — `TestPerformWorkflowInstallsItsGates`, which names all three halves of the
wiring: the candidates, the words, and the `observe.Current` check that stops the last words read
from standing forever.

## Related

- [[Experiment-044-two-live-runs-that-pressed-nothing-and-pressed-grass]] — the press this refuses.
- [[ADR-194-a-screen-is-also-what-it-says]] — the same asymmetry, one level up.
- [[ADR-196-a-control-that-moved-is-not-a-control-that-was-covered]] — the other half of reaching
  the repair, and the rule this one makes reachable in a single look.
- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — what a refusal hands over to.
- [[ADR-190-the-pointer-arrives-before-the-button-goes-down]] — the hover probe, kept diagnostic.
