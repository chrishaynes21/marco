---
type: adr
status: accepted
date: 2026-11-01
decision: "ACCEPTED — THE RAIL'S BUILD STAMP HAS ITS OWN READ, AND THE SCENES VIEW'S WORK STOPS WHEN SCENES IS WITHHELD. [[ADR-283-the-alpha-offers-one-journey-and-the-window-explains-itself]] left `pollLearning` running at ten seconds instead of four hundred milliseconds, because that one poll paints two things and only one of them is on the Scenes view: the other is the rail's build stamp, which is the one thing in this window that makes a stale director visible and whose absence CLAUDE.md records as having cost the owner a day of recordings. The Architect ruled that cadence a disclosed interim compromise rather than completion, and the split is the correction. `Acts.BuildStatus` is the rail's own question — this window's revision, the recorder's, and whether they differ — answered from the SAME two readings `Learning` answers from, so the rail and the Scenes card cannot disagree about a build. It asks the recorder's progress stream and nothing else: no demonstration listed, no transcript copied, no phase worked out. The page polls it every ten seconds WHATEVER IS OFFERED, because the answer changes only when a recording starts or ends and the call is a pair of strings. `pollLearning` goes back to four hundred milliseconds, returns immediately and WITHOUT RESCHEDULING when Scenes is not offered, is started only once `Features` has said it is, and no longer paints the rail at all — the mismatch banner on the Scenes card is still its own. So the Scenes-specific query and painting stop entirely and no stale-Director information is hidden. AND A RECORDER THIS BUILD CANNOT READ IS NOW A MISMATCH. `recordingNow` has a third answer — a progress stream written by a build whose format this one does not understand — and `Learning` reports it as a sentence on the Scenes card without setting `Mismatched`, so the rail stayed quiet about the plainest kind of stale director there is. It is a mismatch in `BuildStatus`, with no second revision to show because the stream is the thing that would have carried one. That is a deliberate difference from what the rail showed before, made in the one answer whose whole job is to say so."
supersedes: []
affects:
  - execution
source_paths:
  - plugins/studio/buildstatus.go
  - plugins/studio/learn.go
  - plugins/studio/frontend/index.html
---

# The rail keeps its build stamp when Scenes is withheld

The follow-up [[ADR-283-the-alpha-offers-one-journey-and-the-window-explains-itself]] disclosed
and did not make. The Architect:

> Keep the build stamp; stop hidden Scenes work. The 10-second cadence is a disclosed interim
> compromise, not completion of the polling requirement. Separate the shared build-status refresh
> from Scenes-specific queries and painting.

## Why one poll was doing two jobs

`Learning` returns a demonstration's whole state — phase, transcript, press counts, what it
learned — and the page took two fields off it to paint the rail's foot. So withholding the Scenes
view left a choice between a poll running two and a half times a second for a surface nobody can
reach, and a rail that stops saying which build is recording.

The stamp is not worth losing. Studio spawns the `director.exe` and the `marco.exe` beside
**itself**, so a stale sibling is invisible: every binary looks freshly built, the window is
current, and the recordings come from whenever that director was last made. CLAUDE.md:

> It cost the owner a day of recordings.

## The split

| | |
| --- | --- |
| `Acts.BuildStatus` | this window's revision, the recorder's, whether they differ. Asks the recorder's progress stream and nothing else |
| `pollBuild` | ten seconds, **whatever is offered**. The answer changes only when a recording starts or ends, and the call is a pair of strings |
| `Acts.Learning` | unchanged, and the Scenes view's own |
| `pollLearning` | four hundred milliseconds, returns **without rescheduling** when Scenes is not offered, started only once `Features` has said it is, and paints the rail no longer |

Both answers come from the same two readings — `a.myBuild()` and `recordingNow()` — so the rail
and the Scenes card cannot disagree about a build. The mismatch banner on the Scenes card is still
`pollLearning`'s, because it is on that view; the rail's stamp is `pollBuild`'s, because it is
not. Painting one element from two polls at two rates would be two answers to one question.

## The recorder this build cannot read

`recordingNow` has a third answer: a progress stream written by a build whose format this one does
not understand. `Learning` reports that as a sentence on the Scenes card and does **not** set
`Mismatched`, so the rail stayed quiet about it — and a recorder whose stream this build cannot
parse is the plainest kind of stale director there is.

It is a mismatch in `BuildStatus`. There is no second revision to show, because the stream is the
thing that would have carried one.

This is a deliberate difference from what the rail showed before, made in the one answer whose
whole job is to say so.

## What this does not do

- It does not change `Learning`, which still carries `Build`, `Recorder` and `Mismatched` for the
  Scenes card.
- It does not add a control for the flags.
- It does not make the rail's read event-driven. Ten seconds on a two-string call is a cadence,
  not a compromise about what is shown.

## Enforced by

- `plugins/studio` `TestTheAlphaWindowOffersOneJourney` — **rendered in headless Edge**, with
  Scenes withheld: the Scenes view's read is made **zero** times, the rail's own read is made,
  the stamp names the recorder's build beside this window's, and it is marked as a mismatch.
- `plugins/studio` `TestTurningBothFlagsOnGivesBackTheWholeRail` — rendered, the converse: with
  Scenes offered the Scenes read runs fast, and the rail's own read still runs.
- `plugins/studio` `TestTheRailSaysWhichBuildThisWindowIs` — with nothing recording and Scenes
  withheld: this window's revision, no recorder named, and **no alarm** — `buildid.Differ` and
  not a naive comparison, or every window with no recorder beside it would redden.
- `plugins/studio` `TestTheRailNamesARecorderOfAnotherBuild` — both revisions, and the mismatch.
- `plugins/studio` `TestTheRailMarksARecorderItCannotRead` — a stream this build cannot parse is
  a mismatch, with no revision invented for it.

## See also

- [[ADR-283-the-alpha-offers-one-journey-and-the-window-explains-itself]] — where the compromise
  was disclosed
- [[Studio-alpha-UI-slice-2026-10-31]]
