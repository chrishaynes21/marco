---
type: guide
status: active
updated: 2026-08-10
source_paths:
  - internal/director
  - internal/orchestrator
  - internal/platform/screenhost
  - cmd/director
---

# Auditing Director

Written for an independent adversarial reviewer. It says what the system claims, where each claim
is enforced, and — deliberately — **where the evidence is thinnest**. Pointing at our own weak
spots is cheaper than having them found by a user.

Start at [[Reading-Marco]] if the language is unfamiliar, then [[Director]] for the map and
[[Learned-Plays]] for the chain this note is mostly about.

## How to read a claim here

Every invariant below is stated as something that must be **false** to break it. The fastest way
to test one is not to read the code: it is to break it deliberately and check that a named test
fails for the right reason. That method, and why this repository trusts nothing else, is
[[Wiring-Tests]].

> A mutation that dies for a reason you have not looked at has not been checked.

Three failure modes we have actually shipped, and would expect an auditor to hunt for more of:

- **A complete mechanism production never invokes.** Three recorded cases. A unit test of a
  helper proves nothing; the test must enter through the production constructor.
- **Code that compiles and does not behave.** A guard whose `or?` arm returned, letting execution
  walk on to the effects anyway. Source-reading and compile-passing tests both approved it.
- **A test that cannot fail.** A cross-application fixture that switched the *selector* and not
  the window, so every session still reported the same application.

## The invariants

### Language

1. **Core v1 is unchanged.** Everything Director generates is Core v1 composition. Enforced by
   `internal/spectest` — generated routes are checked against Core's vocabulary and normative
   examples are compiled.
2. **A generated play names no backstage concept.** No subject ids, digests, verdicts, counts,
   window handles, or session references. `TestALearnedPlayStaysInsideCoreAndOffTheBackstage`,
   `TestGeneratedMarcoNamesNoBackstageConcept`.
3. **Nothing reaches the desktop except as legal Marco source** — [[ADR-005-legal-marco-only]].
   One boundary, no second path.

### Authority

4. **Evidence is not authority; a question is not authority.** Only a yes to `AskRehearse`
   creates a grant, and the grant is single-use and attempt-scoped —
   [[ADR-023-rehearsal-is-attempt-scoped-authority]].
5. **Resolution is not permission.** Knowing which play the user means does not perform it —
   [[ADR-029-resolution-is-not-permission]].
6. **Naming a screen cannot act.** The Screen host has three read methods and no way to press a
   key, run a route, write memory or grant anything.
7. **Authority is never remembered.** `TestPermissionIsNotRemembered`.

### The learned play

8. **A play cannot begin on the wrong screen.** Zero effects, every time, including ambiguous,
   unobservable, unrecognised, missing-recogniser and wrong-application —
   [[ADR-030-a-play-says-where-it-begins]].
9. **A play cannot succeed without arriving.** Exactly one `this is ok!`, inside the arrival
   check — [[ADR-032-a-play-says-where-it-ends]].
10. **The sidecar enforces nothing.** Editing a condition out of the source removes it, and Marco
    stops claiming it verified the file.
11. **Nothing hidden is needed at invocation.** No `ProcedureCandidate`, no `RehearsalEvidence`.
    Grep the orchestrator, screen host, runtime and `cmd/marco` for either; only a test comment
    should match. **This is the single most valuable claim to attack.**

### Memory and privacy

12. **The only durable arbitrary text is a screen name a user typed for that purpose.** One
    constructor, `observe.UserSuppliedScreenName`, with one production call site at the request
    boundary — [[ADR-031-the-user-names-the-stage]].
13. **Forbidden paths:** OCR → `Called`, accessibility label → `Called`, shadow-trace text →
    `Called`, window title → `Called`. No perception or plugin package may reference `ScreenName`.
14. **A name means one screen in one application.** Ambiguity resolves to nothing rather than to
    the nearest match.
15. **A judgement is recomputed, never read back** — [[ADR-021-a-judgement-is-recomputed-not-recorded]].
16. **No confidence floats.** Discrete verdicts and named refusal reasons, everywhere.

### Answers

17. **An answer binds to the durable subject the question was about**, never to ambient screen or
    application, including across a restart and across an application switch.
18. **An invalid or duplicate answer changes nothing and leaves the question open.** Closing a
    question the user did not successfully answer removes the only prompt they will get.
19. **The closed vocabulary stays closed.** Only `AskNameScreen` carries text; every other
    question is answered with yes, no, or not-now, and the generic path refuses a naming question.

## Where the evidence is thinnest

Honest list. These are where we would look first.

**The ordering of the arrival check is proven by construction, not by a killed mutation.** The
test world moves only when the recording host sees the final navigation, so a check placed before
the effects would see the source and fail; the success test also asserts the world was observed
twice. But two attempts at a faithful *"check the destination before the effects"* mutation were
both rejected by the Marco compiler for unrelated reasons, so we have never watched the right test
fail for the right reason. **Attack this first.**

**Four kills in the Roadmap 32 gate came from the compiler, not from a test.** Returning `ok!`
early and de-nesting the effects both produce source Marco rejects (*unreachable code after
terminal return*). That is a genuine structural property of the shape, and it is a weaker form of
evidence than a behavioural test failing. If the generator ever emits a variant the compiler
tolerates, those guarantees are unproven.

**Two Roadmap 31 mutations survived and were kept as defence-in-depth.** The `s.Called != ""`
guard inside `ReviewScreenName` and the cross-record dedup in `ProposeScreenName` are both
unreachable through today's only call site — the trigger prevents the first, the one-open-question
budget prevents the second. Unreachable code is a recorded failure mode here
([[Wiring-Tests]]), so a reviewer may reasonably argue they should go.

**The naming question is ephemeral; the subject is durable.** The proposal ledger does not
persist. After a restart the question is re-surfaced by recomputing the lowering, which is the
documented model — but it means an outstanding question's *identity* does not survive, only its
*need*. Confirm that is acceptable rather than assuming it.

**The one-open-question budget competes with naming.** A naming need can be suppressed by an
unrelated open question. Deterministic and intended, but it means "Marco did not ask" has two
explanations and only one of them is the policy working.

**Live behaviour is unproven by construction.** Every test here is deterministic and dry. No live
run has been performed. Timing, focus loss, real recognition latency and partial input are
entirely unexercised — see the note below.

## What this system has never done

It has never sent input to a real application outside a rehearsal. The whole chain above is proven
against controlled hosts and fixtures. `LIVE_MARCO_MOMENT: READY` means the architecture is proven
end to end and the remaining questions are environmental — not that anything has been observed
working in the world.

## Related

- [[Director]], [[Learned-Plays]], [[Wiring-Tests]], [[Decisions]]
