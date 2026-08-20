---
type: decision
status: accepted
date: 2026-08-20
supersedes: []
affects:
  - demonstrations
  - passive-observation
  - service
  - visibility
  - invocation
source_paths:
  - internal/director/learn
  - internal/voicelearn
  - internal/director/service/protocol.go
  - cmd/director/learncmd.go
  - cmd/director/learnsessionwiring.go
  - cmd/director/learnview.go
  - cmd/marco/main.go
  - cmd/marco/learnui.go
  - plugins/overlay/acts.go
---

# ADR-086 — one acquisition, one word, one request

[[ADR-048-learn-teach-and-do-are-three-different-sentences]] decided the vocabulary and
deliberately licensed no rename: Roadmap 34 was open and mid-flight, and a spelling change mixed
into an unfinished end-to-end transition would have destroyed the ability to tell a behaviour
change from a spelling change. Roadmap 34 closed. This is the rename it deferred, carried out as
one change once nothing was mid-flight.

**This ADR does not supersede ADR-048.** ADR-048's decision is unchanged and unchallenged — the
three sentences, the reservation, the architectural consequence for visual grounding. What that
ADR recorded as a product-vocabulary intention now also holds in the code. `supersedes: []` is
correct, and calling it a supersession would tell a future reader to stop at this page when the
reasoning they need is on that one.

## The word, vertically

```
LEARN   The person acts.   Marco watches and acquires.     <- the acquisition flow
TEACH   The person acts.   Marco guides them through it.   <- RESERVED, nothing acquires under it
DO      Marco acts.        The person delegates.
```

The acquisition flow is spelled `learn` from the verb a person types to the coordinator that runs
it: `marco learn` and `director learn`; the overlay's `learn <name>` and `narrate learn <name>`;
`internal/director/learn` and `internal/voicelearn`; `Runtime.learn` holding a `learnSession`,
`learnPasses`, `learnTail`, `learnSessionView`; `IntentLearn`; `ReferentLearnStart` and
`ReferentLearnDestination`; and the files that carry them — `learncmd.go`, `learntail.go`,
`learnsessionwiring.go`, `learngrounding.go` and their tests.

**Half a rename would have been worse than none.** Renaming only the product surface leaves the
collision in place and hands it, undiminished, to whoever builds Teach — who then has to
disentangle a word from the flow that is not theirs while also designing the flow that is. The
cost of doing it now is one large mechanical diff; the cost of doing it then is the same diff plus
a half-built feature on top of it.

## The two request types were one event described twice

`ObserveTeach` and `ObserveLearn` were not peers, and that is the whole finding.

`ObserveTeach` was the session lifecycle: a name, a `windowref.Selector` target, an actor and a
verb, and the switches (`Finish`, `Surface`, `Dry`, `Evidence`). It was the only thing that could
start anything. `ObserveLearn` was the control surface's vocabulary — Start, Stop, Try, Called,
Skip, Cancel, Places, Rename, Remember, Watch, Unwatch, and a question/session/answer triple. It
could start nothing by itself: the panel's Start was **translated** into the other type's start,
its Stop into the other's `Finish` or `Cancel` depending on the phase the surface could see.

A facade over one type, in a second vocabulary, is not a second event. It is one event with two
names — and one of those names spent the reserved word.

**There is one acquisition request now.** `ObserveLearn` carries both the control surface's verbs
and the session's own configuration (`Target`, `Actor`, `Verb`, `Surface`, `Dry`, `Finish`,
`Evidence`), and `ObserveQuery.Learn` is the only field acquisition arrives on.
`ObserveQuery.Teach` is gone.

**`Surface` says which surface is asking, and therefore which account comes back.** It carries two
facts that are the same fact seen twice: the window this request arrived from is Marco's own, so
the buttons a person presses there are not mistaken for the task they are demonstrating; and a
control surface and a person at a terminal want different readings of one session. That is the
only thing the entrances differ by — a panel presses buttons and says so, `director learn` names a
window and a sentence and does not.

**`Watch` and `Evidence` were kept apart on purpose.** Each type had a field called `Watch` meaning
something different: Light Mode on one, "show me what is underneath" on the other. Merged under one
name, `director learn --watch` would silently have started Light Mode. Two names, no collision.

**ProtocolVersion 7 → 8.** The wire shape changed: a field is gone and another grew. A version
mismatch fails explicitly rather than being negotiated away, which is the point — a client and a
service that disagree about the shape of a command produce the wrong thing happening on the
desktop, not an error.

## The compatibility aliases

`teach` still answers, **undocumented**, in exactly the places a person's muscle memory and an
existing script would reach for it:

- `marco teach` and `director teach` — one `case "learn", "teach":` on each CLI, routed to the
  same function.
- the overlay's `teach <name>` and `narrate teach <name>`.

An alias is documented **once, explicitly, as an alias** — in [[Glossary]] and in the command
reference. The two failure modes are documenting it in ten places as though it were canonical, and
documenting it nowhere at all; both strand somebody.

The narration parser is the one place a spelling was *replaced* rather than aliased: its Cancel
synonym is now `"stop learning"` and `"stop teaching"` is gone. Spoken vocabulary is a closed list
matched exactly, not a prefix a script depends on, and nothing outside the microphone sends it.

Three spellings stay for a different reason and are not aliases:

- **`teach_start` and `teach_destination`** are `ReferentRole` values on the wire. An out-of-tree
  surface reads them; renaming a string nobody sees would have been a protocol change bought with
  nothing. [[Visibility]] is where they are written down, and it must match the constants exactly.
- **`dispatch.IntentLearn` still has the wire VALUE `"teach"`.** An out-of-tree resolver reached
  through `$MARCO_RESOLVER` answers with it, and `plugins/llama/README.md` documents that the
  constant was renamed and the string was not. Prose around it says Learn; that is correct, not
  inconsistent.
- **`routes.KindTaught`** keeps its name and presents as *Recorded*, because *Teach* is spent on
  the other direction of travel — [[ADR-081-a-durable-behaviour-is-a-play]], [[Plays]].

## What this removes from ADR-081's reasoning

[[ADR-081-a-durable-behaviour-is-a-play]] argued that `internal/routes` may keep its name because
"the repository already runs this exact divergence for Learn/`teach` without confusing anybody".
It no longer does. **ADR-081's decision is unaffected** — the noun is Play, the package is
`internal/routes` — but its precedent is withdrawn, and the argument now has to stand on its own,
which it does: a package name is a thing no person ever reads, and nothing else needs the word
*routes*. `teach` was the opposite case, and is the counter-example rather than the precedent.
[[Plays]] carries the corrected argument.

## Considered and rejected

- **Rename the product surface only, and leave the code spelling `teach`.** The position ADR-048
  took, correctly, for the duration of Roadmap 34. It stops being correct the moment the milestone
  closes: the collision does not decay, it waits, and it waits inside the feature that will need
  the word most.
- **Keep both request types and rename one of them.** Two APIs for one event, agreeing forever by
  hand. The facade was the evidence that they were never peers; preserving it would have preserved
  the duplication and renamed the symptom.
- **Rename everything that spells the word, blindly.** This is the option that destroys the record.
  ADR-043, ADR-044 and ADR-045 are titled in the vocabulary of their date and are corroborating
  evidence for ADR-048's central claim; ADR-048's own *"What was wrong"* and *"Considered and
  rejected"* quote the word in its wrong sense **as the evidence**; ADR-079 and ADR-080 quote live
  transcripts a person actually saw. A sweep that rewrote those would leave a vault that agrees
  with itself and can no longer explain why. They were left as written, with an editorial banner
  where a reader could be misled.
- **Retire `marco teach` outright.** It is what `E2E.md`, the overlay's spawn sites and every
  existing script have. Removing it buys tidiness and costs a stranded user.

## Consequences, including the costs

- **A large mechanical diff across `cmd/`, `internal/` and the vault**, landed as one change so
  that a behaviour change is still distinguishable from a spelling change — which is exactly the
  property ADR-048 protected by making it wait.
- **The alias is a second spelling that has to keep working.** It is pinned by test rather than by
  discipline, and it retires with the other legacy verbs, not on its own.
- **Not everything that spells the word was in scope, and the remainder is real.** Recorded here
  rather than discovered later:
  - `internal/orchestrator`'s `Teach`, `TeachAuto` and `TeachVoice` — the legacy
    record-and-simplify loop that `marco learn` still drives — keep their names, and sit outside
    the governance test's file list.
  - `cmd/marco`'s no-name usage line still prints `usage: marco teach "<name>"`, which shows a
    person the alias as though it were canonical.
  - `$MARCO_NO_TEACH` is still set on the overlay's spawn environment, and read by nothing in the
    engine.
  - the overlay's `marcoVerbs` highlight table lists `teach` and not `learn`, so the canonical
    verb does not render as a verb in the command line.
  - a number of Teach-spelled **test function names** survive in `cmd/director`. They are cited by
    ADRs and cost a reader nothing, but they are why the governance test names files rather than
    sweeping the tree.
- **The governance test is a file list, not a walk.** That is honest about what it covers, and it
  is the reason the remainder above is a list rather than a red build; a file added to the
  acquisition path is not automatically covered, and adding it to the list is part of adding it.

## Enforced by

- `cmd/marco` — `TestNoLiveAcquisitionCodeIsNamedTeach`: walks the acquisition packages and the
  product surfaces and refuses any identifier spelling the reserved word, with the compatibility
  aliases named one by one so they cannot grow silently. It deliberately does not police prose, so
  the ADR corpus stays legible. Mutation: reintroduce any Teach-spelled acquisition identifier.
- `cmd/marco` — `TestTheLearnVerbAnswersToItsOldName`: `marco learn` and `director learn` are the
  verbs, and `teach` still reaches the same function on both. Mutation: drop either arm of either
  case, or rename the entry point back.
- `cmd/director` — `TestOneAcquisitionRequestServesBothSurfaces`: the router chooses the account by
  `q.Learn.Surface` and both roads reach the one lifecycle; `ObserveTeach` is gone and the
  observation query carries no second acquisition field; the panel sets `Surface` on every verb
  from its one sender. Mutation: bring back a second request type, or stop the panel identifying
  itself.
- `internal/plays` — `TestEveryKindIsPresentedAsItself`: a demonstrated play may not present as
  "Taught". The reservation, held against a user-visible word.
- `internal/director/service` — `TestProtocolVersionMismatchFailsExplicitly`: a build that
  disagrees about the wire shape fails loudly rather than negotiating.

## Related

[[ADR-048-learn-teach-and-do-are-three-different-sentences]] ·
[[ADR-081-a-durable-behaviour-is-a-play]] ·
[[ADR-043-teaching-is-two-passes-not-a-new-capture]] ·
[[ADR-044-a-teach-attempt-is-one-episode]] ·
[[ADR-045-teaching-is-a-section-of-the-playbill]] ·
[[Demonstrations]] · [[Passive-Observation]] · [[Plays]] · [[Visibility]] · [[Service]] ·
[[Glossary]] · [[Roadmap]]
