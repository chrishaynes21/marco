---
type: decision
status: accepted
date: 2026-10-08
affects:
  - text-editing
  - verification
  - execution
source_paths:
  - internal/director/edit/edit.go
  - internal/director/edit/editor.go
  - internal/director/edit/planner/planner.go
  - internal/director/execute/edit.go
  - internal/director/verify/edit.go
  - pkg/directorapi/action.go
---

# ADR-141 — finding text by what it says

## Context

Earned by a task, not scheduled:

```
"Add a short grocery list, then change milk to oat milk"
```

The last clause needed Marco to find the word "milk" in a document, and **nothing in Marco
could look for anything**. It observes what is in front of it and acts on that; there was no
notion of searching for something by its content.

What happened instead was the worst outcome this project tracks.

```
marco do "replace milk with oat milk"
COMPLETED: the value changed from "scratch\rmilk, eggs and bread"
                              to "oat milkscratch\rmilk, eggs and bread"
[result] performed
```

"Oat milk" was prepended. The word "milk" was never touched. Every step returned success and
the verdict claimed it.

The cause was one line:

```go
if _, after, found := strings.Cut(rest, " with "); found {
```

The before-part was discarded. The designed phrase was "replace THAT with hello", where "that"
is the selection; the code accepted any words in that slot and ignored them. So a request
naming specific text silently became a request about whatever happened to be selected — and
replacing an empty selection inserts at the caret.

The first fix refused it, honestly, naming the missing capability. This ADR is the second: the
capability, because the gap was the whole of what the task needed.

## Decision

**One operation, `edit.ReplaceText{Old, New}`, that searches the value the editor has already
read.**

It needs no new perception. The editor reads a control's value before writing one, for the
reason `AppendText` documents — *"appending requires READING the current value first, and a
planner that expressed it as SetText(old+new) would be making a decision about the old value at
planning time"*. Finding "milk" in a value already in hand is a string search.

**Three answers, and the third is the rule this project applies everywhere.**

| the search finds it | what happens |
|---|---|
| once | replace it |
| not at all | refuse, naming what was looked for |
| more than once | refuse, saying how many |

Ambiguity is not permission to guess. Replacing the first of three occurrences would change text
the person was not looking at, and "milk" in a grocery list is exactly the word that appears
twice. Measured live:

```
change butter to margarine   FAILED: "butter" does not appear in the text
change milk to oat milk      FAILED: "milk" appears 2 times, so there is no way to tell
                                     which one you mean
```

Both left the document byte-for-byte unchanged, checked by an oracle outside Marco's own
pipeline.

**Case-sensitive, and the person's own case is carried through.** "Jon" and "jon" are different
runs of text; matching either would change something nobody pointed at. The planner lower-cases
the phrase to read it and then recovers the original span from the raw input (`originalRun`), so
the search text is what was typed. Case-insensitive matching is a separate request nobody has
made.

**"Change A to B" is accepted alongside "replace A with B"**, because it is the phrasing the
task arrived in. It has a second honest reading — *set that control to that value*, as in
"change the font to Arial" — which nothing in Marco expresses today, so the phrase is unclaimed
and this reading takes it. What makes that acceptable is the shape of being wrong: "the font" is
not in the document, so the operation refuses **by name**. A guess that can only fail honestly
is not the kind of guess the rule forbids. When the other reading exists, the discriminator is
already in hand at execution time and is not a word list: a before-part that appears in the
focused control's value is text; one that names a control in the window is a control.

## This is the only edit whose result is known in advance

Every other arm of `verifyEdit` checks a weaker property — "ends with", "is empty", "changed at
all" — because the exact value depends on a caret, a selection, or an application's own undo
history, none of which the World Model observes. A replace by content depends on none of them:

```
after = before with the first occurrence of Find swapped for Text
```

So the verdict compares against a **computed string**, which is the strongest evidence this
package offers about anything. It is also the only check that catches the original defect: the
value changed, and it *contains* "oat milk", so every weaker property passes the prepended
document.

What is deliberately **not** checked is that the old text is gone. It is not — "oat milk"
contains "milk". A verdict built on the old run's absence would fail every replacement that
extends the word it replaces, which is the commonest kind there is.

An unobserved before-value is **inconclusive**, not failed. The replace may well have worked,
and `failed` is the word that invites somebody to ask again — on a document that now has oat
milk in it, where "milk" appears twice.
[[ADR-006-unknown-is-not-false]].

## Proven live, from the product entrance

An independent oracle read Notepad's document through its own UIA program, beside Marco's
verdict rather than from it.

```
clear the text editor                                      → 0 chars
add milk, eggs and bread, then change milk to oat milk
  [1/2] add milk, eggs and bread — verified
  [2/2] change milk to oat milk — verified
  COMPLETED: 2 of 2 steps verified
  (2 actions performed)
oracle                                                     → "oat milk, eggs and bread"
```

The commas are the person's. See [[ADR-142-the-splitter-keeps-no-vocabulary-of-its-own]] for why
they survived, and for the two defects in the same task that were not about replacing at all.

## What the task still cannot do, and why this ADR stops here

The user's own phrasing was *"add a short grocery list"*, and that is a request for Marco to
**compose** text. Measured:

```
add a short grocery list, then change milk to oat milk
  [1/2] add a short grocery list — verified
  [2/2] change milk to oat milk — failed: "milk" does not appear in the text
oracle → "a short grocery list"
```

Step 1 typed the description. The append verified, correctly, against the text it was given —
and the person did not get a grocery list. That is a false success at the step level, and it is
not a defect in replacing text: telling "add a short grocery list" (a description of content)
from "add milk, eggs and bread" (content) is a semantic judgement about the words, which is the
resolver's business and a capability Marco does not have. Recorded here as the next limiting
layer rather than guessed at.

## Enforced by

- `TestReplacingTextThatAppearsOnce`, `TestReplacingTextThatIsNotThereRefuses`,
  `TestReplacingTextThatAppearsTwiceRefuses`, `TestReplacingTextWithNoReadableValueRefuses`,
  `TestReplacingIsCaseSensitive` — the five answers, at the editor.
- `TestReplacingNamedTextIsASearch`, `TestReplacingTheSelectionStillWorks`,
  `TestTheSearchKeepsThePersonsOwnCase` — the parse, including that the deictic forms still
  mean the selection and that the person's case survives.
- `TestAReplacesSearchTextReachesTheAction` — the wiring, entered through `intent.New()` and
  `Planner.Build` rather than by filling in Parameters, because the search text has been
  dropped at three different hand-offs in this feature's short life.
- `TestAReplaceWithNothingToLookForIsRefused`, `TestAReplaceCarriesBothRuns` — the executor
  boundary, where a string vocabulary crosses into a real edit of somebody's text.
- `TestAReplaceThatProducedTheComputedValueVerifies`,
  `TestTheDefectDoesNotPassTheComputedCheck`,
  `TestAReplacementWhoseNewTextContainsTheOldOneStillVerifies`,
  `TestOnlyTheFirstOccurrenceIsExpectedToChange`,
  `TestAnUnobservedBeforeValueIsInconclusive`,
  `TestTextThatWasNotThereBeforehandIsAFailure` — the verdict, including the measured strings
  from the run that reported `performed`.
- `TestReplacingTextIsNotALowRiskEdit` — it overwrites the person's own words.

Mutation gate: the operation arm, the empty-search guard, the plan's threading, the intent's
parameter, the verdict's arm and the "change" phrasing were each deleted or inverted; all six
killed a test.
