---
type: experiment
status: complete
date: 2026-10-15
backend:
  - structure
  - ocr
  - vision
fixture: ".tmp/game-capture-review/tm-menu-steps-sanitized/frames/tm-menu-steps-0100.png and .tmp/desktop-corpus-review/explorer-fixture-wide, through the real collector, the real fusion engine, the real resolver and the real policy engine"
result: the-row-is-already-on-the-stage-and-fusion-already-computes-the-relation-and-discards-it-the-licence-already-passes-and-only-the-role-derived-affordance-refuses-and-policy-refuses-the-whole-world-at-coverage-zero-before-any-action-is-considered-so-the-smallest-change-is-a-surface-proposal-carried-on-the-resolution-and-the-open-decision-is-what-evidence-earns-one
supersedes: []
affects:
  - perception
  - execution
---

# Experiment 054 — the surface a named Prop does not carry

**Status: MEASUREMENT AND CODE READING, 2026-10-15. Nothing implemented. No threshold, no role,
no detector, no licence and no policy changed.** The harness that produced the numbers was a
throwaway `-tags corpus` test in `cmd/director`, deleted; its source is in the session
scratchpad as `exp054-investigate.go.txt`.

## The question

> Given the instruction "click Settings" and the current Trackmania Stage, can Director identify
> the intended Prop and a defensible interaction surface without requiring Stage to assign
> `RoleButton`?

**The identification already works. The surface already exists on the Stage. The relation
between them is already computed, by fusion, and thrown away. And three separate gates refuse —
only one of which is the one everybody has been looking at.**

## 1. Where the SETTINGS text and the 444x56 row can currently be related

**In exactly one place: `fusion/text.go`'s `textCandidates`, which computes the relation and
keeps only the winner.**

Measured, by reconstructing the same candidate set from the same cycle:

```
  "SETTINGS"  OCR text box {389 763 161 22}  ->  2 owner candidates at >= 0.80 containment
    cov 0.955   e194  {388 764 162 22}    3564 px   0.2% of window   fill 0.61  parts 3
    cov 0.950   e190  { 98 750 444 56}   24864 px   1.2% of window   fill 0.81  parts 12

  "SCOREBOARD" text box {371 643 238 22}  ->  2 owner candidates
    cov 0.996   e175  {372 642 238 24}    5712 px   0.3% of window   fill 0.59  parts 1
    cov 0.962   e174  {118 630 482 56}   26992 px   1.3% of window   fill 0.62  parts 19

  "OPPONENTS"  text box {484 522 209 23}  ->  1 owner candidate
    cov 1.000   e114  {482 388 1438 692} 995096 px  48.0% of window  fill 0.27  parts 1172

  Explorer, "Details" text box {1034 109 35 9}  ->  2 owner candidates
    cov 1.000   e11   {1028 100  42 28}   1176 px   0.1% of window   fill 0.72  parts 1
    cov 1.000   e10   {   8  90 1084 44}  47696 px  5.4% of window   fill 0.95  parts 13
```

**The row IS a Prop.** `e190 {98 750 444 56}` is on the Stage, unnamed, confidence 0.30 — the
`baseConfidence` floor for a structural source. So is its sibling `e174` under SCOREBOARD, and
so is `{88 810 740 56}`.

The containment difference between the word-shaped region and the row is **0.005**, far inside
`ambiguityMargin` (0.15). The only thing that stops fusion calling this ambiguous is
`nestingRatio`: the row is 6.98x the word region's area, past 1.5, so it is classified an
ANCESTOR rather than a peer and the smallest container wins outright. `ambiguousRival` is
therefore the function that already identifies the row *as the thing the named Prop sits inside*,
and the only trace that survives is `TextDecision.Element` naming the winner —
`TextDecision.Candidates` is populated for the ambiguous case only, and the whole `Report` is
dropped at the `Pipeline.Observe` seam ([[Experiment-053-what-the-stage-can-already-say-it-is-unsure-of]]).

**One correction to [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]]'s
wording.** It says fusion chose the word-shaped region over the row, which is exactly right. But
the row does **not** contain the resulting Prop: `e194` runs to x=550 and the row ends at x=542,
so a containment probe over the FUSED elements finds zero containers for SETTINGS. The relation
holds between the row and the OCR TEXT BOX (0.950), not between the row and the Prop the text
named. Any rule that looks for a surface by asking which Prop contains the named Prop finds
nothing on this frame. That eight-pixel overrun is the same one
[[Experiment-052-what-the-structural-proposer-offers-the-stage]] measured.

## 2. Whether existing evidence supports treating the row as a candidate click surface

**For the two menu rows, yes, and the evidence is already on the Stage and already distinct from
the evidence for the regions that must be refused.**

```
                            area    % win   fill   parts   peers   locates   ratio to its word
  SETTINGS row            24 864    1.2%    0.81     12      11      true          6.98x
  SCOREBOARD row          26 992    1.3%    0.62     19      13      true          4.73x
  TM background          995 096   48.0%    0.27   1172       1      true    (sole candidate)
  Explorer toolbar        47 696    5.4%    0.95     13       2      true         40.56x
```

`fill` and `parts` are the proposer's own evidence, carried in attributes by ADR-203 precisely so
a reader could weigh one of these. They separate the background cleanly — 0.27 fill over 1172
merged components is a scene, not a control — and they do **not** separate the Explorer toolbar,
which is a well-drawn band with 13 parts and looks structurally exactly like a menu row.

`peers` (Props of the same height whose x-range overlaps this one) is the owner's *"several
similarly shaped regions might form a menu"*, and it does separate all three: 11 and 13 for the
rows against 1 for the background and 2 for the toolbar. But the SETTINGS **word** has 10 peers
of its own, so a peer count alone says "there are things of this height nearby" and not "this is
one row of a menu". The signal that would mean the second is the regular vertical pitch — the
three 56-px rows sit at y 690, 750, 810 — and nothing measures that today.

`observe.Region.Locates()` — the workflow stack's "a band across the window locates nothing" rule
— **refuses none of the four.** The background is bounded away from the left and top edges
(x 482..1920, y 388..1080) and the toolbar starts at x=8 of a 1092-wide window, which is 0.0073
and outside `edgeTolerance` 0.005. It is a necessary gate and nowhere near a sufficient one, and
the measurement is the point: the one existing geometric refusal does not catch either of the two
cases that must be caught.

**So the honest answer to "is the row defensible" is: on this frame, for these two rows, the
Stage carries enough to distinguish them from both bad cases — but no single existing rule does
it, and the combination that would is not yet a decision anybody has taken.** That is the gap
[[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] named, stated in evidence
rather than in principle.

## 3. How Director could express a proposed Click without mutating the observed RoleUnknown

**The separation the owner asked for already exists in the types. `ResolvedTarget` carries
identity and geometry as different fields, and the geometry is already chosen by one line.**

```
  identity   ElementID, Role, Label, Query          what was meant
  geometry   Point                                   where it would be pressed
  account    Explanation, Confidence, Alternatives   why, and what else was considered
```

`target.resolved` sets `target.Point = el.ClickPoint()` — the centre of the identified element,
and the only place in the semantic stack where an interaction surface is chosen. `uiact.TargetFrom`
copies that `Point` into the ladder's `Target`, and `clickRung` presses it. Measured on this
frame, handed the SETTINGS Prop by hand: `hasbounds=true offscreen=false patternsknown=false
point={469 775}` — the middle of the word.

So an interaction hypothesis is a **different `Point`, with its own sentence and the surface
Prop's own id**, carried on the Resolution. Nothing writes to `Element.Role`, nothing writes to
the Stage, and nothing persists: a Resolution is a value built from one request and returned up
one call, exactly like the licence it already carries. The Stage keeps saying `RoleUnknown`,
because that is still true.

The ladder needs no change at all: `SemanticInvoke` is `invoke -> click -> refuse`, the invoke
rung requires a `NativeID` that a pixel-only Prop does not have, so a licensed structural target
would fall to the click rung, which requires only bounds. That is the correct behaviour and it
already reads as a fallback in the record.

## 4. Which existing policy boundary should evaluate the proposed action

**Grounding, in `target.score`, where [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]]'s
licence already is — because the two boundaries behind it refuse the whole world before any
action is considered.**

Measured, the full chain for "click Settings" on this Stage:

```
  1  resolver, grounding     e194 scores 0.349 {text 1.0, inert 0.35, evidence 0.99}
                             rejected "not an interactive control"
                             -> no usable candidate -> unobservable, blocker "actionability"
                             "nothing in this window can be operated, so no control could be matched"

  2  policy checkWorld gate 1   coverage 0.000  ->  REFUSE
                             "only 0% of this window's content was visible (quality was 6%...)"

  3  policy checkWorld gate 2   Blind(), actionability 0.000  ->  REFUSE
```

And the licence is already satisfied. `e194` has sources `[structure ocr]`, so
`OnlyDescribesPixels()` is true and `DescribesAnObject()` is true, and
`OperableUnder(licence, "click")` returns **true with no reason** — ADR-202 works exactly as
designed on a real structural Stage. The single thing that refuses at grounding is
`acts.Any()`, which is false because `RoleUnknown` affords nothing.

**Gate 1 is a finding in its own right and nobody has reported it.** `Coverage = content/n`
where `ElementRole.Container()` lists `RoleUnknown`, so 228 structural Props are 228 containers
and coverage is 0.000 — the signature the gate was built to catch is *"an application that has
not enabled accessibility reports a handful of anonymous containers"*, which is precisely what a
structural Stage looks like from the outside. Quality is 0.056 for the same reason:
`structuredShare` is 0 because `SourceStructure` is deliberately not `Structured()`, and mean
element confidence is 0.09 because most regions have low contrast.

So even a resolver that resolved would be refused twice, with sentences that are now wrong about
why. Evaluating the hypothesis at grounding keeps it beside the licence, keeps the refusal a
sentence about the TARGET rather than about the world, and leaves `checkWorld` to be reconsidered
on its own terms rather than as a side effect of a target change. A hypothesis admitted at
grounding and then refused by a coverage measure that counts it as evidence of blindness would be
the system contradicting itself.

## 5. How to reject oversized, ambiguous and caption-only cases

The three cases are different and the measurements say so:

- **Oversized / decorative.** `{482 388 1438 692}`, 48% of the window, fill 0.27, 1172 parts, one
  peer, and `Locates()` true. The sole owner candidate for its own text. This is the case where a
  Prop IS named ("OPPONENTS", confidence 0.99) and must still never be pressed, and it is the
  strongest reading of the owner's safety test: a named 48% region is more dangerous than an
  unnamed one, because every rule that keys on having a name admits it.
- **Ambiguous rows.** Already refused twice over, and neither refusal needs changing.
  `Resolver.AmbiguityMargin` (0.12) with `MinContenders` 2 refuses two Props matching the words;
  fusion's `ambiguousRival` refuses two comparably-sized containers for one text. A surface
  proposal must inherit the second: if two candidates contain the identifying text within
  `ambiguityMargin` AND are within `nestingRatio` of each other, there is no surface, and the
  honest answer is the owner's own sentence — *"I found Settings, but I can't confidently
  determine where to click. Show me."*
- **Caption-only.** Already the licence's job, and it already works. `DescribesAnObject()`
  excludes OCR, so a Prop whose only source is a read word is refused with *"only read text
  describes it, which locates a word and not the control it is written on"*. A proposed surface
  must come from a structural observation; a text box may identify and may not be pressed. This is
  [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]]'s lesson one tier up, and ADR-202
  already encodes it.

**What is NOT settled, and should not be settled by an implementation:** the Explorer toolbar.
Fill, parts and `Locates()` all admit it; only area ratio (40.6x vs 6.98x) and peer structure
separate it from a menu row. Deciding between a ratio bound and a peer/pitch rule is the decision
the owner reserved, and it needs a third and fourth interface before either number means anything.

## 6. Whether the regrounder's logic can be reused without coupling to workflowrun

**The primitives, yes, with no coupling at all. The orchestration, no, and it should not be.**

`go list -deps ./internal/director/observe` returns `directorapi`, `perception/selection` and
`perception/windowref`. It does not import `workflowrun`, `workflowstore` or anything in the
pixel stack. So `observe.Region`, `Locates`, `NameCandidates`, `SameControl`, `NamesControl` and
`Covered` are importable from the semantic stack today.

What is reusable is the **shape of the decision**, which `cmd/director/reground.go`'s `surfaceFor`
already states better than anything in the semantic stack does:

> WHAT IDENTIFIED IT IS NOT NECESSARILY WHAT TO PRESS. The text that names a control is often its
> CAPTION, and a caption is not the control. … So the text IDENTIFIES, and the press goes to the
> surface the DEMONSTRATION establishes: the smallest candidate on this look that contains the
> identifying text and agrees with a saved reading of this control. … and nothing invented when
> there is no such candidate — then the text's own region is pressed, as before, and the report
> says which of the two happened.

Four properties to carry over verbatim: identify by text, take the **smallest** containing
candidate, require an INDEPENDENT corroboration, and **report which of the two happened** rather
than silently substituting a surface.

What cannot be carried over is the corroboration itself. `surfaceFor` corroborates against
`g.art.Readings(control)` — a saved demonstration — through `observe.SameControl`, which is
mutual centre containment. There is no demonstration on the Stage path, and the test does not
transfer to the caption/row pair anyway: the SETTINGS row's centre is (320, 778), which is outside
the word's box, so `SameControl` refuses the correct pair. The structure of the rule is reusable;
its evidence is not, and substituting a different corroboration while keeping the name would be
the worst of both.

`cmd/director/reground.go` is in `package main` and is not importable, which is the right place
for it: it is a CLI-level orchestration over an interface the coordinator deliberately cannot see
inside.

## The smallest general-purpose change

Stated as a decision for the owner to approve or refuse, not as a design:

1. **A surface proposal carried on the Resolution**, not on the Element: the surface Prop's id,
   the point, and one sentence saying how it was chosen. `ResolvedTarget` already separates
   identity from `Point`; this adds the account of why `Point` is not `ClickPoint()`.
2. **One decision at grounding**, beside the licence: an element that is named, licensed for this
   verb, has a structural source, and affords nothing because its role is generic MAY be offered
   with a proposed surface — and is refused when the surface is ambiguous, when no structural
   candidate contains the identifying text, or when the only candidate fails whatever rule answers
   the open question in §5.
3. **Nothing else.** No role assignment, no `Any()` change, no `Targetable()` change, no new
   persistent identity, no detector or threshold change, and no second geometry algorithm.

And one consequence that must be decided before the slice can complete end to end: **policy gates
1 and 2 refuse this world outright**, for reasons computed from role shares that a structural
Stage inverts. A surface proposal admitted at grounding and then refused as *"only 0% of this
window's content was visible"* would be worse than today's refusal, because it would be wrong in
a way that reads as right.

## Limitations

- One frame of one game and one desktop fixture. Every number above is two data points at best,
  and the §5 open question is open precisely because two points do not fix a rule.
- The Explorer fixture is a PNG served through `capture.WindowCapture`, so it has **no
  accessibility either**. There is no measurement here of what a surface proposal would do on a
  Stage where accessibility is present; the hand-built `execute/licence_test.go` cases are the
  only coverage of that, and a licence is never reached when an actuating source reported the
  element.
- RESUME is still unread, so the row-shaped region under it has no identifying text at all and
  nothing in this experiment reaches it.
- `report.Text.Decisions` for this frame records 13 filled labels, 1 reinforced, 0 supporting, 5
  standalone and **11 rejected conflicts**. Eleven text observations were refused for
  contradicting a label; none was examined here.

## Related

- [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] — the Stage this measures,
  and the gap it named.
- [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] — the licence,
  measured passing on a real structural Prop for the first time.
- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — the regrounder whose decision
  shape transfers and whose evidence does not.
- [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] — a caption is not the control, one
  tier down.
- [[Experiment-052-what-the-structural-proposer-offers-the-stage]] — the eight-pixel overrun,
  re-measured here against the rule that actually governs.
- [[Experiment-053-what-the-stage-can-already-say-it-is-unsure-of]] — the dropped fusion Report,
  which is where this relation is computed.
