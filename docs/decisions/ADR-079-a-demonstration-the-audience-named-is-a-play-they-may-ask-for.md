---
type: decision
status: accepted
date: 2026-08-19
supersedes: []
affects:
  - learned-plays
source_paths:
  - cmd/director/learntail.go
  - cmd/director/learnedplay.go
  - cmd/director/learnview.go
  - internal/routes/origin.go
  - internal/director/learn/learn.go
---

# ADR-079 — a demonstration the Audience named is a Play they may ask for

Marco finished a Learn and said *"you can ask me to do it later."* Then:

```
marco routes
No routes yet. Teach one with: marco teach "<name>"
```

The play was on disk the whole time — legal, readable, correct — in `<app>/learned/`, which
route discovery deliberately does not scan. Nothing was broken; the Learn workflow simply
stopped one operation short of the thing the Audience had asked for, and the only way past
it was a developer command (`director learned --register --name …`) that no ordinary person
knows exists.

## The two claims that collided

**The wall is real.** Saving and registering are two operations against two directories, and
[[ADR-028-a-learned-play-is-a-file-with-a-past]] made them structural rather
than a flag so `saved == registered` could not be a mistake code makes. That has not changed
and must not.

**The workflow is also real.** A person who demonstrates a behaviour and gives it a name has
not asked for a file they cannot use. Their "learn X" *is* the permission to make X askable;
there is no second thing left for them to consent to.

These are not the same claim. One is about the MECHANISM, the other about WHO CROSSES IT.

## The decision

**The wall stays exactly where it is.** `SaveStaged` and `Register` remain distinct;
`<app>/learned/` remains unscanned; `Register` still refuses a slug collision rather than
overwriting somebody's authored play, and still refuses when the staged provenance no longer
describes the source.

**The Learn workflow performs both.** `learnTail.Save` asks for `Save` *and* `Register` in
one call, because naming a behaviour is the permission to make it askable. Registering moves
a file. It presses nothing, and it creates no authority to press anything — running a learned
play still needs the Audience's yes at invocation time
([[ADR-077-consent-is-the-audiences-authority-is-marcos]]).

**And the failure is honest.** A registration that refuses over an artifact that exists is
`play_not_registered`, never `save_failed`:

- `lifecycle` publishes `v.Saved` the moment the file lands, *before* attempting to register,
  so the error arrives with the evidence that the save worked.
- The reason travels with it (`learn.Saved.Reason`), so a taken name says *"already exists;
  rename the learned play or remove the other one first"* instead of dead-ending.
- Marco's sentence becomes *"I wrote it down and couldn.t make it askable. The play is safe —
  it is a file you can read and edit."* It used to be *"I couldn't save it. Nothing was
  learned."*, said over work the person could have opened.
- The Learn panel's *"Saved. It is in the Routes tab."* now gates on saved **and** registered.
  It was true only by luck — the unregistered state happened to be unreachable — and stopped
  being true the moment registration could fail over a real artifact.

**One play, one place.** A registration that half-lands is rolled back, so a failure can never
leave a copy in both scopes; and `Unregister` removes the staged copy too, because a
refused-registration artifact in `learned/` was otherwise unreachable and unforgettable by any
command a person has.

## What this does NOT decide

A learned play registers into the app's `context/` scope, so it resolves only while its
application is in front. That is what Phase 0 does and it is now pinned by a test rather than
assumed. Whether learned plays should be focus-scoped is a product question nobody has
answered.

## Enforced by

- `TestALearnedPlayIsRegisteredWhenItIsSaved` — registered, discoverable, and scoped to its
  own app and no other (`cmd/director/learntail_test.go`)
- `TestTheTeachTailNeverInvokesWhatItJustSaved` — saving and registering emit no input, claim
  no grant, and hand back nothing that can run (`cmd/director/learntail_test.go`)
- `TestASaveThatCannotRegisterStillReportsTheArtifact`,
  `TestTheTailReportsAnUnregisterablePlayAsWrittenDown`,
  `TestThePanelDoesNotClaimRoutesForAnUnregisteredPlay`
  (`cmd/director/lifecyclewiring_test.go`)
- `TestASavedPlayIsNotYetFindable`, `TestAnEditedStagedPlayCannotBeRegisteredAsLearned` — the
  wall itself, untouched (`cmd/director/lifecyclewiring_test.go`)
- `TestARefusedRegistrationLeavesOneCopy`, `TestAPartlyWrittenRegistrationIsUndone`,
  `TestRegisteringMovesThePlayRatherThanCopyingIt`,
  `TestForgettingRemovesAStagedPlayNothingCouldReach` (`internal/routes/registry_test.go`)
- `TestAPlayIsNotCalledAskableUntilItIsRegistered` — the claim is still refused when
  registration does not happen, and the refusal says why
  (`internal/director/learn/tail_test.go`)
- `TestASuccessfulTeachDoesNotRunThePlay` — a completed Learn registers and performs nothing
  (`internal/director/learn/tail_test.go`)

## Related

[[ADR-028-a-learned-play-is-a-file-with-a-past]] ·
[[ADR-077-consent-is-the-audiences-authority-is-marcos]] ·
[[ADR-078-a-learned-play-is-performed-by-the-director]] · [[Learned-Plays]]
