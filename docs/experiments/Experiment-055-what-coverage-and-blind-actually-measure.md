---
type: experiment
status: complete
date: 2026-10-15
backend:
  - structure
  - ocr
fixture: "the measured Trackmania Stage (228 structural Props) against the hand-built Discord husk of internal/director/policy and cmd/director/shellgate_test.go, plus a reading of every caller of Coverage, Blind and Shallow"
result: coverage-measures-the-share-of-reported-objects-carrying-a-specific-non-container-role-and-blind-measures-whether-any-role-is-an-interactive-kind-so-both-are-accessibility-measures-wearing-general-names-and-the-existing-distinguisher-seen-only-by-pixels-is-itself-role-gated-and-cannot-fire-and-the-resolver-already-states-the-principle-policy-contradicts-so-the-smallest-change-is-for-the-two-world-gates-to-learn-whether-this-request-grounded-a-target-and-no-formula-threshold-or-source-exemption-changes
supersedes: []
affects:
  - execution
  - perception
---

# Experiment 055 — what Coverage and Blind actually measure

**Status: CODE READING AND MEASUREMENT, 2026-10-15. Nothing implemented. No formula, threshold,
dimension, role, licence or detector changed.** The harness is in the session scratchpad as
`exp054-investigate.go.txt`.

## The question

> The measured Trackmania Stage has 274 observations and 228 structural Props, but policy reports
> coverage 0.000 and refuses as though no content was visible. That may be appropriate for
> accessibility coverage, but it is not an accurate description of visual observability.

## 1. What Coverage measures today, exactly

```go
  content := 0
  for _, el := range w.Elements {
      if el.Role.Content() { content++ }      // Content() == !Container()
  }
  c.Coverage = float64(content) / float64(len(w.Elements))
  for range w.Degraded { c.Coverage *= 0.6 }
```

and `ElementRole.Container()` returns true for `Window, Pane, Group, Dialog, Toolbar, TabList,
List, Tree, Table, Menu, RoleUnknown, ""`.

**So Coverage is the share of the objects that were reported whose role is a specific,
non-container kind.** It is a measure of ROLE SPECIFICITY in a reported tree. It is not visual
extent, not window area, not the fraction of the screen accounted for, and not whether anything
was seen. Its own doc comment says *"did we see INTO the application, or only its shell"*, and
that reading is true only while the reporting source is one that assigns specific roles — which
is to say, while it is an accessibility tree.

The measured consequence is that two opposite worlds produce the same number:

```
                                  elements   roles                coverage  quality
  Discord, backgrounded (shell gate)  8      pane 7, window 1      0.00      0.90
  Trackmania, structural Stage      228      unknown 228           0.00      0.056
```

The first is a husk: a window whose contents nobody can see. The second is 228 measured
rectangles, 13 of them carrying OCR-corroborated names, on a frame where a person can read the
menu. Coverage cannot tell them apart, because the only thing it asks is whether a role was
specific, and a proposed rectangle's role may not be
([[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]]).

**Every caller, and what each relies on:**

| Caller | Relies on |
|---|---|
| `policy.checkWorld` gate 1 | `< MinCoverage` (0.35) refuses the whole world outright |
| `policy.Constraints` | `< MinCoverage \|\| Blind()` → `MaxRisk` empty, so a planner may propose nothing |
| `WorldConfidence.Shallow()` | `< 0.35 && quality >= 0.75` — the Electron/browser signature |
| `WorldConfidence.Overall()` | `min(quality, coverage)`, so coverage caps the summary number |
| `target.unobservable` | via `Shallow()` → blocker `coverage`, the "not evidence of absence" answer |
| `execute/replay.go:466` | a hard-coded `< 0.35` costs a repeat 0.3 of its Context score |
| `wait/conditions.absenceConfidence` | `0.4 + 0.5*coverage` gates how much a conclusion of ABSENCE is worth |
| `service/providers.go` | `ProviderStatus.Coverage`, and `Shallow()` → status `"shallow"` |
| `cmd/director/main.go`, `observesnapshot.go` | the five-number report; and `fusedStructure`'s `Shallow() && Blind()` shell gate |

Two of those are load-bearing in ways a rename would not reach. `absenceConfidence` uses coverage
to decide how much "it is not there" is worth, which is the correct use of a *role-specificity*
measure and would be wrong if coverage came to mean visual extent. `Overall()` caps the summary.
And a naming hazard: `BindingDiagnostics.Coverage` in `execute/binding.go` is goal-confirmation
coverage, an unrelated concept with the same word.

## 2. What Blind() measures

```go
  func (c WorldConfidence) Blind() bool { return c.Actionability <= 0 }

  if interactive > 0 { c.Actionability = float64(addressable) / float64(interactive) }
```

where `interactive` counts `roleInteractive(el.Role)` and `addressable` counts `Label != "" &&
Targetable()`. With no interactive role anywhere, `Actionability` is left at zero.

**So `Blind()` is "no reported object has an interactive ROLE, or none of those that do is named,
enabled, on screen and actuable".** Three different worlds answer it identically:

```
  a  nothing was reported at all
  b  objects were reported and not one has a specific interactive kind   <- the structural Stage
  c  interactive kinds exist and none of them is addressable
```

The system already knows this is a problem and already has a distinguisher — but **the
distinguisher is itself role-gated and therefore cannot fire on case (b)**. `SeenOnlyByPixels()`
counts only elements where `Actions().Affords()`, and `Affords()` requires `Any()`, which is
derived from the role. On the structural Stage nothing affords, so the count is zero and the
method returns false. Measured: policy gate 2 prints

> nothing in this window can be operated, so there is nothing here to act on

and never reaches its own better sentence,

> I can see controls here but nothing told me how to operate them — this window was described by
> the screen alone, with no accessibility information

which is the one that is actually true of this Stage. The comment above it says *"those need
opposite responses: the first is a window with nothing in it, the second is one Marco can SEE and
has no mechanism to work"* — exactly right, and unreachable for the case that most needs it.

## 3. Can the existing WorldState and provenance distinguish the three cases

**Yes, per cycle, with no new persistent state. Every fact is already present; what is missing is
a name, not a measurement.**

```
  len(w.Elements)                      were any objects reported at all
  w.Degraded, cycle.Failures           which expected source did not report, and why
  el.Provenance.Sources                which source reported THIS object, each kept apart
  el.Provenance.OnlyDescribesPixels()  did anything that can operate it report it
  el.Provenance.DescribesAnObject()    did anything but read text account for the object
  el.Role.Generic()                    "" / unknown / text — no claim about kind was made
  el.KindEvidence                      described / pixel_named / pixel_only, per FIELD
  el.Label, Bounds, Visible, Offscreen the grounding facts
```

Mapping the owner's four concepts onto what exists:

| Concept | Where it lives today |
|---|---|
| **Observability** — did Marco receive usable information? | Nowhere as a measure. The facts are in `len(Elements)`, `Degraded`, `cycle.Failures` and per-element `Provenance`. |
| **Grounding** — can Marco identify target and surface? | `target.Resolution` — exists, works, and already carries every candidate with its reason. |
| **Actionability** — is there an executable Actor path? | **Split across two places that disagree.** `uiact.Ladder(kind)` plus each `Rung.available(target)` is the real answer, and it runs AFTER policy. `WorldConfidence.Actionability` is a role-and-naming proxy standing in for it, before it. |
| **Authorization** — is Marco permitted to attempt it? | `OperateLicence` at grounding, `PolicyDecision` and the confirmation gates after planning. Exists and is layered correctly. |

That third row is the architectural finding. The question *"does the proposed action have an
executable Actor path"* has a component whose whole job is to answer it, with a rung-by-rung
account and a sentence per unavailable rung — and the gate that refuses for lack of actionability
runs first, using a share computed from roles and labels.

## 4. The smallest change

**The resolver already states the principle policy contradicts.** From `target.ResolveUnder`:

> A match found despite a poor world is still a match: the world being weak overall does not
> unfind a control we can plainly see and use.

It therefore checks `unobservable(w)` — the `Shallow()`/`Blind()`/degraded tests — **only after
finding nothing usable**. `policy.checkWorld` asks the same questions unconditionally, before the
target is in scope at all, and `EvaluatePlan` has no way to know a target was resolved:
`directorapi.Plan` carries steps, risk, conditions and assumptions, and no resolution.

So gates 1 and 2 are a second answer to a question the resolver has already answered, and in the
one case where the two can disagree — a target identified in a world whose dimensions are poor —
policy wins and is wrong.

**Recommended: the two world gates learn one new input, whether this request grounded a target in
this world.**

1. `EvaluatePlan` receives the resolution (or the resolved target) alongside the plan.
   `EvaluateStep` already takes `tgt *directorapi.ResolvedTarget`; this is the same fact, one
   level up. One implementation (`policy.Engine`), two consumers (`execute.Pipeline`,
   `actiongraph.replay`).
2. Gates 1 and 2 become **conditional on the absence of a grounded target** — not weakened, not
   re-thresholded, not exempted by source. They are skipped only when the question they exist to
   ask has already been answered yes by the component whose job it is.
3. Their refusal sentences name which of §2's three cases this is, from the facts in §3. This part
   is required whatever else is decided: a refusal that says *"only 0% of this window's content was
   visible"* about 228 measured rectangles is a wrong sentence regardless of whether the decision
   is right.

Nothing about `Coverage`'s formula, `MinCoverage`, `Blind()`, `RoleUnknown`, `Targetable()`,
`OperateLicence` or any detector threshold changes. No source is exempted from anything.

**The alternatives, and why each is worse:**

| Alternative | Why not |
|---|---|
| `Coverage = 1` when pixels exist | Hides the mismatch and corrupts two correct uses: `absenceConfidence` would value "it is not there" by visual extent, and `Shallow()` would stop recognising the Electron husk. |
| A fifth dimension, "visual coverage" | A new number that nine call sites must each decide how to weigh, and it would not change either gate's decision — both gates would still refuse. |
| Exempt `SourceStructure` from the gates | Source-based exemption is precisely what [[ADR-101-visual-presence-is-not-legal-actionability]] exists to prevent. |
| Lower `MinCoverage` | A global threshold change, and wrong on its own terms: 0.35 is right for the thing coverage measures. |
| Let policy consult the licence | `OperateLicence`'s own contract forbids it — *"It cannot reach policy. Risk, destructiveness and every other permission question are evaluated after planning, from the action itself, and nothing here is consulted there"*. This is the tempting one and it must stay refused. |
| Put the resolution on `Plan` instead of in the signature | Smaller diff, worse model: a plan carrying a snapshot of a snapshot, which every later reader would be entitled to trust after the world had moved. |

## 5. How the other refusals are preserved

| Refusal | Mechanism, and whether it is touched |
|---|---|
| Genuinely blind — nothing reported | `len(w.Elements) == 0`, `w.Degraded`. Becomes its OWN case and its own sentence instead of sharing one with two others. Strengthened, not relaxed. |
| A shell — observed, never seen into | Gate 1 for its real case. A husk resolves no target, so the conditional never fires and the Discord world refuses exactly as it does today. |
| Stale | Gate 4 on `Freshness`, plus `capture.Image.CapturedAt`'s 2 s bound upstream. **Not** conditional on anything: a grounded target in a stale world is a target that may have moved. |
| Ambiguous | `Resolver.AmbiguityMargin` with `MinContenders`, and fusion's `ambiguousRival`. Untouched, and both run before policy. |
| Destructive or irreversible | `plan.Risk`, `checkAction`, the `RiskHigh` confirmation, `destructiveAction`, `MinTargetConfidence` for medium-or-higher risk. Untouched, and the licence cannot reach any of it. |
| Unsupported action | `uiact.Ladder` ends in `refuseRung()` for every verb. Untouched. |
| An application's own rules | `policy.Rules`, which may only NARROW. Untouched, and applied last. |

## 6. What happens to accessibility-backed behaviour and its tests

**The blast radius is bounded by a fact worth stating plainly: no production code path requests
`SourceStructure`.** Grepped — the only caller is the corpus test. An empty `Request.Sources`
means "what the providers can cheaply see" and the structural provider opts out, so every world
the Director builds today is built exactly as it was before ADR-203, and no accessibility-backed
behaviour has changed or would change.

- **All 18 tests in `internal/director/policy` build `WorldConfidence` by hand and pass no
  resolved target.** A conditional skip keyed on a grounded target leaves every one of them
  deciding identically, including `TestQualityCannotCompensateForCoverage` and the vision tests.
- `cmd/director/shellgate_test.go` uses `Shallow() && Blind()` over a hand-built
  `{quality 0.90, coverage 0.00}`. Unaffected.
- At risk, and each needs checking rather than assuming: `target_test.go` where it asserts the
  `coverage`/`actionability` blocker strings; `service/providers.go`'s `"shallow"`/`"unobservable"`
  status strings; `execute/replay.go`'s hard-coded `< 0.35`; `wait/conditions`' `absenceConfidence`.
- **One interaction to check before anything requests structure in a learn or observe session.**
  The shell guard in `fusedStructure` needs `Shallow()`, which needs `quality >= 0.75`. A
  structural Stage measures
  **0.056**, so the rule that stops Marco minting durable places from a husk cannot fire on one —
  it is held off by the low confidence of low-contrast regions, which is a coincidence and not a
  rule. Nothing requests structure in that path today; this must not be true on the day something
  does.

## The tests this change would need

**Named as sentences, not as backticked identifiers: none of these exists, and the vault's own
gate is right to refuse a citation of a test that does not.** Each is paired with the mutation it
must die to.

1. *A world with nothing in it refuses even when a target was grounded.* Kill it by removing the
   empty-world case from the conditional.
2. *A shell world still refuses when nothing was resolved.* The Discord husk, hand-built, deciding
   exactly as it does today. Kill it by making the skip unconditional.
3. *The refusal names which kind of blindness this is.* Three worlds, three distinct sentences —
   nothing reported; objects reported with no specific kind; interactive kinds present and none
   addressable. Kill it by collapsing any two.
4. *A stale world refuses whatever was grounded.* Gate 4 is not conditional. Kill it by extending
   the skip to freshness.
5. *A high-risk action on a pixel-only Stage still requires confirmation.* Kill it by letting the
   grounded-target condition reach the risk path.
6. *Every world the existing policy tests describe decides identically.* The eighteen hand-built
   confidences as a table, asserting before/after equality. This is the regression wall and it
   should be written FIRST, before the change.
7. *Policy never consults a licence.* A static guard that `internal/director/policy` neither
   imports nor names `OperateLicence`, so the tempting alternative in §4 cannot be added quietly
   later.

## Limitations

- One structural Stage and one hand-built husk. The claim that Coverage conflates them is exact
  and measured; the claim that a conditional skip is safe rests on reading the gates' stated
  purposes, not on a corpus.
- Nothing here measures what happens on a Stage that has BOTH accessibility and structure, because
  no fixture has both: the desktop fixtures are PNGs served through `capture.WindowCapture` and
  have no tree either.
- `uiact`'s rung availability was read, not exercised: no measurement here shows what the ladder
  would answer for a licensed structural target, only that nothing asks it before policy.

## Related

- [[Experiment-054-the-surface-a-named-prop-does-not-carry]] — where the coverage-0.000 refusal
  was first measured, as an obstacle to something else.
- [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] — why the Stage's roles are
  generic.
- [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] — the licence
  that must not reach policy.
- [[ADR-101-visual-presence-is-not-legal-actionability]] — why no source gets an exemption.
- [[Experiment-056-whether-a-row-is-one-of-a-set]] — the separate assessment of structural
  evidence.
