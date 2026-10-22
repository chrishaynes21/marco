---
type: adr
status: accepted
date: 2026-10-21
decision: "ACCEPTED — a demonstration keeps, per pressed control, the click in WINDOW pixels (the capture includes the frame, so they are not client coordinates and the record does not call them that), the size of that capture, a display scale recorded as UNKNOWN because nothing in the recording path observes one, a crop of the control AT CAPTURE RESOLUTION, and what that crop was measured to be worth. The region is derived from the proposal under the press — the control's own box plus a quarter of itself — and never from a constant, because a constant means something different at every capture scale. The worth is measured TWO-SIDED across the looks ENDING AT the press — never the ones after it, because a press that works closes the menu it was in and a control measured across that reads as unstable BECAUSE THE CLICK SUCCEEDED: the WORST the control matches itself, and the BEST anything within 160px of it matches the same template, with 'somewhere else' defined as half the template in each direction, over pixels retained from the TEMPLATE and the positions it will be searched at rather than from the control's own box — and the extent actually covered is recorded, because a gap measured over less of the neighbourhood than it claims reads better than it is. A threshold is placed in the gap between them, or, when there is no gap, nothing is placed at all and the recording SAYS SO while the person is still in front of the screen — naming the press by the words read on it. One look is a template and not a calibration. The evidence is stored beside the demonstration under the artifact's own labels and keyed by revision, is removed with it, and is refused when its shape is one this build does not know. Nothing that runs reads it: this is the evidence a guard would be built from, collected first so the guard has something to be built FROM rather than a constant chosen by somebody. And there is ONE look number, kept by the observer and handed back from the fold that advanced it, because everything a demonstration files under a look is read back by the look a press was attributed to — a second counter beside it ran one behind and meant that for three recordings NOTHING was written and nothing said so."
supersedes: []
affects:
  - demonstrations
  - perception
source_paths:
  - internal/director/controlevidence/controlevidence.go
  - internal/director/match/match.go
  - internal/director/demo/controls.go
  - cmd/director/visualevidence.go
  - cmd/director/visualsession.go
---

# What a click is evidence of

## The two halves, and why neither is enough

> The demonstrated point is authoritative about **which instance was taught**, not proof that the
> same instance still occupies that location.

A click says WHICH of several identical controls a person meant. It says nothing about whether
that control is still there. A template says what the control LOOKS LIKE. It says nothing about
which of two identical ones was intended — and on a real screen that is not a hypothetical:
[[Experiment-096-the-click-says-which-one-the-template-only-says-what]] measured whole-window
matching choosing the control in the *other* panel thirty times out of thirty whenever the
demonstrated one was absent or covered, with no ambiguity reported, and
[[Experiment-097-a-translucent-menu-is-not-what-defeats-a-local-template]] found the same hazard
on the owner's own game: a repeated volume value ranked a different row above the demonstrated
one, 0.9151 against 0.8926.

So a demonstration keeps both, and this ADR is about what it takes to keep the second one
honestly.

## Measured on both sides, or not at all

The caution that shaped this:

> positive self-matches alone can make the guard too permissive, so evaluate them against nearby
> non-targets as well

It is not a refinement. It is the difference between a measurement and a reassurance, and the
numbers say so. The same control at full resolution, half, and a third:

| resolution | matches itself | nearest rival | separation |
| --- | --- | --- | --- |
| 1920x1080, as captured | 1.0000 | 0.5962 | 0.4038 |
| 960x540 | 1.0000 | 0.6229 | 0.3771 |
| 640x360 | 1.0000 | 0.7111 | 0.2888 |

The self-match does not move. The margin a guard actually has falls by 28%. A one-sided
calibration is exactly blind to the thing degrading it, so the stored number is the GAP and a
threshold exists only inside one.

## What follows from that, concretely

**The crop is kept at capture resolution.** Not a thumbnail. The screen previews beside it are
thumbnails because a person looks at those; this is measured, and a thumbnail destroys the
measurement. It is why control evidence is stored apart from previews rather than one being made
to serve both.

**The region comes from the control.** The proposal under the press, plus a quarter of itself —
the same fraction `preview` keeps, and deliberately not grown further. A region enlarged until it
swallows a landmark makes appearance sufficient and the click redundant, which is a different
guard with a different failure. The smallest proposal under the press, because proposals nest and
what somebody pressed is the button rather than the bar it sits in.

**"Somewhere else" is half the template in each direction.** A 340-pixel template has a peak
hundreds of pixels broad; excluding only a dozen of them reports the shoulder of the same match
as the nearest rival, which understates the hazard by measuring a control against itself. This
was a real error in the first version of the measurement, found by running it at full resolution.

**One look is a template and not a calibration.** A control seen once has never been seen to STAY
the same, and staying the same is the whole question for a menu drawn over something moving.

**A control that cannot be told from its neighbour is said so about at recording time**, naming
the press by the words read on it. Not refused later, mid-Act, with nobody there: the only moment
anything can be done about it is while the person is still in front of the screen that contains
it.

## Three ways this was wrong first

**It measured the looks AFTER the press.** A press that WORKS closes the menu it was in, so the
control legitimately stops being there — and a calibration built on that reports every
well-behaved button in a navigation menu as unusable BECAUSE THE CLICK SUCCEEDED. Pre-action
stability is measured on the pre-action screen: the looks ending at the press, out of a ring the
recorder keeps as it goes. What happened afterwards is the outcome's question and the outcome
measures it. `TestAButtonThatDisappearsBecauseItsClickWorkedIsStillUsable`.

**The pixels it kept did not cover the search it advertised.** The search moves a TEMPLATE, not a
point: its top-left runs a band either side of the region and at the far end the template extends
its own width past that. Keeping the control's box grown by the band is smaller than that, so the
displacement a rival can be found at is quietly shortened and a rival near the declared edge is
never scored. Nothing about the result would have said so — which is why the extent actually
covered is now recorded beside the numbers. `TestARivalAtTheEdgeOfTheNeighbourhoodIsFound`.

**It called window pixels a client area, and invented a display scale.** The capture includes the
frame. A consumer converting from a client area it was never given would be wrong by the width of
a border, and one told the scale was 1 would compare a window captured at 150%% against one
captured at 100%% and call them the same layout. Unknown is a third answer and stays one.
`TestTheCoordinatesSayWhichPixelsTheyAre`.

## And a fourth way, which meant none of it was written

The three above are measurement errors: the number came out wrong. This one is worse, because no
number came out at all.

A demonstration keeps three things per look — the raw moment, the picture of the screen, and the
band of pixels a control is calibrated from — and reads all three back by ONE key: the look the
OBSERVER says the press arrived on, because the observer is the only thing that attributes a
press to a control. The look loop kept a second counter of its own, taken BEFORE the fold rather
than from it, so it ran exactly one behind: the observer counts from one and the loop counted
from zero.

Nothing failed. The pictures were filed, the evidence was measured, each half was perfectly
consistent with itself — and every lookup between them missed by one and returned nothing. So no
control crop and no control evidence was ever written, by any build, and **not a word was said
about it**: `keepPreviews` skipped the control it could not find a frame for, and `keepControls`
returned a count of zero with no reason, which the recorder prints nothing for.

It was found in the owner's own store, three recordings in, by the absence of a `controls`
directory that every one of those recordings should have made. The fingerprint was in
the raw record all along — ONE press, carried as moment 59 and as press 60.

Three things follow, and the third is the one that generalises:

- **There is one look number and the observer keeps it**, handed back from the fold that advanced
  it. Not a count the loop maintains beside it, however carefully.
- **A total miss is a reason, not a silence.** Each `continue` in the mapping is legitimate alone
  — a step with no target, a press the proposer found nothing under, a press past the bound — so
  the only thing that separates this defect from an ordinary quiet recording is that ALL of them
  missed while presses were attributed and looks were measured. That now comes back as a sentence
  instead of as a zero.
- **A separately constructed fixture could not have caught it, and did not.** The test that
  existed handed the same look number to both halves — to the thing that filed the evidence and
  to the evidence it would later be looked up by — so they agreed by construction. The regression
  test writes no look number at all: it folds through a real observer, lets the production fold
  number the looks, lets the observer attribute the press, and runs the real save path. The
  numbering was lifted out of the loop into `visualResult.fold` for exactly that reason, because
  `watch` itself needs a desktop and cannot be driven. This is the same lesson as the memory note
  *two lists, one fact*, arriving from a new direction: it is not enough for each half to be
  tested, the test has to make the two halves meet.

This is also why the translucent-menu question in the last section stayed open longer than it
had to: the recordings that would have answered it were made, and measured, and discarded.

## Where the measuring happens, and why not in the loop

A look is 480 milliseconds. Measuring a template against everything within a band of itself is
about a billion operations, and a recorder that did that while somebody was still demonstrating
would drop looks — on the one path where a dropped look is a reading nobody takes and an outcome
nobody can check.

So the LOOKS are kept, cheaply, and the measuring happens once at the end when nothing is
waiting. What is kept is a band of grey levels around the control rather than the frame it came
from: a 1920x1080 frame is eight megabytes and a band around a button is a couple of hundred
kilobytes, and the band is all the calibration ever looks at. Bounded the way the pictures are —
five looks per press, eight presses.

## What this deliberately does not do

**Nothing runs on it.** No grounder, no verifier, no runner reads a byte of it. A demonstration
recorded without it performs exactly as one recorded with it. It is collected first so that a
guard around a learned coordinate has something to be built FROM — a threshold measured where the
control is — rather than a constant chosen by somebody and carried onto screens it was never
measured on.

**KNOWN LIMITATION, open: the press-associated look may already be after the action.** A P2, and
not to be relied on for execution until it is closed. The look loop drains the input
subscription and THEN acquires the frame, so the look a press is attributed to is captured after
that press happened. On a fast menu transition that frame can hold post-action pixels, or carry
no proposal under the click at all, and the calibration folds it in and takes the control's
bounds from it. The loop already states the principle for the SCREEN — attributing inputs to the
screen they may have just caused would be that layer deciding the causality it exists to record
— and this does not yet honour it for the pixels. What it costs is bounded, because the other
looks in the ring are genuinely pre-action: one late frame moves the worst case rather than
defining it. What closes it is selecting frames demonstrably PRECEDING the press and taking the
target bounds from that same context, with a test that removes the control BEFORE the first
capture after the press.
`TestAButtonThatDisappearsBecauseItsClickWorkedIsStillUsable` does not catch it: it supplies the
intact button on the press-associated look and removes it only afterwards.

**It is not a recogniser.** It knows one control, in one window, at one size, and says so about
anything else. No vocabulary, no whole-screen matching, nothing that generalises to a control it
was not shown.

**It does not yet cross the no-coordinate execution boundary.** The click is recorded as
DEMONSTRATION EVIDENCE, which [[ADR-181-a-demonstration-marco-can-keep]] and
[[ADR-183-remembering-recognising-and-being-allowed-to-act]] have always allowed; what they
refuse is a coordinate becoming a durable identity to press. Nothing here presses anything. The
narrow exception for fixed-layout geometry replay was authorised separately and is not taken by
this change; when it is taken, it will be its own decision with its own evidence.

**It is kept under the same licence as the pictures.** `--previews`, because it keeps the same
kind of thing — a crop of somebody's screen — at a higher resolution, and a recording that
printed "No frames and no screenshots are kept" while keeping one would be lying.

## And the translucent case is still open

The full-resolution measurement above is of an OPAQUE game menu: the only full-resolution menu
frames in this repository are Rocket League's pause panel, whose pixels do not change while the
scene behind it does. The translucent case was measured only at 640x360, on the owner's previews.

This change is what closes that without asking anybody to interrupt a game: every demonstration
from here keeps a full-resolution crop of its own controls, so the next recording on a
translucent menu measures it as a matter of course.

## Enforced by

- `internal/director/controlevidence/controlevidence_test.go` —
  `TestAControlIsCalibratedAgainstItselfAndItsNeighbours` (the threshold lands inside the gap and
  the crop is the region's own size); `TestAControlThatCannotBeToldFromItsNeighbourIsSaidSoAbout`
  (two identical controls leave no gap, and the reason names what to do);
  `TestOneLookIsATemplateAndNotACalibration`;
  `TestTheRegionComesFromTheControlAndNotFromAConstant` (four times the control, four times the
  region); `TestALookOfAnotherSizeIsNotFoldedIn`;
  `TestABandAroundTheControlFoldsInLikeTheWholeWindow` (the production path scores identically to
  folding in whole frames, and a band that stops short of the control is not a look at it);
  `TestTheRecordShapeMovesWithItsVersion`; and `TestTheCalibrationOnRealGameFrames`, which is the
  one assertion over real 1920x1080 game frames rather than drawn ones — nine looks, 1.0000
  against 0.5999, the rival a sibling button rather than the same match seen off-centre.
- `cmd/director/keepcontrols_test.go` —
  `TestEachControlsEvidenceIsFiledUnderItsOwnLabel` (asserted by WHERE each record says its click
  was, which is what a drifted label mapping would get silently wrong);
  `TestAControlWithAnIdenticalNeighbourIsSaidSoAboutAtRecordingTime` (and the sentence names no
  machine label); `TestAPressOnNothingIsNotGivenEvidence`; `TestTheEvidenceHeldIsBounded`;
  `TestTheControlIsTheSmallestProposalUnderThePress`;
  `TestTheLoopCollectsControlEvidenceAndTheCommandWritesIt`, which is a SOURCE guard and says so
  — it pins that the loop is handed this look's own presses and proposals, because passing `nil`
  would compile, collect nothing, and leave every other test passing.
- `internal/director/demo/controls_test.go` —
  `TestEvidenceOfOneRevisionIsNeverServedForAnother` (and the superseded one is gone from disk,
  not merely unserved); `TestForgettingARoutineForgetsWhatItsControlsWereWorth`;
  `TestEvidenceOfAnotherShapeIsRefused`.
- `cmd/director/looknumbering_test.go` — `TestWhatTheObserverNumberedTheSavePathFinds`, which
  runs the production path end to end and **writes no look number at all**: drawn looks are folded
  through a real `fastobserve.Observer` by the production `visualResult.fold`, the observer
  attributes the press, `workflow.Learn` and `workflowstore.SaveWith` build the artifact, and then
  `keepControls` and `keepPreviews` are asked for the control. Every number on both sides of the
  lookup is production's, which is the point — the fixture that existed before handed the SAME
  number to both halves, so they agreed by construction and the disagreement could not arise.
  Also `TestAllTheLooksMissingIsAReasonAndNotASilence`, which numbers that same real press one
  behind and requires a sentence back, with the two quiet cases held quiet — one of them the
  ordinary press the correlator could attribute to no control at all.
- `TestTheLoopFoldsEveryLookThroughTheOneNumbering` is the one remaining SOURCE guard and says so:
  `watch` builds its own window provider, capture path and navigation hook, so it needs a desktop.
  What it pins is not what the numbering does but that the live loop goes through it, and that
  there is nowhere else a look number could come from. The numbering was lifted out of the loop
  into `fold` precisely so that the rest could be driven.
- Four more mutations for the fourth defect, each killed by the end-to-end test alone and each
  reported as the two numbers that disagreed: the original counter-before-the-fold put back, the
  same error one look the other way, the evidence filed one behind the picture, and the picture
  one behind the evidence. Three more against the reason: it removed, and each of its two guards
  dropped so that it fires on a recording with nothing to complain about.
- Nineteen mutations for the first three, all killed: the loop's collection removed and its arguments emptied, the
  command's write removed, the save removed, the largest proposal taken instead of the smallest,
  both bounds removed, a press on nothing admitted, the sentence never said, the no-gap refusal
  disabled, the superseded evidence kept, the forget removed, the version check dropped — and the
  three defects above put back one at a time: the looks after the press folded in, the retained
  pixels taken from the control's box, `Needed` returning the region, the searched extent not
  recorded, a scale of 1 invented, and `ScaleUnknown` defined as 1.

## Related

[[ADR-246-a-trial-collects-its-own-evidence]] is how the trials that produced these measurements
were read. [[ADR-243-a-door-of-its-own-and-the-order-in-the-file]] is the Act a guard built on
this would protect.
