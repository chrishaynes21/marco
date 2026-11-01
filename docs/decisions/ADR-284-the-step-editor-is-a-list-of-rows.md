---
type: adr
status: accepted
date: 2026-10-31
decision: "ACCEPTED — THE STEP EDITOR IS A LIST OF ROWS: NUMBER · ACTION · VALUE · CONTROLS. The editor listed each step as one sentence in a 28px/1fr/auto grid, under a fifty-word paragraph, so what a person scans — the verb, and the value it acts on — started in a different place on every line. The split needed NO backend change: the page already holds `stepVerbs`, a closed table of prefixes that `verbOf` opens a sentence on, guarded by `TestEveryStepSentenceTheBackendWritesHasAVerb` over `oneStep` for a step of every kind — it was used by the recording review's list and not by the editor's. The action column is FIXED at 7.5em and not `auto`, because each row is its own grid and `auto` would size every one of them to its own content, which is the alignment the change is for; a kind with no verb in the table keeps its whole sentence in the value column rather than losing the front of it to a prefix that happened to fit. AND A REMOVED ROW NO LONGER STRIKES OUT ITS OWN WARNING. `.steprows .gone .what{text-decoration:line-through}` reached `.pair` — the sentence saying that removing this row removes a SECOND step — because `.pair` is a child of `.what` and `text-decoration` propagates through the box tree and cannot be refused by the descendant. So the row said, in the typeface of a thing already deleted, that something else was about to be deleted: the one sentence on that row somebody has to read before they press, in the one state where it appears. The value moves into its own `.said` span and the rule moves with it. The paragraph above the list moves into Help, under `Editing what a routine does`, with a link to it beside the heading; what stays on the panel is what somebody is about to act on — the refusals, the pair warning, what has been asked so far, and what declaring a typed run costs. And every control on a row says WHICH ROW: nine buttons reading `Remove` and nine boxes holding a number are nine controls with the same name, and the step number is the half that tells them apart."
supersedes: []
affects:
  - execution
source_paths:
  - plugins/studio/frontend/index.html
  - plugins/studio/help.go
---

# The step editor is a list of rows

Slice 2 of [[Studio-alpha-UI-slice-2026-10-31]]. The owner:

> the edit screen gets a facelift + usabilty

## The shape

`number · action · value · controls`. One fixed column for the verb, one that takes the rest,
controls on the row.

Fixed and not `auto`, because **each row is its own grid**. `.sr` sets `display:grid` on the row,
not on the list, so an `auto` column is sized against that row's own content and the values start
in a different place on every line — which is exactly the thing being fixed. `Middle-click` is the
longest verb the table knows and 7.5em holds it.

The split needed no backend change. `stepVerbs` and `verbOf` were already in the page, already
guarded — `TestEveryStepSentenceTheBackendWritesHasAVerb` calls the real `oneStep` over a step of
every kind and asks the page's own table to open the result, including the kinds that have no
sentence and must therefore render WHOLE rather than be split by a prefix that happens to fit. The
table was used by the recording review's list and not by the editor's. Now both.

## The strikethrough that ate its own warning

A row asked to be removed gets `.gone`, and the rule was:

```css
.steprows .gone .what{text-decoration:line-through;color:var(--dim)}
```

`.pair` — *"removing this removes step 5 as well, because they are the two halves of holding
`shift`"* — is appended **inside** `.what`. `text-decoration` is propagated to descendants through
the box tree and **a descendant cannot refuse it**: `text-decoration:none` on the child does
nothing.

So the row said, in the typeface of a thing that has already been deleted, that something else was
about to be deleted. It is the one sentence on that row a person has to read before they press
Review, and it only appears in the state that struck it out.

The value now sits in its own `.said` span, the rule moves to `.said`, and `.pair` is a sibling of
it rather than a child.

This is not a mock-only defect. It was in the shipped page, and a screenshot of the proposal
reproduced it, which is how it was found.

## What moved, and what did not

Into Help (`Editing what a routine does`), with a link beside the heading:

> Change a wait, remove a step, or make the text a step types something you supply each time — all
> of that step's text, not part of it. The routine is written again from this list, so removing a
> press removes the whole of it, and the waits around it stay as they are until you change them
> too.

Staying on the panel, because it is what somebody is about to act on rather than an explanation of
the surface:

- the pair warning on its row
- `routineStepAsked` — what has been asked so far, before anything is sent
- the refusal line beside Review
- `typed_says` — what declaring a run of steps to be typed text COSTS, which is a
  destructive-change warning and arrives before the press

## Accessible names

Every control on a row carries the step number: `Remove step 3` / `Keep step 3`,
`Set the wait on step 2`, `Wait on step 2, in milliseconds`, `What to call the value step 6 asks
for`. Nine buttons reading `Remove` are nine controls with one name, and the row number is the
only thing that distinguishes them.

## What this does not do

- It does not change `StepRow`, `oneStep`, or anything else in the backend.
- It does not make any value on a row editable. That is slice 4.
- It does not touch the recording review's list, which is a different surface with a different
  shape.

## Enforced by

- `plugins/studio` `TestTheStepRowsAreCompactAndTheWarningStaysReadable` — **rendered in headless
  Edge**: the rows carry `four`; the action column reads `Bring/Wait/Hold/Left-click/Let go` and
  the value column holds none of those verbs; a removed row's value is struck out and the box
  CONTAINING the warning is not, and the warning is not a descendant of the struck element; the
  paragraph is gone from the panel and a link to Help is in its place; and the remove button and
  the wait box name their step, in both states of the button.
- `plugins/studio` `TestEveryStepSentenceTheBackendWritesHasAVerb` — the pre-existing guard that
  every sentence `oneStep` writes is one the page's table opens, and that a kind with no verb
  keeps its whole sentence.

## See also

- [[Studio-alpha-UI-slice-2026-10-31]]
- [[ADR-283-the-alpha-offers-one-journey-and-the-window-explains-itself]]
- [[ADR-273-a-routine-writes-down-what-it-does]]
