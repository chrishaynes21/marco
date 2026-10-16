---
type: experiment
status: complete
date: 2026-10-15
backend:
  - structure
  - ocr
  - vision
fixture: "five unrelated recorded interfaces — a Trackmania pause menu, Explorer, Windows Settings (Mouse), a browser settings page and a paint canvas — through the real collector, the real fusion engine and the real resolver, with internal/director/surface evaluating each request"
result: the-existing-evidence-is-sufficient-to-refuse-every-indefensible-surface-and-insufficient-to-choose-between-a-word-and-the-row-it-sits-on-so-the-honest-answer-on-the-case-the-milestone-is-about-is-ambiguous-and-the-one-missing-piece-is-which-of-two-nested-surfaces-is-the-control-which-no-amount-of-looking-supplies-and-one-demonstration-does
supersedes: []
affects:
  - execution
  - perception
---

# Experiment 057 — what earns a click, on five interfaces

**Status: MEASUREMENT, 2026-10-15. The evaluator is read-only and executes nothing; it is not
wired into execution.** Harness: `cmd/director/surfacehypothesis_corpus_test.go` under
`-tags corpus`.

## The question

> Can Director evaluate whether an explicitly requested, low-risk click on a `RoleUnknown`
> pixel-only Prop has a defensible interaction surface — without making `RoleUnknown` globally
> clickable, and without a threshold tuned to these five fixtures?

## What each interface answered

```
  TRACKMANIA  "click settings"   AMBIGUOUS — two defensible surfaces
    identity   e194 "SETTINGS" {388 764 162 22}  score 0.349  sources [structure ocr]
                                                 rejected by the resolver: not an interactive control
    e194  {388 764 162 22}  point {469 775}  cov 1.000  self   area 0.2%  fill 0.61 parts  3  sib 1/1   DEFENSIBLE
    e190  { 98 750 444  56}  point {320 778}  cov 0.951  anc    area 1.2%  fill 0.81 parts 12  sib 4/1   DEFENSIBLE
    unsettled: which surface is the control

  TRACKMANIA  "click opponents"  INSUFFICIENT
    e114  {482 388 1438 692}  area 48.0%  fill 0.27  parts 1172
      refused: it also contains "TRACKMANIA ASIA CUP", so its middle stands for several named
      things at once and which of them a press reaches is not something this evidence says

  EXPLORER    "click details"    ONE DEFENSIBLE SURFACE
    e11   {1028 100  42  28}  point {1049 114}  self  area 0.1%            DEFENSIBLE
    e10   {   8  90 1084  44}                   anc   area 5.4%  fill 0.95
      refused: it also contains "—", "WN"
    "The press would land on the region the words themselves were read on, which may be the
     control or may be its caption; 1 surface(s) around it were considered and refused."

  SETTINGS    "click mouse"      AMBIGUOUS — two defensible surfaces
    e68   {348 746 160  34}  self  area 0.5%  fill 0.75 parts 1  sib 0/1   DEFENSIBLE
    e66   {336 732 846  66}  anc   area 4.9%  fill 0.96 parts 1  sib 8/9   DEFENSIBLE
  SETTINGS    "click bluetooth"  AMBIGUOUS — two Props named alike; an identity problem
  SETTINGS    "click pointer"    INSUFFICIENT — nothing named like it (see below)

  BROWSER     "click storage"    ONE DEFENSIBLE SURFACE (the words), the 12.1% panel refused
                                 because it also contains "Appearance" and "About"
  BROWSER     "click about"      ONE DEFENSIBLE SURFACE (the words), same panel refused
  BROWSER     "click check"      ONE DEFENSIBLE SURFACE (the words), the 11.5% group refused
                                 because it also contains "Run the check", "Restore the
                                 defaults", "Up" and one more

  PAINT       "click brush"      INSUFFICIENT — nothing in this window is named like it
  PAINT       "click fill"       INSUFFICIENT — same
```

## What the evidence turned out to be sufficient for

**Every indefensible surface was refused, and the test that did almost all of the work was not
one of the shape measures.**

The decisive rule is **exclusivity**: a candidate is refused when it contains named Props the
request did not name, because then its middle stands for several things at once and which of them
a press reaches is not something the evidence says. It is a consequence of the other rules rather
than a new idea, it needs no threshold, and it refused:

- the 48%-of-window Trackmania background, which carries a name of its own (`OPPONENTS`, at 0.99
  confidence) — the strongest reading of the safety test, because every rule keyed on having a
  name admits it;
- Explorer's toolbar band, the case the owner named as the test of readiness;
- the browser's 12.1% navigation panel and its 11.5% control group, on an interface nothing was
  fitted to;
- and, through `observe.Region.Locates` reached unchanged, any band running the width or height
  of the window.

Paint, which has no readable text at all, produced no identity and therefore no surface — the
right answer for a canvas, reached without knowing it was one.

`fill`, `parts`, area share and sibling structure are carried as evidence on every candidate and
**decide nothing**. That is deliberate: [[Experiment-054-the-surface-a-named-prop-does-not-carry]]
measured fill and parts separating the background cleanly and failing on the toolbar band, and
[[Experiment-056-whether-a-row-is-one-of-a-set]] measured the sibling boundary falling exactly
between the toolbar's one sibling and the browser's two.

## What it is NOT sufficient for, exactly

**Choosing between a word and the row it sits on.** On the case this milestone is about, the
answer is AMBIGUOUS, and it is the honest one:

```
  e194  {388 764 162 22}  the region the words were read on
  e190  { 98 750 444 56}  the row around it
```

Both are reported by the shape proposer, both contain nothing else named, both locate a position
in the window, and fusion's own nesting rule says the second is an ancestor of the first rather
than a peer. There is no evidence here that prefers one. The same shape recurs on Windows
Settings (`e68` inside `e66`), on a completely different toolkit, which is what makes it a general
gap rather than a game's quirk.

**And the apparent successes are the same gap wearing a better hat.** Explorer's "Details" and all
three browser controls resolved to ONE defensible surface — and in every case that surface is the
identity's own region, i.e. the words. The ancestor was refused by exclusivity, which left the
caption as the only survivor. So the evaluator says so, in the sentence, every time:

> The press would land on the region the words themselves were read on, which may be the control
> or may be its caption.

A toolbar button is about the size of its label, so that press is probably right on Explorer.
[[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] is the record of it being wrong: the
caption's own rectangle was 118x32 and its centre sat 17 px above the bottom edge of the 354x221
tile it labelled. **The evidence cannot tell those two apart, so the sentence does not pretend
to.**

### The one missing piece, named precisely

> Which of two nested surfaces — a word's own region and the region containing it — is the
> control.

No amount of further looking supplies it. Both rectangles are really there; neither is a
misreading; no threshold separates them without being fitted to an example. What supplies it is
**one demonstration**: a person pressing the thing once establishes which of the two the press
belongs to, which is exactly what `cmd/director/reground.go`'s `surfaceFor` already uses when a
workflow has one —

> the text IDENTIFIES, and the press goes to the surface the DEMONSTRATION establishes

— and what the Stage path has no equivalent of. That is the honest handover from this milestone to
a teachable one, and the evaluator names it in `Hypothesis.Unsettled` on every ambiguous answer.

A second, weaker possibility that would need its own investigation: **hover**. A control that
highlights under the pointer distinguishes itself from the caption on it, and a single probe is
reversible. Nothing here measures that, and it would be an action rather than an observation.

## Four defects this found in its own first run

Reported because each produced a confident wrong answer that a green suite would not have shown:

1. **A two-letter OCR fragment answered a seven-letter request.** "click pointer" matched a Prop
   OCR had labelled `"te"` — `textScore` rates a label contained IN the request at 0.5, and "te" is
   inside "pointer" — scoring **0.174** against the resolver's `MinScore` of **0.25**, and the
   evaluator reported *one defensible surface* naming it. The resolver keeps a sub-MinScore
   candidate when it has a rejection to report, which every pixel-only Prop does; the fix is to
   ask the resolver's own threshold rather than to invent one.
2. **"One defensible surface" overclaimed when the survivor was the caption.** Now stated in the
   sentence, as above.
3. **Sibling distinctness counted the wrong thing.** It counted each sibling's own label, and a
   row's label is usually empty because fusion gives the words to the smaller region inside it —
   so Windows Settings' nine distinct rows reported `0` distinct. Counting the names INSIDE each
   member reports `9`, agreeing with Experiment-056.
4. **"structure and ocr and ocr"** — an element's provenance can hold two OCR references, and the
   explanation was a sentence about the bookkeeping.

## What was reused rather than rebuilt

- `fusion.Owners` and `fusion.Rivals` — the owner-candidate rule, its three constants and its
  ordering, now in `fusion/owners.go` and shared: `textCandidates` and `ambiguousRival` delegate
  to the same `rankOwners`/`rivalOf`, and a conformance test runs both over the same inputs and
  requires identical answers. There is no second geometry algorithm and `observe.NameCandidates`
  is untouched.
- `target.Resolver` for identity, including its ranking, its `AmbiguityMargin` and its `MinScore`.
  An identity this package chose differently from the resolver would be an identity the rest of
  the system does not agree about.
- `observe.Region.Locates` for the band rule, reached through the same function; the only thing
  written here is the normalisation into the space that subsystem speaks.
- `Provenance.DescribesAnObject` — ADR-202's own rule, which is why a caption is refused in
  ADR-202's own words.

## Limitations

- Five recorded interfaces, one frame each. Enough to refuse things; not enough to promote any
  shape measure to a rule, which is why none is.
- No fixture has an accessibility tree, so nothing here measures the evaluator on a Stage where
  identity and surface are the same object because a source said so. That case does not reach this
  code: a Prop an actuating source described resolves without a licence.
- Nothing is wired into execution. The hypothesis is a value; no path reads it.
- The exclusivity test depends on OCR having read the other things inside a candidate. On Explorer
  the toolbar band was refused because OCR produced two garbage-labelled Props inside it (`"—"`,
  `"WN"`) — the right answer for a shaky reason. Had OCR read nothing else in that band, the band
  would have been defensible and the answer AMBIGUOUS rather than one surface. It would still
  never have been *chosen*, which is the property that matters, but the margin is thinner than
  the result looks.

## Related

- [[ADR-205-a-surface-hypothesis-is-request-scoped-and-refuses-to-guess]] — the decision this
  measured.
- [[Experiment-054-the-surface-a-named-prop-does-not-carry]] — where the two candidate surfaces
  were first measured.
- [[Experiment-056-whether-a-row-is-one-of-a-set]] — the sibling evidence, and why it stays
  evidence.
- [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] — the licence,
  and the caption rule reused verbatim.
- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — the demonstration-backed
  version of the same decision, one tier down.
