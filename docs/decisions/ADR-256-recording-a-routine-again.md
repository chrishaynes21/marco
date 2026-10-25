---
type: adr
status: accepted
date: 2026-10-24
decision: "ACCEPTED — A SAVED ROUTINE OFFERS TO BE RECORDED AGAIN, AND THAT OFFER CARRIES NO AUTHORITY TO REPLACE ANYTHING. Re-recording is what somebody testing a routine does all afternoon, and until now it meant reading the name off the card, typing it back into the box at the top of the page exactly, and remembering which application the first one was saved for. So `RecordAgain` takes the route's whole IDENTITY — not its name, because `settings` can be saved for two applications under two scopes and a name would start a recording of whichever was found first — looks it up AGAIN at the moment the button is pressed, since a card can be on screen for a routine somebody has just deleted, and starts a recording called what that routine calls itself, remembering its application and its scope. It enters through the same `beginRecording` as the ordinary button, so the five refusals are in one place: a second entrance that skipped them would let a recording start while a run was in flight, and a runner pressing things while the hooks watch records Marco's own presses as the person's. `again` is a LABEL AND NOT A LICENCE: the save goes through the same `keep` as every other recording, which asks what is at the target, REFUSES a collision, and offers *Save as* or *Replace that routine by name*. Pressing Save after Record again does exactly what pressing Save does after any other recording. A button that re-recorded and then replaced on an ordinary Save would have reintroduced, through a door marked convenience, exactly the loss the collision check was built to prevent — and the person pressing it would have had more reason than anybody to expect it. The one thing the label does buy is a NOTICE: which routine this is a second recording of, said during the recording and again at review because somebody who pressed the button may reasonably expect Save to replace; the routine's own scope preselected so a replacement is not silently rescoped; and a warning when the demonstration was made in a DIFFERENT application, which is the single case where Save succeeds and does the wrong thing — the identity a save writes is decided by the application that was observed, so a re-recording made elsewhere is a different route with no collision to refuse and a second routine quietly kept beside the one somebody set out to replace."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/studio/again.go
  - plugins/studio/recording.go
  - plugins/studio/replace.go
  - plugins/studio/frontend/index.html
---

# Recording a routine again

## What it replaces, which is typing

Re-recording is the ordinary activity of getting a routine right. The menu moved, a press landed
wrong, the pauses were too tight — so you do it again. Until now that meant:

1. read the name off the card,
2. type it back into the box at the top of the page, exactly,
3. remember which application the first one was saved for, and
4. pick the same scope.

Get the name wrong by a space and you have two routines instead of a new version of one. Get the
scope wrong and the new one works somewhere else than the one it was meant to replace.

So the routine offers it. The identity goes to the backend — **not the name**, because `settings`
can be saved for two applications under two scopes, and a window that handed over the word would
start a recording of whichever one it found first and then offer to replace *that*. The backend
looks the identity up at the moment the button is pressed, because a card can be on screen for a
routine somebody has just deleted from a terminal.

## And what it emphatically does not buy

`capture.again` is a **label, not a licence**. There is no new save path. Save goes through the
same `keep` every other recording goes through: it asks what is at the target, refuses a collision,
and offers *Save as* or *Replace that routine, by name*.

This is the whole of the design, and the reason is in the history. `Registry.Save` is a plain file
write, so keeping a recording under a name already in the library replaced what was there — the
source, and the `.rec.json` beside it, which is the copy a routine is ever saved again from. Nothing
asked, nothing said, nothing kept. The likeliest casualty was the routine somebody was comparing
against.

A *Record again* button that then replaced on an ordinary Save would have reintroduced exactly that
loss, through a door marked convenience, **and the person pressing it would have had more reason
than anybody to expect it**. So the safeguard is unchanged and the button changes nothing about it.

## The one thing the label does buy: three notices

| | why |
|---|---|
| *This is a second recording of "X". Saving it will NOT replace that routine.* | Because somebody who pressed the button may reasonably think it will. Shown **during** the recording and again at review, since the expectation is formed before Save is in front of them. |
| the routine's own scope, preselected | So a replacement is not silently rescoped. A **preference**, not a decision: a forced scope still withdraws the choice, and a scope the recording cannot be performed under is still refused by `keep`. |
| *This was recorded in notepad, and "X" is saved for arkascended. Saving it will make a SECOND routine…* | Because this is the one case where **Save succeeds and does the wrong thing.** |

The third deserves the space. Which routine a save writes is built from the application that was
**observed**, so a re-recording made in a different application is a *different route*: no
collision, no refusal, and a second routine quietly kept beside the one somebody set out to
replace. Nothing is lost, nothing is broken, and it is not what they asked for. Compared only when
both applications are named — a routine about no application has nothing to mismatch, and a
demonstration where nothing was observed has nothing to compare, and inventing a warning for either
would put one on the ordinary case.

## One door, because of the five refusals behind it

`StartRecording` and `RecordAgain` both call `beginRecording`. A second entrance that repeated the
refusals would be five rules in two places; one that skipped them would let a recording start while
a run was in flight, and a runner pressing things while the hooks are watching records Marco's own
presses as the person's. The name rule against a same-named **Act** is among them, because a
recording's name becomes a declaration in any play composed over it.

## What did not move

- **No new save path, no new API.** `SaveDraft`, `Collision`, `SaveDraftAs` and `ReplaceDraft` are
  the four the page already had, and the page calls them in the same order.
- **Replacement is still checked against the identity somebody agreed to.** `ReplaceDraft` compares
  the id it was given with the id this save would write, so agreeing to replace one routine cannot
  replace another that arrived at the target in between. Unchanged, and asserted again here.
- **The button is not disabled for an unrunnable routine.** A routine that refuses every press is
  the one somebody most needs to record again; greying it out there would be the window withholding
  the only way forward. It *is* disabled while a run or a recording is in flight, so a live button
  never answers a press with *something is already being recorded*.

## Enforced by

- `plugins/studio` `TestRecordAgainCannotOverwriteThroughOrdinarySave` — **the claim the slice rests
  on.** Record again, then press Save the ordinary way: it refuses, naming the routine and offering
  a deliberate replacement; the file on disk is unchanged **byte for byte**; the `.rec.json` beside
  it is still there; and the library still holds exactly one routine, which a refusal that had
  quietly saved under another name would hide.
- `plugins/studio` `TestRecordAgainStartsWithTheRoutinesOwnNameAndScope` — the convenience: name,
  identity, scope, and the sentence about what Save will do.
- `plugins/studio` `TestAfterRecordAgainReplacingIsStillAnExplicitChoice` — the way on, and that the
  collision names the routine this was recorded *from* rather than some other one in the library.
- `plugins/studio` `TestCancelAfterRecordAgainLeavesTheRoutineAlone` — Cancel still writes nothing,
  which is the whole reason Stop and Save are separate buttons.
- `plugins/studio` `TestARecordingMadeInAnotherApplicationSaysSoBeforeSave` — both directions: the
  mismatch is reported, and a re-recording made where the routine lives says nothing, because a
  warning on every re-recording is a warning nobody reads.
- `plugins/studio` `TestRecordAgainRefusesARoutineThatIsGone` — and the window can still record
  something else afterwards, asked as that rather than as `a.capture == nil`, which was this
  assertion's first form and was wrong: the saved capture from the previous recording is held on
  purpose.
- `plugins/studio` `TestAnOrdinaryRecordingSaysNothingAboutRecordingAgain` — all five fields empty,
  so a change that set the label always cannot put *this is a second recording of* over somebody's
  first one.
- `plugins/studio` `TestRecordAgainRefusesWhileSomethingElseIsHappening` — a run in flight, and a
  draft waiting to be dealt with.
- `plugins/studio` `TestAReRecordingSaysSavingWillNotReplace` /
  `TestAReRecordingMadeElsewhereIsWarnedAbout` / `TestAnOrdinaryRecordingCarriesNeitherAgainLine` —
  **rendered**. Each asserts the text *and* whether the box exists at all; see the mutation note
  below for why both.

## Mutations

Eight applied, all killed, plus a no-op control that survived. One of them changed the **tests**,
for the third time in this slice-set.

| | mutation | result |
|---|---|---|
| P1 | `RecordAgain` forgets which routine it came from (`again` nil) | **killed** |
| P2 | `keep` lets a re-recording replace through an ordinary Save | **killed** by the central test — the routine on disk changed |
| P3 | the application mismatch is reported for every re-recording | **killed** by the negative half |
| P4 | a routine that has gone is accepted silently | **killed** |
| P5 | the routine's scope is not remembered | **killed** |
| P6 | the review ignores the remembered scope | **killed** in the browser |
| P7 | the page shows the *recorded elsewhere* warning unconditionally | **SURVIVED on its first run**, because the claim-level assertion cannot tell an empty shown box from an absent one. The rendered tests now assert the box's presence as well as its text, and P7 is **killed** |
| P8 | the page shows the *second recording of* line unconditionally | same, and **killed** by the same addition |
| CONTROL | the wording of one refusal is changed | **survived**, as intended |

P7 and P8 are worth the note. An element shown with no text in it is invisible today only because
`.no` and `.muted` are **colour-only** classes — a property of this page's CSS, not of its logic.
Give either a border or a background and an empty box becomes a visible artifact above the Save
button on every ordinary recording. So the tests assert both, with that reason written beside the
helper.
