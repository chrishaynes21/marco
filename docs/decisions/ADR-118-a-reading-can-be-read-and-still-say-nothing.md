---
type: decision
status: accepted
date: 2026-08-31
affects:
  - perception
  - passive-observation
source_paths:
  - internal/director/observe/sufficiency.go
  - internal/director/observe/escalation.go
  - internal/director/observe/place.go
  - cmd/director/escalationwiring.go
---

# ADR-118 — a reading can be read perfectly and still say nothing

## Context

An Xbox game page describes its whole shell, spreads structures through its content area, offers
thirty-two actionable controls — and nothing on it claims a destination. Every game therefore
reads identically and collapses into one Place. Measured live:

```
director name-probe --title XBOX
  navigation    no element reports itself as the selected destination
  claims        none
  NO NAME
```

Tracing it found that **every gate on the path refuses, and each refusal is individually correct**:

```
ReachContent → Sufficiency{Sufficient, content_reached}
  → EscalationOf: case Sufficient → SpendNothing
  → shadow provider skipped
  → no rich evidence enters the World
  → no destination claim
  → NewScreenSignature: roles + cells + total, no text
  → every game is one Place
```

### 37C was not wrong; its conclusion was overgeneralised

37C measured a detector against **healthy desktop accessibility** — screens whose accessibility
already said what everything was — and found it added no actionable semantic item, at 645–1379ms
an inference. That result is sound.

What it became was *"do not buy vision when accessibility structurally reaches the interface"*.
Those are different claims. `SufficiencyOf` reads exactly two fields, `Placed` and `Reach`, and its
own comment says so: *"It is not a claim that the reading is COMPLETE."* It is about ARRANGEMENT.
A reading can be perfect at "could this be read" and silent at "which state is this".

`EscalationOf` already had a `Need` axis — watching, answer, act — and it was consulted **only in
the `Incomplete` branch**. On a sufficient reading no consumer could buy anything, whatever it
needed.

## Decision

### A second question, beside the first

`SemanticSufficiency` asks whether the reading said **which state this is**: `claimed`, `silent`,
or `unreadable`. It is derived, exactly as `SufficiencyOf` is, from a field `PlaceNow` fills from
the naming rule's own tally — there is one place that decides what counts as a claim and this is
not it.

**`SufficiencyState` is untouched.** Folding them would mean either calling a perfectly-read screen
"incomplete" — false, and it would send an owner hunting a broken accessibility tree — or letting a
semantic gap widen what `Incomplete` means, which is the classifier 37D exists to protect.

### Semantic silence may buy, once per settled screen

`EscalationOf` gains one arm: on a structurally sufficient reading, `StateSilent` returns
`SpendMore`. **37C's rule survives exactly where it was measured** — a reading that can say which
state it is buys nothing, for any need, forever.

The budget is **one repair per settled observation epoch**, keyed on `(SessionID, ScreenStateID)`:

- **Not on the Place**, because the Place is the thing currently getting this wrong. A collapsed
  identity would let one state spend the repair that would have told the others apart — the broken
  identity silently preventing its own fix.
- **Not on the signature**, for the same reason.
- `ScreenStateID` is the segmenter's own transient answer to "the screen materially changed",
  session-local and upstream of everything durable. The session travels with it because `state_2`
  in two sessions are unrelated screens.

Nothing about the budget is written down, survives the process, or reaches a Place, a plan or an
input. It is a spending record.

### What did not change

No raw detector label, OCR text, pixel or region count enters `NewScreenSignature`. 37H stands
whole. No OCR provider was added. No sensor is globally enabled. No application appears in any
production rule.

## Consequences

- A structurally healthy but semantically silent screen buys one inference when it settles, and
  none thereafter until the screen materially changes.
- **Not yet proven: that the repair's output becomes usable state evidence.** Vision has no model
  on the development machine (`vision-bridge is unavailable`), so the gate is verified and the
  detector's contribution is not. `SpendMore` followed by regions nobody can use is not success,
  and that half is unmeasured.
- A vision-derived label is not a destination claim — it is neither `Selected` nor `Navigable` —
  so a `StateSemantic` concept distinct from `DestinationClaim` is very likely still needed. It is
  not built here because building it before seeing what the detector actually returns would be
  designing against a guess.

## Measured on the way, and worth recording

The claim rule is a **weaker discriminator than assumed**, in the application that appeared to work
best. Discord's admitted claim reads `"Sometimes Silly, Voice call active"` — the SERVER plus a
transient status — while the window title carries the actual channel, `#clips-and-highlights`. So
the claim does not identify the channel and does change when a call starts or ends. That is a
plausible source of the four durable Places one channel accumulated, and it means "has a
destination claim" is a floor for semantic sufficiency rather than a solution to it.

## Enforced by

- `internal/director/observe` — `SufficiencyOf` unchanged; its own tests unchanged
- `cmd/director` `TestAReadingThatSaysNothingAboutItselfIsSemanticallySilent`
- `cmd/director` `TestASemanticallySilentReadingMayBuyOneRepair`
- `cmd/director` `TestOneSilentScreenBuysOneRepair`,
  `TestTheRepairBudgetIsOnePerSettledScreen`
- `cmd/director` `TestBuyingPerceptionGrantsNothing`
- `internal/director/perception/shadow` `TestASufficientReadingDoesNotBuyAnInference`

## Related

- [[ADR-117-observe-is-a-map]]
- [[ADR-116-watching-follows-the-window-not-the-executable]]
- [[Experiment-022-the-first-dogfood]]
