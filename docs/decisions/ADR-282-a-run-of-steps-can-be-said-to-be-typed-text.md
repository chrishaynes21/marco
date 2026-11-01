---
type: adr
status: accepted
date: 2026-10-31
decision: "ACCEPTED — A PERSON CAN SAY THAT A RUN OF STEPS IS TEXT THEY TYPED, AND NOTHING GUESSES IT. `markHolds` writes a key down as a HOLD when another key goes down before it is released, and that is what typing rollover IS. Measured on the owner's own `cloud pull stone` recording — ten key events out of 688, presses of 93–149ms overlapping the next by 12–56ms — the word `stone` was saved as `KeyDown s · KeyDown t · KeyUp s · KeyDown o · KeyUp t · KeyDown n · KeyUp o · Key e · KeyUp n`: four held keys and one tap. Neither typing-fold pass can reach it, because both need a step carrying TEXT and a hold carries none, so Simplify did nothing and no value could be asked for either, a value the routine asks for having to be one step. THE CLASSIFIER IS NOT CHANGED, AND THAT IS THE DECISION. `s↓ t↓ s↑ t↑` typed and `q↓ e↓ q↑ e↑` held in a game render identically — `hold Q, press E, release Q`, the canonical game gesture — and nothing in the events tells them apart: not the key names, because Q and E are printable, and not the durations, because somebody can type slowly and tap a game key briefly. A containment rule (the inner press completing inside the outer one) was built and measured and REVERTED for exactly this reason: it distinguishes event shapes, not intent, and the 500ms duration backstop does not protect the brief combination. So a person selects the steps and the result is an EDIT to the list: `typedRun` is the one validator, `TypedRunsIn` proposes the runs so an offer is a promise rather than a second reading of the rules, and `Deps.TypedInDraft` applies one to the recording under review. The character of a held key comes from its KEY NAME, which is what that step types when the routine runs. LONG PRESSES CONVERT WHEN THEY ARE DECLARED, because duration does not know, and the control SAYS IN ADVANCE what that costs: the held keys inside the run stop being held and the waits inside it go, while the waits outside it are untouched. A REPEAT IS KEPT EXACTLY — the two-distinct-characters rule belongs to automatic coalescing and has no business overruling somebody who has said what the steps are — and a representation this cannot reproduce is refused rather than flattened. REFUSED: a key that types nothing, an unbalanced hold, anything that is not typing, text the routine already asks for, and a key held ACROSS the whole range, whose hold the conversion would silently shorten by the waits it removes. AND POSITIONS ALONE ARE NOT CONSENT: `Draft.Shown` is a term for the list the card drew and both draft operations refuse one counted against a list that has moved — the reasoning that a draft needed no such term was wrong, because one window holding a draft means no second PROCESS, not no second writer."
supersedes: []
affects:
  - execution
source_paths:
  - internal/orchestrator/stepedit.go
  - internal/orchestrator/draft.go
  - plugins/studio/typeddraft.go
  - plugins/studio/recording.go
  - plugins/studio/supplydraft.go
  - plugins/studio/frontend/index.html
---

# A run of steps can be said to be typed text

## What happened

[[ADR-280-simplify-takes-the-waiting-out]] left one thing written down as open: the word `stone` in
the owner's Ark routine **does not fold**, and Simplify does not help. That mattered more once
[[ADR-281-a-value-can-be-asked-for-from-the-review]] landed, because a value the routine asks for
has to be ONE STEP, and the only way to get a typed run into one step was Simplify — which
re-derives from the demonstration and therefore cannot be the step you then mark.

So the recording-to-argument journey was blocked on a classifier, and the classifier turned out to
be right.

### The defect, measured

The owner's recording is on disk. Resolved through `marcopaths` — `os.UserConfigDir()`, which is
**Roaming** and not Local — `arkascended/focus/cloud-pull-stone.rec.json` holds 688 events, 14 of
them keys. The ten that matter:

```
s down +9.294   t down +9.387   s up +9.443   o down +9.472   t up +9.528
n down +9.540   o up  +9.595    e down +9.656 n up +9.668    e up +9.749
```

Presses of 149 / 141 / 123 / 128 / 93 ms, overlapping the next by 56 / 56 / 55 / 12 ms. Every one
is a fifth of the 500ms a press needs to be a hold on duration alone — so the duration rule is
innocent. What marks them is `markHolds`' other clause, which asks only whether another
non-modifier key went **down** before this one came **up**. That is the definition of rollover.

The saved `.marco` is the proof, and it is the file the owner has:

```
do OS's KeyDown with "s" · KeyDown "t" · KeyUp "s" · KeyDown "o" · KeyUp "t"
       KeyDown "n" · KeyUp "o" · Key "e" · KeyUp "n"
```

Four hold pairs and one tap. `e` escapes only because nothing goes down inside its press, it being
last.

### And the discriminator that looked available is not one

The shape of the two presses distinguishes a deliberate hold spanning a key (`Q↓ … E↓ E↑ … Q↑`,
the inner press *contained*) from rollover (`s↓ t↓ s↑ t↑`, the presses *staggered*). That rule was
built, measured and **reverted**:

- it converted the owner's word correctly and `go test ./...`, `plugins/e2e` and `plugins/studio`
  all passed, including all fifteen of `internal/simplify`'s modifier-hold tests;
- and it is still wrong, because `q↓ e↓ q↑ e↑` — a brief two-key gameplay combination — is the
  same shape as the typing. Containment distinguishes **event shapes, not intent**. The claim that
  the 500ms rule covers deliberate holds is stronger than the evidence: it covers *sustained*
  ones, and says nothing about a brief combination.

There is no reading of the events that answers this. Which is why the answer is not a reading.

## The decision

### A person says, and it is an edit

`markHolds` is untouched. An ordinary save of that demonstration still produces the holds; so does
a rebuild under today's rules of a routine nobody has edited; so does a second reading of the same
events. What is new is an operation on the step LIST, asked for explicitly, with the steps visible
before and after.

| | |
| --- | --- |
| `typedRun` | the one validator — what may be declared, and the text it becomes |
| `TypedRunsIn` | the runs a list offers, each validated by `typedRun`, so an offer is a promise |
| `Deps.TypedInDraft` | applies one to the recording under review, writing nothing |

The chooser offers the gameplay gesture **as readily as the typing**, and that is the honest
answer rather than a shortcoming: the window cannot tell, and a chooser that hid one of them would
be a chooser that had guessed.
`TestABriefGameplayCombinationIsOfferedAndNotTakenIsStillHolds` is the control.

### The character of a held key comes from its name

A hold carries no `Text` — the same fact both fold passes trip on — so a tap's character comes from
`Text` (which knows that `shift`+`t` is `T`) and a held key's comes from `Key` when that name is
itself one character.

The standard being met is *what this step types when the routine runs*, not what the fingers did:
`KeyDown "s"` then `KeyUp "s"` types a lowercase `s`, and so does `Type "s"`. **A capital typed
with rollover is already lower case before this operation exists** — measured: `shift` held across
an overlapping `s` and `t` renders as `keydown "s"`, with no text and no shift, beside `key "t"`
carrying `"T"`, because `markHolds` leaves a modifier out of a hold and only the tap reaches the
chord fold. That is an adjacent defect in how a held key is *represented*, it is not this slice's,
and it is named in `oneCharacter` so the next reader does not conclude this function introduced it.

### What it says before it is pressed

A long press converts when it is declared, because duration does not know what it meant. The
condition that came with allowing that is that the cost is stated in advance, so the caption is a
backend sentence and says all three things: the keys in the run stop being held, the waits
**inside** the run go, and the waits **outside** it are left alone. The last clause is not padding
— "the waits go", read alone, is a sentence about the whole routine.

### What is refused, and the one that is not obvious

- a key that types nothing (`esc`, `tab`, `f5`, a modifier alone), and a multi-character text
- an unbalanced hold: a `keydown` whose `keyup` is outside the range, or the reverse
- anything that is not typing — a click, an activate, a drag, a loop (which could be expanded and
  is refused on purpose: a representation this cannot reproduce exactly must not be flattened into
  an approximation of itself)
- text the routine already asks for, which would bury a `{{name}}` inside a literal
- **a key held ACROSS the whole range.** The range looks clean from inside, because both halves of
  that hold are outside it and untouched — and converting would still change that gesture, because
  the conversion removes the waits inside the range and a hold is as long as what it spans. A
  600ms charge-hold would quietly become as long as typing two letters. Mouse buttons cannot be in
  this state: `macroir` has no step for a button held across other steps, a click being one step
  and a drag being one step, so there is nothing to check for them — which `typedRun` says rather
  than pretending to check.

A repeat is kept exactly. `macroir` folds a key pressed several times into one step with a `Count`,
and dropping it would turn `aaa` into `a`. The two-distinct-characters rule that keeps game
key-spam out of *automatic* coalescing is deliberately not applied here.

### And positions alone are not consent

`Draft.Shown` is a term for the step list the card drew, and both draft operations refuse one
counted against a list that has moved. The saved routine's editor has had that contract since step
editing existed (`EditReview.Basis`); the draft had none, on reasoning this ADR corrects: one
window holding a draft in memory means no second **process**, not no second **writer**. The window
is the other writer — Simplify re-derives the list, going back to the original replaces it, and
each of the two edit controls changes it.

So `Acts.SupplyDraftText` takes the term too. That hazard was always there and one operation made
it easy to miss.

### The steps are written down afterwards

A declared run is not what rendering this demonstration produces, so `Draft.Changed`
([[ADR-281-a-value-can-be-asked-for-from-the-review]]) is set and the save leaves a step account
beside the routine. Without it the routine would run correctly and then refuse the edit panel and
lose the declaration to a rebuild — which is the defect ADR-281 was corrected for, reached from a
third direction.

## What this does not change

- **How a recording is read.** `markHolds`, the duration threshold, the pointer-span rule for
  modifiers and the click clause are all exactly as they were.
- **An ordinary save, and `RebuildCurrent` of an unedited routine.** Both still produce the holds.
- **The demonstration.** Stored as recorded. A declaration is a change to the derived steps.
- **Simplify.** Still re-derives from the events, and still does nothing at all to a
  rollover-shredded word — which is why the staleness test uses the declaration as its intervening
  change rather than Simplify. The first version of that test asserted a basis change that never
  happened.

## Not here yet

**The saved routine's panel**, which is the second of two increments and uses the same validator.
It matters and is not optional: the recording that exposed this is already on disk, and a repair
reachable only from the review would mean re-recording it. This ADR is amended when it lands.

## Enforced by

- `internal/orchestrator` `TestTheOwnersRolloverIsHeldKeysAndOneDeclarableRun` — the measured
  fixture renders as four held keys and no typed text, AND one run is offered reading `stone`.
  Both halves in one test on purpose: a test asserting only the offer would pass on a build that
  had folded the word automatically.
- `internal/orchestrator` `TestTheOwnersRolloverConvertsWhenItIsDeclared` — it becomes one `type`
  step, the holds are gone, the sentence names the text and the waits, the draft it came from is
  untouched, the demonstration is unchanged, and `Changed` is set. One mutation, killed: dropping
  `next.Changed = true` fails the lifecycle test in Studio.
- `internal/orchestrator` `TestABriefGameplayCombinationIsOfferedAndNotTakenIsStillHolds` — **the
  control this design exists to pass.** `hold Q, press E, release Q` is offered, is not converted,
  and reads the same on a second derivation.
- `internal/orchestrator` `TestARepeatedCharacterSurvivesBeingDeclaredTypedText` — `aaa` survives,
  and a `Count` on something that cannot carry one is refused rather than ignored. One mutation,
  killed: dropping the repeat.
- `internal/orchestrator` `TestAKeyHeldAcrossTheRunRefusesTheConversion` — the refusal names the
  key, and the run is not offered either, so the refusal is never something met after a press.
  One mutation, killed: deleting the boundary scan.
- `internal/orchestrator` `TestADeclarationAgainstAListThatMovedIsRefused` — a range that is
  **valid on the new list** is still refused when the term is the old one, and an empty term is
  refused for that reason and not another. Isolated deliberately: the first version asked for the
  old range, which is out of bounds on the shorter list, so it passed with the basis check deleted
  — the mutation found it.
- `internal/orchestrator` `TestWhatCannotBeDeclaredTypedTextIsSaidPlainly` — the six refusals.
- `plugins/studio` `TestTheCardOffersTheRunRolloverShredded` — the option names the text, points
  inside the list on screen, the caption says what the conversion removes and what it leaves, and
  the card carries a basis.
- `plugins/studio` `TestTheOwnersWordConvertsOnlyWhenItIsDeclared` and
  `TestAGameplayCombinationIsOfferedAndLeftAlone` — end to end through the card, and nothing is
  written.
- `plugins/studio` `TestADeclaredRunCanThenBeAskedFor` — **the journey**: held keys → declared →
  one step → asked for by name.
- `plugins/studio` `TestADeclaredRunSurvivesSaveAndEditingAndARebuild` — save → the edit panel
  lists one step typing the word → a rebuild under today's rules still renders it → and the
  demonstration beside the routine is unchanged.
- `plugins/studio` `TestAnOperationAgainstAListThatMovedIsRefused` — both controls refuse a stale
  term, the refusal says what happened, and a refused operation changes nothing. One mutation,
  killed: deleting the check from `TypedInDraft`.
- `plugins/studio` `TestTheReviewCardDrawsTheControlThatDeclaresTypedText` and
  `TestAReviewWithNothingToDeclareHasNoTypedBox` — rendered in headless Edge, including that the
  option's VALUE carries both ends of the range: a label that is right over a value that is one
  number would declare the wrong steps, and no assertion on visible text would notice.
