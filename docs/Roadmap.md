---
type: roadmap
status: active
updated: 2026-08-11
---

# Roadmap

Ordered by **expected information per engineering hour**, not by size. Each item states what
it would tell us, because an item that cannot change what is built next is not worth doing
first.

## 1. ~~Nameable-role coverage metric~~ — DONE 2026-08-06

`visionbench` now reports **nameable-role coverage**: the share of tracked things whose class
maps into the privacy plaintext allowlist (`button, menu_item, menu, tab, checkbox, radio`),
on the same denominator as structural coverage so the two are read side by side. It carries
**10 score points, taken out of structural usefulness** (30 → 20) rather than added beside it
— structure that cannot be named was being paid for twice, which is precisely how a detector
scoring 100% structural while emitting only `icon` survived three milestones. Weights still
sum to 100, so past scores remain comparable in scale.

Only four of the eleven vision classes clear the allowlist. `NameableDetections` records the
accepted-box count alongside the coverage ratio, and the sweep in `cmd/director` gained a
NAMEABLE column.

Enforced by `TestStructurePerfectionDoesNotHideAnUnnameableVocabulary` (two backends at 100%
structural coverage, differing only in whether their roles may be said out loud, must not
score the same) and `TestTheWeightsStillSumToOneHundred`.

Rerun on the frozen Rocket League fixture the same day: **classical CV measures 40% nameable**
(9 of 92 accepted boxes) and drops 61.9 → 55.9; **Grounding DINO measures 20% and is unchanged
at 20.2**, because its structure is entirely nameable and the reweighting holds such a backend
harmless — the intended property. `icon_detect` **could not be measured**: six of six frames
failed with "Vision has no model loaded", so its 0% is still a prediction from its class list.
See [[Experiment-001-vision-backend-comparison]].

- affects: [[Vision]], `internal/director/visionbench/metrics.go`, `score.go`
- constraint: [[ADR-003-evidence-authority-by-source]]

## 2. Swap in a UI-element YOLO detector — UNBLOCKED 2026-08-07, and now the NEXT task

**Challenger selected 2026-08-07: ScreenParser** (docling-project, Apache-2.0 weights,
YOLO11-Large, 55 UI classes). Six of Marco's nameable roles map onto its ontology with high
confidence; nothing else surveyed reaches more than two. Full survey, licensing matrix and the
rejected alternatives: [[director-vision-ui-detector-decision]].

The survey also corrected a stale assumption this project was carrying: OmniParser
`icon_detect_v3` is **MIT** (YOLOv9); only the earlier Ultralytics-based detectors are AGPL.

**Blocked on one decision:** no surveyed candidate ships ONNX. Producing it needs a one-time
Python + ultralytics export — a development dependency, never shipped, on a machine with no
Python installed.

The benchmark can rank a learned challenger honestly at last: ground-truth precision and
recall, sequence-aware temporal correctness, nameability that cannot be inflated, full-screen
normalised geometry, and declared scenery. Validated on the real corpus — a box spammer scores
**12.7** against a narrow-but-correct detector's **47.8**.

It is also now the *only* path left standing. Classical CV measures 0% structural recall on
real frames (item 4a), so "classical CV becomes the primary path" — the fallback this item
names below — is no longer available.

Original framing follows.

Prefer a permissively licensed YOLO-family model exporting to ONNX with a genuine class
vocabulary. The plugin already reads a model's embedded `names` map, so this is a **file
swap**: same runtime, same bridge, same acceptance thresholds, same fixture, same command.

```
director benchmark-vision --fixture fixtures/vision/rocketleague
```

| what to watch | current | success |
|---|---:|---|
| nameable-role coverage | 0% | > 25% |
| anonymous ratio | 100% | < 60% |
| structural persistence | 22% | > 50% |
| false structure | 100% | < 50% |
| median latency | 109ms | ≤ ~200ms |

Either outcome is informative. If it fails, the honest conclusion is that **game UI is not
desktop UI** — these models are trained on web and office screenshots — and classical CV
becomes the primary path.

Independently: the shipping detector's metadata reads `Ultralytics YOLO11m`, i.e.
**AGPL-3.0**. If it is being replaced anyway, replacing it with something permissive resolves
a real exposure at no extra cost.

The `nameable-role coverage` row is now produced by the benchmark itself (item 1), so the
success column above is checkable. The `current` column is still **predicted**, not measured:
running the shipping detector needs `-tags onnxvision` and `$MARCO_VISION_MODEL`, which the
2026-08-06 rerun did not have. The bar a challenger must actually clear is the measured one —
**classical CV at 40% nameable / 55.9** — not `icon_detect`'s 0%.

- ~~blocked by: item 1~~ — unblocked 2026-08-06
- affects: [[Vision]], [[Passive-Observation]], [[Game-Packs]]

## 3. ~~Pin the accessibility provider~~ — DONE 2026-08-06

The mechanism existed at every layer; the sampler never set `observation.Request.Window`.
VS Code went from 1 fused element to **353** (279 stable, with readable `button` and `tab`
labels), terminal foregrounded throughout. See [[director-accessibility-targeting]].

**This changes the framing of items 1 and 2.** The vision vocabulary was treated as *the*
blocker on naming; for accessibility-rich applications it never was. The open question is
now narrower and better posed:

> How strong must vision become for surfaces where accessibility genuinely does not exist?

Items 1 and 2 are still worth doing — a game HUD exposes no accessibility — but they should
be re-scoped against a corrected game baseline rather than against
[[Experiment-002-dnfc-observation-baseline]], whose conclusion is superseded.

## 3a. ~~Window-generation provenance and a fusion guard~~ — DONE 2026-08-06

A provider now reports what it was asked to observe **and** what it can prove it observed,
established after collection by re-reading the platform. Fusion compares the two and admits
no target-scoped evidence that cannot prove itself; an unknown observed target is refused
rather than excused. See [[ADR-011-provenance-is-proven-not-assumed]].

Most of the type layer already existed and **nothing called it** — `ProviderOutcome`,
`TargetProvenance` and `TargetProven` were complete and unit-tested while the collector still
called `Observe`, cycles carried no outcomes, and fusion read a flat list. The work was the
wiring: outcomes on the cycle, one guard call site in `Fuse`, a live `TargetResolver` backed
by the window tracker, `Tracker.Confirm` as a read-only re-validation, and `Request.Target`
set by the sampler. Four mutations verified, including unsetting that last field.

**Validated live 2026-08-06**, against a real desktop with VS Code, Chrome, Discord, a game
and Explorer running. Five sessions:

| test | result |
|---|---|
| stable target (VS Code, never focused) | 81 samples, accessibility proved generation 1 on every one, 0 refusals |
| moved across monitors (x=191 → x=-1800) | generation unchanged at 2, still proven — position is not identity |
| application restart | pid 12864 → 5712, generation 2 → 3, reacquired by selector |
| replaced mid-session | generations [3 4], 11 samples skipped while dead, no sample mixed the two |
| **sustained churn** (11 generations, 5→15) | **the guard fired: 1 of 118 samples refused, 3 observations quarantined** |
| game (DNFC) | 42 samples, generation 16 stable, proven |

The refusal reason was *"the window the accessibility source read is not the Director's
current target; it was replaced or the source fell back to another window"* — the in-flight
race, caught on real hardware rather than in a fixture.

**Cost: negligible, measured.** `Tracker.Confirm` over 400 live calls — median below clock
resolution, p95 511µs, max 639µs, mean 43µs — against a 157–248ms perception cycle and a
200ms sampling floor. Fusion's guard is O(provider outcomes), not O(observations), and
`Confirm` provably does not enumerate (`TestLiveConfirmDoesNotEnumerate`). No optimisation
warranted.

Still open:

- **The mixed-provider case is unproven.** Vision and OCR are unavailable on this machine (no
  ONNX model, no tesseract), so every live session was accessibility-only. That several
  providers of *different speeds* agree on one generation — the case the guard was designed
  for — has been shown in tests and not on a desktop.

- affects: [[Perception]], [[Fusion]], [[Windows]]
- constraint: [[ADR-009-window-identity-is-ephemeral]]

## 4. ~~Improve classical CV~~ — DONE 2026-08-06, ceiling reached

Restructured into generate → features → classify with explainable rejections, ablated rule by
rule, and shadowed over a real 1920x1080 screen. Full account in
[[Experiment-003-classical-cv-tuning]].

**Result: a hard ceiling, and a benchmark defect.** On a real screen the tuned detector halves
its output (83 → 42) while keeping every button and bar the baseline found. On the corpus it
shows no improvement at all — because the corpus is six *crops*, and a normalised size rule
needs opposite thresholds for a crop and a screen (0.045 scores well on the corpus and keeps
4 of 83 candidates on a real screen; 0.015 is safe on a screen and gains nothing on the
corpus).

Border continuity — the rule this was expected to turn on — was measured and refuted: the
corpus's genuine menu buttons score 0.27 and 0.54 while kept panels span 0.15–0.75. Nested
suppression was never built because measured nesting is zero.

The ceiling is not a missing heuristic: **a 24x24 uniform square is a checkbox and is a patch
of arena texture**, and rectangle geometry has nothing left to appeal to. Asserted by
`TestSpeckledTextureIsIndistinguishableFromControls`.

**Decision: B — Classical CV is retained as a region-proposal layer, not promoted.** It is
good at proposing (100% of the corpus's true structure kept, ~0 latency, zero dependencies)
and cannot assign semantic roles. It remains benchmark-only; `cmd/director/shadow_test.go`
still enforces that, so no runtime wiring test applies.

## 4a. ~~Rebuild the vision corpus~~ — DONE 2026-08-07

Corpus captured and measured. **Classical CV scores 0% structural recall on real full-resolution
frames** — it fragments a 400x283 pause panel into ~12 edge slivers, best IoU 0.065. The crop
corpus flattered it because a sliver covers a large fraction of a 240px crop. Decision B
(proposal layer) is **not supported as measured**; the fragments do cluster on real structure,
so a merge step is the specific next experiment. See [[Experiment-004-vision-corpus-v2]].

## 4a-old. Original framing

The measurement is rebuilt and proven; the evidence is not captured. See
[[Experiment-004-vision-corpus-v2]].

**Done.** A versioned ground-truth schema with negative regions, sequence identity and
game-agnostic vocabulary. Paired metrics — structural, nameable, temporal and OCR-region
precision *and* recall — where each partner closes the other's loophole. `ScoreV2`, which
returns *unavailable* rather than zero on a corpus that cannot support it. Corpus versioning,
with the legacy corpus marked and every report warning that it cannot calibrate anything
scale-relative.

The anti-correlation is fixed and the fix is held by a test: a detector that clings to
declared scenery for ten frames now scores **worse** than one that does not, where V1 scored
it better. Six mutations verify the metrics are load-bearing — four survived the first pass
and the tests were sharpened until none do.

**Also measured**: the grid-stride question left open by item 4. A finer stride is the worst
of three options (0 of 8 controls found, 3× the cost); sampling the same stride at two origins
finds all 8 at 2/3 that cost. Default unchanged pending v2 precision data.

**Outstanding, and it needs the user.** Capturing full-resolution game sequences is
mechanical; deciding that an image of somebody's screen may be committed is not a decision a
tool should make, and it is where every previous corpus attempt stalled. The workflow is
written up in [[Vision-Corpus-Workflow]] — capture, review, sanitize or exclude, annotate,
approve. One session of Rocket League with the sequences listed there would unblock items 2
and 4's successors together.

- affects: [[Vision]], `internal/director/visionbench`
- blocks: rebenchmarking Classical, the shipping detector and Grounding DINO on sound evidence

The blocker for every remaining vision question, and the thing three experiments have now
tripped over.

Needed: **full-resolution frames** (not crops) in **ordered sequences** (not unrelated
stills), with per-frame declared ground truth of the kind the current manifest already
carries. Until then:

- the score is anti-correlated with precision — temporal metrics are 40 of 100 points and the
  corpus cannot support them, so emitting more junk scores better;
- normalised thresholds cannot be tuned, because a crop's "fraction of frame" is not a
  screen's;
- no learned challenger can be ranked either, for exactly the same reasons — this is not a
  classical-CV problem.

Scope it small: one game, one session, a handful of sequences (gameplay, pause menu, camera
motion, overlay appearing/disappearing), privacy-reviewed. The declared-ground-truth
precision measurement built in this milestone
(`TestClassicalGroundTruthPrecision`) already works and needs only better frames.

- affects: [[Vision]], `internal/director/visionbench`
- blocks: items 2 and 4's successors

## 4b. Superseded framing, kept for the record

> It won at 61.9, runs in ~0ms, has no licence and no dependency, and already emits `button`.
> Its weaknesses are measured: 67% false structure. Edge-based refinement, connected
> components instead of a grid scan, and rejecting regions that fail to persist would all
> help.

Two of those three were tried. **Edge-based refinement** (border continuity) was measured and
refuted. **Rejecting regions that fail to persist** cannot be evaluated: persistence is the
metric the corpus cannot support. **Connected components** instead of the grid scan remains
untried and is the one suggestion still standing — it would fix the 8px quantisation that
loses a 16px control at a non-grid offset, and it would introduce the nested rectangles that
suppression was written for and that the grid scan cannot produce. Worth doing *after* the
corpus, not before.

## 5. ~~Wire safe bindings into the execution path~~ — ALREADY WIRED, claim was stale

Audited 2026-08-06. `internal/director/binding` **is production-active**, and the entry that
said otherwise had been wrong for some time.

Proven live, not by reading imports: `director execute --dry-run "rename this to Budget"`
against the running service returned *"BLOCKED: "this" needs a file, and what is focused is a
control ("DNFC"). Refusing rather than acting on the wrong object."* — a string that exists at
exactly one place in the repository, `internal/director/binding/resolve.go:127`, inside
`Resolver.Resolve`.

The production call graph:

```
service/server.go → runtime.Handle → Pipeline.Handle / HandleProgram
  → ensureBindings(ctx)                          installs the store (5 sites)
  → expandGoal → newBinder → binding.Resolver.Resolve      bind
  → revalidateBinding → binding.Resolver.Revalidate        re-resolve, per step, pre-policy
  → handleRepeat → Replay → replayBinding → Revalidate     repeat
  → b.Snapshot() → actiongraph node                        persist
```

Three capabilities, three production call sites, all reached. Nothing bypasses it: an ordinary
non-goal action takes the `target.Resolver` path deliberately, because it names a control
rather than an object and has nothing to re-resolve.

**The wiring was already protected**, better than expected — disconnecting the revalidation
call site fails seven tests. What was missing was narrower: every binding test supplied the
store itself (`ctx := ensureBindings(...)`), so nothing proved production installs one.
`binding_wiring_test.go` now enters through the production entry with a bare context. See
[[Wiring-Tests]].

- correction: the [[Goals]] known gap saying otherwise is removed.

## 6. ~~The live navigation producer~~ — DONE 2026-08-09

The discovery loop is real end to end: physical input → closed-vocabulary `NavIntent` →
session-local evidence → transition correlation → discovery graph, with no raw key identity
surviving the platform adapter. Built and falsified with deterministic tests, replay and a fake
platform source rather than with another live session. Full record: [[director-navigation]],
constraint: [[ADR-013-navigation-is-meaning-not-keys]], subsystem: [[Navigation]].

Two defects found, both the same kind — a complete mechanism nothing invoked. The composition
root never opened a subscription (deleting the call failed **no** test in the repository), and
the hook's drop counter was wired to nothing, making backpressure unobservable in a design whose
justification for dropping is that it is safer than blocking. Both now carry mutation-gated
tests; five mutations were run and each killed its test.

Also landed: edge-local **order** (`down, down, confirm` is no longer flattened into a bag),
traces that carry navigation on every slot including skipped ones — so production and replay
agree on attribution and not only on geometry — and a producer diagnostic block that separates
"nobody pressed anything" from "nothing was listening".

- affects: [[Navigation]], [[Passive-Observation]]
- not built, deliberately: controller (no XInput infrastructure to reuse), pointer (no mouse hook
  in `director.exe`), text entry (a separate privilege on a separate channel)
- outstanding: one optional 60-second live sanity check, confirming a finished mechanism

## 7. ~~Semantic hypothesis generation~~ — DONE 2026-08-09

The substrate is complete and bounded: from-state, to-state, observed count, per-intent support,
dominant intent with its support, unattributed count, and ordered runs. The next milestone turns
repeated state-transition + input evidence into **named hypotheses** —

```
possible_menu_entry_transition
  supported because pause preceded state_1→state_3 in 3/3 observations
possible_back_navigation
  supported because back preceded state_3→state_1 in 2/2 observations
```

— with contradictions and unattributed evidence remaining visible, because an edge with a 3-to-2
split between two intents is a different finding from a unanimous one and a hypothesis layer that
renders them identically is worse than none.

**Hypotheses only. No saved capabilities.** A hypothesis is a claim about what was observed; a
capability is a claim that Marco can do something, and the gap between them is
[[ADR-004-vision-cannot-establish-actionability]]'s territory.

- affects: [[Hypotheses]], [[Passive-Observation]]
- record: [[director-hypotheses]], constraint: [[ADR-014-hypotheses-are-evidence-not-identity]]

## 8. ~~State-conditional navigation admission~~ — DONE 2026-08-09

**This item replaced "the proposal loop", which had been recorded as NEXT before the first live
run and was superseded by its measurement.** [[Experiment-008-unknown-game-discovery]] found the
proposal loop unreachable for a stated reason — *Marco can see the screen and cannot see how the
player reached it* — and [[ADR-013-navigation-is-meaning-not-keys]] already named the fix. The
roadmap was simply not updated at the time; recorded here because a resumed session read the
stale entry and had to resolve the two canonical documents against each other.

W/A/S/D and Space are now read as navigation **while, and only while, the last observation showed
a set of choices on screen**, and every such intent is marked as the weaker evidence it is. With
no context the behaviour is unchanged.

The predicate deliberately does **not** measure spacing. Rocket League's menu column scores
uniformity 0.97 and every recurring group in Schedule I scores 0.00–0.01 — both real interfaces
full of real choices — so a spacing rule would have admitted navigation in one game and refused
it in the other, and would have looked like a screen-shape rule until the third game.

**The gain is bounded, not measured, and cannot be measured from any existing trace.** Admission
is decided in the producer at the moment of the press, and a trace records the outcome rather
than the keystroke by construction. What the Schedule I trace does say: **17 of 52 valid
inferences (33%) showed a set of choices**, against 117 ambiguous refusals in that session. A
third of the session was eligible; whether the intents that would now be admitted actually
correlate with screen changes is the open question.

Eight mutations verified, including deleting the production `SetScreenContext` call.

- affects: [[Navigation]], [[Hypotheses]], [[Passive-Observation]]
- constraint: [[ADR-013-navigation-is-meaning-not-keys]] (amended)

## 9. ~~The proposal loop~~ — DONE 2026-08-09

Marco now puts one short, hedged, generic question to the user about a `supported` hypothesis and
records the answer as evidence. Full record: [[director-proposals]], constraint:
[[ADR-015-a-question-is-evidence-not-settlement]].

Three answers kept distinct all the way to the command line: **confirmed** adds support with
`FromUser` provenance and promotes to `validated` only when there are no contradictions;
**contradicted** adds a contradiction and yields `contested` while the supporting observations
remain listed; **declined** touches no semantic evidence at all and only suppresses the question.

Question identity is structural — kind plus role composition and member count — and excludes the
interface terms. That split was a defect first: with terms in the identity, a screen that started
showing one more word became a NEW question, so a declined question came straight back the moment
OCR read another label. A decline is now lifted by new KINDS of evidence, never by more of the
same and never by time passing.

Four mandatory mutations verified, plus three more. Two initially survived and both were real
gaps in the tests rather than false alarms — see the milestone record.

Still no capability, no execution authority and no generated Marco. The output is a validated
semantic hypothesis, and that is all.

- affects: [[Hypotheses]], [[Passive-Observation]]
- constraint: [[ADR-015-a-question-is-evidence-not-settlement]]

## 10. ~~Cross-session semantic memory~~ — DONE 2026-08-09

A confirmed subject is recognised in a later session and the question is not repeated; a
contradiction is equally durable; a decline suppresses until the evidence changes shape. Record:
[[director-semantic-memory]], constraint:
[[ADR-016-cross-session-identity-is-structural-and-conservative]], subsystem:
[[Semantic-Memory]].

`Subject.Fingerprint` was **not** adopted as durable identity. Four measured reasons: `Recurrence`
grows every episode; role counts break on a single missed detection; only `possible_choice_group`
carries an envelope, so most subjects have no geometry at all; and role composition alone
collides across unrelated screens. It stays the evidence source, and equality is replaced by a
tolerant comparison with a discrete verdict.

**A discriminator is required before Marco will say `same`** — matching interface terms, or a
matching envelope. Structure alone is `candidate` and inherits nothing, because "five buttons"
describes a settings screen, a level select, a save-file list and a confirmation dialog. Several
matches means `insufficient`, never the closest.

Seven mandatory mutations verified. One settled assumption was disproved on the way: ADR-015's
evidence digest included the support-source set, which GROWS within a session, so every declined
question returned on every restart. Corrected to kind + structural identity + terms.

- affects: [[Semantic-Memory]], [[Hypotheses]]
- honest limit: with no readable text and no envelope, nothing is recognised — and OCR is
  unavailable on the development machine, so this would not currently fire live

## 11. ~~Make OCR available, and measure whether recognition fires~~ — DONE 2026-08-09

**`OCR_DISCRIMINATOR_INSUFFICIENT`.** Tesseract was already installed and merely undiscoverable;
no installation was needed. Full record: [[Experiment-009-ocr-as-a-semantic-discriminator]].

The A/B that settles it: the same application, OCR on and off, produced the **identical** term set
(`back, notifications, search`). An application exposing roles without accessible names produced
**no terms at all** even with OCR running and accepting text. Every term Marco has ever classified
came from accessibility.

Two defects were found by auditing the path rather than by any failing test, and both would have
made the measurement meaningless:

- **Terms could never qualify.** Scoped OCR runs on ~1 inference in 6; the term ratio divided by
  every inference against a 0.50 threshold, capping a perfectly stable term at ≈0.17. The
  discriminator was unreachable in production.
- **Unavailable was indistinguishable from empty**, so a session that could not read text made the
  matcher report that a remembered screen DIFFERS.

Both fixed. The matcher was **not** loosened.

- affects: [[Semantic-Memory]], [[Hypotheses]], [[Vision]]
- cost measured: OCR 760ms/pass (731ms recognise) against ScreenParser's 625ms median; running
  both at a 2s cadence produced 11 late samples of 9 taken, against 0 of 24 without OCR

## 12. ~~Give the vision detector's roles a nameable path~~ — DONE 2026-08-09

It can. On a fixture where accessibility contributes nothing, the detector's own `button` regions
plus scoped OCR produce `audio, back, controls, settings` where the same fixture through the same
production path produced none — with the structure count unchanged, which is the invariant.

The measurement that matters: two settings screens with identical composition are `candidate` on
structure alone and **`different`** with the terms the detector's boxes carried. That is the
false-merge case [[ADR-016-cross-session-identity-is-structural-and-conservative]] has never had
a way to resolve.

Three links were missing and one had drifted — the shadow provider had no reader, the shadow
representation had nowhere to put a word, semantic classification read only the fused
authoritative side, and nameability existed as four separate allowlists. Now one policy in
`directorapi`, gating what is READ as well as what is kept.

Cost: 129ms per scoped region (measured on a real captured panel; the whole-panel read returned
**zero** spans, reproducing the finding that made scoped reading necessary). A label pass is
~1.0s of reading on top of a ~0.9s inference, so it will occasionally cost one skipped shadow
slot — counted, not silent.

`VISION_SEMANTIC_PATH_PROVEN`. See
[[Experiment-010-vision-structure-as-a-semantic-path]] and
[[ADR-017-structure-earns-a-name-text-never-earns-structure]].

- affects: [[Vision]], [[Semantic-Memory]], [[Perception]]

## 13. ~~Learn what CONNECTS two remembered semantic states~~ — DONE 2026-08-09

Marco can now recognise a screen across sessions on a surface with no accessibility, and say
what it is about in generic terms. What it cannot say is anything about the relationship
*between* two such screens: that this one is reached from that one, that going back returns, that
they are alternatives within one menu.

The evidence for it already exists and is already correlated — screen transitions and
closed-vocabulary navigation intents are recorded per state, and the state-conditional admission
work made the intents trustworthy. What has never been formed is a durable claim about a PAIR of
subjects.

Deliberately still not procedure learning: the claim to form is "these two remembered states are
related, and this kind of navigation was observed at the boundary", not "here is how to get
there". Execution authority is a separate decision and stays where it is.

`observed_transition` is the durable object: subject A was observed becoming subject B, with
per-intent support, unattributed and context-admitted counts, bounded ordered runs, and session
corroboration held apart from the observation count. Both endpoints must reach `same`; anything
less stays session-local and is reported as such. Written once per session, beside the subjects,
with referential integrity enforced at the write and at the load.

Adjacency, never a route — the explanation prints `causal claim: none` on every edge. Eight
mutations verified; one masked assertion found and fixed (a raw key was being refused by the
CAPACITY bound rather than by admission). A real defect surfaced by the audit: two hypotheses
about one screen produced two different fingerprints, so `Remember` stored one screen as two
subjects. Identity is now one function.

See [[ADR-018-a-remembered-relationship-is-adjacency-not-a-route]].

- affects: [[Semantic-Memory]], [[Navigation]], [[Hypotheses]]

## 14. ~~Turn a corroborated relationship into a QUESTION~~ — DONE 2026-08-09

Marco now holds a map and cannot say anything about it to the person who walked it. The next
layer is the one that decides when an edge has earned a sentence: enough observations, enough
independent sessions, navigation evidence that is not entirely unattributed and not entirely
context-admitted — and then puts it through the proposal loop that already exists.

The question is still not "shall I learn how to do that". It is "I have seen you move between
these two screens before — is that a thing you do deliberately?", and the answer is evidence
about the EDGE in the same way an answer about a subject is evidence about the subject.

Marco now asks it. `Proposal.Ask` types the two questions apart so the two `no`s keep their
opposite meanings — a semantic `no` contradicts an interpretation, a learning `no` is a
preference that leaves every observation intact. Eligibility is discrete (≥2 independent
sessions, ≥3 observations, a dominant intent over at least half of them, not mostly
unattributed, not only context-admitted, no scatter of one-off runs, endpoints not rejected)
and every refusal is a closed reason reported per edge. A yes buys a pending `LearningRequest`
on the relationship and nothing executable. Reviewed once at session end, so semantic questions
get first claim on the single interruption slot.

Nine mutations verified. One wording defect found on the way: two different screens sharing a
confirmed interpretation read as "from the settings screen to the settings screen".

See [[ADR-019-an-invitation-to-learn-is-not-a-correction]].

- affects: [[Hypotheses]], [[Semantic-Memory]]

## 15. ~~Bounded demonstration capture for an approved LearningIntent~~ — DONE 2026-08-09

A pending request is a person saying "yes, watch me do it". Nothing yet watches.

The next layer is the deliberate, consented, bounded capture of ONE clean example: start at the
remembered subject the request names, observe the navigation, arrive at the other one, and keep
a procedure CANDIDATE. It has to be explicit rather than automatic precisely because it may need
to observe more than passive discovery does — which is why this milestone stopped at the request
rather than starting a recorder on a yes.

Marco now watches. `internal/recorder` was deliberately NOT reused — it captures raw keys and
feeds a type the executor runs, and a yes to one question about one transition is neither of
those permissions. What is recorded is the closed `NavIntent` vocabulary passive observation
already uses.

Bounded (60 events, 8 checkpoints, 90 observations, 2 restarts), authorised structurally
(`NewCapture` needs a relationship; `armCapture` reads `PendingLearning`), started and ended by
CURRENT evidence, and producing a `ProcedureCandidate` with no `Execute` and `Verified` always
false. Eight mutations verified.

See [[ADR-020-watch-me-is-permission-to-observe-not-to-act]].

- affects: [[Demonstrations]], [[Semantic-Memory]], [[Navigation]]

## 16. ~~Decide whether a candidate is a reproducible procedure~~ — DONE 2026-08-09

Marco has one watched example and no idea whether it generalises. The question is whether that
can be settled WITHOUT executing uncontrolled input: a second demonstration to compare against,
step-by-step consistency, checkpoints that could be verified during a future attempt, and only
then an explicitly rehearsed, explicitly approved execution.

The honest first deliverable is a judgement with named reasons — `single_example`,
`steps_disagree`, `requires_text_entry`, `transient_checkpoint_unverifiable` — in the same shape
Delivered as a judgement, and deliberately NOT as a stored verdict: an assessment is recomputed
from the candidate plus the current topology, so a demonstration whose middle screen the user
later names becomes more verifiable with no new observation. Four verdicts, eleven closed
reasons, each saying whether another example would settle it. Comparison machinery for a second
demonstration is built and proven synthetically, so nobody had to perform one.

Eight mutations verified; one was masked by an overlapping branch and re-run in its faithful form.

See [[ADR-021-a-judgement-is-recomputed-not-recorded]].

- affects: [[Demonstrations]], [[Semantic-Memory]]

## 17. ~~Ask for the second demonstration, and only where it would help~~ — DONE 2026-08-10

The assessment already says which gaps another example could close and which it could not. What
is missing is the loop that acts on it: a proposal that asks for a second demonstration ONLY when
`ResolvableByDemonstration` is true, captures it through the machinery that already exists, and
compares the two with `CompareCandidates`.

Two demonstrations that agree are the first evidence in this whole system that a procedure is
reproducible rather than merely observed. That is still not permission to perform it — rehearsal
and execution authority remain separate decisions — but it is the point at which "verified" stops
being structurally unreachable.

Asking for a demonstration Marco already knows would not help is the failure to avoid: the user
Marco now asks, and much more often decides not to. Eligibility comes from the assessment; one
non-resolvable blocker stops the question; the wording carries the named gap. A third `no`
meaning was typed apart (`provide_second_demonstration`). Lineage is immutable and bounded at two
demonstrations. Two agreeing examples corroborate; two that differ are recorded as disagreeing
and never averaged. Ten mutations verified.

One real defect fixed on the way: a fulfilled learning request stayed `pending` and would have
armed a capture in every later session, forever, without asking again.

See [[ADR-022-ask-only-when-you-can-say-what-it-would-resolve]].

- affects: [[Demonstrations]], [[Semantic-Memory]], [[Hypotheses]]

## 18. ~~Language reconciliation: act, scene, actor, verb~~ — DONE 2026-08-10

The one question the previous reconciliation left open is closed by product intent: an act is the
way in, a scene is where things happen, an actor is a thing in the play, a verb is what one of
them does. They share `KindActor` on purpose — at run time they behave identically — and the
authored word now survives into the graph as `Node.Declared`.

The distinction the compiler can genuinely hold: **only an act exports.** That used to compile on
an actor, which meant `act` carried no obligation a reader could rely on. Six mutations; five
killed, one reported unkilled (see HANDOFF).

`scene` was NOT retired. It earns its keep, and `testdata/act_scene` now demonstrates the
distinction instead of the collapse.

**Language work is closed** unless a concrete Director requirement proves Core v1 insufficient.

- affects: [[Marco-Boundary]]

## 19. ~~Design the rehearsal boundary~~ — DONE 2026-08-10 (design only)

The strongest state this system can now reach is one consistent candidate corroborated by a
second agreeing one, with no authority at all. The next question is the first one that touches
acting: what evidence would justify Marco attempting a transition ITSELF, and under what
containment?

Delivered as [[ADR-023-rehearsal-is-attempt-scoped-authority]]. Five states, one rule
("every action Marco proposes has an expectation it can check afterwards"), one-step-at-a-time
with settle-and-verify between, ten fail-closed aborts, a separate `RehearsalResult` rather than
a mutated demonstration, and one successful rehearsal as the smallest defensible bar.

Text entry and naked pointer coordinates are rehearsal-INELIGIBLE. The within-screen counterexample
(`down` moves a selection Marco cannot see) is stated rather than silently weakened, and resolves
as `progress_unobservable`.

The seven "cannot execute" invariants turned out to be already true STRUCTURALLY — every learned
type lives in `observe`, whose transitive imports reach nothing that can touch a desktop. Now
named in a test so moving one somewhere convenient fails loudly.

Core v1 can express everything a verified procedure does. The two gaps are NAMES (what to call
the actor and the verb), which is a question for the user, not a language change.

- affects: [[Demonstrations]], [[Semantic-Actions]], [[Programs]]

## 20. ~~Rehearsal eligibility and the authorization request~~ — DONE 2026-08-10

`RehearsalJudgement` over a `CandidateAssessment`, the typed `AskRehearse` proposal, and one
ephemeral attempt-scoped `RehearsalGrant` — `internal/director/observe/rehearsal.go`, wired at
`observesession.Runner.reviewRehearsal` (session end) and `Runner.authorizeRehearsal` (the yes).

**Zero desktop input.** The strongest state the whole system can now reach is an inert grant.

Three things that are not each other, and the whole milestone is keeping them apart:

| | |
|---|---|
| `CandidateAssessment` | what does the demonstration evidence support? |
| `RehearsalJudgement` | is that evidence enough to *ask* for one controlled experiment? |
| `RehearsalGrant` | the user said yes to one attempt |

Evidence is not authority; eligibility is not authority; a question is not authority; and a
previous yes to *learn this* is not authority — that one authorised watching. `AskRehearse` is
typed apart from every other question precisely so the two permissions cannot be reached from one
another, and `TestOnlyARehearsalYesCreatesAuthority` is what holds it.

**The per-step rule that decides eligibility:** every action Marco would take must have an
observable expectation it can check afterwards. Three verdicts, because two would force a lie —
`directly_verifiable`, `progress_unobservable`, `unverifiable`. The middle one is `down` in a
menu: the screen must stay the screen it started on, Marco cannot see what moved, and calling
that success is the collapse the vocabulary exists to prevent. Bounded at
`MaxUnobservableRun = 2`, and the last step must always settle the procedure.

Fifteen designed refusals in a closed vocabulary, no confidence float anywhere.

**The grant.** Scoped to one application, one starting screen, one destination, `MaxInputs`,
`MaxUnobservable`, `MaxDuration`; single-use, consumed when the attempt *begins* rather than when
it succeeds; scope re-checked at claim time, not trusted from issue time; revoked by cancellation.
It has no method that performs anything, it lives in the analysis core, and it is never written to
disk — `TestNoAuthoritySurvivesARestart`.

**Found while building it:** two empty application names compare equal, so a candidate belonging
to no application passed a check meant to confine it to one. Now refused outright.

- blocked by: nothing
- affects: [[Demonstrations]], [[Hypotheses]]

## 21. ~~One rehearsal step, dry~~ — DONE 2026-08-10

One authorized step lowered to the last thing before a computer, which is a notebook.

`internal/director/rehearse` (the attempt), `internal/platform/recordhost` (the notebook),
`cmd/director/rehearsedry.go` (the composition root, and the only place that chooses a host).

**The audit found the boundary already built.** [[ADR-005-legal-marco-only]] settled it:
`marcoexec.Operation` → legal Marco → lexer → parser → graph → compile → runtime → `runtime.Host`.
So there is no dry stack. The dry path IS the real path with `oshost.Host` swapped for
`recordhost.Host` — same lowering, same encoder, same compiler, same runtime, same scheduler.
See [[ADR-024-a-dry-step-is-not-evidence]].

**A meaning, never a key.** The Director learned that somebody *confirmed*; it did not learn
Enter, and lowering to a key chord would have put a device binding inside the Director. `os.marco`
gained `Navigate`, `marcoexec` gained `KindNavigate`, and the intent→key table lives backstage in
`internal/oshost/navigate.go` — the acting half of
[[ADR-013-navigation-is-meaning-not-keys]]. Not the observation table reversed: watching admits
W/A/S/D as *conditional* evidence, acting has exactly one key to press.

**Atomic, and the claim comes first.** The grant is spent before anything can be produced, and it
stays spent if setup then fails — otherwise "one attempt" becomes "as many as it takes to get past
the setup". A step whose whole ordered run does not fit its budget is refused before its first
input rather than truncated: half of `down, down, confirm` is a different procedure, ending
somewhere no demonstration ever went. Twelve prechecks, all before the first effect.

**And it claims nothing.** Two words — `would_emit` and `refused`. A dry step is an engineering
artefact: it verifies nothing, counts towards nothing, and no part of the learning loop reads it.

**Found while building it:** answering a question on a FINISHED session recorded the answer in the
ledger and never reached memory. Every question this system asks is asked at session end, so
`g.active` is always nil by the time anybody can answer one — meaning a user could say "yes, learn
that" every session forever and Marco would never once arm a capture. Fixed in
`observationRegistry.Answer`, and it was the wiring test that found it.

- blocked by: 20
- affects: [[Demonstrations]], [[Marco-Boundary]], [[Semantic-Actions]]

## 22. ~~One rehearsal step, live~~ — DONE 2026-08-10

The first milestone in which something Marco learned by watching may move a real computer.

`internal/director/rehearse/live.go` (the attempt), `cmd/director/rehearserun.go` (the composition
root), `director rehearse [--live]` (the trigger). See [[ADR-025-one-move-then-look]].

**Nothing above the host changed.** Same `Attempt`, same claim, same `LowerStep`, same
`KindNavigate`, same legal Marco, same compiler and runtime — [[ADR-005-legal-marco-only]] holds.
What this milestone adds is everything that must be true *around* one real input:

```
look → compare → CLAIM → re-check the window → emit → settle → look again → classify → stop
```

**The starting screen comes from perception, never from an argument.** Roadmap 21 accepted it as
scope input, which was right for a milestone that could not act. Now it is established by watching
and resolved through `SignatureOfState` → `Recall` — the same path a demonstration uses. Wrong
screen, unrecognised screen or ambiguous screen all send nothing.

**The final guard is as late as it can be**, closing the race that matters most: verify the
screen, the user alt-tabs, the keystroke lands in their email. Window identity, process and
generation — never the title.

**A refusal is not a result.** `RehearsalResult` exists only when input was emitted; the line is
the moment the program reaches the runner. Seven outcomes, and `progress_unobservable` (a property
of the step) is kept apart from `unobservable` (Marco failing to look).

**Acting is an explicit decision, twice.** `rehearse.Live` cannot emit until the composition root
hands it an actuator, and `--live` chooses the real host. No session spends a grant; a later
session withdraws an unspent one.

**And a second, caught by reading the output:** a dry run classified the post-input screen, which
with a recording host means reporting `wrong_state` — "the step failed" — when nothing had been
sent. A dry run now concludes nothing.

**Found while building it:** the live path compared the grant's digest against itself, so the
"has the evidence moved" check could not fail. The wiring test caught it.

18 mutations, 17 killed; the survivor is equivalent (the early source check duplicates the grant's
own claim-time comparison, same refusal, grant unspent either way).

**Not yet done deliberately:** no result is durable, no procedure is verified, no Marco is
generated, nothing is promoted. And no real actuation has been performed — the capability is
implemented and proved deterministically, and the first live run is the user's to choose.

- blocked by: 21
- affects: [[Demonstrations]], [[Perception]], [[Marco-Boundary]]

## 23. ~~Multi-step rehearsal state machine~~ — DONE 2026-08-10

Marco proposes one step. Reality answers. Only then may Marco propose the next — and a rehearsal
succeeds only when the WHOLE learned route survives that conversation.
See [[ADR-026-verification-is-derived-from-a-completed-rehearsal]].

**The one-step seam was generalised, not replaced.** `Attempt.LowerStep` was the terminal point;
now `open --lower--> acted --observe--> open` is the only loop in the transition graph, and
`LowerStep` may run only from `open`. **ACT → ACT is impossible at the type**, so an orchestrator
with a bug is refused (`awaiting_observation`) rather than typing twice. No second lowering path,
no second actuator, no sequence executor beside the proven primitive.

**The authorization needed no widening.** `AskRehearse` already asks *"one step at a time, and stop
the moment the screen isn't what I expected"* — a bounded multi-step attempt in the user's own
words. The attempt owns the authorization; a step owns none. Bounds are the plan's own: inputs from
the judgement, steps from the plan, unobservable run from its longest contained stretch.

**Containment comes from the candidate, never from the result.** A step the candidate said was
directly verifiable, whose screen then did not change, is `wrong_state` — inferring
`progress_unobservable` afterwards would let every step that did nothing report itself as safely
contained. And a route cannot succeed on containment: the final step must be directly verified.

**Verification became derived.** `ProcedureCandidate.Verified` stays false and is now deliberately
vestigial. A completed attempt stores `observe.RehearsalEvidence` — bounded, semantic, nothing
executable — and `CandidateAssessment.WithRehearsal` recomputes verification from it plus what
memory holds now: the route completed, the digest still matches, both endpoints are still
recognised. A stored boolean would go on saying yes after the demonstration was revised.

**Failure stays evidence.** A host that could not send is never read as the procedure being wrong;
a wrong destination is never read as the host failing. Nothing invalidates a candidate, and every
failure needs a fresh authorization to try again.

18 mutations, 18 killed.

**Still not done, deliberately:** no learned Marco, no actor or verb naming, no capability, no
retries, no automatic rehearsal. And no live run has been performed.

- blocked by: 22
- affects: [[Demonstrations]], [[Semantic-Memory]], [[Perception]], [[Marco-Boundary]]

## 24. ~~Lower a verified procedure into readable Core v1 Marco~~ — DONE 2026-08-10

*Director watched and verified the behaviour. What it learned is now ordinary readable Marco.*

`internal/director/observe/lowering.go` (may it be written down?),
`internal/director/marcoexec/play.go` (writing it),
`cmd/director/learnedplay.go` + `director learned` (the surface).
See [[ADR-027-what-marco-learned-becomes-marco]].

**The gate is the derived judgement**, not `ProcedureCandidate.Verified` — which is always false
and deliberately vestigial. A route lowers only while a completed rehearsal still matches this
demonstration and memory still knows both endpoints, re-derived on every read. Eleven discrete
refusals, no lowering score.

**The lowering input is meanings and nothing else.** `[][]NavIntent`. No subject ids, screen
states, handles, digests, verdicts, counts or coordinates — never handed to the generator rather
than filtered at it, because a field that exists eventually gets printed.

**The compiler is the authority**, against the canonical `os.marco` rather than a convenient stub.
A meaning Core has no sentence for is a language-expression gap: reported, and that lowering stops.

**Names are provisional and say so** — `UnnamedShortcut`, `Run`. Guessing a meaningful name from a
screen's text would leak OCR into the language; guessing from the application would name the play
after the wrong thing. `naming_required` was not needed, because an inert artifact nothing can ask
for by name is safe to give a placeholder.

**Inert means inert.** It writes no file, registers nothing, joins no resolution path, and
`marcoexec` cannot reach a runtime, host, platform adapter, grant or execution path. The test that
matters snapshots the working tree and requires it byte-unchanged.

12 mutations, 11 killed; the survivor is equivalent and now documented at the guard itself.

**A gap this milestone found and did not paper over:** the repository has no route-metadata sidecar
mechanism, so provenance is carried in the response beside the source rather than inside it. That
becomes load-bearing the moment a play is persisted.

- blocked by: 23
- affects: [[Marco-Boundary]], [[Programs]], [[Demonstrations]]

## 25. ~~Learned play lifecycle — provenance, naming, persistence, registration, resolution~~ — DONE 2026-08-10

*Director can turn verified learning into a named, auditable, durable Marco play that a fresh Marco
process can find again — without that lifecycle granting itself permission to perform it.*

`internal/routes/origin.go` (provenance, staging, registration),
`internal/director/marcoexec/play.go` (naming regenerates),
`cmd/director/learnedplay.go` + `director learned --name --verb --save --register --forget`.
See [[ADR-028-a-learned-play-is-a-file-with-a-past]].

**The prerequisite, re-verified rather than assumed.** There is no metadata sidecar — but there IS
a companion-file convention: a taught route's `<slug>.rec.json`, already carried by `Delete` and
`Rename`. The shape existed; the record did not. Provenance is now `<slug>.origin.json`.

**Saved and registered are different PLACES, not a flag.** Discovery scans `global/`, an app's loose
files, `context/` and `focus/` — nothing else. A saved play lives in `<app>/learned/`: on disk,
readable, editable, and structurally invisible to the resolver. Registering is moving it. No
registry was invented, because discovery already is one.

**The two-file problem fails safe.** Source first, provenance second, both atomic. A crash leaves a
`.marco` with no sidecar — an ordinary authored play, claiming nothing. The reverse would let a
later unrelated file inherit a past it never had, and that is refused separately too.

**The user owns the file.** Editing a learned play is allowed: it still resolves, still compiles.
What changes is the claim — the digest stops matching and the provenance reads `edited`.

**Naming regenerates**, never string-replaces, and asks for the play's own words: `do Volume's
Mute...` — the actor is what it is, the verb is what it does. Nobody names a candidate or a subject.

12 mutations, 12 killed once the harness itself was fixed — an aborted earlier run had left a
damaged file, and three "kills" were build failures. The clean re-run is the one that counts.

**Still false, deliberately:** Marco cannot execute a learned play automatically. Resolution answers
*which play you mean*, not *you may perform it*.

- blocked by: 24
- affects: [[Programs]], [[Marco-Boundary]], [[Semantic-Memory]]

## 26. ~~Invocation authority for a registered learned play~~ — DONE 2026-08-10

*A learned play reaches the same execution door as every other Marco play, but knowing which play
to use is never confused with permission to open that door.*

`internal/orchestrator/authority.go` (the door), `Deps.Resolve` / `Deps.Do` (the split).
See [[ADR-029-resolution-is-not-permission]].

**The audit found no door.** `Do` resolved a phrase and called `Run` in the same breath — one
function, no seam, nothing that could hold "I believe this is the play you mean" without also
performing it. Building that seam WAS the milestone.

**The seam is general.** Every route passes through it; the policy is what differs, not the road.
Authored and taught plays run as they always have. A learned play with intact provenance is asked
about once per invocation — not because it is dangerous, but because Marco composed it and the
user has never read it. An edited learned play is ordinary: refusing it would be refusing the
user's own writing.

**Authority is per invocation and is never written down.** No `Trusted: true` on the file. A
durable play is knowledge; permission to use it now is a different thing, and asking again next
time is correct rather than missing.

**One execution path.** After the decision it is `.marco` → lexer → parser → graph → compile →
runtime → Host. No Director executor, no candidate replay, no learned host, no fast path.

8 mutations, 8 killed.

## THE GAP THIS EXPOSED — read this before Roadmap 27

**A learned play does not encode its own starting state, and Core v1 cannot express one.**

Application context is expressible and is used: a learned play registers as a `context/` route,
which ordinary Marco already refuses to run unless that application is in front. SCREEN-level start
state is not. Core v1 has no sentence for *"only when this looks like the settings menu"*.

So a learned play invoked from the wrong screen inside the right application presses its keys
anyway. A confirmation does not fix that — a user saying yes does not make an invalid starting
state correct — and it was deliberately NOT hidden behind a Director graph consulted at execution
time. The promise is that what Director learned becomes Marco; a play whose safety depends on
something the play cannot say has not kept it.

- blocked by: 25
- affects: [[Programs]], [[Marco-Boundary]]

## 27. ~~Let a play say where it starts~~ — PARTIAL 2026-08-10

**Core v1 did not change. No syntax was added.** The language already said it.

`internal/screenmod/screen.marco` (one read-only act), `marcoexec.LowerPlayStartingOn` (the guard).
See [[ADR-030-a-play-says-where-it-begins]].

The mandatory investigation stopped at step one: `when?` / `or?` over a capability that answers
ok-or-failed is exactly the shape `OS's Focus` has always used. Contracts and anchors were never
reached for. What was missing was a CAPABILITY — nothing could answer *"is the screen the user
named the one in front?"* — and a capability is declared in an act, which is the route
[[Marco-Boundary]] already prescribes.

```marco
this's Mute does...
    do Screen's Showing with "the pause menu"...
        when ok?
            log "starting".
        or?
            this is failed with error "this play starts on the pause menu"!
    do OS's Navigate with "down".
    ...
    this is ok!
```

The compiler found the shape, not guesswork: wrapping the steps inside the `when ok?` arm was
refused — *"falls off the end without an explicit return"* — and the early-return form that
compiles is also the one a person would say out loud.

**Fail closed by construction.** Mismatch, ambiguity, unobservable, unavailable, or a host that
cannot answer are all *not ok*, and `oshost`'s unknown-action branch returns failed — so a Marco
with no recogniser refuses a guarded play rather than running it. Silence is never yes.

## Why PARTIAL, and what remains

**The capability is declared but not fulfilled.** No host answers `Screen's Showing`, so today
every guarded play refuses. Safe, deliberate, and not finished: a learned play is currently
unrunnable until Director exposes a read-only recogniser.

Two things remain, and they are one milestone:

1. **A read-only Director act host** answering `Screen's Showing` from semantic memory — no
   execution authority, durable identity, deterministic refusal when unavailable.
2. **Naming the screen.** `"the pause menu"` is what a USER calls it. Nobody has been asked, and
   Director must not guess it from OCR — the same rule that governs naming the play.

This also settles the Director-availability question plainly: **a learned play cannot validate its
own starting screen without Director.** Application context is ordinary Marco; screen recognition
is Director's.

- blocked by: 26
- affects: [[Programs]], [[Marco-Boundary]], [[Semantic-Memory]]

## 28. ~~Fulfil the screen act, and let the user name the screen~~ — PARTIAL 2026-08-10

**CORE_V1_CHANGED: NO.** No syntax was added.

`RememberedSubject.Called` + `NameSubject`/`SubjectNamed` (semantic memory),
`internal/platform/screenhost` (the read-only host), wired in `cmd/marco`.
See [[ADR-031-the-user-names-the-stage]].

**The correction that matters more than the feature.** Roadmap 27's guard shape *compiled and did
not guard*: a return inside an `or?` arm ends the arm, not the capability, so execution walked past
it and pressed the keys anyway. A test that read the source and asked the compiler shipped it. What
caught it was running the play against a Screen host that said no and watching the keys come out.
The shape that works nests the steps inside the `when ok?` arm. **Compiling is not behaving.**

**One durable user-supplied string**, and the privacy boundary is provenance rather than content:
`the pause menu` typed by a person is allowed; the same words read off a screen are not.

**Exact, application-scoped, never nearest-match.** Two subjects in one application may not share a
name; a duplicate that arrives some other way is ambiguous, and ambiguous is a refusal.

**The host looks and compares and can do nothing else** — three read methods, and it reaches no OS
host, driver, window activation, grant, execution pipeline or orchestrator.

**Standalone Marco fails closed.** It has memory but no recogniser, so every guarded play refuses.
Director figures out the play; Marco performs it; Director still provides the eyes while it does.

**The sidecar enforces nothing.** Delete the guard from the source and the play runs anywhere —
correct, and proved adversarially.

## Why PARTIAL

The naming QUESTION is not wired. `NameSubject` exists and is tested; nothing yet asks the user
*"What do you call this screen?"* through the proposal machinery, and the learned save flow does
not yet require a screen name before lowering. Until it does, a guarded play can only be produced
by a caller that already knows the name.

- blocked by: 27
- affects: [[Semantic-Memory]], [[Marco-Boundary]], [[Programs]]

## 29. ~~Ask the user what the screen is called~~ — PARTIAL 2026-08-10

**CORE_V1_CHANGED: NO.** `observe.ScreenName` with one constructor; `NameSubject` takes the type,
so observed text cannot become a screen name by assignment. `AskNameScreen` added. Lowering now
RESOLVES the name from durable memory for the real source subject and refuses `screen_unnamed`
without it — closing the Roadmap 28 loophole where a caller could pass any name. The production
path emits the corrected nested guard carrying that name.

**Not landed:** the naming QUESTION is not surfaced or answered through the ProposalLedger, and
the combined 28+29 mutation gate was not run. Both are recorded in HANDOFF rather than glossed.

- blocked by: 28
- affects: [[Semantic-Memory]], [[Programs]]

## 30. Surface the naming question, and pay the mutation debt — NEXT

Two things, and the second is overdue. Produce the screen-naming proposal from the learned-play
lifecycle through the existing ledger, bound to the durable subject rather than to whatever is
current when the answer arrives; route the answer through the real response path into
`NameSubject`. Then run the combined Roadmap 28+29 mutation gate with a runner that verifies each
mutation applied, failed for the intended reason, and restored byte-identically.

Until the gate runs, two milestones of safety claims rest on tests nobody has attacked.

The one remaining piece of Roadmap 28. Surface the naming question through the existing proposal
ledger — no second broker — bound to the durable subject rather than to whatever screen happens to
be current when the answer arrives; require a screen name for the source subject before a learned
play may be lowered; and refuse registration of a play whose entry condition names something memory
cannot resolve.

Then the Marco Moment is end to end from a user's words to a guarded, registered, restart-surviving
play.

- blocked by: 28
- affects: [[Semantic-Memory]], [[Programs]]

## 31. A path to teach something on purpose — NEXT after 30

Everything Marco can learn today, it learns because it **noticed** something and asked. There is
no way to say *"I am about to do a thing; pay attention."*

That is a gap rather than a design principle, and it was raised by the first person to try to use
the system rather than build it: somebody who intends to teach a command should be able to teach
it directly, which is what anybody writing a plugin would expect. `director demonstrate start`
looks like that door and is not — it records the outcomes of Director's OWN requests, not the
user's hands.

### Why the discovery path is gated the way it is

Not consent theatre. A learned play is `screen A → navigation → screen B`, and both endpoints
must be durable subjects that can be recognised on another day. In the passive path they already
are, because the relationship was seeded by repeated observation before anybody was asked
anything. A "learn this" that skipped that would hand Marco a route between two things it cannot
name or recognise tomorrow — which is the `start_unverifiable` failure of
[[ADR-041-a-screen-is-not-its-dominant-group]], reached from the other direction.

So the constraint is real, and it constrains the SHAPE of direct teaching rather than forbidding
it.

### What it would actually be

The same pipeline, with the invitation step replaced by an assertion. Everything downstream —
capture, assessment, second demonstration, rehearsal, naming, lowering, registration, the Roadmap
26 authority door — is already built and needs no change:

```
teach "open downloads"
  → establish the START: observe where you are until it is a durable subject
  → record the navigation, under the ordinary capture bounds
  → establish the DESTINATION the same way
  → hand the pair to the existing assessment
```

The only genuinely new work is the first and third steps: standing still until a place is
recognisable. `StatePromotionCount` needs two sightings and `Remember` refuses a subject with no
discriminator, so the flow has a natural shape — *"hold still a moment"*, then *"go on"*, then
*"hold still again"* — and it should say so rather than failing silently.

### The hard limit to state up front

**A screen with no readable terms and no envelope cannot become a durable subject at all**, so
there are places a direct teach must honestly refuse. That is `Discriminating()` and it is not
negotiable by this feature. A teach flow that quietly produced a play whose endpoints could never
be matched again would be worse than no teach flow.

Two smaller ones, both already true and both worth surfacing in the flow rather than at the end:

- **Typing is structurally invisible** and a demonstration that crosses a screen you typed on is
  refused with `requires_text_entry`. Text-entry learning is its own privilege with its own
  lifecycle; see the note at the foot of `navsource.go`.
- **A click cannot be written down.** [[ADR-042-a-click-is-a-place-in-a-window]] made pointer
  presses observable, which fixed attribution; both consumers still refuse them, because a
  position is not a meaning. Until a press can be resolved to the control underneath it, a
  teachable journey is a keyboard journey.

### The audit finding that decides the design — 2026-08-11

`Capture` is a **confirmation** mechanism, not a discovery one, and the whole shape of Teach
follows from that:

```go
beginAt:   if in.Subject != c.Relationship.From  → keep waiting
settleAt:  case next.Subject == c.Relationship.To → ARRIVED
```

It is armed only from `PendingLearning(top)` / `PendingFollowUp(top)` — relationships **already in
durable memory** with a pending request. It says *"you go from A to B; show me"*. Teach needs
*"show me, and I'll see where you end up"*, and B is unknown when the capture would arm.

Two ways out, and the second is much better:

1. **Give `Capture` an open destination** — `Relationship.To == ""` meaning "whatever you reach
   and settle on". Small, but it IS new semantics in the demonstration model, and the model is
   load-bearing for assessment, rehearsal and the wrong-destination guard.
2. **Let the first pass be ordinary observation.** Teach establishes A, says *"go ahead"*, and
   watches with the ordinary passive machinery. When a transition A→B is observed, B is known —
   so the existing confirm-shaped capture can be armed for the second pass: *"show me once
   more"*. **No new semantics at all.**

Option 2 is the design. It costs one extra demonstration, which existing policy wants anyway
(`single_demonstration_only` is the standing ceiling on one example), and it makes the
conversation the milestone sketched fall out naturally rather than being imposed:

```
establish A          →  ordinary observation
"go ahead"           →  passive discovery of the route
transition observed  →  now A→B is known
"once more"          →  arm the EXISTING capture for A→B
capture completes    →  the ordinary assessment, unchanged
```

The one seam still needed is an **explicit arm path**: today a capture arms only from a durable
pending request, and Teach must arm one for a relationship it just discovered. That is a new
caller, not a new rule — and it is where the mutation gate should be aimed.

### Built 2026-08-11 — and the seam turned out not to be one

`internal/director/teach` + `director teach "<name>" --window-id <id>`. See
[[ADR-043-teaching-is-two-passes-not-a-new-capture]].

The "explicit arm path" was never needed. `RememberLearning(..., LearningPending)` is *already*
what a user's yes writes, and `teach "..."` is that yes — so Teach arms the existing capture
through the existing door, and the store's refusal of a route it does not hold enforces four rows
of the refusal matrix for free. Eleven mutations applied, eleven caught.

**PARTIAL.** The coordinator drives establish → discover → arm → demonstrate → assess. Beyond
that it *stops* at `ready_to_rehearse` and `naming` and prints the command that already accepts
the answer, because both need something only a person can give. Driving the tail — rehearse,
name, lower, save under the requested name — is the one next task.

## 32. ~~Finish the Teach tail~~ — COMPLETE 2026-08-11

`ready_to_rehearse` → the existing rehearsal question → an explicit yes → the existing rehearsal
state machine → the recomputed lowering judgement → the existing `AskNameScreen` per unnamed
endpoint → the one persistence path → `I learned "open downloads"`.

The coordinator gained a second seam, `teach.Tail`: an interface of plain values over the
lifecycle, so Teach can follow rehearsal, lowering and saving without being able to reach any of
them. Its boundary test still forbids `marcoexec`, `rehearse` and every platform package.

Two things came out of it that were not orchestration:

- **[[ADR-044-a-teach-attempt-is-one-episode]]** — the session-count audit found a real defect.
  Three teach passes were claiming three independent sightings, which would have let one explicit
  teach satisfy `MinSessions: 2` on its own. Fixed at the store, with the safe default.
- **The busy-start concern was not a defect.** Teach diffs the topology across the discovery pass,
  so history at the start is irrelevant and only same-pass ambiguity refuses. Characterised, not
  changed.

Twenty-four mutations applied and all twenty-four caught. **Fourteen of them survived on the first
attempt** — every one a real gap, closed with a test before the second run. A no-op control
confirmed the harness could report a survivor, and one equivalent mutant is recorded rather than
chased.

### Why this is worth doing before more perception work

It is the difference between a system that learns what it happens to notice and one somebody can
direct. It needs no new identity model, no new perception, no Core change and no new authority —
the pieces are built and proven end to end as of the same-surface migration. What is missing is
one entry point and an honest conversation around it.

- affects: [[Demonstrations]], [[Learned-Plays]], [[Semantic-Memory]]
- constraint: [[ADR-020-watch-me-is-permission-to-observe-not-to-act]] — an explicit teach is
  still permission to OBSERVE. It must not become permission to act, and the rehearsal grant
  stays where it is.
- constraint: [[ADR-041-a-screen-is-not-its-dominant-group]] — the endpoints must be durable
  subjects before the journey between them means anything.

## 33. Marco UI: make Director visible — PARTIAL 2026-08-11

The shell already existed and was already right — see [[ADR-045-teaching-is-a-section-of-the-playbill]].
What was missing was teaching reaching it, and that is now done: `Teaching` is a section of the
one account, sourced from the coordinator, rendered by Normal, Watch and Deep, and visible in the
overlay with no overlay change.

**PARTIAL, and the remaining half is interaction.** Parts 10-17 asked for buttons — confirm the
start, confirm the destination, Try Again, answer a proposal, approve a rehearsal, view the
generated Marco. Every one of those needs input handling in an ebiten window that cannot be
validated headlessly, and the answers already route correctly through the CLI. Building them
blind would be the one thing this milestone was written to prevent.

Also not done: the read-only live observation of Part 36, which needs a person at the machine.

## 34. Make Learn work end to end, against real software — ACTIVE

The whole chain, cold, on a real desktop, with a real person at the keyboard:

```
cold → Learn requested → START established → START visually grounded
     → human demonstration → DESTINATION established → DESTINATION visually grounded
     → durable A→B → second example → assessment → permission to rehearse
     → rehearsal → naming → lowering → save → restart
     → Do from the correct START → refusal from the wrong one
```

Called **Learn** in anything a person reads, per
[[ADR-048-learn-teach-and-do-are-three-different-sentences]]; still invoked as `director teach`
internally for the duration of this milestone, and deliberately not renamed while it is open.

**Why it is human-in-the-loop, and why that is not a gap to engineer around.** The person owns the
foreground, and injected input is unlearnable by design — a machine-driven demonstration would
violate the attribution invariant it is meant to be testing. Only the establishment half is
scriptable. See the memory note *teach-e2e-needs-human-hands*.

Cleared so far:

| gate | state |
|---|---|
| place bootstrap | **DONE** — [[ADR-047-a-place-is-remembered-a-meaning-is-answered]] |
| foreground targeting, no `--window-id` | working |
| START established, cold, and grounded | yes, on Windows Settings |
| human keyboard action observed | yes |
| DESTINATION established, cold recall | yes |
| durable A→B | the current gate |
| vision, OCR | off throughout |

Two things came out of it that were not the chain itself:

- **Grounding is a property of the application, not of Marco.** Explorer cannot be pointed at: its
  navigation tree is virtualised and reports scrolled-out items at y = −1768…+1496, so members are
  unplaceable however reliable the frame is. Classified rather than papered over — no clamping, no
  shifting, no stale geometry. See [[Experiment-012-why-explorer-cannot-be-pointed-at]]. Settings
  grounds cleanly with sixteen regions and became the E2E application.
- **The adjacency defect was measured, not inferred.** Two live Settings runs both recorded
  `A → state_unknown → B` where one sample fell outside both recognised screens, and both halves
  went session-local. Instrumented first with a closed `SessionLocalCause` vocabulary, then fixed
  with a conservative relationship-layer bridge whose bound is segmentation's own
  `StatePromotionCount`.

**Correction, 2026-08-13 — Learn is one-shot.** The chain above asked the person to perform the
route twice, and the second demonstration was costing more than it bought: a live run held the
correct durable route with attributed navigation and both endpoints grounded, then refused to learn
it because the machinery for watching a *second* example failed. One clean demonstration now
reaches the offer to try, and a successful rehearsal is the confirmation.
[[ADR-051-one-demonstration-and-an-attempt]].
The two-pass shape it left behind — discovery, then an armed capture watching the identical route
a second time — was removed the same day: the pass that watches IS the demonstration.
[[ADR-052-the-pass-that-watched-it-is-the-demonstration]].

**Correction, 2026-08-17 — Learn is GOAL-CENTRIC.** The live-training runs exposed a flaw in the
learning model itself, not in its staging: `Learn(A → B)` memorised a route and welded the
capability to A, so every live test became choreography — exact starting screens, returns to
START, waits for observation phases. The model is now:

```
Learn "open mouse settings"
    demonstration:  A --action--> B     (evidence: one KNOWN way in)
    Marco learns:   goal = B            (the outcome, in the person's words)
```

What landed, each with its ADR and its tests:

- **Goal records and planning** — a durable `Goal` has no start structurally; `PlanToGoal`
  composes verified edges from wherever the person is; `director reach` is the read surface;
  honest refusal otherwise. [[ADR-056-a-goal-is-a-destination-not-a-route]].
- **Demonstrations decompose** — `A → B → C` yields one candidate per edge, never a monolithic
  macro; an unrecognisable start no longer ends a Learn; `left_the_start` is gone.
- **Capture first, interpret second** — attributed input is never discarded because a later
  layer failed; `ShadowTotals.InputLog` holds it all.
  [[ADR-057-attributed-input-survives-interpretation]].
- **Clicks resolve to controls** — at event time, against the actionable-control index each
  valid inference pushes; a resolved click assesses clean, rehearses as `Accessibility's
  Invoke` on the live control, and still cannot be written down as a play (follow-on).
  [[ADR-058-a-demonstrated-target-may-keep-its-name]] — the milestone's one deliberate privacy
  widening.
- **Rehearsals fire, and land in the right window** — the step-scope defect that failed every
  live rehearsal unconditionally is fixed and mutation-gated; a real attempt refuses
  `window_not_in_front` pre-claim and waits patiently; silent authorization failures now say
  why. [[ADR-060-input-has-no-address]].
- **Grounding has a lifecycle** — a highlight belongs to its claim, dismisses when the claim
  ends, and a settled session owns no presentation. [[ADR-059-a-presentation-belongs-to-its-claim]].

**Follow-on, deliberately out of this milestone:** writing a click route down as a play (needs a
run-time name-resolving activation capability, the way `Screen's Showing` resolves a screen's
name); executing a multi-edge plan end-to-end (the planner composes knowledge; execution remains
per-edge through saved plays); arbitrary planner topologies beyond BFS-over-verified-edges.

- affects: [[Demonstrations]], [[Semantic-Memory]], [[Passive-Observation]], [[Learned-Plays]]
- constraint: [[ADR-020-watch-me-is-permission-to-observe-not-to-act]] — Learn is permission to
  observe, never to act.
- constraint: [[ADR-046-grounding-a-screen-points-at-its-structure]]
- constraint: [[ADR-051-one-demonstration-and-an-attempt]] — the rehearsal carries the confidence
  the second demonstration used to, so it may not be skipped.
- constraint: [[ADR-056-a-goal-is-a-destination-not-a-route]] — the destination is the
  capability; the route is evidence.

## 35. Marco product experience — Learn · Teach · Do — NEXT after 34

Not "make the current UI prettier". The goal:

> A normal person can launch Marco and understand what it sees, what it is doing, what it wants
> from them, what it has learned, and what they can ask it to do — **without knowing Director
> exists**.

The organising model is [[ADR-048-learn-teach-and-do-are-three-different-sentences]]. At minimum:

**Learn.** Turn Roadmap 34's mechanism into an experience. None of `--window-id`, session ids,
question ids, subject ids, CLI copy-paste, `candidate_consistent`, `SessionLocal` or `AskRehearse`
appears in the normal path. What it must communicate: what Marco thinks the starting place is ·
what it is watching · what action it saw · where it thinks the person ended · whether it needs
another example · when it wants permission to try · whether the rehearsal worked · what was
learned. Visual grounding is central, not ornamental.

**Teach.** Design the experience of Marco guiding a person through a learned behaviour against
their live UI. This may land as a UX/prototype milestone rather than full capability if the route
representation cannot yet supply every intermediate target honestly — **do not fabricate an
intermediate referent to complete a screen**. Establish first what the learned evidence can
actually support.

**Do.** A clear user-facing execution action for a learned behaviour, over the existing authority
model unchanged.

**Home.** The ambient default surface, calmly answering: is Marco active · what place does it
currently understand · is it learning something · is it waiting for me · did it just notice
something. Debug belongs behind Debug.

**Things Marco Knows.** The semantic-memory UI as a product surface, separating *things Marco knows
about the UI* from *things Marco has learned how to do*, with the actions each can honestly
support — Teach me · Do it · Rename · Learn again · Forget.

**Questions.** Attached visually to their referent, answered in human words — Yes · No · Not sure ·
Show me — with correction always available.

**Visual design is in scope, explicitly**: information hierarchy, layout, typography, spacing,
motion, transitions, responsive behaviour, empty states, loading states, refusal states,
onboarding, interaction feedback, grounding presentation. The current UI is an engineering surface
and **is not a baseline that must be preserved**. Substantial redesign is permitted. Preserve
semantics and safety, not today's layout.

**Visual acceptance is mandatory.** DOM elements and strings existing is not completion. It is
inspected running, at several useful window sizes, with the flows walked. Screenshots may be used
*transiently* for developer review; this changes nothing about the privacy architecture and Marco
still persists none. A technically correct but confusing surface is a failed acceptance.

### The boundary

```
ROADMAP 34:  Make Learn + visual grounding actually work end to end.
ROADMAP 35:  Make Marco feel like a coherent product built around Learn · Teach · Do.
```

Nothing from 35 is begun while 34 is open.

- affects: [[Visibility]], [[Demonstrations]], [[Learned-Plays]], [[Semantic-Memory]]
- constraint: [[ADR-048-learn-teach-and-do-are-three-different-sentences]]
- constraint: [[ADR-037-opt-in-is-enforced-on-every-door]]

## Deferred, with reasons

- **Grounding DINO on GPU** — the 9.6s that sank it is a CPU-only `torch` artefact, but it
  also scored 20% structural coverage and 100% false structure. Fixing latency would raise
  its score without making it useful.
- **OmniParser `icon_caption`** — a caption gives a *label*; the privacy allowlist gates on
  *role*. Captioning an `icon` yields a withheld label on a still-unnameable role. Valuable
  **after** roles are solved.
- **Florence-2, OWLv2** — OWLv2 duplicates a question Grounding DINO already answered.
- **Qwen2.5-VL, Molmo** — cannot meet a 500ms sampling budget on likely hardware.
- **SAM2** — excellent masks, no semantics. It cannot supply a role, which is the one thing
  needed.

Full reasoning: [[director-vision-backend-decision]].

## Resolved test debt

- **`TestSeparateRuntimesHaveDifferentEpochs` was flaky, and the flake was the defect.**
  An epoch was `pid + UnixNano`. `UnixNano` is nanosecond-typed but not nanosecond-resolved —
  the Windows system clock advances in ~15ms steps — so two runtimes constructed inside one
  tick produced the *same* epoch, which is exactly the case an epoch exists to distinguish. A
  client could not have told that restart from a log rollover. Fixed with a process-wide
  `atomic.Uint64` tiebreaker (`runtimeEpochCounter`, `cmd/director/runtime.go`); verified at
  `-count=50` and under `-race`. Recorded here because it was misread as test noise for two
  milestones: a test that fails intermittently is reporting a race in the product until proven
  otherwise.
- **`go test -tags onnxvision ./...` never compiled in `plugins/vision`.** `identify_test.go`
  used `nullDetector`, which only exists under `!onnxvision`, so the build tag production
  actually ships was untested. Fixed with a tag-independent stub; `TestNullDetectorDeclines`
  became `TestUnconfiguredDetectorDeclines` and now asserts the invariant that holds in both
  builds (an unconfigured detector finds nothing) rather than one build's exact return value.

## Not on this list

Overlay UI wiring for `marco dispatch --json`, and a live test harness for [[Goals]] — both
real, both waiting on decisions above.

## Related

- [[Director]], [[Decisions]], [[AI-CONTEXT]]
