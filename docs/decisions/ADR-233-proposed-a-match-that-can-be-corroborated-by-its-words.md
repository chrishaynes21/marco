---
type: adr
status: proposed
date: 2026-10-17
decision: PROPOSED — the smallest next change is not word-box targeting but corroboration: a saved control now carries the word it was read as, and the matcher that finds it again compares rectangles only, so a rectangle match should be reported beside whether the words inside it agree with what was saved; the word-box tightening for the panel case is contingent on that corroboration showing the words are reliable at control scale
supersedes: []
affects:
  - perception
source_paths:
  - docs/experiments/Experiment-080-it-learned-the-word-settings.md
  - internal/director/observe/reacquire.go
  - cmd/director/reacquirecmd.go
---

# ADR-233 — proposed: a match that can be corroborated by its words

## What the evidence says

[[Experiment-080-it-learned-the-word-settings]]: a human click in Ark's menu produced a control
whose reading is 312 × 52 pixels and whose text is `["settings"]`, reacquired on 98% of 300
looks with zero collisions. The first demonstration, before the reader was wired in, produced a
650 × 214 rectangle with no name, reacquired on 2%.

Two things follow, and the second is the one that decides what to build.

**The word is the strongest identity Marco has ever had for a game-menu control**, and it arrived
without any new mechanism — `ScreenReading.Words` → `ClickTargets.Reading` → `LabelOf` →
`controlWords`, all of it already written, needing only the reader to be connected to the loop.

**And the matcher does not read it.** `ReacquireStored` compares rectangles: a stored reading
against the proposals in front of it, through `sameRect`. So 98% matched means *a rectangle of
this shape is in this place on 98% of looks* and not *the settings button is there*. The
artifact carries the evidence that would separate those two statements and nothing consults it.

## So the smallest change is corroboration, not targeting

The owner asked for the smallest change using the demonstrated click position, OCR text and local
visual boundaries to identify the clicked control. On this evidence, that is the *second* change.
The first is smaller and is a prerequisite for knowing whether the second is worth making.

**Report, beside each rectangle match, whether the words inside the matched rectangle agree with
the words that were saved.** Three outcomes and nothing else:

| | |
|---|---|
| corroborated | the matched rectangle reads what the control was read as |
| contradicted | it reads something else |
| unread | no reading was available for that look |

### What makes this not another score term

> Do not add another global screen-similarity score.

It is not a term and not a threshold. **It does not change which rectangle matches.** The match
is made exactly as it is today and the words are reported next to it, so the number that comes
back is the same 98% with a second column saying how much of it is about the right thing. A
reader can then see what the match rate was worth, which is the one question `reacquire`'s own
report says it cannot currently answer:

> a match rate is not a rate of right answers, and a person pressing where a match points is what
> would settle it.

A person pressing is still the strongest check. This is the strongest one that does not need a
person, and `observe.TextsDisagree` and `observe.LabelOf` already exist to make it.

### And it is the first wrongness detector at control scale

Collisions catch two controls claiming one rectangle, which needs two controls. Contradiction
needs one: a control that matched a rectangle reading something else is wrong on its own evidence.
[[Experiment-077-the-screen-was-not-supporting-context]] had 2470 off-screen matches it could only
detect with an oracle; a contradiction is detectable live.

## The word-box tightening is contingent, and stays unbuilt

For the panel case — the 650 × 214 block of demonstration 1, which
`director ocr --region` showed holds 20 readable words — the candidate is the smallest OCR word
box containing the demonstrated click, which uses the coordinate to DISCOVER a target without
storing it as identity.

It stays unbuilt until corroboration says the words are reliable at control scale, and it carries
the owner's caution as a requirement rather than a note:

> Do not assume that a word box containing the click is the intended target; clicks may land on
> the padding around a button label.

So a click in the GAP between word boxes must refuse rather than take the nearest, and the
refusal has to be as visible as a match. Granularity also did not improve between the two
demonstrations — it VARIED, because `structure.Propose` runs on pixels and knows nothing about
words — so nothing measured so far says how often the panel case happens.

## What else this run asked for, which is reporting rather than perception

The screens carry no text though 30 readings were delivered and one control was named from them.
The report says how many readings reached the LOOP, not what the observer did with them, and the
observer counts `ReadingsAdmitted`, `ReadingsStaleWindow`, `ReadingsStaleEpoch`, `ReadingsSilent`
and `ReadingsRefusedMerge` without showing any of them. Three explanations are open and the
session cannot choose between them. Printing those counters is a prerequisite for diagnosing it
and changes no behaviour.

## Status

Proposed. Nothing in this ADR is implemented, no threshold has moved, and pixel execution remains
blocked: one control, one press, and the recorder's own note on the step that *clicked once, so
nothing independent confirms that this control can be found again*.
