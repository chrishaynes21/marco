---
type: decision
status: accepted
date: 2026-10-09
affects:
  - passive-observation
  - vision
source_paths:
  - internal/director/perception/selection/selection.go
  - internal/director/perception/selection/cost_test.go
  - internal/director/perception/providers/visualstate/provider.go
  - internal/director/observe/track.go
  - cmd/director/trackwiring.go
---

# ADR-152 — slow discovery, fast tracking: the measurement and the shape

## Status of this ADR

**Built.** It was written as a design with one number missing; the number was measured on a real
game ([[Experiment-024-watching-a-player-navigate-a-real-menu]]) and the path was then built on
it. The section below the measurement describes what exists; "What was still missing" records
what the number turned out to be.

The authority half — what a fast read may conclude — is in `internal/director/observe/track.go`;
the pixels half is in `cmd/director/trackwiring.go`. Everything this ADR said the look must
carry, it carries.

## Context

A ScreenParser inference costs **1972ms median, 2326ms p95**, measured live through the
production loop. The authoritative sampler runs at 500ms. So the interface belief gets about one
look every two to four seconds, and at that rate Marco can follow a menu somebody moves through
every few seconds and cannot follow one they move through quickly.

Those are two different jobs. Finding out that there is a list of nine entries at these
coordinates is expensive and rarely changes. Finding out which of them currently holds the mark
is cheap and changes constantly.

## What was measured

`selection.Measure` — the reader the visual selection work already uses — is pure Go in the
engine. No model, no plugin, no cgo. On a 1920×1080 frame, over the nine entries measured on the
real Trackmania menu:

| | |
| --- | --- |
| one entry | ~61µs |
| a 9-entry menu | **597µs** |
| a ScreenParser inference | 1972ms |
| ratio | **~3300×** |

`TestReadingAMenusShadeIsCheapAgainstAnInference` asserts the order of magnitude rather than the
number, so the claim this ADR rests on fails loudly if it stops being true.

**The reader is not the cost.** Whatever a fast tracking path costs, it is the capture.

## What the existing parts already allow

Nothing here needs a new sensor, and that was the surprise of the investigation.

`visualstate.Provider.Capture(ctx, region, window)` already returns a `Snapshot` carrying
`Image`, retained only while held and never persisted — and the live sampler already calls it
every cycle for the visual lease. `selection.Measure` takes an `image.Image` and a rectangle.
The remembered geometry is already on the candidate group the detector's last discovery
produced.

## The rule that shapes it

`visualstate` refuses a region beyond `MaxRegionArea` (4,000,000 px — "a large dialog, not a
desktop") with a reason that is a design position rather than a limit:

> a region that large is a screen, and watching screens continuously is what this avoids

That rule is right and the fast path must not weaken it. It also happens to prescribe the
correct shape: the region to re-read is **the candidate group's envelope**, not the window. On
the measured Trackmania menu that envelope is a fraction of the screen, which satisfies the
bound by a wide margin — and the discovery that found the entries is exactly what supplies it.
`Provider.Observe` already refuses vision with no region on the same grounds: *"a caller that
has not said where to look has not decided what it wants"*. A discovery is how the caller says
where to look.

## The shape, if it is built

**Discovery** stays as it is: ScreenParser, behind its cadence and its purchase policy,
producing candidate regions and an envelope.

**Tracking** captures only the envelope and measures shade over the remembered rectangles,
feeding the interface belief a look that is explicitly a fast read.

Three things such a look must carry, and they are the whole correctness argument:

1. **Two timestamps, not one.** When the geometry was discovered and when the pixels were read.
   A fast observation presented as contemporaneous with an old discovery is the defect, not the
   feature.
2. **Candidate provenance.** Which discovery the rectangles came from, by its sample number,
   so a belief can say "entry 3 of the list found at sample 41, read at sample 58".
3. **Geometry freshness, refused rather than degraded.** If the window moved or resized since
   the discovery, the rectangles describe pixels that are no longer there.
   `capture.Moved` already makes that judgement for the detector's own frames and is the same
   check. `TestGeometryOffTheFrameReadsAsNothing` pins the half that matters most: a rectangle
   off the frame reads as the ZERO reading, which is distinguishable from a real dark row — so
   "no reading" and "nothing is highlighted" cannot be confused. A fast read against stale
   geometry must be refused, because a confident wrong selection is worse than none.

And a bounded staleness: a discovery expires. The belief machinery already has the vocabulary
for this — `InterfaceBelief.Stale`, `Unconfirmed`, and `interfaceForgetLooks` — so a tracking
look is a confirmation of an existing belief and never an identification of a new interface.
**Only a discovery may identify an interface. Tracking may only move the mark inside one.** That
is the same shape as the authority boundary the whole subsystem rests on: a cheap sensor may
update a value, never establish a structure.

## What was still missing, and what it turned out to be

**The capture cost.** Measured live on Trackmania, through `director vision --application`:

| | |
| --- | --- |
| capture, full 1920×1080 window | **24ms** (32–39ms on repeat reads) |
| detect (ScreenParser) | 1.05–1.25s |
| construct | 0–1ms |

Capture is **~2% of a look**, and more favourable than this ADR assumed: at 24ms for the WHOLE
window the envelope is needed to honour `MaxRegionArea`, not to afford tracking. So the cadence
was chosen from the other side of the arithmetic — `DefaultTrackInterval = 250ms` spends under
a tenth of the machine on this and leaves the rest to the game, which is the condition an
experiment a player is not supposed to notice has to meet.

**And a second number the same run produced, which changed the shape of the build.** The
observation cycle measured **2.6s against a configured 500ms**, with 360 late samples, because a
ScreenParser inference runs synchronously inside the cycle it was offered in. A tracking read
taken once per cycle would therefore be taken once per 2.6 seconds, and the fast path would not
be fast.

So tracking runs on **its own ticker**, off the observation cycle. That is what "without
blocking on ScreenParser" has to mean on this machine, and it is the one thing this ADR did not
anticipate.

## What the build added that the design did not have

**The achieved cadence is reported, never the configured one.** `TrackingStats.Cadence()` is
computed from the timestamps of the looks that happened. The slow path asked for 500ms and got
2.6s and no report said so; that is not allowed to happen twice.

**Every refusal has its own number.** "Tracking produced no readings" has five causes that send
a reader to five different places — nothing identified a list, the geometry went stale, the rows
were not the list's, no row could be measured, the sampler refused before the pixels were read —
and one counter for all of them would answer none of them.

**A fast read counts as a confirmation of the belief, and never as a look that carried
structural evidence.** Without the first half, a belief being confirmed four times a second by
pixels reported itself as unconfirmed, because the cycle the detector sat out marked it stale
immediately afterwards. Without the second, `Observations` and `identified_after` would grow
with reads that identified nothing.

**The liveness check is the sampler's, not the ticker's.** The ticker runs on its own goroutine
and `Runtime.pinnedWindow` is written under `Runtime.mu`; reading it from the ticker is a data
race, and taking the lock instead would stall the ticker for the whole 2.6s cycle — at exactly
the rate it exists to beat. So the sampler, which already holds the lock and already knows the
window, calls `invalidate` when the window moved or was replaced, and the refusal travels to the
belief as a look rather than as a silence. The cost is a gap of up to one cycle, bounded on the
other side by `TrackStaleness`, and it is written down on the type rather than papered over.

**It does not run on an ordinary Director.** A discovery is the only thing that gives this path
geometry and only the experimental detector produces one, so `newFastTracker` returns nil
without it. `visualstate` refuses an oversized region because *"watching screens continuously is
what this avoids"*; the same caution applies to watching a small one for nothing.

## What is still NOT established

**That a tracked selection follows a real player's navigation.** Nothing in
Experiment-024 observed a selection at all, because the detector's geometry never arrived —
which is [[ADR-154-geometry-is-evidence-even-when-the-words-are-not]]'s problem, fixed
separately. The tests below drive the fast path through the production composition with painted
pixels; they do not establish that it tracks a menu somebody is moving through.

**The envelope capture cost.** 24ms is the whole window. The envelope is smaller and has not
been measured on its own, so `MeanCaptureMS` in a live report is the number to read, not an
estimate taken from here.

**Whether the shared reader is right.** The fast path and the slow path feed ONE
`selection.Reader`, so a row's baseline is built from both. That is deliberate — a separate
reader would start from nothing every time a discovery refreshed — but the two measure the same
rows through two different capture paths, and whether that pollutes a baseline is unmeasured.

## Enforced by

- `TestReadingAMenusShadeIsCheapAgainstAnInference` (`internal/director/perception/selection`) —
  the premise of the whole architecture, as an order-of-magnitude bound rather than a number.
- `TestAFastReadMovesTheMarkInsideAKnownList` — the positive case, with both timestamps and the
  gap between them asserted.
- `TestATrackedLookCannotIdentifyAList` — twenty perfectly good fast reads at a session that
  never ran a discovery identify nothing.
- `TestAFastReadChangesOnlyTheMark` — status, where the list is, its entry count, how much
  looking identified it: all the discovery's, all untouched.
- `TestAFastReadAgainstStaleGeometryIsRefused`,
  `TestAFastReadOfTheWrongPartOfTheScreenIsRefused`,
  `TestAFastLookWithNothingMeasurableIsNotAReading`,
  `TestASamplerRefusalArrivesWithItsReason` — the four refusals, each counted by name.
- `TestManyPressesBetweenTwoLooksAreNotManySelections` — the measured burst, holding both
  claims: the intermediate marks are seen, and a selection is never invented per press.
- `TestTheSessionReportsTheCadenceItAchieved` — from the clock, not the configuration.
- `TestASlowLookClearsTheTrackedMarker` — the inverse defect.
- `TestASessionStartsTheFastVisualPath`, `TestEndingASessionStopsTheFastVisualPath`,
  `TestADirectorWithNoDetectorStartsNoCaptureLoop`, `TestTheFastLooksRideOnTheSample`,
  `TestAMovedWindowStopsTheFastPathReadingItsGeometry` (`cmd/director`) — through the
  composition production uses, including that the reads happen at all.
- `TestTheReportSaysWhatTheFastPathRead`, `TestTheReportSaysWhenASelectionWasTracked`.
- `TestTheSameFrameReadsTheSameShade` — determinism, which is what makes a fast re-read
  comparable with a slow one rather than merely adjacent to it.
- `TestGeometryOffTheFrameReadsAsNothing` — the freshness half: no reading and nothing
  highlighted are different answers.
- `BenchmarkMeasureOneEntry`, `BenchmarkMeasureAMenu` — the numbers above, re-runnable.
