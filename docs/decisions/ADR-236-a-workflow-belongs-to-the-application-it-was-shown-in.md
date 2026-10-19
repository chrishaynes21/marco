---
type: adr
status: accepted
date: 2026-10-18
decision: ACCEPTED — a durable workflow records the application it was demonstrated against, taken from the window that was WATCHED and never from a name the caller supplied. It may act on that application and no other. The check is made against the window actually acquired, before anything that can press exists, and again immediately before every press — a valid check at invocation is not permanent authorisation. An artifact saved without a scope is UNSCOPED, which is readable and valid for no application until it is bound from observed provenance. And Marco reads the foreground itself, cold, rather than asking which application is in front.
supersedes: []
affects:
  - learning
  - execution
source_paths:
  - internal/director/workflowstore/scope.go
  - cmd/director/foreground.go
  - cmd/director/performworkflow.go
  - cmd/director/demonstratevisual.go
---

# ADR-236 — a workflow belongs to the application it was shown in

## The defect

`workflowstore.Artifact` recorded no application. Not a weak identity, not a mutable one — none.
So a workflow demonstrated against Ark, handed to `perform-workflow --application solasta`,
would acquire Solasta's window and begin resolving Ark's controls against it. The only thing in
the way was a person answering *is this screen_a?* with no.

The owner, on being shown it:

> Marco's executable workflow doesn't know which application it belongs to. That means an Ark
> workflow could potentially be directed at Solasta, with the user left to catch the mistake.
> That's an architectural safety defect, not a perception problem. We should fix it before
> allowing pixel execution.

A person catching a mistake is not a guard. It was found while answering a different question —
which game is in front — and it is release-blocking for pixel execution.

## The decision

**1. The identity is the executable image name.** `winctx.LiveWindow.Image`, which
`windowref.Ref.Application` already carries and already describes as "the semantic identity that
DOES survive a restart, and the thing reacquisition searches by".

Not a PID: a relaunched game is the same application and a different process. Not a window
handle: handles are recycled, which is the failure this would otherwise invite. Not a title:
titles change while a window lives, two applications can share one, and a game window's title
is the commonest place an account name appears — which is why `capture-game-session` refuses to
record one and why this does too.

**2. It is taken from the window that was WATCHED.**

> Never blindly trust an application name supplied by the caller if it disagrees with observed
> provenance.

`--application` is a selector: it says what to look for. The scope is `v.Ref.Application`, what
the window layer reported about the window actually acquired. They agree almost always, and the
one time they differ is the one time it matters — a scope taken from the argument would compare
a request against a request, and the gate at replay would always pass.

**3. An unscoped artifact is valid for no application.**

> Existing artifacts without application scope must remain readable, but they must not silently
> become valid for every application.

The dangerous reading of an empty field is "matches anything", and that is the reading a zero
value invites. So `Allows` answers no to every application, including the empty one, with a
sentence saying the artifact was saved before scope existed. It loads, it reads, it does not
run. Binding is a deliberate act from observed provenance, and a scope once set is never
rebound: a workflow for another application is another demonstration.

**4. The check is made three times, and the third is the point.**

| when | against what |
|---|---|
| at invocation | the window acquired then, before the grounder, granter, runner or executor exist |
| before every press | the window acquired *now*, as a case in `worldMoved` |
| never | the `--application` argument, which is what somebody asked for and cannot have changed |

> Don't let a valid application check at invocation become permanent authorization. Recheck
> foreground/window identity immediately before each action, and refuse if the application
> changes.

Time of check is not time of use. Between the two the person may have alt-tabbed, the game may
have exited and another taken its handle, or `Acquire` may have resolved a different window of
a different application. The recheck lives in a pure function so the race is reproducible
without a desktop and without anything being pressed.

**5. Marco reads the foreground itself.** `--application` is optional. `director windows`
answers this and is a service client, so a cold machine with no daemon got an error rather than
an answer — which made Marco ask a person for the one fact the operating system hands over for
free. `winprovider.AllCandidates` needs nothing running.

Four refusals rather than a guess, because a guess here picks the window a run then acts on: no
windows at all, nothing in the foreground, the foreground window minimised, two applications
claiming the foreground. Two windows of ONE application still names the application — a scope
compares images, and which window to watch is `windowref.Selector`'s question, which already
refuses that ambiguity itself.

**6. And discovery cannot satisfy the scope check.** `foreground.go` may not import
`workflowstore`, asserted over its imports. Discovery decides what to LOOK at; whether the
workflow may act there is the artifact's own scope, asked separately. A resolver that could
consult an artifact could satisfy the check it is supposed to be independent of.

## What this is not

It is **not** a claim that the workflow will work on that application — only that it may not be
pointed at another one. Everything else the replay path refuses, it still refuses.

It is **not** a new format. One additive string field; every artifact on disk loads, and one
saved without a scope writes no `application` key at all, so a round trip is byte-identical.

It is **not** a licence to act. `Arrival.Verified` is still false and still says why.

## Enforced by

- `internal/director/workflowstore` `TestAnArkWorkflowIsRefusedAgainstAnotherGame` — the
  mismatch, naming both applications.
- `internal/director/workflowstore` `TestALegacyUnscopedWorkflowIsValidForNoApplication` — it
  loads, it is valid for nothing, and it writes no key.
- `internal/director/workflowstore` `TestALegacyWorkflowIsBoundDeliberatelyAndNeverRebound`.
- `internal/director/workflowstore` `TestAnUnidentifiedApplicationIsRefusedSeparately` — three
  ways to be no, three sentences.
- `cmd/director` `TestTheApplicationIsRecheckedAtTheInstantOfThePress` — the race: same world,
  same generation, same epoch, same screen, target in front, only the application differs.
- `cmd/director` `TestForegroundDiscoveryRefusesRatherThanGuessing` — the four refusals, over
  supplied candidates, needing no desktop.
- `cmd/director` `TestAnExplicitApplicationSelectsTheTargetAndNothingElse` — and
  `foreground.go` cannot import `workflowstore`.
- `cmd/director` `TestTheScopeGateRefusesBeforeAnythingIsBuilt` — the gate's condition, and that
  it returns before anything that can press is built.
- `cmd/director` `TestASavedWorkflowIsScopedToTheApplicationThatWasWatched` — observed
  provenance, not the argument.
- `cmd/director` `TestAnArkWorkflowAimedAtAnotherApplicationIsRefusedByTheCommand` and
  `TestALegacyUnscopedWorkflowIsNotAuthorisedByTheCommand` — through the real command, with no
  input emitted.

## A note on what is asserted over source text

The scope DECISION is covered behaviourally: every mutation of `Scope.Allows` died against real
tests. Two things are asserted over the command's text instead — that the gate's refusal is
acted on and comes before anything that can press, and that the teaching path scopes from
`res.application` rather than `*app`.

Both are ordering-and-provenance claims about a command whose behaviour needs a live window of a
particular application to reach, and a mutation run showed that an exit code cannot tell the
gate from a window that was not found. The same pattern already proves `WithConfirm` and
`WithPointer` are installed, for the same reason. Stated here so a reader knows which claims
rest on which kind of evidence.
