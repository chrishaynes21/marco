---
type: decision
status: accepted
date: 2026-08-20
supersedes: []
affects:
  - learned-plays
  - execution
source_paths:
  - cmd/director/learnedplay.go
  - internal/routes/origin.go
  - internal/routes/registry.go
---

# ADR-080 — a learned Play is asked for from anywhere

## The live failure

The Phase-0 acceptance, measured. A Play was learned cleanly — three durable Places, two edges,
2/2 `directly_verified`, saved and registered with no developer command. Four minutes later, with
Discord in front, the Audience asked for it through the product surface:

```
dispatch - no route found  name=open-mouse-settings  app=discord
I don't know "open-mouse-settings" yet. Teach it now? [y]es / [n]o:
```

Marco offered to Learn a Play it had just learned.

## Why

A learned Play registered as a **context** route (`routes.Route{App:…}`, `Focus` false), which
puts it in `<app>/context/` — the foreground-only scope. `Registry.Resolve` reaches a context
route only when that application is *already in front*.

So the Play could be asked for only while the Audience was already looking at the thing it would
take them to. Every other position — which is every position that matters — missed the resolver
entirely and fell through to the Learn offer.

## The decision

**A learned Play registers as a FOCUS route.**

`FocusDir` is documented as "runs anywhere, activates", and `Resolve` step 3 exists precisely to
reach *any other application's* focus route. That is the learned-Play contract exactly:
`Runtime.PerformGoal` brings the application forward itself ([[ADR-078-a-learned-play-is-performed-by-the-director]])
**before** it reads the Stage. The scope was already describing the behaviour; the registration
was not using it.

This is not a widening of authority. Resolution is not permission
([[ADR-029-resolution-is-not-permission]]) — the authority door is unmoved and still asks before a
learned Play performs.

### A collision is about a name, not a file

Making the Play a focus route immediately broke a safety property, and the suite said so:

```
--- FAIL: TestCollisionsAreRefusedRatherThanResolved
    a learned play overwrote somebody's authored route
```

`Register` refused on `r.Has(rt)`, and `Has` is **scope-exact**: it asks whether *this file*
exists. A learned Play now looks in `<app>/focus/` while somebody's authored play of the same
name sits in `<app>/context/`. Scope-exact, they do not collide. To the person who asks for the
name they very much do, and the one that answers is whichever the resolver reaches first.

So the guard now asks whether the **name** is already answerable, in every scope that could answer
it — both of the application's scopes and global (`Registry.nameTaken`). Registering may not put a
second Play behind a name that already means something.

## What this does not change

The wall between saved and registered ([[ADR-079-a-demonstration-the-audience-named-is-a-play-they-may-ask-for]]),
the authority seam, and the performer are all untouched. This is one field on the route that gets
registered, and the collision guard that field revealed.

## Enforced by

- `cmd/director/learntail_test.go` — `TestALearnedPlayIsRegisteredWhenItIsSaved` now requires the
  Play to resolve from another application AND to resolve as a focus route. Deleting
  `Focus: true` fails it (verified).
- `cmd/director/lifecyclewiring_test.go` — `TestCollisionsAreRefusedRatherThanResolved`. Deleting
  any arm of `nameTaken` fails it (verified).

## Related

[[ADR-078-a-learned-play-is-performed-by-the-director]] ·
[[ADR-079-a-demonstration-the-audience-named-is-a-play-they-may-ask-for]] ·
[[ADR-029-resolution-is-not-permission]] · [[Learned-Plays]]
