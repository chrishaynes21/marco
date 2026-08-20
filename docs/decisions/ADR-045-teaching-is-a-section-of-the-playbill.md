---
type: decision
status: accepted
date: 2026-08-11
supersedes: []
affects:
  - visibility
  - passive-observation
source_paths:
  - pkg/playbill/playbill.go
  - pkg/playbill/narrate.go
  - pkg/playbill/guard.go
  - cmd/director/playbill.go
---

# ADR-045 — teaching is a section of the playbill, not a second UI

> *Editorial note, added 2026-08-20.* Written before
> [[ADR-048-learn-teach-and-do-are-three-different-sentences]]. **"Teaching" here is the Learn
> session** — the person demonstrates and Marco acquires — not the reserved **Teach** feature, in
> which Marco guides a person through something it already knows. The section it decides on is the
> type `playbill.LearnSession` today and renders under the header `LEARN SESSION`, kept distinct
> from the passive `playbill.Learning` section directly below it (`LEARNING`). The rename is
> [[ADR-086-one-acquisition-one-word-one-request]].

Roadmap 32 finished the Teach lifecycle and Roadmap 33 asked for a visible face for it. The
audit found the application shell already existed and was already right: `pkg/playbill` is ONE
read-only account, `Normal()`, `Watch()` and `Deep()` are three readings of it, and the overlay
imports that type and nothing else — it chooses colour and layout and decides no facts.

What did not exist was **teaching reaching it**. `learningFrom` derives its account from the
observation session, which cannot tell that a person asked for something, what they called it,
or which cue they are waiting for. Every teach state rendered as ordinary watching.

## The decision

A `Teaching` section on the view, sourced from the coordinator — the canonical owner — and
narrated in the same package as everything else. No second application, no overlay change: the
overlay renders `view.Watch()` lines, so the section appeared the moment the account carried it.

Three facts on it are load-bearing and each is READ rather than derived by a surface:

- **`Armed`** — the bounded demonstration window is open. Read from the coordinator's phase. A
  person being watched normally and a person mid-demonstration must never look alike, and this
  is the one boolean that separates them.
- **`Waiting`** — this is your turn. The coordinator owns which phases it will not advance past
  on a timer; a surface that guessed would eventually disagree with it.
- **`Learned`** — the artifact. Read from the saved play, never from having reached the end of
  the flow.

`Did` is the closed navigation vocabulary, membership-checked at admission, bounded, and its
**empty case is split**: nothing yet, versus a change nobody could attribute. The second is the
commonest honest failure in mouse-driven software and must never render as a blank line.

## What rendering every state found

Two defects that no unit test would have caught, because each state passed on its own:

- **"Getting ready…" outlived its truth.** It appeared in every phase that was not the cue, so
  *"want me to try it once?"* arrived under a line saying Marco was getting ready.
- **Normal collapsed four states into "Teaching".** Waiting for permission, rehearsing, waiting
  for a name and thinking all read identically — so the one thing a person needed to know, that
  it was their turn, was the one thing the surface did not say.

Both are fixed and both now have tests. The walkthrough that found them is kept as
`walkthrough_test.go`: it renders every teaching state, asserts each is admissible, and logs the
rendering under `-v`. Reading the output is the point.

## Enforced by

- `pkg/playbill/learnsession_test.go` — the cue is unmistakable, the two silences are kept apart,
  only navigation meanings may be shown, completion comes from the artifact, a path is refused
  where a play name is allowed, and the cue moves the digest so no surface sleeps through it.
- `cmd/director/learnsectionview_test.go` — the wire, `Armed` and `Waiting` read from the
  coordinator for every phase, and `TestReadingTheAccountChangesNoDirectorState`.
- Seventeen mutations, all caught. Two survived first: completion invented from the phase, and
  `Waiting` never published at all — the field existed, the narration used it, every view test
  passed, and the production wire did not set it.

## Related

[[Visibility]] · [[ADR-043-teaching-is-two-passes-not-a-new-capture]] ·
[[ADR-044-a-teach-attempt-is-one-episode]]
