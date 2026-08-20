---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - learned-plays
  - semantic-memory
source_paths:
  - cmd/director/learntail.go
  - cmd/director/learnsessionwiring.go
  - cmd/director/learncmd.go
  - internal/director/learn/learn.go
---

# ADR-061 — a derived name is said out loud

## Context

A play is a sentence — `do Downloads's Open` — so saving one needs an actor and a verb. The
phrase a person types when they say what they want learned is the shortest way to give both,
and `playNameFor` derived them from it.

It demanded the phrase be **exactly two words**, refusing anything longer rather than welding
it into an identifier. The reasoning was recorded and was sound for the world it was written
in: *"a three-word phrase silently welded into `DownloadsFolder` is a developer identifier
wearing the user's words, and the whole point of this string is that it is theirs."*

The first live E2E round of the goal-centric milestone found what that cost. `director teach
"open mouse settings"` — the phrase in the roadmap, in E2E.md, and in the milestone's own
statement of what a normal person should be able to say — was refused before anything was
observed, with an instruction to learn `--actor` and `--verb`. The unit tests passed, because
they asserted the parser's rule rather than the acceptance criterion.

Two things had changed underneath the old rule:

- **The phrase and the play name are no longer the same artifact.** Under
  [[ADR-056-a-goal-is-a-destination-not-a-route]] the phrase is the OUTCOME's name, kept
  verbatim on the durable goal in the person's own words. The actor and verb are only the
  sentence a saved play becomes.
- **A multi-word actor was always legal.** `MuteEveryone` is `CheckPlayName`'s own example.

And requiring a person to phrase their outcome in exactly two words is Marco's protocol
leaking into the user's sentence, which is the precise failure Roadmap 34 exists to remove.

## The decision

The first word is the verb, the **rest** is the actor. `open mouse settings` →
`do MouseSettings's Open`. A single word is still refused — it cannot be a sentence of two —
and still at the request boundary, before anybody is asked to demonstrate anything.

**And the derivation is said out loud.** The original objection is answered rather than
overruled: what makes a welded identifier a betrayal of somebody's words is that they meet it
for the first time in a saved file. `teach.Session` carries `Actor`/`Verb`, the view carries
`WillBeCalled`, and the CLI prints it before the demonstration begins:

```
If this works out I'll write it down as `do MouseSettings's Open`.
  (say it your way with --actor and --verb)
```

Seen and correctable is a different thing from silent. The flags remain the escape hatch and
are now offered at the moment they are useful, rather than as the price of a three-word
sentence.

## Consequences

Marco names something from the person's words without their explicit approval of that exact
identifier — a real cost, accepted because the name is announced up front, is bound to
nothing until the save, and can be overridden in the same breath. It does not weaken
[[ADR-031-the-user-names-the-stage]]: a SCREEN's name is executable meaning resolved against
durable memory and still comes only from `UserSuppliedScreenName`. A play's actor and verb are
an identifier for a file, derived from words the person typed for that purpose.

## Enforced by

- `TestThePlayNameIsDerivedOrRefusedButNeverMangled` (`cmd/director/learntail_test.go`) —
  the derivation table, including the acceptance criterion's own phrase, and the one-word
  refusal
- `TestAPhraseThatCannotBecomeAPlayIsRefusedBeforeAnybodyDemonstratesAnything`
  (`cmd/director/learnsessionwiring_test.go`) — refused at the request boundary, not at save time
- `TestTheViewSaysWhatThePlayWillBeCalledBeforeAnythingIsDemonstrated`
  (`cmd/director/learnsessionwiring_test.go`) — said out loud, from the first read

## Related

[[ADR-031-the-user-names-the-stage]] · [[ADR-056-a-goal-is-a-destination-not-a-route]] ·
[[Learned-Plays]]
