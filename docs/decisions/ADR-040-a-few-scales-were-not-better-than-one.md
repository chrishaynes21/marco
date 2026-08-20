---
type: decision
status: accepted
result: rejected
date: 2026-08-11
supersedes: []
affects:
  - passive-observation
  - perception
source_paths:
  - internal/director/observe/screenstate.go
  - internal/director/observe/screenfixture/corpus.go
  - internal/director/observe/corpus_test.go
---

# ADR-040 — a few scales were not better than one

A bounded multi-scale structural field, with receptive scales spaced by the golden ratio, was
proposed to fix two named weaknesses in the single-grid local matcher. It was built, measured
against a deterministic corpus alongside the incumbent and a simpler middle option, and **not
adopted**.

## What was measured

Twenty-one structural situations, each labelled with what a person would say happened — *a
different place*, *the same place*, or *a property of the place*. Three matchers, asked the same
questions separately, votes never combined:

| | false state changes | missed meaningful changes |
|---|---:|---:|
| current single grid | 0 | 1 |
| same grid, two alignments | 0 | 1 |
| bounded PHI scale-space, 4 layers, 2 offsets | 0 | 1 |

The one miss is the same case in all three: a same-kind overlay over same-kind structure.

## The two stated weaknesses, probed directly

**Boundary straddling is not real.** The claim was inherited from how the grid matcher was
*described*, and the description predates its summing. It adds replaced mass **across** cells
rather than requiring any one cell to carry the whole change, so a panel split four ways still
contributes all of itself. Measured: the same panel at x = 0.16, 0.33 (the grid's own seam),
0.50 and 0.66, on surfaces of 42, 72 and 360 structures — **detected in all twelve**.

The candidate initially failed this and the incumbent passed it. Its first version required a
single receptive field to carry the change, so a panel across two fields was split below the bar
in both — the exact defect it was proposed to fix, reintroduced by the fix. More spatial
resolution does not by itself buy alignment invariance, and can cost it.

**The same-kind overlay is not a resolution problem.** A panel of the same kind of structure over
the same kind adds no role anywhere, so every composition-based comparison at every scale is blind
to it. The one thing a spatial field carries that a grid does not is *density*, and density does
not separate the populations:

```
                                     worst concentration by layer
same-kind overlay over the centre     0.61  0.37  0.24  0.37
same-kind overlay, larger             0.67  0.35  0.24  0.31
the list loads another page           0.79  0.40  0.26  0.12   ← MORE concentrated
ordinary churn                        0.79  0.40  0.26  0.12
```

A list loading its next page is *more* spatially concentrated than an overlay, at the coarse
scales where the signal would have to live. Classification: **STILL_INDISTINGUISHABLE**.

## What layers actually control — and what they do not

Adding layers bought nothing. Two through six all saw the same nine meaningful changes with zero
false places. What suppresses microchange is **not** the scale floor but the structure-count
floor: at a fixed four layers, lowering it from 8 to 4 to 2 to 1 produces 0, 1, 2 and 3 false
places. This matters beyond the candidate — a future performance policy that economises by
dropping layers would change cost and not meaning, which is the right property but not the one
the design was argued from.

## Cost

Per comparison, and the incumbent is run once per candidate state per inference:

| elements | current | offset | phi |
|---:|---:|---:|---:|
| 350 | 81 µs | 720 µs | 3.0 ms |
| 1200 | 245 µs | 2.4 ms | 10.2 ms |

PHI by layer count at 350 elements: 1.4 / 2.2 / 2.7 / 3.9 ms for 2 / 3 / 4 / 5 — roughly linear,
about 0.8 ms per layer. At the 32-state bound, 10 ms per comparison would dominate a 176–298 ms
perception cycle. A thirty-eight-fold cost for zero invariants is not a trade.

## The decision — REJECTED

The scale-space is **rejected**. The local comparison in `screenstate.go` remains Director's one
and only local-state matcher.

This is not a deferral, a phase, or a mode waiting for a product setting. There is no PHI code in
this repository — the candidate, the offset variant, the scale policy and the structural field
were all deleted once the numbers above were recorded. The word "Quality" was borrowed during the
experiment to name the candidate's policy; it carries no such meaning now, and future
Performance/Balanced/Quality modes, if they ever exist, are a product idea about **how hard Marco
looks** and never about what a remembered state means.

Reopening the question requires new evidence: a deterministic production-path test showing a
concrete failure the incumbent cannot answer. A theoretical improvement is not evidence.

### What was kept, and why

`screenfixture.Corpus()` — twenty-one structural situations, each labelled with what a person
would say happened. It is the lasting artifact, because the next candidate is entitled to be
measured against exactly the same questions rather than against fixtures chosen alongside it.
`internal/director/observe/corpus_test.go` measures the incumbent against it and defines the
narrow seam a future candidate would satisfy.

The PHI implementation was kept for one commit and then deleted. The measurements above are the
result; the code was only the means, and archaeological machinery in a repository reads as a
supported option to everybody who finds it later.

## What would change this

Not more scales. The unsolved case needs an observation dimension the perception vocabulary does
not carry: **containment, z-order, or appearance chronology** — something that says *this
structure is on top of that one* rather than *this structure is near that one*. These are
hypotheses about what is missing, not a roadmap. Whether a provider can supply any of them, and
at what privacy cost, is a separate question with its own evidence to gather — and the next live
session is what should decide whether the limitation matters in ordinary software at all.

## Enforced by

- `internal/director/observe/corpus_test.go` —
  `TestTheProductionMatcherAgainstTheStructuralCorpus`: the incumbent's score on the corpus,
  with the known miss logged rather than asserted away, and a guard that a second miss appearing
  is a regression.
- `internal/director/observe/screenfixture/corpus.go` — the corpus itself, including the
  boundary-straddling cases whose failure was predicted and did not occur.

## Related

[[ADR-039-a-surface-and-a-place-inside-it]] · [[Passive-Observation]]
