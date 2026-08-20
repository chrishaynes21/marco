---
type: reference
status: active
updated: 2026-08-06
---

# Glossary

Terms with a specific meaning in this codebase. Where a word has an everyday sense that
differs, the difference is the point.

**Observation** — a single piece of evidence from one provider at one moment, carrying its
provenance and confidence. Never a fact about the world. See [[Perception]].

**World state** — the Director's *belief* about what is on screen. Produced only by fusion.
See [[ADR-001-observations-vs-belief]].

**Fusion** — the single stage that merges observations into belief. See [[Fusion]].

**Provider** — a source of observations: accessibility, OCR, vision, visual state.

**Provenance** — which provider produced an observation, and when. Retained so a belief can
always be traced back to the evidence that produced it.

**Actionability** — what can actually be *done* to an element, derived from its real
reported capabilities rather than from its appearance. See
[[ADR-004-vision-cannot-establish-actionability]].

**Structural** — describing a *thing* rather than rendered content. A class is structural if
it may become an element observation; `text` and `image` are not.

**Nameable role** — a role whose label may be kept in plaintext, because the name is a
property of the interface rather than of the person using it. The closed allowlist is
`button, menu_item, menu, tab, checkbox, radio`.

**Unknown** — the Director could not look. Never a synonym for false. See
[[ADR-006-unknown-is-not-false]].

**Unsatisfied** — the Director looked, and the condition did not hold. Distinct from Unknown.

**Lowering** — turning a semantic step into legal Marco source. See [[Marco-Boundary]].

**Program** — an ordered sequence of semantic steps with whole-request validation, executed
one at a time with verification between. See [[Programs]].

**Goal** — an outcome the user described, expanded through a hand-written typed procedure
into an ordinary program. Never LLM-generated. See [[Goals]].

**Procedure** — the hand-written expansion of a goal. 18 of them exist.

**Collection** — a bounded semantic *query*, re-run every iteration. Never a captured list
of ids or handles. See [[Collections]].

**Capability ladder** — the run-time choice of how to perform a verb, from the control's
real capabilities down to a refusal, recording every stronger rung rejected and why. See
[[Semantic-Actions]].

**Action graph** — the record of what was done, storing the verb and the query but never
the mechanism, so a replay chooses the lowering again. See [[Action-Graph]].

**Generation** — a window's identity epoch. A recycled handle gets a new generation, which
is what makes stale geometry detectable. See [[Windows]].

**Game pack** — a per-application contribution of entities, detection and safety rules. See
[[Game-Packs]].

**Frozen fixture** — a stored frame corpus a benchmark replays. Two backends measured
against two different moments are not comparable, and the difference would look exactly
like a difference between models.

**Learn** — the person demonstrates, Marco acquires. The name in the product and in the code
alike: `marco learn`, `director learn`, `internal/director/learn`, `internal/voicelearn`, the Learn
panel, and the overlay's `learn <name>`. `teach` still answers on the CLI and in the overlay as an
undocumented compatibility alias for the same flow, and means nothing else. See
[[ADR-048-learn-teach-and-do-are-three-different-sentences]] and
[[ADR-086-one-acquisition-one-word-one-request]].

**Teach** *(product sense)* — Marco guides the person through something it already knows,
pointing at their live UI. The person still performs every action. Reserved for this; it is
**not** the acquisition flow.

**Do** — Marco performs a learned behaviour itself, under the existing authority model.

Those three are one sentence each, and the whole vocabulary turns on who acts:

```
LEARN   The person acts.   Marco watches and acquires.
TEACH   The person acts.   Marco guides them through it.   <- reserved, not built
DO      Marco acts.        The person delegates.
```

**Play** — a durable behaviour Marco can perform: legal Core Marco on disk, with a past. The
product-facing noun; it is STORED by `internal/routes`, which keeps its name because a package
name is something no person ever reads. `teach` was a different case and was renamed — the product
needs that word back. See [[Plays]] and [[ADR-081-a-durable-behaviour-is-a-play]].

**Binding** — a way IN to a Play, never the Play itself: a hotkey (`routes.Binding`) or the
Play's own scoped name resolved by `Registry.Resolve`. See [[Plays]].

**Saved** — a Play is written down, readable and editable, and nothing can ask for it: it sits in
`<app>/learned/`, which route discovery does not scan. **Registered** — the file has been moved
where the resolver looks, so a name reaches it. Two operations against two directories, not a
flag. See [[ADR-028-a-learned-play-is-a-file-with-a-past]].

**Focus** *(scope)* — the Play answers from anywhere AND Marco brings its application forward
before running it. Distinct from **global**, which answers anywhere and switches nothing, and
from **context**, which answers only while its application is already in front. A learned Play
registers as focus. See [[ADR-080-a-learned-play-is-asked-for-from-anywhere]].

**Recorded** — how a Play generated from a demonstration (`routes.KindTaught`) is presented,
because *Teach* above is spent on the other direction of travel. See [[Plays]].

**Invocation** — one request from the Audience, from the moment it arrives to the moment its
outcome is announced. **Intake** — the single place that decides what an invocation means
(`internal/invoke` for the decision, `cmd/marco/intake.go` for the process around it). See
[[Invocation]] and [[ADR-083-one-invocation-intake]].

**Entrance** — a way an invocation arrives: typing, speaking, a hotkey, a clicked Run, a shell.
**Source** is which one, recorded on every request and **never consulted** to decide anything —
transport is not meaning.

**Explicit identity** — a `routes.Route` the surface already holds, handed to the intake instead
of a phrase. A clicked Play or a resolved Binding stays that Play; its words are not read back and
matched a second time.

**Control phrase** — a bare request to stop (`intent.IsControlPhrase`: stop, cancel, abort, halt
and a few spellings of each). One definition, shared by the intake, the overlay and the Director's
own phrase routing. It acts on what is already running and is never ordinary text.

**Outcome** — what became of one invocation, in six words: `performed`, `clarify`, `refused`,
`unavailable`, `cancelled`, `failed`. Announced on the `[result] ` wire line. `performed` means it
ran *and* arrival was verified; nothing less may claim it.
