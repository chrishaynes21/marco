---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - programs
  - marco-boundary
---

# Resolution is not permission

Knowing which play the user means and being allowed to perform it are different questions. Until
this milestone Marco could not tell them apart, because there was nowhere to stand between them.

---
## Context — the audit found no door

`Deps.Do` resolved a phrase and called `Run` in the same breath:

```
Do(name) → Reg.Resolve(app, name) → d.Run(rt) → driver.RunFileWithHosts(...)
```

One function. No seam. Nothing that could hold *"I believe this is the play you mean"* without
also performing it. The `confirm` mechanism existed, but only for teaching — nothing asked before
invoking a resolved route.

That was defensible while every play was authored or taught: a person wrote it or demonstrated it,
and asking for it by name was the whole permission model. It stops being defensible the moment a
play exists that **Marco wrote**, from behaviour it watched, because the user asking for it by name
is not the same as the user having read what it does.

**So the milestone was building the door.**

---
## Decision 1 — the seam is general, and the policy is what differs

`Deps.Resolve` answers WHICH play and returns a `Resolved` — a value that can be inspected, logged
and refused, with nothing on it that runs anything. `Authorize` is the door. `Run` is behind it.

Every route passes through, authored and taught included. There is no `if learned { special path }`
anywhere: what differs is the decision, not the road.

---
## Decision 2 — the policy, in discrete verdicts

| play | verdict | why |
|---|---|---|
| authored | `allowed` | somebody wrote it; asking for it has always been enough |
| taught | `allowed` | they demonstrated it themselves |
| learned, provenance intact | ask, once per invocation | Marco composed it; the user has never read it |
| learned, source edited | `allowed` as ordinary | it is the user's own writing now |
| learned, no way to ask | `refused` | fail closed — a Marco that cannot put the question must not answer it |

No confidence float. `Declined` (the user said no) is kept apart from `Refused` (Marco declined)
and from resolution failing — three different sentences, three different meanings.

---
## Decision 3 — authority is per invocation, and is never written down

There is no `Trusted: true` on a learned play. Director verified one observed procedure under
particular evidence; that is knowledge, and permission to use knowledge now is a different thing.

The question is asked about THIS invocation and answered for it. Asking again next time is correct
behaviour rather than a missing feature, and nothing durable records that anybody ever said yes.

---
## Decision 4 — provenance decides, not the path or the name

`Classify` reads the origin record beside the file. A copied `.origin.json` describes a play that
is not there, its digest does not match, and it reads as `edited` — so it gets the ordinary policy
rather than the learned one. Trust cannot be manufactured by moving files around.

---
## The gap this milestone found

**The generated play does not encode its own starting state, and Core v1 cannot express one.**

Director verified *"from subject A, press down, down, confirm, arrive at B"*. The play says:

```marco
do OS's Navigate with "down".
do OS's Navigate with "confirm".
```

Application context IS expressible and IS used — a learned play is registered as a `context/`
route, which ordinary Marco already refuses to run unless that application is in front. But
SCREEN-level start state is not expressible: Core v1 has no sentence for *"only when the screen
looks like the settings menu"*, and inventing one was explicitly out of scope here.

So today a learned play invoked from the wrong screen inside the right application will press its
keys anyway. **A confirmation does not fix that** — a user saying yes does not make an invalid
starting state correct, and this ADR does not pretend otherwise.

This is classified as a **lowering / language-expression gap**, not papered over with a hidden
Director graph consulted at execution time. The promise is that what Director learned becomes
Marco; a play whose safety depends on something the play cannot say has not kept it.

## Enforced by

- `TestTheMarcoMomentDryPath` — natural request → resolution → authority → ordinary compiler →
  recording host, emitting exactly what the `.marco` says.
- `TestResolvingAPlayPerformsNothing` — the seam held open.
- `TestDecliningIsNotTheSameAsNotFinding`, `TestALearnedPlayIsRefusedWhenThereIsNoWayToAsk`.
- `TestEveryKindOfPlayConvergesOnTheOrdinaryRuntime` — authored, taught, learned and edited all
  reach the same door and the same runtime.
- `TestACopiedOriginRecordManufacturesNoTrust`, `TestTheAuthoritySeamPerformsNothing`,
  `TestPermissionIsNotRemembered`.
