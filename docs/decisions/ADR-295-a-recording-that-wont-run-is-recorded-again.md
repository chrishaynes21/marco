---
type: adr
status: accepted
date: 2026-11-02
decision: "ACCEPTED — A RECORDING THAT WON'T RUN OFFERS RECORD AGAIN FIRST, AND SIMPLIFY SAYS WHAT IT COSTS WHERE IT IS APPROVED. A review whose presses were not all measured offered `Save anyway` as its filled button and had no way to record again; the emphasised next step was saving a routine that refuses every press. Now the won't-run box carries RECORD AGAIN as its filled button and Save anyway is secondary. Record again (1) SAYS beside itself that it discards this recording, (2) discards it through the existing cancel binding and stops if that is refused, (3) keeps the name and the chosen window, (4) REVALIDATES the window by asking for the window list again — a handle that has gone is not replaced by whatever is first in the list; the setup form opens with `The window it was recorded in is no longer open` and the person chooses — and (5) starts through the same call the form uses, or, for a second recording of a saved routine, through `RecordAgain`, so it stays a second recording of that routine. IT IS NOT OFFERED for a recording this version cannot save (`unsupported`): recording the same thing again would be refused again. SIMPLIFY'S CONSEQUENCES ARE IN THE REVIEW, not a tooltip: before the press, beside the button, the timing change and — by name — the values it would take off; after it, beside Save, the same timing change, the way back, and the values it took off. The backend sends `marked` and `unmarked` for that, the one backend change, and `unmarked` is tracked per simplification rather than read from the first simplification's way back, which does not hold marks made after it. The review's summary is one line, and Cancel is `Discard`, which is what it does. The order the review card has been held to — the steps before every sentence about them — is unchanged."
supersedes: []
affects:
  - execution
source_paths:
  - plugins/studio/frontend/index.html
  - plugins/studio/recording.go
---

# A recording that won't run is recorded again

## What happened

[[Studio-coherence-audit-2026-11-02]] finding 4, and the Architect's ruling:

> Approve Record again as primary for a correctable recording refusal, with Save anyway secondary
> where saving is supported. Record again must disclose that it discards the current draft,
> preserve setup choices, and revalidate the chosen window. It must not imply re-recording fixes
> an unsupported machine capability.
>
> Don't hide the meaningful consequences of Simplify in a tooltip alone. Explain timing changes
> and discarded supplied-text marks in the review where the person approves them.

## Two decisions inside the ruling

**The refusal stays below the steps.** The audit's mockup put it at the top of the card. The card
has been held, since [[Studio-UI-audit-2026-10-29]], to showing the demonstrated steps before
every sentence about them. That ordering was not part of this ruling, so it was not reopened. The
refusal box keeps its place and gains its remedy.

**Record again starts recording when the window is still there.** It does not reopen the form for
a second press. The button says what it will do, and the press is the decision. The form appears
only when there is a choice to make again.

## The defect the test found

`unmarked` was first read from `wasSupplied`, the marks kept with the FIRST simplification's way
back. A value marked on an already-simplified draft and then simplified away is not there, and
the new backend assertion failed on exactly that fixture. `dropped` is now recorded at each
simplification, and `unmarked` is `dropped` minus anything marked again since.

## Enforced by

- `plugins/studio` tests for the review:
  - `TestARecordingThatWontRunOffersRecordAgainFirstAndSaysItDiscards`;
  - `TestAnOrdinaryReviewKeepsSaveFirstAndOffersNoRecordAgain`;
  - `TestRecordAgainIsNotOfferedForWhatThisVersionCannotSave`.
- `plugins/studio` tests for the sequence:
  - `TestRecordAgainDiscardsKeepsTheSetupAndStartsInTheSameWindow` checks the exact call order:
    cancel, then start with the same name and window;
  - `TestRecordAgainAsksForAWindowWhenTheOneItUsedHasGone` checks that nothing is started, the
    form opens and says why, and the name is kept;
  - `TestRecordAgainOfASecondRecordingKeepsItASecondRecording`.
- `plugins/studio` `TestSimplifySaysWhatItCostsBeforeAndAfter`: the timing, the marked value by
  name before and after, the way back, and no tooltip.
- `plugins/studio` `TestSimplifyingTakesTheSuppliedMarksOffAndSaidSoFirst` now also holds
  `marked` and `unmarked`.

Five mutations were run and each failed its test:

- no revalidation of the window;
- Record again offered for an unsupported recording;
- Save anyway left filled;
- the marked values left unsaid;
- a second recording started as a first.
