---
type: subsystem
status: active
owners:
  - director
depends_on:
  - service
  - passive-observation
  - semantic-memory
  - hypotheses
used_by:
  - service
updated: 2026-08-12
source_paths:
  - pkg/playbill
  - internal/director/service/playbill.go
  - cmd/director/playbill.go
  - cmd/marco/watch.go
  - plugins/overlay/watch.go
  - plugins/overlay/watchview.go
  - plugins/overlay/cmd/marco-show
  - internal/director/observe/referent.go
  - internal/director/observe/placereferent.go
  - cmd/director/pointing.go
  - cmd/director/sightcmd.go
  - plugins/web-ui/sight.go
---

# Visibility

**One account of what Marco is doing, rendered three ways.**

Everything else in `docs/subsystems` describes something Marco does. This describes the only
thing it says — and the reason it exists is that Marco could already observe, infer, remember,
ask, learn, rehearse, generate Marco, authorise execution and verify reality, and a person
testing it could not see any of that happening.

## The account

`pkg/playbill.View` is the whole of what a presentation may know:

| section  | answers |
|----------|---------|
| CURRENT  | what application and screen does Marco think it is looking at, and how sure is it |
| SEEING   | what usable evidence is reaching it |
| THINKING | what interpretations and relationships are live |
| LEARN SESSION | where an explicit **Learn** session has got to — `playbill.LearnSession`, sourced from the coordinator |
| LEARNING | where PASSIVE learning has got to — what a watcher can tell without being asked. A different owner, deliberately kept apart from the row above |
| DOING    | where execution has got to |
| MARCO ASKS | the one question waiting on a person |
| WHY      | the most recent meaningful refusal or absence |
| JUST NOW | a bounded, semantic timeline |

Plus an opt-in DIAGNOSTICS section — providers, fusion, provenance, budgets, recall verdict,
proposal identity, rehearsal shape, grant state.

## The three readings

```
Director truth → playbill.View → Normal / Watch / Diagnostics
```

`View.Normal()` is one word and one sentence — the consumer surface, and in this milestone the
overlay's always-visible hint line. `View.Watch()` is the human-readable panel. `View.Deep()` is
`Watch()` plus the evidence, by construction.

All three live in `pkg/playbill`. A presentation chooses colour, order and layout; it never
chooses what a sentence says. See [[ADR-033-one-account-many-presentations]].

## The path

```
observationRegistry.Snapshot  ─┐
perception history            ─┼→ Runtime.Playbill      (cmd/director)
durable memory topology       ─┘        │
                                        ▼
                     Server.playbillFor  +  command registry, brokers
                                        │        (internal/director/service)
                                        ▼
                              PLAYBILL, protocol v6
                                        │
              ┌─────────────────────────┼──────────────────────────┐
              ▼                         ▼                          ▼
   marco director watch      overlay Watch / Diagnostics    overlay status line
```

The overlay decodes into `playbill.View` itself — `plugins/overlay/go.mod` carries the
`replace` the ocr and vision plugins already use — rather than into a hand-mirrored struct. A
mirror would drift, and it would drift silently in the one surface whose job is to tell somebody
the truth.

## What it will not do

- It does not analyse. `pkg/playbill` imports no `internal/` package, so a presentation has no
  path to the hypothesis generator or the recogniser.
- It does not act. See [[ADR-034-visibility-grants-no-authority]].
- It does not soften. See [[ADR-035-uncertainty-survives-the-screen]].
- It does not widen the privacy boundary. Every field is a count, a closed-vocabulary word, a
  Director-authored sentence, or a name a person typed. The admission guard in
  `pkg/playbill/guard.go` refuses anything else and a refused account is REPLACED by one saying
  so — failing closed, because a surface reading "the visibility record failed its own check" is
  a bug report and a surface showing a window title is a leak.

## Cost

Measured, because an observability surface that changes what it observes is worse than none.
On a Ryzen 7 9700X, per read:

| | time | allocations |
|---|---|---|
| `Playbill` | ~11 µs | 79 |
| `Playbill` with diagnostics | ~16 µs | 96 |
| `Watch()` render | ~2 µs | 39 |
| digest | ~2.6 µs | 90 |

Against a perception loop that samples every 500 ms and spends 170–730 ms of it on detection.
`cmd/director/playbillcost_test.go` is the benchmark; the number to watch is the order of
magnitude, not the digit.

## Coalescing

`View.Digest` fingerprints everything a person would notice changing, and deliberately excludes
clocks, freshness and sample counts — so a still screen produces a still panel however often it
is polled. The timeline consumes the observation session's own `LiveEvent` log, which already
publishes only material changes; nothing here starts a second recorder.

## Related systems

- [[Service]] — the transport, and the command half of the account
- [[Passive-Observation]] — where CURRENT, SEEING and the timeline come from
- [[Semantic-Memory]] — where a screen's name comes from
- [[Hypotheses]] — where THINKING comes from
- [[Learned-Plays]] — where the end of LEARNING comes from

## Decisions

- [[ADR-045-teaching-is-a-section-of-the-playbill]]
- [[ADR-059-a-presentation-belongs-to-its-claim]] — a grounding highlight is ephemeral
  presentation of one decision: current only during its phase window, dismissed when the
  claim ends, and never current on a settled session. The surface's hold timer is a
  backstop, not a lifecycle.

- [[ADR-033-one-account-many-presentations]]
- [[ADR-034-visibility-grants-no-authority]]
- [[ADR-035-uncertainty-survives-the-screen]]
- [[ADR-039-a-surface-and-a-place-inside-it]] — Watch distinguishes *"Part of the screen changed,
  in the same application"* from *"The screen changed"*, and Diagnostics carries **both**
  identity comparisons. One screen and no transitions is consistent with an application that
  never moved and with a comparison that could not see it move; only the within-a-screen figures
  tell those apart.

## Validated by

- `pkg/playbill/playbill_test.go`
- `internal/director/service/playbill_test.go`
- `cmd/director/playbillwiring_test.go`, `cmd/director/playbillmoment_test.go`
- `plugins/overlay/watch_test.go`

## Known gaps

- **A rehearsal in flight publishes no step position.** The attempt runs inside
  `director rehearse`; the account can say "I can try this once" and "I tried it" and cannot say
  "step 1 of 2" while it happens. `TestTheStagesThisSurfaceCannotSeeAreNamed` fails the day that
  changes, which is the right time to delete it.
- **A finished session carries no timeline.** The live event recorder retires with its session.
  Its findings are in the account; inventing moments for it would mean the second recorder this
  design avoids.
- **The anonymous-label share is not here.** It comes from the quadratic per-element
  explanation; `marco director explain` and the overlay's `perception` word still serve it.
- **Relationship and learning detail is session-terminal.** `Relationships`, `Topology` and
  `Learning` are computed when a session ends, so mid-session LEARNING is thinner than the
  post-session account.

## Pointing: "this is what I mean"

A highlight is a sentence. Marco resolves what it is currently referring to — an open question
first, then a settled one, then an interpretation it formed — to the regions that subject occupies
right now, converts them once in `pkg/referent`, and hands them to a separate presentation process
that draws and decides nothing.

Surfaces onto the same one referent:

| surface | what it points at | role |
|---|---|---|
| `director show-me` | whatever Marco is currently referring to | the proposal's own role |
| `director sight --show` | the same, with the perception account above it | — |
| What Marco Knows → "Show me what this refers to" | a durable judgement, **through recognition** | `knowledge_judgement` |
| Marco asks → "Show me what this refers to" | the question's own subject | `semantic_question` |
| `director learn` | START and DESTINATION, as they are established | `teach_start`, `teach_destination` |

The last row's two role values are **wire protocol**, not user text. They kept their spelling when
the flow was renamed to Learn ([[ADR-086-one-acquisition-one-word-one-request]]) — an out-of-tree
surface reads them, and a rename would have been a protocol change for a string nobody sees. This
table is the only place they are written down; it must match `observe.ReferentLearnStart` and
`ReferentLearnDestination` exactly.

Three rules hold across all of them:

- **It points at something Marco SAID.** Never the largest group, never whatever is convenient. A
  highlight with nothing behind it is Marco claiming to mean something it never said.
- **Never stored geometry.** A durable judgement is located by finding a live proposal that
  *recognised* that subject; the stored `Envelope` is identity, can be years old, and pointing there
  would confidently indicate empty screen — see the Explorer record.
- **Refusing is an answer.** Every reason is typed and every one has a sentence. "I can't see it
  right now" and "I'm not watching anything" send a person to different places.

`director sight` and the Sight panel say what Marco can perceive **with** — accessibility, vision,
OCR — each with its real state and, when off, the reason. Every source needs a positive signal that
it exists: an absent complaint is not availability, which is how OCR came to report ON in a Director
with no text engine wired.

See [[ADR-046-grounding-a-screen-points-at-its-structure]] for how a whole screen is pointed at
when a whole screen is exactly what cannot be pointed at.
