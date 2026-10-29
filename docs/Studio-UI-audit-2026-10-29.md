---
type: reference
status: current
date: 2026-10-29
source_paths:
  - plugins/studio/frontend/index.html
---

# Studio UI audit — the four states of the main journey

Grounded in `plugins/studio/frontend/index.html` at `b1921f6`. No code changed; this is the
audit and the proposed layout, with behaviour changes named apart from layout ones.

## Measured first

| | |
| --- | --- |
| Visible words on the page | **1,436**, in **207** separate messages (819 in markup, 617 built in JS) |
| Longest single explanation | **49 words**; six are over 35 |
| `view-scenes` | **39 controls**, **5** `<h2>` sections, **614** words of static prose |
| `view-acts` / `view-history` / `view-run` | 11 / 10 / 3 controls |
| The Scene-and-Act definition | given **twice**, in different words, on the same page |
| Other overlapping messages | 4 more pairs sharing a run of six words or longer |

The whole main journey lives in one of four views, and that view holds four fifths of the
controls on the page.

## The four states as they are

### 1. Find or record — two record flows, and the question is buried

`view-scenes` opens with **Record a routine**: a card whose first three elements are three
stacked paragraphs (43, 36 and 21 words) explaining what recording is, before the name field.

The natural-language box is not here. `#askBox` ("say what you want, in your own words") and
`#askTarget` sit *inside* the **Your routines** card, below its own paragraph — so the one
control that answers "what do you want Marco to do" is the fourteenth control in the view.

And there are **two complete recording entry points**, each with its own name field, application
chooser, refresh button and Record button: `capName`/`capWindow`/`capBtn` under *Record a
routine*, and `learnName`/`learnApp`/`learnBtn` under *Learn a Scene*. Four controls on this one
view are labelled Record or Record it again.

### 2. Record with confidence — the chosen window is only shown before you choose it

While recording, the page says `Recording "<name>"` and nothing else about where. The chosen
window is in the `#capWindow` dropdown on the previous card; `#capWhere`, which does name the
application, belongs to the **review** card, not the recording one.

Beneath the state line is a 21-word instruction ("Do it now. Come back here and press **Stop
recording** when you are done…") that repeats what the Stop button says.

Pause is already shown on both surfaces, which is right. It arrives as a banner plus a separate
muted line, where one line would do.

### 3. Review and keep — seven status lines above the steps

`#capReview` stacks, in this order: `capSaid`, `capMissing`, `capUnsupported`, `capWhere`,
`capReviewCaptures`, `capAgainReview`, `capAgainElsewhere` — then the scope chooser, then
`#capSteps`, the list of what was actually demonstrated.

So the thing the person is being asked to approve is the eighth element in the card. Save and
Cancel are correctly distinct, and `Save as` / `Replace` correctly appear only after Save has
refused.

### 4. Run and understand — Play is the eighth control after selection

Selection itself is fine: the chosen card gets an inset marker and a tinted background, and it
was checked rather than assumed (`.card.on`).

What is not fine is the order after it. `#routineChosen` presents: checks, captures, who wrote
it, differences, **Rebuild with current rules**, **Simplify typing pauses**, a review card
holding **two `<details open>`** source panes side by side, **Replace the saved routine with
this**, **Cancel**, **Record it again** — and then **Play**.

Two source listings are expanded by default on a screen whose job is to start a run.

The running view (`view-run`) is the healthiest on the page: the transcript is already behind
`Technical details`. Its one weakness is five separate status containers (`doing`, `paused`,
`pausedWhere`, `said`, `next`, `surface`) stacked before the steps.

## Proposed layout

One journey, one screen at a time, in the order a person meets it.

### 1. Find or record

```
  What do you want Marco to do?
  [ say it in your own words            ]  in [ Discord     v ]  ( Find it )

  ------------------------------  or  ------------------------------

  ( + Record something new )            > how recording works

  Your routines
  o  mute discord          Discord
  o  open settings         Ark: Survival Ascended
```

The question comes first and is the first control. **One** Record entry point, which reveals the
name-and-window form rather than keeping it open above three paragraphs. The three paragraphs
become one collapsed detail. *Learn a Scene* and *Build an Act* leave this screen (see B3).

### 2. Recording

```
  (*) Recording   "mute discord"
      in  Discord - #general                      <- the chosen window, named
      14 presses

      ( Stop recording )   ( Cancel )             or press F9

      > what Marco is watching
```

The window is named while it matters. Paused becomes one line in the same block —
`(||) Paused - you are in Chrome` — rather than a banner plus a muted line. The 21-word
instruction goes: Stop says it.

### 3. Review and keep

```
  "mute discord"   6 steps, 4.2s, in Discord

   1  click      (100, 200)
   2  press      ctrl+shift+m
   3  wait       250ms
   ...

  it works  [ only in Discord            v ]
  (!) two presses could not be represented

  ( Save )   ( Cancel )        > the Marco it will save
```

The demonstrated steps come first, because they are what is being approved. Scope sits beside
them. Warnings sit under them rather than above. The source goes behind a detail.

### 4. Run and understand

```
  Your routines
  o  mute discord          Discord
  (o) open settings        Ark: Survival Ascended       <- selected
  -----------------------------------------------------
  open settings            Ark: Survival Ascended
  (!) needs its window in front

  ( > Play )                                  <- the only primary action

  > what it will do        > rebuild and re-record
```

Play is first and alone. The two rebuild buttons, *Record it again*, and the source comparison
move behind one collapsed detail, and the two `<details open>` close.

And while it runs, one status line and one reason line in place of five stacked containers;
`Technical details` stays where it is.

## Confusing controls

1. **Two Record flows** on one screen (`capBtn`, `learnBtn`), each with its own name and
   application fields.
2. **Four controls** say Record or Record it again (`capBtn`, `learnBtn`, `routineAgainBtn`,
   `againBtn`).
3. **Three** say Cancel, plus a Clear and a Keep it.
4. **Two rebuild buttons** sit above Play and read as things to do before running
   (`Rebuild with current rules`, `Simplify typing pauses`).
5. `Play - press for real` carries its own warning in the label, where every other primary
   action does not.

## Duplicated messages

1. *"A Scene is one interaction… an Act puts Scenes in order"* — said in `#actsEmpty` and again
   in the `view-scenes` sub-heading, in different words.
2. *"every press is written down instead of sent"* — twice.
3. *"An Act performs interactions from one demonstration"* — twice.
4. *"the demonstration read no words on…"* — three phrasings.
5. *"It starts on the screen that reads…"* / *"This Act starts on the screen that reads…"*.

## Missing feedback

1. **The chosen window is not shown while recording** (§2). The one fact the recording rule is
   built on is the one fact the recording state omits.
2. **No count of what has been captured so far** during recording — `capCaptures` exists on the
   form and `capReviewCaptures` on the review, but nothing in between.
3. **Nothing says why Play is unavailable in the list.** `routineWhy` explains it only after
   selection, so an unrunnable routine looks identical until it is chosen.

## Behaviour changes, named separately

Layout and wording only above. These four change what the page *does*, and none touches a
backend call:

- **B1.** The record form is revealed by a button instead of being open on arrival.
- **B2.** The two source panes in the rebuild review default to **closed** (`<details open>` →
  `<details>`).
- **B3.** *Learn a Scene*, *Build an Act* and *What you have shown Marco* move off the journey
  screen to their own rail entry. The vocabulary does not change — a Scene is still a Scene and
  an Act is still an Act; only which screen they live on does.
- **B4.** The three explanatory paragraphs on the record card collapse into one detail.

## What does not change

Every backend contract: `Acts.*`, `Recording.*`, the resolve call behind *Find it*, the scope
policy, the consent-to-a-version rule, and the one-Play-click grant. No new Studio surface, no
new execution path, and nothing here reads or writes the route store differently.
