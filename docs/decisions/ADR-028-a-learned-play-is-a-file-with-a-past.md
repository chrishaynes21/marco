---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - programs
  - marco-boundary
  - demonstrations
---

# A learned play is a file with a past

Five boundaries between what Director verified and something a fresh Marco can find — and none of
them grants permission to perform it.

---
## Context

[[ADR-027-what-marco-learned-becomes-marco]] left a verified procedure as readable Core v1 Marco,
inert and ephemeral, and named the gap it could not close: **no route-metadata mechanism**. A
saved learned play that cannot say which demonstration and which rehearsal produced it is a file
nobody can audit.

That claim was re-verified against the tree rather than assumed. It is half true: there is no
*metadata* sidecar, but there IS a **companion-file convention** — a taught route keeps its
recording at `<slug>.rec.json` beside the source, and `Delete` and `Rename` already carry it along.
The shape existed; the record did not.

---
## Decision 1 — provenance is a sidecar, never a comment

`<slug>.origin.json` beside the source. Kind, application, the durable `From`/`To` subjects, which
demonstration, the rehearsal digest that authorized it, a content digest of the `.marco`, and when.

Nothing ephemeral: no session, proposal, screen-state, shadow-track, fingerprint, window generation
or process id. None of those survives a restart, so none can be part of how a play is found again —
and a field that exists gets written.

Nothing in the source. A play carrying candidate ids in its comments is less readable for the sake
of an audit almost nobody performs, and the comments rot the moment the file is edited.

---
## Decision 2 — the two-file problem fails in one direction

There is no filesystem primitive that writes two files atomically. The order is chosen so the
survivable state is the safe one: **source first, provenance second**.

A crash between them leaves a `.marco` with no sidecar — which reads as an ordinary authored play,
claiming nothing. The reverse would leave provenance describing a file that does not exist, and a
later unrelated file under that slug would inherit a past it never had. `Origin` refuses that case
separately (`orphaned`), so it is unreachable twice over.

Both writes are atomic through a temp file and rename, the same pattern semantic memory uses.

---
## Decision 3 — saved and registered are different PLACES, not a flag

Route discovery is a directory scan: `global/`, an app's loose files, `context/`, `focus/`. Nothing
else is read.

So a saved play lives in `<app>/learned/` — on disk, readable, editable, auditable, and
**structurally invisible to the resolver**. Registering is moving it somewhere the resolver looks.

`saved == registered` is therefore not a mistake code can make: there is no boolean to get wrong,
and a `"registered": true` that disagreed with the filesystem cannot exist. No registry was
invented, because discovery already is one.

---
## Decision 4 — the user owns the file, and the claim changes rather than the permission

The generated comment says *change it however you like*. That is honoured: an edited learned play
still resolves, still compiles, still runs under whatever authority ordinary routes run under.

What changes is the CLAIM. The content digest stops matching and the provenance reads `edited` —
an ordinary play that remembers where it started. Nothing is prohibited to preserve provenance;
readable means ownable.

Registering an edited *staged* play is refused, because registering is what makes it findable and
doing that on Director's authority for a file Director did not write would be vouching for
somebody else's edit.

---
## Decision 5 — naming regenerates, and the names are the play's own words

Two names, because a play is a sentence: `do Volume's Mute...`. The ACTOR is what the thing is; the
VERB is what it does. Nobody is asked to name a candidate, a relationship, a subject or a graph.

Names are validated against Marco's own rule — *a capitalised name is a declaration* — with a
readable refusal before the compiler's. That is not a second grammar; the compiler is still the
authority.

Naming **regenerates** from the same ordered meanings rather than replacing strings. A substitution
is a rename that can change a procedure that happens to share a word, and the file would still
compile.

---
## Decision 6 — the lifecycle grants itself nothing

Generation, naming, saving, registration and resolution create no authority. The outstanding
rehearsal grant is untouched across the whole lifecycle; `routes.Route` is a name and a scope with
no method to invoke; resolution answers *which play you mean*, not *you may perform it*.

Forgetting removes the play and its provenance, and touches nothing Director observed. **"Marco no
longer has this play" and "Marco must forget what it saw" are different operations.**

---
## Consequences

- A learned play is a `.marco` file in the ordinary routes tree, found by the ordinary resolver.
  Convergence, not a parallel system.
- The remaining boundary is invocation: resolution to execution, which no path here crosses.

## Enforced by

- `TestALearnedPlaySurvivesToAFreshProcess` — named, saved, registered, and found by a registry
  built from nothing but a directory path.
- `TestASavedPlayIsNotYetFindable` — saved is not registered, structurally.
- `TestEditingALearnedPlayIsAllowedAndChangesWhatItClaims`,
  `TestAnEditedStagedPlayCannotBeRegisteredAsLearned`.
- `TestCollisionsAreRefusedRatherThanResolved` — authored collisions, orphaned provenance, and a
  file with no sidecar.
- `TestForgettingAPlayLeavesWhatDirectorObserved`.
- `TestTheLifecycleGrantsItselfNoAuthority`, `TestTheSavedBytesHoldNothingCaptured`,
  `TestTheProvisionalNameCannotBeSaved`, `TestAnUnverifiedRouteCannotBeSaved`.
