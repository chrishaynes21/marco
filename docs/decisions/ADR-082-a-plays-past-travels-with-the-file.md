---
type: decision
status: accepted
date: 2026-08-20
supersedes: []
affects:
  - plays
  - learned-plays
source_paths:
  - internal/routes/origin.go
  - internal/routes/registry.go
  - cmd/marco/edit.go
  - cmd/marco/assistant.go
---

# ADR-082 — a Play's past travels with the file

## What moving a play used to cost it

Two ordinary operations move a Play's `.marco`: **Rename**, and the scope control in the Plays tab
(`/api/scope`), which writes the source to the new scope and deletes the old one. Neither carried
the `.origin.json` beside it, and `Registry.Delete` does not remove one.

So changing a learned Play's scope did two wrongs at once:

- the moved Play arrived with no provenance and re-listed as **Authored** — somebody's own writing,
  where a moment before it had been something Director watched, rehearsed and verified;
- an orphaned sidecar stayed behind under the old scope, waiting for the next unrelated Play saved
  at that slug. `Origin` refuses that inheritance (`OriginOrphaned`), but not creating the orphan is
  better than refusing it later.

Nothing was visibly broken while nothing displayed provenance. The Plays surface now shows kind and
standing on every row, so it is visible the first time anybody touches the scope dropdown.

## The obvious fix is the wrong one

The natural way to write provenance at a new location is `SaveWithOrigin`, which is how a Play gets
one in the first place. It **recomputes the digest from the source it is handed**.

That turns a move into a re-verification. A Play the person had edited — honestly reading
*"you have changed this since Marco wrote it down"* — comes out of the move reading `ready`, because
the digest now matches the edited file. Marco would be re-vouching for an artifact it never
verified, silently, as a side effect of a scope change, with nothing on screen to say a fact had
been lost.

## The decision

**A Play's provenance moves with it, verbatim.** `Registry.MoveOrigin` reads the sidecar's bytes and
writes those bytes at the destination, then removes the source. It never rebuilds and never
recomputes.

- An **edited** Play stays edited. An **unreadable** sidecar stays unreadable: its state is a fact
  about this Play, not damage to be tidied away by a move.
- A Play with **no** sidecar has nothing to carry, and that is not an error — an ordinary authored
  Play moves as it always did.
- `MoveOrigin` runs **before** `Delete`, because `locDir` falls back to the legacy loose location
  only while the `.marco` is still there; the old sidecar has to be reached while the Play still is.
- `Registry.Rename` moves the sidecar for the same reason, in the same breath as the `.rec.json` and
  the anchor images. The digest is over the source, which renaming does not change, so the past
  still describes the file at its new name.

**Moving a Play is not an event in its history.** The sidecar records where the Play came from, and
where the file happens to sit is not part of that.

## Considered and rejected

- **Rewrite the sidecar at the destination with `SaveWithOrigin`.** This is the failure above: it
  re-verifies an edited Play. Rejected as the specific thing being prevented.
- **Refuse to move a Play whose provenance is not intact.** It punishes the person for having taken
  up the invitation to edit their own Play, and `edited` is explicitly not damage. The scope control
  is theirs.
- **Drop the sidecar on any move and let the Play list as Authored.** Truthful only about the
  destination; it destroys the audit trail for a learned Play on an operation nobody would expect to
  be destructive, and leaves the orphan behind anyway.
- **Move the sidecar lazily, on the next read.** A second mechanism to keep in sync with the first,
  for a rename that already touches four files.

## Consequences, including the costs

- Every future operation that relocates a Play's `.marco` has to carry the sidecar too. There are
  two today, both pinned by mutation-gated tests; a third would be a real chance to reintroduce this.
- `MoveOrigin` copies bytes it does not parse, so a sidecar this version cannot read is moved
  faithfully rather than rejected. That is deliberate — the reader (`Origin`) is the one place that
  judges a sidecar, and it already has a state for *unreadable*.
- A failed move can leave provenance written at the destination while the source `.marco` is still
  present; the handler surfaces the error rather than proceeding to `Delete`, so the Play is never
  lost, but the two halves are not transactional.

## And the same rule going the other way: forgetting one row forgets one row

The Plays surface shows a registered Play and a staged Play of the same name as **two rows**,
because they are two files with two different standings — and that is the *ordinary* position for a
staged Play, since `Register` refuses a name collision and leaves the saved copy where it is.

The first version of the forget action ran both rows through `Registry.Unregister`, because
Unregister is the door that removes a Play together with its provenance. But Unregister means
*"Marco no longer has this play at all"*: it removes the registered Play, its sidecar **and** any
staged copy behind the name. Through a two-row list that is the wrong verb in both directions —

- forgetting the **saved** row deleted the working Play of the same name, its recording included;
- forgetting the **registered** row destroyed the learned Play that was waiting — which is exactly
  what the registry's own refusal tells you to do (*"rename the learned play or remove the other one
  first"*), so following Marco's advice destroyed the artifact the advice was protecting.

Both returned `200` and said nothing.

So each row got a door onto its own files: `Registry.DeleteStaged` reaches `<app>/learned/` and
nowhere else, and the registered row takes `DeleteOrigin` + `Delete`. `Unregister` is unchanged and
keeps its whole-name meaning for the caller that wants it (`director learned --forget`). `marco
forget` takes the registered pair through the same helper the surface uses, so the two doors onto
one act take the same things.

**The general rule this and the section above share: a Play's files belong to that Play.** A move
carries them, a removal takes them, and neither reaches a different Play that happens to share a
name.

## Enforced by

- `internal/routes` — `TestRenamingAPlayCarriesItsPast`: the renamed Play still reads
  `learned`/`intact`, and no sidecar is left under the old slug. Deleting the sidecar rename fails it.
- `cmd/marco` — `TestChangingScopeDoesNotReVerifyAnEditedPlay`: an edited learned Play moved from
  focus to context still reads `edited` and still reads `Learned`. The recorded mutation is
  replacing `reg.MoveOrigin` in `handleScope` with `reg.SaveWithOrigin`, which fails it.
- `cmd/marco` — `TestChangingScopeKeepsALearnedPlayLearned`: dropping the `MoveOrigin` call
  altogether fails it.
- `cmd/marco` — `TestForgettingAPlayLeavesNoOrphanedProvenance`: the other half of the same
  invariant — a Play that leaves takes its past with it rather than stranding one.
- `cmd/marco` — `TestForgettingAStagedPlayLeavesTheRegisteredOneAlone` and
  `TestForgettingARegisteredPlayLeavesAStagedOneAlone`: the two directions of the row rule.
  Calling `reg.Unregister` from either branch of `handleDelete` fails one of them.
- `cmd/marco` — `TestForgettingAPlayTakesItsPastWithIt`: `marco forget` removes the sidecar and
  does **not** reach the staging directory. Both mutations — dropping `DeleteOrigin`, and widening
  `forgetPlay` to `Unregister` — fail it.

## Related

[[Plays]] · [[Learned-Plays]] · [[ADR-028-a-learned-play-is-a-file-with-a-past]] ·
[[ADR-081-a-durable-behaviour-is-a-play]] ·
[[ADR-079-a-demonstration-the-audience-named-is-a-play-they-may-ask-for]] · [[Wiring-Tests]]
