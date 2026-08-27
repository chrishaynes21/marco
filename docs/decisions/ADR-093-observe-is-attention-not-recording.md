---
type: decision
status: accepted
date: 2026-08-21
supersedes: []
affects:
  - passive-observation
  - semantic-memory
  - service
source_paths:
  - internal/director/ambient/ambient.go
  - cmd/director/observeambient.go
  - cmd/marco/observecmd.go
  - internal/director/service/protocol.go
  - cmd/director/serve.go
  - acceptance-36a.ps1
---

# ADR-093 — Observe is attention, not recording

## `marco observe` means Marco is paying attention

Somebody types it, then uses their computer normally. Marco keeps up with where they are — which
application, which screen, what just changed — so that "where am I" is instant instead of a
six-second look, and so a future "learn what I just did" has something to read.

Four things it deliberately is not:

| | |
|---|---|
| **not a recording** | nothing reaches disk. No screenshots, no text, no titles. |
| **not a Learn** | no questions, no naming, nothing made permanent. |
| **not authority** | watching grants no permission to act and no claim on the keyboard. |
| **not a second observer** | one observation substrate, and ambient is its lowest-priority consumer. |

## Storage growth tracks novelty, never time

This is the property the whole ambient product rests on, and it is the one a person would be
right to be suspicious about. Leave Marco watching for eight hours on one page: the desktop is
read thousands of times, nothing new happens, and what Marco holds afterwards is the same size as
after five minutes.

So [`ambient.Buffer`](../../internal/director/ambient/ambient.go) is not a log. Every fact is
keyed on a durable semantic identity and carries a COUNT — a Place seen ten thousand times is one
entry saying ten thousand — and the only ordered thing in it is a short recent walk, bounded at
64 moves, which exists so "what did I just do" has an answer and not so the afternoon does.

Bounds are on DISTINCT things, because that is what can grow: 256 places, 512 edges. Past those
the least *recently* seen is forgotten, and the forgetting is counted rather than silent.

**It is transient, and stopping forgets it.** What it carries is the present tense; keeping it
after Marco stops paying attention would make "stopped" a claim about the future only.

## It holds no words anybody could read

Durable subject ids, counts, times, and a two-word provenance vocabulary. No labels, no window
titles, no screen text, no coordinates, no frames. A transient buffer with weaker rules than the
durable store would be exactly where a privacy boundary quietly stops applying, so it has the
same ones.

## The default licence is none

35A split what an observation session may make durable into three named permissions —
`EstablishPlaces`, `AcquireRouteEvidence`, `NameActivatedTargets` — and made the zero value grant
nothing, so a caller has to name a licence to get it.

Ambient watching names none. Sessions go through `Start`, which hands the runner
`observesession.Episode{}`, so **however long it runs it cannot make anything durable**. Not a
policy check that could be forgotten: there is no licence in the object.

Two consequences a person sees, and both are correct:

- **A screen Marco does not know stays unknown.** No naming question — Observe is not an
  interactive acquisition episode, and a background mode that interrupted somebody would be a
  worse product than one that quietly does not know.
- **Degraded perception is not a Place.** A reading that got no further than the window frame is
  reported as degraded and recorded as nothing, exactly as [[ADR-090-a-verified-outcome-is-the-next-step-s-evidence]]
  requires. Layout invariance must not hide provider failure and neither must attention.

## One observer, and ambient is the one that gives way

The registry allows a single observation session: two would contend for the screen and neither
could attribute what it saw. Ambient watching does not change that — it is a SUPERVISOR that
keeps the one substrate busy when nothing else needs it, and waits when Learn, Here or a
performance's verification owns it.

**Yielding rather than layering, and the cost is written down.** The prettier architecture is one
continuous session whose licences and attention change as consumers come and go; that is a
rewrite of the session runner. This achieves the property that matters — one observer, ambient
resumes afterwards — against the machinery that exists. What it costs is that a Learn interrupts
ambient watching rather than borrowing it, so the ambient tally has a gap for its duration.

**The registry is what enforces one observer; the yield only buys quiet.** Deleting the yield
breaks no safety property and the mutation survives the suite — measured. Without it, ambient
asks for the substrate every few seconds throughout somebody else's Learn and is refused every
time, which is churn for nothing.

## Attention falls while nothing happens, and snaps back when something does

A screen reading costs a run of accessibility snapshots — about 200 ms on one measured machine —
so something reading continuously would be a background process a person can feel, and
"affordable enough to leave on" is the product claim.

Attention doubles from 1 s toward 8 s while the desktop is unchanged, and returns to 1 s
**immediately** on any change. The asymmetry is the point: a gradual ramp would make the first
thing somebody does after a quiet afternoon the thing Marco is slowest to notice, which is
exactly backwards.

## An unknown screen does not break the walk through it

Real navigation passes through frames that are neither endpoint — a page part-way through
arriving, a screen Marco has never seen. Those are recorded as nothing, and the walk survives
them: Home → *unknown* → Bluetooth & devices is recorded as Home → Bluetooth & devices.

One line does that, by skipping an unrecognised reading rather than treating it as somewhere
Marco moved to. Measured: the mutation that removes it survived every other test in the file, and
it is the requirement most easily lost.

## Watching is not something Marco did

Activity is the account of what MARCO has done — every row is an action it took, and a person
reads it to see what happened on their behalf. Somebody's own navigation is not that. Writing
ambient observation into it would turn the one surface that says *here is what I did for you*
into a log of what its owner did all afternoon.

So a watching session leaves the durable action graph — which is what Activity reads — untouched,
however much it sees.

## The Director says whether it is watching, either way

`marco observe status` answers it precisely, and `director status` now answers it too: somebody
asking what the Director is doing should not have to know the first command exists. Printed even
when the answer is no, and carried in `StatusPayload.Watching` as a VALUE rather than a pointer,
because a field that is absent when the answer is no cannot be told apart from a field a Director
too old to have it never sent.

Counts, an application and an opaque screen id. Never a label, a window title or a line of screen
text — the same boundary the buffer has, held on the wire by a closed field set rather than by
inspection.

## Who did it is kept

`ByHuman` and `ByMarco`. Ambient watching is what somebody using their own computer looks like; a
performance's transitions are recorded by the walk that made them. Conflating them would
eventually teach Marco its own behaviour back from itself, and tell somebody in Activity that
they did something Marco did.

## A command, not a setting

Always-on watching that somebody did not switch on is the shape of surveillance whatever the
intent. So:

- it is **off** until asked for
- it **says so** when asked — `marco observe status` reports plainly
- it **stops** when told, and the reply is not sent until the loop has actually stopped
- it does **not survive a restart**, deliberately. A durable toggle is a Settings decision with a
  consent conversation attached, and inventing one here would be an auto-start-at-login privacy
  behaviour arrived at by implication.

**Asking does not answer itself.** `marco observe` may start a Director, because it is a request
for something to happen. `marco observe status` and `marco observe stop` may not: a question
about whether Marco is watching must never be answered by making it watch, and asking it to stop
must never bring something into existence to stop.

## What this does not touch

- **No desktop lease.** Observation is the one thing [[ADR-092-one-director-per-home-one-hand-on-the-keyboard]]
  deliberately does not gate, which is what lets a sandbox watch while the real Director works.
- **No authority.** A Play invoked while watching goes through the ordinary door: its own grant,
  the production lease, the shared walker, the same verification.
- **Learn and Here work with watching off.** Both start bounded observation as they always did.
  Ambient watching is a product mode, not a dependency.
- **35C's evidence handoff is untouched.** Ambient samples are ordinary observations; a
  performance's carried proof is still confirmed against a live reading before it is used.

## KNOWN FOLLOW-ONS

1. **No ambient promotion.** Repeated evidence does not become durable knowledge. The seam is
   `ambient.Buffer` → a policy → the existing durable admission path, and the buffer already
   carries counts, first/last times and provenance for it. Deliberately not built: ambient
   observation had to be proved safe before anything could be promoted from it.
2. **"Learn what I just did" is not implemented**, and the buffer holds enough for it: an ordered
   recent walk of source, destination, application and provenance. What it does NOT hold is which
   control was activated, so a route reconstructed from it would know where somebody went and not
   what they clicked.
3. **Learn interrupts ambient rather than borrowing it.** See the yielding note above.
4. **Live acceptance is UNMEASURED.** CPU, memory and sample rate on a real desktop, and the
   Learn-over-Observe integration, have deterministic gates and no live numbers.
   `acceptance-36a.ps1` is the harness that would produce them: `-Watch -Quiet` answers *does it
   grow with time*, `-Watch -Busy` answers *does it notice anything at all*, and neither means
   anything without the other. It drives nothing, which is itself part of what it checks.
5. **`ambientSession`, `ambientBusy` and `ambientIdle` are internal constants** chosen from one
   machine's measured snapshot cost. They are not configurable and should become so before
   anybody runs this for a working day.

## Enforced by

- `internal/director/ambient` — `TestWatchingLongerDoesNotMeanRememberingMore` (ten thousand
  sightings, one entry); `TestARouteWalkedRepeatedlyIsOneEdge`;
  `TestDistinctScreensAreRememberedUpToTheBound` (novelty still grows, up to a bound that reports
  what it dropped); `TestTheRecentWalkKeepsWhatJustHappened`;
  `TestHumanAndMarcoAreNotTheSameEvidence`; `TestTheBufferHoldsNoWordsAnybodyCouldRead`.
- `cmd/director` — `TestWatchingTwiceIsStillWatchingOnce` (one supervisor AND one loop);
  `TestWatchingWaitsForWhoeverElseIsLooking`; `TestShuttingDownStopsWatching`;
  `TestWatchingRecordsNeitherDegradedNorUnknownScreens`;
  `TestAnUnknownScreenDoesNotBreakTheWalkThroughIt`; `TestWatchingRecordsWhatThePersonDid`;
  `TestAttentionFallsWhenNothingHappensAndSnapsBackWhenItDoes`;
  `TestWatchingAllDayDoesNotFillTheDirectorUp` (the same claim at the supervisor, over ten
  thousand readings); `TestWatchingIsNotSomethingMarcoDid` (the action graph stays empty);
  `TestADirectorSaysWhetherItIsWatching`; `TestWatchingFromOnToOffAndGone` (the whole lifecycle
  in order, and the wire shape held closed).
- `internal/director/service` — `TestStatusAlwaysSaysWhetherMarcoIsWatching` (a pointer field
  must fail it); `TestStatusCarriesWhatWatchingHasNoticed`.
- `cmd/marco` — `TestOnlyAskingMarcoToWatchMayStartADirector` (the autostart flag itself);
  `TestAskingWhetherMarcoIsWatchingStartsNothing`;
  `TestAMisspeltObserveVerbDoesNotStartWatching`.

## Related

[[ADR-092-one-director-per-home-one-hand-on-the-keyboard]] ·
[[ADR-090-a-verified-outcome-is-the-next-step-s-evidence]] ·
[[ADR-091-a-place-is-not-its-presentation]] ·
[[ADR-076-a-place-may-say-what-it-appears-to-be-called]] ·
[[ADR-010-passive-observation-cannot-execute]] ·
[[Passive-Observation]]
