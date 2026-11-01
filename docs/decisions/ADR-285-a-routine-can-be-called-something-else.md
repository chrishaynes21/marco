---
type: adr
status: accepted
date: 2026-10-31
decision: "ACCEPTED — A ROUTINE CAN BE CALLED SOMETHING ELSE, AS A REVIEWED MOVE. Nothing renamed a routine before this, and almost nothing new was needed to: `Rescope` is a complete move engine — two locks in an order neither caller chooses, both ends revalidated against the digests the review was built from, an EXCLUSIVE create unless a version-bound consent says otherwise, the destination READ BACK before the original is removed, a destination written with the original still there reported as TWO ROUTINES rather than as a move, and the recording, the step account (rebound to the source the move writes) and the phrases all carried. A rename is that move with a destination differing by SLUG instead of by application or scope, so `PrepareRescope`'s body became `prepareMove` and `PrepareRename` is the second entrance to it. THE BOX EDITS THE NAME AND NOT WHAT THE ROUTINE ASKS FOR: the clause is shown beside it as a chip, read out of the SOURCE because a saved routine's header does not carry one — the declaration lives in the steps as `{{item}}` in the text of a step that types. A COLLISION IS REFUSED AND THERE IS NOTHING TO AGREE TO: two routines of one name in one place IS the collision and the answer is a different name, so the preparation turns an occupied destination into a refusal, `RenameRoutine` takes the token ALONE, and the page has no press that could consent. AND A ROUTINE THAT READS A CREDENTIAL UNDER ITS OWN NAME CANNOT BE RENAMED AT ALL. `codegen.typeSentence` writes `do OS's Secret with \"<route>:password\"` for a declared secret, where `<route>` is `fileBase(name)` — the routine's own name — and its own comment says what disagreeing about that costs: looking up a secret nobody stored. The check compares the credential names in the two SOURCES rather than deriving the qualifier, because deriving it would be a second copy of a rule in `codegen` and the two would disagree the first time either changed; a name present before and absent after is the defect, and an unqualified secret is identical in both and untouched. Nothing is read from the credential store and nothing is migrated into it. AND THE RENAME TESTS FOUND A DEFECT IN THE MOVE ENGINE THAT IS NOT ABOUT RENAMING: a move re-renders, `codegen` is told which argument names are declared, and an UNDECLARED `{{name}}` is not a plain placeholder to it — `splitSecrets` turns it into `do OS's Secret with \"name\"`, a read of the credential store. The declared set came from `routes.SplitArgs` of the NAME alone, and a saved header carries no clause, so every move of a routine that asked for a value produced one that asked the credential store for it instead, with the review showing that as the version it would write. Fixed in the shared path with `suppliedIn`, which is how every other re-render in the package reads it — including for `PrepareRescope`, because writing the fix only for rename would be writing the bug deliberately for the other caller."
supersedes: []
affects:
  - execution
source_paths:
  - internal/orchestrator/rename.go
  - internal/orchestrator/rescope.go
  - plugins/studio/rename.go
  - plugins/studio/routines.go
  - plugins/studio/frontend/index.html
---

# A routine can be called something else

Slice 3 of [[Studio-alpha-UI-slice-2026-10-31]].

## The engine was already here

`Deps.Rescope` is a complete move. Every guarantee a rename needs it already makes:

| | |
| --- | --- |
| two locks, in a fixed order | `lockOrder` by identity, so two movers ask for the same one first |
| both ends revalidated | the source, the recording, the step account and the aliases, re-read under the locks |
| the create is EXCLUSIVE | unless a version-bound consent says otherwise |
| the destination is read back | a write that returned no error is not a routine that is there |
| the original is removed LAST | and a failure there is reported as a COPY, not a move |
| the recording comes too | with the step account rebound to the source the move writes |
| the phrases come too | rebound to the new route, because a phrase is bound to an identity |

So `PrepareRescope`'s body is `prepareMove` now, and `PrepareRename` is the second entrance.
A move changes an identity, and an identity has two halves: where the routine works, and what it
is called.

## The three things a rename has of its own

### The arguments are a chip, not part of the box

The name is where an argument is declared — `routes.SplitArgs` reads `with item` off it — so a
box holding the whole phrase is a way to drop a value by retyping. The routine would then type
the literal text of the demonstration the next time it ran, with nothing said.

The box holds the base name. A `with …` typed into it is **refused** rather than merged: somebody
typing one means to change what the routine asks for, and a rename is not that operation.

The chip beside the box is read out of the **source**, because a saved routine's header does not
carry the clause at all. `Save` writes `type my address` even when the draft declared `item`; the
declaration lives in the steps, as `{{item}}` in the text of a `StepType`, and `routes.Arguments`
is what reads it. A chip built out of the name would say every routine in the library asks for
nothing.

### A collision is refused, and there is nothing to agree to

The move engine can replace what is at a destination under a consent bound to the version
somebody was shown. A rename offers no such press: two routines of one name in one place IS the
collision, and the answer to it is a different name.

So `PrepareRename` turns an occupied destination into a `Refused`, `RenameRoutine` takes the token
**alone** — not `MoveRoutine(token, false)`, because an argument that could carry a yes is an
argument a later change could start carrying — and the page draws no consent row.

### A credential that cannot follow the name

`codegen.typeSentence`:

```go
if seg.qualified {
    name = g.base + ":" + seg.text
}
return fmt.Sprintf("OS's Secret with \"%s\"", escape(name))
```

`g.base` is `fileBase(name)` — the routine's own name, slugged. The function's own comment says
what a disagreement about that costs: *"where disagreeing means looking up a secret nobody
stored."*

A rename re-renders, so the qualifier moves with the name and the routine goes to the credential
store under a name nothing has ever been saved under. The alternative — moving the credential —
is this code reaching into somebody's credential store, which it does not do. **So the case is
refused, by name.**

`secretsWouldMove` compares the credential names in the two sources rather than deriving the
qualifier. Deriving it would be a second copy of a rule that lives in `codegen`, and the two would
disagree the first time either changed — and disagreeing here means a rename that looks like it
worked and a routine that cannot sign in. A name present before and absent after is the defect;
an **unqualified** secret (a plain typed `{{password}}`) is identical in both and is untouched.

## The defect the rename tests found, which is not about renaming

A move re-renders, and `codegen.RouteAs` is told which argument names are declared.
`splitSecrets`:

> a numeric `{{N}}` or a declared PLAIN `{{name}}` stays in the literal (the run fills it); a
> declared SECRET-typed arg (`{{password}}`) becomes a route-qualified secret; **any other
> `{{name}}` is a global secret.**

The declared set came from `routes.SplitArgs(m.Name)` alone — and a saved routine's header carries
no clause. So the set was **empty for every saved routine**, and a move of one that asked for a
value rendered:

```
do OS's Secret with "item".
```

A read of the person's credential store, in place of a value they type. The move review showed
exactly that as the version it would write.

Fixed where both operations share the code — `dr.Args` gains `suppliedIn(dr.Steps, dr.Args)`,
which is what `declaredFor` does and how every other re-render in the package reads it, and which
leaves a name that MEANS a secret alone so a genuine credential stays a credential.

Fixed for `PrepareRescope` too, because writing the fix only for rename would be writing the bug
deliberately for the other caller.

## What this does not do, and one thing to rule on

- It does not offer to replace a routine that is in the way, in any form.
- It does not read, write or move anything in the credential store.
- It does not change the scope, the application, the recording, the step account or the phrases.
- It does not rename an Act, a Scene or a demonstration.

**A finding for the Architect, out of scope here — AND SINCE CORRECTED.** The secret qualifier
was lost on *every* re-render of a saved routine, not only a rename: a rescope and a step edit
both re-rendered with `declaredFor`, which leaves secret names out of the declared set, so a
routine holding `<slug>:password` came back holding `password`. A different credential. Rename
refused that case from this slice onwards; the other two performed it silently.

The Architect ruled it the next bounded correction, and it is made:
[[ADR-287-a-re-render-keeps-the-credential-it-was-reading]]. The identifier is now PRESERVED
through a step edit, a rebuild under today's rules and a move, and REFUSED where it cannot be —
a rename and a re-simplification. `secretsWouldMove`, which this slice wrote, is `secretsKept`
there, and the reader it needed is `codegen.SecretsIn` beside the composer.

## Enforced by

- `internal/orchestrator` `TestAMoveKeepsTheValuesTheRoutineAsksFor` — over a **rescope**,
  because that is the operation that has been shipping: the review and the moved file both carry
  `{{item}}` and neither reads the credential store.
- `internal/orchestrator` `TestAMoveKeepsTheCredentialItWasReading` — the other side of that fix.
  It was written for this slice under another name, in two halves: the RULE that a credential is never turned into text somebody types, and the
  FINDING below pinned as an assertion that would fail loudly the day it was fixed. It did. The
  test now holds the correction — see
  [[ADR-287-a-re-render-keeps-the-credential-it-was-reading]].
- `internal/orchestrator` `TestARenameOntoAnOccupiedNameIsRefusedWithNothingToAgreeTo` — `Taken`
  is cleared, so `ConsentToReplace` is empty and a consent a caller assembled by hand is still
  refused at the write; the routine in the way is byte for byte what it was.
- `internal/orchestrator` `TestARenameChangesTheSlugAndNothingElseAboutTheIdentity` — the
  application and the scope survive; the old file is gone and the new one calls itself by the new
  name.
- `internal/orchestrator` `TestARenameThatWouldMoveACredentialNameIsRefused` — refused at
  preparation AND at the write, with the routine unchanged.
- `internal/orchestrator` `TestARenameRefusesAClauseAndRefusesANameWithNothingInIt` — a `with …`,
  an empty name, a name with no letters or digits, and the name it already has in three
  spellings.
- `plugins/studio` `TestARoutineCanBeCalledSomethingElse` — against a real library: the scope
  survives, the demonstration comes with it, and nothing is left under the old slug.
- `plugins/studio` `TestARenameKeepsTheValuesTheRoutineAsksFor` — the clause is shown, the
  rendered version still has the placeholder before anything is saved, and the renamed routine's
  Play card still asks for the value.
- `plugins/studio` `TestARenameOntoAnExistingNameIsRefused` — both routines are exactly as they
  were, and nothing was offered to agree to.
- `plugins/studio` `TestARoutineThatReadsACredentialUnderItsOwnNameRefusesToBeRenamed` and
  `TestAClauseTypedIntoTheRenameBoxIsRefused` and
  `TestRenamingARoutineToItsOwnNameIsRefused`.
- `plugins/studio` `TestTheWindowRenamesARoutineAndKeepsWhatItAsksFor` — **rendered in headless
  Edge**: the box opens with the name in it, the chip shows what is kept, the review says what it
  would be called, there is no consent row, and the call that goes out is `RenameRoutine` with
  ONE argument — never the move call, which takes a second.
- `plugins/studio` `TestARefusedRenameSaysSoWhereTheAnswerIs` — rendered: a refusal with no
  review lands beside the box, a refusal with one lands in the panel, the button is dead and
  nothing was written.

## And it was unfindable where it was put

The owner, looking at this window the morning after it landed:

> there's aliasing, but no straight rename

It was there. It was a deed row below the step editor, below the box that adds another phrase a
routine answers to, and below the scope row — and somebody looking for a way to change a
routine's name meets the alias box first, which answers a neighbouring question convincingly
enough to end the search. [[ADR-289-the-panel-says-less-and-play-is-bigger]] moves the control
onto the identity row beside the name it changes. Nothing here changed: not what a rename
preserves, not what it refuses, not the review.

## See also

- [[Studio-alpha-UI-slice-2026-10-31]]
- [[ADR-271-a-rescope-is-a-reviewed-move-between-identities]] — the engine this is built on
- [[ADR-279-a-simplified-save-keeps-its-own-basis]] — the step account a move carries
