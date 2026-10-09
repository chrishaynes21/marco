---
type: decision
status: accepted
date: 2026-10-09
affects:
  - passive-observation
  - navigation
source_paths:
  - internal/director/observe/input.go
  - internal/platform/navsource/navsource.go
  - internal/platform/padsource/session.go
  - internal/platform/padsource/synthetic.go
  - cmd/director/padwiring.go
---

# ADR-150 — silence is not evidence about a device nobody watched

## Context

A live four-minute Trackmania session reported this:

```
navigation   no physical input reached the producer this session
```

beside forty observed screen-state transitions, every one unattributed. The sentence is true
about the producer and reads as a fact about the player. The only producer wired was
`navsource`, which installs `WH_KEYBOARD_LL` and `WH_MOUSE_LL` — and there is no hook for a
gamepad, because XInput is a poll. `InputStats.Unavailable` was empty, so the record said
"navigation observation is working and nobody pressed anything".

The adapter for the missing half already existed. `internal/platform/padsource` was built for
`director capture-game-session`, is read-only by construction — the Windows backend binds
`XInputGetState` and not `XInputSetState` — and already emits the same `observe.InputEvent`.
Nothing in the Director subscribed to it.

## Decision

### Provenance travels on the event

`observe.NavDevice` is a closed vocabulary of three words — keyboard, mouse, controller — on
`InputEvent.Device`, admitted like every other vocabulary here: an unrecognised word is
cleared and the event kept, because provenance is an attribution and losing it is not losing
the observation.

This widened the keylogger guard twice, and both are recorded in
`TestRawKeyIdentityCannotCrossTheBoundary` rather than argued in a commit message. `device` is
on the forbidden-name list because a field by that name is the natural home for a device name
or a handle; the exemption is pinned to the exact field AND the exact type, so `Device string`
or `DeviceID` still fail. What actually holds the line is the assertion beside it: the
vocabulary is three words, each short, and a product name does not pass `Known()`.

### Four states, and only one of them makes quiet meaningful

`DeviceObservation` replaces a single `Received` count with a record per device:

- not wired at all — no record, and `Unobserved` names the device anyway;
- wired and unavailable — the platform cannot read it here;
- wired, available, and the hardware absent — a controller nobody plugged in;
- wired, available, present — and NOW zero events means zero presses.

The third is the one that cost the run. A source polling an empty slot answers "nothing
pressed" forever and is indistinguishable from a player sitting still.

`InputStats.CanAttributeSilence` asks the VOCABULARY, not the wired set. A session that never
wired a controller has established nothing about controllers, and asking only the producers that
exist is how a system comes to answer a question nobody could answer. `SilenceMeans` is the
sentence a report prints, and the report prints it on the busy path too: four hundred keystrokes
beside an unwatched pad support "this is what the player did" no better than none do.

### The Director watches the pad, and says so when it does not

One subscription per session, drained on the sampler's own tick and merged with the hook stream
in time order by `observe.MergeInputs` — in the consumer's package, because neither producer
knows about the other and a correlation asks one question across every device a player touched.

`$MARCO_NO_CONTROLLER` turns it off, on by default, and the costs are not comparable to the
detector's: one function call every 8ms against a 1.25GB model and a two-second inference.

**No injection.** There is no setter bound. Wiring this adds an observation path, which is what
[[ADR-010-passive-observation-cannot-execute]] protects rather than what it forbids.

### What is NOT decided

The deletion of `newPadSource()` from `NewRuntime` is caught by nothing — no test can enter
`NewRuntime` holding a controller, the same position ADR-148 records for the shadow gate. What
has changed is that the omission is LOUD: with no pad source every session's device records name
the controller as unwatched, the report prints `NOT WATCHED: controller`, and
`CanAttributeSilence` goes false.

## Enforced by

- `TestTheProducerNamesTheDevicesItWatches` (`navsource`) — a hook producer covers keyboard and
  mouse and does NOT claim a controller, so its confidence cannot be inherited.
- `TestAnEventSaysWhichDeviceItCameFrom`, `TestAPointerPressSaysItCameFromTheMouse`.
- `TestAnOpenSessionReceivesWhatThePlayerPressed`, `TestAClosedSessionStopsReceiving`,
  `TestNavigationWithNobodyWatchingIsNotKept` (`padsource`) — session-bounded retention.
- `TestAnUnpluggedControllerDoesNotClaimTheMachineWasQuiet` — the three producers that all
  report zero, and which one of them is evidence.
- `TestASessionSubscribesToTheController`, `TestTheSampleCarriesWhatThePlayerDidOnEitherDevice`,
  `TestTheSampleSaysWhichDevicesWereWatched` (`cmd/director`) — through
  `liveSampler.Sample`, because the first version of these drained the two subscriptions itself
  and the mutation that deletes the merge from production survived it.
- `TestTheReportSaysWhichDevicesItWatched`, `TestAQuietSessionWithAnUnwatchedDeviceSaysSo`,
  `TestABusySessionWithAnUnwatchedDeviceIsStillQualified`,
  `TestAProducerThatNamesNoDevicesDoesNotClaimCoverage` — the report, including that an absent
  answer is not a complete one.
- `TestADeviceWordOutsideTheVocabularyIsCleared`, `TestMergingTwoDevicesKeepsBoth`,
  `TestRawKeyIdentityCannotCrossTheBoundary` — admission, the merge, and the privacy boundary.
- `TestAControllerNobodyWiredIsLoudRatherThanSilent` — the mechanism that makes the one
  uncatchable omission self-reporting.
