---
type: adr
status: accepted
date: 2026-11-03
decision: "ACCEPTED — THE WIDGET'S SETTINGS (overlay.json) HAVE ONE READER AND ONE WRITER, `internal/widgetconfig`, USED BY THE WIDGET, THE CONTROL CENTRE AND STUDIO. Reading has the widget's precedence everywhere (defaults, then `$MARCO_OVERLAY_*`, then the file, key by key, so one key of the wrong type keeps the value before it). Writing is a read-modify-write of the file's own fields that writes ONLY THE KEYS THE CALLER NAMES, under the route store's cross-process lock, replaced atomically; every other field, including ones no Marco surface knows, is kept as it was. A file that exists and cannot be read is not overwritten. Each surface names what its person changed: the widget's panel the rows changed since the last save, the voice switch only `voice`, the control centre only the fields that differ from what it showed."
supersedes: []
affects:
  - interface
source_paths:
  - internal/widgetconfig/settings.go
  - internal/widgetconfig/leader.go
  - internal/routes/bindings.go
  - plugins/overlay/config.go
  - cmd/marco/oconfig.go
  - cmd/marco/edit.go
---

# One owner for the widget's settings

S5 of [[Surfaces-alignment-proposal-2026-11-02]] (§5.6), closing finding F5.

## What was wrong

Three copies of one file's shape:

- **The widget** (`plugins/overlay/config.go`) had its own struct, which had no `wake`, and saved
  by writing the whole struct back. A save from the widget's panel, or a `voice on` / `voice off`,
  dropped the activation phrase the control centre had set (F5).
- **The control centre** (`cmd/marco/oconfig.go`) had a second struct with `wake`, read the
  defaults and the file but not the widget's `$MARCO_OVERLAY_*` seed, and also saved the whole
  struct. Somebody whose widget listened for the leader in `$MARCO_OVERLAY_LEADER` saw the default
  backtick there, and saving the activation phrase wrote that backtick into the file, where it
  then won over the environment. Its leader dropdown offered five values, and a leader outside
  them (`f3`) was shown as the first one and written as that on any save.
- **Studio** and the CLI read only the leader, through S3's read-only `widgetconfig.Leader`.

## What was decided

- **One shape** (`widgetconfig.Settings`, which the widget's `Config` is now an alias of) and
  **one read** (`Load`): defaults, then the widget's environment seed, then the file, each key on
  its own. `Leader()` is `Load().Leader`, so Studio's shortcut line, the CLI's sentences and the
  widget's hook read the same leader by the same path.
- **One write** (`Write(s, keys...)`): read the file's fields as raw JSON, replace the named keys,
  write it back atomically, all under `routes.LockFile` (the route store's handle-held, bounded,
  process-death-safe lock, exposed for this). Naming the keys is the rule that matters: a save
  writes what its person changed and nothing it merely read. An unknown key is refused, and a file
  that exists and is not a JSON object is refused rather than replaced by the few keys being
  saved.
- **The widget** marks a panel row as changed when it is changed, and its save writes those rows
  only. The voice switch writes `voice` on its own, so an unsaved panel edit stays unsaved and a
  voice switch already saved is not written again later.
- **The control centre** shows `Load()`, so it shows the leader the widget listens for. The page
  sends only the fields whose value differs from what it showed, and the server writes only what
  was sent (a key it does not know, or a value of the wrong type, is a 400 and writes nothing; a
  file it cannot read is a 500 and is left alone). Every dropdown includes the configured value,
  even one it does not otherwise offer.
- Library and resolver settings are not part of this, as the proposal says.

## Enforced by

- `internal/widgetconfig`: `TestTheSettingsAreReadTheWayTheWidgetReadsThem`,
  `TestASaveKeepsWhatItDidNotChange`, `TestWhatCannotBeWrittenIsNot`, `TestTwoSavesAtOnceBothLand`,
  `TestTheLeaderIsReadTheWayTheWidgetReadsIt`.
- `plugins/overlay`: `TestAWidgetSaveKeepsWhatItDidNotChange` (F5 itself: `wake`, an unknown field
  and a leader changed elsewhere all survive a widget save), `TestTheVoiceSwitchSavesOnlyItself`,
  `TestASavedVoiceSwitchIsNotSavedAgain`, `TestAWidgetSaveDoesNotOverwriteAFileItCannotRead`,
  `TestConfigRoundTrip`, `TestConfigChange`.
- `cmd/marco`: `TestTheControlCentreSavesOnlyWhatWasSent`, `TestTheControlCentreShowsTheRealLeader`,
  `TestTheControlCentreRefusesWhatItCannotSave`, `TestTheActivationPhraseIsReadThroughTheOneOwner`.

Mutations: 14 runs. Twelve were killed: writing every field instead of the named ones; replacing an
unreadable file; dropping the lock; dropping the environment seed; the widget saving every panel
row, and not marking a changed row; the voice switch saving through the panel's save, and leaving
voice marked as changed after saving it; the control centre writing every setting, accepting an
unknown key, answering a failed write as a bad request, and not trimming the activation phrase.
The first run of the last voice mutation survived, because voice was never marked as changed
before the switch; the test now toggles it in the panel first. One survived and was redundant
code, now removed (a nil guard `Write` already performs). One could not be applied as written and
was re-run in a form that compiles.

The page script cannot be entered from a Go test, so it was checked once in headless Edge,
taken verbatim from `edit.go` with `fetch` stubbed. A configured leader of `f3` is shown as `f3`.
A save with nothing changed sends nothing. A save of the activation phrase sends
`{"wake":"jarvis"}` and nothing else. It is kept, with a script that re-runs it against the page as it is now, in
`docs/experiments/S5-control-centre-headless-check/` (its README says how):
reproducible evidence, NOT a committed regression test. Nothing runs it automatically.

## Checks

Windows only, the affected checks (no full-suite rerun, by instruction). Passed: `go build ./...`;
`cmd/marco`, `widgetconfig`, `routes`, `orchestrator`, `winctx`, `navsource`, `wiringcheck` and
`spectest`; the whole Studio package (165s); gofmt; docscheck. Binary timestamps unchanged.

**The overlay suite FAILED on its first run, and the cause was this change's tests.**
`TestASpokenStopReachesTheRunningPlayImmediately`: *the stop generation did not rise (0 -> 0)*,
and the control phrase never reached the intake. This change's new config tests move the widget's
live voice gate (`TestConfigRoundTrip` toggles voice in the panel; the voice-switch tests call
`setVoice(false)`). They restored the settings afterwards but not the gate, so every later test
that speaks to the widget was ignored. The test helper now restores the gate too. The cause was
shown by reproduction, not inferred: with the restore removed, the two tests alone fail the same
way. After the fix the overlay suite passed. Both outputs are kept:
`docs/experiments/Roadmap-38-output/2026-11-03-s5-{engine,overlay,overlay-after-fix,studio}.txt`.

## Not established

- No desktop run and no rebuild, by instruction. The binaries beside the tree still have the old
  copies.
- On a platform that is not Windows, `routes.LockFile` is a lock within one process (see
  `internal/routes/lock_other.go`); two processes saving at the same instant there can still lose
  one save.
- The widget does not reload the file while it runs, as before: a change made in the control
  centre takes effect on the widget's next launch, and the widget's next save no longer
  overwrites it.
- The widget's own leader row still cycles through its five choices, starting from the first when
  the configured leader is not among them. That is only when somebody changes that row.
- The control centre's page has no committed regression test. The headless check above is
  evidence and is run only by hand.
