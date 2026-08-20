---
type: milestone
status: historical
---

# Bounded semantic collections

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

> A collection is a bounded semantic QUERY over the current world — never a list of
> answers. Iteration advances only after the current member is verified.

## Why a query and not a list

The obvious design captures the members once and walks the slice. It is wrong in a way
that only shows up while acting.

Every answer a resolver produces — an element id, a window handle, a coordinate —
describes the screen at the moment it was produced. A bulk action then spends seconds
changing that screen, deliberately. By the third member, the list is a description of a
world that the first two members destroyed:

```
close every Notepad window
  captured: [handle A, handle B, handle C]
  closes A  ->  B and C are still valid
  closes B  ->  a fourth window opens
  closes C  ->  the new one is never closed, and a handle outlives its window
                by exactly as long as it takes to become dangerous
```

So a `Collection` stores the **rule** that defines membership and re-runs it every
iteration. This is more expensive and it is the only version that is correct.

`Query` wraps `directorapi.ElementQuery` rather than restating it. A second definition
of "a Save button in Notepad" would drift until a collection matched something a
singular request would not — the worst possible divergence, because it would be
invisible until a bulk action hit the wrong control.

## Bounded by construction

| bound | value | what it bounds |
|---|---|---|
| `MaximumItems` | 50 | one collection's membership |
| `MaximumIterations` | 50 | one command's total iterations |
| `MaximumCollections` | 10 | one program's collections |

Exceeding a limit **rejects** rather than truncating. Acting on an arbitrary prefix of
what someone asked for is doing something they did not ask for. The one case where
fewer than all matches is correct is when the user asked for a prefix — `Take` /
`FromEnd`, "the first three results" — which is a request, not a truncation
(`TestABoundedPrefixIsNotATruncation`).

A query that constrains nothing is refused: "act on everything on screen" is never a
request worth guessing at.

## Ordering is explicit

`OrderingDocument` (the application's own order, preferred — it is the application's own
answer), `OrderingVisual` (top-to-bottom then left-to-right, which is what a person
means by "the first three"), `OrderingWindowZ`, `OrderingScore`.

Explicit because iteration order is user-visible. An unordered set taken from map
iteration would make the same request do different things on consecutive runs
(`TestOrderingIsDeterministicAcrossRuns`).

## Membership identity is semantic

A member is tracked by a `SemanticKey` — a digest over application, role, normalised
label, the native id *when the provider offers one*, and the query. Not by position, and
not by handle.

- it survives movement, and still distinguishes two members (`TestASemanticKey…`);
- the native id is used when available and never required — many applications expose
  none, and demanding it would make collections unusable in exactly those applications;
- a member with no durable identity at all is **refused**, not guessed at
  (`TestAMemberWithNoDurableIdentityIsRefused`).

Labels are normalised for case and whitespace, which change between observations of one
control for reasons that have nothing to do with identity.

## Empty is not unobservable

Two answers that a boolean would collapse and that mean opposite things: *there are no
matching items* is a completed request, and *the tree cannot be read* is a failure to
look. Kept distinct through capture, through iteration, and across a clarification pause
(`TestEmptyAndUnobservableAreDifferentAnswers`, `TestAnEmptySetIsAnAnswerAndAnUnobservableOneIsNot`,
`TestEmptyAndUnobservableStayDistinctAcrossAPause`).

## The loop belongs to the Director

`ForEach` is a Director construct, not a Marco one. Marco executes one deterministic
member action at a time and knows nothing about the set. Between members the Director
must observe, re-resolve, evaluate policy and verify — none of which a language-level
loop could do without becoming the Director.

Each member is a full pass of the ordinary single-step path, so every member is
independently observed, compiled to legal Marco, executed and verified
(`TestAnIterationActsOnEveryMemberOnceAndVerifiesEach`, `TestEachIterationObservesFreshly`).

- a member that fails stops the rest **immediately** — running member 4 against a world
  member 3 failed to produce is the sequencing hazard, at bulk scale
  (`TestAFailedMemberStopsTheRestImmediately`);
- partial success is never reported as success (`TestPartialSuccessIsNeverReportedAsSuccess`);
- cancellation is checked before the next member, never mid-action
  (`TestCancellationStopsBeforeTheNextMember`);
- each successful member gets its own Action Graph node carrying collection provenance,
  and a member node is **not replayable on its own** (`TestACollectionMemberIsNotReplayableOnItsOwn`).

There is deliberately no `while`, no nesting and no parallelism. Each of those changes
the loop from "a bounded set, one at a time" into something whose extent cannot be shown
to a person before it runs.

## Progress is classified, not assumed

Verification already decided *whether* something happened. Bulk needs to know **what**,
because the answers differ in what they imply about continuing:

- **member removed** — normal, and what a successful "close every window" looks like;
- **member state changed** — normal, and what "focus every item" looks like;
- **member unchanged and still there** — the dangerous one. Repeating an ineffective
  action fifty times is the loop this classification exists to catch by name.

The vocabulary is closed, the precedence is deterministic and total, and every kind is
derived from evidence the verifier already gathered — no extra observation, and
`TestClassificationNeverReadsCoordinates` holds it to semantic evidence. Only real
progress permits advancing (`TestOnlyRealProgressPermitsAdvancing`).

## Bulk is a second policy gate

Per-member policy runs anyway, because every iteration goes through the ordinary path.
It is necessary and it is not sufficient:

> Repeating a safe action many times can create an unsafe outcome.

One click on a button is low risk; fifty clicks on fifty different buttons is a
different act with a different worst case. Member-level policy can never see that,
because at each member it is looking at one action. So a collection-level gate runs
**before the first member** and is rechecked at every member.

The permitted set is a closed **allowlist** — `focus` and `activate`. A denylist is
wrong by default for anything it has not heard of, and "invoke every control whose
effect I do not know" is precisely the request that should not be guessed at
(`TestAnUnknownOperationIsRefusedRatherThanAssumedSafe`).

**Click is known and never silent in bulk.** Its effect is whatever the control does —
one may toggle a checkbox and the next may be "Delete all" — so a bulk click always asks.
"Every" is not consent (`TestClickIsNeverSilentInBulk`).

| threshold | value | meaning |
|---|---|---|
| `BulkAutoApproveLimit` | 5 | low-risk members actionable silently — about as many as a person holds in mind when they say "every" |
| `BulkConfirmationLimit` | 20 | the most actionable *with* confirmation |
| `BulkAbsoluteLimit` | 50 | the ceiling nothing crosses, refused before anything else runs |

The request is **typed**, not a phrase: deciding bulk risk by matching request text would
make safety depend on wording, letting "click every X" and "click all X" differ. Member
labels are carried only to detect a destructive-looking target, and are never rendered
into the confirmation prompt — a label may carry private text, and a prompt is the last
place it should surface (`TestPolicyReasonsCarryTheCountAndOperationButNoMemberText`).

A confirmation is invalidated by a changed count (`TestAChangedCountInvalidatesAConfirmation`):
the user approved acting on *seven things*, not on whatever is there now. An empty set is
never silently approved, and no member runs before bulk policy passes
(`TestNoMemberRunsBeforeBulkPolicyPasses`).

## Drift across a clarification pause

> An ordinal belongs to one offered list in one observed world.

The user is offered `1. New tab` / `2. New window`. While they think, a "New folder"
appears at the top. They say "the first one". A system that applied the ordinal to
whatever list it now holds would click a control the user never saw offered, chosen by
an answer they gave about something else.

So an ordinal is never applied because the fresh list is merely long enough. `Fingerprint`
records the ordered membership *as offered* — query digest, ordered key digests, matched
count, and **no members** — and the answer is applied only when the position it referred
to still means what it meant. A contender that moved is not selected by position; one
that disappeared is reported honestly rather than substituted; a different query makes
the answer untrustworthy outright.

The offer carries an `EventID` that changes when the offer changes
(`TestAnEventIDChangesWhenTheOfferChanges`), and `ClarifyPayload.EventID` is validated
**explicitly** rather than inferred — without it, an answer written for one contender
list would be applied to whichever list happens to be pending when it arrives, which is
the same failure as a stale ordinal reached through the transport instead of through the
world. An absent id (an older client) is compatible; only a mismatch is refused.

An ambiguous member pauses the iteration rather than stopping it, and resuming continues
at that member's real position with the processed ledger intact — iterations 1 and 2 do
not run again (`TestAnAmbiguousMemberPausesRatherThanStopping`).

## What is observable

```
director collections                      the running or paused program's collections
director collections --json
director explain collection <name>        one collection's full account
director status                           progress, beside values and clarifications
```

Counts, queries, orderings, limits and progress — never a member list. `explain
collection` states what is *not* kept, because a reader cannot see an absent field:

```
Processed members:
  N semantic key(s), digests only
  no element ids, handles or coordinates are retained
```

Lifecycle events (`collection_bound`, `collection_capture_started`/`completed`,
`collection_policy_started`/`completed`, `collection_empty`, `collection_unobservable`,
`collection_membership_changed`, `collection_reordered`, `collection_paused`/`resumed`,
`collection_completed`, `collection_cleared`) are emitted as value events and carry no
member text (`TestCollectionLifecycleEventsAreEmittedSafely`). An unrecognised kind from a
later build passes through rather than being dropped.

`director status` renders the section only when a program holds a collection: the server
sends nil rather than an empty snapshot so an idle status says nothing about collections
instead of printing a heading that implies an iteration is under way.

## Lifetime

A collection lives exactly as long as its program. A completed collection is **gone**,
and nothing reconstructs one from its iteration nodes — doing so would resurrect exactly
the stale membership the design refuses to keep (`TestAnEnvironmentEndsWithItsProgram`,
`TestCollectionsDisappearWhenTheProgramEnds`). A collection cannot be rebound after
capture, for the same reason a captured value cannot: a later step re-pointing what an
earlier step named would make "iterate items" mean something different depending on when
it ran.

The diagnostics answer from the running program first, then the paused one — a paused
collection is still alive, waiting for an answer that may arrive from another process,
and its progress must stay inspectable until it resumes or is abandoned.

## The control-plane lock

`ActiveCollections` and `ActiveValues` read the paused program under `pausedMu`, **not**
under the command lock. `Runtime.mu` is held for the entire duration of desktop work, so
reading the paused program under it made `director status`, `director collections` and
`explain value` block behind whatever command was in flight — the control plane going
silent exactly while a slow command made someone want to ask what it was doing. The
common case was the worst one: a program *with* collections set `activeCollections` and
returned before the lock, so it was the ordinary command with no collections at all that
hung status.

Ordering: the command path takes `mu` then `pausedMu`; the control plane takes `pausedMu`
alone.

Both guards in `cmd/director/lockrule_test.go` were verified to fail against the
unfixed code — the textual scan now follows same-receiver helper calls, because the
delegation is how the bug hid (`ActiveCollections` mentions no lock; the `mu` was one
call down in `liveCollections`).

## Known gaps

- **Nothing enforces the bulk allowlist's completeness.** Adding an operation is a
  judgement recorded in `bulkAllowlist`, not a check a new capability must pass — the
  same gap `director-cancellation.md` records for `CancellationMode`.
- **Bulk confirmation is a count and an operation, never a preview.** A user confirming
  "focus 12 items" cannot see which twelve without the labels the prompt deliberately
  withholds. Both positions are defensible; the privacy one was chosen, and it does mean
  the confirmation is less informative than it could be.
- **`MaximumIterations` and `MaximumItems` are the same number**, so a single collection
  at its limit exhausts the per-command budget. Correct today because nesting does not
  exist; it stops being obviously correct the day a program iterates twice.
