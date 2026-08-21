---
type: subsystem
status: active
owners:
  - director
depends_on:
  - perception
  - programs
used_by:
  - goals
  - game-packs
updated: 2026-08-06
source_paths:
  - internal/director/service
  - cmd/director/serve.go
  - cmd/director/runtime.go
  - cmd/marco/director.go
  - internal/director/service/playbill.go
  - cmd/director/playbill.go
  - plugins/overlay/insight.go
---

# Service

`director serve` holds the **warm accessibility client**. `marco director "<phrase>"` is the
thin client.

**The overlay and voice no longer reach it directly.** Both enter the one intake
([[Invocation]]), which submits a phrase here only when Marco does not already know exactly what
was meant. `PERFORM` is a registry command like an executed phrase — visible to `director status`,
refusing a concurrent mutating request, and reachable by `CANCEL_ACTIVE` —
[[ADR-085-a-performance-is-a-registry-command]].

## Why it must be a service

Chromium only exposes its accessibility tree to a *sustained* client. A per-command attach
always got a cold, shallow tree — the process would answer, but with almost nothing in it.
Warmth is not an optimisation here; it is the difference between seeing a browser's contents
and not.

## Transport

Loopback TCP with a token. Local-only by construction.

## Diagnostic surface

`status`, `explain`, `plan`, `trace`, `lower`, `wait`, `visual`, `ocr`, `observations`,
`fusion`, `collections`, `graph`, `history`, `stop` — plus the newer `goals`, `procedures`,
`actions`, `demo`, `game`, `windows`, `benchmark-vision`.

These exist because a system that decides things must be able to show its work. Most of them
answer "why did you think that" rather than "do this".

Each answers a SPECIALIST's question, and that turned out to be the problem: a front-end that
wanted to say "I recognise this as the pause menu" had to poll four of them and join the results
itself. `PLAYBILL` (protocol v6) is the one account a presentation renders instead — see
[[Visibility]] and [[ADR-033-one-account-many-presentations]]. It is non-mutating, so it stays
answerable while a command is in flight.

## The thin client's insight surface

The full diagnostic surface lives on `director`, which is right for a developer tool. But
the overlay shells to `marco`, not to `director`, so the thin client carries two of them:

```
marco director perception [--json]   providers, fusion, cycles — cheap
marco director explain    [--json]   the same plus the per-element account — expensive
```

`--json` wraps the picture with the service's running state, so a front-end renders "not
running" as a normal condition and gets both from **one** process spawn.

The split is the diagnostics package's own. `Perception` reads the service's own
observation history rather than observing afresh — so polling it cannot perturb the session
it describes, and cannot attach a second accessibility client. `Explain` reconstructs the
per-element account, which is quadratic in the observation count and is the **only** thing
that populates `Observations`. The anonymous share therefore exists on the deep path and
not on the poll.

Typing `director` in the overlay opens a live panel over the cheap path; `explain` takes a
frozen deep snapshot. See [[Passive-Observation]] for why the anonymous share is the number
worth looking at.

## Responsibilities

- Hold the warm client and the runtime.
- Route requests, including voice.
- Enforce registry validation at startup (`goal.NewValidatedRegistry`).
- Define cancellation boundaries.

## Related systems

- [[Perception]] — what the warm client feeds
- [[Programs]] — what a request becomes
- [[Control-Flow]] — cancellation boundaries

## Decisions

- [[ADR-010-passive-observation-cannot-execute]] — the observe guard applies inside the service
- [[ADR-085-a-performance-is-a-registry-command]] — PERFORM is visible, refusable and stoppable
- [[ADR-084-a-plays-identity-is-its-subject]] — PERFORM carries a subject id (protocol 7)
- [[ADR-092-one-director-per-home-one-hand-on-the-keyboard]] — the endpoint file is DISCOVERY;
  ownership is a process-lifetime kernel object claimed before the runtime exists, and the
  physical desktop is a second, machine-wide lease held around a production

## Validated by

- `internal/director/service/service_test.go`
- `cmd/director/runtime_test.go`, `cmd/director/flagorder_test.go`,
  `cmd/director/observeguard_test.go`

## Known gaps

See *Known limitation* and *Deprecated* in [[director-service]].

## Milestone record

[[director-service]] — transport, commands, voice routing, safety properties.
