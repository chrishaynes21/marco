---
type: adr
status: accepted
date: 2026-11-04
decision: "ACCEPTED — STEPS CAN BE REPEATED, THROUGH THE SHARED PREVIEW/SAVE PATH. `orchestrator.EditRepeat` changes how many times a step that already repeats does: a key press by its count, or a repeated block. Half of a hold has no count. `EditLoop` repeats COMPLETE steps as one new block, reviewed on its own because it changes the list's length. It never splits a hold and never takes in the step that brings the window forward. It is refused in a routine whose presses are checked, asked of `codegen.Unpropagatable` so it cannot accept a block the render then refuses. Both are kept by the stored account, so the next edit and a rebuild under today's rules (which renders the account) preserve them. Studio offers a count box on a key-press or block row, and a block chooser, or the sentence saying why there is none."
supersedes: []
affects:
  - interface
source_paths:
  - internal/orchestrator/stepedit.go
  - plugins/studio/stepedit.go
  - plugins/studio/recording.go
  - plugins/studio/frontend/index.html
---

# Steps can be repeated

## The decision

- **A count, where a step already repeats.** A key press counts its presses; a repeated block
  counts its passes. The count runs from 1 to 999.
  - In a routine whose presses are checked, a repeated key still becomes that many separate
    checked presses: `codegen`'s `keyByKey` splits it, so each press can stop the routine.
  - **Half of a hold has no count.** A keydown repeated alone would press a key it never lets go
    of. A hold repeats only as part of a block.
- **A block, of complete steps.**
  - It is reviewed on its own, like a typed run, because it makes the list shorter.
  - Refused if it would split a hold. The halves are found by `holdPartner`, the function a
    removal and a key change use.
  - Refused if it would take in the step that brings the window forward.
  - Refused in a routine whose presses are checked. A repeated block cannot be wrapped so that
    one failed press stops the routine, which is why `codegen` refuses to generate one there.
    The check is asked of the generator's own rule.
- **Kept by everything after the save.** The block and the count live in the stored step
  account. The next edit reads them from it. A rebuild under today's rules renders the account,
  not the demonstration, so it keeps them too.
- **Studio:**
  - A key-press or block row carries a `×` count box, read when Review is pressed. Hold halves
    have none.
  - A block reads `Repeat | 2 times: press a; wait 50ms; …`, so what repeats is never hidden.
  - The block chooser (first step, last step, how many times) is its own button, and it waits
    for any other change instead of throwing it away.
  - In a routine whose presses are checked the chooser is replaced by the sentence saying why,
    and that a key press can still repeat by its count.

## Verified (Windows only)

- **Orchestrator:** each rule and refusal, and conflicts in either order. Through save, the
  production driver, the next edit and a rebuild under today's rules:
  - a checked routine's esc ×3 is three checked presses, and still three after the rebuild;
  - an unchecked routine's block ×2 runs twice, survives the rebuild, and its count can then be
    changed to 4;
  - a letter ×3 later declared typed text types three letters.
- **Studio:**
  - through the bindings: a count saved; a checked routine offered no block and told why; a
    block saved and listed as one row;
  - rendered: which rows carry a count; what is sent; the block waiting for other changes; the
    chooser offered or explained.
- **Mutants:** 14, all killed. Seven in the orchestrator:
  - a hold half repeatable;
  - a count never applied;
  - a block splitting a hold;
  - a block in a checked routine;
  - a block taking in the window coming forward;
  - a block sharing a review;
  - a removal ignoring a repeat.

  Seven in Studio:
  - the count dropped;
  - a block offering no count;
  - a checked routine offered blocks;
  - the page never sending a count;
  - a block not waiting for other changes;
  - the chooser shown where no block can exist;
  - a hold half offered a count.
- **Changed guard:** `TestEveryStepSentenceTheBackendWritesHasAVerb` held that a loop has no
  sentence and must render whole. A loop has a sentence now (`repeat N times: …`) and the page
  has its verb.

## Not established

- Not seen on the desktop. Rendered headless and run against a recording host.

## Enforced by

- `internal/orchestrator` `TestAKeyPressCanHappenMoreTimes`,
  `TestACountIsRefusedWhereItWouldNotBeACompleteStep`,
  `TestARepeatAndARemovalOfOneStepAreRefusedInEitherOrder`,
  `TestCompleteStepsCanBeRepeatedAsOneBlock`, `TestABlockIsRefusedWhereItWouldSplitOrBreakSomething`,
  `TestARepeatedKeyIsWhatTheRoutineDoesAndSurvivesARebuild`,
  `TestARepeatedBlockIsWhatTheRoutineDoesAndSurvivesARebuild`,
  `TestARepeatedLetterIsThatManyLettersWhenLaterDeclaredTyped`.
- `plugins/studio` `TestAKeyPressesCountIsChangedThroughTheStepPanel`,
  `TestAGuardedRoutineOffersNoBlockAndSaysWhy`, `TestStepsAreRepeatedAsABlockThroughTheStepPanel`,
  `TestTheStepPanelOffersACountAndABlockAndSendsThem`, `TestAGuardedRoutineIsToldWhyItHasNoBlock`.
