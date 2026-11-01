---
type: adr
status: accepted
date: 2026-10-31
decision: "ACCEPTED — STUDIO SAYS A ROUTINE CAN TAKE A VALUE, AND A VALUE CAN BE ASKED FOR FROM THE REVIEW. The capability was COMPLETE AND SILENT: a routine can be told to ASK for a piece of text instead of typing what was demonstrated, three ways, and Studio wired all three and mentioned none. `Deps.Begin` splits the recording's name through `routes.SplitArgs`, so `cloud pull stone with item` declares an argument; `Deps.ArgKey` is passed from `$MARCO_ARG_KEY` and falls back to F9 (`simplify.DefaultArgKey`), so the reserved key drops a placeholder where the value goes; and [[ADR-278-the-text-a-step-types-can-be-made-something-you-supply]] can turn a typed step into a supplied one. `marco learn` prints the key in its prompt (`argKeyHint`); the window with the Record button did not, so the owner of this repository went looking for the feature and reported it missing — which is the only report a silent capability can produce. TWO CHANGES, AND NEITHER ADDS A MECHANISM. First, the arg key is NAMED on the record card and in the recording band, from `orchestrator.ArgKeyFor` so that ONE reading of the override serves the recorder and the window alike — an unset variable is F9, not silence, which is the exact defect the stop key shipped with three rows up the same card. Before a recording the card says what the key is for and that the name's `with` clause names the value; during one the band names the values this recording declared, IN ORDER, because the Nth press fills the Nth name and nothing afterwards can tell that two went the wrong way round. Second, `Deps.SuppliedInDraft` makes a step's text supplied on the recording UNDER REVIEW, before anything is saved — the operation existed only on a saved routine, so somebody who realised at review had to save, find it in the panel, edit it, and save again. The rules are not duplicated: `suppliedName` is the whole of what may be asked and `EditArgument` asks it too, which keeps the credential refusal in one place. A VALUE IS ONE STEP AND A FRESH RECORDING IS ONE STEP PER CHARACTER, so on most drafts there is nothing to pick and the card SAYS WHY and names Simplify, rather than showing an empty chooser — an empty control reads as the capability being absent, which is how this slice came to be asked for. AND SIMPLIFY TAKES THE MARKS OFF, having said so first and named them: it re-derives from the events, and a position cannot honestly be carried across a list that folded. AND THE SAVE WRITES THE LIST DOWN, which it did not: a mark changes a step's text, so from that moment the steps are a rendering of nothing — and `accountFor` asked `Draft.Basis`, which only a REBUILD sets, so a mark made on a draft nobody had simplified saved with no step account beside it. The routine itself was correct and ran and asked for its value; everything afterwards was wrong, because no account LICENSES rendering the demonstration and treating the result as what the file is (`routes.StepsNone`) — so the edit panel REFUSED with a sentence about the source and the demonstration no longer agreeing, and a rebuild under today's rules re-derived the mark away. `Draft.Changed` says the steps were changed after they were rendered, and `Draft.ownSteps` is the one question the two facts answer, because `Basis` names which of two rebuilds rendered a list and a marked list was rendered by neither — `RebuildCurrent` would have been a word that means the opposite. Invisible until a run typed at TYPING SPEED turned out to fold without Simplify, which is also what makes the whole control reachable on an ordinary recording. NOT DONE, and ruled on: a button in the recording band that drops a placeholder mid-demonstration. It needs a stored recorder event kind of its own (`Draft.Events` strips every existing marker) and can only be pressed by leaving the window being demonstrated in, which is strictly worse than the key that already exists."
supersedes: []
affects:
  - execution
source_paths:
  - internal/orchestrator/orchestrator.go
  - internal/orchestrator/draft.go
  - internal/orchestrator/stepedit.go
  - plugins/studio/recording.go
  - plugins/studio/supplydraft.go
  - plugins/studio/frontend/index.html
---

# A value can be asked for from the review

## What happened

The owner asked where arguments are added to a recording in Studio, and whether Studio is meant to
have parity with Director. Reading the code, the answer was awkward: **it already does, and says
nothing about it.**

A routine can take a value three ways, and every one of them is wired in Studio:

| | how | where it is wired |
| --- | --- | --- |
| the name declares it | call the recording `cloud pull stone with item` | `Deps.Begin` → `routes.SplitArgs` |
| the reserved key | press F9 during the demonstration where the value goes | `Deps.ArgKey`, `simplify.go`'s `argKey` branch |
| the step edit | turn a typed step into a supplied one afterwards | [[ADR-278-the-text-a-step-types-can-be-made-something-you-supply]] |

The third is Studio's own and Director has no equivalent. The first two were printed by
`marco learn`'s terminal prompt — `argKeyHint`, *"(tap F9 where a value should be an argument)"* —
and by nothing in the window that has a Record button.

So the report was not wrong. A capability nothing mentions is indistinguishable from one that does
not exist, and the person who could not find it owns the product.

## The decision

### The arg key is named where it can still be pressed

`orchestrator.ArgKeyFor` resolves the override, and the window asks it rather than reading
`$MARCO_ARG_KEY` again. **One reading of one variable**, because the second reading is a defect
this card has already shipped: the stop key was parsed at one site and passed raw at another, so an
unset `$MARCO_STOP_KEY` read as *"there is no shortcut"* on the record card and as F12 in the
recording band — three rows apart, about the same keyboard
([[ADR-268-record-only-the-chosen-window]]'s amendment). An unset arg key is **F9**, not silence.

Two states, one field (`Recording.ArgSays`), like `Everywhere` beside it:

- **Before Record** — what the key is for, and that calling it `… with item` names the value. On
  this card because the key is pressed during the demonstration, and because the field the `with`
  clause goes in is two rows above the sentence.
- **While recording** — the values this recording declared, *in order*. The order is the one fact
  worth saying while it is running: the Nth press of the key fills the Nth name, and nothing
  afterwards can tell that two of them went the wrong way round.

The band carries it as well as the card, because the card is hidden while a recording runs and the
band is the only surface on screen — behind the application, which is why the card has to say it
first.

### A value can be asked for from the review

`Deps.SuppliedInDraft` does to a draft what `EditArgument` does to a saved routine. The same
reasoning as [[ADR-279-a-simplified-save-keeps-its-own-basis]]' sibling, `SimplifiedDraft`: the one
surface where somebody is looking at a demonstration they just made — steps in front of them, Save
under their hand — could not say *"this bit is the item name"*. They had to save, find the routine
in the panel, edit it, and save again. Two writes and a hunt to express something they knew while
they were demonstrating.

**The rules are not duplicated.** `suppliedName` is the whole of what may be asked — the step types
text, it is not already supplied, the name is a name, and it is **not** a word `codegen` reads from
the credential store — and both ways in ask it. A second copy of those four checks is a second
place for the credential refusal to be missing from, and that is the one of them that quietly
changes what a routine does.

**And the mark declares the name.** `codegen` keeps a `{{name}}` inside a typed literal only when
the name is DECLARED and turns any other one into a credential-store read, so a mark that changed
the step text and nothing else would save a routine that types whatever is in the store under that
word. `declaredFor` answers it, which is the same function the saved edit and the rebuild ask —
the third site of the defect ADR-278 recorded.

### A value is one step, and a fresh recording is one step per character

The part that would have made the control useless. A recording made at a human pace holds each
keystroke as its own step — `type "s"`, `wait 180ms`, `type "t"` — because `coalesceTyping` folds
only what is adjacent and a 400ms gap is a wait between them. The run becomes one `type "stone"`
when the list is derived with a typing rhythm, which is what **Simplify** does.

**A run typed at typing speed is the exception, and it matters twice.** `simplify.MinWaitMs` is 30,
so gaps under that emit no wait at all and `coalesceTyping` merges the keys in the ORDINARY draft —
which is to say a value can be marked on a recording nobody simplified. That is the common case
for anybody who can type, it is why this control is reachable without Simplify at all, and it is
the state the lifecycle defect below lived in.

So on most fresh drafts there is nothing to pick, and the reason is not a missing feature: *"ask
for this value"* needs a value, and until the run is folded there are five of them. The card says
what the recording is, what a value has to be, and which button closes the gap. **An empty chooser,
or no control at all, reads as the capability being absent** — which is how this whole slice came
to be asked for, so it is the one outcome the design refuses.

### And Simplify takes the marks off, having said so first

`SimplifiedDraft` re-derives from the EVENTS, so a marked value is typed text again, and there is
no honest way to carry a position across a list that folded. The caption names the values while
they are still there — *"your marks"* is not something anybody can check — and the marks really do
come off. Going back to the original clears them too, for a plainer reason: the original is the
recording as it was MADE, and a mark was applied to a list derived from it.

### And the save writes the list down, because a marked list is a rendering of nothing

The half this slice shipped without, found by the Architect reading it before it was committed.

A mark rewrites one step's text into `{{name}}`. From that moment the step list is not what
rendering this demonstration produces — the demonstration types `stone` and the list asks for a
value — under today's rules or the other rebuild's. Which is precisely the condition
[[ADR-279-a-simplified-save-keeps-its-own-basis]] exists for: a save leaves a **step account**
beside the routine so that nothing later has to guess what the file is the render of.

**It asked the wrong question.** `accountFor` read `Draft.Basis`, and `Basis` is set by
`draftFor` for a *rebuild under other rules*. A simplified draft has one, so a value marked after
Simplify was fine and every test in the first version of this slice went through Simplify first.
A draft nobody had simplified has none — and the save wrote no account:

| | what happened |
| --- | --- |
| the save | CORRECT. The routine landed, ran, and asked for its value by name. |
| the edit panel | **REFUSED.** No account means `basisFor` renders the DEMONSTRATION and compares; it types `stone`, the file asks for a value, and the sentence is *"its saved source is not what its own demonstration renders to under today's rules"* — which reads like file damage. |
| a rebuild with today's rules | **LOST THE MARK.** `Prepare` renders the account when there is a usable one and the demonstration when there is not, so it re-derived and dropped the argument. |

So a draft says when its steps were CHANGED, not only when they were rendered under other rules:

- **`Draft.Changed`**, set by `SuppliedInDraft`. A separate field rather than
  `Basis = RebuildCurrent`, because `Basis` names *which of two rebuilds* rendered a list and a
  marked list was rendered by neither — and `RebuildCurrent` is the word for "today's rules",
  which is exactly the case the field documents as needing no account. It would have put that
  word in the log beside an account it does not describe.
- **`Draft.ownSteps`** is the one question both facts answer. A function and not a comparison
  written out where it is asked, because the defect WAS the comparison: one reader, asking half of
  it. A third draft operation is a third way into this state, and whoever writes it has one
  sentence to find rather than one condition to remember.

**And the way back keeps its marks.** Because a value can now be marked before Simplify is ever
pressed, the draft `SimplifyDraft` keeps as the original can be one with placeholders in it.
`capture.wasSupplied` travels with `capture.wasDraft` and is restored with it: a draft that comes
back with `{{item}}` in its steps under a card that says nothing about it is the same wrongness as
a card warning about a mark that is gone, only quieter.

## What this does not change

- **Nothing is written until Save.** The draft under review is swapped for the marked one, exactly
  as Simplify swaps it, and Save persists whichever is being shown.
- **The demonstration.** Stored as recorded; a mark is a change to the derived steps.
- **The saved routine's panel.** Unchanged, and now the junior partner: the same operation is
  reachable before the first write.
- **The reserved key.** Still there, still F9, and now said out loud. It costs nothing while your
  hands are in the other window; the review costs no foreknowledge. Both on purpose.

## Not done, and why

**A button in the recording band that drops a placeholder mid-demonstration.** Asked for, and
ruled against on two grounds:

1. **It cannot be pressed without leaving the window being demonstrated in.** The band is behind
   that window; reaching it pauses the recording. That is strictly worse than a key you press
   without moving your hands, which is what F9 is for.
2. **The mark would need a stored recorder event kind of its own.** `Draft.Events` carries no
   markers at all — `EvLeft`, `EvEntered`, `EvCrossed`, `EvTargetGone` and `EvHeldAcross` are
   capture state and are stripped before storage — and simplification re-derives from the stored
   events on every rebuild, so a mark that is not in them is lost at the first Simplify. Making it
   an ordinary `EvKey` would be **simulating a human press into the recording**, which is not a
   thing this product does.

Left open deliberately, with the F9 disclosure and the review-time mark standing in for it.

## Enforced by

- `plugins/studio` `TestTheWindowSaysHowToMakeAValueSomethingYouSupply` — `Recording` names the
  key as **F9** with no override set, and the sentence says what pressing it does and how the name
  declares the value. The "no override" half is the defect's shape: a raw read would go silent in
  every installation that never set the variable.
- `plugins/studio` `TestTheRecordingBandNamesTheDeclaredValuesInOrder` — a recording called
  `cloud pull stone with item, count` has its values named *in order* while it runs, and the band
  does not repeat the advice about declaring them.
- `plugins/studio` `TestAnArgKeyTurnedOffIsNotOffered` and
  `TestTheWindowNamesTheConfiguredArgKey` — `off` offers nothing; `f8` is named as F8.
- `plugins/studio` `TestTheRenderedPageSaysHowToMakeAValueSomethingYouSupply` — rendered in
  headless Edge: the sentence is on the record card, and the element is hidden when the backend
  names no key.
- `plugins/studio` `TestAFreshRecordingSaysToSimplifyBeforeAskingForAValue` — a recording with one
  step per character offers nothing, says why in those words, names Simplify, **and pressing
  Simplify really does produce something to ask for** — the advice has to be true.
- `plugins/studio` `TestTheReviewOffersToAskForTextItWouldType` — the chooser's positions are
  positions in the step list on screen, each option names the text it types, a mark changes the
  step, the step is not offered twice, and nothing is written.
- `plugins/studio` `TestASuppliedValueSavesAsSomethingTheRoutineAsksFor` — review → Simplify →
  mark → Save → the Play card asks for `item` as TEXT, and the source types `{{item}}` rather than
  reading the credential store. One mutation, killed: dropping `declaredFor` from
  `SuppliedInDraft` leaves the saved routine asking for nothing and rendering a secret read.
- `plugins/studio` `TestADraftValueCannotBeCalledSomethingTheGeneratorReadsFromTheStore` and
  `TestADraftValueNeedsAName` — the credential refusal and the name pattern, through the draft
  path, with the draft untouched after each refusal.
- `plugins/studio` `TestSimplifyingTakesTheSuppliedMarksOffAndSaidSoFirst` and
  `TestTheOriginalComesBackWithNoSuppliedMarksOnIt` — the warning is on screen BEFORE the press,
  names the value, and the mark really comes off. One mutation, killed: leaving `cp.supplied` set
  through a simplify leaves the card warning about a mark that is gone. This test also found a
  real defect in review — marking the only typed step made the caption fall back to *"press
  Simplify first"*, which is why the caption is composed from sentences rather than chosen between
  them.
- `plugins/studio` `TestTheReviewCardDrawsTheControlThatAsksForAValue`,
  `TestAReviewWithNothingToAskForSaysToSimplifyFirst` and `TestAReviewOfClicksHasNoSupplyBox` —
  rendered: the box follows the sentence and the row follows the options, which are two different
  facts, so a mutation showing either unconditionally survives an assertion on the other.
- `plugins/studio` `TestARecordingThatTypesNothingIsOfferedNoValueToAskFor` — a recording of
  clicks gets no control and no sentence, and asking anyway is refused.
- `plugins/studio` `TestAMarkedOrdinaryRecordingCanStillBeEditedAndRebuilt` — **the lifecycle**,
  on a recording nobody simplified: mark → Save → the edit panel lists it as something the person
  supplies → a rebuild under today's rules still renders `{{item}}` and no credential read → and
  the demonstration beside the routine is the one that was made, with no mark in it and unchanged
  by the review. One mutation, killed: dropping `next.Changed = true` from `SuppliedInDraft`
  fails it at the panel, with the exact sentence above. Deleting `|| dr.Changed` from `ownSteps`
  fails it the same way, which is the point of the function.
- `plugins/studio` `TestTheOriginalOfAMarkedRecordingComesBackWithItsMarkDescribed` — a draft
  marked before Simplify comes back from *Use the original* with its mark AND the sentence about
  it. One mutation, killed: restoring the draft and clearing the marks leaves a placeholder on
  screen that nothing describes.
