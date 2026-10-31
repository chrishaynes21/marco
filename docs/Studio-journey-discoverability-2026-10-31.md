---
type: reference
status: implemented
date: 2026-10-31
source_paths:
  - plugins/studio/frontend/index.html
  - plugins/studio/recording.go
  - plugins/studio/replace.go
  - internal/orchestrator/draft.go
  - internal/orchestrator/stepedit.go
---

# Can a new person get from Record to a first run?

Audited at `21dcf87`, the layout slice. The audit found three things; the Architect ruled on all
three and they are implemented, in two separately reviewable changes — the copy and layout pass,
and the simplified-save repair. The `after` screens below are rendered from the page as it now
stands, not from a proposal.

The gate at the audited commit: `go run ./cmd/gate` — **passed, 0 failures, 0 findings, 30
tolerated skips, 5m18s**. `plugins/studio` passed with 0 skipped.

## The five abilities, as they were found

| | |
| --- | --- |
| **Choose the window and understand what is captured** | the chooser worked; what the card SAID about it was false — 1 |
| **Finish with Stop or the shortcut** | yes, and the shortcut was shown only in the window you cannot see — 4 |
| **Find Simplify and understand what it changes** | found, directly above Save, with a sentence before the press. It did not say what it cost — 2 |
| **Turn a typed step into an argument** | yes. *Edit its steps* names it on the button's own line, and the control is on the typing row. **Blocked** for a routine saved from a simplified draft — 2 |
| **Supply it beside Play and confirm** | the box and the read-back were there. Play was dark until the box was filled and said nothing — 3 |

Two of the five were built and reachable. The audit's reading was that the other three failed on
what the page SAID; the Architect's ruling corrected that on one of them — the simplified-draft
dead end is a journey defect and was repaired rather than documented.

## 1. The record card denied the thing the control under it did

**What a person saw.** A warning banner, the largest thing on the record card:

> While this is recording, Marco watches your keyboard and mouse EVERYWHERE — every application,
> not just the one you are demonstrating in. Anything you do in the meantime becomes part of the
> routine. Studio's own window is the one exception and is left out. **Recording only a chosen
> window is not something Marco can do yet.**

Immediately below it, a row reading `record [Untitled - Notepad] [refresh]`.

**It was not true.** [[ADR-268-record-only-the-chosen-window]] landed on 2026-10-28:
`orchestrator.Watching`'s zero value is `OneWindow`, Studio sets no `Watch`, and `StartRecording`
refuses a recording with no window of its own. The band two states later says so three times over
— `capInWindow` names the window, `capPaused` says the recording has gone silent because you left
it, `capExit` says the two keys that brought you back were trimmed. All of that exists only
because confinement does.

**What landed.** `everywhereSentence` in `plugins/studio/recording.go`, in the Architect's wording:

> Records actions in the window you choose. Recording pauses outside it. Return to Studio and
> press Stop, or press F12.

Same field, same element, same banner. The key is interpolated from the backend and the
disclosure now carries the way out as well — see 4.

**What moved with it.** *TestTheWindowSaysRecordingWatchesEverythingBeforeRecord* asserted each
of the dead clauses, so the test was holding the false claim in place. It is now
`TestTheWindowSaysWhatARecordingTakesInBeforeRecord`, and it asserts the four live clauses present
**and the two retired ones absent** — a disclosure can name the chosen window and still go on to
deny it, and every positive assertion would pass. [[ADR-268-record-only-the-chosen-window]] carries
the amendment; [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]] cited the test by
its old name and its old claim.

**Rendered:** `6-what-is-captured-before.png`, `6-what-is-captured-after.png`.

## 2. Simplify cost step editing, and the journey asked for it next

**What a person saw.** Beside *Simplify…*, before the press: *Folds typing pauses and runs.
Nothing is written until you save.* After it: *These steps are a simplified version. Nothing is
written until you save.* Both true, both about the card.

**What was not said.** A routine saved from a simplified draft **could not have its steps edited
afterwards**, which is the next thing in the journey. No step account at save time, so
`PrepareEditOf` rebuilt from the demonstration under today's rules, got the unsimplified version,
and refused with:

> *"write a note"* cannot be edited: its saved source is not what its own demonstration renders to
> under today's rules, so what the file does and what was demonstrated no longer agree — reviewing
> a rebuild is where that is decided

Accurate, and it reads like file damage. The person pressed the two buttons in the order the
journey puts them in.

**The audit proposed a sentence; the ruling was to fix it.** It proposed saying what the cost was,
and naming the order that avoided it — save as recorded, then simplify from the saved routine's
own card and Replace, because a replacement from a rebuild writes the account. The Architect
declined the workaround: *both paths should produce an editable routine*, and authorized a bounded
backend increment.

**What landed.** A draft that was rendered under rules a rebuild does not use carries that fact
(`Draft.Basis`), and the save writes the step account beside the routine — after the source, the
assets and the demonstration, with both digests read back from where the save put them. The
demonstration is stored exactly as before. A plain save still writes no account at all, so every
routine already in a library is still edited by rebuilding from its own demonstration. A failed
account is reported as what it is: the routine runs, its steps were not written down, and that is
neither a success nor a refusal. See [[ADR-279-a-simplified-save-keeps-its-own-basis]].

*TestASimplifiedSaveCannotBeStepEditedYet* said to delete itself when this was
fixed. It is now
`TestASimplifiedSaveCanBeStepEditedAndSavedAgain` and walks Simplify → Save → Edit → Save again
through the window's own entrances.

**Rendered:** `6b-simplify-before.png`, `6b-simplify-after.png`.

## 3. Play was dark and silent while the text it needed was empty

**What a person saw.** A routine that types supplied text draws its fields above Play. Arriving at
that card, `message` is empty — nothing is ever prefilled, by rule — so `paintRoutinePlay`
disabled Play, and nothing on the card said why. The end of the journey was a greyed button beside
an empty box.

Four conditions darken that button and three already had a sentence: not runnable is `routineWhy`,
no window to press in is `routineIntoWhy`, a run in flight writes `routineNote` itself.

**What landed.** One muted line beside Play, in its own element rather than as a second writer of
`routineNote`:

> Fill in message above to play it.

On **unsupplied** fields, not empty ones — `missingArgs` reads the same predicate as
`suppliedArgs`, so a box somebody typed into and cleared is a value and a routine asked to type
nothing types nothing. Secrets are never named: the store answers for those and there is no box.
The line is written only when the routine is otherwise ready, so it never competes with a refusal,
and only its own sentence is cleared.

**Rendered:** `6c-play-before.png`, `6c-play-after.png`.

## 4. The way out, which the audit named and did not propose

**The stop shortcut was shown only in the window a person cannot see.** `capKey` renders *or press
F12* as muted text at the end of the row, after a red Cancel — and it existed ONLY in the recording
band, which while recording is behind the application being demonstrated in. The record card, on
screen while somebody is still looking at Studio and deciding, never named the key.

**And the audit was wrong about the key itself.** It said that with `$MARCO_STOP_KEY` unset there
is no key and the silence should say so. There is: `recorder.ParseStopKey("")` is
`recorder.DefaultStopKey`, which is F12. One of Studio's two report sites read the environment
variable raw and the other parsed it, so the same installation named F12 during a recording and
nothing at all before one — the unset variable reading as *no shortcut* in the one state where
somebody still had time to learn it. Both sites go through `stopLabel` now, and the record card
names the key through the disclosure in 1.

**Rendered:** `6d-the-way-out.png` is the band as it was, with the shortcut as the smallest thing
in it; `6-what-is-captured-after.png` is the record card now naming it before Record.

## The visual pass

All four approved, and none of them changes a sentence, a refusal or a boundary.

- **The window chooser is above the name.** The window is the consequential choice and the one
  needing preparation (*Open it first if it is not in the list*), and the card asked for the name
  first — so the line telling somebody to go and open their application sat at the bottom of a
  form they had already started filling in. No test held the order inside `captureCard`.
- **The argument fields are labelled.** *this routine types text you supply* was a lowercase muted
  fragment over an unlabelled field; it is a heading now, and each row's label is bound to its box.
- **Review no longer repeats the record card.** `capReviewCaptures` said *Where you press and how
  long you pause…* between the scope chooser and Simplify — between a person and the two buttons
  they came for, about a choice made several states ago. It is said beside Record, and a saved
  routine's own card states how it was made. `TestThePageSaysWhatARecordingWillCapture` asserted
  the repetition and now asserts its absence.
- **A step row's controls stay on its row.** On a typing row, `name it` landed on its own line
  above *Ask me for it* and *Remove*, so one step was two rows deep. The step's words shrink
  instead; they already break anywhere.

## And one the renders showed

**Two consecutive sentences said Play brings a window forward and presses there.**
`routineIntoSays` (the backend's, naming the window that is selected and whether its presses are
checked) sat directly above `routinePlayMeans` (the page's, naming the application), in the same
muted grey. The window is the accurate half — resolved, the one a run will raise — so `playMeans`
now answers only for the routine that names no application, which raises nothing and has no window
to name. A routine with an application and no window to press in has a refusal on screen instead,
which is what that person needs to read.

## Rendered states

| | |
| --- | --- |
| `6-what-is-captured-before.png` / `-after.png` | the record card: the banner that denied the chooser under it, and the one that names it |
| `6b-simplify-before.png` / `-after.png` | review, in the simplified state, where Save is the next button |
| `6c-play-before.png` / `-after.png` | a selected routine that takes text, with its steps open on the *Ask me for it* row |
| `6d-the-way-out.png` | the recording band, with the shortcut as the smallest thing in it |

All are headless Edge renders of stubbed state. The fixtures quote the backend's own sentences
(`capturesSentence`, `everywhereSentence`) rather than inventing them, and the three routines are
`app`, `app-anywhere` and `everywhere` so the list reads as the product writes it.

## NOT ESTABLISHED

**No desktop, and no window.** Studio was not closed, rebuilt or launched for any of this; the
screen was not taken; no routine of the owner's was read or changed. That a recording of one
window behaves as finding 1's sentence now describes it is held by `internal/recorder` and
[[ADR-268-record-only-the-chosen-window]], and has not been exercised on this desktop since the
layout slice. The simplified-save repair is established by
`TestASimplifiedSaveCanBeStepEditedAndSavedAgain` through Studio's own entrances and by two
mutations, not by pressing the buttons.
