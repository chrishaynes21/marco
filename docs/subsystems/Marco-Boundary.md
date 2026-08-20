---
type: subsystem
status: active
owners:
  - director
depends_on:
  - programs
used_by:
  - execution
  - action-graph
updated: 2026-08-06
source_paths:
  - internal/director/marcoexec
  - internal/platform/marcorunner
  - internal/osmod/os.marco
  - internal/director/rehearse
  - internal/uiamod/accessibility.marco
  - internal/production
  - internal/platform/theaterhost
---

# Marco Boundary

The single point where the Director affects the desktop. Each planned step is **lowered to
legal Marco source**, which then goes through lexer → parser → graph → compile → runtime like
any taught route.

There is no second path. The former `internal/platform/marcohost` adapter was deleted rather
than kept as a fallback, and a boundary test asserts no duplicate implementation exists.

## Responsibilities

- Lower a semantic step to Marco source.
- Encode values safely — note that `strconv.Quote` is *wrong* here, and why, in
  [[director-marco-boundary]].
- Compile **before** mutation, so a malformed program cannot half-apply.
- Get values back out of a completed run.
- Preserve status rather than collapsing everything into FAILED.

## What it may not do

- Call `Host.Invoke` directly.
- Express an action the language cannot. If Marco has no word for it, the capability is
  declared in `os.marco` or `uia.marco` first.

## Writing down what Marco learned

`marcoexec.LowerPlay` turns a VERIFIED procedure into an ordinary `.marco` program: `use os.`, one
actor, one verb, one flat list of `do OS's Navigate with "…"`, and the four endings. It is held to
Core v1 by the real compiler against the canonical `os.marco`, and a meaning Core has no sentence
for stops the lowering rather than widening the language.

The result is INERT: no file, no registry, no resolution path. See
[[ADR-027-what-marco-learned-becomes-marco]].

## Navigation is lowered as a MEANING

`KindNavigate` lowers to `do OS's Navigate with "confirm".` — never to a key chord. The
Director learned that somebody confirmed; which key that is belongs to the host, and the
intent→key table lives in `internal/oshost/navigate.go`. See
[[ADR-024-a-dry-step-is-not-evidence]] and [[ADR-013-navigation-is-meaning-not-keys]].

A **live** rehearsal is the same path again with `oshost.Host` beneath instead of the recorder —
see [[ADR-025-one-move-then-look]]. The composition root is the only thing that changes.

A MULTI-STEP rehearsal is the same path again, N times, with an observation between every two —
and every step lowers through `marcoexec`. There is no step-2 shortcut, which is the point:
learned behaviour must not acquire a private route to a host at the moment it gains authority.

The same boundary is what a **dry** rehearsal uses: `internal/director/rehearse` lowers one
authorized step through `marcoexec` exactly as a live one will, and the composition root installs
`recordhost.Host` instead of `oshost.Host`. There is no second path — that is the point.

## What this caught

Lowering revealed three capabilities that worked from `oshost` but that **no line of Marco
could call**: `ClipboardGet`/`ClipboardSet`, `MoveWindow`, and the `Accessibility` act. A
direct-invoke design would never have noticed.

## Related systems

- [[Programs]] — produces the steps
- [[Semantic-Actions]] — decides *what* to lower; this decides how
- [[Action-Graph]] — stores the verb, never the lowering, so replay lowers again

## Decisions

- [[ADR-005-legal-marco-only]]
- [[ADR-024-a-dry-step-is-not-evidence]]
- [[ADR-025-one-move-then-look]]
- [[ADR-026-verification-is-derived-from-a-completed-rehearsal]]
- [[ADR-027-what-marco-learned-becomes-marco]]
- [[ADR-070-one-production-body-and-the-caller-brings-the-verification]]
- [[ADR-028-a-learned-play-is-a-file-with-a-past]]
- [[ADR-029-resolution-is-not-permission]]
- [[ADR-030-a-play-says-where-it-begins]]
- [[ADR-031-the-user-names-the-stage]]

## Validated by

- `TestDirectorHasNoDuplicatePlatformImplementation`, `TestDirectorImportsNoPlatformCode`
  (`internal/director/boundary_test.go`)
- `cmd/director/lockrule_test.go`, `cmd/director/keystroke_test.go`
- `TestOneAuthorizedStepIsLoweredToARecordingHost`, `TestTheRealHostReceivesTheStepsOwnOrderedIntents`
  (`cmd/director`) — one authorized step through the whole real path, into a notebook
- `director lower` renders the generated program for inspection

## Known gaps

See *What is NOT compiled into Marco* in [[director-marco-boundary]] — notably the wait
engine, which stays on the Go side deliberately.

## Milestone record

[[director-marco-boundary]] — the capability table, escaping, foreground protection, secrets,
and the tests that hold it up.

## The Screen act

`screen.marco` declares one read-only export, `Showing`, answering a single question: *is the
screen the user named the one in front?* It is fulfilled by `internal/platform/screenhost` over a
three-method read-only view of the world — it cannot press a key, focus a window, run a route,
write to memory or grant anything.

It exists because a learned play has to say where it begins and where it expects to finish, and
until something could answer that sentence every guarded play refused. **No language change was
needed** — the `when ok? … or?` shape was already Core v1; what was missing was a capability.

Silence is never yes. An unknown name, an ambiguous name, an ambiguous screen, a screen nobody can
see, a missing recogniser and unreadable memory all return `failed`. A standalone `marco` with no
recogniser wired refuses rather than degrading into blind replay.

See [[ADR-030-a-play-says-where-it-begins]], [[ADR-032-a-play-says-where-it-ends]] and
[[Learned-Plays]].

## The Theater act, and the one production body

`theater.marco` declares one export, `Activate`: do to this target what pressing it would do. It
is fulfilled by `internal/platform/theaterhost`, which resolves the semantic target against
whatever can see it tonight, casts an available Actor, and runs what that Actor writes.

**An Actor writes legal Marco and performs nothing.** `Actor.Cast` returns a program; the
Production boundary runs it through an injected `MarcoRunner`. So a press reaching the machine
through the Theater is subject to exactly the guarantee this note is about — compiled before
mutation — rather than being a second route with no compile gate and nothing for a dry run to
record. An Actor that called a host directly would be that second route, and
`TestAnActorNeverReachesAHostDirectly` fails if one does.

**Both callers use it.** A saved play asks through the ordinary act. A live rehearsal asks through
`internal/production`, a contract of types and interfaces that lets the Director name a production
without importing the Theater — the boundary that keeps `internal/director/rehearse` unable to act
by itself. The rehearsal's old direct path (resolve a name to an element id here, walk a private
activation ladder) is deleted rather than kept as a fallback, for the same reason `marcohost` was.

**Authority and verification cross with the request.** A permission that can be spent once and
only for the named target; and whatever the caller can check with, which for a standalone runtime
is honestly nothing. See
[[ADR-070-one-production-body-and-the-caller-brings-the-verification]] and
[[ADR-068-the-theater-is-the-durable-semantic-world]].

The ownership question this settled — which side of the Director/Theater line each responsibility
falls on — was worked out first in [[34E-director-theater-audit]].

**A cast program cannot re-enter the Theater.** The act map it can reach is built without the
Theater in it, in `cmd/marco/theaterwiring.go`, so the recursion is impossible rather than bounded.
