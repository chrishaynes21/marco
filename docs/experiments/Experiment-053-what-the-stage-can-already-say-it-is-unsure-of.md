---
type: experiment
status: complete
date: 2026-10-15
backend:
  - structure
  - ocr
fixture: "a reading of directorapi.Observation, Element, Provenance, KindEvidence and perception/fusion/text.go against the SETTINGS row, the oversized background region and the unread RESUME row"
result: yes-the-existing-representation-expresses-all-five-distinctions-already-and-fusion-associates-text-by-fractional-coverage-not-total-containment-so-the-smallest-change-is-a-provider-and-a-source-constant-with-one-limitation-the-fusion-report-is-dropped-at-the-observe-seam
supersedes: []
affects:
  - perception
---

# Experiment 053 — what the Stage can already say it is unsure of

**Status: DESIGN READING, 2026-10-15. No code written and none changed. No threshold, no
containment rule, no detector, no licence, no runtime.**

## The question

> Can the existing Stage representation retain a structural region and an associated OCR
> observation, preserving uncertainty, without claiming that the region is an operable control?

**Yes — every one of the five distinctions is already modelled, and one of them is modelled
better than the question assumes.**

## The five, against the three contrasting examples

### Can a structural region enter WorldState without a readable name?

Yes. `Element.Label` is an ordinary string with no non-empty requirement, and the type already
separates two different certainties:

```
  Confidence        how sure the Director is that this element EXISTS as described
  LabelConfidence   how sure it is about what the element is CALLED
                    "Zero means not separately assessed"
```

The comment is explicit about why: *"two sources disagreeing about a label still agree that a
control is there. One number would force a choice between under-trusting the element and
over-trusting its name."*

**The unread RESUME row is exactly this case** and needs nothing new: a region with bounds, no
label, and existence confidence from the source that proposed it.

There is a second, finer mechanism for the same idea. `KindEvidence` is *"provenance ABOUT A
FIELD rather than about an element"* with three values — `described`, `pixel_named`,
`pixel_only` — and its doc records that two of them were once collapsed and the collapse cost a
fix. So "something is here and nothing could say what it is" is already a representable claim.

### Can OCR and structural evidence remain separately attributable?

Yes. `Provenance.Sources` is `[]ObservationReference{Observation, Source}` — every contributing
observation keeps its own id and its own source, and `Provenance.Has`, `OnlyDescribesPixels` and
`DescribesAnObject` all read it. `Element.Sources` is the distinct set beside it.
`StateEvidence map[string]StateFact` already does per-FIELD attribution for state flags, which
is the precedent if per-field attribution is ever wanted for more.

### Can a possible association be represented without asserting that the text names the region?

**Yes, and this is the one already modelled better than the question assumes.**
`perception/fusion/text.go` has a four-outcome vocabulary for what a text observation did:

```
  reinforced_label       the element already had this label; the text agrees
  filled_missing_label   the element had no usable label and now has one
  supporting_evidence    ATTACHED AS PROVENANCE WITHOUT CHANGING ANY FIELD
  standalone_text        no structural element to attach to; stays in the observation
                         graph and becomes no part of belief
```

`supporting_evidence` **is** the owner's `relationship: spatially_associated, certainty:
uncertain`. The text reference joins the element's provenance; `Label`, `Role` and `Bounds` are
untouched. And `rejected_conflict` lowers `LabelConfidence` while its own sentence says
*"confidence in the LABEL is reduced, confidence that the element exists is not"* — the exact
separation being asked for, already written down.

The safety property is stated as a counter that cannot exist: *"there is deliberately no counter
for 'text that became an element', because that number cannot be anything but zero."*

### And the oversized background region does not win

The worry — *a 1438x692 background region overlaps SETTINGS too, that does not make it the
SETTINGS button* — is already answered, by a rule written against a measured failure:

```
  cov := el.Bounds.Covers(t.Box)      the share of the TEXT inside the ELEMENT
  minContainment = 0.80               admitted above this
  ambiguityMargin = 0.15              a rival must be within this to compete
  nestingRatio = 1.5                  a rival more than this much bigger is an
                                      ANCESTOR, not a peer
```

The comment on `ambiguousRival`: *"NESTING IS NOT AMBIGUITY … every ancestor of the real owner
scores exactly 1.0 as well … Measured live against VS Code, treating that as a tie rejected all
217 text observations — which looks like caution and is really just blindness."* Smallest
container wins among equals.

So on the measured frame: the SETTINGS word is 161 px wide and the 444x56 row contains 153 of
them — **coverage 0.95**, admitted. The 1438x692 background covers it at 1.00 but is roughly
forty times the row's area, far past `nestingRatio`, so it is an ancestor and not a rival. The
row wins.

### Which corrects [[Experiment-052-what-the-structural-proposer-offers-the-stage]]

That experiment measured the association with `observe.NameCandidates`, whose rule is TOTAL
containment, and concluded the eight-pixel overrun was the thing to decide about.
**`NameCandidates` is the workflow stack's rule and fusion does not use it.** On the Stage path
the association already succeeds. Nothing needs relaxing, and the instruction not to relax it
costs nothing.

### Can Director inspect these when target resolution refuses?

Yes, for the result. `Resolution.Candidates` carries every candidate considered *including the
rejected, with the reason*, and the pipeline attaches the resolution to the outcome before the
status check — which is how the live run reported *"only read text describes it, which locates a
word and not the control it is written on"*. `WorldState.Elements` and their `Provenance` are in
hand throughout.

## The first concrete limitation

**The fusion `Report` is dropped at the observation seam.**

```
  fusion.Engine.Fuse(cycle) (WorldState, Report, error)
  execute.Pipeline.Observe  func(ctx) (WorldState, error)
```

`Report.Text.Decisions` is where a `TextDecision` lives — outcome, rule, containment, and a
sentence. The *fact* of an association survives into `Element.Provenance`; the *grade* of it —
`supporting_evidence at 0.95 by rule X` — does not reach the Director. So Director can see that
an OCR observation contributed to an element and cannot see how well, or why fusion declined to
let it name the thing.

That is one seam and one signature, not a representation change.

## What would be needed, smallest first

1. **A `directorapi.ObservationSource` for structural proposals.** There is none today, and that
   absence is the safety story: `ActuatingSource` lists the four sources that can say how to
   OPERATE something, and anything not in it is non-`Actuable` by default. Proposed Props would
   arrive unoperable and
   [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]]'s licence
   would remain necessary.
2. **A provider that emits them.** `providers.NewCollector` takes any `observation.Provider`;
   the comment beside `rt.vision` already says *"adding a source is adding a line here"*.
3. **Nothing else.** No relationship graph, no ontology, no new Element field.

## Limitations that no representation change reaches

- **RESUME is unread** because a highlighted entry is dark text on white. The Stage can say *a
  row-shaped region is here and its text is unknown*; it cannot say it is Resume, and should
  not.
- **Four of nine entries got row-shaped proposals** on that frame. Representing uncertainty
  well does not make the proposer complete, and this experiment makes no claim that it is.

## Related

- [[Experiment-052-what-the-structural-proposer-offers-the-stage]] — the measurement, and the
  conclusion this corrects.
- [[Experiment-051-the-stage-two-real-applications-produced]] — why the Stage had no Props.
- [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] — the licence
  that stays necessary.
- [[ADR-107-a-sensor-appearing-is-not-the-screen-changing]] — where `KindEvidence`'s three
  answers came from.
