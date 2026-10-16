---
type: experiment
status: complete
date: 2026-10-16
backend:
  - structure
  - ocr
  - vision
fixture: "two recorded interfaces — a Trackmania pause menu and a Windows Settings mouse page — through the real collector, fusion engine and resolver; and the real ambient press reader, coalescer, attributor and crossing correlator in cmd/director over a scripted observation session"
result: a-press-read-through-the-production-input-path-settles-a-surface-question-only-when-it-tells-the-candidates-apart-a-human-crossing-attributed-to-exactly-one-action-is-the-only-thing-that-promotes-it-to-usable-and-the-production-grounder-then-reuses-it-on-a-fresh-reading-of-two-unrelated-toolkits-while-marcos-own-crossing-two-actions-and-an-unchanged-screen-each-keep-it-as-a-demonstration-that-cannot-aim
supersedes: []
affects:
  - execution
  - perception
---

# Experiment 059 — one demonstration from a person into durable memory

**Status: MEASUREMENT, 2026-10-16.** Harnesses: `cmd/director/surfacelearn_test.go` for the
production press and crossing path, `cmd/director/demonstratedsurface_corpus_test.go` under
`-tags corpus` for the recorded frames, and `internal/director/surface/pending_test.go` for the
lifecycle rules.

## The question

> Can one genuine demonstration from a person reach durable memory and be reused on a later
> instruction — with the confirmation derived from the existing transition correlator rather than
> asserted, and with no second input recorder, no second correlator and no second screen identity?

## The five halves, each measured separately

### 1. Production request wiring

`Pipeline.Surfaces` and `Pipeline.Place` are non-nil in the daemon the real `NewRuntime` builds.
The place seam reads `ambientObserver.standing()`, which reports what ambient watching's own
resolver already decided — `observe.Place.Subject`, or empty when Marco does not recognise the
screen. Nothing new resolves anything.

### 2. Genuine input attribution

Through `ambientObserver.drain`, the production reader of the admitted input log:

```
  on a part only the row covers   settled — the press is inside row and inside none of the
                                  other 1 surface(s), so it tells them apart
  on the words, inside both       undiscriminating — consistent with all of them and settles
                                  nothing; a press on a part the words do not cover would
                                  tell them apart
  on neither                      elsewhere — the press at (0.010, 0.019) is not inside any
                                  of the 2 surfaces this question is about
  on a different screen           not an answer at all — the press carries the state it was
                                  performed ON, and it is not this question's
  from another observation session not an answer — state ids are session-local counters
```

### 3. Persistence only after an attributable transition

The witness is built from the acts the real correlator filed and handed back through
`takeForCrossing`:

```
  a human crossing after the press      LEARNED — a region containing the identity at depth 1
                                        for "SETTINGS", confirmed because the screen changed
                                        from subj_menu to subj_settings and the crossing was
                                        attributed to one human action
  no crossing at all                    nothing stored; the press stands settled and unused
  a crossing Marco caused               kept unconfirmed — "not attributed to a person acting"
  a crossing attributed to two actions  kept unconfirmed — "does not single out the one being
                                        asked about"
  the screen did not change             kept unconfirmed — "the screen did not change, so
                                        nothing confirms that the press did anything"
  a screen with no durable identity     nothing stored — "a target with no name and no place
                                        could never be found again"
```

Every one of the four refusals leaves a record that `Usable` rejects, so the press is not lost and
cannot aim.

### 4. Reload and fresh regrounding, on two toolkits

Re-collected rather than re-used: the Stage is built again from the frame, so ids, proposals and
fusion are all produced afresh and the stored record has to find its way by relation alone.

```
  TRACKMANIA  "click settings"
    asked      the 162x22 region the words were read on, or the 444x56 area around them
    e194 {388 764 162 22}  the identity's own region       depth 0
    e190 { 98 750 444  56}  a region containing it          depth 1
    remembered depth 1; 1.2% of the window, kind="line"  fill=0.81 parts=12 siblings=4
    grounded   the press would go to e190 at {320 778}
    scoreboard memory does not recognise this target, so there is a question to ask

  WINDOWS SETTINGS  "click mouse"      — a different toolkit, and not a game
    asked      the 160x34 region the words were read on, or the 846x66 area around them
    e68  {348 746 160 34}  the identity's own region       depth 0
    e66  {336 732 846 66}  a region containing it          depth 1
    remembered depth 1; 4.9% of the window, kind="block" fill=0.96 parts=1  siblings=8
    grounded   the press would go to e66 at {759 765}
    bluetooth  memory does not recognise this target, so nothing it holds applies
```

The same shape, the same relation and the same refusal to travel, on two renderers that share no
code — which is what makes the loop a general one rather than one fitted to a game.

### 5. Guarded execution

Unchanged from [[Experiment-058-one-question-one-answer-and-the-same-request-again]] and re-run:
`left click sent at (320,778)` through the real resolver, planner, policy engine, capability
ladder and executor, with ADR-202's licence still required and the Prop still scoring 0.35 because
it is still inert.

## What is NOT demonstrated, stated plainly

- **`verified` is not reached.** An attributed crossing says the interaction did something; it
  does not say the intended page opened. Nothing in the tree constructs the third level and
  nothing pretends to.
- **No live input.** Every press above is an `observe.AttributedInput` handed to the production
  reader; no hook was installed and no game was run.
- **The window binding is carried and not independently checked.** `ambientLook` has no window id,
  so what stands in its place is the screen-state stamp, the application check, and `navsource`'s
  own refusal of a press it cannot place inside the watched window.
- **The question's wording is accurate and slightly unnatural.** Asking somebody to click inside
  the row but outside the words is a true description of what would settle it and not yet a good
  sentence. The owner has this as a later Studio concern — highlight both regions and say "click
  the part you want me to use" — and it does not change anything measured here.

## Two defects, both found by this experiment's own gates

### The mutation gate caught a crossing hook nothing proved

Nineteen of twenty mutants died at once; the twentieth was **deleting the production call that
offers a crossing as a confirmation**, and nothing failed. Every test of the crossing called
`offerCrossing` directly, so the wiring it depends on was unasserted — the exact shape the
project has recorded eight times before: complete code that is never invoked, and a test suite
that cannot tell.

Closed by driving `ambientObserver.record` — the function the supervisor's own `sample` calls —
twice with the press drained in between, in production order. It kills the mutant and it is the
only test in this set that would notice either hook being removed.

### And a negative case that passed for the wrong reason

**A look is the session's WHOLE input log, not a delta**, and feeding a delta makes the second
reading appear empty.

`ambientObserver.drain` keeps an ABSOLUTE cursor into the session stream, for the reason its own
note gives: the log drops its oldest entries when it overflows, and an index into the slice would
silently re-deliver everything after a drop. The two-presses case fed two separate one-element
looks, the second drain correctly saw nothing new, and the crossing was therefore attributed to
ONE action and learned — the very thing that case exists to refuse.

It is a fixture defect rather than a production one, and the reason it is reported is that the
test was mine, the fixture was mine, and together they agreed about nothing: a negative case that
passes for the wrong reason is worse than no negative case, because it is evidence of a refusal
that is not happening. The same lesson as Experiment-058's third defect, one layer out.

## What was reused rather than rebuilt

- `navsource` and the session input log, whole — including the injected-press refusal, the
  Marco-owned-surface refusal and the unplaceable-press refusal, all of which run before anything
  here sees an event.
- `coalesce`, `attribute`, `takeForCrossing` and the carry across a session boundary — the real
  correlator, reached through `drain`, with the demonstration path reading the same log rather
  than consuming it.
- The ambient crossing's own provenance question, which is what `ByHuman` means: "a play running
  while watching is on moves the screen too".
- `observe.Place` from ambient watching's resolver, for the durable place.
- `observe.TargetSignature` and `semanticmemory.RememberTarget` for the durable target, and
  `observesession.LearnLicence` for the permission to name the control the person activated.
- `observe.ScreenName`'s one-constructor discipline, as the shape of an unforgeable confirmation.

## Related

- [[ADR-207-a-question-is-answered-by-a-press-and-confirmed-by-a-crossing]] — the decision this
  measured.
- [[Experiment-058-one-question-one-answer-and-the-same-request-again]] — the reasoning, before
  it was wired.
- [[Experiment-057-what-earns-a-click-on-five-interfaces]] — where the missing piece was named.
