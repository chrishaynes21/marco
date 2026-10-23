---
type: adr
status: accepted
date: 2026-10-22
decision: "ACCEPTED, NARROWLY — GEOMETRY REPLAY IS AN EXPLICIT EXECUTION MODE. A run GRANTED it presses THE POINT THE PERSON CLICKED, taken from the press and never computed from a rectangle a proposer drew around it, and only while the window is the exact size it was demonstrated in. The window is checked ALWAYS in that mode — at the plan and again at the press boundary — and not only when recognition fails: a check that runs only after the vision path gives up is not the licence it claims to be. The press boundary in that mode has its OWN terms: the window alive, the application in scope, the window not replaced, the target leading the desktop, and the geometry unchanged. It does not ask for a screen hypothesis, an observation generation, a look taken at the instant of the press, a reading of that look's words or a proposal under the point, and it takes NO look at all — those are the prerequisites of recognising a control, and this mode exists for the menus where recognition has no answer. The licence is per run, granted from outside the planner and outside the runner, and off everywhere else; a run that carries it still PREFERS a screen it can establish. The demonstration records the window it was made in — outside the pictures licence, because a size is not a picture — and records NOTHING when that window was resized partway, because that is not a demonstration of a fixed layout. A routine with no recorded window is REFUSED on this route rather than trusted, which is the difference between a narrow exception and a hole. The window check is EXACT in both directions and has no tolerance, deliberately. The route replaces exactly two refusals — the screen and the re-acquisition — and skips nothing else: a control recorded as a band across the window still locates nothing, words that put the control somewhere else still contradict, nobody's agreement and nobody's authorisation is waived, and verification of what the press DID is untouched. Everything it plans is reported in its own words — `demonstrated layout` support, a grounding that says in a sentence that nobody looked — so no run's record describes this as recognition."
supersedes: []
affects:
  - execution
  - demonstrations
source_paths:
  - internal/director/workflowstore/stepplan.go
  - internal/director/workflowstore/workflowstore.go
  - internal/director/workflowrun/workflowrun.go
  - internal/director/workflow/workflow.go
  - cmd/director/performworkflow.go
  - cmd/director/demonstratevisual.go
  - cmd/director/visualsession.go
---

# Replaying a demonstrated layout

## The case with no answer

A press is planned by asking, in order: *where am I, is this control's identity settled, can I
see it, has anybody agreed, am I allowed.* The first question is `StepScreenUnestablished`, and
for one whole class of screen it has no answer and never will.

A menu drawn over a live 3D world is filed as two different screens depending on what is moving
behind it. The owner's own Ark recording is the measurement: the same pause menu, one look with
138 proposed rectangles and the server panel read on it, the next with 79 and **zero words that
stayed**. [[ADR-246-a-trial-collects-its-own-evidence]] read three live trials and all three died
there, every one of them with **0 actions sent**. Waiting does not settle it, because there is
nothing to settle — the fold is not slow, it is bimodal.

So this is a SECOND question, asked only when the first has no answer, and it is a different
question rather than a weaker version of the same one:

> not *is this the screen I recognise* but *is this the window the demonstration was made in*

## What it is an exception to

[[ADR-181-a-demonstration-marco-can-keep]] and
[[ADR-183-remembering-recognising-and-being-allowed-to-act]] refuse a coordinate as a durable
replay target. That rule stands everywhere else. This is the narrow override that was authorised
for a fixed-layout MVP, and it is written down here rather than hidden in a UI wording, which was
a condition of granting it.

## Four things it was wrong about first

The first two were found in review, before any of it ran live. The third was found by reading the
owner's next recording. The fourth was found by the trial itself, and nothing short of a trial
could have found it.

All four are one mistake told four times: **a path is not vision-free until every layer on it
agrees to be.** Three layers had to stop requiring a named screen — the step plan, the press
boundary, and the run coordinator in front of both — and each was noticed only after the one
behind it was fixed. A green suite was available at every stage, because each layer's tests
entered at that layer.

**It replayed a detected rectangle's centre, not the person's click.** The point came from the
first saved reading of the control and the middle was computed from it. A proposal is generous on
purpose — a live look once matched PLAY with a region running to the window's left edge, whose
middle landed three pixels inside a button starting at 62 — so its centre is a *different point*
from the one somebody pressed, and on an off-centre click it can be a different control
altogether. That is the segmenter's idea of the control wearing the person's authority.

So the click itself is stored, taken from the press and never from a rectangle: `workflow.Step.At`
and `workflowstore.Step.At`, per STEP rather than per control, because a control pressed twice was
pressed twice and the two points are two facts while readings merge. A proposal is not required
for any of it — a press the proposer found NOTHING under still records where it was, and on a menu
no recogniser can name that is the ordinary case rather than the exception.

**The point reached only half the steps.** A step is built by two loops. The first walks the
transitions the demonstration made; the second (`panelSteps`) picks up the presses that changed
something without changing the screen — a panel replaced under a tab bar that stays put. The
click was wired into the first and not the second, so every press whose screen label was the same
at both ends was refused by this route for having no point, about a press whose point had been
observed and thrown away.

That omission selected for the worst possible half. On the owner's second Ark recording the panel
presses are the majority of the procedure AND all of the usable evidence: GRAPHICS and SOUND each
match themselves at 0.99999 across the looks ending at the press, against the best rival within
160px at 0.35 and 0.44. The one step that did change screen — the click on the translucent pause
menu — proposed a region covering the whole window, 1896x1080 of it, and matched itself at 0.73.
So the step that kept its point was the one worth least, and the two worth most were the ones
refused. A press that changes a panel is the ordinary case on exactly the menus this route exists
for, which is the reason to expect it rather than a coincidence.

The fix is two lines; the lesson is the test. The walk half had four tests driven through `Learn`
and they all passed throughout, because a fixture built over one loop agrees with whichever loop
it was written against. The panel half now has its own, through the same production entry point,
and the geometry route has a same-screen step among its cases — which it did not before, so the
press this subsystem was built for was the one case the subsystem never planned.

**The press boundary still demanded vision.** `worldMoved` asks eight questions, five of which
need a look taken at that instant: a screen hypothesis standing, an observation generation
unmoved, a look that could be taken at all, a reading of that look's own words, and a proposal
under the approved point. The planner would license the press and the boundary would turn it
down, so the mode could not send an action — and the geometry it was granted for was checked
nowhere in that sequence.

So geometry replay has its own boundary, and no look is taken at all on it. A look at the instant
of a press is a window capture, a segmentation and sometimes an OCR request waited for inside a
budget; on this path nothing it produces is consulted, so every one of them is pure cost inside
the one moment that must not have an unbounded pause in it.

**And the run coordinator in front of both still would not start.** The step plan asked nothing
about the screen. The press boundary asked nothing about the screen. And `workflowrun.attempt`,
which runs before either of them, called its grounder on every look and treated a failure to
recognise as a reason to wait — until patience ran out and the run refused, having never reached
the plan.

Found live, and only live. Two runs against the owner's Ark pause menu, whose arrangement agreed
with the best saved screen at **0.500 and 0.442** against a 0.55 threshold. Both refused with
ZERO actions sent. That is the exact case the licence exists for — a menu over a live 3D world,
filed differently depending on what moves behind it — so the gate was turning down the one run it
had been built to let through. The suite was green throughout, because the tests for each inner
layer entered at that layer and the outer one had no test asking it to proceed without a screen.

So a licensed run is no longer held there. The grounder is still CALLED — the screen is worth
knowing when it can be known, and not calling it would be the different and larger claim that
this mode stops perceiving — but failing to name a screen no longer stops the run. Nothing
downstream got easier: the licensed plan still checks the window and the demonstrated point, and
a step that can only be finished by ARRIVING somewhere still cannot be, because nothing
established its destination either.

**What is still vision, on purpose.** A step finished by its own outcome is checked by comparing
what was read before the press with what was read after it, so the panel must be READABLE even
though its screen need not be NAMEABLE. With nothing readable at all the press does not go out,
licence or no licence — a click into a window nothing can say anything about is what this
subsystem exists to refuse. That is the owner's separation holding: localization and outcome
verification are different questions, and only the first one is waived here.

**And the trial could not say whether the licence was on.** Neither the journal's `play` note nor
the director's run record carried it, so two runs were read back as evidence about coordinate
replay while nothing in the evidence distinguished *the mode was off* from *the mode was on and
bypassed*. Both now record it. A trial whose record is silent about the one variable it is about
is not evidence, whichever way it comes out.

## The five things that keep it narrow

**It is granted, never inferred.** `workflowstore.Now.Layout` is set by the caller, exactly as
`Authorised` is, and for the same reason the planner has never been allowed to grant its own
permission. `workflowrun` carries it as a per-run flag that only `WithFixedLayout` sets, and only
`director perform-workflow --fixed-layout` passes it. A planner or a runner that could turn this
on for itself would be the hole the rule exists to close.

**It is a MODE, so its checks always run.** This is the correction that made it honest. While it
was a fallback — geometry consulted only after recognition had already failed — a run whose
screen happened to settle took the recognition path's terms, which say nothing about the window,
and a demonstrated point could go out into a window half the size with every check passing. A
check that only sometimes runs is not the licence it claims to be. So a granted run is in the
mode: the window is checked, the screen is not asked about, and the vision path is untouched and
is what every run without the mode takes.

**The window is recorded, and it is checked EXACTLY.** Every reading in an artifact is normalised
to its window, so the arithmetic produces a confident-looking point at any window size whatever.
That is precisely why the size is asked about rather than trusted to come out in the wash. There
is no tolerance and that is the decision, not an oversight: a tolerance would be this subsystem
deciding how much change a menu survives, which is the question nobody has measured and the
reason the licence is narrow at all. A window one pixel different is a different window, and the
person gets a sentence instead of a click somewhere plausible.

**A routine that cannot say what window it was demonstrated in is refused.** Not waved through.
The licence permits a demonstrated point to be pressed WHILE the window is unchanged, and an
artifact with no recorded window cannot support that sentence however plausible its coordinates
look. Every routine recorded before this change is in exactly that position, and
`StepLayoutUnrecorded` says what fixes it: demonstrate the Scene again.

**It replaces the questions that need vision, and no others.** The screen, the re-acquisition
that depends on it, and — at the press boundary — the look taken at that instant with everything
read off it. Everything else is asked exactly as before:

- the boundary still asks **where the click will land**: the window alive, the application in
  scope, the window not replaced since the plan, and the target leading the desktop. A click
  carries no address; it goes to whatever is in front of the pointer at the instant it is sent;
- and it asks the **geometry again there**, not only at the plan, for the reason every other term
  is asked twice: a person authorising a press is an unbounded pause in the middle of the one
  sequence that must not have one, and a window can be resized inside it;

- a control recorded as a band across the window is **no longer a problem here, and that is a
  change**. `Locates` is the rule that a MATCH running edge to edge across the window names
  nowhere, and it refuses four of the nine controls in the Rocket League artifact. It is a
  statement about MATCHING, and this mode matches nothing: the point is the click, which is
  already a point and does not become less of one because the proposer drew a band around it.
  Refusing on that basis would discard the person's own evidence in favour of the segmenter's.
  The rule is untouched on the recognition path, where it is the only thing standing between a
  window-spanning match and a press in the middle of nowhere;
- **words that put the control somewhere else still contradict**, in the same one direction as
  everywhere else: text may refuse a match and may never propose one. A menu that has scrolled
  still has all its words, in the wrong places, and that is visible without recognising the
  screen at all. Missing words are *not* a contradiction — zero readable words is the whole Ark
  case, and a route that refused on silence would refuse exactly the demonstrations it exists
  for;
- the **clarification** question, the **step confirmation** and the **authorisation** are
  untouched;
- **verification is untouched.** Finding the point proves nothing about the press. Localization
  and outcome stay separate, which is what makes the trial this enables decisive rather than
  circular.

## It says what it rests on, in its own words

The one thing [[Experiment-044-two-live-runs-that-pressed-nothing-and-pressed-grass]] proved costs a person their trust is a weaker kind of evidence
printed in a stronger kind's sentence: `re-acquired on this look`, over a patch of pitch.

So this route has its own vocabulary and does not borrow:

| | says |
| --- | --- |
| `workflow.SupportDemonstratedLayout` | not `geometric`, which means *on a screen whose arrangement matches* — and no arrangement matched |
| `GroundingDemonstratedLayout` | "where the demonstration left it, in a window of the demonstrated size" — in words, *nobody looked* |
| `StepLayoutChanged` | names both sizes |
| `StepLayoutUnrecorded` | names what would fix it |
| the run's banner | `FIXED LAYOUT: …` beside `LIVE`, naming the window it was demonstrated in |
| the authorisation line | "the point somebody clicked when they demonstrated this", never "the middle of a region" |
| Studio's own choice | offered only for an Act whose demonstration recorded both, and off unless ticked |

A corroborating reading does NOT upgrade the grounding. The screen still was not established;
`Identity` carries what the words made of it, separately from what placed it.

## What the recorder now keeps

Two things, and neither can be added to a recording afterwards — which is why a routine recorded
before them is refused on this route rather than guessed at.

**The point of every press**, taken from the press itself. See the first of the two corrections
above for why it is not the middle of the proposal.

**The window every look was taken in**, kept **outside the `--previews` licence** because a size is
not a picture of anybody's screen — a demonstration recorded with no pictures still has to be
replayable.

A demonstration whose window was **resized partway records nothing**, and says so while the
person is still there. It is not a demonstration of a fixed layout, and choosing the first size
or the last would be the recorder deciding which half of it they meant.

## And it is offered in Studio, never taken for anybody

An Act's listing says whether its demonstration can support this — both the window and every
press point — and a tick box beside Play is where the person takes it. Hidden entirely for an Act
that cannot, because what is missing is in the RECORDING and no choice on a page repairs it; the
sentence then names the fresh demonstration that would.

It is refused in `Play`, not at the first press, for the same reason: a person who asked for it
and would have watched a minute of refusals is told now, in the words that say what would fix it.
And the page is not where the exception is enforced — the backend refuses the combination
regardless of what any page sends.

## What this does not settle

**Whether it works.** This is the mechanism for the deciding trial, not the trial. The remaining
proof is replay: reset, Play, the correct coordinate click, the actual outcome, repeated under
unchanged geometry. A green suite settles none of it.

**It does not make screen recognition unnecessary**, and it is not a reason to stop. It is what
lets the MVP proceed while the fold over live 3D remains unsolved, and
[[ADR-247-what-a-click-is-evidence-of]]'s calibration — which could one day answer the same
question from the pixels — stays deferred, with its open timing limitation.

**Nothing reads control evidence.** This route rests on the artifact's own saved readings, and
not one byte of the calibration. That separation is deliberate: the calibration is deferred, and
a route that quietly depended on it would have made it a prerequisite again.

## Enforced by

- `internal/director/workflowstore/layoutreplay_test.go` —
  `TestWithoutTheLicenceAnUnestablishedScreenStillRefuses` (first, because it is the claim the
  rest of the subsystem depends on); `TestTheReportSaysNothingWasRecognised` (the grounding, the
  support and the sentence all say the screen was never established);
  `TestAWindowOfAnotherSizeIsRefused`, which also holds that **one pixel is a different window**,
  so a tolerance cannot be added to make a failing trial pass;
  `TestADifferentDisplayScaleIsRefusedAndAnUnknownOneIsNotADifference`;
  `TestAnArtifactWithNoRecordedWindowIsRefused`;
  `TestWordsThatPutTheControlSomewhereElseStillRefuse` (and a look nobody could read a word on is
  not a contradiction, which is the case this route exists for);
  `TestTheLicenceSkipsNeitherAgreementNorAuthorisation`.
- `cmd/director/demonstratedwindow_test.go` — `TestADemonstrationRecordsTheWindowItWasMadeIn`,
  `TestADemonstrationAcrossTwoWindowSizesRecordsNeither` and `TestALookWithNoWindowIsNotAResize`,
  all three driven through the production `visualResult.fold`; and
  `TestTheRecorderWritesDownTheWindowItWasDemonstratedIn`, a SOURCE guard that pins both the
  write and the branch that reaches it, and that it sits outside the pictures licence.
- `cmd/director/performworkflowwiring_test.go` — `TestPerformWorkflowInstallsItsGates` gained
  `WithFixedLayout(*fixedLayout)`, pinned as the flag and not a literal: with a literal the
  exception is granted to every run that never asked for it, and without the call `--fixed-layout`
  is a flag that changes nothing.
- `internal/director/workflow/demonstratedpoint_test.go` —
  `TestAStepRecordsThePointThePersonClicked` (and it is NOT the middle of the proposal, asserted
  so the fixture cannot stop posing its question); `TestAClickWithNoProposalStillRecordsItsPoint`;
  `TestEachStepKeepsItsOwnClick` (readings merge across the steps that use one control and clicks
  do not); `TestAStepThatWasNotAPressHasNoClick`.
- `internal/director/workflow/panelpoint_test.go` — the other loop that builds a step, driven
  through the same `Learn`: `TestAPanelPressRecordsThePointThePersonClicked` (and the fixture
  asserts the step's two ends ARE the same screen, so it cannot stop posing its question);
  `TestEachPanelPressKeepsItsOwnClick`, which is the owner's GRAPHICS tab pressed twice;
  `TestAPanelPressWithNoReadingsStillRecordsItsPoint`; and
  `TestAnInertPressWithNoVerifiedOutcomeIsStillNotAStep`, the guard on the other side — carrying
  a point must not have turned a click that did nothing observable into a step, and most clicks
  are that.
- `internal/director/workflowstore/panellayout_test.go` — the same press through the real
  `StepReadinessAt`: `TestAPanelPressIsReplayedAtItsDemonstratedPoint`, at the owner's own
  842,274 inside a tab the proposer read as 781,253 to 933,296, so the click is left of that
  rectangle's middle; `TestAPanelPressInAnotherWindowIsRefused`, asserted on this step
  specifically because its screens are equal at both ends and an implementation that reached the
  geometry check through a transition's arrival logic could pass every transition test and skip
  it entirely here; and `TestAPanelPressWithNoClickIsRefusedRatherThanCentred`, where the tab's
  middle is available and is the wrong point.
- `internal/director/workflowrun/unrecognisedlayout_test.go` — the outermost layer, against a
  grounder that can never name what it is looking at:
  `TestALicensedRunPressesThroughAnUnrecognisableScreen` (and the grounder is asserted to have
  been CALLED, because the licence suppresses the refusal and not the attempt to recognise);
  `TestWithoutTheLicenceAnUnrecognisableScreenStillSendsNothing`, which pins **this** gate by its
  own sentence and the recogniser's measured number rather than by the outcome — measured, by
  deleting the gate outright: the step plan refuses an unestablished screen too, so an
  outcome-and-stage assertion alone passed with the gate gone;
  `TestALicensedRunWithAnUnreadablePanelStillSendsNothing`, the verification half the licence does
  not waive; and `TestALicensedPressIsVerifiedByTheWordsThatAppeared`, which is the whole trial in
  one test — a screen nothing can name, a press at the demonstrated point, a verdict from the
  words that appeared under the tab.
- `plugins/studio/onlayout_test.go` also holds `TestTheJournalSaysWhetherTheLayoutWasGranted`,
  driven both ways through the real spawn, and `TestTheTrialNoteShapeMovesWithItsVersion` forced
  the journal to version 3 rather than letting the field arrive silently — `omitempty` makes a
  writer that never heard of a field indistinguishable from one whose answer was no.
- `internal/director/workflowstore/layoutreplay_test.go` also holds
  `TestThePointPressedIsTheClickAndNotTheMiddleOfTheProposal`, where the proposal's centre lands
  in a DIFFERENT control; `TestAClickWithNoProposalAtAllIsStillPressable`;
  `TestAStepWithNoRecordedClickIsRefusedRatherThanGivenOne`; and
  `TestTheWindowIsCheckedEvenWhenTheScreenIsEstablished`, which is the mode-not-a-fallback
  claim — the screen established, the control re-acquired, and the run still refused on the
  window.
- `cmd/director/geometryboundary_test.go` —
  `TestGeometryReplayPressesWithNoReadingAndNoProposal` (and the same world with the mode off
  still refuses, so this added a second boundary rather than weakening the first);
  `TestAResizedWindowRefusesAtThePressBoundary`;
  `TestGeometryReplayStillAsksWhereTheClickWillLand` (four cases);
  `TestTheBoundaryIsNotPutInAModeItHasNoWindowFor`; and
  `TestGeometryReplayTakesNoLookAtThePressBoundary`, which counts the looks and requires ZERO in
  the mode and exactly one on the vision path. That one is live and skips where no window can be
  acquired, as the boundary tests beside it do.
- `cmd/director/teachingevaluation_test.go` — the no-coordinate guard was NARROWED rather than
  deleted, and now asserts the boundary instead of the absence: the demonstrated press is in the
  artifact, the MISTAKE — a click on bare background that never became a step — is not, and
  neither is the tally of presses like it.
- `plugins/studio/play_test.go` — the page guard pins `acts.Play(act, live, onLayout, seen)`, so
  the layout choice has to leave the page with the press.
- `plugins/studio/onlayout_test.go` — `TestAnActOverAFullDemonstrationIsOfferedItsLayout` (and
  the sentence names the window AND says another size is refused, because an offer that named
  only what it would do would read as a guarantee);
  `TestAnActOverAnOlderDemonstrationSaysWhatWouldFixIt`;
  `TestAWindowWithoutAClickIsRefusedByTheInteractionsName` (the two halves fail separately and a
  person told the wrong one looks in the wrong place);
  `TestPlayRefusesTheLayoutWhenTheDemonstrationCannotSupportIt`, which also holds that the
  refusal comes BEFORE the application is raised — raising pulls a game over whatever the person
  was looking at, so an impossible request must cost them nothing; and
  `TestTheLayoutChoiceReachesTheDirector`, driven through the real spawn in both directions,
  because the whole Studio half is a decoration unless the flag arrives at the child and a build
  that passed it regardless would make the narrow exception the default.
- Seven more for the Studio half, all killed: the layout offered to every Act; a missing click not
  noticed; Play no longer refusing the impossible choice; the game raised before that refusal
  again; the flag never appended; the flag appended on every run; and the choice dropped between
  `Play` and `start`.
- Ten more mutations for the two corrections, all killed: the point taken from the proposal's
  centre again; a point invented for a step that recorded none; the mode turned back into a
  fallback; readings required again; the geometry dropped from the press boundary; the boundary's
  frontness term dropped; a look taken in the mode after all; the mode entered with no window to
  check; `Learn` no longer recording the click; and `Learn` computing it from the proposal.
- Thirteen earlier mutations, all killed: the licence ignored so the route is always taken; the
  window never compared; an unrecorded window waved through; the contradiction check skipped; the
  grounding and the support each left as `geometric`; an unknown display scale treated as a
  difference; a 64-pixel tolerance on the window size; the recorder's write made unreachable; the
  resize branch replaced by keeping the last size; and a look with no window counted as a resize.
  Two of that set no longer apply and are recorded as having gone rather than quietly dropped:
  `Locates` skipped on this route, and the route preferred over a screen that WAS established —
  the first because the point is a click and not a match, the second because this is a mode and
  not a fallback. Both of those claims changed, and the tests that held them were deleted with
  them.

## Related

[[ADR-247-what-a-click-is-evidence-of]] is the calibration this deliberately does not use.
[[ADR-246-a-trial-collects-its-own-evidence]] is how the three trials that died at
`StepScreenUnestablished` were read. [[ADR-243-a-door-of-its-own-and-the-order-in-the-file]] is
the Act this runs.
