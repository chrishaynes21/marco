---
type: experiment
status: complete
date: 2026-10-15
backend:
  - structure
  - ocr
  - vision
game: rocketleague
fixture: "two live bounded runs against Rocket League, then the rl-freeplay-1 recording re-read with word positions and re-learned with and without the recorded detections"
result: screen-recognition-worked-and-the-blocker-is-learned-control-identity-TRAINING-is-recoverable-from-the-existing-demonstration-only-with-the-detector-and-FREE-PLAY-is-not-recoverable-at-all
supersedes: []
affects:
  - passive-observation
  - execution
---

# Experiment-049 — the control it could see, and could not name

**Status: RUN live, 2026-10-15, twice, under the owner's bounded authorisation. One action sent
in total. Then offline on the recording, where nothing was executed.**

## The two live runs

The first never got going. **602 of 607 looks saw another window in front** — launching took the
foreground and the game spent 60 of its 75 seconds waiting. In the three usable looks it did
attempt step 0, and [[ADR-198-a-move-that-reported-completed-and-did-not-happen]]'s guard stopped
it: the pointer was sent to (109,488) and the operating system reported it at (67,488), 42 px
away, so nothing was pressed. **It did not recur**, and the second run put the pointer exactly
where it was sent at the same coordinate. A foreground artefact, not a pointer defect.

The second run, with the regenerated semantic artifact:

```
  step 0: pressed — lowered into legal Marco, compiled and run at (109,488)
    pointer: the operating system puts the pointer at (109,488), where it was sent
    hover:   the control's pixels changed by 170.7/255 over 952 px
  screen_b inferred: ... the screen now agrees with screen_b at 0.796
  step 0: destination established
```

PLAY was re-acquired geometrically, the pointer went where it was sent, the button lit, the click
landed, **the screen changed**, and the destination was established at 0.796. 70 of 72 looks
usable. **Goal reached 1, goal recognised 1.**

Then step 1 refused, and the refusal is the finding:

```
  step 1: control_2 was not found on any look while the run waited for it.
          Looking for it on the screen instead.
    matching against the workflow's own name ("enter free play");
    128 region(s) are candidates, 7 of them carry text and 0 of those are worded in it:
      "QUICK PLAY"     x  284.. 386  y  890.. 924
      "TRAINING"       x  402.. 520  y  718.. 750      <- the control it wanted
      "PLAY OFFLINE"   x  780.. 948  y  720.. 752
    nothing on this screen is named in the workflow's own name
```

**The word was on the screen, read, and in the list.** Nothing connected it to the control,
because the demonstration never recorded that the thing the person clicked said TRAINING. The
owner's reading of it, which is the right one:

> That's not a perception failure. The live perception system found the word. The connection
> between the observed word and the demonstrated control was missing. The refusal was
> appropriate.

[[ADR-200-a-destination-a-workflow-recognises-for-itself]]'s `Reached` path never fired in this
run, correctly: the observer DID record `visual_1 -> visual_2`, so the ordinary inference handled
it. The new rule is for when no change is recorded.

## Why `Control.Text` was empty

Traced end to end:

```
  Observer.Observe(lk)  ->  clicks.Press(look, at, standing, cands)
                              smallest containing candidate is the hypothesis
                              recordTarget                   -> TargetRecord{Where, Containers}
  workflow.Learn(...)   ->  Step{Readings, Containers}
  workflowstore.SaveWith -> Control{Readings, Containers}
```

No text anywhere along it. `Control.Text` has existed since
[[ADR-193-a-control-re-grounded-from-the-workflows-own-name]], and `Artifact.ExpectedWords` has
preferred it to the workflow's name, with provenance, ever since. **The rule was already there
and the field was always empty** — and the cached OCR beside the recording stored each phrase but
threw away its rectangle, so nothing could have filled it.

## What the demonstration actually contains

The recording was re-read keeping each phrase's rectangle (300 readings, 287 with positions), and
every demonstrated press examined against them:

```
  press at look  73 (PLAY)       hypothesis x 0.014..0.100 y 0.433..0.470   locates
  press at look 108 (TRAINING)   hypothesis x 0.000..0.405 y 0.441..0.654   locates
  press at look 138 (FREE PLAY)  hypothesis x 0.000..1.000 y 0.000..0.957   LOCATES NOTHING
  press at look 169              hypothesis x 0.000..1.000 y 0.000..0.957   LOCATES NOTHING
```

and then the three routes by which a name could be recovered:

```
  (a) a word under the click itself, at look 138       NONE
  (b) a word inside the Training tile, any look 78..113 NONE -- the caption sits at
      y 0.673..0.688 and the proposer's tile stops at y 0.654
  (c) a word inside the PLAY rectangle, looks 0..77     PLAY, on 17 looks
```

Three separate findings, and each one decided a piece of the design:

- **PLAY is recoverable, and not from the press's own look.** OCR read PLAY on seventeen looks of
  the main menu and on none of the six before the press — the entry is highlighted by the time
  the pointer reaches it, and highlighted text renders differently. **The moment of the press is
  the single worst look to ask.**
- **TRAINING is not recoverable by containment in the proposer's rectangle.** The tile's caption
  is below the tile. Nothing inside the demonstrated region ever says what it is.
- **FREE PLAY is not recoverable at all.** The press landed where the only region containing it
  was the whole window, and no word lies under the click.

## The one route that recovers TRAINING

The learned detector of [[ADR-192-a-second-source-in-the-live-loop]] runs beside the proposer
live, and its detections for this recording were already on disk. Asked what contains the
Training press:

```
  look 108, press at 482,602
    image  0.42  x 397..757 y 527..753   (y 0.488..0.697)   covers the caption: YES
```

**The detector's box includes the caption and is less than half the area of the proposer's
tile**, so it wins the smallest-containing-region rule on its own merits — no preference for
being semantic, which is the rule ADR-192 settled. Re-learning with the recorded detections:

```
  control_1   play                       x 0.014 y 0.433 0.086x0.037
  control_2   training                   x 0.208 y 0.491 0.185x0.205
  control_3   (nothing read)             x 0.000 y 0.000 1.000x0.957
  control_8   garage                     control_9  back orange view
  control_14  customization options view control_7  yes
```

Without the detector, the same fold learns `control_1 play` and leaves `control_2` unnamed.

## Does the existing re-grounder resolve them offline

The production rule — `NameCandidates` to label every candidate on the look, then match against
what the artifact says the control is — run over looks of each screen, with no reference to the
workflow's title:

```
  control_1, wanted [play], from the text the demonstration read on this control
    look   0  RESOLVED at x 0.000 y 0.424 0.072x0.041  "PLAY"
    look  12  RESOLVED        look 24  RESOLVED        look 36  RESOLVED
    look  48  REFUSED (2 candidates match, nothing chooses between them)

  control_2, wanted [training], from the text the demonstration read on this control
    look  81  RESOLVED at x 0.209 y 0.665 0.061x0.030  "TRAINING"
    look  90  RESOLVED        look 111  RESOLVED
    look  99  REFUSED (nothing named)   look 108  REFUSED (nothing named)

  control_3, wanted [enter free play], from the workflow's own name
    looks 117..141   REFUSED on every one
```

**Yes, and the ambiguity refusal is intact** — look 48 had two candidates matching and the rule
declined rather than choosing. Look 108 is the press look again, where OCR read nothing named;
the same frame that defeated a press-time rule.

One thing worth stating plainly: `control_2` resolves to the **caption's** rectangle, not the
tile's. Its centre is inside the detector's box for the tile and below the proposer's, so the
press would land on the lower part of the tile rather than its middle.

## The match direction had to follow the provenance

`Covered(said, wanted)` — every word of a candidate's label is in the wanted set — is right for
the workflow's TITLE and wrong for a demonstrated label. A control learned as `training` must
match a later look reading `CUSTOM TRAINING`; under the title rule that is a miss because
`custom` is not in the saved label. So `NamesControl` runs `wanted ⊆ said` for a demonstrated
label and the original direction for the title, and the two are one function with the reasoning
written beside it.

## Answers

1. **Is TRAINING now learned as `Control.Text` for step 1?** Yes — `control_2 training` — **when
   the learning fold runs with the detector**. Without it the proposer's tile excludes the
   caption and nothing is learned. `control_1 play` is learned either way.
2. **Can FREE PLAY be learned for step 2?** **No.** The press landed inside no region but the
   whole window, and no word lies under the click. The demonstration does not contain the
   evidence, and nothing here guesses a rectangle for it — the artifact records that the control
   was never read, so a reader can tell "unlabelled" from "not looked at".
3. **Do the saved targets remain spatially associated with their demonstrated clicks?** Yes.
   Every saved reading still contains its own press: `control_1` (0.065,0.444) in
   x 0.014..0.100 y 0.433..0.470; `control_2` (0.251,0.557) in x 0.208..0.393 y 0.491..0.696;
   `control_4` (0.249,0.512) in x 0.241..0.315 y 0.428..0.526. The hypothesis moved from the
   proposer's rectangle to the detector's where the detector offered a smaller one, which is the
   existing rule and not a new preference.
4. **Can the re-grounder resolve those controls offline without the workflow title?** Yes, for
   both labelled controls, on most looks of their screens, with the ambiguity refusal preserved.

## What this does not claim

**That the route now runs.** Step 2's control is still unrecoverable, so Main Menu → Play →
Training is the executable prefix. No live run was made after these changes.

**That one recording is a corpus.** Thirteen controls of one demonstration, four of them pressed
on the route under test.

## Related

- [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] — the rule, and what may become a
  name.
- [[Experiment-046-the-destination-it-reached-and-did-not-recognise]] and
  [[Experiment-048-the-destination-a-workflow-can-see-for-itself]] — the screen-recognition half,
  which worked here.
- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — the preference that was waiting
  for this data.
- [[ADR-192-a-second-source-in-the-live-loop]] — the detector whose box is what makes TRAINING
  recoverable.
- [[ADR-197-a-geometric-match-nominates-and-does-not-identify]] — why a rectangle alone was never
  going to be enough.
