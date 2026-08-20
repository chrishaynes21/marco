---
type: decision
status: accepted
date: 2026-08-20
supersedes: []
affects:
  - invocation
  - learned-plays
  - plays
  - semantic-memory
source_paths:
  - cmd/director/perform.go
  - cmd/marco/perform.go
  - internal/director/service/protocol.go
  - internal/routes/origin.go
---

# ADR-084 — a Play's identity is its subject; its name is a label

## The join that lost a play to an apostrophe

A learned Play is performed by the Director ([[ADR-078-a-learned-play-is-performed-by-the-director]]),
so `cmd/marco` has to tell the service WHICH remembered outcome the Audience asked for. It used to
tell it in words:

```
"open dad's settings"  --routes.Slug-->  open-dad-s-settings
                       --prettyRoute-->  "open dad s settings"
                       --EqualFold----->  Goal.Name "Open Dad's Settings"    ✗
```

`routes.Slug` discards punctuation and collapses runs of whitespace, and `prettyRoute` is not its
inverse. So the round trip is lossy for every phrase that is not made of plain words, and the
Director answered `not_learned` for a behaviour it had watched, rehearsed, verified and written
down four minutes earlier. `"Open Mouse Settings!"`, doubled spaces and `"e-mail steve"` all failed
identically.

The failure mode is the worst available one: **Marco refuses a play it has**, and the reason is
invisible, because both spellings look right to a person reading them.

## The decision

**The subject is the identity. The name is only a label.**

A Play's `routes.Origin.To` and its goal's `observe.Goal.Subject` are the SAME remembered subject
id, written in the same breath by the same Learn pass. The join is on that id:

- `cmd/marco/perform.go` `performIdentity` reads `Origin.To` from the sidecar — only when the
  provenance verifies — and sends it as `PerformQuery.Subject`.
- `service.PerformQuery` carries `Subject` on the wire beside `Name` and `Application`.
- `cmd/director/perform.go` `namesOutcome` compares subject ids **before** it looks at any words:
  exact, case-sensitive, no folding.

**The name still travels, and is consulted for exactly one case.** A goal remembered with NO
subject predates the sidecar, and its name is the only handle anybody has on it. So the words are
the fallback for that goal and no other — once both sides carry an id, an id that does not match
is a *different outcome*, whatever the words say.

Consulting the name first instead would let a goal whose label happens to match the caller's
spelling be performed in place of the one they identified. That ordering is the decision, not an
implementation detail.

**And the application comes from the sidecar too**, rather than from the Route's folder: the
sidecar is what Director wrote, and the folder is where somebody could later move the file.

### This is the same rule the intake applies one layer up

[[ADR-083-one-invocation-intake]] says a clicked Play or a resolved Binding stays that Play and is
not converted back into words to be guessed at again. This is the same sentence about the boundary
between `marco` and the Director service: an identity crosses a process boundary as an identity.

Where the intake's version protects against landing on the WRONG Play, this one protects against
landing on NO play. Both come from the same fact — `routes.Slug` is a one-way function and a
display name is a rendering of its output.

## Considered and rejected

- **Make the slug lossless — escape punctuation so `prettyRoute` inverts it.** A slug is a
  filesystem handle a person reads and types; encoding apostrophes into it makes every filename
  worse for a problem that only exists because words were being used as an identity at all.
  It also cannot fix the plays already on disk.
- **Match the goal name fuzzily.** The identical objection to fuzzy Play lookup in
  [[ADR-083-one-invocation-intake]]: it makes "which behaviour did you mean" a judgement, at the
  one layer that is supposed to be executing a decision somebody already made.
- **Store the goal's exact name on the sidecar and compare THAT.** It works, and it is a second
  copy of a fact that would then need keeping in sync through a rename on either side. The subject
  id is already there, already durable, and already the thing both halves were written from.
- **Look the goal up by name and fall back to the subject.** The wrong way round. Once both sides
  carry an id, a label collision would beat an exact identity — the failure would be silent and
  would perform the wrong behaviour, which is worse than refusing.
- **Refuse any goal that has no subject.** It abandons every goal Learned before the sidecar
  existed, for a purity that buys nothing: those goals have exactly one handle and the name join is
  correct for them.

## Consequences, including the costs

- **A rename on either side no longer breaks performance.** The Play can be renamed, the goal can
  be relabelled, and the id still joins them. That is the point, and it also means a person can no
  longer make two things stop matching by fixing a typo.
- **Two join rules coexist, permanently**, with the id winning. The name branch is reachable only
  for goals with no subject; nothing writes such a goal today, so it is legacy support that will be
  exercised less and less and must not be quietly deleted while any old store exists.
  `TestAPlayWithNoSidecarStillJoinsByName` is what keeps it.
- **The refusal message needs the words.** A subject id in *"I haven't learned how to reach
  subj_7f13…"* is unreadable, so `askedFor` prefers the name and falls back to the id. The label
  therefore still travels on every request even though it decides nothing — one more field that
  exists for humans and not for the machine.
- **The sidecar's provenance now gates the identity.** `performIdentity` reads `Origin.To` only
  when the state verifies, so a Play whose sidecar is unreadable falls back to the lossy name join
  and can still refuse for the old reason. That is the honest behaviour — an unverifiable sidecar
  is not evidence of anything — but it means the old failure is reachable in one narrow state.
- **`ProtocolVersion` went 6 → 7.** Adding `Subject` is additive on the wire, but the version check
  is strict by design, so a stale `director.exe` beside a fresh `marco.exe` refuses rather than
  silently dropping the field. Everything must be rebuilt together.

## Enforced by

- `cmd/director` — `TestTheSubjectIdentifiesTheOutcomeAndTheNameIsOnlyALabel`: deleting the subject
  branch, or testing the name before it, fails it.
- `cmd/director` — `TestAPlayWithNoSidecarStillJoinsByName`: the legacy branch is still reachable,
  so removing it as dead code fails here.
- `internal/director/service` — `TestTheSubjectTravelsOnThePerformRequest`: the id reaches the
  runtime on the wire rather than being dropped at the door.
- `cmd/marco` — `TestTheLearnedPlayNameJoinRoundTrips`: the awkward names (apostrophes,
  punctuation, doubled spaces) now reach their goal instead of refusing. Deleting the `Origin.To`
  read in `performIdentity` fails it.
- `cmd/marco` — `TestALearnedPlayIsPerformedByTheDirector`: the delegation this identity is for is
  still wired.

## Related

[[Invocation]] · [[Learned-Plays]] · [[Plays]] · [[Semantic-Memory]] ·
[[ADR-083-one-invocation-intake]] · [[ADR-078-a-learned-play-is-performed-by-the-director]] ·
[[ADR-028-a-learned-play-is-a-file-with-a-past]] ·
[[ADR-082-a-plays-past-travels-with-the-file]] ·
[[ADR-085-a-performance-is-a-registry-command]] · [[ADR-069-a-name-is-authored-and-can-be-taken-back]]
