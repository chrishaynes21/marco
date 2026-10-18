---
type: adr
status: accepted
date: 2026-10-18
decision: ACCEPTED — a demonstration is kept as three separate records with three different jobs. The RAW DEMONSTRATION is what the person physically did, written durably beside the routine, never derived from an interpretation and never consulted to act. The LEARNED PROCEDURE is Marco's reading of it and is the only thing a performer loads; a person may correct it, and a correction removes a step from it without altering the raw record. GUARDED EXECUTION is unchanged. A coordinate may live in the first and still may not live in the second.
supersedes: []
affects:
  - learning
  - perception
source_paths:
  - internal/director/workflowstore/evidence.go
  - internal/director/workflowstore/correction.go
  - internal/director/observe/input.go
  - internal/platform/navsource/navsource.go
  - cmd/director/whatidid.go
  - cmd/director/correctcmd.go
---

# ADR-235 — three records, and only one of them may act

## The owner's statement, which is the decision

> Marco remembers what it thinks the user taught it better than it remembers exactly what the
> user did. That needs to change before we invest much more in interpretation.

And the shape they gave it:

| record | what it is |
|---|---|
| raw demonstration | every actual click, keypress, release, coordinate, timestamp, focus change and associated observation. Preserved durably. |
| learned procedure | the subset Marco interpreted as intentional and repeatable. Supports corrections, uncertainty, names and step ordering. |
| guarded execution | the actions Marco can ground and safely perform, with verification and immediate cancellation. |

## Why it needed saying

[[ADR-234-the-person-does-not-have-to-prove-the-click]] stopped a demonstrated press from being
discarded because perception could not interpret it. It fixed the press and left the rest of the
demonstration in the same state: the production teaching path kept **counters**.

```
input: 6 of 11 event(s) classified, 3 key_release, 2 no_session
```

Eleven events, and not one of them could be named, placed in time, or recognised as the thing
the person remembers doing. `director capture-game-session` had kept a timeline since before any
of this — every input with its own timestamp, device and pointer position — and the path a
person actually teaches through threw it away one look at a time, inside `splitNav`, which took
the pointer positions out of each drain and counted the rest.

Two consequences, both measured rather than argued:

1. **A press that WORKED lost its coordinate.** `ClickTargets` filed each press as attributed,
   inert or blind, and only the last two kept the press. So the coordinate of the one click in a
   session that Marco understood was the one coordinate the session discarded.
2. **A tap, a hold, two rapid presses and a chord were one record.** The releasing edge and the
   auto-repeats were counted and dropped, so nothing could tell them apart —
   `received: 11, classified: 6, key_release: 3` is all four.

## The decision

**1. The raw demonstration is written durably, beside the routine, in its own file.**
`evidence/<name>.json`, linked to `sequences/<name>.json` by name AND by the artifact's own
`SavedAt`. The link is checked on every read: a routine that was re-demonstrated leaves the
earlier session's hands on disk under the same name, and reading those as evidence of the new
one is the one way a companion record could lie about what somebody did.

**2. It is assembled from the session, never from the draft.** A draft is Marco's reading and
leaves things out — the press no proposal contained, the key whose effect was never observed,
the release that bounded a hold. Those are exactly the things a person asks about afterwards,
so the record they are in must not be derived from the reading that discarded them.

**2a. And it speaks the ROUTINE's language, not the session's.** A press is recorded against
`screen_a` and `control_1` — the artifact's own labels, allocated by the same function
`SaveWith` uses — and never against `visual_1` or `target_2`. This was not the first design: the
first one wrote the session identities, and
`TestAVisualScreenCannotReachDurableMemory` refused it, correctly. A weak identity written into
a durable file invites a later reader to match on it across sessions, which is the exact failure
[[ADR-181-a-demonstration-marco-can-keep]] exists to prevent. The LOOK is kept either way, and
it is the stronger reference: a press on a screen the saved route does not contain has no label
and still has its look, its position and the number of regions that held it.

**3. A coordinate may live there and still may not live in the artifact.** This is the whole of
why keeping one is safe, and it is the owner's own sentence:

> A demonstrated coordinate is authoritative evidence of where the person acted. It must not
> become an unconditional coordinate replay instruction.

[[ADR-181-a-demonstration-marco-can-keep]] and
[[ADR-183-remembering-recognising-and-being-allowed-to-act]] are unchanged: the artifact still
carries no press position and still cannot authorise anything. Nothing that performs reads the
raw record, and `workflowstore`'s own import walk still refuses every path to an effect.

**4. The quiet edges of a press become evidence, and are not navigation.** A release and a
repeat are emitted with the intent their own press was classified as, into a **separate
collection** on the subscription. A correlator drains `Drain()` and gets exactly what it always
got — one event per press — because an edge is not in the collection it reads. The separation is
structural rather than a filter: a filter is a thing somebody removes.

**5. A person may correct the learned procedure, and the correction does not touch the raw
record.** A correction drops one step and keeps it, with the person's own words, in the
artifact's own `Corrections`. The raw record is a different file and the correction path cannot
write it.

**6. And no new format, engine or question.** The record is `observe.InputEvent` and
`observe.ClickObservation`, both of which already existed; the store is a fourth directory in
the store that already had three; the correction is a step list with one step fewer.

## What this is not

It is **not** a widening of the privacy boundary in substance. `InputEdge` is three words — a
press, a release, a repeat — and the press is the empty value. An edge carries the closed intent
its press carried and nothing else: a key nobody recognised emits none at all, so the record
still cannot say that an unrecognised key was held, let alone which one it was. The allowlist in
both boundary tests was widened deliberately, with the argument written where the change is.

It is **not** permission to act on a coordinate. A press with no candidate region still produces
no step and is still refused at replay.

It is **not** a record of the screen. No frames, no screenshots, no window titles — the window
is identified by the SELECTOR the person typed, because a game window's title is the commonest
place an account name appears.

It is **not** unbounded. `MaxEvidenceMoments` bounds the record at the write, only looks that
CARRIED input are moments, and a truncated record says how much it dropped.

## Enforced by

- `internal/director/workflowstore` `TestTheRawRecordKeepsTheCoordinateOfThePressThatWorked` —
  the press that resolved to a control keeps its position, screen and look.
- `internal/director/workflowstore` `TestAHoldAndATapAreDifferentInTheRecord` and
  `TestTwoKeysHeldTogetherAreVisibleAsAnOverlap` — the four shapes are distinguishable, and an
  overlap is reported as an observation rather than resolved into an intention.
- `internal/platform/navsource` `TestAnEdgeIsNotNavigation` — the navigation stream carries one
  event per press, and `admissibleInputs` refuses an edge offered to it as well.
- `internal/platform/navsource` `TestAKeyThatIsNotNavigationEmitsNoEdges` — the privacy half.
- `internal/director/workflowstore` `TestTheRawRecordCannotCarryWhatSomebodyTyped` — the durable
  file carries no key, code, scan or character.
- `internal/director/demo` `TestEvidenceIsReturnedOnlyForTheSaveItDescribes` — the link.
- `internal/director/workflowstore`
  `TestACorrectedProcedureIsCorrectForAReaderThatIgnoresCorrections` — the argument for not
  bumping the artifact version, asserted so that it cannot stop being true.
- `cmd/director` `TestAnAccidentalClickIsCorrectedAndTheDemonstrationSurvivesIt` — the whole
  path: demonstrate, save both, correct, and read the demonstration back unchanged.
- `internal/director/workflowstore` `TestTheRawRecordNamesScreensTheWayTheRoutineDoes` — the
  record carries `screen_a` and `control_1` and no session identity, asserted over the bytes.
- `cmd/director` `TestAMomentBehindAnotherWindowIsNamedAndNotDiscarded` — a moment taken while
  something covered the application is reported and kept.
- `cmd/director` `TestTheRawRecordAndTheExecutableArtifactAreSeparate` and
  `TestTheCorrectionAndRawRecordVerbsCannotAct` — two files, and neither verb can press
  anything.

## What is in the record, against the owner's own list

| asked for | kept as |
|---|---|
| every actual click | `Presses`, every one, in the order made |
| coordinates, window-relative | `At`, normalised against the window it was captured from |
| timestamps | `AtMS` on each event and each moment, relative to the session's start |
| keydown / keyup | a press event and a `release` edge of it |
| repeats | a `repeat` edge per auto-repeat, so a hold is visible as a hold |
| chords | overlapping press intervals, reported as an overlap and not as an intention |
| focus / window provenance | the selector as typed, the number of window generations, and `Front` per moment |
| references to visual observations | `Look` on every moment and every press, and the routine's screen label |
| exact physical input events | the closed INTENT, never the key. This is the one line of the list that is deliberately not met in full, and `observe/input.go` is the reason: a promise not to record text is worth less than an inability to |

And deliberately not in it: no frames, no screenshots, no OCR text, no window titles, no key
identity. The owner's own bound — *avoid unnecessary retention of unrelated screen data or
sensitive input content* — is the reason, and `MaxEvidenceMoments` is the other half of it.
