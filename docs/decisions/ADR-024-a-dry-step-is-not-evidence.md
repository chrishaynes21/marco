---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - demonstrations
  - marco-boundary
  - semantic-actions
---

# A dry step is not evidence, and a meaning is not a key

Two decisions taken while building the last milestone before real learned actuation.

---
## Context

[[ADR-023-rehearsal-is-attempt-scoped-authority]] left the system holding an inert
`RehearsalGrant` and nothing that could spend it. Roadmap 21 spends one — into a notebook.

The question the milestone had to answer first was **where the last boundary before physical
input already is**, because a rehearsal-specific input stack would mean the dry run and the live
run were different code, and only one of them would be tested.

---
## Decision 1 — the dry path is the real path with its host swapped

The boundary already exists and [[ADR-005-legal-marco-only]] already names it:

```
marcoexec.Operation → legal Marco source → lexer → parser → graph → compile → runtime → Host
```

`runtime.Host` is the last thing between a compiled program and a computer. `oshost.Host`
presses keys; `recordhost.Host` appends a line. **Nothing else changes.** The dry rehearsal uses
the same lowering, the same encoder, the same compiler, the same runtime and the same frame
scheduler, and the composition root — one line in `cmd/director/rehearsedry.go` — decides which
host is installed.

The alternative, a dry recorder called directly from the rehearsal code, was rejected for the
reason this repository has recorded three times already: a mechanism that production never
invokes passes its own tests forever. A dry run that skipped the compiler would prove the skip
works.

**Consequence.** `internal/director/rehearse` imports no host and cannot build one. It takes a
`directorapi.MarcoRunner`, and `TestTheDryPathCannotReachAHostByItself` holds that.

---
## Decision 2 — the Director lowers a MEANING, never a key

The Director learned that somebody **confirmed**. It did not learn Enter.

[[ADR-013-navigation-is-meaning-not-keys]] settled this for the watching direction:
`internal/platform/navsource` admits a key only where its navigation meaning is conventional, and
marks W/A/S/D and Space as *conditional* evidence because they mean navigation only on a screen
that looked like a set of choices. Lowering a `NavIntent` to a key chord would throw that away on
the acting side and put a binding — a property of a device and an application — inside the
Director.

So `os.marco` gained one capability:

```marco
this exports Navigate. // one navigation MEANING: "up"|"down"|"left"|"right"|"confirm"|"back"|"pause"
```

and `marcoexec` gained `KindNavigate`, which lowers to `do OS's Navigate with "confirm".` The
intent→key table lives in `internal/oshost/navigate.go`, backstage, beside the code that already
knows how to press things.

**The table is not navsource's, reversed.** The two directions are not inverses: watching admits
conditional surrogates, acting cannot. There is exactly one key to press, and pressing W when the
arrow key was meant would drive the car. Each meaning maps to its single unambiguous key and the
surrogates appear nowhere.

`point` is deliberately **not** a navigation meaning. A pointer press needs a position, a position
is not a meaning, and `OS's Click` with a `Point` is the capability that already exists for it.
Naming `point` here would invite a coordinate into a vocabulary whose whole value is that it holds
none.

**This is not a language change.** No new syntax; one new act capability, which is exactly the
route [[Marco-Boundary]] already prescribes for an effect the language cannot yet express.

---
## Decision 3 — a dry step is an engineering artefact, and nothing reads it

`DryStep` is **not** `RehearsalResult`. It does not increment a rehearsal count, create
contradiction evidence, verify a candidate, alter a demonstration, a relationship or semantic
memory, generate learned Marco, or promote anything.

The vocabulary is held to two words — `would_emit` and `refused`. Not "success", not "verified",
not "rehearsed": no key was pressed, nothing was observed afterwards, and the world is exactly as
it was. A dry run that quietly counted would let Marco convince itself a procedure works without
ever having tried it, which is the failure the whole rehearsal design exists to prevent.

---
## Decision 4 — a step is atomic, and the claim comes first

- **The grant is claimed BEFORE anything can be produced.** If attempt construction then fails,
  the grant stays spent. The alternative — claim, fail, retry — turns "one attempt" into "as many
  attempts as it takes to get past the setup".
- **A step whose whole ordered run does not fit its budget is refused before its first input.**
  Half of `down, down, confirm` is not a smaller version of the procedure; it is a different one,
  and it leaves the interface somewhere no demonstration ever went.
- **Every precheck runs before the first effect**: grant state, application, source, route,
  digest, expiry, step classification, input bound, unobservable bound, cancellation, terminality.
  A predictable violation must not be discovered after the first input.

---
## Consequences

- The next milestone changes one thing: the host. Everything above it is already proved.
- `progress_unobservable` lowers — it is a step Marco may take — and arrives carrying its weaker
  marker, which is what stops it being read as arrival.
- A grant's `MaxUnobservable` is now the authorized plan's own longest unobservable run rather
  than the constant, so a plan whose every step lands on a remembered screen carries a budget of
  zero and cannot have a contained step smuggled into it.

## Enforced by

- `TestOneAuthorizedStepIsLoweredToARecordingHost` — the whole chain through the production
  registry: relationship → demonstrations → assessment → judgement → question → yes → grant →
  claim → one step → recorder.
- `TestTheGrantIsClaimedBeforeAnythingIsProduced` — lifecycle ordering; the claim precedes the
  effect and never follows it.
- `TestTheRealHostReceivesTheStepsOwnOrderedIntents` — exact order out of Marco's own runtime,
  including the repeated meaning a sort or a dedup would lose.
- `TestAStepThatExceedsTheInputBoundEmitsNothingAtAll` — atomicity.
- `TestTheDryRefusalMatrix` — every designed blocker, each proving zero calls reached the host.
- `TestClassificationAndExpectationSurviveLowering` — the weaker marker and the expectation the
  next milestone will check both survive intact.
- `TestNothingTheDirectorProducesNamesAKey` — no key, scancode or button anywhere the Director
  can see.
- `TestTheDryPathCannotReachAHostByItself`, `TestTheRecordingHostIsIncapableOfActing`,
  `TestTheLearningLayerCannotReachTheDryPath` — the import guarantees.
- `TestAStepEmissionIsNotEvidence`, `TestAAttemptChangesNothingLearned`,
  `TestAStepEmissionClaimsNothing` — an artefact, not a result.
- `TestADryRehearsalProducesNoRealEffectOfAnyKind` — every call that reached the host was a
  navigation meaning, and no capability that touches a device, a window, a control or the
  clipboard was asked for.
