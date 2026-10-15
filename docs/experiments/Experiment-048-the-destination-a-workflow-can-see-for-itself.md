---
type: experiment
status: complete
date: 2026-10-15
backend:
  - structure
  - ocr
game: rocketleague
fixture: "freeplay.workflow.json and the same recording re-saved through SaveWith, scored against rl-freeplay-1's four pages, 222 folds of the recorded navigation, and the held-out rl-playtrain-1 capture"
result: the-evidence-to-recognise-the-expected-destination-is-present-on-the-first-look-of-it-and-the-run-never-asks-because-the-question-is-gated-on-the-global-observer-minting-an-identity-and-scored-against-the-standing-screens-stored-signature
supersedes: []
affects:
  - passive-observation
  - execution
---

# Experiment-048 — the destination a workflow can see for itself

**Status: RUN offline, 2026-10-15. Read-only on every production contract. Nothing was
executed, nothing authorised, no production file changed. An investigation, as asked.**

## The question

> Investigate whether Director can recognize an expected workflow destination directly from
> current observations, without requiring the global observer to mint a new durable screen
> identity first. […] Keep global screen learning conservative. Do not let workflow expectations
> rewrite observed screen identity.

[[Experiment-047-what-one-reading-is-allowed-to-do]] closed the merge-rule line: the authorised
single-reading veto separated six more folds of 222 and left the live four-look dwell unchanged,
and the variant that moved it minted a second main-menu identity. The owner's reading of that is
the premise here — **one mechanism is being asked to satisfy two requirements** — so this asks
whether the workflow executor can answer its own question from evidence the global model is
right to be slow about.

## What the live run actually did, read again

[[Experiment-046-the-destination-it-reached-and-did-not-recognise]], verbatim:

```
  step 0: destination established — screen_b inferred as visual_2, arrangement agreed 0.662
  step 1: refused — screen_c not established within 60 observation(s); the screen
          standing is visual_2
```

Step 0 **worked**: the Play page was recognised as `screen_b` within the four looks it was held
for. The failure is step 1, and it is not a four-look failure at all — Marco was standing on the
Training page, the page it needed, for **sixty observations**, and never established it.

So the thing to explain is not a page that vanished too quickly. It is a page that was there the
whole time and was never asked about.

## Two gates, and what each is handed

Tracing `promptGrounder.Establish` in `cmd/director/performworkflow.go` and
`SessionBinding.Infer` in `internal/director/workflowstore/anchor.go`:

```
  1. Establish returns EARLY when the step's SOURCE label is still established in this
     observation:
         if _, ok := s.Established(p.standing, at.Epoch, at.Generation); ok { return nil }
     A binding is live for exactly one observation generation, and the epoch moves only when
     the GLOBAL observer records a screen change. The Play and Training pages merged, so no
     change was recorded, so screen_b stayed live, so the question "is this screen_c?" was
     NEVER PUT -- for sixty observations.

  2. and when it is put, what it scores is not this look:
         ground.arrangement = obs.Arrangement(up.Screen)
     obs.Arrangement(id) is the screen's STORED signature, fixed at `promote` and never
     updated afterwards. Standing on the Training page under visual_2, the saved destination
     is compared against the PLAY PAGE's own arrangement.
```

Both gates are the global model. The first decides whether the workflow may ask; the second
decides what it may look at. Neither consults the pixels in front of the planner, although
`lk.Regions` is in hand on the same line and `observe.ArrangeLook` is the public function that
turns it into an arrangement.

**This is the conflation the owner named, located in two lines.**

## The evidence, scored look-local

Two artifacts, both written by production code from the same recording:

```
  freeplay.workflow.json             what the live run carried: 9 screens, arrangement only,
                                     NO `text` key on any screen
  freeplay-with-text.workflow.json   the same recording through
                                     perceptionreplay -frames -workflow -semantic -save,
                                     which reaches SaveWith: 13 screens, 11 of them with words
                                       screen_a  main menu       challenges club competitive ... (19)
                                       screen_b  Play page       match offline play private
                                       screen_c  Training page   aerial goalie striker
                                       screen_d  Free Play menu  create free join online play
```

Every look of all four pages, scored with `ArrangementSimilarity(saved, ArrangeLook(regions))`
and, where the recording carries a reading, `TextsDisagree(saved.Said(), reading)`. Nothing is
tuned; `VisualScreenMatch` is 0.55 as it has been.

### The step that failed: screen_b → screen_c

```
                            vs screen_c                      vs screen_b
  ON the Training page      0.600 (0.550..0.686)  28 of 28   0.448   0 of 28
  on the Play page          0.480 (0.379..0.703)  10 of 34   0.729  32 of 34
  on the main menu          0.295                  0 of 28   0.338   0 of 28
  on the Free Play submenu  0.570 (0.483..0.634)  21 of 28   0.418   0 of 28

  the readings
  ON the Training page      10 reading(s)   agreed with screen_c 10, disagreed  0
  on the Play page          12 reading(s)   agreed  0, disagreed 12
  on the main menu          10 reading(s)   agreed  0, disagreed 10
  on the Free Play submenu  10 reading(s)   agreed  0, disagreed 10

  would establish screen_c:      arrangement   + above the source   + a reading agreeing
  ON the Training page   TRUE          28            28               10 of 10 readings
  on the Play page       false         10             2                0
  on the main menu       false          0             0                0
  on the Free Play menu  false         21            21                0
```

**The destination clears the existing threshold on every one of the 28 looks of it, and on none
of the 28 looks of the main menu.** It is not a near miss and no threshold moved: the saved
Training page agrees with a Training look at 0.600 and with a Play look at 0.480.

And the arrangement alone is **not safe**: it would establish the Training page on 10 of 34 Play
looks and on 21 of 28 submenu looks. Every one of those is killed by a reading, and no reading of
a Play or submenu look ever agreed with the Training page's words.

### The step that worked: screen_a → screen_b — and it is the mirror image

```
                        vs screen_b                 readings
  ON the Play page      0.729  32 of 34   above screen_a 34 of 34   agreed 12 of 12
  on the main menu      0.338   0 of 28   above screen_a  0 of 28   AGREED 8 of 10
```

On the main menu, a reading **agrees** with the Play page 8 times in 10 — because `screen_b`'s
whole saved vocabulary is four words, `match offline play private`, and the main menu says all
of them. Here the arrangement separates cleanly (0.729 against 0.338) and the words do not.

**The two halves fail in opposite places, which is the argument for requiring both.** It is also
why one tempting extra term must be rejected: requiring the reading to *contradict the source*
works perfectly on screen_b → screen_c (10 of 10) and destroys screen_a → screen_b, where it
drops arrivals from 11 of 12 readings to 1.

### The step after: screen_c → screen_d, and the only false arrival in the set

```
                        vs screen_d                      would establish screen_d
  ON the Free Play menu 0.822 (0.479..1.000) 27 of 28     27 arr   27 +margin   9 of 10 readings
  on the Training page  0.768 (0.709..0.836) 28 of 28     28 arr   28 +margin   1 of 10 readings
```

The Training page matches the submenu's saved arrangement on **all 28 looks** — the submenu
replaces one panel of it. Only the words refuse it, and they refuse 9 of the 10 readings. The
one that does not is **look 141**, and it is honest:

```
  look 141 vs screen_d  apart=false  kept 0.40  novel 0.88
            read: aerial create custom field free goalie hit invited party pla players
                  practice soiardlbicn striker the training
```

Look 141 is the transition frame. `FREE PLA…` and `CREATE` are already on the glass, because the
submenu is appearing. Calling that an arrival one look early is not a wrong answer.

### Held out: a separate capture, never learned from

`rl-playtrain-1` is 400 frames of the Training page, a different capture session, no inputs, and
nothing was ever learned from it. Read at stride 10 (40 readings, newly OCR'd for this):

```
  vs screen_c   0.608 (0.459..0.660)   cleared 0.55 in 39 of 40   above screen_b in 40 of 40
  vs screen_b   0.450 (0.412..0.500)   cleared 0.55 in  0 of 40
  40 reading(s): agreed with screen_c 40, disagreed 0; disagreed with screen_b 40
  would establish screen_c: 39 of 40
```

**39 of 40, on pixels and words the artifact has never seen.** The margin is not an artifact of
measuring a recording against itself.

## The same 222 folds

The grid of [[Experiment-047-what-one-reading-is-allowed-to-do]], unchanged — which look becomes
the Play page's signature × how long it is held × reading density × arrival latency — folded
through `fastobserve` on the live late-reading path. **69 of the 222 folds merge the Training
page onto the Play page's identity**, which is the live failure, reproduced.

In those 69 folds, the destination `screen_c` would be established by:

```
  the comparison the code makes today                        0 of 69
  the same comparison, look-local                           69 of 69
  look-local AND a reading not refusing it                  69 of 69
  look-local AND that reading also refusing the page
      it came from                                          69 of 69
```

and the false arrivals — looks on the **Play page**, where the Training tile has not been
clicked and anything that establishes `screen_c` is wrong — summed over all 222 folds:

```
  the arrangement term today's rule uses                   288 look(s)
  the same comparison, look-local                          624 look(s)
  look-local AND a reading not refusing it                   0 look(s)
```

Today's rule contributes no false arrival here, and the reason is worth stating exactly: the
epoch gate blocks it. **The same gate produces the 0 of 69.** It is not discriminating between a
true arrival and a false one; it is declining to answer, and the 288 says the evidence it would
have used could not have told the difference anyway.

The words are what replace that gate as the defence, and they replace it completely: **zero false
arrivals in 222 folds.**

### How long it waits

```
  look-local arrangement alone        mean 0.00 look(s), worst 0
  with a reading corroborating it     mean 5.33 look(s), worst 8
```

The arrangement is there on the **first** look of the destination. The corroborating reading
takes five looks on average and eight at worst — about 3 to 5 seconds at the live cadence of
~640ms a look, against a step patience of 60 observations. The four-look dwell does not bind
here: the destination is the page the workflow needs to act on, so it is the page Marco stays on.

### And one thing the merge breaks that is worth naming

```
  readings taken before the click that the epoch rule admitted anyway, because the
  merge kept the epoch still: 130
```

[[ADR-195-a-reading-that-arrives-after-the-look-it-describes]] refuses a reading whose epoch has
moved. When the global observer records no change, the epoch does not move, so a reading of the
page being LEFT is admitted after the click. The direction of that error is safe for an arrival
test — a reading of the Play page disagrees with the Training page and withholds the arrival —
but it is the reason the corroboration has to be **required** rather than consulted when
convenient: without it, the arrangement alone fires on the first look, and 624 of those were
wrong.

## What evidence is missing

One thing, and it is not a mechanism:

**The artifact the live run carried has no `text` key on any screen.** `Screen.Text` exists,
`Screen.Said()` reads it, and `workflowstore.SaveWith` writes it — but only
`cmd/perceptionreplay`'s `-save` path calls `SaveWith`. The other three save paths call `Save`,
which passes `nil`, and the live artifact came from one of those. The semantic half of
destination evidence is one call away from being saved, and the 13-screen artifact above is what
the same recording produces when it is.

Two smaller notes, recorded rather than fixed:

- `TextsDisagree` needs at least `TextDisagreementWords` (3) words on **both** sides. `screen_c`
  has exactly three. A destination with fewer than three saved words cannot refuse anything, so
  the corroboration term would silently degrade to "a reading exists" for it. Two of the 13
  saved screens — the two at 400 regions — carry no words at all.
- `Control.Text` is still empty in every artifact, which is unchanged and is the next learning
  improvement the owner named.

## Answers

1. **When the global observer merges Play and Training, how strongly does the current
   observation match the workflow's expected Training destination?** 0.600 mean over the 28
   recorded looks of it (0.550..0.686), against 0.55 — above it on every look — while the saved
   Play page scores 0.448 and clears on none. 0.608 on a held-out capture. The evidence is
   strong, present on the first look, and the run is not looking at it.
2. **Can the saved destination's arrangement and semantic evidence distinguish Training from
   Play using the observations actually available at the live four-look dwell?** Yes, and it
   needs both. Arrangement separates Training from Play (0.600 / 0.448) but not Training from
   the Free Play submenu (21 of 28 false). Words separate every pair tested here but not the
   main menu from the Play page, whose four saved words the menu also says. The dwell is not the
   constraint: the arrangement answers on look 0 and a reading corroborates by look 5 on
   average, 8 at worst, on a page the step then waits on for up to 60 observations.
3. **Can Director establish arrival from positive destination evidence even if the global screen
   identity remains unchanged?** Yes — 69 of 69 of the merged folds, where today's path manages
   0 of 69. And the separation is structural rather than promised: `internal/director/observe`
   does not import `workflowstore`, so a workflow binding cannot reach the global screen model
   even by accident. `SessionBinding` has no JSON tags and is not a field of `Artifact`, so the
   binding dies with the session.
4. **What prevents false arrival when the source and destination are visually similar?** Three
   things measured, in increasing strength: the destination must clear 0.55 on **this look's own
   arrangement**; it must score **above the saved source** (which alone cuts the Play-page false
   arrivals from 10 of 34 to 2); and an **admissible reading of this look must not contradict
   the saved destination's words** (which takes every false arrival in the set to zero, 0 of 222
   folds). What must NOT be used is the fact that an action was sent — nothing above consults it,
   and the one remaining false positive in the whole measurement is a transition frame on which
   the destination was genuinely appearing.
5. **Does the same approach preserve main-menu revisits and the Training → Free Play
   distinction?** It cannot affect either: it mints no global identity, writes no arrangement,
   and folds no text, so the 449-look single main-menu identity and the Training/Free Play
   separation of Experiment-047 are untouched by construction. Measured anyway on the pages
   themselves: no main-menu look would establish the Play page or the Training page (0 of 28,
   either), and the Training → Free Play step establishes the submenu on 27 of 28 submenu looks
   while the words hold the reverse direction down to the single transition frame at look 141.

## The smallest implementation this supports

Not implemented. Proposed, for the owner to accept or refuse:

```
  1. ASK THE QUESTION. Establish must not short-circuit the step's DESTINATION because the
     step's SOURCE is still established. One condition, in performworkflow.go.
  2. LOOK AT THIS LOOK. Score the saved destination against observe.ArrangeLook(lk.Regions)
     instead of obs.Arrangement(up.Screen). The regions are already on the same line.
  3. REQUIRE ALL THREE, in workflowstore: agreed >= VisualScreenMatch, as now; the destination
     above the saved source; and an admissible reading of this look that does not disagree with
     the saved destination's words. No new constant and no new threshold.
  4. SAY WHICH IT IS. A fourth `Ground` beside `confirmed` and `inferred`, so a report can tell
     an arrival that a person vouched for from one a workflow reached on its own evidence.
     `Independent()` stays false for it: the expectation still came from the workflow.
  5. AND SAVE THE WORDS. Route the artifact-writing path through `SaveWith`, so a saved
     destination carries what it was read to say.
```

What stays exactly as it is: `Arrival.Verified()` is still false and still says why; text still
cannot propose a merge, reach a stored signature or name a screen; the global observer keeps
every rule Experiment-047 measured.

## What this does not claim

**That it has been implemented or run live.** Nothing here changed a production file. The
numbers are a measurement of evidence already on disk.

**That one recorded navigation is the population.** 222 folds are one walk spliced four ways,
and the artifacts were learned from that same walk — which is why `rl-playtrain-1` is in here,
and it is one held-out page, not a corpus.

**That three steps are a workflow.** The artifact has 39 steps and this measures three of them,
chosen because they are the ones the live run reached.

## Related

- [[Experiment-046-the-destination-it-reached-and-did-not-recognise]] — the live run, and the
  log line this re-reads.
- [[Experiment-047-what-one-reading-is-allowed-to-do]] — the merge-rule line this closes, and
  the 222 folds reused here.
- [[ADR-183-remembering-recognising-and-being-allowed-to-act]] — the three lifetimes a binding
  lives under, which is why a workflow-local arrival cannot become durable knowledge.
- [[ADR-186-a-person-can-say-where-they-are]] — the establishment path this would add a ground
  to, and why an inference is weaker than somebody looking.
- [[ADR-195-a-reading-that-arrives-after-the-look-it-describes]] — the admission rule, and the
  130 readings a merge lets through it.
