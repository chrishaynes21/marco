---
type: decision
status: accepted
date: 2026-08-09
supersedes: []
affects:
  - vision
  - perception
  - privacy
  - semantic-memory
---

# Structure earns a name; text never earns structure

## Context

[[Experiment-009-ocr-as-a-semantic-discriminator]] measured OCR as the source of the
discriminator [[ADR-016-cross-session-identity-is-structural-and-conservative]] requires, and
found it supplied nothing. Not because it failed to run — it ran, through production, with its
raw text correctly discarded — but because of *where* its output could attach.

The chain was: fusion's `TextFilledMissingLabel` rule turns OCR text into a label only when
**accessibility** has already supplied a structurally real element with an allowlisted role.
So where accessibility names things, OCR is redundant; where it exposes nothing — the case a
detector exists for — the text had nowhere to land.

Meanwhile the vision detector was producing exactly the missing half. ScreenParser reports
`button` and `menu` regions on surfaces with no accessibility tree at all, and those regions
reached the session as a role and a rectangle, with whatever was written on them dropped.

Two things were missing, and one thing had to stay impossible.

## Decision

**A visual structure earns the right to carry readable text because of the ROLE it maps to.
Text never earns a structure.**

The ordering is fixed and one-way:

```
vision finds structure
  → the structural observation is accepted on its own evidence
    → the role decides whether this thing may be named
      → only then is text read, scoped to that region
        → the privacy classifier decides whether it may be kept in the clear
          → closed-vocabulary terms are derived
            → the raw text dies
```

Never `OCR says SETTINGS → therefore a button exists`.

### One canonical role policy

`directorapi.ElementRole.NameablePlaintext` is the allowlist — `button`, `menu_item`, `menu`,
`tab`, `checkbox`, `radio` — and it is now the *only* one. Four copies existed: the session's
privacy classifier, the shadow diagnostic, the vision benchmark's nameable-role coverage, and
(newly) the scoped label reader. Two of those copies carried a written rationale for being
copies, on the argument that a benchmark measures what a backend *could* offer while a policy
decides what may be *stored*.

That argument no longer holds, and the change is deliberate. Since nameability now gates what is
**read**, a role counted nameable in one place and withheld in another is a promise of evidence
the system refuses to collect. Widening the set is a privacy change and must be reviewed as one,
in one file.

### Structural, nameable and actionable are three properties

Not one boolean, and nothing may collapse them:

| vision class | canonical role | structural | nameable | OCR-eligible | actionable | why |
|---|---|---|---|---|---|---|
| `button` | `button` | yes | **yes** | **yes** | never from vision | a button's name is a fact about the interface |
| `menu` | `menu` | yes | **yes** | **yes** | never from vision | same |
| `checkbox` | `checkbox` | yes | **yes** | **yes** | never from vision | same |
| `radio` | `radio` | yes | **yes** | **yes** | never from vision | same |
| `icon` | `icon` | yes | no | no | never from vision | an icon's "text" on a desktop is a document title or a contact |
| `field` | `text_field` | yes | no | no | never from vision | a field's contents are what the person typed |
| `slot` | `list_item` | yes | no | no | never from vision | a grid cell's contents are user data |
| `bar` | `progress_bar` | yes | no | no | never from vision | a meter has no name |
| `panel` | `group` | yes | no | no | never from vision | reading a panel reads whatever it contains |
| `text` | — | **no** | no | **yes, screen-level only** | never | rendered text; may mean something about the SCREEN and names nothing |
| `image` | — | no | no | no | never | pictorial content, and reading inside one is the hallucination case |

Actionability is absent from every row on purpose:
[[ADR-004-vision-cannot-establish-actionability]] is untouched.

### Two privacy surfaces, not one

- A **session-local safe label** may hold readable text, for a nameable role, for the length of
  a session. A structural role vouches for it.
- A **durable semantic term** is never text. It is a member of the closed vocabulary in
  `observe/terms.go`, and there is nowhere in the durable representation to put a string.

A text region takes the stricter deal unconditionally: its words may become terms and may never
be released in the clear, because unlike a button's name nothing structural vouches for what a
text region says.

### Nameability gates READING, not only keeping

This is a change from "read everything structural, let the classifier withhold". Same answer,
several times the cost, and one avoidable risk: a scoped reading is a 129ms serial round trip
(measured, below), so reading a panel and forty icons to discard all of it spends the budget the
buttons needed — and it puts an icon's text into the process for no purpose.

### Ambiguous association is refused, never resolved

Text belongs to a region when it was read *from* that region and no other nameable region
competes for it. Nesting is not competition — the innermost thing a word sits in is the thing it
names, which is how interfaces are drawn. Overlap without nesting IS competition, above
`AmbiguousOverlap` (0.25 of the smaller box). Stacked menu buttons overlap by ~6% through
detector jitter and must both be named; two boxes genuinely competing share most of one.

Nothing is resolved by iteration order or by nearest centre. An unattributable reading is
dropped and counted.

### Provenance is not waived for being experimental

Semantic evidence is attached only when the inference can prove it observed the window
generation the cycle was about. The consequence of getting this wrong is larger for a term than
for a box: a box is counted this session, a term reaches cross-session identity and outlives the
mistake.

### Unknown is not empty, on this path too

Carried forward from [[Experiment-009-ocr-as-a-semantic-discriminator]]. A cycle that did not pay
for a reading, one whose regions were all unsayable, and one that read text and matched no
concept are different facts. Only the last is evidence about the screen.

## Consequences

- An application with no accessibility tree can now produce interface terms, reach a screen
  state, a hypothesis and a durable signature. Measured: 0 terms → 4 terms on the same fixture,
  with structure unchanged.
- Two structurally identical settings screens are now separable: `candidate` on structure alone,
  `different` with the terms the detector's own boxes carried.
- A label pass costs more. 8 readings at 129ms median is ~1.0s on top of a ~0.9s inference,
  against a 2s shadow cadence — so a label pass will occasionally cost one skipped shadow slot.
  Skips are counted and were already reported.
- The vision benchmark's nameable-role coverage may move, because it now measures the real
  policy. The four surviving classes are unchanged.

## Enforced by

- `TestVisionStructureBecomesSemanticWithoutAccessibility` — THE production path, from a fake
  capture through the registered detector, `newShadowProvider`, the real collector and sampler,
  to a durable signature, with accessibility contributing nothing. Deleting the reader, deleting
  the nameability gate, dropping terms before `SignatureOf`, or disconnecting the association
  call in `shadowSampleFor` each fail it.
- `TestReadTextNeverBecomesAControl` — a read heading contributes a screen-level concept and
  produces no element, no role and no detection.
- `TestAnUnsayableRoleIsNeitherReadNorNamed` — an icon, a panel and a progress bar that plainly
  say SETTINGS are not read at all and yield no term. Widening the allowlist fails it three ways.
- `TestOverlappingControlsDoNotTakeEachOthersWords` — and its second half, that a plain vertical
  menu is *not* ambiguous.
- `TestSemanticEvidenceIsRefusedFromAStaleTarget` — generation 8 against an expected 1.
- `TestAVisionPassWithoutTheTextBudgetReportsUnknownNotEmpty` — unknown ≠ empty.
- `TestNoReadableTextLeavesTheVisionSemanticBoundary` — asserts on the SHAPE of what crosses.
- `TestTheLabelBudgetIsReportedRatherThanSilent` — the cap, and which kind of drop it was.
- `TestTheDeterministicGainIsMeasuredAndReported` — before/after on one fixture.
- `TestSimilarStructuresAreSeparatedByVisionDerivedTerms` — the false-merge case.
- `TestTheSameSubjectAndTheOneMissingTerm` /
  `TestATransientlyLostReadingDoesNotChangeIdentity` — the exact-set rule, measured rather than
  changed.
- `TestATracedSemanticReadingReplaysToTheSameSignature` — replay parity, and no text on disk.
