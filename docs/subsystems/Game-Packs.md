---
type: subsystem
status: active
owners:
  - director
depends_on:
  - fusion
  - vision
  - windows
used_by:
  - service
updated: 2026-08-06
source_paths:
  - internal/director/game
  - internal/gamepacks
  - cmd/director/gamecmd.go
---

# Game Capability Packs

A per-application contribution of entities, detection rules and safety constraints — the way
the Director learns that "the pause menu" means something specific in one game without
teaching the Director about games in general.

## What a pack contributes

- **Entities** — the nameable things this application has.
- **Detection** — how to tell it is running and what state it is in.
- **Safety** — what must never be done unattended here.

Each piece plugs into an existing seam. A pack adds knowledge; it does not add mechanism.

## Attached to the application, not the window

A pack survives a restart because application identity survives a restart. Nothing in a pack
is positional, because window identity does not survive — see
[[ADR-009-window-identity-is-ephemeral]].

## Verification stays semantic

A pack may not lower the bar for what counts as proof. "The screen changed" is not evidence
that a game action worked, in a pack or anywhere else.

## The dependency on Vision

Packs are the clearest consumer of nameable roles. A rule that says "when the pause menu
shows RESUME GAME" needs a label, and a label needs a structural role — which is the
constraint currently blocking [[Vision]]. Until a detector emits nameable roles, a pack for a
custom-rendered game UI has little to attach to.

## Related systems

- [[Vision]] — blocked upstream dependency
- [[Windows]] — application vs window identity
- [[Fusion]] — where a pack's entities become belief

## Decisions

- [[ADR-009-window-identity-is-ephemeral]]
- [[ADR-004-vision-cannot-establish-actionability]]
- [[ADR-003-evidence-authority-by-source]]

## Validated by

- `internal/director/game` unit tests
- `director game` commands

## Known gaps

- Only the Palworld pack exists as a worked example.
- See the *Known gaps* section of [[director-games]].

## Milestone record

[[director-games]] — the claim and how it is checkable, entities, detection, safety, and the
two general outcomes that arrived with the first pack.
