---
type: reference
status: accepted
date: 2026-10-30
source_paths:
  - internal/routes/registry.go
  - internal/routes/bindings.go
  - internal/routes/steps.go
  - internal/resolveroute/resolveroute.go
  - internal/nlu/nlu.go
  - internal/invoke/invoke.go
  - internal/orchestrator/keep.go
  - internal/orchestrator/rescope.go
  - cmd/marco/intake.go
  - cmd/marco/assistant.go
  - cmd/director/resolveroutewiring.go
  - plugins/studio/routines.go
---

# Also answers to — a routine's other names

Traced at `7a2372d`. The delivery is seven lines:

- add and remove "Also answers to" phrases on one exact routine;
- Director resolves an exact alias without requiring Ollama;
- aliases inherit the routine's availability, and grant no execution;
- normalized collisions clarify rather than pick a winner;
- rename and rescope keep the aliases attached, and deletion removes them;
- the canonical routine is displayed before Play;
- several phrases for a toggle claim nothing about the state it leaves behind.

This is what exists, the rules those seven become, the three decisions that had to be settled, and
what each rule is held by.

**Both increments are now built.** This note is the trace and the rules they were built to; what
was built, and what holds each rule in place, is
[[ADR-275-a-routine-may-answer-to-more-than-one-phrase]]. The three decisions in §3 were taken on
the recommendations below and are recorded there; two of them earned their keep immediately, by
refusing fixtures written for the tests themselves. No live execution and no acceptance run were
taken.

## 1. What exists

**One name per routine, and it is the file.** A route IS its scope and its slug —
`global/<slug>`, `<app>/context/<slug>`, `<app>/focus/<slug>` — and `routes.Slug` folds case,
punctuation, surrounding quotes and runs of whitespace onto that one durable identity.
`Registry.Resolve(app, name)` slugs the words and walks four scopes in priority order: the
foreground application's in-place routine, its anywhere-in-application one, any other
application's anywhere-in-application one, then global. There is no second name anywhere, no
alternate spelling, and nothing in `spec/`.

**The normalization the product wanted was already there**, and it is the reason an alias can be
exact rather than fuzzy: `TestNormalizationIsTheSameFromEveryEntrance` and
`TestAnApostropheDoesNotHideAPlay` hold one answer to "what did these words reduce to" for every
entrance. See [[Invocation]].

**Two durable name tiers, and one door.** `invoke.Decide`'s arm 4 is the exact play lookup above;
arm 5 is a routine somebody demonstrated by hand, resolved through a different store
(`demo.Store.Sequence`) and performed by a different thing. Arm 5 sits AFTER arm 4 on purpose,
which is the compatibility guarantee this slice inherits: *a name that runs something today keeps
running it* (`TestAPlayStillWinsOverADemonstrationOfTheSameName`).

**Director's resolver already has the offline arm and the clarification.**
`internal/resolveroute` builds slug-shaped LABELS for the eligible routines and hands them to
`dispatch.Decide`, whose FIRST arm is `internal/nlu`'s exact match — deterministic, offline, free,
reported as `exact` and as `not-asked` even with a resolver configured. Two of its three arms need
no model at all. And a name more than one eligible routine answers to is already a QUESTION rather
than a near miss: `sharedNames` counts names by `nlu.Key`, and `whichOnesAnswerTo` names every
routine that answers to it, in the product's own words for a scope, selecting nothing. See
[[ADR-267-director-owns-route-resolution]] and
[[ADR-265-say-what-you-want-and-studio-offers-the-routine]].

**The sidecar convention, and the four operations that carry one.** A routine keeps its
demonstration at `<slug>.rec.json`, its provenance at `<slug>.origin.json`, its step account at
`<slug>.steps.json` and its anchors at `<slug>-anchor-*.png`. `Registry.Rename` moves each one
beside the source, `Registry.Delete` removes them, and a MOVE uses `DeleteSource` then
`DeleteLeftovers` in that order so that "original retained" is true when the removal fails — and
`DeleteLeftovers` reports what it could not sweep, because litter under a slug is what the next
routine saved there would inherit. See [[ADR-271-a-rescope-is-a-reviewed-move-between-identities]]
and [[ADR-273-a-routine-writes-down-what-it-does]].

**The closest thing to a phrase store is `bindings.json`** — one library-level file mapping a
leader hotkey and an application to a command. It is not a name: it is a key, and the command it
holds is text that gets resolved.

**And the discipline for an alias is already written down**, for the CLI's own
`teach`-answers-to-`learn`: an alias is documented once, explicitly, as an alias, and the named
failure mode is *showing a person the alias as though it were canonical*
([[ADR-086-one-acquisition-one-word-one-request]]). This slice is that rule in a second place.

**What does not exist:** any store of alternate phrases, any reverse lookup from a phrase to a
routine, and any place that parses a routine's name for meaning.

## 2. The rules

**R1. An alias is a PHRASE, and the routine is still the file.** No second `.marco`, no copy of
the source, no second identity in the library, and no new recording path. One routine, one place
in the library, several ways in. A surface that listed an alias as a routine would have made the
library lie about how much is in it.

**R2. The store is a sidecar: `<slug>.aliases.json`.** Three fields and no history:

| field | is |
| --- | --- |
| `version` | the format, so an unreadable future one is refused rather than half-understood |
| `route` | the identity these phrases belong to — `<app>/context/<slug>` and so on |
| `phrases` | what the routine also answers to, stored VERBATIM as the person typed them |

A sidecar rather than a library-level file for the reason the account is one: the four lifecycle
operations already carry a sidecar beside its source, and a shared file would need every one of
them to rewrite a row under a lock while something else is reading it. And the phrases are stored
verbatim because they are what somebody chose to call it — the normalization is applied when they
are COMPARED, not when they are kept, so the list a person reads back is the list they wrote.

**R3. And it carries NO digests, which is the deliberate difference from the step account.** The
account records a rendering basis, so it is bound to the bytes it rendered to and a moved source
digest means something. A phrase is a name for the routine's IDENTITY: editing a step,
re-simplifying, regenerating or rebuilding all change the source and change nothing about what the
routine is called, and a sidecar bound to the source would have quietly dropped somebody's phrases
every time they fixed a wait.

**R4. It is bound to the identity instead, and the identity is what moves.** A rename or a rescope
rewrites `route` as part of the move, because the file is now beside a different identity. A
sidecar whose `route` is not the route reading it is NOT believed: its phrases are not offered and
the card says the file is there and describes something else. Fail-closed in that one direction,
for the same reason `origin.json` refuses an inheritance — the cost of believing an orphan is a
phrase that runs a routine nobody attached it to.

**R5. The slug always wins — and the door does not resolve a phrase at all.** CORRECTED AFTER
REVIEW. As written, this rule put the alias lookup at the end of `Registry.Resolve`, which keeps a
routine's own name safe and misses the other half: a phrase two routines answer to was then
selected by scope priority and performed, while Director asked which. *Exact* and *unambiguous*
are different properties, and the second is a fact about the whole library that a door with
nowhere to ask may not settle. `Registry.Resolve` is therefore unchanged from before phrases
existed, `AnsweringTo` lists every routine a phrase answers for, and Director chooses. The cost is
stated: with Director unavailable a phrase reaches nothing, where a routine's own name still runs.

**R6. Exact, offline, through one normalization.** The comparison is `routes.Slug` on both sides
— the phrase and the words — so an alias is as exact as a name and needs nothing running beyond
Director itself. In Director's resolver an alias is simply another LABEL for the same candidate, which
means `dispatch.Decide`'s first arm answers it: `exact`, `not-asked`, no plugin, no model server.
A library whose routines all answer to several phrases still resolves with Ollama stopped.

**R7. Availability is inherited, never granted.** An alias is offered exactly when its routine is,
through the rules that already decide that: `resolveroute.Eligible`'s three scope rules and
`Runnable`. An alias of an in-place routine is not offered when nothing is known to be in front;
an alias of a routine whose every press would refuse is not offered at all. There is no alias-only
scope, and an alias cannot widen one.

**R8. And resolution is still not permission.** A request that arrived by an alias crosses
`orchestrator.Authorize` like every other one ([[ADR-029-resolution-is-not-permission]],
`TestExactPlayMatchDoesNotBypassAuthority`). Nothing here touches the Play grant: one press, one
`marco do`, one run.

**R9. The canonical routine is displayed before Play.** Whatever found a routine, what a person is
shown is the routine's OWN name and scope, with the phrase that answered named as a phrase beside
it rather than in place of it. `[route] ` already says which play the words became and keeps
saying the canonical name. This is the failure mode ADR-086 named, in the one place where it would
cost more than a confusing sentence: a person who is shown only the phrase they typed cannot tell
which of two namesakes is about to press something.

**R10. A collision clarifies, and never picks a winner.** A phrase more than one eligible routine
answers to selects NOTHING and asks which, through the clarification that exists — extended so
that `sharedNames` counts phrases as names and `whichOnesAnswerTo` lists a routine that answers by
an alias as readily as one that answers by its slug. Three shapes, one rule:

- an alias against another routine's slug;
- an alias against another routine's alias;
- an alias against its own routine's slug, which is not an error but a phrase that adds nothing,
  and is said rather than stored.

**R11. An add or a remove names the routine and the phrase, never a position.** The step-editing
slice learnt this the hard way: an operation stated against a list somebody saw, applied to a list
that has changed, is somebody's intention landing on a different thing with nothing afterwards
able to catch it. A phrase list needs no basis token because a removal can name the PHRASE, which
is its own identity — there is no index to misapply. What the write still needs is the ordinary
version binding: the routine must still be the one that was shown, and the phrase list must be the
one that was read, or the write refuses and says which.

**R12. An alias claims nothing about an outcome.** Several phrases for a toggle are several ways
to ask for the same press, and none of them is a statement about the state that press leaves
behind. So nothing parses a phrase: no "on" or "off" is read out of it, no phrase becomes a
verification, and no phrase changes what the routine does or what it says it did. The routine's own
account — its `log "<name>: done"`, the seven outcome words, a guarded contract's sentence about
what could not be checked — keeps the canonical name and is untouched. Adding both "mute" and
"unmute" to one toggle is allowed, and means only that either phrase presses it.

## 3. The three decisions

**D1. Does a colliding phrase refuse at the moment of adding, or is it accepted and clarified?**
R10 is about resolution and settles what happens to a collision that EXISTS. It does not settle
whether one may be created on purpose. **Recommended: refuse at the add, naming the routine it
collides with, and clarify at resolution for the ones an add cannot see.** The add is the single
moment somebody can choose a different phrase, and a phrase whose only possible answer is a
question is worth not creating; but an add cannot see a rename, a move or a routine saved later,
so resolution must clarify regardless. Refusing at the add is therefore a courtesy and not the
guarantee — which is worth saying, because a product that refuses at the add often grows a
resolution path that assumes it cannot happen.

**D2. One normalization, or two?** The door reduces with `routes.Slug`; the resolver compares on
`nlu.Key`, which also drops hyphens and stop words — so `settings-in-ark` and `settings-ark` are
two phrases at the door and ONE at the resolver. **Recommended: store verbatim, match the door on
`routes.Slug`, and check uniqueness on BOTH at the add**, refusing a phrase that either
normalization cannot tell from an existing name. A phrase that is distinct at the door and
ambiguous in the resolver is a phrase whose meaning depends on which surface was asked, and that
is the shape of defect this repository keeps finding.

**D3. Does this reach a demonstration in the demo store, or routes only?** **Recommended: routes
only, stated as a boundary.** Arm 5's artifacts have their own name field, their own
`Sequence(name)` lookup and their own store, and the lifecycle in R4 — rename, rescope, delete — is
a route lifecycle. Reaching both stores in one slice would mean two alias mechanisms or one that
spans two stores, neither of which the seven lines ask for. Said here so it is a boundary rather
than an omission discovered later.

## 4. What each operation does to the sidecar

Every operation that touches a routine, with no blank rows — the lesson of the account's §6.

| operation | the phrases |
| --- | --- |
| rename | carried beside the source, `route` rewritten |
| rescope (a move) | written at the destination and the destination verified BEFORE the original is removed, which is ADR-271's rule |
| delete | removed |
| a move's second half (`DeleteLeftovers`) | removed, and reported as residue if it cannot be |
| replace, regenerate, Simplify, a step edit | UNTOUCHED — none of them changes what the routine is called (R3) |
| saving over a routine (`Keep`'s collision) | the sentence says the phrases go with it, as it already says about a stored step list |
| a new routine saved under a slug an alias answers to | the collision question names it (D1) |
| the legacy `marco edit` save | untouched, like the account: it knows nothing about either sidecar and sweeps neither |

## 5. What must hold

Not written, and named here as not written — this note is the contract, not the delivery. In the
repository's own order: the store, then resolution, then the lifecycle, then the surface.

**The store.** A sidecar read back as it was written, verbatim. An absent one is an absence and
nothing else, exactly as `Registry.Steps` learnt to treat `os.IsNotExist` and nothing else as
absence. An unreadable one, or one of a version this build does not know, is refused rather than
half-understood. A sidecar whose `route` names another identity is not believed, and its phrases
are not offered.

**Resolution, both halves and both directions.** At the door: a phrase resolves to the canonical
routine; a slug still beats every alias; a phrase that resolves to nothing is still handed to
Director as words rather than claimed. In Director's resolver: an alias answered with the plugin
absent, reporting `exact` and `not-asked`, which is the Ollama-free claim stated as an assertion
rather than as a hope; an alias of an in-place routine not offered when nothing is known to be in
front; an alias of an unrunnable routine not offered at all; and a phrase two eligible routines
answer to producing the question with both canonical routines named and nothing selected.

**The lifecycle, one case per row of §4**, including the two that must change NOTHING — a step
edit and a regeneration — because a rule that something is untouched is only a rule if something
fails when it is touched.

**And the two that are about what a person sees.** A resolution by alias displaying the canonical
routine and its scope, with the phrase named as a phrase. And a removal that names a phrase
succeeding against a list that has changed in some other way, which is R11's claim that there is no
index to misapply.

## 6. The increments

1. **The store, resolution and the lifecycle** — BUILT. `internal/routes` owns the sidecar and the
   comparison, `internal/orchestrator` the add and remove, `internal/resolveroute` the labels, and
   every row of §4 goes through the production path rather than a stand-in.
2. **The surface** — BUILT. `plugins/studio/answers.go` and the panel on the routine card: the
   phrases listed, one added, one removed by name, and a collision named rather than resolved. A
   CLI entrance was not specified and was not built.

Neither needs a screen: the comparisons are pure, the lifecycle rows are file operations, and a
page can be rendered headless against a stub as the step panel's was.

## What this would NOT establish

That a routine reached by one of its other names presses anything **on a desktop**. Everything
above is a name resolving to an identity; the press is the same press it already was, through the
same door, with the same guards. A live trial is a separate thing and this slice does not move it.
