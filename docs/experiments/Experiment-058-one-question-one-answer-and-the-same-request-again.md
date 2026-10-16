---
type: experiment
status: complete
date: 2026-10-16
backend:
  - structure
  - ocr
  - vision
fixture: "the recorded Trackmania pause menu through the real collector, the real fusion engine and the real resolver, with internal/director/surface asking the question and internal/director/execute driving the whole pipeline over a scripted Stage of the same shape"
result: one-press-on-a-part-of-the-control-the-words-do-not-cover-settles-which-of-two-nested-surfaces-is-the-control-and-the-relation-that-press-establishes-grounds-the-same-request-against-a-fresh-reading-and-reaches-the-actuator-at-the-rows-own-middle-through-the-unchanged-resolver-policy-and-ladder-while-a-press-on-the-words-themselves-settles-nothing-and-says-so
supersedes: []
affects:
  - execution
  - perception
---

# Experiment 058 — one question, one answer, and the same request again

**Status: MEASUREMENT, 2026-10-16.** Harnesses:
`cmd/director/demonstratedsurface_corpus_test.go` under `-tags corpus` for the recorded frame, and
`internal/director/execute/demonstratedsurface_test.go` for the whole pipeline.

## The question

> A person says "click Settings". Director finds the target and cannot establish the interaction
> surface. The person demonstrates one click. Can Marco later use that demonstration to ground the
> same kind of instruction against fresh Stage evidence — without persisting a coordinate, without
> bypassing the resolver, policy, the licence or the ladder, and without teaching itself anything
> about any other control?

## The loop, over the recorded frame

```
  TRACKMANIA  "click settings"

  asked        I found "SETTINGS", but I cannot tell which area is the control: the 162x22
               region the words themselves were read on, or the 444x56 area around them.
               Click it once and I will learn where to aim; click a part of it the words do
               not cover, or the press will not tell the two apart
    e194  {388 764 162 22}  the identity's own region          at depth 0
    e190  { 98 750 444  56}  a region containing the identity   at depth 1

  press on the words   the press is inside 2 of these surfaces at once, so it is consistent
                       with all of them and settles nothing; a press on a part of the control
                       the words do not cover would tell them apart
  press on the row     the press is inside e190 and inside none of the other 1 surface(s),
                       so it tells them apart

  remembered   a region containing the identity at depth 1
               1.2% of the window, kind="line" fill=0.81 parts=12 siblings=4

  ── a FRESH reading of the same pixels: 274 observations, 228 Props, all ids re-minted ──

  grounded     the owner demonstrated on 16 Oct 2026 that the press for "SETTINGS" belongs on
               a region containing the identity, and this reading agrees
               the press would go to e190 {98 750 444 56} at {320 778}, 1.2% of the window
  scoreboard   memory does not recognise this target (insufficient), so nothing it holds
               applies, so there is a question to ask
```

## And it reaches the executor

Through the real resolver, planner, policy engine, capability ladder and executor:

```
  intent    click "Settings"
  observe   2 elements, coverage 1.00, actionable 1.00
  surface   chris demonstrated on 15 Oct 2026 that the press for "SETTINGS" belongs on a
            region containing the identity, and this reading agrees
  resolve   unknown "SETTINGS" (0.35)
  plan      click "SETTINGS" (medium risk)
  policy    allowed
  execute   left click sent at (320,778)
```

`(320,778)` is the ROW's own middle. The words' middle is `(469,775)`, which is where every rule
with no demonstration aims. Nothing was bypassed: the Prop still scores 0.35 because it is still
inert, ADR-202's licence was still required, and the record names a person as the reason the press
was allowed.

**The same request with no grounder wired** returns `failed — "Settings" is not an interactive
control, so it cannot be acted on`: byte-for-byte the refusal it was before any of this existed.

## The one real discovery

**A press on the words settles nothing, and almost every obvious design gets this wrong.**

A person shown "the word, or the row?" and told "click it" will click the WORD, because that is
where the word is. The word is inside the row, so the press is consistent with both surfaces. The
existing smallest-containing rule — the press correlator's, and the one `actionables.containing`
applies to a live press — would have answered "the word" every single time, confidently, and the
teaching would have learned nothing while appearing to work perfectly.

So a press settles the question only when EXACTLY ONE option contains it, a press inside several
is the honest answer `undiscriminating`, and the question itself says which part of the control
would tell them apart. This is the one place where the generic attribution machinery was the wrong
tool, and it is the reason the press is read against the question's own options rather than through
it.

## Four defects this found, three of them in its own design

Reported because each would have produced a confident wrong answer, and the first two were found
only by driving the whole path:

1. **The resolver cannot resolve to the surface.** The first design handed the resolver the ROW as
   the demonstrated element. The row has no label — fusion gives the words to the smaller region —
   so `textScore` is 0 and the candidate is dropped as `label does not match` before the
   affordance clause is ever reached. A request for "Settings" can only ever resolve to the thing
   called Settings.

   The fix is the split the design now rests on, and it is the better design: the words IDENTIFY
   and the demonstrated surface LOCATES, carried as `ResolvedTarget.Surface` — which
   `ResolvedTarget` always had room for, and which `reground.go` has been saying one tier down all
   along.

2. **The press point is recomputed at the moment of acting, so the surface was silently
   discarded.** `cmd/director/resolveRef` re-observes before the executor acts and sets
   `Point: el.ClickPoint()` from the element it finds — which is right, and which threw away the
   one number a demonstration contributes. Found as a click landing on the words' middle while
   every stage of the record said the row. The surface now travels on `ElementReference` and is
   re-observed by the same rule, so the press is still computed from a live reading and a surface
   that has gone leaves the identity's own middle standing.

3. **The end-to-end test's fake memory answered `same` to every signature**, so the assertion that
   a demonstration does not travel to a sibling control passed on a memory that could not tell two
   controls apart. The real store identifies a target by `{label, kind, place}` and cannot. Fixed
   by making the fake keyed on the label, after which the sibling genuinely gets asked its own
   question — which is the result the assertion was supposed to be about. The author-cannot-run-
   their-own-gate lesson again, in a fixture rather than in production code: the assertion was
   mine, the fake was mine, and together they agreed about nothing.

4. **A durable semantic target has no identity without a place.** `Discriminating` refuses
   `{label, kind, ""}` — "a target with no name and no place could never be found again" — so
   grounding needs the durable subject id of the screen, and the semantic request path does not
   know it. Not a defect in this work so much as one it ran into, and it is the reason
   `Pipeline.Place` exists as a seam the composition root fills.

## What was reused rather than rebuilt

- `internal/director/surface`'s evaluator, whole and unmodified. Every candidate is still judged
  by ADR-205's four tests, and the relation is derived from the candidates it produces.
- `observe.TargetSignature`, `semanticmemory.RememberTarget` and `ResolveTarget` — the one durable
  target identity and the one matcher. There is no second target database.
- `RehearsalEvidence`'s shape for a durable evidence record, and its discipline: a derived
  `Usable` recomputed on every read rather than a stored boolean, and the same referential drop at
  load.
- `workflowstore.AnswerOutcome`'s three outcomes, including `refused`, which becomes `elsewhere`.
- `Resolver.ResolveUnder`'s own body, now shared by all three entry points, so the licensed,
  unlicensed and demonstrated paths cannot diverge.
- `fusion.Owners` for nesting, through the evaluator, unchanged.

## Limitations

- **No production path grounds a demonstration today.** `Pipeline.Surfaces` and `Pipeline.Place`
  are both nil in `cmd/director/runtime.go`, so the daemon behaves exactly as it did. This bounds
  the blast radius of everything above, and it is deliberate: the capture side — a person's press
  arriving while a question is open — is not wired either.
- **Nothing here produces `verified`.** The slice reaches `attributed` at best, and the end-to-end
  test's own verdict is `partial — the click had no clearly observable effect`, because a scripted
  after-state is not a real one. The press reaching the actuator at the right point is what is
  asserted; a synthetic success was available by contorting the fixture and was not taken.
- **The transition attribution is not wired to the question either.** `ConfirmationAttributed` is
  a value a caller states, and the existing correlator is what should state it. Nothing yet
  connects the two, so every `attributed` record in these tests is one a test wrote.
- One frame, one application, one control. Enough to show the loop closes; not enough to say
  anything about how often a relation holds across an application's screens, which is the next
  thing worth measuring.
- The relation is selected by depth among the surfaces present NOW, so a candidate that appears
  between the words and the row shifts depth 1 to something new. The stored shape is what refuses
  it, which is a contradiction test and not a matching rule — so the failure mode is a question
  asked again, never a press somewhere new.

## Related

- [[ADR-206-a-demonstrated-surface-is-one-fact-about-one-target]] — the decision this measured.
- [[Experiment-057-what-earns-a-click-on-five-interfaces]] — where the missing piece was named.
- [[ADR-205-a-surface-hypothesis-is-request-scoped-and-refuses-to-guess]] — the evaluator reused
  whole.
- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — the demonstration-backed version
  of the same decision, one tier down.
