---
type: decision
status: accepted
date: 2026-08-19
supersedes: []
affects:
  - demonstrations
  - learned-plays
  - execution
source_paths:
  - cmd/director/perform.go
  - cmd/director/performcmd.go
  - internal/director/rehearse/live.go
---

# ADR-078 — a learned Play is performed by the Director

## LEARN → RUNNABLE PLAY: COMPLETE

Proven from a **cold process** with an **unrelated foreground application**. Director resolved the
durable learned goal, brought the application forward, obtained fresh Stage truth without relying
on session history, planned two verified edges, executed both through the shared bounded `Perform`
walker, verified after each edge, and positively confirmed the final Place.

One demonstration produced three durable semantic Places, two reusable edges, 2/2 verified, a
complete legal Marco Play — saved, registered, and callable by the phrase it was Learned under.

## The gap this closes

A learned play could be demonstrated, verified edge by edge, written down as legal Marco,
registered and resolved — and nothing in the system would walk it. Measured, all three surfaces:

| surface | outcome |
|---|---|
| `marco do "Open Mouse Settings"` | resolved and ran; refused at its own first line — standalone Marco has no perception, so `Screen's Showing with "Home"` can never be satisfied |
| `director execute "open mouse settings"` | ignored the Play; one-shot semantic lookup — `absent: nothing matching "mouse settings"` |
| `director reach "Open Mouse Settings"` | knew the outcome, planned only, and planned from where a finished session ended |

## Decision

**Execution is its own path above the walker rehearsal already uses.**

`Rehearse` is grant-state guards, then `Perform`. `Perform` is: establish → source check →
`BeginAttempt` → step loop verifying after every step. Both callers go through it. A second caller
reimplementing the walk would be a second set of answers to *"did that step work"*, and every
verification claim in this system rests on there being one.

What differs is **above** the walk:

- **rehearsal authority** — Marco asked whether it may try something once, and was told yes;
- **execution authority** — the Audience named a learned behaviour and asked for it.

Both mint the same object, because that object is also the **budget**: `BeginAttempt` binds an
attempt to its input and duration bounds and consumes it, so there is no path to real input without
one. Execution's is minted under the epoch `asked`, so an audit can tell the two apart.

### The order is not arrangeable

**Foreground → look → decide.** A Stage read taken while another application is in front describes
somebody else's window, and the source check made from it would refuse a reachable route or accept
an unreachable one.

### No planning from history

`freshPlace` takes a real look through `StartObservation` — the same path Sight uses — and resolves
it with `observe.PlaceNow`. No second resolver: the freshness is in the evidence, not in a new
opinion about it. `reach` answered from the newest finished session and told the Audience
*"You're already there"* about a screen they had left.

### Fresh is not enough: the evidence must be about THIS application

Freshness was the only condition, and it admits a second way of being confidently wrong. A live
session watching another program answers with a real subject id, from a look taken a moment ago,
about a window the play is not in — live evidence, wrong subject, and nothing downstream can tell.

So a live reading is trusted only when the session's application is the one being performed
(`placeNowIn`), and both the fast path and the poll loop go through that one gate — the loop
previously asked the registry without the live check at all, so a look that started and then died
answered from whatever session happened to be newest.

### Performing waits while something else is being watched

One observation session runs at a time, so a session that is running is somebody demonstrating. A
performance would bring another application forward under them, and every reading after that would
be about a window their session is not watching — ADR-065 keeps operating and demonstrating apart.

Refused with `watching_elsewhere` **before** the foreground is touched, rather than discovered
later by a look that cannot be taken: by then the interruption has already happened. The refusal
names what is in the way, because *"I can't tell which screen is in front"* would be a true-sounding
sentence about the wrong cause.

### Cold start is the test of durability

Both the application and the window used to come from session history, which quietly means "Marco
can run this if it happened to observe the application in this process" — a warm cache wearing
durability's clothes. The Audience's phrase names a goal; durable memory says which application the
goal lives in; the desktop says where that application's window is. History remains available only
as history, for when the desktop cannot be read at all.

### Stop at the first honest failure

A route that got half way is a different fact from one that never started. Replanning around a
failed edge is a decision nobody has made yet.

### A plan that ran is not a goal that was reached

A second fresh look confirms arrival. The last edge's own verification says the step worked; this
says the Audience is where they asked to be.

## Known follow-ons

1. **Redundant settle work makes normal execution slower than necessary.** `freshPlace` starts a
   bounded observation to answer "where am I", returns as soon as the place resolves, and leaves the
   session alive; `Perform` then establishes again immediately, settling the same unchanged screen
   twice. The planning look and the first establish should share one settle, and the look should
   finish as soon as it has its answer.
2. **Application-name foregrounding is ambiguous when several top-level windows share a process.**
   Settings, XBOX and Realtek Audio Console are all `applicationframehost`; activating by
   application name can raise the wrong one, and the fresh look then cannot place the screen —
   observed live as `place_unknown` on a first attempt. Foreground selection should prefer the
   candidate window whose fresh Stage resolves to the required Place, and fail honestly when
   ambiguous.

Neither invalidates the acceptance. Both are efficiency and selection, not correctness of the walk.

> **Clarification, 2026-08-20 — delegation is for a play whose provenance is still intact.**
>
> "A learned play is performed by the Director" is about the artifact Director verified. The moment a
> person EDITS one, it stops being that artifact: `orchestrator.Resolved.Learned()` is
> `Kind == learned && Provenance.Verified()`, the authority seam gives it
> `edited_since_learned`/`Allowed`, and it runs locally like anything else they have written.
>
> **It must not be delegated instead, and the reason is worth stating so nobody "fixes" it.**
> `performLearned` sends only `Name`, `Application` and `Subject` — the Director re-plans from the
> goal and never reads the file. Routing an edited play to it would run the ORIGINAL behaviour while
> the person watched, believing they were running their edit. Failing is bad; silently doing
> something else is worse.
>
> What was actually wrong was the other end: the local runner could not see, so an edited play
> refused at its own first line. That is now fixed — marco's Screen host asks the Director which
> place is showing, and still refuses when nothing can answer. See
> [[ADR-031-the-user-names-the-stage]], Decision 4's amendment.

## Enforced by

- `internal/director/rehearse/rehearse_test.go` — `TestRehearsalAndExecutionShareOneWalker`
  (removing the shared call fails 23 tests)
- `cmd/director/performwiring_test.go` — `TestAColdProcessFindsItsWindowOnTheDesktop`;
  `TestTheWindowComesFromTheDesktopNotAPreviousSession` (seeds a live desktop AND session history,
  and requires the live one to win); `TestAnUnlearnedOutcomeIsRefused`
- `cmd/director/performplace_test.go` — where the Audience is standing, and the walk:
  - `TestAFinishedSessionIsNotWhereTheAudienceIsNow` (deleting the live conjunct in `placeNowIn`
    plans the route from where a retired session left off)
  - `TestALiveSessionElsewhereIsNotWhereTheAudienceIsNow` (deleting the application check answers
    with the screen somebody is standing on in another program)
  - `TestExecutionPlansFromAFreshLook` (deleting the look drops the reason with it)
  - `TestPerformingWaitsWhileSomethingElseIsBeingWatched`
  - `TestExecutionStopsAtTheFirstUnverifiedEdge` (through `performPlan`, the production loop)
  - `TestArrivalIsConfirmedByLookingNotByFinishing` (both halves: a look that answers confirms, a
    look that cannot answer is not arrival)
  - `TestNothingIsReadFromTheStageBeforeTheApplicationIsBroughtForward` — **the order IS gated
    now.** The earlier record said it could not be, because foregrounding needs a live desktop the
    fakes cannot move. That is about the EFFECT; the invariant is the ORDER, and order is
    observable: foregrounding an application no window answers to fails, so a correct `PerformGoal`
    returns having asked the desktop nothing. A counting fake desktop sees the read the moment
    `bringForward` is moved after the look. Verified by applying that reordering.

## Related

[[ADR-077-consent-is-the-audiences-authority-is-marcos]] ·
[[ADR-075-a-learn-episode-outlives-its-sessions]] ·
[[ADR-074-one-demonstration-every-leg-reviewed]] ·
[[ADR-076-a-place-may-say-what-it-appears-to-be-called]] ·
[[ADR-029-resolution-is-not-permission]] · [[Learned-Plays]]
