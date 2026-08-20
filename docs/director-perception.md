---
type: milestone
status: historical
---

# Perception: the Observation Graph and the Fusion Engine

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

> **Observations are evidence. World State is belief.**
> The Director reasons only over belief. Providers contribute only evidence.
> The Fusion Engine is the sole component allowed to convert one into the other.

## Why

The Director used to have one perception source, and the source *was* the world model:
what the accessibility bridge reported became elements, and every component downstream
silently inherited its assumptions.

That is fine with one source and fatal with two. When OCR reads "Save" where
accessibility already found a Save button, there is nowhere for the second account to
go except into a duplicate of the first — because nothing owns the decision that they
are one object.

## The pipeline

```
Accessibility ─┐
OCR ───────────┤
Browser DOM ───┼──▶ Observation Cycle ──▶ Fusion Engine ──▶ World State ──▶ Planner
Vision ────────┤        (evidence)                            (belief)
Skills ────────┘
```

| | package | knows about |
|---|---|---|
| evidence | `perception/observation` | sources, kinds, cycles |
| collection | `perception/providers` | bridges, window APIs |
| conversion | `perception/fusion` | **both** — the only place |
| belief | `world`, `target`, `plan`, `policy`, `verify`, `execute`, `actiongraph` | elements only |

Only `perception/fusion`, `perception/diagnostics`, the recorded-evidence adapter
(`internal/recorded`) and the composition root (`cmd/director`) may see both.
Enforced by `internal/director/perception_boundary_test.go`, in **both** directions —
perception may not import the reasoning layers either, because evidence that could
consult the planner is not evidence.

## Observation

An interface, because the kinds are genuinely different shapes:

```go
type Observation interface {
    ID() directorapi.ObservationID
    Source() Source
    Timestamp() time.Time
    Bounds() directorapi.Rect
    Confidence() float64
    Kind() Kind
}
```

Kinds: `element`, `text`, `window`, `application`. Only `element`, `window` and
`application` are emitted today; `text` is the shape OCR will arrive in and is defined
so that fusion already knows what it is holding.

The distinction between `element` and `text` is not pedantry. Text is a claim about
pixels, not objects: reading "Save" somewhere is *not* evidence that a Save button
exists, only that the word is on screen. Fusion may use it to reinforce a control an
accessibility source already found, and must not invent a control from it alone.

## Provider

```go
type Contributor interface {
    Observe(ctx context.Context, req Request) ([]Observation, error)
}
type Provider interface {
    Contributor
    Name() string
    Sources() []Source
}
```

`Contributor` is what a skill will implement — the smallest thing that can be asked for
evidence. It deliberately cannot see the World State: a source that could agree with
the belief it already produced is not corroborating anything.

`Request` narrows a cycle (`Window`, `Region`, `Sources`). Nothing honours `Region`
today, and the accessibility provider **says so** rather than filtering by bounds — a
provider that silently pretended to honour a narrowing would leave the caller believing
it had scoped the work.

The `Request` parameter is a deliberate departure from the narrowest possible
signature. A region-scoped provider cannot be given its region afterwards, so leaving
it out would guarantee the refactor this milestone exists to avoid.

### Refiner

```go
type Refiner interface { Refine(obs []Observation) []Observation }
```

One source adds detail to evidence another produced. The window system uses it: the
platform's window *enumeration* currently happens inside the accessibility bridge's
walk, so the window provider has nothing of its own to enumerate and instead adds
bounds, state and monitor placement. When it does enumerate, it becomes an ordinary
`Provider` and fusion merges the two accounts of each window exactly as it merges two
accounts of a button.

## Cycle

Observations are **ephemeral**; the World State is persistent. Evidence belongs to
exactly one cycle, and when the cycle is forgotten the evidence goes with it. What
survives is the belief, plus the provenance references naming what the evidence *was*.

The service retains the last **5** cycles for diagnostics — enough to see across an
observe/act/observe sequence, few enough that an always-on service does not leak.

A cycle where every provider failed is a **legitimate state**, not an error: no
observations, every provider in `Failures`. Turning that into an error would let a
caller retry it as though the desktop were unreachable, when the honest answer is that
the Director looked and could not see. `Fuse` returns an `error` it never uses, for the
same reason.

## Provenance

Every element records why it exists:

```go
type Provenance struct { Sources []ObservationReference }
type ObservationReference struct {
    Observation ObservationID
    Source      ObservationSource
    Kind        ObservationKind
    Cycle       string
    At          time.Time
}
```

The source and kind travel *with* the reference rather than being looked up. Provenance
is read long after the observations have been discarded — a bare id would by then name
nothing.

## Fusion report

Fusion is the one place in the Director where information is deliberately destroyed:
several accounts of a control become one element, and the losers leave no trace in the
result. Correct, and completely opaque without diagnostics.

```
director observations   who observed, and what they produced
director fusion         what fusion made of it
```

Both read the **running service** rather than observing afresh — a diagnostic that took
its own snapshot would report on a cycle nothing ever planned against, and would attach
a second accessibility client to the desktop to do it.

| field | meaning |
|---|---|
| `Merged` | observations absorbed into an element another had established. Zero with one source *by definition* — a source enumerating the desktop once cannot corroborate itself. |
| `Rejected` | evidence that produced no element and reinforced none. Not an error; a large count against a small element count means a source is not landing. |
| `Conflicts` | two sources disagreeing about one field. Not an error — the ladder resolves it, and this records that it did. |

## Explanation

> If the Director believes something about the desktop, it can justify that belief
> using structured evidence rather than implementation details.

```
director explain               every element of the latest cycle, one line each
director explain <element-id>  one element in full
director explain --last        the most recently minted element
director explain --chain       source → observation → fusion → identity → replay
director explain --json        the whole model
```

Every element answers: which observations created it, which were considered and
**refused**, which became primary, which merge rules applied, why it has this role and
this label, how its confidence was derived, and how it kept or acquired its identity.

### When it is computed

**On demand**, by re-running clustering over the retained cycle with recording enabled.

Recording an explanation for every element on every cycle is the obvious approach and
the wrong one: every command observes, a warm editor reports thousands of elements, and
establishing why each did *not* merge with each of the others is quadratic. A
diagnostics layer that made every command slower would be paid for by every user to
benefit whoever happens to be debugging.

Re-running is sound because clustering is a **pure, deterministic function** of its
observations. The one thing that is *not* reproducible is element identity, which
depends on the previous cycle's tracker state — so identity, and only identity, is
recorded live. That record is O(elements) and a few words each, bounded to the same
five cycles as the evidence: retaining identity for a cycle whose observations are gone
would be retaining something nothing can be said about.

Determinism is asserted by test, including byte-identical JSON across runs. Element ids
sort **numerically** (`e2` before `e10`), because a list that looks shuffled reads as
non-deterministic even when it isn't.

### Recorded, not reconstructed

The recorder hangs off the same code path that makes the decisions. A separate "work
out why these did not merge" routine would be a second copy of the merge rules, and the
first time the two disagreed the explanation would be confidently wrong — worse than
having none, because it would be believed.

`pairVerdict` returns the score *and* the rule name from one function for the same
reason.

### What is not recorded

Refusals between observations with **no spatial relationship**. That is not a decision;
it is the absence of one. Live against VS Code a single cycle records ~2,250 plausible
refusals out of ~14,000 pairs — recording the rest would bury the ones that explain a
duplicate element under noise nobody can read.

### Confidence

Opaque values are replaced by their derivation:

```
base       +0.90
           base trust in accessibility (90%), scaled by its own certainty
           in this observation (100%)
total       0.90
```

A 0.90 from one accessibility observation and a 0.90 from three weak sources agreeing
are different findings that a threshold comparison downstream cannot tell apart.
`ConfidenceExplanation.Consistent()` checks the derivation sums to its own total —
worth checking rather than assuming, because a derivation that does not add up is
actively misleading and fails silently.

### Diagnostics only

`internal/director/perception_boundary_test.go` forbids **any** package outside
`perception/` from importing `explain` — the planner, replay, verification, policy, the
action graph and the service included. The service transports explanations inside a
diagnostics payload without importing the package: it is a pipe with no opinion about
what flows through it.

If explanations could change what the Director does, they would stop being a
description of what it did.

### Observation browser

`director observations` follows each observation **forward** into what became of it:

```
ID                             SOURCE         KIND      LABEL              CONF BECAME
acc:uia:42.855212.4.12.8.346   accessibility  element   Explorer (Ctrl...  1.00 e246 (primary)
acc:uia:42.855212.4.12.8.347   accessibility  element   (unlabelled)       1.00 e247 (primary)
    refused by e246: same_source
```

An observation with an empty `BECAME` column is evidence that vanished — the one thing
this view exists to make impossible to miss.

## Merge criteria

`MergeCandidate` names the decision fusion makes the moment a second source exists.
Implemented only as far as one source needs; the fuller version weighs, roughly
strongest first:

- **shared window** — a hard gate, not a signal. Windows overlap, and geometry alone
  would merge a dialog's OK button with whatever sits behind it.
- **matching role** — *compatible*, not equal. OCR calls everything text.
- **bounds overlap** — the one signal every source produces. Coincident boxes score on
  IoU; a contained box (an OCR word inside its button) scores on coverage, which IoU
  rates near zero.
- **matching label** — strong confirmation; a genuine conflict is disqualifying.
- **shared parent** — decisive where both sources expose a tree. OCR and vision never
  will.
- **observation age** — a slow provider's box describes a screen that has moved.
- **provider reliability** — not the ladder (a fixed order over sources) but the
  observed behaviour of *this* provider on *this* application.

Speculative heuristics tuned against no second source would be tuned against nothing.

## Cost

No additional desktop work. One cycle is one accessibility snapshot, one batched window
enrichment and one monitor query — exactly as before; pinned by
`TestACycleCostsExactlyTheSamePlatformCallsAsBefore`. Fusion itself runs below the
platform clock's resolution on a 169-observation cycle.

Providers run **in order, not concurrently**. Concurrency would be free wall-clock today
(one provider does real work) and would immediately cost determinism: clustering breaks
ties by input order, and fixture reproducibility is worth more than milliseconds until
there is a second slow source to overlap with.

---

# OCR: the first source beyond accessibility

> **Accessibility may establish structure. OCR may establish visible text.**
> Fusion decides whether the evidence describes the same entity.
> **Only structural evidence may establish actionability.**

## Why the restraint

Reading the word "Export" somewhere is **not** evidence that an Export button exists. It
might be a heading, a log line, a tooltip, a disabled entry, or the word in a document
someone is writing.

A permissive OCR fusion turns an application with no accessibility support from *"the
Director cannot see into this"* into *"the Director is confidently wrong about this"* —
which is far worse, because the first state is visible and the second is not.

So the provider emits `observation.Text` **and nothing else**. It cannot construct an
element, assign a role, or make anything actionable, because none of those are
expressible in what it returns.

## Shape

```
active window ──▶ WindowCapture ──▶ Engine ──▶ Result
                                                 │  (image-local pixels)
                                    CoordinateTransform
                                                 ▼
                                        observation.Text ──▶ Observation Graph ──▶ Fusion
```

| layer | package | responsibility |
|---|---|---|
| engine | `ocr.Engine` | recognition, and only recognition |
| runtime | `platform/ocrclient` → `plugins/ocr` → tesseract | the external dependency, behind a bridge |
| capture | `platform/wincapture` | pixels + where they came from |
| provider | `perception/providers/ocr` | scoping, conversion, filtering, diagnostics |
| belief | `perception/fusion` | whether text and structure describe one entity |

Tesseract lives in `plugins/ocr`, its own module, launched as a subprocess — the same
arrangement as the accessibility bridge, and for the same reason: the engine module
permits no external dependencies.

## Trigger policy

**Opt-in.** `Request.Include` names OCR; an empty `Sources` means "everything the
providers can cheaply see", and a screen capture plus a recognition pass is not that.
The provider enforces this itself rather than trusting callers.

`Include` is distinct from `Sources` — which *restricts* — because asking for OCR
through `Sources` would switch accessibility **off**, and text with no structure beside
it is exactly what fusion refuses to believe anything about.

No automatic fallback is wired. The spec permits leaving it out; the provider and the
fusion rules come first.

## Conservative fusion

Text may reinforce or fill a label when **all** hold: same application, same window,
compatible observation period, ≥80% containment, and the structural label is empty or
compatible.

| outcome | meaning |
|---|---|
| `filled_missing_label` | a structurally real control that had no name |
| `reinforced_label` | an independent source read the same words |
| `standalone_text` | no structure under it — stays evidence, never a control |
| `rejected_conflict` | structural label **kept**; the disagreement recorded, label confidence reduced |
| `rejected_ambiguous` | two comparably-sized elements could own it |
| `rejected_stale` / `rejected_scope` / `rejected_geometry` | |

**Nesting is not ambiguity.** Containment measures how much of the *text* lies inside
the *element*, so every ancestor scores 1.0: a word on a button is equally inside the
toolbar, the pane and the window. Measured live against VS Code, treating that as a tie
rejected **all 217** text observations — which looks like caution and is really
blindness. A word is printed on the innermost control that contains it. Ambiguity means
something narrower: two candidates with equal containment **and** comparable size —
siblings, not ancestors (`nestingRatio`).

## Confidence

Bounded, named contributions — never `0.9 + 0.8 = 1.7`. Corroboration removes a
fraction of the remaining doubt and can never pass the cap.

`Element.LabelConfidence` is separate from existence confidence. Two sources disagreeing
about a name still agree there is a control there, so a conflict reduces the first and
not the second.

## Standalone text stays in the observation graph

Deliberately **not** promoted to World State elements this milestone. Doing so would
change element counts, move the coverage and actionability denominators, and put
OCR-only "elements" in front of the target resolver — widening downstream assumptions
for a benefit nothing yet needs. The spec permits this choice; it is recorded here
rather than assumed.

## Failure

OCR is optional and its failures are isolated. **"OCR is not installed" and "this window
has no text" never print the same way** — the first is a capability gap the user can
fix, the second a fact about the screen. `ocr.Unavailable` is a distinct error type, and
the provider never returns an empty success for a failed pass.

A window that **moved during capture** is refused outright: every box read from it would
be placed confidently on whatever is now at those coordinates.

## Diagnostics

```
director ocr                              read the active window
director ocr --region x,y,width,height    a rectangle, in canonical desktop coordinates
director ocr --json
```

Counters are named and every engine result is accounted for:
`accepted`, `rejected_empty`, `rejected_confidence`, `rejected_geometry`,
`rejected_stale_capture`. Thresholds are configurable and **provisional** — chosen by
reading tesseract output on real UI crops, not derived.

## Measured

VS Code, live: capture 17 ms, tesseract 724 ms, 261 accepted of 289 results, fused in
2 ms. 169 accessibility + 261 OCR observations → **167 elements, zero created by OCR**.
1 label filled, 16 reinforced, 11 conflicts recorded and refused.

The window sat at desktop x = **−1568** throughout; negative monitor coordinates are
ordinary and pass through the transform untouched.

---

# Visual state: appearance, and whether something changed

> **Accessibility may establish structure and actionability.**
> **OCR may establish visible text.**
> **Visual perception may establish appearance, state, and change.**
> Fusion decides whether that evidence belongs to an element that already exists.
> **Pixels alone must not create an actionable control.**

## Why: the Chrome double-Back

Clicking Back navigated the page — but the navigation had not finished within the settle
delay, so structural verification failed and the retry clicked Back a **second time**,
sending the browser two pages back when the user asked for one.

The guard that fixed it was *retry only when the world is byte-for-byte what it was*.
That is correct and **blind**: a page mid-navigation has an unchanged accessibility tree
— the old one, because the new one does not exist yet — so "nothing changed" and
"everything is changing" look identical from the structural side.

Watching the pixels answers exactly that question and nothing more.

## Fingerprinting

Deterministic, stdlib, no model. A region is reduced to a **grid of average colours**
(24×24 by default). Downscaling is what makes the comparison robust rather than merely
fast: antialiasing and sub-pixel text rendering average away, while anything a person
would notice moves whole cells.

| verdict | meaning | retry |
|---|---|---|
| `identical` | pixel-for-pixel | **safe** — the only state that proves nothing happened |
| `minor_change` | caret blink, cursor, clock digit | the structural guard decides |
| `meaningful_change` | something happened | **refused** |
| `still_changing` | it is happening *now* | **refused**, and the Director waits |

`still_changing` needs three captures — a region that differs from *itself*. That is why
`Watch` exists and why it is bounded: a spinner would otherwise be watched forever, and
after a few rounds "this is still going" is already enough to decide not to retry.

A comparison that **cannot be made** — a missing fingerprint, a resized region — reports
`meaningful_change`. Unknown must never read as "nothing happened", because that permits
a retry.

## Fusion rules

Visual state attaches only to an element that already exists, and only where the
element's **role permits the state**: a tab may look selected, a checkbox may look
checked, a button may look pressed. A pane may not look like a button, because there is
no visual kind that says "button".

| outcome | meaning |
|---|---|
| `filled_state` | nothing structural reported it; appearance filled the gap |
| `reinforced_state` | structure said the same thing |
| `recorded_change` | a region changed — evidence about an **event**, written onto nothing |
| `rejected_conflict` | structure says otherwise and **wins**; the disagreement is recorded |
| `rejected_role` / `rejected_geometry` / `rejected_ambiguous` / `rejected_stale` / `rejected_scope` | |

`Element.StateEvidence` records **which source** established each flag. `applyState` now
writes structural claims into it too, which is what lets fusion distinguish *"structure
said false"* from *"structure said nothing"* — two situations a bare boolean cannot tell
apart, calling for opposite behaviour.

A conflict reduces confidence in the **state** and touches nothing else. Two sources
disagreeing about whether a tab is selected still agree there is a tab there.

Appearance is the most **perishable** evidence the Director has — a hover that has since
ended looks exactly like one that has not — so `maxVisualAge` is 1.5 s, tighter than
text's 3 s.

## Change is not identity

A changed region is evidence about an **event**, not a durable property. It is never
written onto an element and never enters semantic identity — otherwise "click that
again" would break every time something animated.

## Detection says nothing rather than guessing

Only two appearances are inferred from a colour grid: a **flat wash contrasting with its
surroundings** (highlight) and a **region with almost no colour variation** (greyed out).
`checked`, `pressed`, `expanded`, `loading` and `progress` return **no observation** —
they need a before-image, a template, or semantics this package deliberately lacks.

A missing observation costs a piece of state the Director never had. A wrong one puts a
confident falsehood into fusion, where it is attached to a real control and believed.

## Diagnostics

```
director visual                              two captures of the active window
director visual --region x,y,width,height    canonical desktop coordinates, negatives included
director visual --json
```

Two captures, always — one frame can report appearance and can say nothing about change.
The output states the **retry consequence** explicitly, because that is what the command
is for.

## Bounded by construction

- one small rectangle per action — the target's bounds plus a margin, because a click's
  effect is frequently just *outside* the control;
- `MaxRegionArea` refuses a desktop-sized "region";
- the watch loop is capped at `MaxWatchRounds`;
- nothing captures on an ordinary observation cycle.

## Measured

Live: a click on a VS Code tab produced

```
✓ visual_region_changed   the target region changed appearance, which is consistent
                          with the action having landed without establishing that it
                          did: 13.0% of the region differs (settled after 2 round(s))
```

Weight 0.45 — appended beside the structural checks, never substituted for them. A
region changing after a click is consistent with the click working and equally
consistent with an unrelated repaint.
