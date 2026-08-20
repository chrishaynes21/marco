---
type: experiment
status: active
date: 2026-08-17
result: >
  Same-place revisits are stable where it matters — Settings 15/15 and VS Code 15/15
  identical signatures across six independent visits; Chrome and Discord vary because their
  content genuinely changes. The "24 vs 10 members" premise was wrong: a screen state's
  identity is roles and terms only, and those figures came from the grounding line. The
  remaining suspect is the settle contract, which guarantees a stable MODE rather than a
  complete composition — an evidence-completeness question, not a tolerance one. Duration
  independence is not yet measured; that batch was void.
affects:
  - semantic-memory
  - passive-observation
source_paths:
  - cmd/identityprobe
  - internal/director/observe/recall.go
  - internal/director/observe/hypothesis.go
---

# Experiment 014 — identity variance across real applications

## Question

Roadmap 34's blocker is that the same real screen sometimes resolves to a different durable
subject, or to none. Before changing the matcher: **which layer actually varies, and on which
applications?**

## Method

`cmd/identityprobe` re-runs the PRODUCTION identity path over finished observation sessions:
`observe.SignatureOfState` — the same projection `PlaceNow`, `PlaceToEstablish` and the
relationship layer use — and `observe.CompareStructure`, the one matcher. It computes no
tolerance of its own, so a verdict here is the verdict the Director would reach.

Sessions were ordinary `director observe-game` runs against live software; each session is
one independent identity opportunity, which is what every teach pass is. Vision and OCR off
throughout.

## What the identity of a screen state is actually made of

Established by reading the producer, and worth stating because it corrects a common
assumption:

```
stateFingerprint → Roles (settled modal count per role) + Terms + TermsKnown
SignatureOf      → Members = 0, Envelope = nil        ← for a screen state, ALWAYS
```

So a screen state's durable identity rests on **role composition and interface terms only**.
It carries no member count and no envelope. The "24 members vs 10 members" figures that
prompted this experiment came from the GROUNDING line, which reports the dominant structural
group's size — a different quantity that identity never reads. That alone retired the leading
hypothesis.

## Results — same place, repeated independent visits (6s each)

| application | pairs same | pairs not-same | reading |
|---|---|---|---|
| Windows Settings | 15 | 0 | stable |
| VS Code | 15 | 0 | stable |
| Chrome | 4 | 11 | unstable |
| Discord | 10 | 5 | partly unstable |

Settings — the application whose live Learn attempts failed — produced **byte-identical
signatures across all six visits**. The producer is not inherently noisy.

Chrome and Discord vary by large amounts in content-bearing roles (`text` 85 → 141,
`list_item` 40 → 58, totals 418 → 498). Both were displaying live content: a polling web UI
and a chat channel. Classified **LEGITIMATE_UI_STATE_CHANGE**, not a producer defect — though
see the open question below about whether that is the right granularity.

## What this rules out, and what it leaves

- **PROVIDER_VARIANCE** — not implicated for Settings; the raw tree probe found a stable 32
  admissible controls, and six sessions agreed exactly.
- **IDENTITY_PROJECTION_VARIANCE** — not implicated for Settings.
- **SETTLE_FALSE_POSITIVE / PARTIAL_RENDER_ADMITTED** — **still open, and now the leading
  hypothesis.** Idle revisits are stable; the live failures all happened during passes that
  spanned a NAVIGATION, where a place is seen briefly and possibly mid-render rather than for
  thirteen settled samples.

## The settle contract, read from the code

`stateSettled` → `ScreenState.Settled` → `settledComposition()`:

```go
if s.n < StatePromotionCount || len(s.tally) == 0 { return false }
for role := range s.tally {
    if s.tally[role][s.settledCount(role)] < StatePromotionCount { return false }
}
```

With `StatePromotionCount = 2`, that reads: **at least two observations, and every role's
modal count seen at least twice.** That is a claim that the composition has a stable MODE. It
is not a claim that the composition is COMPLETE. A partially-rendered page observed twice
satisfies it.

Over a 13-sample dwell the mode is the complete composition, which is why idle visits are
stable. Over the handful of samples a place gets while somebody is navigating through it, the
mode is whatever was on screen for those few frames.

**That is the gap to close, and it is an evidence-completeness question rather than a
tolerance question** — exactly as [[ADR-062-a-scroll-bar-is-not-a-screen]] warned when it
fixed the previous cause narrowly.

## Method defect, recorded

The batch-3 driver read the session list before a new session had registered and captured the
PREVIOUS session's id, so the "short visit" files contain unrelated sessions (`inf=41` for a
2s visit is impossible). Those files were discarded rather than interpreted. Batch 1 is sound:
its session ids increment monotonically in the log and every file's application matches its
intended target.

**Duration independence is therefore NOT yet measured.** It is the next thing this experiment
owes, and it is the measurement the settle hypothesis needs.

## Open question for identity granularity

Chrome and Discord are the same PLACE to a person across visits and different SUBJECTS to
Marco. Whether that is correct is a semantic decision this experiment does not settle: a chat
channel with new messages is arguably the same place, and a durable subject per screenful of
content is a store that grows without bound. Recorded here rather than answered.

## Related

[[ADR-016-cross-session-identity-is-structural-and-conservative]] ·
[[ADR-062-a-scroll-bar-is-not-a-screen]] · [[Semantic-Memory]] ·
[[Experiment-011-two-level-identity-against-real-software]]
