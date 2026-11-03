---
type: adr
status: accepted
date: 2026-11-03
decision: "ACCEPTED — THE THREE ROUTINE SCOPES HAVE ONE VOCABULARY AND ONE DEFAULT, AND AN ONLY-IN ROUTINE IS CHECKED AT THE DOOR. The scopes are named Only in <app>, From anywhere → <app> and From anywhere → current foreground, from one table (`orchestrator.ScopeLabel`/`ScopeMeans`). The default is From anywhere → this application, from one function (`orchestrator.DefaultScope`) read by the terminal's question, the unattended save, narration and Studio's chooser; all five sites used to default to Only in. With NO application known there is no default: the terminal and narration ask with Enter meaning no, and the unattended path refuses — none of them silently saves for the current foreground any more. The terminal's question says that Enter changed. An Only-in routine is admitted at `performOnePlay`, the one place every performance passes, by `orchestrator.AdmitInPlace`: either the window HANDED to the run is owned, as the host validated it (`oshost.GiveWindow`), by the routine's application — Studio's Play — or the application in front at that instant is. Supplying `--window` is not admission; owning the right window is. An unreadable foreground refuses. Focus and current-foreground routines are untouched, and the request's source is never consulted."
supersedes: []
affects:
  - execution
  - interface
source_paths:
  - internal/orchestrator/scopeterms.go
  - internal/orchestrator/orchestrator.go
  - cmd/marco/intake.go
  - cmd/marco/assistant.go
  - plugins/studio/recording.go
  - plugins/studio/frontend/index.html
---

# One scope vocabulary, one default, and Only-in is checked at the door

Rulings R2 and R3 of [[Surfaces-alignment-proposal-2026-11-02]], and the foundation for the rest
of that proposal.

## What was wrong

- **Five defaults, all the same, all against the ruling.** Enter at `marco learn`'s question,
  `LearnAuto`, narration's `chooseScope`, the voice path and Studio's chooser (`'app'`) all
  defaulted to *Only in this application*. The ruled default is *From anywhere → this
  application*.
- **No application meant a silent current-foreground save.** Each site returned the
  current-foreground scope without asking, because it was the only scope left.
- **Many phrasings.** "only in notepad", "in notepad from anywhere", "everywhere", "from anywhere,
  switching nothing".
- **Only-in was a resolution rule, not a run rule.** A phrase found an in-place routine only while
  its application was the context. An explicit identity (`marco do --play … --app …`, a clicked
  Run, a resolved binding) skipped resolution: `--app` became the request's app, the generated
  route carries no `Activate` and no check, so it ran into whatever was in front.

## What was decided

- `orchestrator.ScopeLabel` / `ScopeMeans` are the vocabulary; `orchestrator.DefaultScope` is the
  default. Every save path reads them.
- With no application: asked (`[y]es / [N]o`, Enter is no) where somebody can answer, refused
  where nobody can.
- `orchestrator.AdmitInPlace` at `performOnePlay`, with the validated owner of a handed-over
  window (`Deps.HandedTo`, set only after `GiveWindow` accepted it) or a fresh foreground reading.

Studio's chooser **labels** are not changed here; that waits for the rendered Studio slice. Only
the default it opens on changed.

## Behaviour changes

- `marco learn`: Enter at the scope question now saves *From anywhere → app*.
- `marco learn --auto` with no application: not saved.
- `marco learn` / narration with no application: asked; Enter saves nothing.
- `marco do --play X --app A` for an only-in-A routine with A not in front: refused.

## Enforced by

- `internal/orchestrator` `TestTheThreeScopesHaveOneVocabulary`,
  `TestTheDefaultScopeIsFromAnywhereToTheApplication`, `TestNoApplicationIsNeverADefault`,
  `TestEnterAtTheScopeQuestionIsTheRuledDefault`, `TestNoApplicationIsAskedNotSavedForTheForeground`,
  `TestAnUnattendedRecordingWithNoApplicationIsNotSaved`,
  `TestAnOnlyInRoutineRefusesWithAnotherApplicationInFront`,
  `TestAHandedOverWindowOfAnotherProgramIsNotAdmission`,
  `TestAHandedOverWindowOfTheApplicationIsAdmission`, `TestAnUnreadableForegroundRefuses`,
  `TestFocusAndCurrentForegroundRoutinesAreNotChecked`.
- `cmd/marco` `TestAnExplicitOnlyInRoutineIsRefusedWithAnotherAppInFront`,
  `TestAnExplicitOnlyInRoutineRunsWithItsAppInFront`,
  `TestAValidatedHandoffToTheAppAdmitsAnOnlyInRoutine`,
  `TestAHandoffToAnotherProgramDoesNotAdmitAnOnlyInRoutine`, `TestSourceIsNotAdmission`,
  `TestOnlyTheOnlyInScopeIsCheckedAtTheDoor`.
- `plugins/studio` `TestALegacyRecordingKeepsTheChoice` (default asserted),
  `TestThePageOpensTheScopeChooserOnTheBackendsDefault`.

Mutation runs (each guard deleted, its test watched failing): the door call; the handed-window
comparison; the foreground comparison; the unreadable-foreground sentence; the no-application
arm; the default value; Enter's meaning; carrying the validated program into `Deps`. All killed.

## Not established

- Not run on a desktop: the handed-over window path is exercised with a stated `HandedTo`, not a
  real `GiveWindow`.
- The suite is not established as reliably green: the first full run of this slice panicked once
  in `cmd/director` with no trace retained, cause and relationship to this change unknown. See
  [[Experiment-098-a-director-panic-seen-once-with-no-trace]].
