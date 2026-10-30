---
type: reference
status: accepted
date: 2026-10-30
source_paths:
  - cmd/marco/edit.go
  - internal/codegen/codegen.go
  - internal/orchestrator/regenerate.go
  - internal/orchestrator/rescope.go
  - internal/orchestrator/draft.go
  - internal/routes/registry.go
  - internal/routes/origin.go
  - internal/simplify/simplify.go
  - plugins/studio/replace.go
  - plugins/studio/generation.go
---

# Editing a step, and what that costs the demonstration

Traced at `3de2ffc`. The queued first delivery is four lines:

- change a wait;
- remove an accidental step;
- review the resulting behaviour;
- save through version-bound replacement, or Cancel without writing.

This is what exists, what each of those four actually needs, the three questions that have to be
resolved before any of it is built, and the smallest behaviour that closes the gap.

**The backend is now built.** This note is the trace it came out of and the rules it was built to;
what was built, and what holds each rule in place, is
[[ADR-273-a-routine-writes-down-what-it-does]]. Two rules moved while it was being built and say so
below. No live execution and no acceptance run were taken.

**Revision 3** is what is being built. Revision 2 answered four corrections and two test
adjustments; revision 3 answers three more. Where each landed:

| the correction | where it is |
| --- | --- |
| an already-edited routine must stay editable | **R2**, and the account's definition in §5 |
| bind the steps file to the source it describes | **R7** (version, digests, ordering, crash) and **R11** (token, revalidation) |
| account for every operation that touches it | **R12** and the lifecycle table in §6 |
| define "edits discarded" precisely | **R8** |
| removing a chain's last action regenerates, it does not refuse | **R5** and **R10**, and the two tests in §7 |
| fake-host execution tests | §7, *What runs* |
| a repair may not discard an account it cannot read | **R13**, and the repair row of §6 |
| no durable editing history | the account in §5, and §8 |
| describe a mismatch neutrally | the state table in §5 |
| a move verifies the account before removing the original | the move row of §6 |

## 1. A step editor already exists, and it edits SOURCE LINES

`cmd/marco/edit.go` is the control centre's Edit view, reached by `marco edit "<name>"` and filed
under **Advanced** on purpose ([[Plays]], and the audit that moved it there). It is a real editor
and two of its operations are the two this contract asks for:

| | what it is | where |
| --- | --- | --- |
| the rows | `parseSteps()` — a regex walk of the source, one row per `do …` line at the body indent | `edit.go:953` |
| change a wait | `saveReq.Waits`, keyed **line index → ms** | `edit.go:1150` |
| remove a step | `saveReq.Deletes`, a list of **line indexes** | `edit.go:1154` |
| the gate | `driver.CheckSource(updated)` — the one compile gate, [[ADR-005-legal-marco-only]] | `edit.go:1204` |
| the write | `Registry.Save` — `os.WriteFile`, no lock, no digest, no review | `registry.go:161` |

So the contract's first two verbs exist in one surface, and the second two do not exist anywhere
near them: a save is immediate, it compares nothing, and it takes no lock. That is the same write
`Deps.Replace`'s own note names as the hazard it was built against — *"a person can edit the
routine in `marco edit` … while the review is open"* (`regenerate.go:28`).

### And its remove is ALREADY KNOWN TO BE BROKEN for a generated routine

This is not a prediction. `cmd/marco/editsave_test.go` carries the case, in its own words:

> the step editor renders `do OS's Find with p1...` as a single row labelled "Find target, then
> click", with a ✕ beside it, and deleting a Find header does **NOT** cascade to its arms the way
> a repeat header does. So one click on a button the page offers leaves `when ok?` with nothing to
> be an arm of.

`TestASaveThatWouldNotCompileIsRefused` holds the outcome: the compile gate refuses, the file is
untouched, the person's work is kept. **Refused is the right answer and it is not the asked-for
behaviour** — and that shape is not an edge case. It is what every press in a current-generation
routine looks like.

## 2. What a "step" is in a generated routine, in the two shapes

`codegen.RouteAs` writes one of two shapes, decided by `chained := guardsAnything(steps)`
(`codegen.go:138`) — whether any press asked to be checked against the window it was made in
([[ADR-250-the-recorder-runs-in-the-window]]).

**Unguarded** (every legacy routine): one `this's Run does...` body, steps emitted flat at depth 1.
An anchored click is still a multi-line tree — the `Anchor` declaration, `do OS's Find with a1...`,
`when ok?` → click the scored location, `or?` → the text locator, then vision, then the recorded
coordinate (`emitAnchoredClick`, `codegen.go:566`). One step, up to a dozen lines, three indents.

**Chained** (what a measured demonstration saves as today): each press is **its own capability** —
`this can Step3.` / `this's Step3 does...` — and the next one is called inside the previous one's
`when ok?`, with `this is that!` adopting its answer. The single `this is ok!` sits inside the
success of the LAST link, and the trailing settle wait with it (`writeChain`, `codegen.go:1200`).

Removing one press from the chained shape therefore means: delete that capability's whole block,
repoint the previous link's `do Actor's StepN.`, and — if it was the last — move the trailing wait
and the one `this is ok!` into the link that is now last. **A shorter chain is still a chain**, and
that re-render is the ordinary case rather than a refusal; what it costs is that it cannot be done
by deleting lines. Removing one press from the unguarded shape means deleting a three-deep tree and
leaving its neighbours alone.

**Neither is line surgery.** A wait is one line in both shapes (`do OS's Sleep with N.`) and is the
only operation the existing editor can do correctly on a generated routine.

## 3. The version-bound machinery is already built, and is not the missing part

`internal/orchestrator/regenerate.go` is SELECT → PREPARE → REVIEW → REPLACE, and every rule the
fourth contract line asks for is already there:

- `Prepare(rt, kind)` renders the routine again from its own stored demonstration and **writes
  nothing** — the result is held in memory with a digest of each of the two files it was built from.
- `Prepared.Token` is derived from the content, so two preparations of one routine cannot be
  confused for each other; `Prepared.library` makes the store part of the target.
- `Replace(p)` takes the route's cross-process lock **before the first read**, re-reads and
  re-digests the source and the recording, refuses if either moved and names which, writes the
  assets first because they are additive, and lands the source atomically through
  `Registry.Replace`.
- Cancel is the absence of an operation, because nothing was written.

Studio's panel over it (`plugins/studio/generation.go`) already holds the review backend-side and
hands the page nothing but an identity. **The missing behaviour is not the save. It is everything
before it** — plus one field: `sealed()` joins the two digests it was built from
(`regenerate.go:439`), and a third has to join them (R11).

## 4. The three questions, resolved

### 4.1 How an edit preserves the original demonstration

By never writing it. `.rec.json` is the only durable account of what the person did, and `Replace`
already touches the source and the assets and nothing else. An edit is a decision about **what
Marco will do**, and the demonstration is the record of **what somebody showed it**; a surface that
rewrote the second to express the first would be editing somebody's afternoon.

That is a rule with a test, not a property of the current code path: `SaveRecording` exists and is
one call away from any writer.

### 4.2 How dependent gestures stay valid

By expressing the edit over the IR, where the dependencies are visible, instead of over the text,
where they are not. `macroir.Step` already makes most gestures **one step**: a drag is one
`StepDrag` carrying both edges and one window rectangle ([[ADR-259-a-guarded-drag-is-two-points-in-one-window]]),
a typed run is one `StepType` ([[ADR-261-typed-text-is-a-sequence-of-submissions]]), an anchored
click is one `StepClick` with its `Template`, and a folded repetition is one `StepLoop` with its
body nested inside it.

Two cases are **not** one step and are the whole of this question:

- **A held key** is a `StepKeyDown` and a later `StepKeyUp` ([[ADR-258-a-chord-is-a-promise-to-let-go]]).
  Removing either half alone leaves a routine that holds a key it never releases, or releases one
  it never held. So a removal resolves the pair and takes **both**, and says so; a pair it cannot
  resolve is **refused**, never guessed.
- **A chain link's position.** In the chained shape the links are ordered by construction, so
  removing one is a re-render and not a repointing — including the last one, which hands the
  trailing wait and the single `this is ok!` to the link that is now last.

### 4.3 What a later Simplify or rebuild does to an edited routine

Today: **it reverts the edit, and the card's sentence implies the opposite.** The rebuild renders
the demonstration, which never contained the edit, so `Prepared.Changes()` is true and the panel
offers a replacement under a generation difference — while what it would actually write is the
person's own change, undone. The left-hand pane shows them their text, which is the protection
`regenerate.go` claims for a hand-edited routine, and the sentence beside it is still about
generations.

It is worse than it looks, and the reason is worth stating plainly: **Marco cannot tell an edited
routine from an unedited one.** A demonstrated routine gets no `.origin.json` — nothing writes
`routes.KindTaught`, and `plays.taughtIfRecorded` promotes from the presence of a recording and
never from provenance (`plays.go:415`) — so there is no durable digest of "the source as
generated". The only available answer is computed: render the demonstration and compare. That
answer means "hand-edited" **only** when the stated generation is the current one; for an older
generation, `Now != Was` for reasons that have nothing to do with anybody's edit.

So the resolution has to be durable, and it is the **account** below: the routine's step list
written down beside it and bound to the two files it describes. A generation rebuild renders the
account; a re-simplification from the demonstration is a separate choice that discards it and says
so in the words the Architect gave.

## 5. The smallest missing behaviour

Twelve rules. R1 to R9 keep the numbers they had in revision 1; R10 to R12 are new. The ones I
considered and dropped are in §8.

### The account

One file, `<slug>.steps.json`, and one definition the rest of these rules lean on:

```json
{
  "version": 1,
  "source": "<16 hex>",
  "demonstration": "<16 hex>",
  "steps": [ "… the IR …" ]
}
```

**`steps` is the routine's account of itself** — what a render produces the saved source from.
`source` and `demonstration` are `routes.DigestOf` over the two files this account describes. Four
fields, and **no editing history**: what the routine does now is necessary, and a record of how it
got there is not. The operations somebody is applying are summarised in the review they are looking
at, in memory, and are gone when it is.

**And its presence means "the basis this source was rendered from is stored here" — not "somebody
edited this".** Two things write one: a step edit, and a re-simplification, which has no edit in it
at all (see R8). Nothing on disk tells them apart, so nothing in this design may say which, and a
surface that wants to tell somebody they edited a routine needs evidence this file does not
carry.

**The generation is deliberately NOT in it.** The source states its own
(`generationStated`, `generation.go`) and the account is bound to the source, so putting it here
would be a second record of one fact — the thing `ModeOf`'s note says not to do.

The account is read into **one of six answers**, the shape `StagedOrigin` already uses
(`origin.go:369`) and for the same reason:

| | what it means | what is offered |
| --- | --- | --- |
| **none** | no file — nothing has stored a basis, so the demonstration is the basis | editing, against the demonstration's render |
| **intact** | both digests match what is on disk | editing, against the account's render |
| **mismatched** | the source digest moved | nothing; the rebuild review, saying the source and the stored steps no longer match |
| **adrift** | the demonstration digest moved | nothing; the rebuild review, saying the stored steps are bound to a different demonstration |
| **orphaned** | no source | nothing |
| **unreadable** | unparseable, a version from the future, **or there and unable to be read** | nothing, and it says so rather than treating it as absent |

**Only `intact` is believed.** That is the whole of "a stale or partially updated account must
never silently replace the executable source's meaning": a half-written or outdated file cannot be
rendered from, because the digest it records is not the digest of the file in front of it.

**And a mismatch is reported as what it is, not as what it suggests.** A source digest that moved
is consistent with somebody editing the text by hand AND with an account write that failed after
the source landed (R7, row 2), and nothing on disk distinguishes them. So the sentence says the
source and the stored steps no longer match, and offers the rebuild review. It does not say
anybody edited anything.

**CORRECTED WHILE BUILDING IT: a read failure is not absence.** The first implementation answered
`none` to every error from reading the file, so a permission, a directory in its place or a handle
something else held all meant "nothing has stored a basis" — which is the single answer that
licenses rendering the demonstration instead and writing over whatever the file says. Absence is
`os.IsNotExist` and nothing else; everything else is `unreadable`, and every caller refuses. The
digest a review binds itself to carries that third answer too, because an unreadable file
answering `""` would compare equal to an absent one at the write.

**R1. An edit changes the STEP LIST, not the source text.** The two shapes in §2 are the reason:
a step's boundary is in the IR and only approximately in the file, and the one operation line
surgery gets right is the one the existing editor already does.

**R2. Step editing is offered only for a routine whose saved source is exactly what its own
CURRENT ACCOUNT renders today** — the stored steps when the account is `intact`, the demonstration
when it is `none`. So the first edit compares against the demonstration and every later one
compares against the stored steps, which is what keeps this from being a one-use editor: after a
save the source differs from the demonstration's render **on purpose**, and that difference is the
edit rather than a reason to refuse.

Everything else — `mismatched`, `adrift`, `unreadable` — is source nothing here explains, and it
goes to the rebuild review with the reason named. Without this precondition, changing one wait on a
generation-2 routine would rewrite the whole file under generation-4 rules, which is the review
flow's decision, made by somebody who was shown both sources, and not an edit.

**R3. Two edits, and they are the whole vocabulary.** Set a wait's duration; remove one step. No
adding, no reordering, no editing a coordinate, a key or a text.

**R4. A gesture is removed whole or not at all.** Over the IR, per §4.2: a removal that resolves a
held key's partner takes both and says so, and one that cannot resolve a pair is refused.

And a removal leaves the **waits** around what it removed. A wait is its own step, and deciding how
long to wait once a press has gone is the other operation — one somebody makes on purpose, looking
at the result. An editor that collapsed them would be choosing a duration nobody asked for.

**R5. The review writes nothing, and the proposed source passes the one gate before it can be
offered.** The panel shows the edit in a sentence, the current and proposed source **in full**
(both, as the rebuild review does — a list of changed lines is an interpretation), the proposed
step list, and any refusal. The gate is `driver.CheckSource` through a seam on `Deps`, because an
edited step list can produce a shape no recording produces. `codegen` is trusted today because its
input came from a recorder; after an edit it did not.

What the gate is **not** there to catch: a chain with its last link removed. That is a shorter
chain and it must render correctly (§2, and the test in §7) — the gate is the backstop for the
shapes nobody predicted, and a rule that leaned on it for a case this proposal already describes
would be a rule that shipped a refusal.

**R6. Saving goes through the version-bound path and nothing else.** The backend holds the review;
the page sends the token and nothing else. The digests are re-read under the route's lock, assets
first, source atomically next, the account last. Cancel drops the token and writes nothing.

**R7. The account is written beside the routine, bound to both files it describes, and written
LAST.** The ordering, and what each failure leaves:

| order | why | a crash here leaves |
| --- | --- | --- |
| 1. assets | additive; the source refers to them | images nothing yet points at — already the rule for a replacement |
| 2. source, atomically (`Registry.Replace`) | the executable truth | the edit in force with an account that no longer matches it (or none at all): further editing is refused and SAID, and execution is unaffected because a run reads only the `.marco` |
| 3. account, atomically | describes what 2 wrote, so it can only be written after it | nothing; the previous row is the state |

The account cannot be written first, because the digest it records is of the source that has not
been written yet. The failure in row 2 is therefore the one that matters, and it is the
conservative one: **the routine still runs, the editor refuses it, and the rebuild review is where
it goes** — the same place a hand-edited routine goes. A save that lands the source and not the
account says so, in the words `SaveWithOrigin` already uses for exactly this
(*"%s was saved but its provenance was not"*, `origin.go:162`), rather than letting the caller
assume the pair landed.

`writeAtomic` (`origin.go:171`) is the write, unchanged: a reader never sees a half-written file,
and a crash leaves either the old bytes or the new ones. Partial update is therefore not a state
this file can be in — only old, which the digests catch.

**R8. A rebuild renders the account; a re-simplification discards the edits and SAYS it.** Two
different offers, kept apart:

| the ask | renders from | what it does to an edit |
| --- | --- | --- |
| a newer generation's rules (`RebuildCurrent`) | the account | keeps it — the generator's rules reach an edited routine, and the account is rewritten bound to the new source |
| re-simplify the demonstration (`RebuildSimplifyTyping`, and any re-simplification) | `.rec.json` | **discards** it, says so, and writes an account of what it rendered in its place |

Two sentences, because they are two situations — and **neither says anybody edited anything**, for
the reason in §5: a stored basis comes from a step edit AND from a re-simplification, and a sentence
about "your step edits" would be a guess in one of the two cases.

| what it does | what it says |
| --- | --- |
| replaces the stored steps with a fresh rendering | *This rebuild replaces the stored steps with a fresh simplification of the demonstration.* |
| removes them and puts nothing back | *This rebuild removes the stored steps, which are no longer a description of this routine.* |

Beside either, the comparison the review already shows — the source as it stands and the source this
would write, both in full — which is where somebody sees exactly what they would lose. **No count
and no list**, because neither can be had honestly without durable history, and the comparison is
better evidence than either would be.

**CORRECTED WHILE BUILDING IT.** This rule said the re-simplification REMOVES the file. Doing that
left the routine permanently inexplicable: a re-simplification renders under rules that are not the
default, so its source is not what the demonstration produces today and never will be, and the next
edit refused it for a disagreement nobody had. So the discarded account is **replaced by an account
of what was just written** rather than removed. The edits are still gone; what is kept is the
file's own explanation of itself. A rebuild under the ordinary rules writes none, because for that
routine the demonstration is the account.

The only operation that REMOVES the file without putting one back is a rebuild over an account that
no longer describes the routine — there is nothing to replace it with, and the demonstration is the
account again.

**R9. Residue is named.** A removed anchored click leaves its image on disk; the outcome says so,
as a rescope's does ([[ADR-271-a-rescope-is-a-reviewed-move-between-identities]]).

**R10. An edit that would leave a routine with no steps is REFUSED, and says to delete the routine
instead.** The empty-routine policy, separate from the chain rule it used to be bundled with. A
routine that does nothing is not a smaller routine; deleting one is an operation that already
exists and already sweeps what sits beside it (`Registry.Delete`), and offering an edit that
quietly amounts to a delete would be offering it without any of that.

**R11b. An operation refers to the step list it was read off.** ADDED FOR THE SURFACE SLICE, and
it is a window the three digests do not cover: they catch the routine moving between the REVIEW and
the write, and this is the gap between a panel LISTING the steps and somebody stating a row number
against them. Another writer inserting or removing a step moves every position after it, and
nothing afterwards can catch it, because by then the operations have been applied to the wrong
steps. So a listing is handed a `Basis` — the three digests, and nothing else, because the step
list is a pure function of them — the surface sends it back with the operations, and a routine
written since is refused before anything is rendered. Studio refuses an operation that states no
basis at all, which is a rule of the surface because only the surface knows a list was on a screen.

**R11a. And the token binds the source the review SHOWED, not only a digest of the file it read.**
Found while reading the slice back and corrected across all three review types, because it is one
contract: `Was` is exported and mutable, and a digest taken at preparation is a stored value, so
rewriting the field left the token matching and the write went ahead having displayed something
nobody read off disk. `sealed` digests the field too. See
[[ADR-273-a-routine-writes-down-what-it-does]].

**R11. The account is in the token, and revalidated at the write.** One more field in
`sealed()` beside `sourceAt` and `recordingAt` (`regenerate.go:440`), and one more digest re-read
under the lock in `Replace`, refusing with which of the three moved. The reason is the one the
rescope note gives for the other two: the review was of a set of files, and any of them can move
while somebody reads it — and an account that moved means another edit of the same routine landed,
which is the case `TestTwoEditsOfOneRoutineAreTwoReviews` is about.

**R12. Every operation that touches a routine states what it does to the account.** The table in
§6. Nothing is left to a later slice, because what is left to a later slice is a conflict that
ships: a move that regenerated from the demonstration would silently undo somebody's edit, and it
would do it through a surface whose whole job is to show them what will be written.

**R13. A repair may not discard an account it cannot read.** `Resave` renders the account when it
is `intact` and proceeds from the demonstration when there is `none`. For `mismatched`, `adrift`,
`unreadable` or `orphaned` it **refuses the automatic repair** and offers the rebuild review
instead. A routine being unrunnable is not evidence that somebody's edits are disposable, and
`Resave` is the one path that writes without anybody consenting to anything (`draft.go:757`) — so
it is the one path where "proceed and mention it" would be a silent loss.

### Where it lives

Studio, mirroring the Rescope surface exactly: the contract and its rules in
`internal/orchestrator`, the panel in `plugins/studio/edit.go` holding one review per routine
keyed by token, the page sending `(token)` and nothing it could author itself.

## 6. Everything that touches a routine, and what it does to the account

| the operation | the code | the account |
| --- | --- | --- |
| **an edit's save** | the new path | written last, bound to the source it just wrote (R7). |
| **a move** (rescope) | `Deps.PrepareRescope` / `Rescope` | **renders the destination from the account**, not the demonstration, when it is `intact`; the account travels, rebound to the source the move wrote. Its demonstration digest is unchanged, because `Rescope` writes the same recording beside it (`rescope.go:446`). This is the change correction 3 names: today `PrepareRescope` renders from the recording, so moving an edited routine would lose the edit. **The account is part of reading the destination back**, beside the source and the demonstration, and the original is not removed until it is there — an account that did not arrive makes the move a COPY and says so, exactly as a demonstration that did not arrive already does (`rescope.go:446`). The review still shows `Was` and `Now` in full, so a **hand** edit still costs exactly what the rescope note says it costs: there is no third thing that would not be inventing source nobody wrote. An edit made through this editor is no longer that case, because the account is what nobody had to invent. |
| **a repair** (resave) | `Deps.Resave` | renders the account under current rules, so a repair keeps the edits, and rewrites it bound to the repaired source. With `none` it renders the demonstration, as today. With anything else it **refuses** and offers the rebuild review (R13): it is immediate and consents to nothing (`draft.go:757`), so proceeding would be the one silent loss in this table. |
| **a generation rebuild** | `Deps.Prepare(RebuildCurrent)` + `Replace` | renders the account; rewrites it (R8). |
| **a re-simplification** | `Deps.Prepare(RebuildSimplifyTyping)` + `Replace` | discards it, says the sentence beside the source comparison, and writes an account of what it rendered in its place (R8). |
| **recording the routine again** | `Deps.Keep` → `Registry.Replace` + `SaveRecording` | **removed.** The account describes a demonstration that is gone, and the consent sentence says the step edits go with it. This is the one place where losing them is right: the person demonstrated the thing again. |
| **a rename** | `Registry.Rename` | travels, beside the source, the recording, `.origin.json` and the anchor images — the argument `Rename`'s own comment already makes for the provenance sidecar, unchanged: *"the digest is over the source, which renaming does not change"* (`registry.go:176`). Leaving it behind would do the two wrongs that comment names, in the same order. |
| **a delete** | `Registry.Delete` | removed, with the recording and the images. |
| **a move's removal** | `Registry.DeleteSource` then `Registry.DeleteLeftovers` | removed by the second, and **reported as residue** if it cannot be: an account left under a dead slug is a file the next routine saved there would inherit. |
| **`marco edit`'s save** | `Registry.Save` | **UNCHANGED, and this is the statement of that.** It writes the source and nothing else, so the account's source digest stops matching and the account reads `mismatched`: the routine still runs, step editing refuses it with the reason, and the rebuild review is where it goes. Nothing is corrupted and nothing is silently believed — but the person's step edits are no longer the explanation of their file, and `marco edit` does not tell them so. That is the cost of leaving it out of this slice, stated rather than discovered. |

One existing gap, noted because the account must not repeat it: **`Registry.Delete` does not remove
`.origin.json`** (`registry.go:211`), and neither does `DeleteLeftovers`. That is survivable
because `Origin` refuses the inheritance by digest — the mechanism `Rename`'s comment names — and
the account is bound the same way, so an orphan of it is harmless too. The account is removed
anyway: **the digest is the backstop, removal is the intent.**

## 7. The tests

Headless, production-path, no screen. Each one named for what it holds.

### The contract (`internal/orchestrator`)

**The demonstration**

- `TestAnEditedRoutineKeepsItsDemonstrationByteForByte` — the `.rec.json` digest before and after.

**What may be edited** (correction 1)

- `TestAnEditIsOfferedAgainstTheDemonstrationWhenThereIsNoAccount` — the first edit.
- `TestAnAlreadyEditedRoutineCanBeEditedAgain` — the second edit, compared against the stored
  steps; it is the test that would fail under revision 1's R2.
- `TestAnEditIsRefusedWhenTheSourceIsNotTheRenderOfItsAccount` — `mismatched`, `adrift` and an
  older generation, each with the sentence that names which, and each pointing at the rebuild
  review.
- `TestAMismatchIsNotReportedAsAHandEdit` — the neutral wording, because a moved source digest is
  equally consistent with a failed account write.
- `TestAnUnreadableAccountIsSaidAndNotTreatedAsAbsent`, and
  `TestAnUnreadableAccountStopsAMoveBeingPrepared`.

**What a removal means**

- `TestARemovedHoldTakesBothHalvesOrNothing` — and the refusal when the pair cannot be resolved.
- `TestARemovedStepIsRemovedWholeInBothShapes` — the chained routine and the unguarded one, each
  with an anchored click, each re-rendered and compiled.
- `TestRemovingTheLastLinkOfAChainRendersAShorterChain` — **not a refusal** (adjustment 1): a
  three-link chain becomes two, the trailing settle wait and the single `this is ok!` sit in the
  link that is now last, and it compiles.
- `TestAnEditThatWouldEmptyARoutineIsRefusedAndSaysToDeleteItInstead` — R10, the empty-routine
  policy on its own.
- `TestAnEditedRenderThatIsNotLegalMarcoIsRefusedBeforeAnythingIsWritten` — the backstop, with a
  stubbed `CheckSource` refusal through the `Deps` seam, because the point is that the gate is
  consulted and not that a particular shape fails it.

**What runs** (adjustment 2) — `driver.RunFileWithHosts` with the `recordingHost` the package
already has (`authority_test.go:40`), which records `Act's Action with Input` per call

- `TestAChangedWaitIsTheWaitThatRuns` — `OS's Sleep with 800` among the calls, and the recorded
  duration nowhere in them.
- `TestAnEditedRoutinePressesWhatIsLeftInOrder` — the remaining actions, in their order, and the
  removed one absent. The assertion is the call list, not the source text: the source is what
  §5 R5 already shows somebody, and this is the other question.
- `TestAFailedStepStopsWhatFollowsInAnEditedRoutine` — a host that answers something other than
  `ok` at the second press; nothing after it is called. The chained shape's `when ok?` is the
  mechanism, and this is what says a re-render did not flatten it.

**The write**

- `TestTheAccountIsWrittenAfterTheSourceAndNeverBefore` — the order in R7, observed.
- `TestASourceWrittenWithoutItsAccountStillRunsAndRefusesTheNextEdit` — the crash in row 2, staged
  by failing the account write: the routine runs through the ordinary driver, and editing refuses.
- `TestAnEditIsRefusedWhenTheWorldMovedWhileItWasOpen` — all three digests, one case each: the
  source, the demonstration, and the account on its own. The third is the case only the third
  digest can see, because a whole second edit moves the source too and the source check would
  answer first and say the wrong thing about it. `TestAnEditRevalidatesInsideTheLock` observes
  that the route's lock cannot be taken in the instant between the comparison and the write
  (R11).
- `TestAnEditCannotBeSavedTwice` — the token is spent, and the refusal comes from the layer that
  writes.
- `TestTwoEditsOfOneRoutineAreTwoReviews` — the `Prepared.Token` lesson: a slot keyed by routine
  writes whichever arrived last. Plus a review assembled from two, and one saved into another
  library.
- `TestAReviewsTokenCoversWhatTheRoutineSaysAboutItself` — **added because a mutation survived**:
  the account's digest in either token could be deleted with nothing noticing, because every
  writer moves the source too and the source digest answers a moment later. The case is built by
  moving the account's BYTES and not its meaning.
- `TestTheStepListAReviewHandsOutIsADeepCopy` — a caller cannot change what a sealed review says it
  is, **including through the two references a step carries**: `Template`, the picture an anchored
  click looks for, and `Steps`, a repetition's body. The first version copied the outer slice only,
  which looked exactly like a copy and defended neither.
- `TestAnAccountThatCannotBeReadIsNotTakenForAnAbsentOne` — a read failure refuses an edit and a
  rebuild, with a genuinely absent account as the control in the same test.
- `TestAReplacementIsRefusedWhenTheStoredStepsMovedWhileItWasOpen` and
  `TestWhatAReplacementWouldDoToTheStoredStepsIsPartOfWhatWasApproved` — the third digest at the
  rebuild end, and the integrity of the two unexported fields a replacement branches on.
- `TestCancellingAnEditWritesNothing` — source, recording, assets and the account, all four.

**The lifecycle** (correction 3) — one per row of §6

- `TestAMoveOfAnEditedRoutineKeepsItsEdits`, and `TestAMoveOfAHandEditedRoutineStillShowsTheLoss`
  — the two halves of the rescope row, so the new behaviour cannot be read as weakening the old
  warning. `TestAMoveIsRefusedWhenTheRoutineWasEditedWhileItWasOpen` is the third digest at the
  other end.
- `TestARepairOfAnEditedRoutineKeepsItsEdits`, and
  `TestARepairIsRefusedWhenTheAccountCannotBeRead` — R13, one case per state.
- `TestAMoveVerifiesTheAccountAtTheDestinationBeforeRemovingTheOriginal`, and the COPY it reports
  when the account did not arrive.
- `TestARebuildOfAnEditedRoutineKeepsTheEdits`.
- `TestAReSimplificationSaysItDiscardsTheStepEdits` — the sentence verbatim beside the source
  comparison, no count and no list, the edit gone from the account, and the routine still editable
  against what was written.
- `TestARebuildOverAnAccountThatNoLongerDescribesTheRoutineSweepsIt` — the one case where the file
  is removed and nothing replaces it.
- `TestRecordingARoutineAgainRemovesItsAccountAndSaysSo`.
- `TestRenamingAnEditedRoutineCarriesItsAccount` — beside `TestRenamingAPlayCarriesItsPast`, which
  is the test `Rename`'s comment says must fail if the provenance rename is deleted.
- `TestDeletingARoutineTakesItsAccount`, and
  `TestAMovesRemovalTakesTheAccountOrReportsIt`.

### `marco edit` (`cmd/marco`)

- `TestTheLegacyEditorsSaveLeavesTheAccountMismatchedAndTheRoutineRunnable` — the unchanged behaviour,
  pinned down where it lives so it cannot drift into something nobody decided. **`marco edit` is
  not changed by this slice**: it stays an Advanced source editor over its own line model, writing
  through `Registry.Save`. Routing it through `Replace` is a separate, smaller slice.

### The surface (`plugins/studio`)

Built in the second increment, in two halves: `stepeditacts_test.go` against a real library and
`stepeditpage_test.go` rendered against a stub. Named in full in
[[ADR-273-a-routine-writes-down-what-it-does]].

**And the gate**: a mutation run over every new call site, twice — once on the first
implementation and again after the corrections. See
[[ADR-273-a-routine-writes-down-what-it-does]] for the result and for the two tests a survivor
wrote.

### What these do NOT establish

That an edited routine presses anything **on a desktop**. The execution tests above run the real
engine through a recording host, which settles order, duration and the stop-on-failure chain; what
they do not settle is a real press at a real coordinate in a real window. That is a live trial, and
it belongs with the held physical-recording trial rather than inside this slice.

## 8. Not proposed

- **Adding or reordering steps.** A third and fourth verb, and the first two are enough to find out
  whether the shape is right.
- **Editing a coordinate, a key or a text.** The legacy editor does this; doing it here would mean
  deciding what a changed coordinate means for an anchor's picture, which is a question of its own.
- **Durable editing history.** The account says what the routine does, not how it came to. A
  journal was in revision 2 to let a discard sentence enumerate; the source comparison already
  shows exactly what would be lost, and a count could not be had honestly anyway — two
  operations on one wait are one change to read and two to count. What is summarised is the
  operations in the review being looked at, in memory.
- **Re-applying edits across a re-simplification.** It needs an edit anchored to something stable in
  the demonstration, and `internal/simplify` keeps no provenance from events to steps — `timedStep`
  holds a time and drops it. An index with a fingerprint beside it was the alternative; it refuses
  wherever the rules move, which is exactly when somebody wants the rebuild.
- **A reader from generated Marco back to the IR.** A third account of one routine, and the one most
  likely to disagree with the other two. Not requested, and not needed: the account is written by
  the thing that rendered the source.
- **A provenance framework.** The account is one sidecar with two digests, read through the state
  model `StagedOrigin` already has. No `Kind`, no `Origin` for demonstrated routines, nothing that
  claims to answer where a routine came from — it answers one question, which is what the saved
  source is the render of.
- **Changing `marco edit`.** See §6, where its unchanged behaviour and the cost of that are stated.
- **Any new Marco word.** The edit is a change to what is generated; the language says nothing new.
- **Touching the recording, the recording scope, or `Deps.Watch`.** Settled
  ([[ADR-268-record-only-the-chosen-window]]).

## 9. The cost, stated

`<slug>.steps.json` carries the anchor images a second time — they are `[]byte` on
`macroir.Step.Template`, and `.rec.json` already holds the same PNGs on the click events they came
from. A routine's library footprint roughly doubles once it has been edited. The alternative that
avoids the duplication is the amendment list in §8, and it buys the saving by refusing to apply an
edit whenever the simplification rules move.

The second cost is in §6's last row: until `marco edit` joins this, two surfaces can change one
routine and only one of them keeps the explanation.

## 10. The increments

1. **The backend** — BUILT. The account and its six-state read, the edit/review/save contract,
   R11's token and revalidation, and every row of §6's lifecycle, each through the production path
   rather than a stand-in.
2. **The surface** — BUILT. `plugins/studio/stepedit.go` and its panel, mirroring Rescope, plus
   R11b's basis, which the surface slice added to the contract.

Neither needed a screen: the tests run the real engine over the real generator through a recording
host, the lifecycle rows are all file operations, and the page is rendered headless against a stub.
What neither establishes is that an edited routine presses anything on a desktop.
