---
type: decision
status: accepted
date: 2026-10-15
deciders: director, owner
affects:
  - passive-observation
  - execution
source_paths:
  - internal/director/observe/controllabel.go
  - internal/director/observe/clicktarget.go
  - internal/director/observe/identity.go
  - internal/director/observe/reacquire.go
  - internal/director/workflow/workflow.go
  - internal/director/workflowstore/workflowstore.go
  - internal/director/workflowstore/stepplan.go
  - cmd/director/reground.go
  - cmd/perceptionreplay/semantic.go
---

# ADR-201 — a demonstrated click remembers what it clicked

## Context

[[Experiment-049-the-control-it-could-see-and-could-not-name]] ran the demonstrated route against
the live game. Marco pressed PLAY, the screen changed, the Play page was recognised at 0.796 —
and the next step refused:

```
  step 1: control_2 was not found on any look while the run waited for it.
    matching against the workflow's own name ("enter free play");
    7 region(s) carry text and 0 of those are worded in it:
      "TRAINING"   x  402.. 520  y  718.. 750     <- the control it wanted
    nothing on this screen is named in the workflow's own name
```

The word was on the screen, read, and in its own candidate listing. The owner's reading:

> That's not a perception failure. The live perception system found the word. The connection
> between the observed word and the demonstrated control was missing. The refusal was
> appropriate.

`Artifact.ExpectedWords` has preferred saved control text to the workflow's name, with
provenance, since [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]].
**The rule was already there; the field was always empty** — nothing between a press and the file
carried a name, and the cached OCR beside a recording stored each phrase and discarded its
rectangle, so nothing could have filled it.

## Decision

**A demonstrated click records what the thing it clicked says.**

The association is `observe.NameCandidates`, unchanged and shared with the live identity tier: a
word belongs to the smallest *locating* candidate that wholly contains it. Three properties
follow from reusing it rather than inventing a rule, and each was measured:

| | what it does | why |
|---|---|---|
| inside the control's own rectangle | a word must lie wholly within the hypothesis the press established | not the nearest word, which is what the owner ruled out; not the screen's vocabulary |
| a band across the window learns nothing | `Locates` refuses a full-window region, so its words name nothing | `control_3` is saved as 0,0 1.000x0.957; naming it from the words inside would teach that the control is called everything on the screen |
| over the looks of its screen, not the press's | a word is kept when read on at least `TextCommon` of them | OCR read PLAY on seventeen looks of the main menu and **none** of the six before the press: the entry is highlighted by the time the pointer reaches it. The moment of the press is the single worst look to ask |

The last is why `ClickTargets.Reading` is a per-look fold and why `MaxRecentReadings` of them are
held: a press is folded before the rectangle it establishes exists, so the readings that name a
control are usually the ones *before* it was clicked. A look that read nothing inside the
rectangle is not counted at all — the difference between a control nobody could read and a
control that says nothing, one tier down from *missing evidence is not contradictory evidence*.

**And the match direction follows the provenance.** `observe.NamesControl` runs `said ⊆ wanted`
for the workflow's TITLE — unchanged, ADR-193's rule, a control whose label is made of the task's
words — and `wanted ⊆ said` for a DEMONSTRATED label, because a control learned as `training`
must match a later look reading `CUSTOM TRAINING`. One function, both directions, the reasoning
beside them. Neither direction proposes on its own: a caller still refuses when more than one
candidate matches.

**Nothing here moves a target, mints one, or changes which rectangle a press identified.** A
label is evidence about a control that already exists, and the smallest-containing-region rule is
untouched.

**And the word cache keeps rectangles now.** A reading without them is a vocabulary, and a
vocabulary cannot say which control said something.

## Consequences

**Measured on the recording that produced the live run's artifact.** Re-learned with the
recorded detections of [[ADR-192-a-second-source-in-the-live-loop]]:

```
  control_1   play                        x 0.014 y 0.433 0.086x0.037
  control_2   training                    x 0.208 y 0.491 0.185x0.205
  control_3   (nothing read)              x 0.000 y 0.000 1.000x0.957
  control_8   garage       control_9  back orange view     control_7  yes
  control_14  customization options view
```

**TRAINING is recoverable only with the detector running**, and the reason is geometric rather
than preferential: the proposer's tile stops at y 0.654 and the caption sits at y 0.673, while
the detector's box for the same tile covers it and is less than half the area — so it wins the
existing smallest-containing-region rule on its own merits. Without the detector the same fold
learns `play` and leaves `control_2` unnamed.

**FREE PLAY is not recoverable, and is recorded as such.** Its press landed inside no region but
the whole window and no word lies under the click, so the artifact says the control was never
read. Nothing guesses a rectangle for it.

**The existing re-grounder resolves both labelled controls offline**, with no reference to the
workflow title, on most looks of their screens — and refused on a look where two candidates
matched, which is the ambiguity refusal the owner asked to keep.

**What the provenance report now says.** An inferred arrival used to print "on a reading that
only FAILED TO CONTRADICT it", because `Positive` and `Thin` are set only by `Reached`. An
inference has no reading in it at all; it is a screen change the observer recorded and an
arrangement that agreed. It overstated nothing about strength and misnamed the KIND, in the one
report whose job is keeping those apart. Corrected.

**What this does not fix.** Step 2 of the route. Main Menu → Play → Training is the executable
prefix, and the Free Play control needs re-teaching rather than more patience.

## Since — the text identifies, and a demonstrated surface is pressed

The owner asked one question before authorising a live run, and it was the right one:

> the TRAINING regrounder resolves to the caption rectangle rather than the tile's original
> center. That may be perfectly clickable, but the saved click point and the semantic text region
> serve different purposes.

Inspected on nine looks of the Play page, in window pixels:

```
  the SAVED control reading      x  399.. 754 y  530.. 751 (355x221)   centre 576,640
  look  81  identified by text at x  402.. 520 y  718.. 750 (118x 32)  centre 461,734
  look  93  identified by text at x  397.. 757 y  528.. 755 (360x227)  centre 577,642
```

Two findings, and the second is worse than the first. **The caption's centre sits 17 px above
the bottom edge of the tile it labels**, where the demonstrated press was 111 px clear of every
edge. And **it was not stable**: whether the matcher returned the caption or the tile depended on
whether the proposer happened to box the caption separately on that look, so the press point
moved between the tile's middle and its lower lip from one observation to the next. A margin of
17 px is inside the jitter already visible in the data — the caption box reads 718..750 on one
look and 722..748 on another, and the tile's own bottom edge moves between 751 and 755.

So the two roles are separated. The text IDENTIFIES; the press goes to `surfaceFor` — the
smallest candidate on this look that contains the identifying text **and agrees with a saved
reading of that control** by `observe.SameControl`, the mutual-centre-containment test
re-acquisition has always used. No new threshold, no second detector, and the demonstrated
relationship preserved rather than guessed at: the demonstration pressed a surface of this shape,
and this is a surface of that shape with the control's name inside it.

```
  after: every resolving look presses 577,640 — the demonstrated centre, 111 px from the
         nearest edge — whether or not the caption was boxed separately
```

**When nothing around the text agrees, the text itself is pressed, and the report says so.** On
the one look where the page was changing, the tile was not proposed at all and the only thing
containing the caption was the caption's own 353x34 row, which is not a reading of a 355x221
tile. Refusing entirely would turn a control Marco correctly identified into one it cannot use;
claiming a surface would be inventing one. The run report now prints both rectangles on separate
lines, because they are not the same claim:

```
    re-grounded: the only control on this screen whose text ("TRAINING") is made of …
      identified by its text at x 402..520 y 718..750
      pressing the surface around it at x 400..754 y 529..751, which agrees with the
      demonstrated control
```

## Enforced by

`internal/director/observe` — `TestAControlIsNamedByTheWordsInsideIt`,
`TestAFullWindowHypothesisLearnsNoName`, `TestALookNobodyReadNamesNothing`,
`TestAControlKeepsTheWordsMostOfItsLooksAgreedOn`,
`TestAControlReadBeforeItWasClickedIsStillNamed`,
`TestAReadingOfAnotherScreenDoesNotNameThisControl`,
`TestWhichWayACandidateHasToBeNamedDependsOnWhereTheWordsCameFrom`.

`internal/director/workflow` — `TestADraftCarriesTheNameOfTheControlThatWasPressed`,
`TestADraftSaysWhenNothingWasReadOnTheControl`.

`internal/director/workflowstore` — `TestASavedControlCarriesWhatTheDemonstrationReadOnIt`.

`cmd/director` — `TestAnAttemptSaysWhatItEstablishedAndThatTheActionIsStillNotVerified`,
`TestAControlIdentifiedByItsCaptionIsPressedOnTheSurfaceAroundIt`,
`TestWithNoAgreeingSurfaceTheTextItselfIsPressed`,
`TestWithNoSavedReadingTheTextItselfIsPressed`,
`TestAFullWindowBandIsNotAnInteractionSurface`,
`TestTheSmallestAgreeingSurfaceIsTheOnePressed`,
`TestRegroundingACaptionHandsBackTheSurfaceToPress`.

Seventeen mutants run and all seventeen killed. For the press surface: the caption pressed with
no surface sought; any containing candidate accepted as a surface; a band across the window
pressed; and the largest agreeing surface taken instead of the smallest. For the naming rules: a band across the window naming a control; one look
being enough; readings before the press discarded; a reading of any screen naming this control;
the match direction ignoring provenance; the draft dropping the name; a control nobody read not
recorded as unread; the artifact dropping the name; a demonstrated label reported as the weak
source; and an inference described as corroborated by a reading.

## Related

- [[Experiment-049-the-control-it-could-see-and-could-not-name]] — the two live runs, the three
  recovery routes, and what each decided.
- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — the preference this fills in,
  and the fallback that stays weaker.
- [[ADR-192-a-second-source-in-the-live-loop]] — the detector whose box makes TRAINING
  recoverable.
- [[ADR-197-a-geometric-match-nominates-and-does-not-identify]] — why a rectangle alone was never
  enough.
- [[ADR-200-a-destination-a-workflow-recognises-for-itself]] — the screen half, which worked on
  the live run this came from.
