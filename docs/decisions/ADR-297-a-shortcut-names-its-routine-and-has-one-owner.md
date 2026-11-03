---
type: adr
status: accepted
date: 2026-11-03
decision: "ACCEPTED — A LEADER SHORTCUT NAMES ITS ROUTINE, AND ONE OWNER DECIDES EVERYTHING ABOUT IT. `orchestrator` shortcuts.go is the owner (S2 of the alignment proposal): assign, check, remove, list, carry on a move, forget with a routine, and resolve a press. A shortcut stores ROUTINE IDENTITIES (`routes.Binding.Routes`, one per chained step), resolved exactly once when it is assigned; a press performs those identities through the one intake and looks no name up, so a namesake or a rename can no longer redirect a key. A shortcut may be narrower than its routine and never wider: an Only-in routine's shortcut may be listened for only in its application. Taking or removing a key that is in use needs consent to the BINDING VERSION the person was shown (a digest of the stored entry), re-checked under a cross-process bindings lock at the write — a route ID is not consent, because the routine on a key can stay the same while its binding changes. The widget's own keys (m, h, esc) and the configured leader cannot be bound. Forgetting a routine removes its shortcuts; a move carries them, and a move that replaces a routine removes the replaced routine's shortcuts first so they do not start running what replaced it. A shortcut set up by name by an older build is kept on disk untouched, does not run, says so, and is reassigned only by `marco bind --reassign`, which shows the routine and asks with Enter meaning no. A recording in progress holds every shortcut off: `Deps.Begin` announces it under the Marco home (internal/recordingnow, one file per recording process identified by pid AND start time) and `Capture.Finish` clears it. Locks have one order: route, then bindings."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - internal/orchestrator/shortcuts.go
  - internal/routes/bindings.go
  - internal/routes/lock_windows.go
  - internal/routes/lock_other.go
  - internal/recordingnow/recordingnow.go
  - internal/recordingnow/process_windows.go
  - cmd/director/recordingnow.go
  - cmd/director/learnsessionwiring.go
  - cmd/director/visualsession.go
  - cmd/director/capturegamesession.go
  - cmd/marco/panicstop.go
  - internal/invoke/invoke.go
  - internal/orchestrator/draft.go
  - internal/orchestrator/rescope.go
  - cmd/marco/bind.go
  - cmd/marco/edit.go
  - cmd/marco/assistant.go
  - plugins/studio/recording.go
---

# A shortcut names its routine, and has one owner

S2 of [[Surfaces-alignment-proposal-2026-11-02]], under rulings R4 and R6 and the amendment's
corrections 3 and 4. No surface control changed: the widget's `bind`/`unbind` words spawn the same
commands they always did, the control centre's page is unchanged, and Studio gained no control.

## What was wrong

From [[Shortcuts-from-Studio-proposal-2026-11-02]] §1.2 (G3–G13), each confirmed by reading:

- a binding stored **words** and every press re-resolved them by name with the foreground's
  four-scope priority — a namesake took a key over, a rename stranded it, and a routine saved later
  under the old name silently inherited it;
- `Bind` replaced whatever was on a key without a word, and wrote with no lock;
- the widget's own `m` and `h` could be bound and could never fire;
- the CLI and the control centre each chose the shortcut's application by a different rule;
- nothing held shortcuts off while somebody was recording.

## What was decided

| rule | where |
|---|---|
| identities, resolved once at assignment | `cmd/marco` `resolveSteps`; `orchestrator.AssignShortcut` stores `Routes` |
| a press looks nothing up | `orchestrator.ShortcutPressed` → `pressHotkey` → `runInvocation` with each identity |
| narrower, never wider | `orchestrator.wider` |
| consent is to the version shown | `routes.Binding.Version`; `AssignShortcut` / `RemoveShortcut` compare under `routes.UpdateBindings` |
| bindings lock, atomic write, unreadable file refused | `routes.LockBindings`, `UpdateBindings` (shares `lockFile` with the route locks) |
| one lock order: route, then bindings | `routes.LockBindings` doc; `Rescope` carries after both route holds |
| reserved keys | `orchestrator.reservedKeys` + the configured leader |
| forget removes | `forgetPlay` (CLI one and all), control centre delete |
| move carries; replace drops first | `Rescope`, control centre scope move, CLI rename |
| legacy refuses; explicit reassignment | `ShortcutPressed`; `marco bind --reassign` |
| a recording holds shortcuts off | `internal/recordingnow`; `Deps.RecordingHome` set by `newDeps` and Studio's recording |
| listing | `orchestrator.ListShortcuts`; `marco bindings [--here] [--json]` |

How each surface collects "listened for where" is the only difference left: the terminal and the
widget use the foreground at the moment of typing, or `--everywhere` / `--in=<app>`; the control
centre uses the routine's own application, because its foreground is a browser.

## Behaviour changes

- A shortcut set up by name by an earlier build **stops running** until reassigned. Its entry is
  untouched on disk.
- Binding a key that is in use **asks** in the terminal and the widget (Enter is no), and is
  **refused** by the control centre unless the page sends the version it showed.
- `m`, `h`, `esc` and the leader are refused.
- An Only-in routine cannot be bound everywhere or in another application.
- Forgetting a routine removes its shortcuts.
- While a recording runs, a bound key refuses with a sentence instead of running.

## Corrections after review (2026-11-03)

The first version of this slice had three P1 gaps, found in review before commit. Each is closed
by ordering rather than by reporting afterwards:

1. **Cleanup now happens before an identity changes.** A replacing `Rescope` invalidates the
   replaced routine's shortcuts under both route locks BEFORE anything is published at the
   destination, and a failure writes nothing. The moved routine's shortcuts are carried while it is
   in both places, BEFORE the original is removed; a carry that fails keeps the original, which
   they still name. `ForgetRoutine` holds the routine's lock and removes its shortcuts FIRST; a
   failure forgets nothing. `RenameInPlace` (the CLI's unreviewed rename) and the control
   centre's scope move hold both routes' locks and KEEP THE ORIGINAL until the destination holds
   the routine AND its shortcuts have moved (see the second review below).
   And a press is PINNED: `ShortcutPressed` takes the step routines' locks, reads the shortcut
   again, and pins each routine's bytes (`invoke.Request.Pinned`, not Studio's `Expect`, because
   nobody confirmed them); the run refuses other bytes. So a press that resolved just before a
   replacing move cannot run what the move publishes.
2. **Assignment takes the route locks.** `AssignShortcut` takes its routines' locks in identity
   order, revalidates under them, then takes the bindings lock and writes. A forget or a move of
   those routines waits, then removes or carries the new shortcut.
3. **Recording admission fails closed.** `Begin` announces BEFORE the hooks go on and refuses to
   record if it cannot; a recorder that then fails to start leaves no announcement.
   `recordingnow.InProgress` answers None, Recording or **Unknown** — an unreadable directory or
   marker, or a process whose liveness cannot be read, is Unknown, and a press refuses on Unknown.
   Only a process confirmed gone (no such pid, exited, or a pid reused by a process that started
   at another time) makes a marker stale. Off Windows nothing can be identified, so a recording
   cannot be announced and does not start. Director's three recording entrances announce too:
   `demonstrate-visual`, `capture-game-session` and a learn session (until its capture ends), each
   before its hooks or observation start, each refusing if it cannot. Director's ambient watching
   does not: it is not a demonstration being recorded, and it runs whenever the service does.

### Second review: a failed rename (2026-11-03)

`RenameInPlace` used to carry the shortcuts first and roll them back if the rename failed. A
rename and a rollback that BOTH failed left them naming the new identity while it was empty: they
refused only until something was saved there, and then ran it, which is the redirect this slice
exists to prevent. The order is now copy, carry, finish, and nothing is rolled back:

1. **Create** the destination as a copy of the source, exclusively (`Registry.Create`). The
   shortcuts still name the original, which still exists.
2. **Carry** the shortcuts, while the routine is in both places.
3. **Finish:** `Registry.Rename` moves the original's files over the copy, which removes it.

A failure at 1 or 2 leaves the shortcuts on the original and DELETES NOTHING. Whatever is at the
new name is left there and reported as permitted residue: a routine no shortcut names. A first
version of this removed the destination when its bytes were a prefix of the source, as "this
rename's own copy". That is not ownership evidence: another writer's empty file, or any prefix of
the source, passes it. So no path is deleted on its bytes. If a cleanup is ever wanted, it needs
file-identity ownership, not a content comparison. A failure at 3 is a finished rename with the
original left over, still holding its recording and history and no shortcuts, and the error says
so. The control centre's scope move already had this shape (copy, carry, then delete) and is
unchanged.

### Ruling: an agreed replacement keeps its shortcuts (2026-11-03)

Replacing a routine through `Keep`, by recording the same name again and agreeing to replace it,
is a new version of the routine the shortcuts were bound to, so its shortcuts are kept and run the
new version. A forget followed by an unrelated routine saved under the name is the opposite case,
and inherits nothing. The replacement question (`Collision.Says`, which the terminal prints) now
says so once, naming the keys (*Its shortcut (a in notepad) is kept and will run the new
version.*), with no confirmation per shortcut. An unreadable shortcuts file is said as that, never
as none. `Collision.Shortcuts` carries the keys on the wire. Studio's Replace card does not show
`Says`; it writes its own sentence, so it does not say this yet. That is S3 surface work.

## Not done here, deliberately

- No surface controls (S3 Studio, S4 widget), each after a rendered review.
- Studio's *Forget everything* keeps its text; it deletes Acts, scenes and runs, not routines, so
  no routine shortcut is affected by it.
- `marco rename` still renames without review (R7, S7); it now carries shortcuts.

## Enforced by

- `internal/orchestrator`: `TestTheSameKeyRunsDifferentRoutinesPerApp`,
  `TestAnAppKeyShadowsTheGlobalKey`, `TestAShortcutCannotBeWiderThanItsRoutine`,
  `TestAGlobalShortcutForACurrentForegroundRoutine`, `TestConsentIsToTheBindingVersionShown`,
  `TestRemovingAShortcutThatChangedIsRefused`, `TestReservedKeysCannotBeBound`,
  `TestALegacyBindingRefusesUntilReassigned`, `TestAStaleShortcutRefusesAndSaysWhich`,
  `TestAShortcutRunsTheRoutineItWasAssignedNotItsNamesake`,
  `TestForgettingARoutineRemovesItsShortcuts`, `TestCarryingPointsShortcutsAtTheNewIdentity`,
  `TestAReplacedRoutinesShortcutsAreNotInheritedByWhatReplacedIt`,
  `TestARealMoveCarriesShortcutsAndDropsTheReplacedRoutines`,
  `TestTheLockOrderIsRouteThenBindings`, `TestARecordingIsAnnouncedBeforeItsHooksGoOn` (Windows).
- `internal/routes`: `TestParseIDReadsBackEveryScope`, `TestABindingVersionIsTheEntryNotTheRoutine`,
  `TestAnUnreadableShortcutsFileIsNotOverwritten`.
- `internal/recordingnow`: `TestARecordingIsInProgressUntilItIsCleared`,
  `TestOnlyAConfirmedDeadRecorderIsStale`, `TestNoHomeAnnouncesNothing`.
- `cmd/marco`: `TestARecordingHoldsShortcutsOff` (Windows), `TestAPressedShortcutIsTheRoutineItWasAssigned`,
  `TestBindAsksBeforeReplacingAndEnterMeansNo`, `TestBindIsListenedForWhereItWasTyped`,
  `TestReassignShowsTheRoutineAndAsks`, `TestUnbindSaysWhatItRemoved`,
  `TestForgettingARoutineRemovesItsShortcutsAtTheDoor`, `TestTheBindingsListIsWhatAPressWouldFire`,
  `TestAShortcutNeedsNoModel`, `TestNoProductionCodeWritesALegacyBinding`,
  `TestTheTerminalAnnouncesItsRecordings`,
  `TestTheControlCentreTakesAKeyOnlyWithConsentToWhatItShowed`,
  `TestTheControlCentreForgetsAndCarriesShortcuts`, plus the existing
  `TestAHotkeyPerformsTheBoundPlayItself`, `TestAHotkeyResolvesInTheScopeItWasBoundIn`,
  `TestABindingIsValidatedTheWayItWillBeResolved`.
- `plugins/studio`: `TestARecordingInStudioIsAnnouncedToTheWidget`.

Mutation runs (guard removed, its test watched failing), all killed: the press's recording check;
forget's removal (CLI and control centre); each of the three consent arms of assignment and the
removal's; the wider rule; the legacy refusal; the stale refusal; the replaced-routine drop and
the carry in `Rescope`; the control centre's carry and its consent; `Begin`'s announcement and
`Finish`'s clearing; a reserved key; Enter meaning no; the shadow flag; and `RecordingHome` in
both production constructors.

Added with the corrections (orchestrator unless marked):
`TestAReplacedRoutinesShortcutsGoBeforeItIsReplaced`,
`TestAReplacingMoveWhoseShortcutsCannotBeInvalidatedWritesNothing`,
`TestACarryThatFailsKeepsTheOriginal`, `TestAForgetWhoseShortcutsCannotBeRemovedForgetsNothing`,
`TestARoutineRecreatedAfterAForgetDoesNotInheritItsShortcuts`,
`TestAnAssignmentInterleavedWithAForgetLeavesNoOrphan`,
`TestAnAssignmentInterleavedWithARenameFollowsTheRoutine`,
`TestARenameWhoseShortcutsCannotBeCarriedRenamesNothing`, `TestAPressPinsTheRoutineItResolved`,
`TestAPressThatLosesItsShortcutMidwayRunsNothing`, `TestAnUnannouncedRecordingDoesNotStart`,
`TestTheLockOrderIsRouteThenBindings` (now a scan of every engine file);
`internal/recordingnow` `TestARecordingThatCannotBeAnnouncedIsRefused`,
`TestOnlyAConfirmedDeadRecorderIsStale`, `TestUnreadableStateIsUnknownNotAbsent`;
`cmd/marco` `TestAPressDoesNotRunWhatWasPublishedAfterItResolved`,
`TestAPressThatCannotTellWhetherSomebodyIsRecordingIsHeldOff`,
`TestTheControlCentreMoveCarriesBeforeItDeletes`; `cmd/director`
`TestDirectorRecordingEntrancesAnnounceBeforeTheirHooks`, `TestAnUnannouncedLearnSessionDoesNotStart`.

**Windows-only** (they read this process's start time, which only Windows can):
`internal/orchestrator` `TestARecordingIsAnnouncedBeforeItsHooksGoOn`,
`TestARecorderThatFailsToStartLeavesNoAnnouncement`; `internal/recordingnow`
`TestThisProcessCanBeIdentified`; `cmd/marco` `TestARecordingHoldsShortcutsOff`; `cmd/director`
`TestALearnSessionIsAnnouncedUntilItsCaptureEnds`; and `plugins/studio`
`TestARecordingInStudioIsAnnouncedToTheWidget`. Every result in this record was obtained on
Windows; nothing was run on another platform.

The corrections' mutation runs (each guard removed, its test watched failing), all killed:
`Begin` ignoring the announcement's failure, announcing nothing, and leaving the announcement when
the recorder fails; `InProgress` skipping a corrupt marker, sweeping an unsure one, and reading an
unlistable directory as None; `Mark` announcing an unidentified process; the press treating Unknown
as None; the learn session starting unannounced; `Rescope` invalidating late and carrying late;
`ForgetRoutine` ignoring a failed cleanup; assignment without route locks; the press's second read;
the pin dropped from the request and ignored by the run; the control centre's carry-before-delete;
`RenameInPlace`'s carry-first; and both forget doors bypassing `ForgetRoutine`.

The second review's tests (orchestrator): `TestAFailedRenameLeavesNoShortcutForTheNextRoutineThere`
(the creation or the carry fails, whatever it left stays, an UNRELATED routine is then saved at
the new name, and the shortcut must still run the original, whose bytes are unchanged),
`TestARenameWhoseOriginalCannotBeRemovedStillRunsTheSameRoutine`,
`TestARenameCarriesOnlyOnceTheNewNameHoldsTheRoutine`,
`TestARenameWhoseCarryFailsLeavesItsCopyAndSaysSo`, and the control
`TestAFailedRenameLeavesSomebodyElsesFileAlone` (another writer's EMPTY, PREFIX-MATCHING,
IDENTICAL or unrelated file at the new name must remain untouched). Each step is failed through
`Deps.RenameFault`, which is nil in production. Mutations, all killed: the old carry-first order
with no rollback (killed by the discriminating test); finishing before carrying; a swallowed finish
error; the residue not reported; a delete of anything that is a prefix of the source; a delete of
anything at all. A read-back comparison of the copy was written and then removed, because no test
could reach it. For the ruling: `TestAReplacementKeepsItsShortcutsAndSaysSo` (the question names
the key; after the agreed replacement the key runs the new version's bytes) and
`TestAReplacementQuestionNeverSaysNoShortcutsWhenItCannotTell`; mutations removing the sentence
and reading an unreadable file as none were both killed.

## Not established

- `director capture-game-session` and `demonstrate-visual` are held by a source-order check, not
  entered: both install real hooks. The learn session is entered.
- Off Windows a process start time cannot be read, so a recording cannot be announced and is
  refused, and a marker present there is Unknown. The product does not ship there.
- A rename whose last step fails leaves two copies of the routine; the one at the old name has no
  shortcuts and keeps the recording and history. Nothing tidies it, and the error says so.
- A rename that fails at its create or carry step can leave a file at the new name. Nothing
  removes it; the error names it and says no shortcut runs it.
- The engine suite's last full run had one failure outside this record's tests, recorded
  separately and unresolved: [[Experiment-099-two-activation-failures-for-one-absent-application]].
- An unreadable marker holds every shortcut off until somebody removes it; nothing sweeps it,
  by design (Unknown is never treated as absent).
- What the recorder would actually have captured from a shortcut injected during a recording is
  still not demonstrated; the hold-off is a requirement, not a fix for an observed defect.
- No desktop run: every test states the foreground and uses a synthetic library.
