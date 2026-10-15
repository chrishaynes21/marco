---
type: experiment
status: complete
date: 2026-10-15
backend:
  - structure
  - ocr
  - vision
game: rocketleague
fixture: "a bounded live run that sent nothing, then the rl-freeplay-1 recording re-learned and driven back through the production runner under -tags corpus"
result: one-PLAY-identity-instead-of-two-and-a-text-match-that-must-agree-with-where-the-control-was-demonstrated-both-steps-of-the-prefix-now-reach-their-press-offline-through-the-real-runner
supersedes: []
affects:
  - passive-observation
  - execution
---

# Experiment-050 — two readings of one button

**Status: the live run RAN and sent nothing — 0 of 2 allowed actions, both refusals correct.
Everything after it is offline, on the recording.**

## The run that pressed nothing

```
step 0: control_1 … refused for 60 observation(s)
  matched: 1 proposal agreeing with a reading of this control; the session could not
  tell this identity from another on the same screen, so this matches a pair
  ↓ re-grounder asked
  matching against the text the demonstration read on this control ("play");
  227 region(s) are candidates, 16 carry text, 3 of those are worded in it
  3 candidates match equally and nothing here chooses between them
```

61 of 63 looks usable, the main menu held one identity throughout. **Two independent ambiguity
refusals, both correct, and two different defects behind them.**

## One: the detector split PLAY into two controls

```
  control_1   label play   x  26..192 y 468..508 (166x40)   used by step 0
  control_10  label play   x  59..367 y 469..513 (308x44)   used by step 20
  "this session could not tell control_1 from [control_10] on screen_a:
   they led to the same place"
```

`sameControl` is mutual centre containment. control_10's centre (213,491) lies outside
control_1, so the two readings are not the same control by that test and two identities were
minted for one button. `MaybeOneControl` paired them — correctly, since a shared destination is
not proof — and re-acquisition will not act on a pair.

The proposer-only artifact does not have this problem and is the one where `control_2` has no
label. **Neither artifact could do both steps.**

### What the demonstration actually says

Every pointer press in the recording, tested against both rectangles:

```
  look  73  at ( 125, 480)   in control_1: true   in control_10: true
  look 539  at ( 159, 499)   in control_1: true   in control_10: true
```

**Both presses lie inside both rectangles.** The person pressed one button twice and two sources
drew it differently. That is evidence the rectangles cannot supply about themselves, and it is
not a shared name or a shared destination — the two things the owner ruled out.

So `oneControlTwice`: the existing control's rectangle contains the press being folded now, AND
the rectangle this press landed in contains every press that control already has. Both must
locate, or a band across the window would swallow every control on its screen.

### And the merged identity keeps ONE rectangle

Keeping both was tried first and measured:

```
  control_1 on visual_1: ambiguous — 2 proposal(s) support this control but describe
  2 different ones; nothing here chooses between them        … on every look of the screen
```

The same defect one tier down. `sameControl` is how a stored rectangle is matched, so two
readings that fail it against each other make every re-acquisition ambiguous — storing both is
storing a contradiction. The identity therefore keeps the **tightest rectangle that still covers
every press**: the most specific claim the evidence supports, and 166x40 covers both presses.

```
  before   14 controls, 5 uncertain steps, step 0 uncertain
  after    13 controls, 3 uncertain steps, step 0 settled, one PLAY reading of 166x40
```

The three that stay uncertain are elsewhere in the walk and are genuine: nothing in the
demonstration resolves them.

## Two: "play" is also what the news panel says

The saved label is one word, and the demonstrated-direction rule is `wanted ⊆ said` — so any
region whose text contains "play" matches. The preflight capture shows the panel had rotated to
DOUBLE DOWN, reading *"Play 10 Competitive 2v2 Doubles Matches"*.

This was visible offline before the run and was under-weighted: `control_1 [play]` refused on one
look in five for exactly this reason, reported as "one ambiguous offline look". Live, with a
different panel, it was every look.

**The demonstration knows more than the word.** It recorded where the control was, and
`surfaceFor` already asks the only question that settles it — is there a candidate around this
text that agrees with a saved reading of this control. A news banner does not agree with a 166x40
menu row. So a text match must be corroborated by the demonstrated geometry.

**Only where there is geometry to ask with.** Four of nine controls in the measured artifact are
bands across the window that locate nothing; ADR-193 exists for exactly those, and holding them
to a rectangle that names nowhere would refuse every repair it was built to make. The
corroboration is required where the demonstration left a usable rectangle, skipped where it did
not, and the report says which.

## Driven back through the runner

The offline check before the live run exercised the label matcher alone and said both controls
resolved. It missed both defects because neither is in the matcher. So the validation now drives
what the live loop drives, in the order it drives it — `fastobserve`, `reacquireStored`,
`promptGrounder`, `regrounder`, `workflowrun.Runner` — over recorded frames, with a notebook
where the mouse would be:

```
  main menu -> Play
    look 2  waiting   control not re-acquired
    look 3  PRESSED   step 0 lowered into legal Marco, compiled and run at (82,485)
            control_1 at 0.036,0.444 — inside the demonstrated PLAY reading

  Play -> Training
    look 81 waiting   what was matched is not the control — "TRAINING" on the same look
                      is what this control is recorded as
    look 82 PRESSED   step 1 lowered into legal Marco, compiled and run at (577,640)
            control_2 at 0.301,0.593 — the demonstrated tile centre
```

**Both steps of the prefix reach their press.** Look 81 is worth keeping: the identity tier
refused a position-only match because the words said otherwise, and the next look pressed the
right thing — ADR-197 doing its job inside the same path.

What a recording cannot validate is what the application does next: the frames are fixed, and
pressing a recorded pixel moves nothing. So each step is driven over the frames of the screen it
starts on, and the assertions are about where the press would land.

## Against the owner's criteria

| | |
|---|---|
| one learned PLAY identity | yes — 13 controls, one `play`, one 166x40 reading |
| zero news-panel selections | yes — the press lands inside the demonstrated reading, and a direct test refuses the banner |
| TRAINING still correctly grounded | yes — `training`, pressed at the tile centre (577,640) |
| the prefix through the actual runner | yes — both steps reach `OutcomePressed` under `-tags corpus` |
| distinct controls still distinguishable | yes — two menu rows, a band, a press outside the control, and the same place on two screens all stay separate |
| ambiguous matches still refuse | yes — three steps remain uncertain, and the band-saved control still refuses |

## What this does not claim

**That the route runs live.** No input has been sent since the refusals. Step 2's control is
still unlearned and is not attempted.

**That the detector is optional.** `control_2`'s saved rectangle IS a detector box, so
re-acquisition needs the detector running — a rig without it cannot re-acquire the tile at all,
which is how the corpus test first failed.

## Related

- [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] — extended here with the identity
  merge and the geometric corroboration.
- [[Experiment-049-the-control-it-could-see-and-could-not-name]] — the run before this one.
- [[ADR-182-two-controls-that-could-never-be-found-again]] — the container/control ambiguity this
  sits beside.
- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — the fallback that must keep
  working for band-saved controls.
- [[ADR-197-a-geometric-match-nominates-and-does-not-identify]] — visible at look 81.
