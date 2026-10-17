---
type: adr
status: accepted
date: 2026-10-17
decision: a dry-run request is refused rather than served by a runtime that can perform real input, and foregrounding goes through one guarded door because it sits outside the host abstraction
supersedes: []
affects:
  - execution
source_paths:
  - internal/director/service/server.go
  - cmd/director/runtime.go
  - cmd/director/perform.go
  - cmd/director/experiment.go
---

# ADR-216 — a dry run is dry at the boundary

Two defects, found by reading rather than by running, while auditing before the read-only Alpha 1
evaluation of Windows Settings. Both are safety-contract violations; neither had ever failed a
test, because neither was ever tested at the boundary where it mattered.

## One: `--dry-run` was a flag that did nothing

`director execute --dry-run` sent `ExecutePayload.DryRun: true`. `Server.execute` never read it
and called `s.runtime.Handle` unconditionally. Only `director serve --dry-run` ever made a runtime
dry — it chooses `runtime.DryRunHost` at construction — and the two facts were never compared.

So the flag documented as *"plan and verify without performing real input"* typed, clicked and
focused for real against any ordinarily started Director.

A flag that claims a safety property and does not have it is worse than no flag at all, because it
is the one somebody reaches for when they are being careful. It was reached for in exactly that
spirit during the evaluation, which is how it was found.

### Refusing, rather than making the request dry

The host is chosen once, when the runtime is built, and shared by everything in flight. Swapping
it per request would open a window in which a concurrent command — or the ambient observer, or a
confirmation already granted — ran against the wrong one. A preview is not worth that, and a
refusal a person can read is worth more than a preview they cannot trust.

So the service asks `Runtime.PerformsRealInput()` and refuses, naming somewhere to go:

> this Director was started to perform real input, so it cannot preview a request without
> performing it. Nothing was done. For a preview that touches nothing, use
> `director plan "<request>"`; for a Director that cannot reach the desktop at all, start one with
> `director serve --dry-run`.

`director plan` and `director goal --dry-run` are both pure, need no service, and between them
answer what the flag was reached for. Ordinary execution is untouched.

`PerformsRealInput` is reported from a field set at construction rather than by type-switching the
host, so a future host that is dry for some calls and not others cannot answer it ambiguously.

## Two: foregrounding sat outside the host abstraction

Every desktop *effect* goes through the Theater and its host, which is what makes `DryRunHost` a
structural property rather than a promise. Foregrounding did not: `winctx.Activate` and
`winctx.ActivateTitle` were called straight from `cmd/director`, so a runtime built with
`DryRunHost` could still move somebody's screen.

The reachable instance was specific. With an empty store no Place resolves, so
`disambiguateWindow` — whose job is to tell an application's windows apart *by looking at them* —
would have raised every `applicationframehost` window in turn (Settings, XBOX, Realtek Audio
Console) during an evaluation whose whole premise was that it touched nothing.

`Runtime.raiseWindow` is now the one door, and it refuses when the runtime cannot perform real
input. Refusing rather than pretending to succeed: a disambiguation that cannot look at the
candidates has not disambiguated anything, and saying so leaves the ambiguity honest.

`restoreDesktop` is deliberately NOT gated on it. That was tried and two tests refused it,
correctly: restoration is a REPAIR, and refusing to put a desktop back leaves the person standing
in Marco's experiment with nothing said about it, which is worse than the motion it avoids. The
dry case needs no gate because it closes itself — nothing moved, so the already-there branch
returns before anything is activated, and a gate would only suppress the sentence.

## Enforced by

- `internal/director/service` `TestADryRunNeverReachesALiveRuntime` — drives the real `Server`
  over a real connection with a runtime that says it can reach the desktop, and asserts the phrase
  never arrives, plus the refusal names somewhere to go. The other half asserts a genuinely dry
  runtime still serves the request, because that is the case where the flag tells the truth.
  **At the server**, deliberately: a CLI test proves the flag is sent and cannot prove anybody
  acts on it. The test fake defaults to `wet: true`, because a fake that defaulted to dry would
  let every other test pass through the guard without noticing it.
- `internal/director/service` `TestAnOrdinaryRequestIsNotRefusedByTheDryRunGuard`.
- `cmd/director` `TestADryRuntimeMovesNobodysDesktop` — walks every non-test file in the package
  for a direct foregrounding call and allows three named lines, each itself guarded or
  substitutable. It walks rather than naming sites for the reason
  `internal/platform/navsource/pump_test.go` does: a list of call sites is a list
  somebody forgets to extend, and the next person needing to raise a window will reach for
  `winctx.ActivateTitle` because that is what it is called.
- `cmd/director` `TestRaiseWindowRefusesWhenTheRuntimeCannotPerformRealInput` — and that a runtime
  which *can* perform real input is not refused as dry, so the guard cannot swallow the live path.
