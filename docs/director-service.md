---
type: milestone
status: historical
---

# The Director service

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

The Director runs as one long-lived local process. Client commands submit intent; the
service owns state, observation, execution, verification and cancellation.

```
voice → overlay → spawn marco.exe (thin client) → IPC → Director service
                                                          ↓
                                            streamed events + result
```

## Why

Spawn-per-command stopped working when replay arrived.

- **Cancellation.** "Repeat that ten times" takes real time, and a spoken "stop"
  arrives as a *different process* — which cannot cancel a context inside the one
  running the loop. The old workaround was a timestamped stop file polled between
  iterations.
- **Accessibility hydration.** Chromium and Electron expose their interiors only
  after sustained accessibility-client presence. Chrome went 65 → 2248 elements over
  minutes of continuous attachment; a process that lived for one command never got
  past the first number. Measured after this change: VS Code reports **251** elements
  where the per-command model saw 195, and providers accumulate observations across
  separate client invocations.
- **Conversation.** "Do that again" could not mean anything across processes that
  each exited after one phrase.

## Transport: loopback TCP with a token

Considered:

| Option | Verdict |
|---|---|
| **Windows named pipe** | Strongest on security — local by construction, ACL-able. Rejected: Go's standard library cannot create one, `go-winio` is an external dependency (Marco's engine permits none), hand-rolled overlapped I/O is real work with subtle shutdown failure modes, and it is Windows-only. |
| **gRPC** | A large dependency for a JSON-shaped local protocol. |
| **Loopback TCP + token** | **Chosen.** Pure standard library, streams naturally, many clients, works unchanged on macOS/Linux later. |

The honest cost: a loopback port is reachable by any process running as this user,
where a named pipe could be ACL'd. Mitigations:

- listener binds `127.0.0.1` explicitly — there is no configuration that would bind
  it to a routable interface;
- a 256-bit random token, regenerated per service start, in a `0600` file;
- the token is required **before any request is read**, compared in constant time.

This protects against another local program stumbling onto the port. It is *not*
protection against a process already running as you, which could read the token file
anyway. A named pipe with a security descriptor is the upgrade path if that
distinction ever matters.

## Commands

```
director serve                 run the service (owns Director construction)
director status [--json]       what it is doing
director stop                  cancel the active command
director observations          which providers observed, and what they produced
director explain [id]           why an element exists, and why alternatives lost
director fusion                what fusion made of the evidence
director shutdown              stop the service

marco director "<phrase>"      submit a phrase       ← the front-end entry point
marco director stop            cancel
marco director status          state, including any pending question
marco director history         recent actions
marco director shutdown        stop the service
```

`marco director "<phrase>"` **routes** rather than simply executing: it asks the
service what it is doing, and that answer decides whether the phrase is a
cancellation, an answer to a pending question, or a new request. See *Voice routing*.

> **Superseded on the entry point.** `marco director "<phrase>"` is no longer where a front
> end enters — `marco do` is, for every surface. The routing described here (control phrase /
> pending question / new request) moved up into the one intake and gained two arms above the
> Director tier. `marco director` remains the thin client the intake itself calls. See
> [[Invocation]].

`director execute`, `history`, `last`, `graph`, `show` and `analyze` are unchanged
from a user's point of view; `execute` and `history` now go through the service.
`graph`, `show` and `analyze` read the action graph directly, so they work with no
service running.

Director construction lives in `cmd/director` only. `marco director` is a pure
client — it never builds a Director, it locates `director.exe` (next to `marco.exe`,
or `$DIRECTOR_BIN`) and starts one if none is running.

## Voice routing

Vosk recognition is untouched. The voice plugin emits final phrases:

```json
{"feed":"Voice","event":"Final","data":"click the save button"}
```

The overlay receives these as a `RunVoice` act and now dispatches them to the
Director (`plugins/overlay/director.go`), with route lookup kept as the fallback.
`MARCO_DIRECTOR=off` restores the old behaviour exactly.

Typed commands are **untouched**. Someone typing `` `m my route `` is naming a route
they taught; someone speaking is describing what they want to happen.

> **Superseded — this whole section is the defect Phase 2 removed.** Splitting the intake
> by input device meant a play you could run by typing was not found by saying its name.
> Typed and spoken now build the same argv and differ only in `--source`, which is recorded
> and never consulted; the engine's intake decides whether a request is for the Director.
> `MARCO_DIRECTOR=off` no longer gates dispatch at all — it only gates the watch panel's
> polling. Read [[Invocation]] and [[ADR-083-one-invocation-intake]] for what is true now.

### Who decides what

| | captures | routes | interprets |
|---|---|---|---|
| overlay | ✅ speech | ✅ which request to send | ❌ never |
| Director | ❌ | ❌ | ✅ what a phrase means, which candidate |

Routing is a decision about *which request*. Interpretation is a decision about *the
desktop*. The overlay does the first and never the second — it does not choose
candidates, and it does not resolve targets.

The routing rule lives in `service.RoutePhrase`, is pure, and is checked in this order:

1. **A control phrase always cancels** — `stop`, `cancel`, `stop that`, `cancel that`
   (and `stop it`, `cancel it`, `abort`, `halt`). It never reaches the intent planner.
   This holds even while a question is pending, so a user who changes their mind
   mid-clarification is not stuck answering it. The list is deliberately narrow:
   "stop the music" is a request, not a cancellation.
2. **A pending question captures the next phrase** as its answer.
3. **Everything else is a new request**, and supersedes any unanswered question.

### The clarification exchange

When the Director cannot tell which control was meant it asks, carrying the Command ID,
the question, and up to five numbered candidates with their labels and roles.

The answer is turned into a **refinement** of the original query — an ordinal, or a
role — never into an element id, and the whole request is then resolved again from
scratch against a fresh world. The screen may have changed while the user was thinking.

- Only **viable** candidates are offered (never a disabled control or inert text).
- Only **contenders** are offered — the candidates actually in contention, not every
  loose match. The offer is a *prefix* of the ranked list, never re-ordered or
  de-duplicated, which is what keeps "the second one" meaning the same thing to the
  user and to the resolver.
- An answer for a **different Command ID** is refused, not applied to whatever is
  pending now.
- An **unparseable** answer is treated as a new request rather than forced into a
  choice. Every word must be a choosing-word: "open the file menu" contains "menu" and
  is still a request.
- "never mind" / "cancel" **abandons** the request without doing anything.

### The seven events a front-end renders

`ACKNOWLEDGED` · `PROGRESS` · `CLARIFICATION_REQUIRED` · `COMPLETED` · `UNVERIFIED` ·
`FAILED` · `CANCELLED`.

`CLARIFICATION_REQUIRED` is terminal for the *request* but not for the *command*: the
client stops reading, the command stays open awaiting an answer, and `status` reports
it so the next phrase — from a different process — routes to it.

### Failure

`marco director` exit codes are the front-end's branch point:

| code | meaning | fall back? |
|---|---|---|
| 0 | completed and verified | no |
| 1 | delivered; failed, unverified, or asked a question | **no** — it was handled |
| 3 | never delivered — the Director is unreachable | **yes** |

Auto-start is attempted, then retried **once**, then reported. A spoken phrase that
hangs waiting for a service that will never appear is its own failure. On code 3 the
phrase is echoed to stderr and the overlay falls back to route lookup: a recognised
phrase is never silently discarded.

Dispatch is **asynchronous**. A replay can run for minutes and a spoken "stop" arrives
as a second phrase while the first is still going; a blocking dispatch would make
cancellation impossible exactly when it is wanted.

## Perception

All perception flows through observation providers into one fusion engine, which is the
only component permitted to turn evidence into belief. `PERCEPTION`, `EXPLAIN`, `READ_TEXT`
and `READ_REGION` (protocol v3) expose what the providers and the engine did; the service passes both
through without summarising, so there is no second opinion between a caller and the
fusion engine. `EXPLAIN` is a separate request because reconstructing an explanation is
quadratic in the observation count, and no ordinary command should pay for it.
See [director-perception.md](director-perception.md).

## Safety properties

- One mutating command at a time; a second gets a structured `BUSY` response.
- Status, history and cancellation stay available *during* execution.
- Client disconnect is **not** cancellation — work continues, the outcome is retained.
- A cancelled command reports how many actions completed.
- An unverified iteration stays unverified, never "failed".
- A restarted service never fabricates an active command.
- Protocol version mismatches fail explicitly rather than being negotiated.
- No request is read before the token is validated.
- A control phrase never reaches the intent planner, so "stop" cannot be resolved as a
  control to click.
- A recognised phrase is never silently discarded: it is delivered, or reported.
- The overlay never chooses a clarification candidate.

## Known limitation

Two offered candidates can share a label — live against VS Code, "click new" offers two
identically-named "New Terminal" buttons. They are genuinely distinct elements, so
de-duplicating them would break the numbering the ordinal indexes into; distinguishing
them properly needs geometry in `TargetCandidate`, which is wider than this milestone.
Picking either does something reasonable, but the question as posed is not fully
answerable.

## Cancellation boundaries

Replay checks for cancellation before each iteration, after observing, before
executing, and after verifying — never in the middle of a platform action, because a
half-sent click is worse than a completed one and the actuator's calls do not support
interruption.

## Deprecated

The timestamped stop file (`director-stop`) is no longer the cancellation mechanism.
It survives behind `service.DeprecatedStopCheck` purely so a pre-service client build
can still stop a replay, and the service does not consult it. One adapter, one place
to delete.
