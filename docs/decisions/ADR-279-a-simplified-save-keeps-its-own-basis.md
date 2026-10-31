---
type: adr
status: accepted
date: 2026-10-31
decision: "ACCEPTED — A DRAFT THAT WAS RENDERED UNDER RULES A REBUILD DOES NOT USE CARRIES THAT FACT, AND SAVING IT WRITES THE STEP ACCOUNT BESIDE THE ROUTINE. The case is the simplified draft, and the dead end was in the middle of the one journey Studio exists for: the review card puts Simplify directly above Save, and the next thing it offers is editing the steps — which refused, with a sentence about the saved source and the demonstration no longer agreeing. Nobody had done anything wrong; they pressed the two buttons in the order they were offered. NO ACCOUNT MEANS RENDER THE DEMONSTRATION, which is the inference `routes.StepsNone` licenses and which is true of every ordinary save by construction, because its source came from that same rendering — and FALSE of a simplified one, whose source was folded harder than a rebuild folds, so the two disagreed the moment it was written. The fix is at the save and not at the panel: `Draft.Basis` says which rules rendered these steps when they are not today's, `draftFor` sets it for `RebuildSimplifyTyping` alone, and `Deps.write` puts the account back after the source, the assets and the demonstration are on disk. THE SWEEP STAYS AND IS NOT WEAKENED: what lands in `write` is a rendering of a recording and an account from before describes a demonstration that is gone, so the old one goes and the new one is written immediately afterwards rather than an old one being allowed to survive. BOTH DIGESTS ARE READ BACK FROM WHERE THE SAVE PUT THEM, never computed from what went in, or one representation change in `recorder` apart the account reads `adrift` the instant it is written. AND A FAILED ACCOUNT IS NOT A FAILED SAVE: the routine is on disk and runs, because execution reads only the `.marco`, so `ErrStepsNotWritten` — which until now meant a step edit — is the answer, and every surface that saves a draft tells it apart from a refusal. Returned as a refusal it was actively wrong: Studio's card would have asked what was in the way and offered to replace the routine this save had just written, and `marco learn` would have asked the same question in words. THE DEMONSTRATION IS UNTOUCHED and a plain save still writes no account at all, which is what keeps the rule narrow: a save that wrote one for every draft would make the rebuild inference unreachable and would be a second, quieter claim about every routine already in somebody's library."
supersedes: []
affects:
  - execution
source_paths:
  - internal/orchestrator/draft.go
  - internal/orchestrator/stepedit.go
  - internal/orchestrator/orchestrator.go
  - plugins/studio/replace.go
  - plugins/studio/recording.go
---

# A simplified save keeps its own basis

## The dead end

Studio's review card offers two buttons, in this order: **Simplify…**, then **Save**. The routine
panel it drops you on afterwards offers **Edit its steps**. Pressing all three in the order they
are offered produced this:

> *"say hello"* cannot be edited: its saved source is not what its own demonstration renders to
> under today's rules, so what the file does and what was demonstrated no longer agree — reviewing
> a rebuild is where that is decided

That sentence is accurate, it is about file integrity, and it reads like damage. The person did
nothing unusual. It was reachable in three clicks from a fresh recording.

## Why it happened

A routine with **no step account** beside it is rendered from its demonstration, and the result is
treated as what the file is. That inference is the whole of how step editing works for the routines
already in people's libraries, and it is sound — for an ordinary save. An ordinary save's source
*came from* that same rendering, so re-running it reproduces the file exactly.

A simplified save's did not. `RebuildSimplifyTyping` folds a typed run harder than the default
rules fold it, so the source on disk is one the demonstration does not render to. `basisFor` found
the disagreement, correctly, and described it the only way it could from there: the file and the
demonstration no longer agree.

So the two features were each right and the pair of them was a trap. It had been noted and left:
*TestASimplifiedSaveCannotBeStepEditedYet* recorded the limitation and said to delete itself when
somebody fixed it. The reason given for leaving it — *the fix is a change to the save path every
surface shares* — was true, and it is the change made here.

## The decision

**A draft says what rules rendered it, when those are not the rules a rebuild would use, and the
save writes that down.**

- `Draft.Basis` is empty for an ordinary recording and holds the rebuild kind otherwise.
  `draftFor` sets it for `RebuildSimplifyTyping` and nothing else: `RebuildCurrent` renders the
  demonstration under today's rules, which is exactly what a reader with no account does, so it
  needs none.
- `Deps.write` sweeps any account at the route, as it always has, and then calls `accountFor`,
  which writes one when the draft has a basis. The sweep is **not** weakened to let an old account
  survive — an account from before describes a demonstration that is gone, and the question "is
  the old one still right" is one nothing on disk can answer. The new one is written immediately
  afterwards instead.
- The account is written **last**, after the source, the assets and the demonstration, because the
  digests it records describe files that have to exist first. Both digests are **read back from
  where the save put them** rather than computed from what went in. `Deps.Resave` already does
  this and for the same reason: one representation change inside `recorder` apart, an account bound
  to the bytes that went in is bound to bytes nobody read out, and it would read `adrift` the
  instant it was written.

## A failed account is a third outcome, not a failed save

The routine is on disk and it runs — execution reads only the `.marco`. What is missing is the
explanation beside it, so the next step edit will refuse until the routine has been rebuilt.

`orchestrator.ErrStepsNotWritten` already existed for exactly this, from the step-edit path, and it
now has two writers. Its own sentence says only *its steps were not written down beside it*; each
caller supplies the clause naming what **was** saved.

Reporting it as a refusal was not a cosmetic mistake. Both surfaces answer a refused save by asking
what is already there and offering to replace it — and what is already there, after this
particular failure, is the routine the save just wrote. Studio would have offered to replace it;
`marco learn` would have asked the same question in words. So:

- `plugins/studio` carries it as `Recording.Partial`, drawn as a banner on the review card beside
  a routine the same card reports as saved, and written into the trial journal with *rebuild it
  from its demonstration before editing its steps* as what to do next.
- `keepWhatWasShown` says `Learned`, prints the caveat, and returns — before the collision
  question it would otherwise have reached.

## What this does not change

- **The demonstration.** Stored exactly as recorded, as the only account of what somebody did.
  Simplification remains a re-derivation from it and never a transformation of it.
- **A plain save.** It writes no account, and the routine is still edited by rebuilding from its
  own demonstration. This is what keeps the rule narrow: a save that wrote an account for every
  draft would make that inference unreachable and would be a second, quieter claim about every
  routine already in somebody's library. `TestAPlainSaveIsEditedFromItsDemonstration` holds it,
  reading the registry directly, because *it can be edited* is true of both answers and only one of
  them is this.
- **The workaround nobody should need.** Saving as recorded, then simplifying from the routine's
  own card and replacing, always produced an editable routine, because a replacement from a rebuild
  writes the account. That ordering is no longer the only one that works and is not documented as
  advice.

## Enforced by

- `plugins/studio` `TestASimplifiedSaveCanBeStepEditedAndSavedAgain` — the whole journey through
  the window's own entrances, in the order the cards offer them: `SimplifyDraft`, `SaveDraft`, a
  save asserted **whole** (`Recording.Partial` empty), `StepEditing`, the listed steps asserted to
  be the SIMPLIFIED ones rather than a rebuild's five keystrokes, `ReviewStepEdit` turning the
  typed step into a supplied argument, `SaveStepEdit`, and the routine editable **again**
  afterwards. It is the renamed *TestASimplifiedSaveCannotBeStepEditedYet*, which asserted
  the refusal.
- `plugins/studio` `TestAPlainSaveIsEditedFromItsDemonstration` — no account written, `StepsNone`
  read off the registry, and the routine still editable.
- Two mutations, both killed: `accountFor` returning early for every draft fails the journey test
  with the exact refusal this ADR is about, and writing an account for every draft fails the plain
  save's.

## See also

- [[ADR-278-the-text-a-step-types-can-be-made-something-you-supply]] — the panel this dead end
  blocked, and where the limitation was first written down.
- [[ADR-266-one-set-of-rules-for-keeping-a-recording]] — `Keep`, which this save path runs under.
- [[Studio-journey-discoverability-2026-10-31]] — the audit that found it in the middle of the
  journey rather than as a note in a test file.
