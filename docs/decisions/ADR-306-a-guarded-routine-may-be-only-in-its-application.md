---
type: adr
status: accepted
date: 2026-11-03
decision: "ACCEPTED — A GUARDED ROUTINE MAY BE 'ONLY IN' ITS APPLICATION: AT ADMISSION THE WINDOW IN FRONT IS READ ONCE, ADMITS THE RUN AND BECOMES ITS TARGET, AND EVERY PRESS IS STILL CHECKED AGAINST IT. Nothing is raised and nothing is searched for. The window in front need not be the one the routine was recorded in: an eligible sibling already in front at admission is the one bound, and a window that comes to the front after the binding is refused at the next press. A moved window is still pressed where it now is; resized, rescaled, closed, backgrounded, re-owned or other-program windows still refuse. Only routines whose presses check their window are bound; a legacy in-place routine keeps its admission exactly. Studio's Play is unchanged (it raises the chosen window and hands it over). The generated source does not change, so there is no new generation. Only the current-foreground scope stays refused for a guarded recording, and the default stays From anywhere → this application."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - cmd/marco/intake.go
  - cmd/marco/guardedinplace_test.go
  - internal/oshost/infront.go
  - internal/oshost/desktopfortest.go
  - internal/orchestrator/keep.go
  - internal/orchestrator/draft.go
  - internal/orchestrator/orchestrator.go
  - internal/orchestrator/scopeterms.go
  - plugins/studio/recording.go
  - plugins/studio/frontend/index.html
---

# A guarded routine may be "Only in" its application

S9: R1 of [[Surfaces-alignment-proposal-2026-11-02]]. The review and its four corrections are in
[[S9-guarded-only-in-review-2026-11-03]], approved by the owner without a generation bump.

## What was decided

- **Bound at admission, from one reading** (`cmd/marco` `performOnePlay`). For an Only-in
  routine whose saved presses check their window (`ContractOf` guarded or incomplete), with no
  window handed over, the intake reads the window in front once (`Host.ReadInFront`: handle, owner
  and program). It admits the run on that program, then binds that handle (`Host.BindInFront`).
  - Binding verifies the window is alive, still owned by that process, and in front.
  - It does not raise, it does not search (`winctx.Bind`), and it does not set `activated`.
  - A different application in front, an unreadable foreground, or a window that changed between
    the reading and the bind refuses, with nothing pressed.
- **The run's target is the window in front.** It need not be the window or document the routine
  was recorded in. A sibling window of the application already in front at admission is the one
  bound, and every press then checks it against the recorded size, scale and program. Nothing here
  establishes which document a window holds. A window that comes to the front after the binding
  (another application, or a sibling) is not the bound handle and refuses at the next press.
- **Press-time checks are unchanged** (`oshost.confirmWindow`). A **moved** window is pressed at
  its new origin plus the offset. A resized, rescaled, closed, backgrounded or re-owned window, or
  one of another program, refuses.
- **Only where needed.** A legacy (unguarded) Only-in routine attempts no binding and is admitted
  by the application in front exactly as before. It gains no new refusal.
- **Studio's Play is unchanged.** It raises the window it chose (the person's own press, in a
  window that is itself in front) and hands it over with `--window=<handle>:<owner>`. `GiveWindow`
  validates it and binds it, `AdmitInPlace` admits by it, and the presses check it.
- **The scope rule.** `ScopeFor` became `ChecksItsWindow`. A guarded recording may now be saved,
  rebuilt or moved to **Only in** or **From anywhere → app**. `WhyNotThisScope` refuses only
  **From anywhere → current foreground**, which names no application. `DefaultScope` and the
  terminal's Enter still mean From anywhere → app. Studio keeps the chooser and shows the current
  foreground option as unavailable, with the backend's sentence.
- **Existing routines.**
  - A guarded Only-in routine saved before this runs, when explicitly invoked, with no rewrite and
    no acknowledgement. Its file is already what a new save writes.
  - `Unrunnable` now lists only guarded routines with no application, and Resave applies to those.
  - Stored step accounts, rebuilds and rename are unchanged: they render under the routine's own
    scope.
- **The generated source does not change**, so `codegen.Generation` stays 1.

## Runtime compatibility, stated as it is

An older engine binds nothing for a guarded Only-in routine, so its guarded presses refuse. But the
routine can type, press a key or do other unguarded things before its first guarded press, and an
older engine performs them. So it is **not** established that an older engine does nothing wrong
with such a routine. A generation bump would not prevent that, because an unknown generation stays
playable under the rules it always had (ADR-262), on purpose. An older Studio may also offer
Resave, which rewrites the routine as From anywhere.

The same-build stack rule keeps one installation's parts together. It does not stop an older,
internally consistent installation opening the shared library. No mechanism is claimed against
this.

## Enforced by

`cmd/marco` (through the intake into the real host, over `oshost.DesktopForTest`: stated windows,
foreground and input, with the production admission, binding and guards):

- `TestAGuardedOnlyInRunIsBoundToTheWindowInFront`: the correct foreground, no raise and no
  search, and a window moved between presses pressed where it now is.
- `TestASiblingInFrontAtAdmissionIsTheOneBound`.
- `TestAGuardedOnlyInRunRefusesAWindowThatIsNotItsTarget`: another application at admission, then
  resized, rescaled, closed, backgrounded, a sibling taking the front, and another owner.
- `TestStudiosHandoffRunsAGuardedOnlyInRoutine`, entered through `--window` and never by
  assigning `HandedTo`, with a handoff of another program's window refused.
- `TestALegacyOnlyInRoutineIsAdmittedAsBefore`.

`internal/oshost`: `TestBindingTheWindowInFrontChecksItAndActivatesNothing`.

`internal/orchestrator`:

- `TestAGuardedRoutineCanBeMovedToOnlyIn`.
- `TestAGuardedRoutineCannotBeMovedWhereItsPressesWouldRefuse`, now the current foreground only.
- `TestNoSurfaceSavesARoutineThatRefusesEveryPress`: Only in is saved as asked, the current
  foreground saves nothing, and Enter means the default.
- The repair tests, whose fixture is now the remaining unrunnable shape.

`plugins/studio`:

- `TestAGuardedRecordingKeepsTwoScopesAndSaysWhyNotTheThird`.
- `TestAGuardedRecordingCanBeSavedOnlyIn`.
- `TestSavingAGuardedRecordingUnderAnImpossibleScopeIsRefused`.
- `TestAGuardedRoutineIsRefusedTheScopesThatBringNoWindow`.
- The two namesake tests.

Mutations: 11 runs, all killed:

- no binding in the intake;
- binding legacy routines;
- binding no routines;
- admitting on the separate foreground reading;
- raising at the bind;
- searching at the bind;
- setting `activated` (it survived the intake tests, so a host test was added);
- the old scope refusal back;
- `Unrunnable` listing Only-in routines;
- binding with no front check;
- binding with no owner check.

## Checks

Windows only. The first boundary run (`2026-11-03-s9-*.txt`) built, and the overlay passed, but
it **failed**, and every failure was this change's:

- `wiringcheck` (2 tests):
  - **I had overwritten an existing file.** The new intake tests were written to
    `cmd/marco/inplace_test.go`, which already held the Only-in admission tests
    (`TestAnExplicitOnlyInRoutineIsRefusedWithAnotherAppInFront` and five others). The original
    is restored from the last commit, and the new tests are in `guardedinplace_test.go`. Every
    other file created this session was checked against the tree before S4; only S5's two
    deliberate rewrites had existed.
  - ADR-251 cited a Studio test this change renamed.
- Studio (5 tests):
  - three repair and namesake fixtures still used the old unrunnable shape;
  - two page tests failed because **a Perl edit with an escaped em dash in its replacement had
    re-encoded the whole of `frontend/index.html` and `routines_test.go`**, corrupting every
    non-ASCII character. Both files were restored from the last commit and the S9 changes
    re-applied with the editor. Every changed file was scanned for the same damage, and none
    had it.

After the fixes, Studio's whole package (146s), `cmd/marco`, `orchestrator`, `oshost` and
`wiringcheck` passed, along with gofmt and docscheck, with binaries unchanged
(`2026-11-04-s9-*-after-fixes.txt`). The full engine suite was not run again: its only failures
were `wiringcheck`'s, which now passes.

## Not established

- No desktop run, rebuild or launch. The real foreground, geometry and scale were not exercised.
  The harness scenario (gate stage 5) is not part of this.
- Studio's chooser change was not rendered.
- Windows only.
