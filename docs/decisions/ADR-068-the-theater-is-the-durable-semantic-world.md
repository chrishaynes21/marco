---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - semantic-memory
  - learned-plays
  - demonstrations
  - perception
source_paths:
  - internal/director/observe/recall.go
  - internal/director/observe/remember.go
  - internal/director/semanticmemory/store.go
  - internal/director/marcoexec/play.go
---

# ADR-068 — the Theater is the durable semantic world

A learned play was about to say `do Accessibility's Invoke with a Control called "Mouse"`. It
worked, it was verified live, and it welded a **provider** into a **behaviour**: a play learned on
a machine with the accessibility bridge would require that bridge forever, on every machine, even
where sight or reading could resolve the same thing perfectly well.

This settles what a learned play refers to, before more of them are written.

## The ontology

| | what it is | where it lives |
|---|---|---|
| **Theater** | the durable semantic world Marco knows: places, targets, routes, goals, and who said so | a semantic contract over `semanticmemory` |
| **Scene** | **a Marco language concept, unchanged by this ADR** | `spec/Core.md` |
| **Target** | a semantic thing that can be grounded in a place and acted upon | a durable subject, new here |
| **Play** | behaviour written against goals and targets | generated `.marco` |
| **Director** | observes, resolves intent, plans, executes, verifies | `internal/director` |
| **Audience** | the person, whose demonstrations, names and corrections are the only source of semantic authority | not a type — a provenance |
| **Fusion** | reconciles live observations toward semantic identities; persists nothing | `perception/fusion` |
| **Providers** | produce observations and execution capabilities | accessibility, sight, reading |

### Director decides what should happen; Theater makes it happen

Theater is not a wrapper over storage. It is the world a play is performed in, and it owns five
things:

| | |
|---|---|
| **Repertoire** | the durable semantic world — targets, places, routes, goals, and who said so |
| **Stage** | what is live right now: applications, windows, the place in front, the targets present |
| **Casting** | which Player can actually perform this action on this Stage |
| **Production** | mounting the play and running it |
| **Verification** | did the Scene change the way the play said it would |

**Players** are the capabilities that can act — accessibility, keyboard, pointer, OS, and a
sighted actuator later. A play never names one. Theater casts whichever can perform the part on
the Stage it finds, which is precisely what makes a learned play portable.

`semanticmemory.Store` is the **storage engine underneath the Repertoire** and stays exactly
that: one file, one match-or-append, one subject bound. It is one part of Theater, not all of it.

No `theater/` package is created for the metaphor, and nothing is renamed to chase it. The concept
names the responsibility; implementations keep the names that already describe what they do.

### Scene is untouched

`scene` is a first-class Marco word — *a place in the play, holding actors, with verbs of its own*
— guarded by `internal/spectest`, and the Director has never used it. Director's `screen_state`
is a durable place identity, which is a different thing that happens to rhyme. **This ADR changes
nothing about Marco's `scene`, and deliberately does not map the two.** If that mapping is ever
wanted it is its own decision.

## Durable Target identity

A new subject kind, `SubjectTarget`, through the canonical machinery — the same
match-or-append, the same id derivation, the same `MaxSubjects` bound, the same `Called` and
`Knowledge` fields that a place already has. Not a parallel database.

A Target's identity is **name, kind and place**:

```
Subject: target
Label:   "Mouse"          the word the person saw
Kind:    button           a small semantic vocabulary, never a provider's control-type list
Place:   subj_543…        the durable place it was grounded in
```

Matched by exact label and kind within a place — a name is not a composition, so the tolerance
rules that make screens robust do not apply and would be wrong here.

### What a Target may never contain

UIA RuntimeIds, accessibility node ids, DOM ids, bounding boxes, OCR rectangles, vision boxes,
absolute coordinates. **Those are live resolution evidence, not identity.** They belong to
whichever provider produced them and expire with the frame they came from. A Target that held one
would be a Target that stopped working after a redraw — which is the failure this whole ADR
exists to prevent.

## What the Play says, and what `Name` means

```marco
the mouse is a Target with Name "Mouse".
do Theater's Activate with mouse.
```

`Name` is **not** an instruction to search for a string. It is a reference into Theater, resolved
the way a place already is: `do Screen's Showing with "Mouse Settings"` names a durable subject by
the word a person uses, and the fingerprint stays backstage. A Target works identically — the
durable record carries the evidence, and the play carries the reference.

That precedent is why this needs no new syntax. Two constraints were verified against the real
compiler and shape the form:

- an act and a set **cannot share a name**, so `Target` is the set and the act is something else;
- `named "Mouse"` is **not** Marco syntax, and `with Name "Mouse"` is both legal and the better
  English — the field is a noun, like every other field in the language.

The act is **`Theater`** because Theater is what performs: the play asks the world it is being
staged in to activate a target, and the world casts a Player. Not `Screen`, whose own first
paragraph says every capability in it READS and that *deciding must not be doing*; and not
`Accessibility`, because naming a provider in a play is the thing being fixed.

## Runtime

```
play:      Activate(mouse)
Director:  which Target is that?          → Theater
Fusion:    what resolves it right now?    → accessibility | sight | reading
Director:  which executor can act on it?
executor:  Accessibility → Control.Name → Invoke
Director:  did the world change as expected?
```

`Control.Name` keeps its place and its tests — it is now an **Accessibility executor detail**,
one way to satisfy a Target activation, reached after resolution rather than written into the
play.

**Training provider is provenance, not a dependency.** Theater records that "Mouse" was first
learned from accessibility evidence; the play does not require accessibility to run. Where an
operation genuinely needs a capability, it is expressed semantically — *activate a target*, *set a
value* — never *requires UIA*.

## Provenance: who said so

Theater records where a durable semantic claim came from, because authority differs:

| | examples |
|---|---|
| **Audience** | the Learn intent, a goal's name, a screen's name, a rename, a correction, consent to rehearse |
| **Perception** | an observed label, an observed role, geometry, presence |

A perception-authored label is evidence. An audience-authored name is a decision. They must never
be stored as the same kind of claim, because a correction has to be able to win.

## Privacy: this is a widening, stated exactly

**PRIVACY_WIDENED: YES.**

Today the durable text boundary is sharp: `RememberedSubject.Called` is *"the one durable string
that comes from a person rather than from perception"*, and an accessibility label may never land
there. A durable Target's `Label` is perception-derived, so it crosses that line.

The boundary that replaces it, and nothing wider:

- a label may become durable **only** for a control the Audience themselves activated, during a
  session under an explicit Learn licence;
- it must already have passed `observe.AdmittedTargetLabel` — the single label policy: the
  plaintext role allowlist, widened to activatable roles only under that licence, plus the shape
  filter either way;
- the label is stored as **perception-provenanced evidence**, never as the Audience's own word;
- nothing else observed in the session becomes a Target. Theater does not remember visible text
  because it might be useful.

## What this does not do

It does not build every Theater capability, a relationship model for targets, multi-target
disambiguation, or a second executor. It defines the world model and proves one vertical through
it.

## Enforced by

- `internal/director/observesession/targetwiring_test.go` — a demonstrated click becomes a durable
  target grounded in the place it was PRESSED on, not the one it navigated to;
  `TestADemonstratedTargetBecomesDurable` enters through the real session;
  `TestAnUnlicensedSessionEstablishesNoTarget` is the control;
  `TestADemonstratedTargetSurvivesARestart` reopens the file as a new process would;
  `TestADurableTargetHasNowhereToPutAProviderHandle` walks the type reflectively, so a field
  somebody adds tomorrow is caught; `TestAnUnknownKindDoesNotDisagree` is what keeps an
  accessibility-trained target reachable by a resolver with no opinion about control types.
- `internal/platform/theaterhost/theaterhost_test.go` — one match performs and verifies; zero
  refuses; SEVERAL refuse rather than pick the first;
  `TestATargetLearnedByOneActorIsActivatedByAnother` is the portability proof, with the trained
  actor unavailable and the play unchanged; `TestThePlaySentenceGoesThroughTheTheater` holds the
  Marco boundary against a host that answers `ok` without casting anybody. Performing and
  succeeding are kept apart at the production boundary now rather than in `Activate` — see
  [[ADR-070-one-production-body-and-the-caller-brings-the-verification]].
- `internal/platform/theaterhost/accessibility_test.go` —
  `TestTheAccessibilityActorSendsANameAndNeverAHandle` is the restart proof in its strongest form:
  nothing but a label ever crosses the bridge, so every future restart is covered rather than the
  one a test happened to perform.
- `internal/spectest/invokeplay_test.go` — `TestALearnedPlayNamesNoProvider` compiles the
  generated play with the real compiler and fails on any provider vocabulary in it.
- `internal/director/observe/recall_test.go` — the durable-text exceptions are a closed list with
  the licence written against each, and their count is asserted.

## Related

[[ADR-067-a-play-may-name-a-control]] · [[ADR-031-the-user-names-the-stage]] ·
[[ADR-058-a-demonstrated-target-may-keep-its-name]] ·
[[ADR-047-a-place-is-remembered-a-meaning-is-answered]] ·
[[ADR-016-cross-session-identity-is-structural-and-conservative]] ·
[[Semantic-Memory]] · [[Learned-Plays]]
