---
type: decision
status: accepted
date: 2026-09-08
affects:
  - passive-observation
source_paths:
  - internal/director/observe/establish.go
  - cmd/director/observeambient.go
  - cmd/director/serve.go
---

# ADR-134 — where every tallied control went

## Context

An audit of the durable store, after a five-minute natural Watch & Learn run, found:

```
durable Places (subjects):   512    chrome 245 · discord 203 · explorer 35 · code 29
durable relationships:         7
durable targets/affordances:   1
```

Place establishment is plainly reachable and general. One durable affordance against 512 Places
and tens of thousands of admitted controls is not obviously either healthy or broken — and the
existing counters could not tell those apart. `AffordancesAdmitted` says how many controls a
sweep was willing to name RIGHT NOW; `AffordancesStored` says how many became durable. Between
them sit three gates in `TargetsToRecord`, and every one of them was a silent `continue`.

The three have opposite remedies. "Nothing held still", "nothing resolved to a Place" and
"nothing recurred" are different problems, and conservatism working exactly as designed looks
identical to a gate nothing can reach.

## Decision

`TargetsToRecordExplained` returns the signatures and an account of where everything else went,
counted in CONTROLS:

```
affordance_recordable
affordance_not_recurrent
state_has_no_durable_place
state_not_settled
affordance_already_counted
affordance_ceiling_reached
```

`TargetsToRecord` is that function with the account dropped, so there is one traversal and one
set of gates. A caller that re-derived these counts would be a second implementation of the
policy, free to agree with the code and disagree with the run — the rule the crossing funnel
([[ADR-133-a-crossing-that-becomes-nothing-says-which-door-it-left-by]]) and the escalation note
both follow.

Counted in controls rather than states because the question is where thousands of admitted
controls went, so a state refused before its contents are considered contributes all of the
controls it was holding. A settled state holding no affordances is a real exit and is
deliberately absent: it has nothing to count, and a zero-valued outcome that never moves is a
line a reader learns to skip.

## What it does not do

It weakens no gate and changes no behaviour. The three concepts stay distinct — *currently
actionable*, *admitted current Target*, *durable reusable affordance* — and nothing here implies
the admitted ones should become durable. Discord already demonstrated why content cannot become
reusable interface memory.

It carries counts and an application name. The outcomes are `observe`'s own closed vocabulary:
never a control's name, its bounds, or the words read inside it. The ambient-status allowlist is
the guard, and it correctly refused the field until it was reviewed and named.

## And the reporting layer holds no membership list

`printFunnel` knows an ORDER and nothing else, so an outcome added to production later is
printed rather than silently dropped. A report that omitted a new outcome while production
recorded it faithfully would recreate the exact silence these funnels exist to end.

## Enforced by

- `TestTheAffordanceFunnelNamesEachGate` — each gate, one outcome, exactly one per control.
- `TestTheAffordanceFunnelKeepsUnsettledApartFromPlaceless` — the two refusals with opposite
  remedies cannot be collapsed.
- `TestTheAffordanceAccountDescribesTheWriteItCameFrom` — the account and the signatures come
  from one traversal.
- `TestTheFunnelReportDoesNotHoldAClosedList` — an unrecognised outcome is printed, known ones
  lead, and a zero is not printed at all.
- `TestWatchingFromOnToOffAndGone` — the ambient status may not grow a field without review.

## A correction this audit forced

`director graph` is the ACTION graph. It was read as "everything Marco knows", and an empty one
was reported as zero durable knowledge while semantic memory held 512 Places and 7 relationships.
Three layers exist and the surfaces do not yet make that unmistakable:

```
semantic memory   Places, durable Targets, relationships
action graph      executable, plannable topology
watching          transient observation, candidates, funnels
```

That is a product-surface problem, recorded here and not addressed.
