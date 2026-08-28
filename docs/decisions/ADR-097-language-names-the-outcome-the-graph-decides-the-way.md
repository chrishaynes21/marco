---
type: decision
status: accepted
date: 2026-08-27
supersedes: []
affects:
  - learned-plays
  - semantic-memory
  - demonstrations
  - passive-observation
  - invocation
source_paths:
  - cmd/director/goalresolve.go
  - cmd/director/goalresolve_test.go
  - cmd/director/perform.go
  - cmd/director/reach.go
  - cmd/director/observerecent.go
  - internal/director/observe/goal.go
  - internal/director/learn/learn.go
  - internal/director/learn/say.go
  - acceptance-36d.ps1
---

# ADR-097 — Language names the outcome, the graph decides the way

## The four layers, and they are four

```
LANGUAGE     the words somebody said
GOAL         the outcome those words name
DESTINATION  a subject in the semantic graph
GRAPH        the transitions Marco knows
PLAN         the route chosen NOW, from where they are standing
```

The graph tells Marco **how the computer can move**. A goal tells Marco **what outcome the person
wants**. Language tells Marco **what the person means**. The planner decides **how to get there
today**. Execution proves **which of those edges Marco can actually walk**.

[[ADR-096-observe-and-learn-are-two-doors-into-one-graph]] settled the two layers below this one:
one canonical graph, taught through either door, with the route selected at invocation. This ADR
is about the layer above — where a phrase becomes a destination — which was the thinnest part of
the stack.

## What was already right

`observe.Goal` is `{Name, Application, Subject}`. It has **no start** — not an empty start, no field
one could go in — settled by [[ADR-056-a-goal-is-a-destination-not-a-route]]. `PerformGoal` resolves
the name to a destination, brings the application forward, takes a **fresh** look at where the
person is, and asks `observe.PlanToGoal` over the canonical topology. The route belongs to the
moment of asking.

So a goal already could not own a route, and none of that changed.

## The three things that were wrong, all in the language step

**One phrase meaning two things resolved by sort order.** `PerformGoal` searched every application
holding goals, sorted, and took the first match. Measured: two outcomes named "open settings", one
in Windows Settings and one in a mail client, resolved to `mail` — because `m` sorts before `s`.
Nothing said so. The person who taught it in Settings would have had their mail client brought
forward and a route walked in it.

Deterministic is not the same as right. A sort order is not evidence about which afternoon somebody
is having, and **a wrong answer nobody was told about is worse than a question.** More than one
application answering is now `ambiguous_outcome`: a refusal, before anything is brought forward,
naming both and naming the way through (`--application`).

**A name that stopped meaning what it meant said nothing.** The store REBINDS a reused name rather
than refusing it, deliberately and for a reason measured live on 2026-08-17: a goal left behind by a
*failed* learn held its name hostage, so refusing punished the person for Marco's own earlier
failure. That rule stands and this ADR does not touch it.

What was missing was the saying. Measured: somebody taught "mouse settings" for one screen, later
taught it for another, and was told

> "I saw what you did. Learned it as mouse-settings. 2 screen(s) I hadn't seen before are now ones
> I know. Still watching."

Not one word about the name having meant somewhere else. They would believe they had two commands,
and the old one would silently be the new one. **Rebinding is still what happens. Silence is not** —
`observe.ReboundFrom` is read before the write on both Learn paths, and the answer travels to the
person.

**The diagnostic and the performer were two loops.** `director reach` exists so somebody can ask
what Marco would do without Marco doing it, which is only worth having if it is the *same* answer.
It was a second similar-looking loop: name-only, first-match, unable to honour a supplied identity
and unable to report that the words meant two things. Somebody debugging "why did it go to the
wrong place" would have been told the right place by a diagnostic that could not see the defect.
There is now one `resolveGoal`, and both call it.

## Goal identity, precisely

A goal is identified by `(application, name)` and points at a `Subject`. Nothing else participates:
not the historical route, not the starting Place, not a Learn or Observe session, not a timestamp,
not a coordinate, not a presentation size, not a candidate handle. The type has no field for any of
them.

**Aliases are the representation the repository already had.** Two names for one destination are two
goal records pointing at one subject — two user-facing names, one outcome, no duplicate topology and
no duplicate destination. There was nothing to build; what was missing was a gate saying both work
and neither weakens the other. Forgetting one leaves the other, and leaves everything they both
point at, because the graph belongs to neither.

**Identity beats words.** A registered play carries its own provenance — the destination subject it
was learned for — and `namesOutcome` matches that exactly and case-sensitively before any phrase is
considered. That is the path the product uses when somebody says a learned play's name, and it can
never be ambiguous: subject ids are content-derived per application.

## Four refusals, and they are four

| | what it means | where somebody goes next |
|---|---|---|
| `not_learned` | I have never been taught those words | teach it |
| `ambiguous_outcome` | I know two of those | say which application |
| `no_known_route` | I know it, and I can't get there from here | show me a way, or move |
| `position_unknown` | I don't know where you are | let me look again |

Collapsing any two of these would make the difference invisible exactly when it matters. Nothing
approximates: a phrase one letter off resolves to nothing, and there is no search for a vaguely
similar label. Deterministic exact matching is the policy; approximate matching is a decision
nobody has made.

## What is deliberately NOT here

No LLM router. No NLP infrastructure. No second graph, second planner or second store. No automatic
goal naming from ambient watching — Observe learns Places, Targets and Edges, and *naming an
outcome is a person's act*. No conversion of every observed label into invocation language: the
graph can know a hundred Places without any of them becoming a command.

No foreground-preference disambiguation either. Reading which application is in front and quietly
preferring its goal is a real and tempting idea; it is also resolution CONTEXT being smuggled into
identity, and refusing with the choice named is the honest first behaviour. Recorded as a follow-on.

## KNOWN FOLLOW-ONS

1. **Ambiguity refuses rather than asking interactively.** The refusal names the candidates and the
   flag that answers it, which a client can turn into a choice. Putting the question in front of
   somebody through the existing clarification machinery is a surface decision this did not make.
2. **Contextual preference is unbuilt.** See above. If it lands, it must be resolution context and
   never part of what a phrase MEANS.
3. **The generated `.marco` still shows the historical demonstration**, not the route Marco would
   plan now. Unchanged from [[ADR-096-observe-and-learn-are-two-doors-into-one-graph]]; the artifact
   owns nothing and the execution path is graph-native, so this is a readability debt.
4. **Planner ranking became evidence-aware in
   [[ADR-098-the-planner-prefers-better-evidence-and-says-why]].** The goal layer remains the
   obvious place somebody would be tempted to put ranking, and it must not go there: the goal
   hands over a destination and stops.
5. **`reach` plans from where somebody was LAST seen standing** unless `--from` names a source
   (added by ADR-098), and says which it used. `PerformGoal`
   takes a fresh look. The two agree about what a phrase MEANS, which is what this ADR made true;
   they can still disagree about where the person is, and the diagnostic labels its answer with the
   session it came from.
6. **The semantic store has no delete path**, so forgetting a name cannot reach the graph. Still
   structural, still worth the gate.

## Enforced by

- `cmd/director` — `TestOnePhraseInTwoApplicationsIsAQuestionNotAGuess`;
  `TestNamingTheApplicationResolvesTheAmbiguity`; `TestASubjectIdentityIsNeverAmbiguous`;
  `TestPerformingRefusesAmbiguousWordsBeforeTouchingAnything` (the production method, refusing
  before the foreground); `TestUnknownWordsAndUnreachableOutcomesAreDifferentRefusals`;
  `TestAskingWhatAPhraseMeansAnswersLikePerformWould`;
  `TestTheDiagnosticReportsAmbiguityRatherThanChoosing`;
  `TestTeachingANameAgainSaysWhatItUsedToMean`; `TestTeachingTheSameThingAgainIsNotARebinding`;
  `TestTwoNamesForOneDestinationBothResolve`; `TestForgettingOneNameLeavesTheOther`;
  `TestWordsNobodyTaughtResolveToNothing`; `TestResolvingWordsTouchesNothingThatActs`;
  `TestANameMeansTheSameOutcomeAfterARestart`.
- `internal/director/learn` — `TestReusingANameSaysWhatItUsedToMean`;
  `TestLearningRecordsTheDestinationAsAGoal`; `TestAGoalNameConflictRefusesHonestly`.
- And, holding the layers below, [[ADR-096-observe-and-learn-are-two-doors-into-one-graph]]'s
  `cmd/director/onegraph_test.go` — in particular
  `TestTheRouteIsChosenWhenSomebodyAsksNotWhenTheyDemonstrated`,
  `TestAnotherWayInFoundLaterNeedsNoSecondLearn` and
  `TestAShorterWayFoundLaterIsNotBlockedByTheDemonstration`, which are the same-goal-different-route
  claims this ADR sits on top of.

## Related

[[ADR-096-observe-and-learn-are-two-doors-into-one-graph]] ·
[[ADR-056-a-goal-is-a-destination-not-a-route]] ·
[[ADR-080-a-learned-play-is-asked-for-from-anywhere]] ·
[[ADR-029-resolution-is-not-permission]] ·
[[ADR-069-a-name-is-authored-and-can-be-taken-back]] ·
[[ADR-005-legal-marco-only]] ·
[[Learned-Plays]] · [[Invocation]]
